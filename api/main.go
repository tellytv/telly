package api

import (
	"bytes"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/context"
	"github.com/zsais/go-gin-prometheus"
)

var log = &logrus.Logger{
	Out: os.Stderr,
	Formatter: &logrus.TextFormatter{
		FullTimestamp: true,
	},
	Hooks: make(logrus.LevelHooks),
	Level: logrus.DebugLevel,
}

func ServeAPI(cc *context.CContext) {
	log.Debugln("creating webserver routes")

	if viper.GetString("log.level") != logrus.DebugLevel.String() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(cors.Default())
	router.Use(gin.Recovery())

	if viper.GetBool("log.logrequests") {
		router.Use(ginrus())
	}

	p := ginprometheus.NewPrometheus("http")
	p.Use(router)

	box := packr.NewBox("../frontend/dist/telly-fe")

	router.Use(ServeBox("/", box))

	apiGroup := router.Group("/api")

	apiGroup.GET("/lineup/scan", scanM3U)
	apiGroup.GET("/guide/scan", scanXMLTV)

	apiGroup.GET("/guide_sources", wrapContext(cc, getGuideSources))
	apiGroup.POST("/guide_sources", wrapContext(cc, addGuide))

	apiGroup.GET("/video_sources", wrapContext(cc, getVideoSources))
	apiGroup.POST("/video_sources", wrapContext(cc, addVideoSource))

	log.Infof("telly is live and on the air!")
	log.Infof("Broadcasting from http://%s/", viper.GetString("web.listen-address"))
	log.Infof("EPG URL: http://%s/epg.xml", viper.GetString("web.listen-address"))

	if err := router.Run(viper.GetString("web.listen-address")); err != nil {
		log.WithError(err).Panicln("Error starting up web server")
	}
}

func split(data []byte, atEOF bool) (advance int, token []byte, spliterror error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0:i], nil
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		// We have a cr terminated line
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}
