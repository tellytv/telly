package providers

import (
	"fmt"
	"strings"

	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

// http://iptv-area-51.tv:2095/get.php?username=username&password=password&type=m3uplus&output=ts
// http://iptv-area-51.tv:2095/xmltv.php?username=username&password=password

type area51 struct {
	BaseConfig Configuration
}

func newArea51(config *Configuration) (Provider, error) {
	return &area51{*config}, nil
}

func (i *area51) Name() string {
	return "Area51"
}

func (i *area51) PlaylistURL() string {
	return fmt.Sprintf("http://iptv-area-51.tv:2095/get.php?username=%s&password=%s&type=m3u_plus&output=ts", i.BaseConfig.Username, i.BaseConfig.Password)
}

func (i *area51) EPGURL() string {
	return fmt.Sprintf("http://iptv-area-51.tv:2095/xmltv.php?username=%s&password=%s", i.BaseConfig.Username, i.BaseConfig.Password)
}

// ParseTrack matches the provided M3U track an XMLTV channel and returns a ProviderChannel.
func (i *area51) ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error) {
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

func (i *area51) ProcessProgramme(programme xmltv.Programme) *xmltv.Programme {
	return &programme
}

func (i *area51) Configuration() Configuration {
	return i.BaseConfig
}

func (i *area51) RegexKey() string {
	return "group-title"
}
