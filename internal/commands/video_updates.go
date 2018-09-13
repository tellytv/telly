package commands

import (
	"fmt"

	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/models"
)

// FireVideoUpdatesCommand Command to fire one off video source updates
func FireVideoUpdatesCommand() {
	cc, err := context.NewCContext(nil)
	if err != nil {
		log.WithError(err).Errorf("couldn't create context")
	}
	if err = fireVideoUpdates(cc, nil); err != nil {
		log.WithError(err).Errorf("could not complete video updates")
	}
}

func fireVideoUpdates(cc *context.CContext, provider *models.VideoSource) error {
	log.Debugf("Video source update is beginning for provider", provider.Name)

	channels, channelsErr := cc.VideoSourceProviders[provider.ID].Channels()
	if channelsErr != nil {
		return fmt.Errorf("error while getting video channels during update of %s: %s", provider.Name, channelsErr)
	}

	for _, channel := range channels {
		newTrackErr := cc.API.VideoSourceTrack.UpdateVideoSourceTrack(provider.ID, channel.StreamID, models.VideoSourceTrack{
			VideoSourceID: provider.ID,
			Name:          channel.Name,
			StreamID:      channel.StreamID,
			Logo:          channel.Logo,
			Type:          string(channel.Type),
			Category:      channel.Category,
			EPGID:         channel.EPGID,
		})
		if newTrackErr != nil {
			return fmt.Errorf("error while inserting video track (source id: %d stream id: %d name: %s) during update: %s", provider.ID, channel.StreamID, channel.Name, newTrackErr)
		}
	}

	return nil
}

// StartFireVideoUpdates Scheduler triggered function to update video sources
func StartFireVideoUpdates(cc *context.CContext, provider *models.VideoSource) {
	err := fireVideoUpdates(cc, provider)
	if err != nil {
		log.WithError(err).Errorln("could not complete video updates for provider", provider.Name)
	}

	log.Infof("Video source %s has been updated successfully", provider.Name)
}
