package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func addGuide(cc *context.CContext, c *gin.Context) {
	var payload models.GuideSource
	if c.BindJSON(&payload) == nil {
		newGuide, providerErr := cc.API.GuideSource.InsertGuideSource(payload, nil)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		providerCfg := newGuide.ProviderConfiguration()

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		cc.GuideSourceProviders[newGuide.ID] = provider

		log.Infoln("Detected passed config is for provider", provider.Name())

		channels, channelsErr := provider.Channels()
		if channelsErr != nil {
			log.WithError(channelsErr).Errorln("unable to get channels from provider")
			c.AbortWithError(http.StatusBadRequest, channelsErr)
			return
		}

		for _, channel := range channels {
			newChannel, newChannelErr := cc.API.GuideSourceChannel.InsertGuideSourceChannel(newGuide.ID, channel, nil)
			if newChannelErr != nil {
				log.WithError(newChannelErr).Errorln("Error creating new guide source channel!")
				c.AbortWithError(http.StatusInternalServerError, newChannelErr)
				return
			}
			newGuide.Channels = append(newGuide.Channels, *newChannel)
		}

		c.JSON(http.StatusOK, newGuide)
	}
}

func getGuideSources(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.GuideSource.GetAllGuideSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}
	c.JSON(http.StatusOK, sources)
}

func getAllChannels(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.GuideSource.GetAllGuideSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}

	channels := make([]models.GuideSourceChannel, 0)

	for _, source := range sources {
		for _, channel := range source.Channels {
			channel.GuideSourceName = source.Name
			channels = append(channels, channel)
		}
	}

	c.JSON(http.StatusOK, channels)
}

func getAllProgrammes(cc *context.CContext, c *gin.Context) {
	programmes, programmesErr := cc.API.GuideSourceProgramme.GetProgrammesForGuideID(2)
	if programmesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, programmesErr)
		return
	}
	c.JSON(http.StatusOK, programmes)
}
