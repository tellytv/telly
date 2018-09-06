// Package videoproviders is a telly internal package to provide video stream information.
package videoproviders

import (
	"regexp"
	"strings"
)

var streamNumberRegex = regexp.MustCompile(`/(\d+).(ts|.*.m3u8)`).FindAllStringSubmatch

// var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
// var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
// var hdRegex = regexp.MustCompile(`hd|4k`)

// Configuration is the basic configuration struct for videoproviders with generic values for specific providers.
type Configuration struct {
	Name     string `json:"-"`
	Provider string

	// Only used for Xtream provider
	Username string
	Password string
	BaseURL  string

	// Only used for M3U provider
	M3UURL      string
	NameKey     string
	LogoKey     string
	CategoryKey string
	EPGIDKey    string
}

// GetProvider returns an initialized VideoProvider for the Configuration.
func (i *Configuration) GetProvider() (VideoProvider, error) {
	switch strings.ToLower(i.Provider) {
	case "xtream", "xstream":
		return newXtreamCodes(i)
	default:
		return newM3U(i)
	}
}

// Category describes a grouping of streams.
type Category struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ChannelType is used for enumerating the ChannelType field in Channel.
type ChannelType string

const (
	// LiveStream is the constant describing a live stream.
	LiveStream ChannelType = "live"
	// VODStream is the constant describing a video on demand stream.
	VODStream = "vod"
	// SeriesStream is the constant describing a TV series stream.
	SeriesStream = "series"
)

// Channel describes a channel available in the providers lineup with necessary pieces parsed into fields.
type Channel struct {
	Name     string
	StreamID int
	Logo     string
	Type     ChannelType
	Category string
	EPGID    string

	// Only needed for M3U provider
	streamURL string
}

// VideoProvider describes a IPTV provider configuration.
type VideoProvider interface {
	Name() string
	Categories() ([]Category, error)
	Formats() ([]string, error)
	Channels() ([]Channel, error)
	StreamURL(streamID int, wantedFormat string) (string, error)

	Refresh() error
	Configuration() Configuration
}
