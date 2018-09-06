package guideproviders

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tellytv/go.schedulesdirect"
	"github.com/tellytv/telly/internal/utils"
	"github.com/tellytv/telly/internal/xmltv"
)

// SchedulesDirect is a GuideProvider supporting the Schedules Direct JSON service.
type SchedulesDirect struct {
	BaseConfig Configuration

	client   *schedulesdirect.Client
	channels []Channel
	stations map[string]sdStationContainer
}

func newSchedulesDirect(config *Configuration) (GuideProvider, error) {
	return &SchedulesDirect{BaseConfig: *config}, nil
}

// Name returns the name of the GuideProvider.
func (s *SchedulesDirect) Name() string {
	return "Schedules Direct"
}

// SupportsLineups returns true if the provider supports the concept of subscribing to lineups.
func (s *SchedulesDirect) SupportsLineups() bool {
	return true
}

// LineupCoverage returns a map of regions and countries the provider has support for.
func (s *SchedulesDirect) LineupCoverage() ([]CoverageArea, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	coverage, coverageErr := s.client.GetAvailableCountries()
	if coverageErr != nil {
		return nil, fmt.Errorf("error while getting coverage from provider %s: %s", s.Name(), coverageErr)
	}

	outputCoverage := make([]CoverageArea, 0)

	for region, countries := range coverage {
		for _, country := range countries {
			outputCoverage = append(outputCoverage, CoverageArea{
				RegionName:        region,
				FullName:          country.FullName,
				PostalCode:        country.PostalCode,
				PostalCodeExample: country.PostalCodeExample,
				ShortName:         country.ShortName,
				OnePostalCode:     country.OnePostalCode,
			})
		}
	}

	return outputCoverage, nil
}

// AvailableLineups will return a slice of AvailableLineup for the given countryCode and postalCode.
func (s *SchedulesDirect) AvailableLineups(countryCode, postalCode string) ([]AvailableLineup, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	headends, headendsErr := s.client.GetHeadends(countryCode, postalCode)
	if headendsErr != nil {
		return nil, fmt.Errorf("error while getting available lineups from provider %s: %s", s.Name(), headendsErr)
	}

	lineups := make([]AvailableLineup, 0)
	for _, headend := range headends {
		for _, lineup := range headend.Lineups {
			lineups = append(lineups, AvailableLineup{
				Location:   headend.Location,
				Transport:  headend.Transport,
				Name:       lineup.Name,
				ProviderID: lineup.Lineup,
			})
		}
	}

	return lineups, nil
}

// PreviewLineupChannels will return a slice of Channels for the given provider specific lineupID.
func (s *SchedulesDirect) PreviewLineupChannels(lineupID string) ([]Channel, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	channels, channelsErr := s.client.PreviewLineup(lineupID)
	if channelsErr != nil {
		return nil, fmt.Errorf("error while previewing channels in lineup from provider %s: %s", s.Name(), channelsErr)
	}

	outputChannels := make([]Channel, 0)

	for _, channel := range channels {
		outputChannels = append(outputChannels, Channel{
			Name:      channel.Name,
			Number:    channel.Channel,
			CallSign:  channel.CallSign,
			Affiliate: channel.Affiliate,
			Lineup:    lineupID,
		})
	}

	return outputChannels, nil
}

// SubscribeToLineup will subscribe the user to a lineup.
func (s *SchedulesDirect) SubscribeToLineup(lineupID string) (interface{}, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	newLineup, addLineupErr := s.client.AddLineup(lineupID)
	if addLineupErr != nil {
		return nil, fmt.Errorf("error while subscribing to lineup from provider %s: %s", s.Name(), addLineupErr)
	}
	return newLineup, nil
}

// UnsubscribeFromLineup will remove a lineup from the provider account.
func (s *SchedulesDirect) UnsubscribeFromLineup(lineupID string) error {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	_, deleteLineupErr := s.client.AddLineup(lineupID)
	if deleteLineupErr != nil {
		return fmt.Errorf("error while deleting lineup from provider %s: %s", s.Name(), deleteLineupErr)
	}
	return nil
}

// Channels returns a slice of Channel that the provider has available.
func (s *SchedulesDirect) Channels() ([]Channel, error) {
	return s.channels, nil
}

