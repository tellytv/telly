package main

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/viper"
	m3u "github.com/tombowditch/telly/internal/m3uplus"
	"github.com/tombowditch/telly/internal/providers"
	"github.com/tombowditch/telly/internal/xmltv"
)

// var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
// var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
// var hdRegex = regexp.MustCompile(`hd|4k`)

// hdHomeRunLineupItem is a HDHomeRun specification compatible representation of a Track available in the lineup.
type hdHomeRunLineupItem struct {
	XMLName xml.Name `xml:"Program"    json:"-"`

	AudioCodec  string             `xml:",omitempty" json:",omitempty"`
	DRM         convertibleBoolean `xml:",omitempty" json:",string,omitempty"`
	Favorite    convertibleBoolean `xml:",omitempty" json:",string,omitempty"`
	GuideName   string             `xml:",omitempty" json:",omitempty"`
	GuideNumber int                `xml:",omitempty" json:",string,omitempty"`
	HD          convertibleBoolean `xml:",omitempty" json:",string,omitempty"`
	URL         string             `xml:",omitempty" json:",omitempty"`
	VideoCodec  string             `xml:",omitempty" json:",omitempty"`

	provider        providers.Provider
	providerChannel providers.ProviderChannel
}

func newHDHRItem(provider *providers.Provider, providerChannel *providers.ProviderChannel) hdHomeRunLineupItem {
	return hdHomeRunLineupItem{
		DRM:             convertibleBoolean(false),
		GuideName:       providerChannel.Name,
		GuideNumber:     providerChannel.Number,
		Favorite:        convertibleBoolean(providerChannel.Favorite),
		HD:              convertibleBoolean(providerChannel.HD),
		URL:             fmt.Sprintf("http://%s/auto/v%d", viper.GetString("web.base-address"), providerChannel.Number),
		provider:        *provider,
		providerChannel: *providerChannel,
	}
}

// lineup contains the state of the application.
type lineup struct {
	Sources []providers.Provider

	Scanning bool

	// Stores the channel number for found channels without a number.
	assignedChannelNumber int
	// If true, use channel numbers found in EPG, if any, before assigning.
	xmlTVChannelNumbers bool

	channels map[int]hdHomeRunLineupItem
}

// newLineup returns a new lineup for the given config struct.
func newLineup() *lineup {
	var cfgs []providers.Configuration

	if unmarshalErr := viper.UnmarshalKey("source", &cfgs); unmarshalErr != nil {
		log.WithError(unmarshalErr).Panicln("Unable to unmarshal source configuration to slice of providers.Configuration, check your configuration!")
	}

	if viper.IsSet("iptv.playlist") {
		log.Warnln("Legacy --iptv.playlist argument or environment variable provided, using Custom provider with default configuration, this may fail! If so, you should use a configuration file for full flexibility.")
		regexStr := ".*"
		if viper.IsSet("filter.regex") {
			regexStr = viper.GetString("filter.regex")
		}
		cfgs = append(cfgs, providers.Configuration{
			Name:      "Legacy provider created using arguments/environment variables",
			M3U:       viper.GetString("iptv.playlist"),
			Provider:  "custom",
			Filter:    regexStr,
			FilterRaw: true,
		})
	}

	lineup := &lineup{
		assignedChannelNumber: viper.GetInt("iptv.starting-channel"),
		xmlTVChannelNumbers:   viper.GetBool("iptv.xmltv-channels"),
		channels:              make(map[int]hdHomeRunLineupItem),
	}

	for _, cfg := range cfgs {
		provider, providerErr := cfg.GetProvider()
		if providerErr != nil {
			panic(providerErr)
		}

		lineup.Sources = append(lineup.Sources, provider)
	}

	return lineup
}

// Scan processes all sources.
func (l *lineup) Scan() error {

	l.Scanning = true

	totalAddedChannels := 0

	for _, provider := range l.Sources {
		addedChannels, providerErr := l.processProvider(provider)
		if providerErr != nil {
			log.WithError(providerErr).Errorln("error when processing provider")
		}
		totalAddedChannels = totalAddedChannels + addedChannels
	}

	if totalAddedChannels > 420 {
		log.Panicf("telly has loaded more than 420 channels (%d) into the lineup. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You must use regular expressions to filter out channels. You can also start another Telly instance.", totalAddedChannels)
	}

	l.Scanning = false

	return nil
}

