package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	ssdp "github.com/koron/go-ssdp"
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
		ScanInProgress: convertibleBoolean(opts.lineup.Refreshing),
		ScanPossible:   convertibleBoolean(true),
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}))
	router.POST("/lineup.post", func(c *gin.Context) {
		scanAction := c.Query("scan")
		if scanAction == "start" {
			if refreshErr := opts.lineup.Refresh(); refreshErr != nil {
				c.AbortWithError(http.StatusInternalServerError, refreshErr)
			}
			c.AbortWithStatus(http.StatusOK)
			return
		} else if scanAction == "abort" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.String(http.StatusBadRequest, "%s is not a valid scan command", scanAction)
	})
	router.GET("/device.xml", deviceXML(upnp))
	router.GET("/lineup.json", lineup(opts.lineup))
	router.GET("/stream/:channelID", stream)
	router.GET("/debug.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, opts.lineup)
	})

	if opts.SSDP {
		log.Debugln("advertising telly service on network via UPNP/SSDP")
		if _, ssdpErr := setupSSDP(opts.BaseAddress.String(), opts.FriendlyName, opts.DeviceUUID); ssdpErr != nil {
			log.WithError(ssdpErr).Errorln("telly cannot advertise over ssdp")
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

func lineup(lineup *Lineup) gin.HandlerFunc {
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

func setupSSDP(baseAddress, deviceName, deviceUUID string) (*ssdp.Advertiser, error) {
	log.Debugf("Advertising telly as %s (%s)", deviceName, deviceUUID)

	adv, err := ssdp.Advertise(
		"upnp:rootdevice",
		fmt.Sprintf("uuid:%s::upnp:rootdevice", deviceUUID),
		fmt.Sprintf("http://%s/device.xml", baseAddress),
		deviceName,
		1800)

	if err != nil {
		return nil, err
	}

	go func(advertiser *ssdp.Advertiser) {
		aliveTick := time.Tick(15 * time.Second)

		for {
			select {
			case <-aliveTick:
				if err := advertiser.Alive(); err != nil {
					log.WithError(err).Panicln("error when sending ssdp heartbeat")
				}
			}
		}
	}(adv)

	return adv, nil
}
