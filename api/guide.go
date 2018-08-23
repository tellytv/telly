package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func addGuide(cc *context.CContext, c *gin.Context) {
	var payload models.GuideSource
	if c.BindJSON(&payload) == nil {
		newProvider, providerErr := cc.API.GuideSource.InsertGuideSource(payload)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		providerCfg := newProvider.ProviderConfiguration()

		log.Infof("providerCfg %+v", providerCfg)

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		log.Infoln("Detected passed config is for provider", provider.Name())

		xmlTV, xmlErr := models.GetXMLTV(provider.EPGURL(), false)
		if xmlErr != nil {
			log.WithError(xmlErr).Errorln("unable to get XMLTV file")
			c.AbortWithError(http.StatusBadRequest, xmlErr)
			return
		}

		for _, channel := range xmlTV.Channels {
			displayNames, _ := json.Marshal(channel.DisplayNames)
			urls, _ := json.Marshal(channel.URLs)
			icons, _ := json.Marshal(channel.Icons)
			newChannel, newChannelErr := cc.API.GuideSourceChannel.InsertGuideSourceChannel(models.GuideSourceChannel{
				GuideID:       newProvider.ID,
				XMLTVID:       channel.ID,
				DisplayNames:  displayNames,
				URLs:          urls,
				Icons:         icons,
				ChannelNumber: strconv.Itoa(channel.LCN),
			})
			if newChannelErr != nil {
				log.WithError(newChannelErr).Errorln("Error creating new guide source channel!")
				c.AbortWithError(http.StatusInternalServerError, newChannelErr)
				return
			}
			newProvider.Channels = append(newProvider.Channels, *newChannel)
		}
		c.JSON(http.StatusOK, newProvider)
	}
}
