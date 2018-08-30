package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/guideproviders"
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

		lineupMetadata, reloadErr := provider.Refresh(nil)
		if reloadErr != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error while initializing guide data provider: %s", reloadErr))
			return
		}

		if updateErr := cc.API.GuideSource.UpdateGuideSource(newGuide.ID, lineupMetadata); updateErr != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error while updating guide source with provider state: %s", updateErr))
			return
		}

		channels, channelsErr := provider.Channels()
		if channelsErr != nil {
			log.WithError(channelsErr).Errorln("unable to get channels from provider")
			c.AbortWithError(http.StatusBadRequest, channelsErr)
			return
		}

		for _, channel := range channels {
			newChannel, newChannelErr := cc.API.GuideSourceChannel.InsertGuideSourceChannel(newGuide.ID, channel, nil)
			if newChannelErr != nil {
				log.WithError(newChannelErr).Errorf("Error creating new guide source channel %s!", channel.ID)
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

func getLineupCoverage(provider guideproviders.GuideProvider, cc *context.CContext, c *gin.Context) {
	coverage, coverageErr := provider.LineupCoverage()
	if coverageErr != nil {
		c.AbortWithError(http.StatusInternalServerError, coverageErr)
		return
	}
	c.JSON(http.StatusOK, coverage)
}

func getAvailableLineups(provider guideproviders.GuideProvider, cc *context.CContext, c *gin.Context) {
	countryCode := c.Query("countryCode")
	postalCode := c.Query("postalCode")
	lineups, lineupsErr := provider.AvailableLineups(countryCode, postalCode)
	if lineupsErr != nil {
		c.AbortWithError(http.StatusInternalServerError, lineupsErr)
		return
	}
	c.JSON(http.StatusOK, lineups)
}

func previewLineupChannels(provider guideproviders.GuideProvider, cc *context.CContext, c *gin.Context) {
	lineupId := c.Param("lineupId")
	channels, channelsErr := provider.PreviewLineupChannels(lineupId)
	if channelsErr != nil {
		c.AbortWithError(http.StatusInternalServerError, channelsErr)
		return
	}
	c.JSON(http.StatusOK, channels)
}

func subscribeToLineup(provider guideproviders.GuideProvider, cc *context.CContext, c *gin.Context) {
	lineupId := c.Param("lineupId")
	if subscribeErr := provider.SubscribeToLineup(lineupId); subscribeErr != nil {
		c.AbortWithError(http.StatusInternalServerError, subscribeErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "okay"})
}

func unsubscribeFromLineup(provider guideproviders.GuideProvider, cc *context.CContext, c *gin.Context) {
	lineupId := c.Param("lineupId")
	if unsubscribeErr := provider.UnsubscribeFromLineup(lineupId); unsubscribeErr != nil {
		c.AbortWithError(http.StatusInternalServerError, unsubscribeErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "okay"})
}

func guideSourceRoute(cc *context.CContext, originalFunc func(*models.GuideSource, *context.CContext, *gin.Context)) gin.HandlerFunc {
	return wrapContext(cc, func(cc *context.CContext, c *gin.Context) {
		guideSourceID, guideSourceIDErr := strconv.Atoi(c.Param("guideSourceId"))
		if guideSourceIDErr != nil {
			c.AbortWithError(http.StatusBadRequest, guideSourceIDErr)
			return
		}
		guideSource, guideSourceErr := cc.API.GuideSource.GetGuideSourceByID(guideSourceID)
		if guideSourceErr != nil {
			c.AbortWithError(http.StatusInternalServerError, guideSourceErr)
			return
		}
		originalFunc(guideSource, cc, c)
	})
}

func guideSourceLineupRoute(cc *context.CContext, originalFunc func(guideproviders.GuideProvider, *context.CContext, *gin.Context)) gin.HandlerFunc {
	return wrapContext(cc, func(cc *context.CContext, c *gin.Context) {
		guideSourceID, guideSourceIDErr := strconv.Atoi(c.Param("guideSourceId"))
		if guideSourceIDErr != nil {
			c.AbortWithError(http.StatusBadRequest, guideSourceIDErr)
			return
		}

		provider, ok := cc.GuideSourceProviders[guideSourceID]
		if !ok {
			c.AbortWithError(http.StatusNotFound, fmt.Errorf("%d is not a valid guide source provider", guideSourceID))
			return
		}

		if !provider.SupportsLineups() {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("Provider %s does not support lineups", guideSourceID))
			return
		}

		originalFunc(provider, cc, c)
	})
}
