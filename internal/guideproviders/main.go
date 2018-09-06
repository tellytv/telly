// Package guideproviders is a telly internal package to provide electronic program guide (EPG) data.
// It is generally modeled after the XMLTV standard with slight deviations to accommodate other providers.
package guideproviders

import (
	"strings"

	"github.com/tellytv/telly/internal/xmltv"
)

// Configuration is the basic configuration struct for guideproviders with generic values for specific providers.
type Configuration struct {
	Name     string `json:"-"`
	Provider string

	// Only used for Schedules Direct provider
	Username string
	Password string
	Lineups  []string

	// Only used for XMLTV provider
	XMLTVURL string
}

// GetProvider returns an initialized GuideProvider for the Configuration.
func (i *Configuration) GetProvider() (GuideProvider, error) {
	switch strings.ToLower(i.Provider) {
	case "schedulesdirect", "schedules-direct", "sd":
		return newSchedulesDirect(i)
	default:
		return newXMLTV(i)
	}
}

// Channel describes a channel available in the providers lineup with necessary pieces parsed into fields.
type Channel struct {
	// Required Fields
	ID     string `json:",omitempty"`
	Name   string `json:",omitempty"`
	Logos  []Logo `json:",omitempty"`
	Number string `json:",omitempty"`

	// Optional fields
	CallSign  string   `json:",omitempty"`
	URLs      []string `json:",omitempty"`
	Lineup    string   `json:",omitempty"`
	Affiliate string   `json:",omitempty"`

	ProviderData interface{} `json:",omitempty"`
}

// XMLTV returns the xmltv.Channel representation of the Channel.
func (c *Channel) XMLTV() xmltv.Channel {
	ch := xmltv.Channel{
		ID:   c.ID,
		LCN:  c.Number,
		URLs: c.URLs,
	}

	// Why do we do this? From tv_grab_zz_sdjson:
	//
	// MythTV seems to assume that the first three display-name elements are
	// name, callsign and channel number. We follow that scheme here.
	ch.DisplayNames = []xmltv.CommonElement{
		{
			Value: c.Name,
		},
		{
			Value: c.CallSign,
		},
		{
			Value: c.Number,
		},
	}

	for _, logo := range c.Logos {
		ch.Icons = append(ch.Icons, xmltv.Icon{
			Source: logo.URL,
			Width:  logo.Width,
			Height: logo.Height,
		})
	}

	return ch
}

// A Logo stores the information about a channel logo
type Logo struct {
	URL    string `json:"URL"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// ProgrammeContainer contains information about a single provider in the XMLTV format
// as well as provider specific data.
type ProgrammeContainer struct {
	Programme    xmltv.Programme
	ProviderData interface{}
}

// AvailableLineup is a lineup that a user can subscribe to.
type AvailableLineup struct {
	Location   string
	Transport  string
	Name       string
	ProviderID string
}

// CoverageArea describes a region that a provider supports.
type CoverageArea struct {
	RegionName        string `json:",omitempty"`
	FullName          string `json:",omitempty"`
	PostalCode        string `json:",omitempty"`
	PostalCodeExample string `json:",omitempty"`
	ShortName         string `json:",omitempty"`
	OnePostalCode     bool   `json:",omitempty"`
}

// GuideProvider describes a IPTV provider configuration.
type GuideProvider interface {
	Name() string
	Channels() ([]Channel, error)
	Schedule(daysToGet int, inputChannels []Channel, inputProgrammes []ProgrammeContainer) (map[string]interface{}, []ProgrammeContainer, error)

	Refresh(lineupStateJSON []byte) ([]byte, error)
	Configuration() Configuration

	// Schedules Direct specific functions that others might someday use.
	SupportsLineups() bool
	LineupCoverage() ([]CoverageArea, error)
	AvailableLineups(countryCode, postalCode string) ([]AvailableLineup, error)
	PreviewLineupChannels(lineupID string) ([]Channel, error)
	SubscribeToLineup(lineupID string) (interface{}, error)
	UnsubscribeFromLineup(providerID string) error
}
