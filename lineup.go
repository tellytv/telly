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
	"strconv"
	"strings"

	"github.com/nathanjjohnson/GoSchedulesDirect"
	"github.com/spf13/viper"
	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/providers"
	"github.com/tellytv/telly/internal/xmltv"
)

// var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
// var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
// var hdRegex = regexp.MustCompile(`hd|4k`)
var xmlNSRegex = regexp.MustCompile(`(\d).(\d).(?:(\d)/(\d))?`)

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

	sd *GoSchedulesDirect.Client
}

// newLineup returns a new lineup for the given config struct.
func newLineup() *lineup {
	var cfgs []providers.Configuration

	if unmarshalErr := viper.UnmarshalKey("source", &cfgs); unmarshalErr != nil {
		log.WithError(unmarshalErr).Panicln("Unable to unmarshal source configuration to slice of providers.Configuration, check your configuration!")
	}

	if viper.GetString("iptv.playlist") != "" {
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

	lineup.sd = GoSchedulesDirect.NewClient(viper.GetString("schedulesdirect.username"), viper.GetString("schedulesdirect.password"))

	status, statusErr := lineup.sd.GetStatus()
	if statusErr != nil {
		panic(statusErr)
	}
	log.Infof("SD status %+v", status)

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

	successChannels := []string{}
	failedChannels := []string{}

	for _, track := range m3u.Tracks {
		// First, we run the filter.
		if !l.FilterTrack(provider, track) {
			failedChannels = append(failedChannels, track.Name)
			continue
		} else {
			successChannels = append(successChannels, track.Name)
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

	log.Debugf("These channels (%d) passed the filter and successfully parsed: %s", len(successChannels), strings.Join(successChannels, ", "))
	log.Debugf("These channels (%d) did NOT pass the filter: %s", len(failedChannels), strings.Join(failedChannels, ", "))

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

	if closeM3UErr := reader.Close(); closeM3UErr != nil {
		log.WithError(closeM3UErr).Panicln("error when closing m3u reader")
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

		needsMoreInfo := make(map[string]xmltv.Programme) // TMSID:programme
		haveAllInfo := make(map[string][]xmltv.Programme) // channel number:[]programme

		for _, channel := range epg.Channels {
			epgChannelMap[channel.ID] = channel

			for _, programme := range epg.Programmes {
				if programme.Channel == channel.ID {
					epgProgrammeMap[channel.ID] = append(epgProgrammeMap[channel.ID], *provider.ProcessProgramme(programme))
					if len(programme.EpisodeNums) == 1 && programme.EpisodeNums[0].System == "dd_progid" {
						needsMoreInfo[programme.EpisodeNums[0].Value] = programme
					} else {
						haveAllInfo[channel.ID] = append(haveAllInfo[channel.ID], *provider.ProcessProgramme(programme))
					}
				}
			}
		}

		tmsIDs := make([]string, 0)

		// r := strings.NewReplacer("/", "", ".", "")

		for tmsID := range needsMoreInfo {
			splitID := strings.Split(tmsID, ".")
			tmsIDs = append(tmsIDs, fmt.Sprintf("%s%s", splitID[0], splitID[1]))
		}

		log.Infof("GETTING %d programs from SD", len(tmsIDs))

		//ids := []string{"EP00000204.0125.0/2", "EP00000204.0126.1/2", "EP03022620.0011.0/3", "EP03022786.0001", "EP03022786.0001", "EP03022786.0001", "EP03022786.0001", "EP03023628.0001", "EP03023750.0001", "EP03023787.0001", "EP03023787.0002", "EP03023971.0001", "EP03025363.0001", "EP03025363.0002", "EP03025363.0003", "EP03025363.0004", "EP03025363.0005", "EP03025363.0006", "EP03026541.0001", "EP03026541.0001", "EP03026541.0001", "EP03027284.0005", "EP03027284.0005", "EP03029229.0001", "MV00000031.0000", "SH00246313.0000", "SH02485979.0000.0/3", "SH02485979.0000.1/3"}

		allResponses := make([]GoSchedulesDirect.ProgramInfo, len(tmsIDs))

		for _, chunk := range chunkStringSlice(tmsIDs, 5000) {
			moreInfo, moreInfoErr := l.sd.GetProgramInfo(chunk)
			if moreInfoErr != nil {
				log.WithError(moreInfoErr).Errorln("Error when getting more program details from Schedules Direct")
				return epgChannelMap, epgProgrammeMap, moreInfoErr
			}

			allResponses = append(allResponses, moreInfo...)
		}

		log.Infoln("Got %d responses from SD", len(allResponses))

		for _, program := range allResponses {
			newProgram := MergeSchedulesDirectAndXMLTVProgramme(needsMoreInfo[program.ProgramID], program)
			log.Infof("newProgram %+v", newProgram)
		}

		//panic("bye")

		// needsMoreInfo
		//epgProgrammeMap[channel.ID] = append(epgProgrammeMap[channel.ID], *provider.ProcessProgramme(programme))

	}

	return epgChannelMap, epgProgrammeMap, nil
}

func getM3U(path string, cacheFiles bool) (io.ReadCloser, error) {
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

	if closeXMLErr := file.Close(); closeXMLErr != nil {
		log.WithError(closeXMLErr).Panicln("error when closing xml reader")
	}

	return tvSetup, nil
}

func getFile(path string, cacheFiles bool) (io.ReadCloser, string, error) {
	transport := "disk"

	if strings.HasPrefix(strings.ToLower(path), "http") {

		resp, err := http.Get(path)
		if err != nil {
			return nil, transport, err
		}

		if strings.HasSuffix(strings.ToLower(path), ".gz") {
			log.Infof("File (%s) is gzipp'ed, ungzipping now, this might take a while", path)
			gz, gzErr := gzip.NewReader(resp.Body)
			if gzErr != nil {
				return nil, transport, gzErr
			}

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

func writeFile(path, transport string, reader io.ReadCloser) (io.ReadCloser, string, error) {
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

func chunkStringSlice(sl []string, chunkSize int) [][]string {
	var divided [][]string

	for i := 0; i < len(sl); i += chunkSize {
		end := i + chunkSize

		if end > len(sl) {
			end = len(sl)
		}

		divided = append(divided, sl[i:end])
	}
	return divided
}

func MergeSchedulesDirectAndXMLTVProgramme(programme xmltv.Programme, sdProgram GoSchedulesDirect.ProgramInfo) xmltv.Programme {

	allTitles := make([]string, 0)

	for _, title := range programme.Titles {
		allTitles = append(allTitles, title.Value)
	}

	for _, title := range sdProgram.Titles {
		allTitles = append(allTitles, title.Title120)
	}

	for _, title := range UniqueStrings(allTitles) {
		programme.Titles = append(programme.Titles, xmltv.CommonElement{Value: title})
	}

	allKeywords := make([]string, 0)

	for _, keyword := range programme.Keywords {
		allKeywords = append(allKeywords, keyword.Value)
	}

	for keywordType, keywords := range sdProgram.Keywords {
		log.Infoln("Adding keywords category", keywordType)
		for _, keyword := range keywords {
			allKeywords = append(allKeywords, keyword)
		}
	}

	// FIXME: We should really be making sure that we passthrough languages.
	allDescriptions := make([]string, 0)

	for _, description := range programme.Descriptions {
		allDescriptions = append(allDescriptions, description.Value)
	}

	for _, descriptions := range sdProgram.Descriptions {
		for _, description := range descriptions {
			allDescriptions = append(allDescriptions, description.Description)
		}
	}

	for _, description := range UniqueStrings(allDescriptions) {
		programme.Descriptions = append(programme.Descriptions, xmltv.CommonElement{Value: description})
	}

	for _, keyword := range UniqueStrings(allKeywords) {
		programme.Keywords = append(programme.Keywords, xmltv.CommonElement{Value: keyword})
	}

	allRatings := make(map[string]string, 0)

	for _, rating := range programme.Ratings {
		allRatings[rating.System] = rating.Value
	}

	for _, rating := range sdProgram.ContentRating {
		allRatings[rating.Body] = rating.Code
	}

	for system, rating := range allRatings {
		programme.Ratings = append(programme.Ratings, xmltv.Rating{Value: rating, System: system})
	}

	hasXMLTVNS := false

	for _, epNum := range programme.EpisodeNums {
		if epNum.System == "xmltv_ns" {
			hasXMLTVNS = true
		}
	}

	if !hasXMLTVNS {
		seasonNumber := 0
		episodeNumber := 0
		numbersFilled := false

		for _, meta := range sdProgram.Metadata {
			for _, metadata := range meta {
				if metadata.Season != nil {
					seasonNumber = *metadata.Season - 1 // SD metadata isnt 0 index
					numbersFilled = true
				}
				if metadata.Episode != nil {
					episodeNumber = *metadata.Episode - 1
					numbersFilled = true
				}
			}
		}

		if numbersFilled {
			// FIXME: There is currently no way to determine multipart episodes from SD.
			// We could use the dd_progid to determine it though.
			xmlTVNS := fmt.Sprintf("%d.%d.0/1", seasonNumber, episodeNumber)
			programme.EpisodeNums = append(programme.EpisodeNums, xmltv.EpisodeNum{System: "xmltv_ns", Value: xmlTVNS})
		}
	}

	return programme
}

func extractXMLTVNS(str string) (int, int, int, int, error) {
	matches := xmlNSRegex.FindAllStringSubmatch(str, -1)

	if len(matches) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid xmltv_ns: %s", str)
	}

	season, seasonErr := strconv.Atoi(matches[0][1])
	if seasonErr != nil {
		return 0, 0, 0, 0, seasonErr
	}

	episode, episodeErr := strconv.Atoi(matches[0][2])
	if episodeErr != nil {
		return 0, 0, 0, 0, episodeErr
	}

	currentPartNum := 0
	totalPartsNum := 0

	if len(matches[0]) > 2 && matches[0][3] != "" {
		currentPart, currentPartErr := strconv.Atoi(matches[0][3])
		if currentPartErr != nil {
			return 0, 0, 0, 0, currentPartErr
		}
		currentPartNum = currentPart
	}

	if len(matches[0]) > 3 && matches[0][4] != "" {
		totalParts, totalPartsErr := strconv.Atoi(matches[0][4])
		if totalPartsErr != nil {
			return 0, 0, 0, 0, totalPartsErr
		}
		totalPartsNum = totalParts
	}

	// if season > 0 {
	// 	season = season - 1
	// }

	// if episode > 0 {
	// 	episode = episode - 1
	// }

	// if currentPartNum > 0 {
	// 	currentPartNum = currentPartNum - 1
	// }

	// if totalPartsNum > 0 {
	// 	totalPartsNum = totalPartsNum - 1
	// }

	return season, episode, currentPartNum, totalPartsNum, nil
}

func UniqueStrings(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}

	return u
}
