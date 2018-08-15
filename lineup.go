package main

import (
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

	"github.com/spf13/viper"
	"github.com/tombowditch/telly/m3u"
	"github.com/tombowditch/telly/providers"
	"github.com/tombowditch/telly/xmltv"
)

var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
var hdRegex = regexp.MustCompile(`hd|4k`)

// Track describes a single M3U segment. This struct includes m3u.Track as well as specific IPTV fields we want to get.
type Track struct {
	*m3u.Track
	SafeURI          string `json:"URI"`
	Catchup          string `m3u:"catchup" json:",omitempty"`
	CatchupDays      string `m3u:"catchup-days" json:",omitempty"`
	CatchupSource    string `m3u:"catchup-source" json:",omitempty"`
	GroupTitle       string `m3u:"group-title" json:",omitempty"`
	TvgID            string `m3u:"tvg-id" json:",omitempty"`
	TvgLogo          string `m3u:"tvg-logo" json:",omitempty"`
	TvgName          string `m3u:"tvg-name" json:",omitempty"`
	TvgChannelNumber string `m3u:"tvg-chno" json:",omitempty"`
	ChannelID        string `m3u:"channel-id" json:",omitempty"`

	XMLTVChannel    *xmlTVChannel      `json:",omitempty"`
	XMLTVProgrammes *[]xmltv.Programme `json:",omitempty"`
}

func (t *Track) PrettyName() string {
	if t.XMLTVChannel != nil {
		return t.XMLTVChannel.LongName
	} else if t.TvgName != "" {
		return t.TvgName
	} else if t.Track.Name != "" {
		return t.Track.Name
	}

	return t.Name
}

