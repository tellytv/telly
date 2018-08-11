package main

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

func serve(opts config) {
	discoveryData := opts.DiscoveryData()

	log.Debugln("creating device xml")
	upnp := discoveryData.UPNP()

	log.Debugln("creating webserver routes")

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	if opts.LogRequests {
		router.Use(ginrus())
	}

	p := ginprometheus.NewPrometheus("http")
	p.Use(router)

	router.GET("/", deviceXML(upnp))
	router.GET("/discover.json", discovery(discoveryData))
	router.GET("/lineup_status.json", lineupStatus(LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}))
	router.GET("/lineup.post", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotImplemented)
	})
	router.GET("/device.xml", deviceXML(upnp))
	router.GET("/lineup.json", lineup(opts.lineup))
	router.GET("/stream/:channelID", stream)

	if opts.SSDP {
		log.Debugln("advertising telly service on network via UPNP/SSDP")
		if _, ssdpErr := setupSSDP(opts.BaseAddress.String(), opts.FriendlyName, opts.DeviceUUID); ssdpErr != nil {
			log.WithError(ssdpErr).Warnln("telly cannot advertise over ssdp")
		}
	}

	log.Infof("Listening and serving HTTP on %s", opts.ListenAddress)
	if err := router.Run(opts.ListenAddress.String()); err != nil {
		log.WithError(err).Panicln("Error starting up web server")
	}
}

func deviceXML(deviceXML UPNP) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.XML(http.StatusOK, deviceXML)
	}
}

func discovery(data DiscoveryData) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, data)
	}
}

func lineupStatus(status LineupStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, status)
	}
}

func lineup(lineup []LineupItem) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, lineup)
	}
}

func stream(c *gin.Context) {

	channelID := c.Param("channelID")

	log.Debugf("Parsing URI %s to %s", c.Request.RequestURI, channelID)

	decodedStreamURI, decodeErr := base64.StdEncoding.DecodeString(channelID)
	if decodeErr != nil {
		log.WithError(decodeErr).Errorf("Invalid base64: %s", channelID)
		c.AbortWithError(http.StatusBadRequest, decodeErr)
		return
	}

	log.Debugln("Redirecting to:", string(decodedStreamURI))
	c.Redirect(http.StatusMovedPermanently, string(decodedStreamURI))
}

func ginrus() gin.HandlerFunc {
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

		entry := log.WithFields(logFields)

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			entry.Error(c.Errors.String())
		} else {
			entry.Info()
		}
	}
}
