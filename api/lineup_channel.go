package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/commands"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
	"github.com/tellytv/telly/utils"
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
	providedChannels := make([]models.LineupChannel, 0)
	guideSources := make(map[int]*models.GuideSource)
	existingChannelIDs := make([]string, 0)
	passedChannelIDs := make([]string, 0)

	for _, channel := range lineup.Channels {
		existingChannelIDs = append(existingChannelIDs, strconv.Itoa(channel.ID))
	}

	if c.BindJSON(&providedChannels) == nil {
		for _, channel := range providedChannels {
			if channel.ID > 0 {
				passedChannelIDs = append(passedChannelIDs, strconv.Itoa(channel.ID))
			}
		}

		deletedChannelIDs := utils.Difference(existingChannelIDs, passedChannelIDs)

		for idx, channel := range providedChannels {
			if channel.ID > 0 {
				passedChannelIDs = append(passedChannelIDs, strconv.Itoa(channel.ID))
			} else if utils.Contains(deletedChannelIDs, strconv.Itoa(channel.ID)) {
				// Channel is about to be deleted, no reason to upsert it.
				continue
			}
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
			providedChannels[idx] = *newChannel
		}

		for _, deletedID := range deletedChannelIDs {
			if deleteProgrammesErr := cc.API.GuideSourceProgramme.DeleteGuideSourceProgrammesForChannel(deletedID); deleteProgrammesErr != nil {
				c.AbortWithError(http.StatusInternalServerError, deleteProgrammesErr)
				return
			}
			if deleteErr := cc.API.LineupChannel.DeleteLineupChannel(deletedID); deleteErr != nil {
				c.AbortWithError(http.StatusInternalServerError, deleteErr)
				return
			}
		}

		lineup.Channels = providedChannels

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
