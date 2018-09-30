package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/models"
	"github.com/tellytv/telly/internal/utils"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

func scanM3U(c *gin.Context) {
	rawPlaylist, m3uErr := utils.GetM3U(c.Query("m3u_url"))
	if m3uErr != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("unable to get m3u file: %s", m3uErr))
		return
	}

	c.JSON(http.StatusOK, rawPlaylist)
}

func scanXMLTV(c *gin.Context) {
	epg, epgErr := utils.GetXMLTV(c.Query("epg_url"))
	if epgErr != nil {
		c.AbortWithError(http.StatusInternalServerError, epgErr)
		return
	}

	epg.Programmes = nil

	c.JSON(http.StatusOK, epg)
}

// LineupStatus exposes the status of the channel lineup.
type LineupStatus struct {
	ScanInProgress models.ConvertibleBoolean
	ScanPossible   models.ConvertibleBoolean `json:",omitempty"`
	Source         string                    `json:",omitempty"`
	SourceList     []string                  `json:",omitempty"`
	Progress       int                       `json:",omitempty"` // Percent complete
	Found          int                       `json:",omitempty"` // Number of found channels
}

func ginrus(cc *context.CContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		end = end.UTC()

		logFields := logrus.Fields{
			"status":    c.Writer.Status(),
			"method":    c.Request.Method,
			"path":      path,
			"ipAddress": c.ClientIP(),
			"latency":   latency,
			"userAgent": c.Request.UserAgent(),
			"time":      end.Format(time.RFC3339),
		}

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			logFields["error"] = c.Errors.String()
			cc.Log.WithFields(logFields).Errorln("Error while serving request")
		} else if viper.GetBool("log.requests") {
			cc.Log.WithFields(logFields).Infoln()
		}
	}
}

func wrapContext(cc *context.CContext, originalFunc func(*context.CContext, *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := cc.Copy()
		originalFunc(ctx, c)
	}
}

// ServeBox returns a middleware handler that serves static files from a Packr box.
func ServeBox(urlPrefix string, box packr.Box) gin.HandlerFunc {
	fileserver := http.FileServer(box)
	if urlPrefix != "" {
		fileserver = http.StripPrefix(urlPrefix, fileserver)
	}
	return func(c *gin.Context) {
		if box.Has(c.Request.URL.Path) {
			fileserver.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		}
	}
}

var prom = ginprometheus.NewPrometheus("http")

func newGin(cc *context.CContext) *gin.Engine {
	router := gin.New()
	router.Use(cors.Default())
	router.Use(gin.Recovery())
	router.Use(ginrus(cc))

	prom.Use(router)
	return router
}

// StartTuner will start a new tuner server for the given lineup.
func StartTuner(cc *context.CContext, lineup *models.Lineup) {
	tunerChan := make(chan bool)
	cc.Tuners[lineup.ID] = tunerChan
	go ServeLineup(cc, tunerChan, lineup)
}

// RestartTuner will trigger a restart of the tuner server for the given lineup.
func RestartTuner(cc *context.CContext, lineup *models.Lineup) {
	if tuner, ok := cc.Tuners[lineup.ID]; ok {
		tuner <- true
	}
	StartTuner(cc, lineup)
}
