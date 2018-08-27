package guide_providers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tellytv/go.schedulesdirect"
	"github.com/tellytv/telly/internal/xmltv"
	"github.com/tellytv/telly/utils"
)

type SchedulesDirect struct {
	BaseConfig Configuration

	client   *schedulesdirect.Client
	channels []Channel
	stations map[string]sdStationContainer
}

func newSchedulesDirect(config *Configuration) (GuideProvider, error) {
	provider := &SchedulesDirect{BaseConfig: *config}

	if loadErr := provider.Refresh(); loadErr != nil {
		return nil, fmt.Errorf("error when refreshing provider data: %s", loadErr)
	}

	return provider, nil
}

func (s *SchedulesDirect) Name() string {
	return "Schedules Direct"
}

func (s *SchedulesDirect) Channels() ([]Channel, error) {
	return s.channels, nil
}

func (s *SchedulesDirect) Schedule(channelIDs []string) ([]xmltv.Programme, error) {
	// First, convert the string slice of channelIDs into a slice of schedule requests.
	reqs := make([]schedulesdirect.StationScheduleRequest, 0)
	for _, channelID := range channelIDs {
		splitID := strings.Split(channelID, ".")[1]
		reqs = append(reqs, schedulesdirect.StationScheduleRequest{
			StationID: splitID,
			Dates:     []string{time.Now().Format("2006-01-02"), time.Now().AddDate(0, 0, 7).Format("2006-01-02")},
		})
	}

	// Next, get the results
	schedules, schedulesErr := s.client.GetSchedules(reqs)
	if schedulesErr != nil {
		return nil, fmt.Errorf("error getting schedules from schedules direct: %s", schedulesErr)
	}

	// Then, we need to bundle up all the program IDs and request detailed information about them.
	neededProgramIDs := make(map[string]struct{}, 0)

	for _, schedule := range schedules {
		for _, program := range schedule.Programs {
			neededProgramIDs[program.ProgramID] = struct{}{}
		}
	}

	extendedProgramInfo := make(map[string]sdProgramContainer, 0)

	programsWithArtwork := make(map[string]struct{}, 0)

	// IDs slice is built, let's chunk and get the info.
	for _, chunk := range utils.ChunkStringSlice(utils.GetStringMapKeys(neededProgramIDs), 5000) {
		moreInfo, moreInfoErr := s.client.GetProgramInfo(chunk)
		if moreInfoErr != nil {
			return nil, fmt.Errorf("error when getting more program details from schedules direct: %s", moreInfoErr)
		}

		for _, program := range moreInfo {
			extendedProgramInfo[program.ProgramID] = sdProgramContainer{
				Info: program,
			}
			if program.HasArtwork() {
				programsWithArtwork[program.ProgramID] = struct{}{}
			}
		}
	}

	allArtwork := make(map[string][]schedulesdirect.ProgramArtwork, 0)

	// Now that we have the initial program info results, let's get all the artwork.
	for _, chunk := range utils.ChunkStringSlice(utils.GetStringMapKeys(programsWithArtwork), 500) {
		artworkResp, artworkErr := s.client.GetArtworkForProgramIDs(chunk)
		if artworkErr != nil {
			return nil, fmt.Errorf("error when getting artwork from schedules direct: %s", artworkErr)
		}

		for _, artworks := range artworkResp {
			allArtwork[artworks.ProgramID] = *artworks.Artwork
		}
	}

	// We finally have all the data, time to convert to the XMLTV format.
	programmes := make([]xmltv.Programme, 0)

	// Iterate over every result, converting to XMLTV format.
	for _, schedule := range schedules {
		station := s.stations[schedule.StationID]

		for _, airing := range schedule.Programs {
			programInfo := extendedProgramInfo[airing.ProgramID]
			endTime := airing.AirDateTime.Add(time.Duration(airing.Duration) * time.Second)
			length := xmltv.Length{Units: "seconds", Value: strconv.Itoa(airing.Duration)}

			// First we fill in all the "simple" fields that don't require any extra processing.
			xmlProgramme := xmltv.Programme{
				Channel: fmt.Sprintf("I%s.%s.schedulesdirect.org", station.ChannelMap.Channel, station.Station.StationID),
				ID:      airing.ProgramID,
				Languages: []xmltv.CommonElement{xmltv.CommonElement{
					Value: station.Station.BroadcastLanguage[0],
					Lang:  station.Station.BroadcastLanguage[0],
				}},
				Length: &length,
				Start:  &xmltv.Time{airing.AirDateTime},
				Stop:   &xmltv.Time{endTime},
			}

			// Now for the fields that have to be parsed.
			xmlProgramme.Titles = make([]xmltv.CommonElement, 0)
			for _, sdTitle := range programInfo.Info.Titles {
				xmlProgramme.Titles = append(xmlProgramme.Titles, xmltv.CommonElement{
					Value: sdTitle.Title120,
				})
			}

			if programInfo.Info.EpisodeTitle150 != "" {
				xmlProgramme.SecondaryTitles = []xmltv.CommonElement{xmltv.CommonElement{
					Value: programInfo.Info.EpisodeTitle150,
				}}
			}

			xmlProgramme.Descriptions = make([]xmltv.CommonElement, 0)
			for _, sdDescription := range programInfo.Info.GetOrderedDescriptions() {
				xmlProgramme.Descriptions = append(xmlProgramme.Descriptions, xmltv.CommonElement{
					Value: sdDescription.Description,
					Lang:  sdDescription.Language,
				})
			}

			for _, sdCast := range append(programInfo.Info.Cast, programInfo.Info.Crew...) {
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

			if programInfo.Info.Movie.Year != "" {
				yearInt, yearIntErr := strconv.Atoi(programInfo.Info.Movie.Year)
				if yearIntErr == nil { // Date isn't that important of a field, if we hit an error while parsing just don't add date.
					xmlProgramme.Date = xmltv.Date(time.Date(yearInt, 1, 1, 1, 1, 1, 1, time.UTC))
				}
			}

			xmlProgramme.Categories = make([]xmltv.CommonElement, 0)
			seenCategories := make(map[string]struct{})
			for _, sdCategory := range programInfo.Info.Genres {
				if _, ok := seenCategories[sdCategory]; !ok {
					xmlProgramme.Categories = append(xmlProgramme.Categories, xmltv.CommonElement{
						Value: sdCategory,
					})
					seenCategories[sdCategory] = struct{}{}
				}
			}

			entityTypeCat := programInfo.Info.EntityType

			if programInfo.Info.EntityType == "episode" {
				entityTypeCat = "series"
			}

			if _, ok := seenCategories[entityTypeCat]; !ok {
				xmlProgramme.Categories = append(xmlProgramme.Categories, xmltv.CommonElement{
					Value: entityTypeCat,
				})
			}

			seenKeywords := make(map[string]struct{})
			for _, keywords := range programInfo.Info.Keywords {
				for _, keyword := range keywords {
					if _, ok := seenKeywords[keyword]; !ok {
						xmlProgramme.Keywords = append(xmlProgramme.Keywords, xmltv.CommonElement{
							Value: utils.KebabCase(keyword),
						})
						seenKeywords[keyword] = struct{}{}
					}
				}
			}

			if programInfo.Info.OfficialURL != "" {
				xmlProgramme.URLs = []string{programInfo.Info.OfficialURL}
			}

			if artworks, ok := allArtwork[programInfo.Info.ProgramID[:10]]; ok {
				for _, artworkItem := range artworks {
					if strings.HasPrefix(artworkItem.URI, "assets/") {
						artworkItem.URI = fmt.Sprint(schedulesdirect.DefaultBaseURL, schedulesdirect.APIVersion, "/image/", artworkItem.URI)
					}
					xmlProgramme.Icons = append(xmlProgramme.Icons, xmltv.Icon{
						Source: artworkItem.URI,
						Width:  artworkItem.Width,
						Height: artworkItem.Height,
					})
				}
			}

			xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{
				System: "dd_progid",
				Value:  programInfo.Info.ProgramID,
			})

			xmltvns := getXMLTVNumber(programInfo.Info.Metadata, airing.ProgramPart)
			if xmltvns != "" {
				xmlProgramme.EpisodeNums = append(xmlProgramme.EpisodeNums, xmltv.EpisodeNum{System: "xmltv_ns", Value: xmltvns})
			}

			sxxexx := ""

			for _, metadata := range programInfo.Info.Metadata {
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

			if !time.Time(programInfo.Info.OriginalAirDate).IsZero() {
				if !airing.New {
					xmlProgramme.PreviouslyShown = &xmltv.PreviouslyShown{
						Start: xmltv.Time{time.Time(programInfo.Info.OriginalAirDate)},
					}
				}
				timeToUse := time.Time(programInfo.Info.OriginalAirDate)
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
			for _, rating := range append(programInfo.Info.ContentRating, airing.Ratings...) {
				if _, ok := seenRatings[rating.Body]; !ok {
					xmlProgramme.Ratings = append(xmlProgramme.Ratings, xmltv.Rating{
						Value:  rating.Code,
						System: rating.Body,
					})
					seenRatings[rating.Body] = rating.Code
				}
			}

			for _, starRating := range programInfo.Info.Movie.QualityRating {
				xmlProgramme.Ratings = append(xmlProgramme.Ratings, xmltv.Rating{
					Value:  fmt.Sprintf("%s/%s", starRating.Rating, starRating.MaxRating),
					System: starRating.RatingsBody,
				})
			}

			if airing.IsPremiereOrFinale != "" {
				xmlProgramme.Premiere = &xmltv.CommonElement{
					Lang:  "en",
					Value: string(airing.IsPremiereOrFinale),
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
			programmes = append(programmes, xmlProgramme)

		}
	}

	return programmes, nil
}

func (s *SchedulesDirect) Refresh() error {
	if s.client == nil {
		sdClient, sdClientErr := schedulesdirect.NewClient(s.BaseConfig.Username, s.BaseConfig.Password)
		if sdClientErr != nil {
			return fmt.Errorf("error setting up schedules direct client: %s", sdClientErr)
		}

		s.client = sdClient
	}

	// First, get the lineups added to the users account.
	// SD API docs say to check system status before proceeding.
	// NewClient above does that automatically for us.
	status, statusErr := s.client.GetStatus()
	if statusErr != nil {
		return fmt.Errorf("error getting schedules direct status: %s", statusErr)
	}

	allLineups := make([]string, 0)

	for _, lineup := range status.Lineups {
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
		return fmt.Errorf("attempting to add more than %d lineups to a schedules direct account will fail, exiting prematurely", status.Account.MaxLineups)
	}

	// Add needed lineups
	for _, neededLineupName := range neededLineups {
		if _, err := s.client.AddLineup(neededLineupName); err != nil {
			return fmt.Errorf("error when adding lineup %s to schedules direct account: %s", neededLineupName, err)
		}
		allLineups = append(allLineups, neededLineupName)
	}

	// Next, let's fill in the available channels in all the lineups.
	for _, lineupName := range allLineups {
		channels, channelsErr := s.client.GetChannels(lineupName, true)
		if channelsErr != nil {
			return fmt.Errorf("error getting channels from schedules direct for lineup %s: %s", lineupName, channelsErr)
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

	return nil
}

func (s *SchedulesDirect) Configuration() Configuration {
	return s.BaseConfig
}

type sdStationContainer struct {
	Station    schedulesdirect.Station
	ChannelMap schedulesdirect.ChannelMap
}

type sdProgramContainer struct {
	Info    schedulesdirect.ProgramInfo
	Artwork []schedulesdirect.ProgramArtwork
}

func getXMLTVNumber(mdata []map[string]schedulesdirect.Metadata, multipartInfo schedulesdirect.Part) string {
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

		partNumber := multipartInfo.PartNumber
		totalParts := multipartInfo.TotalParts

		partStr := "0"
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
