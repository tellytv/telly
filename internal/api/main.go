package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/internal/context"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

var (
	log = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}

	prom = ginprometheus.NewPrometheus("http")
)

// ServeAPI starts up the telly frontend + REST API.
func ServeAPI(cc *context.CContext) {
	log.Debugln("creating webserver routes")

	if viper.GetString("log.level") != logrus.DebugLevel.String() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := newGin()

	box := packr.NewBox("../frontend/dist/telly-fe")

	router.Use(ServeBox("/", box))

	router.GET("/epg.xml", wrapContext(cc, xmlTV))

	apiGroup := router.Group("/api")

	apiGroup.GET("/guide/scan", scanXMLTV)

	apiGroup.GET("/lineups", wrapContext(cc, getLineups))
	apiGroup.POST("/lineups", wrapContext(cc, addLineup))
	apiGroup.GET("/lineups/:lineupId", lineupRoute(cc, getLineup))
	apiGroup.PUT("/lineups/:lineupId/channels", lineupRoute(cc, updateLineupChannels))
	apiGroup.POST("/lineups/:lineupId/channels", lineupRoute(cc, addLineupChannel))
	apiGroup.PUT("/lineups/:lineupId/refresh", lineupRoute(cc, refreshLineup))
	apiGroup.GET("/lineup/scan", scanM3U)

	apiGroup.GET("/guide_sources", wrapContext(cc, getGuideSources))
	apiGroup.POST("/guide_sources", wrapContext(cc, addGuide))
	apiGroup.GET("/guide_sources/channels", wrapContext(cc, getAllChannels))
	apiGroup.GET("/guide_sources/programmes", wrapContext(cc, getAllProgrammes))

	apiGroup.GET("/guide_source/:guideSourceId/coverage", guideSourceLineupRoute(cc, getLineupCoverage))
	apiGroup.GET("/guide_source/:guideSourceId/lineups", guideSourceLineupRoute(cc, getAvailableLineups))
	apiGroup.PUT("/guide_source/:guideSourceId/lineups/:lineupId", guideSourceLineupRoute(cc, subscribeToLineup))
	apiGroup.DELETE("/guide_source/:guideSourceId/lineups/:lineupId", guideSourceLineupRoute(cc, unsubscribeFromLineup))
	apiGroup.GET("/guide_source/:guideSourceId/lineups/:lineupId/channels", guideSourceLineupRoute(cc, previewLineupChannels))

	apiGroup.GET("/video_sources", wrapContext(cc, getVideoSources))
	apiGroup.POST("/video_sources", wrapContext(cc, addVideoSource))
	apiGroup.GET("/video_sources/tracks", wrapContext(cc, getAllTracks))

	apiGroup.GET("/streams", func(c *gin.Context) {
		c.JSON(http.StatusOK, cc.Streams)
	})

	log.Infof("telly is live and on the air!")
	log.Infof("Broadcasting from http://%s/", viper.GetString("web.listen-address"))
	log.Infof("EPG URL: http://%s/epg.xml", viper.GetString("web.listen-address"))

	if err := router.Run(viper.GetString("web.listen-address")); err != nil {
		log.WithError(err).Panicln("Error starting up web server")
	}
}