// Schedule returns a slice of xmltv.Programme for the given channelIDs.
func (s *SchedulesDirect) Schedule(daysToGet int, inputChannels []Channel, inputProgrammes []ProgrammeContainer) (map[string]interface{}, []ProgrammeContainer, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}
	// First, convert the slice of channelIDs into a slice of schedule requests.
	reqs := make([]schedulesdirect.StationScheduleRequest, 0)
	channelsCache := make(map[string]map[string]schedulesdirect.LastModifiedEntry)
	requestingDates := getDaysBetweenTimes(time.Now(), time.Now().AddDate(0, 0, daysToGet))
	channelShortToLongIDMap := make(map[string]string)
	for _, inputChannel := range inputChannels {
		splitID := strings.Split(inputChannel.ID, ".")[1]

		channelShortToLongIDMap[splitID] = inputChannel.ID

		if len(inputChannel.ProviderData.(json.RawMessage)) > 0 {
			channelCache := make(map[string]schedulesdirect.LastModifiedEntry)
			if unmarshalErr := json.Unmarshal(inputChannel.ProviderData.(json.RawMessage), &channelCache); unmarshalErr != nil {
				return nil, nil, unmarshalErr
			}

			if len(channelCache) > 0 {
				fmt.Printf("Channel %s exists in cache already with %d days of schedule available\n", inputChannel.ID, len(channelCache))
				channelsCache[splitID] = channelCache
			}
		}

		reqs = append(reqs, schedulesdirect.StationScheduleRequest{
			StationID: splitID,
			Dates:     requestingDates,
		})
	}

	// Next, we get all modified parts of the schedule for any channels.
	lastModifieds, lastModifiedsErr := s.client.GetLastModified(reqs)
	if lastModifiedsErr != nil {
		return nil, nil, fmt.Errorf("error getting lastModifieds from lastModifieds direct: %s", lastModifiedsErr)
	}

	channelsNeedingUpdate := make(map[string][]string)

	for stationID, dates := range lastModifieds {
		longStationID := channelShortToLongIDMap[stationID]
		if channelsNeedingUpdate[stationID] == nil {
			channelsNeedingUpdate[stationID] = make([]string, 0)
		}
		for date, lastMod := range dates {
			needsData := false
			if cachedDate, ok := channelsCache[stationID][date]; ok {
				fmt.Printf("For date %s: checking cached MD5 %s against server MD5 %s for %s\n", date, cachedDate.MD5, lastMod.MD5, longStationID)
				if cachedDate.MD5 != lastMod.MD5 {
					fmt.Printf("Station %s needs updated data for %s\n", longStationID, date)
					needsData = true
					channelsNeedingUpdate[stationID] = append(channelsNeedingUpdate[stationID], date)
				}
			} else {
				fmt.Printf("Station %s needs data for %s\n", longStationID, date)
				needsData = true
				channelsNeedingUpdate[stationID] = append(channelsNeedingUpdate[stationID], date)
			}
			if needsData {
				if channelsCache[stationID] == nil {
					channelsCache[stationID] = make(map[string]schedulesdirect.LastModifiedEntry)
				}
				channelsCache[stationID][date] = lastMod
			}
		}
		if _, ok := channelsCache[stationID]; !ok {
			fmt.Printf("Station %s needs initial data\n", longStationID)
			channelsNeedingUpdate[stationID] = requestingDates
			continue
		}
	}

	fullScheduleReqs := make([]schedulesdirect.StationScheduleRequest, 0)
	// Next, using the channelsNeedingUpdate, build new schedule requests for station(s) missing data for date(s).
	// Let's also add all these values to channelsCache to use that for the return.
	for stationID, dates := range channelsNeedingUpdate {
		if len(dates) > 0 {
			fmt.Printf("Requesting dates %s for station %s\n", strings.Join(dates, ", "), stationID)
			fullScheduleReqs = append(fullScheduleReqs, schedulesdirect.StationScheduleRequest{
				StationID: stationID,
				Dates:     dates,
			})
		}
	}

	outputChannelsMap := make(map[string]interface{})
	for shortChannelID, longChannelID := range channelShortToLongIDMap {
		outputChannelsMap[longChannelID] = channelsCache[shortChannelID]
	}

	if reflect.DeepEqual(outputChannelsMap, channelsCache) {
		outputChannelsMap = nil
	}

	// Great, we don't need to get any new schedule data, let's terminate early.
	if len(fullScheduleReqs) == 0 {
		fmt.Println("No updates required, exiting Schedule()")
		return outputChannelsMap, nil, nil
	}

	// So we do have some requests to make, let's do that now.
	schedules, schedulesErr := s.client.GetSchedules(fullScheduleReqs)
	if schedulesErr != nil {
		return nil, nil, fmt.Errorf("error getting schedules from schedules direct: %s", schedulesErr)
	}

	// Next, we need to bundle up all the program IDs and request detailed information about them.
	neededProgramIDs := make(map[string]struct{})

	for _, schedule := range schedules {
		for _, program := range schedule.Programs {
			neededProgramIDs[program.ProgramID] = struct{}{}
		}
	}

	extendedProgramInfo := make(map[string]schedulesdirect.ProgramInfo)

	programsWithArtwork := make(map[string]struct{})

	// IDs slice is built, let's chunk and get the info.
	for _, chunk := range utils.ChunkStringSlice(utils.GetStringMapKeys(neededProgramIDs), 5000) {
		moreInfo, moreInfoErr := s.client.GetProgramInfo(chunk)
		if moreInfoErr != nil {
			return nil, nil, fmt.Errorf("error when getting more program details from schedules direct: %s", moreInfoErr)
		}

		for _, program := range moreInfo {
			extendedProgramInfo[program.ProgramID] = program
			if program.HasArtwork() {
				for _, programID := range program.ArtworkLookupIDs() {
					programsWithArtwork[programID] = struct{}{}
				}
			}
		}
	}

	allArtwork := make(map[string][]schedulesdirect.Artwork)

	// Now that we have the initial program info results, let's get all the artwork.
	artworkResp, artworkErr := s.client.GetArtworkForProgramIDs(utils.GetStringMapKeys(programsWithArtwork))
	if artworkErr != nil {
		return nil, nil, fmt.Errorf("error when getting artwork from schedules direct: %s", artworkErr)
	}

	for _, artworks := range artworkResp {
		allArtwork[artworks.ProgramID] = *artworks.Artwork
	}

	// We finally have all the data, time to convert to the XMLTV format.
	programmes := make([]ProgrammeContainer, 0)

	// Iterate over every result, converting to XMLTV format.
	for _, schedule := range schedules {
		station := s.stations[schedule.StationID]
		for _, airing := range schedule.Programs {
			programInfo := extendedProgramInfo[airing.ProgramID]
			artworks := make([]schedulesdirect.Artwork, 0)
			for _, lookupKey := range programInfo.ArtworkLookupIDs() {
				if hasArtwork, ok := allArtwork[lookupKey]; ok {
					artworks = append(artworks, hasArtwork...)
				}
			}

			sort.Slice(artworks, func(i, j int) bool {
				tier := func(a schedulesdirect.Artwork) int {
					return int(parseArtworkTierToOrder(a.Tier))
				}
				category := func(a schedulesdirect.Artwork) int {
					return int(parseArtworkCategoryToOrder(a.Category))
				}
				a := tier(artworks[i])
				b := tier(artworks[i])
				if a == b {
					return category(artworks[i]) < category(artworks[j])
				}
				return a < b
			})

			programme, programmeErr := s.processProgrammeToXMLTV(airing, extendedProgramInfo[airing.ProgramID], artworks, station)
			if programmeErr != nil {
				return nil, nil, fmt.Errorf("error while processing schedules direct result to xmltv format: %s", programmeErr)
			}
			programmes = append(programmes, *programme)
		}
	}

	return outputChannelsMap, programmes, nil
}

