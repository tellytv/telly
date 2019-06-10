package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/models"
)

func getVideoSources(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.VideoSource.GetAllVideoSources(false)
	if sourcesErr != nil {
		cc.Log.WithError(sourcesErr).Errorln("error getting all video sources")
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

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			cc.Log.WithError(providerErr).Errorln("error getting provider")
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		cc.VideoSourceProviders[newProvider.ID] = provider

		cc.Log.Infoln("Detected passed config is for provider", provider.Name())

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
				cc.Log.WithError(newTrackErr).Errorln("Error creating new video source track!")
				c.AbortWithError(http.StatusInternalServerError, newTrackErr)
				return
			}
			newProvider.Tracks = append(newProvider.Tracks, *newTrack)
		}
		c.JSON(http.StatusCreated, newProvider)
	}
}

func saveVideoSource(cc *context.CContext, c *gin.Context) {
	videoSourceID := c.Param("sourceId")

	iVideoSourceID, err := strconv.ParseInt(videoSourceID, 0, 32)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var payload models.VideoSource
	if c.BindJSON(&payload) == nil {
		provider, providerErr := cc.API.VideoSource.UpdateVideoSource(int(iVideoSourceID), payload)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		c.JSON(http.StatusOK, provider)
	}
}

func deleteVideoSource(cc *context.CContext, c *gin.Context) {
	videoSourceID := c.Param("sourceId")

	iVideoSourceID, err := strconv.ParseInt(videoSourceID, 0, 32)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	err = cc.API.VideoSource.DeleteVideoSource(int(iVideoSourceID))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
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
