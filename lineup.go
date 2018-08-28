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
	"time"

	"github.com/spf13/viper"
	"github.com/tellytv/go.schedulesdirect"
	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/providers"
	"github.com/tellytv/telly/internal/xmltv"
)

// var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
// var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
// var hdRegex = regexp.MustCompile(`hd|4k`)
var xmlNSRegex = regexp.MustCompile(`(\d).(\d).(?:(\d)/(\d))?`)
var ddProgIDRegex = regexp.MustCompile(`(?m)(EP|SH|MV|SP)(\d{7,8}).(\d+).?(?:(\d).(\d))?`)

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

	sd *schedulesdirect.Client
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

	if viper.IsSet("schedulesdirect.username") && viper.IsSet("schedulesdirect.password") {
		sdClient, sdClientErr := schedulesdirect.NewClient(viper.GetString("schedulesdirect.username"), viper.GetString("schedulesdirect.password"))
		if sdClientErr != nil {
			log.WithError(sdClientErr).Panicln("error setting up schedules direct client")
		}

		lineup.sd = sdClient
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
		return 0, prepareErr
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
		if viper.GetBool("misc.ignore-epg-icons") {
			channel.EPGChannel.Icons = nil
		}
		channel.EPGChannel.Icons = append(channel.EPGChannel.Icons, xmltv.Icon{Source: channel.Logo})
	}

	return channel, nil
}

