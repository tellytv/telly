package providers

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tombowditch/telly/m3u"
)

var channelNumberExtractor = regexp.MustCompile(`/(\d+).(ts|.*.m3u8)`).FindAllStringSubmatch

type Configuration struct {
	Name     string `json:"-"`
	Provider string

	Username string `json:"username"`
	Password string `json:"password"`

	M3U string `json:"-"`
	EPG string `json:"-"`

	VideoOnDemand bool `json:"-"`
}

func (i *Configuration) GetProvider() (Provider, error) {
	switch strings.ToLower(i.Provider) {
	case "vaders":
		log.Infoln("Source is vaders!")
		return newVaders(i)
	case "custom":
	default:
		log.Infoln("source is either custom or unknown, assuming custom!")
	}
	return nil, nil
}

// ProviderChannel describes a channel available in the providers lineup with necessary pieces parsed into fields.
type ProviderChannel struct {
	Name         string
	InternalID   int // Should be the integer just before .ts.
	Number       *int
	Logo         string
	StreamURL    string
	HD           bool
	Quality      string
	OnDemand     bool
	StreamFormat string
}

// Provider describes a IPTV provider configuration.
type Provider interface {
	Name() string
	PlaylistURL() string
	EPGURL() string

	// These are functions to extract information from playlists.
	ParseLine(line m3u.Track) (*ProviderChannel, error)

	AuthenticatedStreamURL(channel *ProviderChannel) string

	MatchPlaylistKey() string
}

// UnmarshalProviders takes V, a slice of Configuration and transforms it into a slice of Provider.
func UnmarshalProviders(v interface{}) ([]Provider, error) {
	providers := make([]Provider, 0)

	uncasted, ok := v.([]interface{})
	if !ok {
		panic(fmt.Errorf("provided slice is not of type []Configuration, it is of type %T", v))
	}

	for _, uncastedProvider := range uncasted {
		ipProvider := uncastedProvider.(Configuration)
		log.Infof("ipProvider %+v", ipProvider)
	}

	return providers, nil
}

// func testProvider() {
// 	v, vErr := NewVadersTV("hunter1", "hunter2", false)
// 	if vErr != nil {
// 		log.WithError(vErr).Errorf("Error setting up %s", v.Name())
// 	}
// 	log.Infoln("Provider name is", v.Name())
// 	log.Infoln("Playlist URL is", v.PlaylistURL())
// 	log.Infoln("EPG URL is", v.EPGURL())
// 	log.Infof("Stream URL is %+v", v.AuthenticatedStreamURL(&ProviderChannel{
// 		Name:       "Test channel",
// 		InternalID: 2862,
// 	}))

// 	return
// }
