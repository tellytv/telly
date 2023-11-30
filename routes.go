package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	ssdp "github.com/koron/go-ssdp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	ginprometheus "github.com/tellytv/telly/internal/go-gin-prometheus"
	"github.com/tellytv/telly/internal/xmltv"
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
	router.Use(cors.Default())
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

	box := packr.NewBox("./frontend/dist/telly-fe")

	router.StaticFS("/manage", box)

	log.Infof("telly is live and on the air!")
	log.Infof("Broadcasting from http://%s/", viper.GetString("web.base-address"))
	log.Infof("EPG URL: http://%s/epg.xml", viper.GetString("web.base-address"))
	log.Infof("Lineup JSON: http://%s/lineup.json", viper.GetString("web.base-address"))

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
		GeneratorInfoURL:  "https://github.com/tellytv/telly",
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
			channelURI := channel.providerChannel.Track.URI

			log.Infof("Serving channel number %d", channelID)

			if !lineup.FfmpegEnabled {
				log.Debugf("Redirecting caller to %s", channelURI)
				c.Redirect(http.StatusMovedPermanently, channelURI.String())
				return
			}

			log.Infoln("Remuxing stream with ffmpeg")
			run := exec.Command("ffmpeg", "-i", "pipe:0", "-c:v", "copy", "-f", "mpegts", "pipe:1")
			log.Debugf("Executing ffmpeg as \"%s\"", strings.Join(run.Args, " "))
			ffmpegout, err := run.StdoutPipe()
			if err != nil {
				log.WithError(err).Errorln("StdoutPipe Error")
				return
			}

			stderr, stderrErr := run.StderrPipe()
			if stderrErr != nil {
				log.WithError(stderrErr).Errorln("Error creating ffmpeg stderr pipe")
			}

			if startErr := run.Start(); startErr != nil {
				log.WithError(startErr).Errorln("Error starting ffmpeg")
				return
			}
			defer run.Wait()

			go func() {
				scanner := bufio.NewScanner(stderr)
				scanner.Split(split)
				for scanner.Scan() {
					log.Println(scanner.Text())
				}
			}()

			continueStream := true
			c.Header("Content-Type", `video/mpeg; codecs="avc1.4D401E"`)

			c.Stream(func(w io.Writer) bool {
				defer func() {
					log.Infoln("Stopped streaming", channelID)
					if killErr := run.Process.Kill(); killErr != nil {
						panic(killErr)
					}
					continueStream = false
					return
				}()
				if _, copyErr := io.Copy(w, ffmpegout); copyErr != nil {
					log.WithError(copyErr).Errorln("Error when copying data")
					continueStream = false
					return false
				}
				return continueStream
			})

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
			<-aliveTick
			if err := advertiser.Alive(); err != nil {
				log.WithError(err).Panicln("error when sending ssdp heartbeat")
			}
		}
	}(adv)

	return adv, nil
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
