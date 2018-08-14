package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tombowditch/telly/m3u"
)

// Track describes a single M3U segment. This struct includes m3u.Track as well as specific IPTV fields we want to get.
type Track struct {
	*m3u.Track
	SafeURI       string `json:"URI"`
	Catchup       string `m3u:"catchup" json:",omitempty"`
	CatchupDays   string `m3u:"catchup-days" json:",omitempty"`
	CatchupSource string `m3u:"catchup-source" json:",omitempty"`
	GroupTitle    string `m3u:"group-title" json:",omitempty"`
	TvgID         string `m3u:"tvg-id" json:",omitempty"`
	TvgLogo       string `m3u:"tvg-logo" json:",omitempty"`
	TvgName       string `m3u:"tvg-name" json:",omitempty"`
}

// Channel returns a Channel struct for the given Track.
func (t *Track) Channel(number int, obfuscate bool) *HDHomeRunChannel {
	var finalName string
	if t.TvgName == "" {
		finalName = t.Name
	} else {
		finalName = t.TvgName
	}

	// base64 url
	fullTrackURI := t.URI
	if obfuscate {
		trackURI := base64.StdEncoding.EncodeToString([]byte(t.URI))
		fullTrackURI = fmt.Sprintf("http://%s/stream/%s", opts.BaseAddress.String(), trackURI)
	}

	// if strings.Contains(t.URI, ".m3u8") {
	//   log.Warnln("your .m3u contains .m3u8's. Plex has either stopped supporting m3u8 or it is a bug in a recent version - please use .ts! telly will automatically convert these in a future version. See telly github issue #108")
	// }

	hd := false
	if strings.Contains(strings.ToLower(t.Track.Raw), "hd") {
		hd = true
	}

	return &HDHomeRunChannel{
		GuideNumber: number,
		GuideName:   finalName,
		URL:         fullTrackURI,
		HD:          convertibleBoolean(hd),

		track: t,
	}
}

// Playlist describes a single M3U playlist.
type Playlist struct {
	*m3u.Playlist
	*M3UFile

	Tracks              []Track
	Channels            []HDHomeRunChannel
	TracksCount         int
	FilteredTracksCount int
}

// Filter will filter the raw m3u.Playlist m3u.Track slice into the Track slice of the Playlist.
func (p *Playlist) Filter() error {
	for _, oldTrack := range p.Playlist.Tracks {
		track := Track{
			Track:   oldTrack,
			SafeURI: safeStringsRegex.ReplaceAllStringFunc(oldTrack.URI, stringSafer),
		}

		if unmarshalErr := oldTrack.UnmarshalTags(&track); unmarshalErr != nil {
			return unmarshalErr
		}

		if opts.Regex.MatchString(track.Name) == opts.RegexInclusive {
			p.Tracks = append(p.Tracks, track)
		}
	}

	return nil
}

// M3UFile describes a path and transport to a M3U provided in the configuration.
type M3UFile struct {
	Path      string `json:"-"`
	SafePath  string `json:"Path"`
	Transport string
}

// HDHomeRunChannel is a single channel found in the playlist.
type HDHomeRunChannel struct {
	// These fields match what HDHomeRun uses and Plex expects to see.
	AudioCodec  string             `json:",omitempty"`
	DRM         convertibleBoolean `json:",string,omitempty"`
	Favorite    convertibleBoolean `json:",string,omitempty"`
	GuideName   string             `json:",omitempty"`
	GuideNumber int                `json:",string,omitempty"`
	HD          convertibleBoolean `json:",string,omitempty"`
	URL         string             `json:",omitempty"`
	VideoCodec  string             `json:",omitempty"`

	track *Track
}

// Lineup is a collection of tracks
type Lineup struct {
	Playlists           []Playlist
	PlaylistsCount      int
	TracksCount         int
	FilteredTracksCount int

	StartingChannelNumber int
	channelNumber         int
	ObfuscateURL          bool

	Refreshing    bool
	LastRefreshed time.Time `json:",omitempty"`
}

// NewLineup returns a new Lineup for the given config struct.
func NewLineup(opts config) *Lineup {
	return &Lineup{
		StartingChannelNumber: opts.StartingChannel,
		channelNumber:         opts.StartingChannel,
		ObfuscateURL:          !opts.DirectMode,
		Refreshing:            true,
		LastRefreshed:         time.Now(),
	}
}

// AddPlaylist adds a new playlist to the Lineup.
func (l *Lineup) AddPlaylist(path string) error {
	reader, info, readErr := l.getM3U(path)
	if readErr != nil {
		log.WithError(readErr).Errorln("error getting m3u")
		return readErr
	}

	rawPlaylist, err := m3u.Decode(reader)
	if err != nil {
		log.WithError(err).Errorln("unable to parse m3u file")
		return err
	}

	playlist, playlistErr := l.NewPlaylist(rawPlaylist, info)
	if playlistErr != nil {
		return playlistErr
	}

	l.Playlists = append(l.Playlists, *playlist)
	l.PlaylistsCount = len(l.Playlists)
	l.TracksCount = l.TracksCount + playlist.TracksCount
	l.FilteredTracksCount = l.FilteredTracksCount + playlist.FilteredTracksCount

	return nil
}

// NewPlaylist will return a new and filtered Playlist for the given m3u.Playlist and M3UFile.
func (l *Lineup) NewPlaylist(rawPlaylist *m3u.Playlist, info *M3UFile) (*Playlist, error) {
	playlist := &Playlist{rawPlaylist, info, nil, nil, len(rawPlaylist.Tracks), 0}

	if filterErr := playlist.Filter(); filterErr != nil {
		log.WithError(filterErr).Errorln("error during filtering of channels, check your regex and try again")
		return nil, filterErr
	}

	for _, track := range playlist.Tracks {

		channel := track.Channel(l.channelNumber, l.ObfuscateURL)

		playlist.Channels = append(playlist.Channels, *channel)

		l.channelNumber = l.channelNumber + 1
	}

	playlist.FilteredTracksCount = len(playlist.Tracks)
	exposedChannels.Add(float64(playlist.FilteredTracksCount))
	log.Debugf("Added %d channels to the lineup", playlist.FilteredTracksCount)

	return playlist, nil
}

// Refresh will rescan all playlists for any channel changes.
func (l *Lineup) Refresh() error {

	log.Warnln("Refreshing the lineup!")

	l.Refreshing = true

	existingPlaylists := make([]Playlist, len(l.Playlists))
	copy(existingPlaylists, l.Playlists)

	l.Playlists = nil
	l.TracksCount = 0
	l.FilteredTracksCount = 0
	l.StartingChannelNumber = 0

	for _, playlist := range existingPlaylists {
		if addErr := l.AddPlaylist(playlist.M3UFile.Path); addErr != nil {
			return addErr
		}
	}

	l.LastRefreshed = time.Now()
	l.Refreshing = false

	return nil
}

func (l *Lineup) getM3U(path string) (io.Reader, *M3UFile, error) {
	safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)
	log.Infof("Loading M3U from %s", safePath)

	info := &M3UFile{
		Path:      path,
		SafePath:  safePath,
		Transport: "disk",
	}

	if strings.HasPrefix(strings.ToLower(path), "http") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, nil, err
		}
		//defer resp.Body.Close()

		info.Transport = "http"

		return resp.Body, info, nil
	}

	file, fileErr := os.Open(path)
	if fileErr != nil {
		return nil, nil, fileErr
	}

	return file, info, nil
}
