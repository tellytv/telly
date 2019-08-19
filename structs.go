package main

import (
	"encoding/xml"
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/zenjabba/telly/m3u"
)

type config struct {
	RegexInclusive bool
	Regex          *regexp.Regexp

	DirectMode        bool
	M3UPath           string
	ConcurrentStreams int
	StartingChannel   int

	DeviceAuth      string
	DeviceID        int
	DeviceUUID      string
	FriendlyName    string
	Manufacturer    string
	ModelNumber     string
	FirmwareName    string
	FirmwareVersion string
	SSDP            bool

	LogRequests bool
	LogLevel    string

	ListenAddress *net.TCPAddr
	BaseAddress   *net.TCPAddr

	lineup []LineupItem
}

func (c *config) DiscoveryData() DiscoveryData {
	return DiscoveryData{
		FriendlyName:    c.FriendlyName,
		Manufacturer:    c.Manufacturer,
		ModelNumber:     c.ModelNumber,
		FirmwareName:    c.FirmwareName,
		TunerCount:      c.ConcurrentStreams,
		FirmwareVersion: c.FirmwareVersion,
		DeviceID:        strconv.Itoa(c.DeviceID),
		DeviceAuth:      c.DeviceAuth,
		BaseURL:         fmt.Sprintf("http://%s", c.BaseAddress),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", c.BaseAddress),
	}
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
	ScanInProgress int
	ScanPossible   int
	Source         string
	SourceList     []string
}

// LineupItem is a single channel found in the playlist.
type LineupItem struct {
	GuideNumber string
	GuideName   string
	URL         string
}

// Track describes a single M3U segment. This struct includes m3u.Track as well as specific IPTV fields we want to get.
type Track struct {
	*m3u.Track
	Catchup       string `m3u:"catchup"`
	CatchupDays   string `m3u:"catchup-days"`
	CatchupSource string `m3u:"catchup-source"`
	GroupTitle    string `m3u:"group-title"`
	TvgID         string `m3u:"tvg-id"`
	TvgLogo       string `m3u:"tvg-logo"`
	TvgName       string `m3u:"tvg-name"`
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
