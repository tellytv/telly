package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	upnp "github.com/NebulousLabs/go-upnp/goupnp"
	"github.com/gin-gonic/gin"
	ssdp "github.com/koron/go-ssdp"
	"github.com/spf13/viper"
	ccontext "github.com/tellytv/telly/context"
	"github.com/tellytv/telly/metrics"
	"github.com/tellytv/telly/models"
)

func ServeLineup(cc *ccontext.CContext, exit chan bool, lineup *models.Lineup) {
	channels, channelsErr := cc.API.LineupChannel.GetChannelsForLineup(lineup.ID, true)
	if channelsErr != nil {
		log.WithError(channelsErr).Errorln("error getting channels in lineup")
		return
	}

	guideSources, guideSourceErr := cc.API.GuideSource.GetGuideSourcesForLineup(lineup.ID)
	if guideSourceErr != nil {
		log.WithError(guideSourceErr).Errorln("error getting guide sources for lineup")
		return
	}

	guideSourceUpdateMap := make(map[int][]string)

	hdhrItems := make([]models.HDHomeRunLineupItem, 0)
	for _, channel := range channels {
		hdhrItems = append(hdhrItems, *channel.HDHR)

		guideSourceUpdateMap[channel.GuideChannel.GuideSource.ID] = append(guideSourceUpdateMap[channel.GuideChannel.GuideSource.ID], channel.GuideChannel.XMLTVID)
	}

	for _, guideSource := range guideSources {
		if channelsToGet, ok := guideSourceUpdateMap[guideSource.ID]; ok {
			log.Infof("Beginning import of guide data from provider %s, getting channels %s", guideSource.Name, strings.Join(channelsToGet, ", "))
			schedule, scheduleErr := cc.GuideSourceProviders[guideSource.ID].Schedule(channelsToGet)
			if scheduleErr != nil {
				log.WithError(scheduleErr).Errorf("error when updating schedule for provider %s", guideSource.Name)
				return
			}

			for _, programme := range schedule {
				_, programmeErr := cc.API.GuideSourceProgramme.InsertGuideSourceProgramme(guideSource.ID, programme)
				if programmeErr != nil {
					log.WithError(programmeErr).Errorln("error while inserting programmes")
					return
				}
			}

			log.Infof("Completed import of %d programs", len(schedule))

		}
	}

	metrics.ExposedChannels.WithLabelValues(lineup.Name).Set(float64(len(channels)))
	discoveryData := lineup.GetDiscoveryData()

	log.Debugln("creating device xml")
	upnp := discoveryData.UPNP()

	router := newGin()

	router.GET("/", deviceXML(upnp))
	router.GET("/device.xml", deviceXML(upnp))
	router.GET("/discover.json", discovery(discoveryData))
	router.GET("/lineup_status.json", lineupStatus(lineup))
	router.POST("/lineup.post", scanChannels(lineup))
	router.GET("/lineup.json", serveHDHRLineup(hdhrItems))
	router.GET("/lineup.xml", serveHDHRLineup(hdhrItems))
	router.GET("/auto/:channelNumber", stream(cc, lineup))

	baseAddr := fmt.Sprintf("%s:%d", lineup.ListenAddress, lineup.Port)

	if lineup.SSDP {
		if _, ssdpErr := setupSSDP(baseAddr, lineup.Name, lineup.DeviceUUID); ssdpErr != nil {
			log.WithError(ssdpErr).Errorln("telly cannot advertise over ssdp")
		}
	}

	log.Infof(`telly lineup "%s" is live at http://%s/`, lineup.Name, baseAddr)

	srv := &http.Server{
		Addr:    baseAddr,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Panicln("Error starting up web server")
		}
	}()

	for {
		select {
		case <-exit:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.WithError(err).Fatalln("error during tuner shutdown")
			}
			log.Warnln("Tuner restart commanded")
			return
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

type dXMLContainer struct {
	upnp.RootDevice
	XMLName xml.Name `xml:"urn:schemas-upnp-org:device-1-0 root"`
}

func deviceXML(deviceXML upnp.RootDevice) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.XML(http.StatusOK, dXMLContainer{deviceXML, xml.Name{}})
	}
}

func discovery(data models.DiscoveryData) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, data)
	}
}

type hdhrLineupContainer struct {
	XMLName  xml.Name                     `xml:"Lineup"    json:"-"`
	Programs []models.HDHomeRunLineupItem `xml:"Program"`
}

func serveHDHRLineup(hdhrItems []models.HDHomeRunLineupItem) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.String(), ".xml") {
			buf, marshallErr := xml.MarshalIndent(hdhrLineupContainer{Programs: hdhrItems}, "", "\t")
			if marshallErr != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling lineup to XML: %s", marshallErr))
			}
			c.Data(http.StatusOK, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+string(buf)))
			return
		}
		c.JSON(http.StatusOK, hdhrItems)
	}
}

