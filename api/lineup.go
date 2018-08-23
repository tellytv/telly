package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/models"
)

func addLineup(cc *context.CContext, c *gin.Context) {
	var payload models.SQLLineup
	if c.BindJSON(&payload) == nil {
		newLineup, lineupErr := cc.API.Lineup.InsertLineup(payload)
		if lineupErr != nil {
			c.AbortWithError(http.StatusInternalServerError, lineupErr)
			return
		}

		tunerChan := make(chan bool)
		cc.Tuners[newLineup.ID] = tunerChan
		go ServeLineup(cc, tunerChan, newLineup)

		c.JSON(http.StatusOK, newLineup)
	}
}

func getLineups(cc *context.CContext, c *gin.Context) {
	allLineups, lineupErr := cc.API.Lineup.GetEnabledLineups(true)
	if lineupErr != nil {
		c.AbortWithError(http.StatusInternalServerError, lineupErr)
		return
	}

	c.JSON(http.StatusOK, allLineups)
}

func lineupRoute(cc *context.CContext, originalFunc func(*models.SQLLineup, *context.CContext, *gin.Context)) gin.HandlerFunc {
	return wrapContext(cc, func(cc *context.CContext, c *gin.Context) {
		lineup, lineupErr := cc.API.Lineup.GetLineupByID(c.Param("lineupId"))
		if lineupErr != nil {
			c.AbortWithError(http.StatusInternalServerError, lineupErr)
			return
		}
		originalFunc(lineup, cc, c)
	})
}
