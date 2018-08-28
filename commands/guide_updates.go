package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/utils"
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
	// FIXME: Don't hardcode this
	if err = fireGuideUpdates(cc, 1); err != nil {
		log.Errorln("Could not complete guide updates " + err.Error())
	}
}

func fireGuideUpdates(cc *context.CContext, providerID int) error {

	log.Infoln("Guide source update is beginning")

	guideChannels, guideChannelsErr := cc.API.LineupChannel.GetEnabledChannelsForGuideProvider(providerID)
	if guideChannelsErr != nil {
		return fmt.Errorf("error getting guide sources for lineup: %s", guideChannelsErr)
	}

	channelsToGet := make([]string, 0)

	for _, channel := range guideChannels {
		if !utils.Contains(channelsToGet, channel.GuideChannel.XMLTVID) {
			channelsToGet = append(channelsToGet, channel.GuideChannel.XMLTVID)
		}
	}

	log.Infof("Beginning import of guide data from provider %d, getting channels %s", providerID, strings.Join(channelsToGet, ", "))
	schedule, scheduleErr := cc.GuideSourceProviders[providerID].Schedule(channelsToGet)
	if scheduleErr != nil {
		return fmt.Errorf("error when updating schedule for provider %s: %s", providerID, scheduleErr)
	}

	for _, programme := range schedule {
		_, programmeErr := cc.API.GuideSourceProgramme.InsertGuideSourceProgramme(providerID, programme, nil)
		if programmeErr != nil {
			return fmt.Errorf("error while inserting programmes: %s", programmeErr)
		}
	}

	log.Infof("Completed import of %d programs", len(schedule))

	return nil
}

// StartFireGuideUpdates Scheduler triggered function to update guide sources
func StartFireGuideUpdates(cc *context.CContext, providerID int) {
	err := fireGuideUpdates(cc, providerID)
	if err != nil {
		log.Errorln("Could not complete guide updates " + err.Error())
	}

	log.Infoln("Guide source has been updated successfully")
}