func (l *lineup) processProvider(provider providers.Provider) (int, error) {
	addedChannels := 0
	m3u, channelMap, programmeMap, prepareErr := l.prepareProvider(provider)
	if prepareErr != nil {
		log.WithError(prepareErr).Errorln("error when preparing provider")
	}

	if provider.Configuration().SortKey != "" {
		sortKey := provider.Configuration().SortKey
		sort.Slice(m3u.Tracks, func(i, j int) bool {
			if _, ok := m3u.Tracks[i].Tags[sortKey]; ok {
				log.Panicf("the provided sort key (%s) doesn't exist in the M3U!", sortKey)
				return false
			}
			ii := m3u.Tracks[i].Tags[sortKey]
			jj := m3u.Tracks[j].Tags[sortKey]
			if provider.Configuration().SortReverse {
				return ii < jj
			}
			return ii > jj
		})
	}

	for _, track := range m3u.Tracks {
		// First, we run the filter.
		if !l.FilterTrack(provider, track) {
			log.Debugf("Channel %s didn't pass the provider (%s) filter, skipping!", track.Name, provider.Name())
			return addedChannels, nil
		}

		// Then we do the provider specific translation to a hdHomeRunLineupItem.
		channel, channelErr := provider.ParseTrack(track, channelMap)
		if channelErr != nil {
			return addedChannels, channelErr
		}

		channel, processErr := l.processProviderChannel(channel, programmeMap)
		if processErr != nil {
			log.WithError(processErr).Errorln("error processing track")
		} else if channel == nil {
			log.Infof("Channel %s was returned empty from the provider (%s)", track.Name, provider.Name())
			continue
		}
		addedChannels = addedChannels + 1

		l.channels[channel.Number] = newHDHRItem(&provider, channel)
	}

	log.Infof("Loaded %d channels into the lineup from %s", addedChannels, provider.Name())

	return addedChannels, nil
}

func (l *lineup) prepareProvider(provider providers.Provider) (*m3u.Playlist, map[string]xmltv.Channel, map[string][]xmltv.Programme, error) {
	cacheFiles := provider.Configuration().CacheFiles

	reader, m3uErr := getM3U(provider.PlaylistURL(), cacheFiles)
	if m3uErr != nil {
		log.WithError(m3uErr).Errorln("unable to get m3u file")
		return nil, nil, nil, m3uErr
	}

	rawPlaylist, err := m3u.Decode(reader)
	if err != nil {
		log.WithError(err).Errorln("unable to parse m3u file")
		return nil, nil, nil, err
	}

	channelMap, programmeMap, epgErr := l.prepareEPG(provider, cacheFiles)
	if epgErr != nil {
		log.WithError(epgErr).Errorln("error when parsing EPG")
		return nil, nil, nil, epgErr
	}

	return rawPlaylist, channelMap, programmeMap, nil
}

func (l *lineup) processProviderChannel(channel *providers.ProviderChannel, programmeMap map[string][]xmltv.Programme) (*providers.ProviderChannel, error) {
	if channel.EPGChannel != nil {
		channel.EPGProgrammes = programmeMap[channel.EPGMatch]
	}

	if !l.xmlTVChannelNumbers || channel.Number == 0 {
		channel.Number = l.assignedChannelNumber
		l.assignedChannelNumber = l.assignedChannelNumber + 1
	}

	if channel.EPGChannel != nil && channel.EPGChannel.LCN == 0 {
		channel.EPGChannel.LCN = channel.Number
	}

	if channel.Logo != "" && channel.EPGChannel != nil && !containsIcon(channel.EPGChannel.Icons, channel.Logo) {
		channel.EPGChannel.Icons = append(channel.EPGChannel.Icons, xmltv.Icon{Source: channel.Logo})
	}

	return channel, nil
}