func stream(cc *ccontext.CContext, lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		channel, channelErr := cc.API.LineupChannel.GetLineupChannelByID(lineup.ID, c.Param("channelNumber")[1:])
		if channelErr != nil {
			c.AbortWithError(http.StatusInternalServerError, channelErr)
			return
		}

		log.Infoln("Serving", channel)

		streamUrl, streamUrlErr := cc.VideoSourceProviders[channel.VideoTrack.VideoSourceID].StreamURL(channel.VideoTrack.StreamID, "ts")
		if streamUrlErr != nil {
			c.AbortWithError(http.StatusInternalServerError, streamUrlErr)
			return
		}

		if !viper.IsSet("iptv.ffmpeg") {
			c.Redirect(http.StatusMovedPermanently, streamUrl)
			return
		}

		log.Infoln("Transcoding stream with ffmpeg")

		run := exec.Command("ffmpeg", "-re", "-i", streamUrl, "-codec", "copy", "-bsf:v", "h264_mp4toannexb", "-f", "mpegts", "-tune", "zerolatency", "-progress", "pipe:2", "pipe:1")
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

		metrics.ActiveStreams.WithLabelValues(lineup.Name).Inc()

		go func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Split(split)
			buf := make([]byte, 2)
			scanner.Buffer(buf, bufio.MaxScanTokenSize)

			for scanner.Scan() {
				line := scanner.Text()
				status := processFFMPEGStatus(line)
				if status != nil {
					fmt.Printf("\rFFMPEG Status: channel number: %d bitrate: %s frames: %s total time: %s speed: %s", channel.ID, status.CurrentBitrate, status.FramesProcessed, status.CurrentTime, status.Speed)
				}
			}
		}()

		continueStream := true

		streamVideo := func(w io.Writer) bool {
			defer func() {
				metrics.ActiveStreams.WithLabelValues(lineup.Name).Dec()
				log.Infoln("Stopped streaming", channel.ChannelNumber)
				if killErr := run.Process.Kill(); killErr != nil {
					log.WithError(killErr).Panicln("error when killing ffmpeg")
				}
				continueStream = false
				return
			}()
			if _, copyErr := io.Copy(w, ffmpegout); copyErr != nil {
				log.WithError(copyErr).Errorln("error when streaming from ffmpeg to http")
				continueStream = false
				return false
			}
			return continueStream
		}

		c.Stream(streamVideo)

		return

		c.AbortWithError(http.StatusNotFound, fmt.Errorf("unknown channel number %d", channel.ChannelNumber))
	}
}

func scanChannels(lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		scanAction := c.Query("scan")
		if scanAction == "start" {
			// FIXME: Actually implement a scan...
			// if refreshErr := lineup.Scan(); refreshErr != nil {
			// 	c.AbortWithError(http.StatusInternalServerError, refreshErr)
			// }
			c.AbortWithStatus(http.StatusOK)
			return
		} else if scanAction == "abort" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.String(http.StatusBadRequest, "%s is not a valid scan command", scanAction)
	}
}

func lineupStatus(lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := LineupStatus{
			ScanInProgress: models.ConvertibleBoolean(false),
			ScanPossible:   models.ConvertibleBoolean(true),
			Source:         "Cable",
			SourceList:     []string{"Cable"},
		}
		// FIXME: Implement a scan param on Lineup.
		if false {
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

type FFMPEGStatus struct {
	FramesProcessed string
	CurrentTime     string
	CurrentBitrate  string
	Progress        float64
	Speed           string
}

func processFFMPEGStatus(line string) *FFMPEGStatus {
	status := new(FFMPEGStatus)
	if strings.Contains(line, "frame=") && strings.Contains(line, "time=") && strings.Contains(line, "bitrate=") {
		var re = regexp.MustCompile(`=\s+`)
		st := re.ReplaceAllString(line, `=`)

		f := strings.Fields(st)
		var framesProcessed string
		var currentTime string
		var currentBitrate string
		var currentSpeed string

		for j := 0; j < len(f); j++ {
			field := f[j]
			fieldSplit := strings.Split(field, "=")

			if len(fieldSplit) > 1 {
				fieldname := strings.Split(field, "=")[0]
				fieldvalue := strings.Split(field, "=")[1]

				if fieldname == "frame" {
					framesProcessed = fieldvalue
				}

				if fieldname == "time" {
					currentTime = fieldvalue
				}

				if fieldname == "bitrate" {
					currentBitrate = fieldvalue
				}
				if fieldname == "speed" {
					currentSpeed = fieldvalue
					if currentSpeed == "1x" {
						currentSpeed = "1.000x"
					}
				}
			}
		}

		status.CurrentBitrate = currentBitrate
		status.FramesProcessed = framesProcessed
		status.CurrentTime = currentTime
		status.Speed = currentSpeed
		return status
	}
	return nil
}
