package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/models"
)

func scanM3U(c *gin.Context) {
	reader, m3uErr := models.GetM3U(c.Query("m3u_url"), false)
	if m3uErr != nil {
		log.WithError(m3uErr).Errorln("unable to get m3u file")
		c.AbortWithError(http.StatusBadRequest, m3uErr)
		return
	}

	rawPlaylist, err := m3uplus.Decode(reader)
	if err != nil {
		log.WithError(err).Errorln("unable to parse m3u file")
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	c.JSON(http.StatusOK, rawPlaylist)
}

func scanXMLTV(c *gin.Context) {
	epg, epgErr := models.GetXMLTV(c.Query("epg_url"), false)
	if epgErr != nil {
		c.AbortWithError(http.StatusInternalServerError, epgErr)
		return
	}

	epg.Programmes = nil

	c.JSON(http.StatusOK, epg)
}

// DiscoveryData contains data about telly to expose in the HDHomeRun format for Plex detection.
type DiscoveryData struct {
	FriendlyName    string
	Manufacturer    string
	ModelNumber     string
	FirmwareName    string
	TunerCount      int
	FirmwareVersion string
	DeviceID        string
	DeviceAuth      string
	BaseURL         string
	LineupURL       string
}

// UPNP returns the UPNP representation of the DiscoveryData.
func (d *DiscoveryData) UPNP() UPNP {
	return UPNP{
		SpecVersion: upnpVersion{
			Major: 1, Minor: 0,
		},
		URLBase: d.BaseURL,
		Device: upnpDevice{
			DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
			FriendlyName: d.FriendlyName,
			Manufacturer: d.Manufacturer,
			ModelName:    d.ModelNumber,
			ModelNumber:  d.ModelNumber,
			UDN:          fmt.Sprintf("uuid:%s", d.DeviceID),
		},
	}
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

type upnpVersion struct {
	Major int32 `xml:"major"`
	Minor int32 `xml:"minor"`
}

type upnpDevice struct {
	DeviceType   string `xml:"deviceType"`
	FriendlyName string `xml:"friendlyName"`
	Manufacturer string `xml:"manufacturer"`
	ModelName    string `xml:"modelName"`
	ModelNumber  string `xml:"modelNumber"`
	SerialNumber string `xml:"serialNumber"`
	UDN          string `xml:"UDN"`
}

// UPNP describes the UPNP/SSDP XML.
type UPNP struct {
	XMLName     xml.Name    `xml:"urn:schemas-upnp-org:device-1-0 root"`
	SpecVersion upnpVersion `xml:"specVersion"`
	URLBase     string      `xml:"URLBase"`
	Device      upnpDevice  `xml:"device"`
}

func GetDiscoveryData() DiscoveryData {
	return DiscoveryData{
		FriendlyName:    viper.GetString("discovery.device-friendly-name"),
		Manufacturer:    viper.GetString("discovery.device-manufacturer"),
		ModelNumber:     viper.GetString("discovery.device-model-number"),
		FirmwareName:    viper.GetString("discovery.device-firmware-name"),
		TunerCount:      viper.GetInt("iptv.streams"),
		FirmwareVersion: viper.GetString("discovery.device-firmware-version"),
		DeviceID:        strconv.Itoa(viper.GetInt("discovery.device-id")),
		DeviceAuth:      viper.GetString("discovery.device-auth"),
		BaseURL:         fmt.Sprintf("http://%s", viper.GetString("web.base-address")),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", viper.GetString("web.base-address")),
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