// Refresh causes the provider to request the latest information.
func (s *SchedulesDirect) Refresh(lastStatusJSON []byte) ([]byte, error) {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return nil, fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	lineupsMetadataMap := make(map[string]schedulesdirect.Lineup)
	var lastStatus schedulesdirect.StatusResponse
	if len(lastStatusJSON) > 0 {
		if unmarshalErr := json.Unmarshal(lastStatusJSON, &lastStatus); unmarshalErr != nil {
			return nil, fmt.Errorf("error unmarshalling cached status JSON: %s", unmarshalErr)
		}

		for _, lineup := range lastStatus.Lineups {
			lineupsMetadataMap[lineup.Lineup] = lineup
		}
	}

	// First, get the lineups added to the users account.
	// SD API docs say to check system status before proceeding.
	// NewClient above does that automatically for us.
	status, statusErr := s.client.GetStatus()
	if statusErr != nil {
		return nil, fmt.Errorf("error getting schedules direct status: %s", statusErr)
	}

	marshalledLineups, marshalledLineupsErr := json.Marshal(status)
	if marshalledLineupsErr != nil {
		return nil, fmt.Errorf("error when marshalling schedules direct lineups to json: %s", marshalledLineupsErr)
	}

	// If there's anything in this slice we know that channels in the SD lineup are changing.
	allLineups := make([]string, 0)

	for _, lineup := range status.Lineups {
		// if existingLineup, ok := lineupsMetadataMap[lineup.Lineup]; ok {
		// 	// If lineup modified in database is not equal to lineup modified API provided
		// 	// append lineup ID to allLineups
		// 	if !existingLineup.Modified.Equal(lineup.Modified) {
		// 		allLineups = append(allLineups, lineup.Lineup)
		// 	}
		// }
		allLineups = append(allLineups, lineup.Lineup)
	}

	// Figure out if we need to add any lineups to the account.
	neededLineups := make([]string, 0)

	for _, wantedLineup := range s.BaseConfig.Lineups {
		needLineup := true
		for _, previouslyAddedLineup := range allLineups {
			if previouslyAddedLineup == wantedLineup {
				needLineup = false
				allLineups = append(allLineups, previouslyAddedLineup)
			}
		}
		if needLineup {
			neededLineups = append(neededLineups, wantedLineup)
		}
	}

	// Sanity check
	if len(status.Lineups) == status.Account.MaxLineups && len(neededLineups) > 0 {
		return marshalledLineups, fmt.Errorf("attempting to add more than %d lineups to a schedules direct account will fail, exiting prematurely", status.Account.MaxLineups)
	}

	// Add needed lineups
	for _, neededLineupName := range neededLineups {
		if _, err := s.client.AddLineup(neededLineupName); err != nil {
			return marshalledLineups, fmt.Errorf("error when adding lineup %s to schedules direct account: %s", neededLineupName, err)
		}
		allLineups = append(allLineups, neededLineupName)
	}

	// Next, let's fill in the available channels in all the lineups.
	for _, lineupName := range allLineups {
		channels, channelsErr := s.client.GetChannels(lineupName, true)
		if channelsErr != nil {
			return marshalledLineups, fmt.Errorf("error getting channels from schedules direct for lineup %s: %s", lineupName, channelsErr)
		}

		stationsMap := make(map[string]sdStationContainer)

		for _, stn := range channels.Stations {
			stationsMap[stn.StationID] = sdStationContainer{Station: stn}
		}

		for _, entry := range channels.Map {
			if val, ok := stationsMap[entry.StationID]; ok {
				val.ChannelMap = entry
				stationsMap[entry.StationID] = val
			}
		}

		s.stations = make(map[string]sdStationContainer)

		if s.channels == nil {
			s.channels = make([]Channel, 0)
		}

		for _, station := range stationsMap {
			logos := make([]Logo, 0)

			for _, stnLogo := range station.Station.Logos {
				logos = append(logos, Logo{
					URL:    stnLogo.URL,
					Height: stnLogo.Height,
					Width:  stnLogo.Width,
				})
			}

			s.channels = append(s.channels, Channel{
				ID:       fmt.Sprintf("I%s.%s.schedulesdirect.org", station.ChannelMap.Channel, station.Station.StationID),
				Name:     station.Station.Name,
				Logos:    logos,
				Number:   station.ChannelMap.Channel,
				CallSign: station.Station.CallSign,
				Lineup:   lineupName,
			})

			s.stations[station.Station.StationID] = station
		}
	}

	// We're done!

	return marshalledLineups, nil
}

