package providers

import (
	"fmt"
	"strings"

	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

// http://irislinks.net:83/get.php?username=username&password=password&type=m3uplus&output=ts
// http://irislinks.net:83/xmltv.php?username=username&password=password

type iris struct {
	BaseConfig Configuration
}

func newIris(config *Configuration) (Provider, error) {
	return &iris{*config}, nil
}

func (i *iris) Name() string {
	return "Iris"
}

func (i *iris) PlaylistURL() string {
	return fmt.Sprintf("http://irislinks.net:83/get.php?username=%s&password=%s&type=m3u_plus&output=ts", i.BaseConfig.Username, i.BaseConfig.Password)
}

func (i *iris) EPGURL() string {
	return fmt.Sprintf("http://irislinks.net:83/xmltv.php?username=%s&password=%s", i.BaseConfig.Username, i.BaseConfig.Password)
}

// ParseTrack matches the provided M3U track an XMLTV channel and returns a ProviderChannel.
func (i *iris) ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error) {
	nameVal := track.Name
	if i.BaseConfig.NameKey != "" {
		nameVal = track.Tags[i.BaseConfig.NameKey]
	}

	logoVal := track.Tags["tvg-logo"]
	if i.BaseConfig.LogoKey != "" {
		logoVal = track.Tags[i.BaseConfig.LogoKey]
	}

	pChannel := &ProviderChannel{
		Name:         nameVal,
		Logo:         logoVal,
		Number:       0,
		StreamURL:    track.URI,
		StreamID:     0,
		HD:           strings.Contains(strings.ToLower(track.Name), "hd"),
		StreamFormat: "Unknown",
		Track:        track,
		OnDemand:     false,
	}

	epgVal := track.Tags["tvg-id"]
	if i.BaseConfig.EPGMatchKey != "" {
		epgVal = track.Tags[i.BaseConfig.EPGMatchKey]
	}

	if xmlChan, ok := channelMap[epgVal]; ok {
		pChannel.EPGMatch = epgVal
		pChannel.EPGChannel = &xmlChan
	}

	return pChannel, nil
}

func (i *iris) ProcessProgramme(programme xmltv.Programme) *xmltv.Programme {
	return &programme
}

func (i *iris) Configuration() Configuration {
	return i.BaseConfig
}

func (i *iris) RegexKey() string {
	return "group-title"
}
