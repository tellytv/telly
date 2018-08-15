package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"regexp"
)

type config struct {
	Filter struct {
		RegexInclusive bool           `toml:"Filter.RegexInclusive"`
		Regex          *regexp.Regexp `toml:"-"`
		RegexStr       string         `toml:"Filter.Regex"`
	}

	IPTV struct {
		Playlists           []string `toml:"IPTV.Playlists"`
		ConcurrentStreams   int      `toml:"IPTV.ConcurrentStreams"`
		StartingChannel     int      `toml:"IPTV.StartingChannel"`
		XMLTVChannelNumbers bool     `toml:"IPTV.XMLTVChannelNumbers"`
	}

	Discovery struct {
		DeviceAuth      string `toml:"Discovery.DeviceAuth"`
		DeviceID        int    `toml:"Discovery.DeviceID"`
		DeviceUUID      string `toml:"Discovery.DeviceUUID"`
		FriendlyName    string `toml:"Discovery.FriendlyName"`
		Manufacturer    string `toml:"Discovery.Manufacturer"`
		ModelNumber     string `toml:"Discovery.ModelNumber"`
		FirmwareName    string `toml:"Discovery.FirmwareName"`
		FirmwareVersion string `toml:"Discovery.FirmwareVersion"`
		SSDP            bool   `toml:"Discovery.SSDP"`
	}

	Log struct {
		LogRequests bool   `toml:"Log.Requests"`
		Level       string `toml:"Log.Level"`
	}

	Web struct {
		ListenAddress    *net.TCPAddr `toml:"-"`
		BaseAddress      *net.TCPAddr `toml:"-"`
		ListenAddressStr string       `toml:"Web.ListenAddress"`
		BaseAddressStr   string       `toml:"Web.BaseAddress"`
	}

	lineup *Lineup
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
	ScanInProgress convertibleBoolean
	ScanPossible   convertibleBoolean `json:",omitempty"`
	Source         string             `json:",omitempty"`
	SourceList     []string           `json:",omitempty"`
	Progress       int                `json:",omitempty"` // Percent complete
	Found          int                `json:",omitempty"` // Number of found channels
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

type convertibleBoolean bool

func (bit *convertibleBoolean) MarshalJSON() ([]byte, error) {
	var bitSetVar int8
	if *bit {
		bitSetVar = 1
	}

	return json.Marshal(bitSetVar)
}

func (bit *convertibleBoolean) UnmarshalJSON(data []byte) error {
	asString := string(data)
	if asString == "1" || asString == "true" {
		*bit = true
	} else if asString == "0" || asString == "false" {
		*bit = false
	} else {
		return fmt.Errorf("Boolean unmarshal error: invalid input %s", asString)
	}
	return nil
}