// Configuration returns the base configuration backing the provider.
func (s *SchedulesDirect) Configuration() Configuration {
	return s.BaseConfig
}

type sdStationContainer struct {
	Station    schedulesdirect.Station
	ChannelMap schedulesdirect.ChannelMap
}

func getXMLTVNumber(mdata []map[string]schedulesdirect.Metadata, multipartInfo *schedulesdirect.Part) string {
	seasonNumber := 0
	episodeNumber := 0
	totalSeasons := 0
	totalEpisodes := 0
	numbersFilled := false

	for _, meta := range mdata {
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

		partStr := "0"

		partNumber := 0
		totalParts := 0

		if multipartInfo != nil {
			partNumber = multipartInfo.PartNumber
			totalParts = multipartInfo.TotalParts
		}

		if partNumber > 0 {
			partStr = fmt.Sprintf("%d", partNumber)
			if totalParts > 0 {
				partStr = fmt.Sprintf("%d/%d", partNumber, totalParts)
			}
		}

		return fmt.Sprintf("%s.%s.%s", seasonNumberStr, episodeNumberStr, partStr)
	}

	return ""
}

type sdProgrammeData struct {
	Airing      schedulesdirect.Program
	ProgramInfo schedulesdirect.ProgramInfo
	AllArtwork  []schedulesdirect.Artwork
	Station     sdStationContainer
}

