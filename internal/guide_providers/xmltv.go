package guide_providers

import (
	"fmt"

	"github.com/tellytv/telly/internal/xmltv"
	"github.com/tellytv/telly/utils"
)

type XMLTV struct {
	BaseConfig Configuration

	channels []Channel
	file     *xmltv.TV
}

func newXMLTV(config *Configuration) (GuideProvider, error) {
	provider := &XMLTV{BaseConfig: *config}

	if loadErr := provider.Refresh(); loadErr != nil {
		return nil, loadErr
	}

	return provider, nil
}

func (x *XMLTV) Name() string {
	return "XMLTV"
}

func (x *XMLTV) Channels() ([]Channel, error) {
	return x.channels, nil
}

func (x *XMLTV) Schedule(channelIDs []string) ([]xmltv.Programme, error) {
	channelIDMap := make(map[string]struct{})
	for _, chanID := range channelIDs {
		channelIDMap[chanID] = struct{}{}
	}

	filteredProgrammes := make([]xmltv.Programme, 0)

	for _, programme := range x.file.Programmes {
		if _, ok := channelIDMap[programme.Channel]; ok {
			filteredProgrammes = append(filteredProgrammes, programme)
		}
	}

	return filteredProgrammes, nil
}

func (x *XMLTV) Refresh() error {
	xTV, xTVErr := utils.GetXMLTV(x.BaseConfig.XMLTVURL, false)
	if xTVErr != nil {
		return fmt.Errorf("error when getting XMLTV file: %s", xTVErr)
	}

	x.file = xTV

	for _, channel := range xTV.Channels {
		logos := make([]Logo, 0)

		for _, icon := range channel.Icons {
			logos = append(logos, Logo{
				URL:    icon.Source,
				Height: icon.Height,
				Width:  icon.Width,
			})
		}

		x.channels = append(x.channels, Channel{
			ID:       channel.ID,
			Name:     channel.DisplayNames[0].Value,
			Logos:    logos,
			Number:   channel.LCN,
			CallSign: "UNK",
		})
	}

	return nil
}

func (x *XMLTV) Configuration() Configuration {
	return x.BaseConfig
}