func (l *lineup) FilterTrack(provider providers.Provider, track m3u.Track) bool {
	config := provider.Configuration()
	if config.Filter == "" && len(config.IncludeOnly) == 0 {
		return true
	}

	if v, ok := track.Tags[config.IncludeOnlyTag]; len(config.IncludeOnly) > 0 && ok {
		return contains(config.IncludeOnly, v)
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
		filterKey = config.FilterKey
	}

	if key, ok := track.Tags[filterKey]; key != "" && !ok {
		log.Warnf("the provided filter key (%s) does not exist or is blank, skipping track: %s", config.FilterKey, track.Raw)
		return false
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

		augmentWithSD := viper.IsSet("schedulesdirect.username") && viper.IsSet("schedulesdirect.password")

		sdEligible := make(map[string]xmltv.Programme)    // TMSID:programme
		haveAllInfo := make(map[string][]xmltv.Programme) // channel number:[]programme

		for _, channel := range epg.Channels {
			epgChannelMap[channel.ID] = channel

			for _, programme := range epg.Programmes {
				if programme.Channel == channel.ID {
					ddProgID := ""
					if augmentWithSD {
						for _, epNum := range programme.EpisodeNums {
							if epNum.System == "dd_progid" {
								ddProgID = epNum.Value
							}
						}
					}
					if augmentWithSD == true && ddProgID != "" {
						idType, uniqID, epID, _, _, extractErr := extractDDProgID(ddProgID)
						if extractErr != nil {
							log.WithError(extractErr).Errorln("error extracting dd_progid")
							continue
						}
						cleanID := fmt.Sprintf("%s%s%s", idType, padNumberWithZero(uniqID, 8), padNumberWithZero(epID, 4))
						if len(cleanID) < 14 {
							log.Warnf("found an invalid TMS ID/dd_progid, expected length of exactly 14, got %d: %s\n", len(cleanID), cleanID)
							continue
						}

						sdEligible[cleanID] = programme
					} else {
						haveAllInfo[channel.ID] = append(haveAllInfo[channel.ID], programme)
					}
				}
			}
		}

		if augmentWithSD {
			tmsIDs := make([]string, 0)

			for tmsID := range sdEligible {
				idType, uniqID, epID, _, _, extractErr := extractDDProgID(tmsID)
				if extractErr != nil {
					log.WithError(extractErr).Errorln("error extracting dd_progid")
					continue
				}
				cleanID := fmt.Sprintf("%s%s%s", idType, padNumberWithZero(uniqID, 8), padNumberWithZero(epID, 4))
				if len(cleanID) < 14 {
					log.Warnf("found an invalid TMS ID/dd_progid, expected length of exactly 14, got %d: %s\n", len(cleanID), cleanID)
					continue
				}
				tmsIDs = append(tmsIDs, cleanID)
			}

			log.Infof("Requesting guide data for %d programs from Schedules Direct", len(tmsIDs))

			allResponses := make([]schedulesdirect.ProgramInfo, 0)

			artworkMap := make(map[string][]schedulesdirect.ProgramArtwork)

			chunks := chunkStringSlice(tmsIDs, 5000)

			log.Infof("Making %d requests to Schedules Direct for program information, this might take a while", len(chunks))

			for _, chunk := range chunks {
				moreInfo, moreInfoErr := l.sd.GetProgramInfo(chunk)
				if moreInfoErr != nil {
					log.WithError(moreInfoErr).Errorln("Error when getting more program details from Schedules Direct")
					return epgChannelMap, epgProgrammeMap, moreInfoErr
				}

				log.Debugf("received %d responses for chunk", len(moreInfo))

				allResponses = append(allResponses, moreInfo...)
			}

			artworkTMSIDs := make([]string, 0)

			for _, entry := range allResponses {
				if entry.HasArtwork() {
					artworkTMSIDs = append(artworkTMSIDs, entry.ProgramID)
				}
			}

			chunks = chunkStringSlice(artworkTMSIDs, 500)

			log.Infof("Making %d requests to Schedules Direct for artwork, this might take a while", len(chunks))

			for _, chunk := range chunks {
				artwork, artworkErr := l.sd.GetArtworkForProgramIDs(chunk)
				if artworkErr != nil {
					log.WithError(artworkErr).Errorln("Error when getting program artwork from Schedules Direct")
					return epgChannelMap, epgProgrammeMap, artworkErr
				}

				for _, artworks := range artwork {
					if artworks.ProgramID == "" || artworks.Artwork == nil {
						continue
					}
					artworkMap[artworks.ProgramID] = append(artworkMap[artworks.ProgramID], *artworks.Artwork...)
				}
			}

			log.Debugf("Got %d responses from SD", len(allResponses))

			for _, sdResponse := range allResponses {
				programme := sdEligible[sdResponse.ProgramID]
				mergedProgramme := MergeSchedulesDirectAndXMLTVProgramme(&programme, sdResponse, artworkMap[sdResponse.ProgramID])
				haveAllInfo[mergedProgramme.Channel] = append(haveAllInfo[mergedProgramme.Channel], *mergedProgramme)
			}
		}

		for _, programmes := range haveAllInfo {
			for _, programme := range programmes {
				processedProgram := *provider.ProcessProgramme(programme)
				hasXMLTV := false
				itemType := ""
				for _, epNum := range processedProgram.EpisodeNums {
					if epNum.System == "dd_progid" {
						idType, _, _, _, _, extractErr := extractDDProgID(epNum.Value)
						if extractErr != nil {
							log.WithError(extractErr).Errorln("error extracting dd_progid")
							continue
						}
						itemType = idType
					}
					if epNum.System == "xmltv_ns" {
						hasXMLTV = true
					}
				}
				if (itemType == "SH" || itemType == "EP") && !hasXMLTV {
					t := time.Time(processedProgram.Date)
					if !t.IsZero() {
						processedProgram.EpisodeNums = append(processedProgram.EpisodeNums, xmltv.EpisodeNum{System: "original-air-date", Value: t.Format("2006-01-02 15:04:05")})
					}
				}
				epgProgrammeMap[programme.Channel] = append(epgProgrammeMap[programme.Channel], processedProgram)
			}
		}

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

		transport = "http"

		req, reqErr := http.NewRequest("GET", path, nil)
		if reqErr != nil {
			return nil, transport, reqErr
		}

		// For whatever reason, some providers only allow access from a "real" User-Agent.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/68.0.3440.106 Safari/537.36")

		resp, err := http.Get(path)
		if err != nil {
			return nil, transport, err
		}

		if strings.HasSuffix(strings.ToLower(path), ".gz") || resp.Header.Get("Content-Type") == "application/x-gzip" {
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

func MergeSchedulesDirectAndXMLTVProgramme(programme *xmltv.Programme, sdProgram schedulesdirect.ProgramInfo, artworks []schedulesdirect.ProgramArtwork) *xmltv.Programme {

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

	for _, keywords := range sdProgram.Keywords {
		for _, keyword := range keywords {
			allKeywords = append(allKeywords, keyword)
		}
	}

	for _, keyword := range UniqueStrings(allKeywords) {
		programme.Keywords = append(programme.Keywords, xmltv.CommonElement{Value: keyword})
	}

	// FIXME: We should really be making sure that we passthrough languages.
	allDescriptions := make([]string, 0)

	for _, description := range programme.Descriptions {
		allDescriptions = append(allDescriptions, description.Value)
	}

	for _, descriptions := range sdProgram.Descriptions {
		for _, description := range descriptions {
			if description.Description != "" {
				allDescriptions = append(allDescriptions, description.Description)
			}
			if description.Description != "" {
				allDescriptions = append(allDescriptions, description.Description)
			}
		}
	}

	for _, description := range UniqueStrings(allDescriptions) {
		programme.Descriptions = append(programme.Descriptions, xmltv.CommonElement{Value: description})
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

	for _, artwork := range artworks {
		programme.Icons = append(programme.Icons, xmltv.Icon{
			Source: getImageURL(artwork.URI),
			Width:  artwork.Width,
			Height: artwork.Height,
		})
	}

	hasXMLTVNS := false
	ddProgID := ""

	for _, epNum := range programme.EpisodeNums {
		if epNum.System == "xmltv_ns" {
			hasXMLTVNS = true
		} else if epNum.System == "dd_progid" {
			ddProgID = epNum.Value
		}
	}

	if !hasXMLTVNS {
		seasonNumber := 0
		episodeNumber := 0
		totalSeasons := 0
		totalEpisodes := 0
		numbersFilled := false

		for _, meta := range sdProgram.Metadata {
			for _, metadata := range meta {
				if metadata.Season > 0 {
					seasonNumber = metadata.Season - 1 // SD metadata isnt 0 index
					numbersFilled = true
				}
				if metadata.Episode > 0 {
					episodeNumber = metadata.Episode - 1
					numbersFilled = true
				}
				if metadata.TotalEpisodes > 0 {
					totalEpisodes = metadata.TotalEpisodes
					numbersFilled = true
				}
				if metadata.TotalSeasons > 0 {
					totalSeasons = metadata.TotalSeasons
					numbersFilled = true
				}
			}
		}

		if numbersFilled {
			seasonNumberStr := fmt.Sprintf("%d", seasonNumber)
			if totalSeasons > 0 {
				seasonNumberStr = fmt.Sprintf("%d/%d", seasonNumber, totalSeasons)
			}
			episodeNumberStr := fmt.Sprintf("%d", episodeNumber)
			if totalEpisodes > 0 {
				episodeNumberStr = fmt.Sprintf("%d/%d", episodeNumber, totalEpisodes)
			}

			partNumber := 0
			totalParts := 0

			if ddProgID != "" {
				var extractErr error
				_, _, _, partNumber, totalParts, extractErr = extractDDProgID(ddProgID)
				if extractErr != nil {
					panic(extractErr)
				}
			}

			partStr := "0"
			if partNumber > 0 {
				partStr = fmt.Sprintf("%d", partNumber)
				if totalParts > 0 {
					partStr = fmt.Sprintf("%d/%d", partNumber, totalParts)
				}
			}

			xmlTVNS := fmt.Sprintf("%s.%s.%s", seasonNumberStr, episodeNumberStr, partStr)
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

// extractDDProgID returns type, ID, episode ID, part number, total parts, error.
func extractDDProgID(progID string) (string, int, int, int, int, error) {
	matches := ddProgIDRegex.FindAllStringSubmatch(progID, -1)

	if len(matches) == 0 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid dd_progid: %s", progID)
	}

	itemType := matches[0][1]

	itemID, itemIDErr := strconv.Atoi(matches[0][2])
	if itemIDErr != nil {
		return itemType, 0, 0, 0, 0, itemIDErr
	}

	specificID, specificIDErr := strconv.Atoi(matches[0][3])
	if specificIDErr != nil {
		return itemType, itemID, 0, 0, 0, specificIDErr
	}

	currentPartNum := 0
	totalPartsNum := 0

	if len(matches[0]) > 2 && matches[0][4] != "" {
		currentPart, currentPartErr := strconv.Atoi(matches[0][4])
		if currentPartErr != nil {
			return itemType, itemID, specificID, 0, 0, currentPartErr
		}
		currentPartNum = currentPart
	}

	if len(matches[0]) > 3 && matches[0][5] != "" {
		totalParts, totalPartsErr := strconv.Atoi(matches[0][5])
		if totalPartsErr != nil {
			return itemType, itemID, specificID, currentPartNum, 0, totalPartsErr
		}
		totalPartsNum = totalParts
	}

	return itemType, itemID, specificID, currentPartNum, totalPartsNum, nil
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

func getImageURL(imageURI string) string {
	if strings.HasPrefix(imageURI, "https://s3.amazonaws.com") {
		return imageURI
	}
	return fmt.Sprint(schedulesdirect.DefaultBaseURL, schedulesdirect.APIVersion, "/image/", imageURI)
}

func padNumberWithZero(value int, expectedLength int) string {
	padded := fmt.Sprintf("%02d", value)
	valLength := countDigits(value)
	if valLength != expectedLength {
		return fmt.Sprintf("%s%d", strings.Repeat("0", expectedLength-valLength), value)
	}
	return padded
}

func countDigits(i int) int {
	count := 0
	if i == 0 {
		count = 1
	}
	for i != 0 {
		i /= 10
		count = count + 1
	}
	return count
}

func contains(s []string, e string) bool {
	for _, ss := range s {
		if e == ss {
			return true
		}
	}
	return false
}