func (s *SchedulesDirect) processProgrammeToXMLTV(airing schedulesdirect.Program, programInfo schedulesdirect.ProgramInfo, allArtwork []schedulesdirect.Artwork, station sdStationContainer) (*ProgrammeContainer, error) {
	stationID := fmt.Sprintf("I%s.%s.schedulesdirect.org", station.ChannelMap.Channel, station.Station.StationID)
	endTime := airing.AirDateTime.Add(time.Duration(airing.Duration) * time.Second)
	length := xmltv.Length{Units: "seconds", Value: strconv.Itoa(airing.Duration)}

	// First we fill in all the "simple" fields that don't require any extra processing.
	xmlProgramme := xmltv.Programme{
		Channel: stationID,
		ID:      airing.ProgramID,
		Length:  &length,
		Start:   &xmltv.Time{Time: *airing.AirDateTime},
		Stop:    &xmltv.Time{Time: endTime},
	}

	// Now for the fields that have to be parsed.
	for _, broadcastLang := range station.Station.BroadcastLanguage {
		xmlProgramme.Languages = []xmltv.CommonElement{{
			Value: broadcastLang,
			Lang:  broadcastLang,
		}}
	}

	xmlProgramme.Titles = make([]xmltv.CommonElement, 0)
	for _, sdTitle := range programInfo.Titles {
		xmlProgramme.Titles = append(xmlProgramme.Titles, xmltv.CommonElement{
			Value: sdTitle.Title120,
		})
	}

	if programInfo.EpisodeTitle150 != "" {
		xmlProgramme.SecondaryTitles = []xmltv.CommonElement{{
			Value: programInfo.EpisodeTitle150,
		}}
	}

	xmlProgramme.Descriptions = make([]xmltv.CommonElement, 0)
	if d1000, ok := programInfo.Descriptions["description1000"]; ok && len(d1000) > 0 {
		// TODO: This doesn't account for if the program has descriptions in different languages.
		// It will always just use the first description.
		xmlProgramme.Descriptions = append(xmlProgramme.Descriptions, xmltv.CommonElement{
			Value: d1000[0].Description,
			Lang:  d1000[0].Language,
		})
	}

	if d100, ok := programInfo.Descriptions["description100"]; ok && len(d100) > 0 {
		xmlProgramme.Descriptions = append(xmlProgramme.Descriptions, xmltv.CommonElement{
			Value: d100[0].Description,
			Lang:  d100[0].Language,
		})
	}

	for _, sdCast := range append(programInfo.Cast, programInfo.Crew...) {
		if xmlProgramme.Credits == nil {
			xmlProgramme.Credits = &xmltv.Credits{}
		}
		lowerRole := strings.ToLower(sdCast.Role)
		if strings.Contains(lowerRole, "director") {
			xmlProgramme.Credits.Directors = append(xmlProgramme.Credits.Directors, sdCast.Name)
		} else if strings.Contains(lowerRole, "actor") || strings.Contains(lowerRole, "voice") {
			role := ""
			if sdCast.Role != "Actor" {
				role = sdCast.Role
			}
			xmlProgramme.Credits.Actors = append(xmlProgramme.Credits.Actors, xmltv.Actor{
				Role:  role,
				Value: sdCast.Name,
			})
		} else if strings.Contains(lowerRole, "writer") {
			xmlProgramme.Credits.Writers = append(xmlProgramme.Credits.Writers, sdCast.Name)
		} else if strings.Contains(lowerRole, "producer") {
			xmlProgramme.Credits.Producers = append(xmlProgramme.Credits.Producers, sdCast.Name)
		} else if strings.Contains(lowerRole, "host") || strings.Contains(lowerRole, "anchor") {
			xmlProgramme.Credits.Presenters = append(xmlProgramme.Credits.Presenters, sdCast.Name)
		} else if strings.Contains(lowerRole, "guest") || strings.Contains(lowerRole, "contestant") {
			xmlProgramme.Credits.Guests = append(xmlProgramme.Credits.Guests, sdCast.Name)
		}
	}

	if programInfo.Movie != nil && programInfo.Movie.Year != nil && !programInfo.Movie.Year.Time.IsZero() {
		xmlProgramme.Date = xmltv.Date(*programInfo.Movie.Year.Time)
	}

	xmlProgramme.Categories = make([]xmltv.CommonElement, 0)
	seenCategories := make(map[string]struct{})
	for _, sdCategory := range programInfo.Genres {
		if _, ok := seenCategories[sdCategory]; !ok {
			xmlProgramme.Categories = append(xmlProgramme.Categories, xmltv.CommonElement{
				Value: sdCategory,
			})
			seenCategories[sdCategory] = struct{}{}
		}
	}

	entityTypeCat := string(programInfo.EntityType)

	if programInfo.EntityType == "episode" {
		entityTypeCat = "series"
	}

	if _, ok := seenCategories[entityTypeCat]; !ok {
		xmlProgramme.Categories = append(xmlProgramme.Categories, xmltv.CommonElement{
			Value: entityTypeCat,
		})
	}

	seenKeywords := make(map[string]struct{})
	for _, keywords := range programInfo.Keywords {
		for _, keyword := range keywords {
			if _, ok := seenKeywords[keyword]; !ok {
				xmlProgramme.Keywords = append(xmlProgramme.Keywords, xmltv.CommonElement{
					Value: utils.KebabCase(keyword),
				})
				seenKeywords[keyword] = struct{}{}
			}
		}
	}

	if programInfo.OfficialURL != "" {
		xmlProgramme.URLs = []string{programInfo.OfficialURL}
	}

	for _, artworkItem := range allArtwork {
		if strings.HasPrefix(artworkItem.URI, "assets/") {
			artworkItem.URI = fmt.Sprint(schedulesdirect.DefaultBaseURL, schedulesdirect.APIVersion, "/image/", artworkItem.URI)
		}
		xmlProgramme.Icons = append(xmlProgramme.Icons, xmltv.Icon{
			Source: artworkItem.URI,
			Width:  artworkItem.Width,
			Height: artworkItem.Height,
		})
	}

	xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{
		System: "dd_progid",
		Value:  programInfo.ProgramID,
	})

	xmltvns := getXMLTVNumber(programInfo.Metadata, airing.ProgramPart)
	if xmltvns != "" {
		xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{System: "xmltv_ns", Value: xmltvns})
	}

	sxxexx := ""

	for _, metadata := range programInfo.Metadata {
		for _, mdProvider := range metadata {
			if mdProvider.Season > 0 && mdProvider.Episode > 0 {
				sxxexx = fmt.Sprintf("S%sE%s", utils.PadNumberWithZeros(mdProvider.Season, 2), utils.PadNumberWithZeros(mdProvider.Episode, 2))
			}
		}
	}

	if sxxexx != "" {
		xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{System: "SxxExx", Value: sxxexx})
	}

	for _, videoProperty := range airing.VideoProperties {
		if xmlProgramme.Video == nil {
			xmlProgramme.Video = &xmltv.Video{}
		}
		if station.Station.IsRadioStation {
			continue
		}
		xmlProgramme.Video.Present = "yes"
		if strings.ToLower(videoProperty) == "hdtv" {
			xmlProgramme.Video.Quality = "HDTV"
			xmlProgramme.Video.Aspect = "16:9"
		} else if strings.ToLower(videoProperty) == "uhdtv" {
			xmlProgramme.Video.Quality = "UHD"
		} else if strings.ToLower(videoProperty) == "sdtv" {
			xmlProgramme.Video.Aspect = "4:3"
		}
	}

	for _, audioProperty := range airing.AudioProperties {
		switch strings.ToLower(audioProperty) {
		case "dd":
			xmlProgramme.Audio = &xmltv.Audio{Stereo: "dolby digital"}
		case "dd 5.1", "surround", "atmos":
			xmlProgramme.Audio = &xmltv.Audio{Stereo: "surround"}
		case "dolby":
			xmlProgramme.Audio = &xmltv.Audio{Stereo: "dolby"}
		case "stereo":
			xmlProgramme.Audio = &xmltv.Audio{Stereo: "stereo"}
		case "mono":
			xmlProgramme.Audio = &xmltv.Audio{Stereo: "mono"}
		case "cc", "subtitled":
			xmlProgramme.Subtitles = append(xmlProgramme.Subtitles, xmltv.Subtitle{Type: "teletext"})
		}
	}

	if airing.Signed {
		xmlProgramme.Subtitles = append(xmlProgramme.Subtitles, xmltv.Subtitle{Type: "deaf-signed"})
	}

	if programInfo.OriginalAirDate != nil && !programInfo.OriginalAirDate.Time.IsZero() {
		if !airing.New {
			xmlProgramme.PreviouslyShown = &xmltv.PreviouslyShown{
				Start: xmltv.Time{Time: *programInfo.OriginalAirDate.Time},
			}
		}

		timeToUse := programInfo.OriginalAirDate.Time
		if airing.New {
			timeToUse = airing.AirDateTime
		}

		xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{
			System: "original-air-date",
			Value:  timeToUse.Format("2006-01-02 15:04:05"),
		})
	}

	if airing.Repeat && xmlProgramme.PreviouslyShown != nil {
		xmlProgramme.PreviouslyShown = nil
	}

	seenRatings := make(map[string]string)
	for _, rating := range append(programInfo.ContentRating, airing.Ratings...) {
		if _, ok := seenRatings[rating.Body]; !ok {
			xmlProgramme.Ratings = append(xmlProgramme.Ratings, xmltv.Rating{
				Value:  rating.Code,
				System: rating.Body,
			})
			seenRatings[rating.Body] = rating.Code
		}
	}

	if programInfo.Movie != nil {
		for _, starRating := range programInfo.Movie.QualityRating {
			xmlProgramme.StarRatings = append(xmlProgramme.StarRatings, xmltv.Rating{
				Value:  fmt.Sprintf("%s/%s", starRating.Rating, starRating.MaxRating),
				System: starRating.RatingsBody,
			})
		}
	}

	if airing.IsPremiereOrFinale != nil && *airing.IsPremiereOrFinale != "" {
		xmlProgramme.Premiere = &xmltv.CommonElement{
			Lang:  "en",
			Value: string(*airing.IsPremiereOrFinale),
		}
	}

	if airing.Premiere {
		xmlProgramme.Premiere = &xmltv.CommonElement{}
	}

	if airing.New {
		elm := xmltv.ElementPresent(true)
		xmlProgramme.New = &elm
	}

	// Done processing!
	return &ProgrammeContainer{
		Programme: xmlProgramme,
		ProviderData: sdProgrammeData{
			airing,
			programInfo,
			allArtwork,
			station,
		},
	}, nil

}