// Playlist describes a single M3U playlist.
type Playlist struct {
	*m3u.Playlist
	*M3UFile

	Tracks              []Track
	Channels            []HDHomeRunChannel
	TracksCount         int
	FilteredTracksCount int
	EPGProvided         bool
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

		if GetStringAsRegex("filter.regexstr").MatchString(track.Raw) == viper.GetBool("filter.regexinclusive") {
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
	Providers []providers.Provider

	Playlists           []Playlist
	PlaylistsCount      int
	TracksCount         int
	FilteredTracksCount int

	StartingChannelNumber int
	channelNumber         int

	Refreshing    bool
	LastRefreshed time.Time `json:",omitempty"`

	xmlTvChannelMap     map[string]xmlTVChannel
	channelsInXMLTv     []string
	xmlTv               xmltv.TV
	xmlTvSourceInfoURL  []string
	xmlTvSourceInfoName []string
	xmlTvSourceDataURL  []string
	xmlTVChannelNumbers bool

	chanNumToURLMap map[string]string
}

// NewLineup returns a new Lineup for the given config struct.
func NewLineup() *Lineup {
	tv := xmltv.TV{
		GeneratorInfoName: namespaceWithVersion,
		GeneratorInfoURL:  "https://github.com/tombowditch/telly",
	}

	lineup := &Lineup{
		xmlTVChannelNumbers:   viper.GetBool("iptv.xmltv-channels"),
		chanNumToURLMap:       make(map[string]string),
		xmlTv:                 tv,
		xmlTvChannelMap:       make(map[string]xmlTVChannel),
		StartingChannelNumber: viper.GetInt("iptv.starting-channel"),
		channelNumber:         viper.GetInt("iptv.starting-channel"),
		Refreshing:            false,
		LastRefreshed:         time.Now(),
	}

	var cfgs []providers.Configuration

	if unmarshalErr := viper.UnmarshalKey("source", &cfgs); unmarshalErr != nil {
		log.WithError(unmarshalErr).Panicln("Unable to unmarshal source configuration to slice of providers.Configuration, check your configuration!")
	}

	for _, cfg := range cfgs {
		log.Infoln("Adding provider", cfg.Name)
		provider, providerErr := cfg.GetProvider()
		if providerErr != nil {
			panic(providerErr)
		}
		if addErr := lineup.AddProvider(provider); addErr != nil {
			log.WithError(addErr).Panicln("error adding new provider to lineup")
		}
	}

	return lineup
}

// AddProvider adds a new Provider to the Lineup.
func (l *Lineup) AddProvider(provider providers.Provider) error {
	reader, info, readErr := l.getM3U(provider.PlaylistURL())
	if readErr != nil {
		log.WithError(readErr).Errorln("error getting m3u")
		return readErr
	}

	rawPlaylist, err := m3u.Decode(reader)
	if err != nil {
		log.WithError(err).Errorln("unable to parse m3u file")
		return err
	}

	if provider.EPGURL() != "" {
		epg, epgReadErr := l.getXMLTV(provider.EPGURL())
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

	playlist, playlistErr := l.NewPlaylist(provider, rawPlaylist, info)
	if playlistErr != nil {
		return playlistErr
	}

	l.Playlists = append(l.Playlists, playlist)
	l.PlaylistsCount = len(l.Playlists)
	l.TracksCount = l.TracksCount + playlist.TracksCount
	l.FilteredTracksCount = l.FilteredTracksCount + playlist.FilteredTracksCount

	return nil
}

// NewPlaylist will return a new and filtered Playlist for the given m3u.Playlist and M3UFile.
func (l *Lineup) NewPlaylist(provider providers.Provider, rawPlaylist *m3u.Playlist, info *M3UFile) (Playlist, error) {
	hasEPG := provider.EPGURL() != ""
	playlist := Playlist{rawPlaylist, info, nil, nil, len(rawPlaylist.Tracks), 0, hasEPG}

	if filterErr := playlist.Filter(); filterErr != nil {
		log.WithError(filterErr).Errorln("error during filtering of channels, check your regex and try again")
		return playlist, filterErr
	}

	for idx, track := range playlist.Tracks {
		tt, channelNumber, hd, ttErr := l.processTrack(provider, track)
		if ttErr != nil {
			return playlist, ttErr
		}

		if hasEPG && tt.XMLTVChannel == nil {
			log.Warnf("%s (#%d) is not being exposed to Plex because there was no EPG data found.", tt.Name, channelNumber)
			continue
		}

		playlist.Tracks[idx] = *tt

		guideName := tt.PrettyName()

		log.Debugln("Assigning", channelNumber, l.channelNumber, "to", guideName)

		hdhr := HDHomeRunChannel{
			GuideNumber: channelNumber,
			GuideName:   guideName,
			URL:         fmt.Sprintf("http://%s/auto/v%d", viper.GetString("web.base-address"), channelNumber),
			HD:          convertibleBoolean(hd),
			DRM:         convertibleBoolean(false),
		}

		if !channelExists(playlist.Channels, hdhr) {
			playlist.Channels = append(playlist.Channels, hdhr)
			l.chanNumToURLMap[strconv.Itoa(channelNumber)] = tt.Track.URI
		}

		if channelNumber == l.channelNumber { // Only increment lineup channel number if its for a channel that didnt have a XMLTV entry.
			l.channelNumber = l.channelNumber + 1
		}

	}

	sort.Slice(l.xmlTv.Channels, func(i, j int) bool {
		first, _ := strconv.Atoi(l.xmlTv.Channels[i].ID)
		second, _ := strconv.Atoi(l.xmlTv.Channels[j].ID)
		return first < second
	})

	playlist.FilteredTracksCount = len(playlist.Tracks)
	exposedChannels.Add(float64(playlist.FilteredTracksCount))
	log.Debugf("Added %d channels to the lineup", playlist.FilteredTracksCount)

	return playlist, nil
}

func (l Lineup) processTrack(provider providers.Provider, track Track) (*Track, int, bool, error) {

	hd := hdRegex.MatchString(strings.ToLower(track.Track.Raw))
	channelNumber := l.channelNumber

	if xmlChan, ok := l.xmlTvChannelMap[track.TvgID]; ok {
		log.Debugln("found an entry in xmlTvChannelMap for", track.Name)
		if l.xmlTVChannelNumbers && xmlChan.Number != 0 {
			channelNumber = xmlChan.Number
		} else {
			xmlChan.Number = channelNumber
		}
		l.channelsInXMLTv = append(l.channelsInXMLTv, track.TvgID)
		track.XMLTVChannel = &xmlChan
		l.xmlTv.Channels = append(l.xmlTv.Channels, xmlChan.RemappedChannel(track))
		if xmlChan.Programmes != nil {
			track.XMLTVProgrammes = &xmlChan.Programmes
			for _, programme := range xmlChan.Programmes {
				newProgramme := programme
				for idx, title := range programme.Titles {
					programme.Titles[idx].Value = strings.Replace(title.Value, " [New!]", "", -1) // Hardcoded fix for Vaders
				}
				newProgramme.Channel = strconv.Itoa(channelNumber)
				if hd {
					if newProgramme.Video == nil {
						newProgramme.Video = &xmltv.Video{}
					}
					newProgramme.Video.Quality = "HDTV"
				}
				l.xmlTv.Programmes = append(l.xmlTv.Programmes, newProgramme)
			}
		}
	}

	return &track, channelNumber, hd, nil
}

// Refresh will rescan all playlists for any channel changes.
func (l Lineup) Refresh() error {

	if l.Refreshing {
		log.Warnln("A refresh is already underway yet, another one was requested")
		return nil
	}

	log.Warnln("Refreshing the lineup!")

	l.Refreshing = true

	existingPlaylists := make([]Playlist, len(l.Playlists))
	copy(existingPlaylists, l.Playlists)

	l.Playlists = nil
	l.TracksCount = 0
	l.FilteredTracksCount = 0
	l.StartingChannelNumber = 0

	// FIXME: Re-implement AddProvider to use a provider.
	// for _, playlist := range existingPlaylists {
	// 	if addErr := l.AddProvider(playlist.M3UFile.Path); addErr != nil {
	// 		return addErr
	// 	}
	// }

	log.Infoln("Done refreshing the lineup!")

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

func (x *xmlTVChannel) RemappedChannel(t Track) xmltv.Channel {
	newX := x.Original
	newX.ID = strconv.Itoa(x.Number)
	if t.TvgLogo != "" {
		newX.Icons = append(newX.Icons, xmltv.Icon{Source: t.TvgLogo})
	}
	if t.Track.Name != "" {
		newX.DisplayNames = append(newX.DisplayNames, xmltv.CommonElement{Value: t.Track.Name})
	}
	return newX
}

func (l *Lineup) processXMLTV(tv *xmltv.TV) (map[string]xmlTVChannel, error) {
	programmeMap := make(map[string][]xmltv.Programme)
	for _, programme := range tv.Programmes {
		programmeMap[programme.Channel] = append(programmeMap[programme.Channel], programme)
	}

	channelMap := make(map[string]xmlTVChannel, 0)
	for _, tvChann := range tv.Channels {
		xTVChan := &xmlTVChannel{
			ID:       tvChann.ID,
			Original: tvChann,
		}
		if programmes, ok := programmeMap[tvChann.ID]; ok {
			xTVChan.Programmes = programmes
		}
		if channelNumberRegex(tvChann.ID) {
			xTVChan.Number, _ = strconv.Atoi(tvChann.ID)
		}
		displayNames := []string{}
		for _, displayName := range tvChann.DisplayNames {
			displayNames = append(displayNames, displayName.Value)
		}
		sort.StringSlice(displayNames).Sort()
		for i := 0; i < 10; i++ {
			extractDisplayNames(displayNames, xTVChan)
		}
		channelMap[xTVChan.ID] = *xTVChan
		// Duplicate this to first display-name just in case the M3U and XMLTV differ significantly.
		for _, dn := range tvChann.DisplayNames {
			channelMap[dn.Value] = *xTVChan
		}
	}

	return channelMap, nil
}

func extractDisplayNames(displayNames []string, xTVChan *xmlTVChannel) {
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

func channelExists(s []HDHomeRunChannel, e HDHomeRunChannel) bool {
	for _, a := range s {
		if a.GuideName == e.GuideName {
			return true
		}
	}
	return false
}
