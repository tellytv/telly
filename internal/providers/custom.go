package providers

import (
	"strconv"
	"strings"

	m3u "github.com/tombowditch/telly/internal/m3uplus"
	"github.com/tombowditch/telly/internal/xmltv"
)

type customProvider struct {
	BaseConfig Configuration
}

func newCustomProvider(config *Configuration) (Provider, error) {
	return &customProvider{*config}, nil
}

func (i *customProvider) Name() string {
	return i.BaseConfig.Name
}

func (i *customProvider) PlaylistURL() string {
	return i.BaseConfig.M3U
}

func (i *customProvider) EPGURL() string {
	return i.BaseConfig.EPG
}

// ParseTrack matches the provided M3U track an XMLTV channel and returns a ProviderChannel.
func (i *customProvider) ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error) {
	channelVal := track.Tags["tvg-chno"]
	if i.BaseConfig.ChannelNumberKey != "" {
		channelVal = track.Tags[i.BaseConfig.ChannelNumberKey]
	}

	chanNum := 0

	if channelNumber, channelNumberErr := strconv.Atoi(channelVal); channelNumberErr == nil {
		chanNum = channelNumber
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
		Number:       chanNum,
		StreamURL:    track.URI,
		StreamID:     chanNum,
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

func (i *customProvider) ProcessProgramme(programme xmltv.Programme) *xmltv.Programme {
	return &programme
}

func (i *customProvider) Configuration() Configuration {
	return i.BaseConfig
}

func (i *customProvider) RegexKey() string {
	return i.BaseConfig.FilterKey
}
