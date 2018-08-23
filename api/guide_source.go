package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
)

func getGuideSources(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.GuideSource.GetAllGuideSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}
	c.JSON(http.StatusOK, sources)
}
