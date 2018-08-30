package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/commands"
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
	guideSources := make(map[int]*models.GuideSource)
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
			guideSources[newChannel.GuideChannel.GuideSource.ID] = newChannel.GuideChannel.GuideSource
			newChannels[idx] = *newChannel
		}

		lineup.Channels = newChannels

		// Update guide data for every provider with a new channel in the background
		for _, source := range guideSources {
			go commands.StartFireGuideUpdates(cc, source)
		}

		// Finally, restart the tuner
		RestartTuner(cc, lineup)

		c.JSON(http.StatusOK, lineup)
	}
}

func refreshLineup(lineup *models.Lineup, cc *context.CContext, c *gin.Context) {
	guideSources := make(map[int]*models.GuideSource)

	for _, channel := range lineup.Channels {
		guideSources[channel.GuideChannel.GuideSource.ID] = channel.GuideChannel.GuideSource
	}

	// Update guide data for every provider with a new channel in the background
	for _, source := range guideSources {
		go commands.StartFireGuideUpdates(cc, source)
	}

	c.JSON(http.StatusOK, gin.H{"status": "okay", "message": "Beginning refresh of lineup data"})
}
