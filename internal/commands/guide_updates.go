package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/guideproviders"
	"github.com/tellytv/telly/internal/models"
)

var (
	log = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}
)

// FireGuideUpdatesCommand Command to fire one off guide source updates
func FireGuideUpdatesCommand() {
	cc, err := context.NewCContext()
	if err != nil {
		log.Fatalln("Couldn't create context", err)
	}

	provider, providerErr := cc.API.GuideSource.GetGuideSourceByID(1)
	if providerErr != nil {
		log.Fatalln("couldnt find guide source", providerErr)
	}

	if err = fireGuideUpdates(cc, provider); err != nil {
		log.Errorln("Could not complete guide updates " + err.Error())
	}
}

func fireGuideUpdates(cc *context.CContext, provider *models.GuideSource) error {

	log.Infoln("Guide source update is beginning")

	lineupMetadata, reloadErr := cc.GuideSourceProviders[provider.ID].Refresh(provider.ProviderData)
	if reloadErr != nil {
		return fmt.Errorf("error when refreshing for provider %s (%s): %s", provider.Name, provider.Provider, reloadErr)
	}

	if updateErr := cc.API.GuideSource.UpdateGuideSource(provider.ID, lineupMetadata); updateErr != nil {
		return fmt.Errorf("error when updating guide source provider metadata: %s", updateErr)
	}

	// TODO: Inspect the input metadata and output metadata and update channels as needed.

	guideChannels, guideChannelsErr := cc.API.LineupChannel.GetEnabledChannelsForGuideProvider(provider.ID)
	if guideChannelsErr != nil {
		return fmt.Errorf("error getting guide sources for lineup: %s", guideChannelsErr)
	}

	channelsToGet := make(map[string]guideproviders.Channel)

	for _, channel := range guideChannels {
		var pChannel guideproviders.Channel
		if marshalErr := json.Unmarshal(channel.GuideChannel.Data, &pChannel); marshalErr != nil {
			return fmt.Errorf("error when marshalling channel.data to guideproviders.channel: %s", marshalErr)
		}
		pChannel.ProviderData = channel.GuideChannel.ProviderData
		channelsToGet[channel.GuideChannel.XMLTVID] = pChannel
	}

	channelIDs := make([]string, 0)
	existingChannels := make([]guideproviders.Channel, 0)
	for channelID, channel := range channelsToGet {
		channelIDs = append(channelIDs, channelID)
		existingChannels = append(existingChannels, channel)
	}

	// Get all programmes in DB to pass into the Schedule function.
	existingProgrammes, existingProgrammesErr := cc.API.GuideSourceProgramme.GetProgrammesForActiveChannels()
	if existingProgrammesErr != nil {
		return fmt.Errorf("error getting all programmes in database: %s", existingProgrammesErr)
	}

	programmeContainers := make([]guideproviders.ProgrammeContainer, 0)
	for _, programme := range existingProgrammes {
		programmeContainers = append(programmeContainers, guideproviders.ProgrammeContainer{
			Programme:    *programme.XMLTV,
			ProviderData: programme.ProviderData,
		})
	}

	log.Infof("Beginning import of guide data from provider %d, getting %d channels: %s", provider.ID, len(channelsToGet), strings.Join(channelIDs, ", "))
	channelProviderData, newProgrammes, scheduleErr := cc.GuideSourceProviders[provider.ID].Schedule(14, existingChannels, programmeContainers)
	if scheduleErr != nil {
		return fmt.Errorf("error when updating schedule for provider %d: %s", provider.ID, scheduleErr)
	}

	for channelID, providerData := range channelProviderData {
		marshalledPD, marshalErr := json.Marshal(providerData)
		if marshalErr != nil {
			return fmt.Errorf("error when marshalling schedules direct channel data to json: %s", marshalErr)
		}
		log.Infof("Updating Channel ID: %s to %s", channelID, string(marshalledPD))
		if updateErr := cc.API.GuideSourceChannel.UpdateGuideSourceChannel(channelID, marshalledPD); updateErr != nil {
			return fmt.Errorf("error while updating provider specific data to guide source channel: %s", updateErr)
		}
	}

	for _, programme := range newProgrammes {
		_, programmeErr := cc.API.GuideSourceProgramme.InsertGuideSourceProgramme(provider.ID, programme.Programme, programme.ProviderData)
		if programmeErr != nil {
			return fmt.Errorf("error while inserting new programmes: %s", programmeErr)
		}
	}

	log.Infof("Completed import of %d programs", len(newProgrammes))

	return nil
}

// StartFireGuideUpdates Scheduler triggered function to update guide sources
func StartFireGuideUpdates(cc *context.CContext, provider *models.GuideSource) {
	err := fireGuideUpdates(cc, provider)
	if err != nil {
		log.Errorf("could not complete guide updates: %s", err)
	}

	log.Infoln("Guide source has been updated successfully")
}