func getDaysBetweenTimes(start, end time.Time) []string {
	dates := make([]string, 0)
	for last := start; last.Before(end); last = last.AddDate(0, 0, 1) {
		dates = append(dates, last.Format("2006-01-02"))
	}
	return dates
}

type artworkTierOrder int

const (
	episodeTier artworkTierOrder = 1
	seasonTier  artworkTierOrder = 2
	seriesTier  artworkTierOrder = 3

	dontCareTier artworkTierOrder = 10
)

func parseArtworkTierToOrder(tier schedulesdirect.ArtworkTier) artworkTierOrder {
	switch tier {
	case schedulesdirect.EpisodeTier:
		return episodeTier
	case schedulesdirect.SeasonTier:
		return seasonTier
	case schedulesdirect.SeriesTier:
		return seriesTier
	default:
		return dontCareTier
	}
}

type artworkCategoryOrder int

const (
	bannerL1  artworkCategoryOrder = 1
	bannerL1T artworkCategoryOrder = 2
	banner    artworkCategoryOrder = 3
	bannerL2  artworkCategoryOrder = 4
	bannerL3  artworkCategoryOrder = 5
	bannerLO  artworkCategoryOrder = 6
	bannerLOT artworkCategoryOrder = 7

	dontCareCategory artworkCategoryOrder = 10
)

func parseArtworkCategoryToOrder(Category schedulesdirect.ArtworkCategory) artworkCategoryOrder {
	switch Category {
	case schedulesdirect.BannerL1:
		return bannerL1
	case schedulesdirect.BannerL1T:
		return bannerL1T
	case schedulesdirect.Banner:
		return banner
	case schedulesdirect.BannerL2:
		return bannerL2
	case schedulesdirect.BannerL3:
		return bannerL3
	case schedulesdirect.BannerLO:
		return bannerLO
	case schedulesdirect.BannerLOT:
		return bannerLOT
	}

	return dontCareCategory
}
