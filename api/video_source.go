package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func getVideoSources(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.VideoSource.GetAllVideoSources(false)
	if sourcesErr != nil {
		log.WithError(sourcesErr).Errorln("error getting all video sources")
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}
	c.JSON(http.StatusOK, sources)
}

func addVideoSource(cc *context.CContext, c *gin.Context) {
	var payload models.VideoSource
	if c.BindJSON(&payload) == nil {
		newProvider, providerErr := cc.API.VideoSource.InsertVideoSource(payload)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		providerCfg := newProvider.ProviderConfiguration()

		log.Infof("providerCfg %+v", providerCfg)

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			log.WithError(providerErr).Errorln("error getting provider")
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		log.Infoln("Detected passed config is for provider", provider.Name())

		channels, channelsErr := provider.Channels()
		if channelsErr != nil {
			c.AbortWithError(http.StatusInternalServerError, channelsErr)
			return
		}

		for _, channel := range channels {
			newTrack, newTrackErr := cc.API.VideoSourceTrack.InsertVideoSourceTrack(models.VideoSourceTrack{
				VideoSourceID: newProvider.ID,
				Name:          channel.Name,
				StreamID:      channel.StreamID,
				Logo:          channel.Logo,
				Type:          string(channel.Type),
				Category:      channel.Category,
				EPGID:         channel.EPGID,
			})
			if newTrackErr != nil {
				log.WithError(newTrackErr).Errorln("Error creating new video source track!")
				c.AbortWithError(http.StatusInternalServerError, newTrackErr)
				return
			}
			newProvider.Tracks = append(newProvider.Tracks, *newTrack)
		}
		c.JSON(http.StatusOK, newProvider)
	}
}

func getAllTracks(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.VideoSource.GetAllVideoSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}

	tracks := make([]models.VideoSourceTrack, 0)

	for _, source := range sources {
		for _, track := range source.Tracks {
			track.VideoSourceName = source.Name
			tracks = append(tracks, track)
		}
	}

	c.JSON(http.StatusOK, tracks)
}
