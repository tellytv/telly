package api

import (
	"bufio"
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
	ssdp "github.com/koron/go-ssdp"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/xmltv"
	"github.com/tellytv/telly/models"
	"github.com/zsais/go-gin-prometheus"
)

func ServeLineup(cc *context.CContext) {
	discoveryData := GetDiscoveryData()

	log.Debugln("creating device xml")
	upnp := discoveryData.UPNP()

	router := gin.New()
	router.Use(cors.Default())
	router.Use(gin.Recovery())

	if viper.GetBool("log.logrequests") {
		router.Use(ginrus())
	}

	p := ginprometheus.NewPrometheus("http")
	p.Use(router)

	router.GET("/", deviceXML(upnp))
	router.GET("/device.xml", deviceXML(upnp))
	router.GET("/discover.json", discovery(discoveryData))
	router.GET("/lineup_status.json", lineupStatus(false)) // FIXME: replace bool with cc.Lineup.Scanning
	router.POST("/lineup.post", scanChannels(cc))
	router.GET("/lineup.json", serveHDHRLineup(cc.Lineup))
	router.GET("/lineup.xml", serveHDHRLineup(cc.Lineup))
	router.GET("/auto/:channelID", stream(cc.Lineup))
	router.GET("/epg.xml", xmlTV(cc.Lineup))
	router.GET("/debug.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, cc.Lineup)
	})

	if viper.GetBool("discovery.ssdp") {
		if _, ssdpErr := setupSSDP(viper.GetString("web.base-address"), viper.GetString("discovery.device-friendly-name"), viper.GetString("discovery.device-uuid")); ssdpErr != nil {
			log.WithError(ssdpErr).Errorln("telly cannot advertise over ssdp")
		}
	}

	if err := router.Run(viper.GetString("web.listen-address")); err != nil {
		log.WithError(err).Panicln("Error starting up web server")
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
	Programs []models.HDHomeRunLineupItem
}

func serveHDHRLineup(lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		channels := make([]models.HDHomeRunLineupItem, 0)
		for _, channel := range lineup.Channels {
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

func xmlTV(lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		// FIXME: Move this outside of the function stuff.
		epg := &xmltv.TV{
			GeneratorInfoName: "telly",
			GeneratorInfoURL:  "https://github.com/tellytv/telly",
		}

		for _, channel := range lineup.Channels {
			if channel.ProviderChannel.EPGChannel != nil {
				epg.Channels = append(epg.Channels, *channel.ProviderChannel.EPGChannel)
				epg.Programmes = append(epg.Programmes, channel.ProviderChannel.EPGProgrammes...)
			}
		}

		sort.Slice(epg.Channels, func(i, j int) bool {
			return epg.Channels[i].LCN < epg.Channels[j].LCN
		})

		buf, marshallErr := xml.MarshalIndent(epg, "", "\t")
		if marshallErr != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling EPG to XML"))
		}
		c.Data(http.StatusOK, "application/xml", []byte(xml.Header+`<!DOCTYPE tv SYSTEM "xmltv.dtd">`+"\n"+string(buf)))
	}
}

func stream(lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelIDStr := c.Param("channelID")[1:]
		channelID, channelIDErr := strconv.Atoi(channelIDStr)
		if channelIDErr != nil {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("that (%s) doesn't appear to be a valid channel number", channelIDStr))
			return
		}

		if channel, ok := lineup.Channels[channelID]; ok {
			log.Infof("Serving channel number %d", channelID)

			if !viper.IsSet("iptv.ffmpeg") {
				c.Redirect(http.StatusMovedPermanently, channel.ProviderChannel.Track.URI)
				return
			}

			log.Infoln("Transcoding stream with ffmpeg")

			run := exec.Command("ffmpeg", "-re", "-i", channel.ProviderChannel.Track.URI, "-codec", "copy", "-bsf:v", "h264_mp4toannexb", "-f", "mpegts", "-tune", "zerolatency", "pipe:1")
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

			go func() {
				scanner := bufio.NewScanner(stderr)
				scanner.Split(split)
				for scanner.Scan() {
					log.Println(scanner.Text())
				}
			}()

			continueStream := true

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

func scanChannels(cc *context.CContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		scanAction := c.Query("scan")
		if scanAction == "start" {
			if refreshErr := cc.Lineup.Scan(); refreshErr != nil {
				c.AbortWithError(http.StatusInternalServerError, refreshErr)
			}
			c.AbortWithStatus(http.StatusOK)
			return
		} else if scanAction == "abort" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.String(http.StatusBadRequest, "%s is not a valid scan command", scanAction)
	}
}

func lineupStatus(scanning bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := LineupStatus{
			ScanInProgress: models.ConvertibleBoolean(false),
			ScanPossible:   models.ConvertibleBoolean(true),
			Source:         "Cable",
			SourceList:     []string{"Cable"},
		}
		if scanning {
			payload = LineupStatus{
				ScanInProgress: models.ConvertibleBoolean(true),
				// Gotta fake out Plex.
				Progress: 50,
				Found:    50,
			}
		}

		c.JSON(http.StatusOK, payload)
	}
}
