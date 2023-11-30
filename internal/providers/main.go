package providers

import (
	"regexp"
	"strings"

	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

var streamNumberRegex = regexp.MustCompile(`/(\d+).(ts|.*.m3u8)`).FindAllStringSubmatch
var channelNumberRegex = regexp.MustCompile(`^[0-9]+[[:space:]]?$`).MatchString
var callSignRegex = regexp.MustCompile(`^[A-Z0-9]+$`).MatchString
var hdRegex = regexp.MustCompile(`hd|4k`)

type Configuration struct {
	Name     string `json:"-"`
	Provider string

	Username string `json:"username"`
	Password string `json:"password"`

	M3U string `json:"-"`
	EPG string `json:"-"`

	Udpxy string `json:"udpxy"`

	VideoOnDemand bool `json:"-"`

	Filter    string
	FilterKey string
	FilterRaw bool

	SortKey     string
	SortReverse bool

	Favorites   []string
	FavoriteTag string

	IncludeOnly    []string
	IncludeOnlyTag string

	CacheFiles bool

	NameKey          string
	LogoKey          string
	ChannelNumberKey string
	EPGMatchKey      string
}

func (i *Configuration) GetProvider() (Provider, error) {
	switch strings.ToLower(i.Provider) {
	default:
		return newCustomProvider(i)
	}
}

// ProviderChannel describes a channel available in the providers lineup with necessary pieces parsed into fields.
type ProviderChannel struct {
	Name         string
	StreamID     int // Should be the integer just before .ts.
	Number       int
	Logo         string
	StreamURL    string
	HD           bool
	Quality      string
	OnDemand     bool
	StreamFormat string
	Favorite     bool

	EPGMatch      string
	EPGChannel    *xmltv.Channel
	EPGProgrammes []xmltv.Programme
	Track         m3u.Track
}

// Provider describes a IPTV provider configuration.
type Provider interface {
	Name() string
	PlaylistURL() string
	EPGURL() string

	// These are functions to extract information from playlists.
	ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error)
	ProcessProgramme(programme xmltv.Programme) *xmltv.Programme

	RegexKey() string
	Configuration() Configuration
}

func contains(s []string, e string) bool {
	for _, ss := range s {
		if e == ss {
			return true
		}
	}
	return false
}
