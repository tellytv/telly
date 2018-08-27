package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func getLineup(lineup *models.Lineup, cc *context.CContext, c *gin.Context) {
	c.JSON(http.StatusOK, lineup)
}

func addLineupChannel(lineup *models.Lineup, cc *context.CContext, c *gin.Context) {
	var payload models.LineupChannel
	if c.BindJSON(&payload) == nil {
		payload.LineupID = lineup.ID
		newChannel, lineupErr := cc.API.LineupChannel.InsertLineupChannel(payload)
		if lineupErr != nil {
			c.AbortWithError(http.StatusInternalServerError, lineupErr)
			return
		}

		RestartTuner(cc, lineup)

		c.JSON(http.StatusOK, newChannel)
	}
}

func updateLineupChannels(lineup *models.Lineup, cc *context.CContext, c *gin.Context) {
	newChannels := make([]models.LineupChannel, 0)
	if c.BindJSON(&newChannels) == nil {
		for idx, channel := range newChannels {
			channel.LineupID = lineup.ID
			channel.GuideChannel = nil
			channel.HDHR = nil
			channel.VideoTrack = nil
			newChannel, lineupErr := cc.API.LineupChannel.UpsertLineupChannel(channel)
			if lineupErr != nil {
				c.AbortWithError(http.StatusInternalServerError, lineupErr)
				return
			}
			newChannel.Fill(cc.API)
			newChannels[idx] = *newChannel
		}

		lineup.Channels = newChannels

		RestartTuner(cc, lineup)

		c.JSON(http.StatusOK, lineup)
	}
}