func (l *lineup) FilterTrack(provider providers.Provider, track m3u.Track) bool {
	config := provider.Configuration()
	if config.Filter == "" {
		return true
	}

	filterRegex, regexErr := regexp.Compile(config.Filter)
	if regexErr != nil {
		log.WithError(regexErr).Panicln("your regex is invalid")
		return false
	}

	if config.FilterRaw {
		return filterRegex.MatchString(track.Raw)
	}

	log.Debugf("track.Tags %+v", track.Tags)

	filterKey := provider.RegexKey()
	if config.FilterKey != "" {
		if key, ok := track.Tags[config.FilterKey]; key != "" && ok {
			filterKey = config.FilterKey
		} else {
			log.Panicf("the provided filter key (%s) does not exist or is blank", config.FilterKey)
		}
	}

	if _, ok := track.Tags[filterKey]; !ok {
		log.Panicf("Provided filter key %s doesn't exist in M3U tags", filterKey)
	}

	log.Debugf("Checking if filter (%s) matches string %s", config.Filter, track.Tags[filterKey])

	return filterRegex.MatchString(track.Tags[filterKey])

}

func (l *lineup) prepareEPG(provider providers.Provider, cacheFiles bool) (map[string]xmltv.Channel, map[string][]xmltv.Programme, error) {
	var epg *xmltv.TV
	epgChannelMap := make(map[string]xmltv.Channel)
	epgProgrammeMap := make(map[string][]xmltv.Programme)
	if provider.EPGURL() != "" {
		var epgErr error
		epg, epgErr = getXMLTV(provider.EPGURL(), cacheFiles)
		if epgErr != nil {
			return epgChannelMap, epgProgrammeMap, epgErr
		}

		for _, channel := range epg.Channels {
			epgChannelMap[channel.ID] = channel

			for _, programme := range epg.Programmes {
				if programme.Channel == channel.ID {
					epgProgrammeMap[channel.ID] = append(epgProgrammeMap[channel.ID], *provider.ProcessProgramme(programme))
				}
			}
		}
	}

	return epgChannelMap, epgProgrammeMap, nil
}

func getM3U(path string, cacheFiles bool) (io.Reader, error) {
	safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)
	log.Infof("Loading M3U from %s", safePath)

	file, _, err := getFile(path, cacheFiles)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func getXMLTV(path string, cacheFiles bool) (*xmltv.TV, error) {
	safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)
	log.Infof("Loading XMLTV from %s", safePath)
	file, _, err := getFile(path, cacheFiles)
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

func getFile(path string, cacheFiles bool) (io.Reader, string, error) {
	transport := "disk"

	if strings.HasPrefix(strings.ToLower(path), "http") {

		resp, err := http.Get(path)
		if err != nil {
			return nil, transport, err
		}

		// defer func() {
		// 	err := resp.Body.Close()
		// 	if err != nil {
		// 		log.WithError(err).Panicln("error when closing HTTP body reader")
		// 	}
		// }()

		if strings.HasSuffix(strings.ToLower(path), ".gz") {
			log.Infof("File (%s) is gzipp'ed, ungzipping now, this might take a while", path)
			gz, gzErr := gzip.NewReader(resp.Body)
			if gzErr != nil {
				return nil, transport, gzErr
			}

			defer func() {
				err := gz.Close()
				if err != nil {
					log.WithError(err).Panicln("error when closing gzip reader")
				}
			}()

			if cacheFiles {
				return writeFile(path, transport, gz)
			}

			return gz, transport, nil
		}

		if cacheFiles {
			return writeFile(path, transport, resp.Body)
		}

		return resp.Body, transport, nil
	}

	file, fileErr := os.Open(path)
	if fileErr != nil {
		return nil, transport, fileErr
	}

	return file, transport, nil
}

func writeFile(path, transport string, reader io.Reader) (io.Reader, string, error) {
	// buf := new(bytes.Buffer)
	// buf.ReadFrom(reader)
	// buf.Bytes()
	return reader, transport, nil
}

func containsIcon(s []xmltv.Icon, e string) bool {
	for _, ss := range s {
		if e == ss.Source {
			return true
		}
	}
	return false
}
