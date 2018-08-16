package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ssdp "github.com/koron/go-ssdp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	ginprometheus "github.com/tombowditch/telly/internal/go-gin-prometheus"
	"github.com/tombowditch/telly/internal/xmltv"
)

func serve(lineup *lineup) {
	discoveryData := getDiscoveryData()

	log.Debugln("creating device xml")
	upnp := discoveryData.UPNP()

	log.Debugln("creating webserver routes")

	if viper.GetString("log.level") != logrus.DebugLevel.String() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	if viper.GetBool("log.logrequests") {
		router.Use(ginrus())
	}

	p := ginprometheus.NewPrometheus("http")
	p.Use(router)

	router.GET("/", deviceXML(upnp))
	router.GET("/discover.json", discovery(discoveryData))
	router.GET("/lineup_status.json", func(c *gin.Context) {
		payload := LineupStatus{
			ScanInProgress: convertibleBoolean(false),
			ScanPossible:   convertibleBoolean(true),
			Source:         "Cable",
			SourceList:     []string{"Cable"},
		}
		if lineup.Scanning {
			payload = LineupStatus{
				ScanInProgress: convertibleBoolean(true),
				// Gotta fake out Plex.
				Progress: 50,
				Found:    50,
			}
		}

		c.JSON(http.StatusOK, payload)
	})
	router.POST("/lineup.post", func(c *gin.Context) {
		scanAction := c.Query("scan")
		if scanAction == "start" {
			if refreshErr := lineup.Scan(); refreshErr != nil {
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
	router.GET("/lineup.json", serveLineup(lineup))
	router.GET("/lineup.xml", serveLineup(lineup))
	router.GET("/auto/:channelID", stream(lineup))
	router.GET("/epg.xml", xmlTV(lineup))
	router.GET("/debug.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, lineup)
	})

	if viper.GetBool("discovery.ssdp") {
		if _, ssdpErr := setupSSDP(viper.GetString("web.base-address"), viper.GetString("discovery.device-friendly-name"), viper.GetString("discovery.device-uuid")); ssdpErr != nil {
			log.WithError(ssdpErr).Errorln("telly cannot advertise over ssdp")
		}
	}

	log.Infof("telly is live and on the air!")
	log.Infof("Broadcasting from http://%s/", viper.GetString("web.listen-address"))
	log.Infof("EPG URL: http://%s/epg.xml", viper.GetString("web.listen-address"))
	if err := router.Run(viper.GetString("web.listen-address")); err != nil {
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

type hdhrLineupContainer struct {
	XMLName  xml.Name `xml:"Lineup"    json:"-"`
	Programs []hdHomeRunLineupItem
}

func serveLineup(lineup *lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		channels := make([]hdHomeRunLineupItem, 0)
		for _, channel := range lineup.channels {
			channels = append(channels, channel)
		}
		sort.Slice(channels, func(i, j int) bool {
			return channels[i].GuideNumber < channels[j].GuideNumber
		})
		if strings.HasSuffix(c.Request.URL.String(), ".xml") {
			buf, marshallErr := xml.MarshalIndent(hdhrLineupContainer{Programs: channels}, "", "\t")
			if marshallErr != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling lineup to XML"))
			}
			c.Data(http.StatusOK, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+string(buf)))
			return
		}
		c.JSON(http.StatusOK, channels)
	}
}

func xmlTV(lineup *lineup) gin.HandlerFunc {
	epg := &xmltv.TV{
		GeneratorInfoName: namespaceWithVersion,
		GeneratorInfoURL:  "https://github.com/tombowditch/telly",
	}

	for _, channel := range lineup.channels {
		if channel.providerChannel.EPGChannel != nil {
			epg.Channels = append(epg.Channels, *channel.providerChannel.EPGChannel)
			epg.Programmes = append(epg.Programmes, channel.providerChannel.EPGProgrammes...)
		}
	}

	sort.Slice(epg.Channels, func(i, j int) bool { return epg.Channels[i].LCN < epg.Channels[j].LCN })

	return func(c *gin.Context) {
		buf, marshallErr := xml.MarshalIndent(epg, "", "\t")
		if marshallErr != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling EPG to XML"))
		}
		c.Data(http.StatusOK, "application/xml", []byte(xml.Header+`<!DOCTYPE tv SYSTEM "xmltv.dtd">`+"\n"+string(buf)))
	}
}

func stream(lineup *lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelIDStr := c.Param("channelID")[1:]
		channelID, channelIDErr := strconv.Atoi(channelIDStr)
		if channelIDErr != nil {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("that (%s) doesn't appear to be a valid channel number", channelIDStr))
			return
		}

		if channel, ok := lineup.channels[channelID]; ok {
			log.Infof("Serving channel number %d", channelID)
			c.Redirect(http.StatusMovedPermanently, channel.providerChannel.Track.URI)
			return
		}
		c.AbortWithError(http.StatusNotFound, fmt.Errorf("unknown channel number %d", channelID))
	}
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
