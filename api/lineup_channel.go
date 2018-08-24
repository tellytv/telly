package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func getLineup(lineup *models.SQLLineup, cc *context.CContext, c *gin.Context) {
	c.JSON(http.StatusOK, lineup)
}

func addLineupChannel(lineup *models.SQLLineup, cc *context.CContext, c *gin.Context) {
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
