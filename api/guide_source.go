package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
	"github.com/tellytv/telly/utils"
)

func addGuide(cc *context.CContext, c *gin.Context) {
	var payload models.GuideSource
	if c.BindJSON(&payload) == nil {
		newGuide, providerErr := cc.API.GuideSource.InsertGuideSource(payload)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		providerCfg := newGuide.ProviderConfiguration()

		log.Infof("providerCfg %+v", providerCfg)

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		log.Infoln("Detected passed config is for provider", provider.Name())

		xmlTV, xmlErr := utils.GetXMLTV(provider.EPGURL(), false)
		if xmlErr != nil {
			log.WithError(xmlErr).Errorln("unable to get XMLTV file")
			c.AbortWithError(http.StatusBadRequest, xmlErr)
			return
		}

		for _, channel := range xmlTV.Channels {
			newChannel, newChannelErr := cc.API.GuideSourceChannel.InsertGuideSourceChannel(newGuide.ID, channel)
			if newChannelErr != nil {
				log.WithError(newChannelErr).Errorln("Error creating new guide source channel!")
				c.AbortWithError(http.StatusInternalServerError, newChannelErr)
				return
			}
			newGuide.Channels = append(newGuide.Channels, *newChannel)
		}
		// FIXME: Instead of importing _every_ programme when we add a new guide source, we should only import programmes for channels in a lineup.
		// Otherwise, SQLite DB gets a lot bigger and harder to manage.
		for _, programme := range xmlTV.Programmes {
			_, programmeErr := cc.API.GuideSourceProgramme.InsertGuideSourceProgramme(newGuide.ID, programme)
			if programmeErr != nil {
				log.WithError(programmeErr).Errorln("Error creating new guide source channel during programme import!")
				c.AbortWithError(http.StatusInternalServerError, programmeErr)
				return
			}
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
