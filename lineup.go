package main

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tombowditch/telly/m3u"
	"github.com/tombowditch/telly/xmltv"
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

	XMLTVChannel    xmlTVChannel      `json:",omitempty"`
	XMLTVProgrammes []xmltv.Programme `json:",omitempty"`
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

// HDHomeRunChannel is a HDHomeRun specification compatible representation of a Track available in the Lineup.
type HDHomeRunChannel struct {
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

	xmlTvChannelMap     map[string]xmlTVChannel
	channelsInXMLTv     []string
	xmlTv               xmltv.TV
	xmlTvSourceInfoURL  []string
	xmlTvSourceInfoName []string
	xmlTvSourceDataURL  []string
}

// NewLineup returns a new Lineup for the given config struct.
func NewLineup(opts config) *Lineup {
	tv := xmltv.TV{
		GeneratorInfoName: namespaceWithVersion,
		GeneratorInfoURL:  "https://github.com/tombowditch/telly",
	}

	lineup := &Lineup{
		xmlTv:                 tv,
		xmlTvChannelMap:       make(map[string]xmlTVChannel),
		StartingChannelNumber: opts.StartingChannel,
		channelNumber:         opts.StartingChannel,
		ObfuscateURL:          !opts.DirectMode,
		Refreshing:            true,
		LastRefreshed:         time.Now(),
	}

	return lineup
}

// AddPlaylist adds a new playlist to the Lineup.
func (l *Lineup) AddPlaylist(plist string) error {
	// Attempt to split the string by semi colon for complex config passing with m3uPath,xmlPath,name
	splitStr := strings.Split(plist, ";")
	path := splitStr[0]
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

	if len(splitStr) > 1 {
		epg, epgReadErr := l.getXMLTV(splitStr[1])
		if epgReadErr != nil {
			log.WithError(epgReadErr).Errorln("error getting XMLTV")
			return epgReadErr
		}

		chanMap, chanMapErr := l.processXMLTV(epg)
		if chanMapErr != nil {
			log.WithError(chanMapErr).Errorln("Error building channel mapping")
		}

		for chanID, chann := range chanMap {
			l.xmlTvChannelMap[chanID] = chann
		}
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

	for idx, track := range playlist.Tracks {

		channelNumber := l.channelNumber

		if xmlChan, ok := l.xmlTvChannelMap[track.TvgID]; ok && !contains(l.channelsInXMLTv, track.TvgID) {
			log.Infoln("found an entry in xmlTvChannelMap for", track.TvgID)
			channelNumber = xmlChan.Number
			l.channelsInXMLTv = append(l.channelsInXMLTv, track.TvgID)
			track.XMLTVChannel = xmlChan
			l.xmlTv.Channels = append(l.xmlTv.Channels, xmlChan.Original)
			if xmlChan.Programmes != nil {
				track.XMLTVProgrammes = xmlChan.Programmes
				l.xmlTv.Programmes = append(l.xmlTv.Programmes, xmlChan.Programmes...)
			}
			playlist.Tracks[idx] = track
		}

		channel := track.Channel(channelNumber, l.ObfuscateURL)

		playlist.Channels = append(playlist.Channels, *channel)

		if channelNumber == l.channelNumber { // Only increment lineup channel number if its for a channel that didnt have a XMLTV entry.
			l.channelNumber = l.channelNumber + 1
		}
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

	file, transport, err := l.getFile(path)
	if err != nil {
		return nil, nil, err
	}

	return file, &M3UFile{
		Path:      path,
		SafePath:  safePath,
		Transport: transport,
	}, nil
}

func (l *Lineup) getXMLTV(path string) (*xmltv.TV, error) {
	file, _, err := l.getFile(path)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)
	tvSetup := new(xmltv.TV)
	if err := decoder.Decode(tvSetup); err != nil {
		log.WithError(err).Errorln("Could not decode xmltv programme")
		return nil, err
	}

	return tvSetup, nil
}

func (l *Lineup) getFile(path string) (io.Reader, string, error) {
	safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)
	log.Infof("Loading file from %s", safePath)

	transport := "disk"

	if strings.HasPrefix(strings.ToLower(path), "http") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, transport, err
		}
		//defer resp.Body.Close()

		return resp.Body, transport, nil
	}

	file, fileErr := os.Open(path)
	if fileErr != nil {
		return nil, transport, fileErr
	}

	return file, transport, nil
}

var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString

type xmlTVChannel struct {
	ID        string
	Number    int
	CallSign  string
	ShortName string
	LongName  string

	NumberAssigned bool

	Programmes []xmltv.Programme

	Original xmltv.Channel
}

func (l *Lineup) processXMLTV(tv *xmltv.TV) (map[string]xmlTVChannel, error) {
	programmeMap := make(map[string][]xmltv.Programme)
	for _, programme := range tv.Programmes {
		programmeMap[programme.Channel] = append(programmeMap[programme.Channel], programme)
	}

	channelMap := make(map[string]xmlTVChannel, 0)
	startManualNumber := 10000
	for _, tvChann := range tv.Channels {
		xTVChan := xmlTVChannel{
			ID:       tvChann.ID,
			Original: tvChann,
		}
		if programmes, ok := programmeMap[tvChann.ID]; ok {
			xTVChan.Programmes = programmes
		}
		displayNames := []string{}
		for _, displayName := range tvChann.DisplayNames {
			displayNames = append(displayNames, displayName.Value)
		}
		sort.StringSlice(displayNames).Sort()
		for i := 0; i < 10; i++ {
			iterateDisplayNames(displayNames, &xTVChan)
		}
		if xTVChan.Number == 0 {
			xTVChan.Number = startManualNumber + 1
			startManualNumber = xTVChan.Number
			xTVChan.NumberAssigned = true
		}
		channelMap[xTVChan.ID] = xTVChan
	}
	return channelMap, nil
}

func iterateDisplayNames(displayNames []string, xTVChan *xmlTVChannel) {
	for _, displayName := range displayNames {
		if channelNumberRegex(displayName) {
			if chanNum, chanNumErr := strconv.Atoi(displayName); chanNumErr == nil {
				log.Debugln(displayName, "is channel number!")
				xTVChan.Number = chanNum
			}
		} else if !strings.HasPrefix(displayName, fmt.Sprintf("%d", xTVChan.Number)) {
			if xTVChan.LongName == "" {
				xTVChan.LongName = displayName
				log.Debugln(displayName, "is long name!")
			} else if !callSignRegex(displayName) && len(xTVChan.LongName) < len(displayName) {
				xTVChan.ShortName = xTVChan.LongName
				xTVChan.LongName = displayName
				log.Debugln(displayName, "is NEW long name, replacing", xTVChan.ShortName)
			} else if callSignRegex(displayName) {
				xTVChan.CallSign = displayName
				log.Debugln(displayName, "is call sign!")
			}
		}
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
