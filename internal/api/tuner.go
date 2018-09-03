package api

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	upnp "github.com/NebulousLabs/go-upnp/goupnp"
	"github.com/gin-gonic/gin"
	"github.com/koron/go-ssdp"
	uuid "github.com/satori/go.uuid"
	ccontext "github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/metrics"
	"github.com/tellytv/telly/internal/models"
	"github.com/tellytv/telly/internal/streamsuite"
)

// ServeLineup starts up a server dedicated to a single Lineup.
func ServeLineup(cc *ccontext.CContext, exit chan bool, lineup *models.Lineup) {
	channels, channelsErr := cc.API.LineupChannel.GetChannelsForLineup(lineup.ID, true)
	if channelsErr != nil {
		log.WithError(channelsErr).Errorln("error getting channels in lineup")
		return
	}

	hdhrItems := make([]models.HDHomeRunLineupItem, 0)
	for _, channel := range channels {
		hdhrItems = append(hdhrItems, *channel.HDHR)
		metrics.ExposedChannels.WithLabelValues(lineup.Name, channel.VideoTrack.VideoSource.Name, channel.VideoTrack.VideoSource.Provider).Inc()
	}

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
		if ssdpErr := setupSSDP(baseAddr, lineup.Name, lineup.DeviceUUID, exit); ssdpErr != nil {
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

func setupSSDP(baseAddress, deviceName, deviceUUID string, exit chan bool) error {
	log.Debugf("Advertising telly as %s (%s) on %s", deviceName, deviceUUID, baseAddress)

	adv, err := ssdp.Advertise(
		ssdp.RootDevice,
		fmt.Sprintf("uuid:%s::upnp:rootdevice", deviceUUID),
		fmt.Sprintf("http://%s/device.xml", baseAddress),
		`telly/2.0 UPnP/1.0`,
		1800)

	if err != nil {
		return err
	}

	go func() {
		aliveTick := time.Tick(300 * time.Second)

	loop:
		for {
			select {
			case <-exit:
				break loop
			case <-aliveTick:
				log.Debugln("Sending SSDP heartbeat")
				if err := adv.Alive(); err != nil {
					log.WithError(err).Panicln("error when sending ssdp heartbeat")
				}
			}
		}

		adv.Bye()
		adv.Close()
	}()

	return nil
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

// NewStreamStatus creates a new stream status
func NewStreamStatus(cc *ccontext.CContext, lineup *models.Lineup, channelID string) (*streamsuite.Stream, string, error) {
	statusUUID := uuid.Must(uuid.NewV4()).String()
	ss := &streamsuite.Stream{
		UUID: statusUUID,
	}
	channel, channelErr := cc.API.LineupChannel.GetLineupChannelByID(lineup.ID, channelID)
	if channelErr != nil {
		if channelErr == sql.ErrNoRows {
			return nil, statusUUID, fmt.Errorf("unknown channel number %s", channelID)
		}
		return nil, statusUUID, channelErr
	}

	ss.Channel = channel

	streamURL, streamURLErr := cc.VideoSourceProviders[channel.VideoTrack.VideoSourceID].StreamURL(channel.VideoTrack.StreamID, "ts")
	if streamURLErr != nil {
		return nil, statusUUID, streamURLErr
	}

	ss.StreamURL = streamURL

	if lineup.StreamTransport == "ffmpeg" {
		ss.Transport = streamsuite.FFMPEG{}
	} else {
		ss.Transport = streamsuite.HTTP{}
	}

	ss.PromLabels = []string{lineup.Name, channel.VideoTrack.VideoSource.Name, channel.VideoTrack.VideoSource.Provider, channel.Title, ss.Transport.Type()}

	return ss, statusUUID, nil
}

func stream(cc *ccontext.CContext, lineup *models.Lineup) gin.HandlerFunc {
	return func(c *gin.Context) {
		stream, streamUUID, streamErr := NewStreamStatus(cc, lineup, c.Param("channelNumber")[1:])
		if streamErr != nil {
			log.WithError(streamErr).Errorf("Error when starting streaming")
			c.AbortWithError(http.StatusInternalServerError, streamErr)
			return
		}

		cc.Streams[streamUUID] = stream

		log.Infof("Serving via %s: %s", stream.Transport.Type(), stream.Channel)

		stream.Start(c)

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
