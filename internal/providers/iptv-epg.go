package providers

import (
	"fmt"
	"strconv"
	"strings"

	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

// M3U: http://iptv-epg.com/<random string>.m3u
// XMLTV: http://iptv-epg.com/<random string>.xml

type iptvepg struct {
	BaseConfig Configuration
}

func newIPTVEPG(config *Configuration) (Provider, error) {
	return &iptvepg{*config}, nil
}

func (i *iptvepg) Name() string {
	return "IPTV-EPG"
}

func (i *iptvepg) PlaylistURL() string {
	return fmt.Sprintf("http://iptv-epg.com/%s.m3u", i.BaseConfig.Username)
}

func (i *iptvepg) EPGURL() string {
	return fmt.Sprintf("http://iptv-epg.com/%s.xml", i.BaseConfig.Password)
}

// ParseTrack matches the provided M3U track an XMLTV channel and returns a ProviderChannel.
func (i *iptvepg) ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error) {
	channelVal := track.Tags["tvg-chno"]
	if i.BaseConfig.ChannelNumberKey != "" {
		channelVal = track.Tags[i.BaseConfig.ChannelNumberKey]
	}

	channelNumber, channelNumberErr := strconv.Atoi(channelVal)
	if channelNumberErr != nil {
		return nil, channelNumberErr
	}

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
		Number:       channelNumber,
		StreamURL:    track.URI,
		StreamID:     channelNumber,
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

func (i *iptvepg) ProcessProgramme(programme xmltv.Programme) *xmltv.Programme {
	return &programme
}

func (i *iptvepg) Configuration() Configuration {
	return i.BaseConfig
}

func (i *iptvepg) RegexKey() string {
	return "group-title"
}
