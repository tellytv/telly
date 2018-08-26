package video_providers

import (
	"fmt"

	xc "github.com/tellytv/go.xtream-codes"
)

type XtreamCodes struct {
	BaseConfig Configuration

	client xc.XtreamClient

	categories map[int]xc.Category
	streams    map[int]xc.Stream
	channels   []Channel
}

func newXtreamCodes(config *Configuration) (VideoProvider, error) {
	xc := &XtreamCodes{BaseConfig: *config}
	if loadErr := xc.Refresh(); loadErr != nil {
		return nil, loadErr
	}
	return xc, nil
}

func (x *XtreamCodes) Name() string {
	return "Xtream Codes Server"
}

func (x *XtreamCodes) Categories() ([]Category, error) {
	outputCats := make([]Category, 0)
	for _, cat := range x.categories {
		outputCats = append(outputCats, Category{
			Name: cat.Name,
			Type: cat.Type,
		})
	}
	return outputCats, nil
}

func (x *XtreamCodes) Formats() ([]string, error) {
	return x.client.UserInfo.AllowedOutputFormats, nil
}

func (x *XtreamCodes) Channels() ([]Channel, error) {
	return x.channels, nil
}

func (x *XtreamCodes) StreamURL(streamID int, wantedFormat string) (string, error) {
	return x.client.GetStreamURL(streamID, wantedFormat)
}

func (x *XtreamCodes) Refresh() error {
	client, clientErr := xc.NewClient(x.BaseConfig.Username, x.BaseConfig.Password, x.BaseConfig.BaseURL)
	if clientErr != nil {
		return fmt.Errorf("error creating xtream codes client: %s", clientErr)
	}
	x.client = *client

	if x.categories == nil {
		x.categories = make(map[int]xc.Category)
	}

	if x.streams == nil {
		x.streams = make(map[int]xc.Stream)
	}

	for _, xType := range []string{"live", "vod", "series"} {
		cats, catsErr := x.client.GetCategories(xType)
		if catsErr != nil {
			return fmt.Errorf("error getting %s categories: %s", xType, catsErr)
		}
		for _, cat := range cats {
			x.categories[cat.ID] = cat
		}

		streams, streamsErr := x.client.GetStreams(xType, "")
		if streamsErr != nil {
			return fmt.Errorf("error getting %s streams: %s", xType, streamsErr)
		}
		for _, stream := range streams {
			x.streams[stream.ID] = stream
		}
	}

	for _, stream := range x.streams {
		categoryName := ""
		if val, ok := x.categories[stream.CategoryID]; ok {
			categoryName = val.Name
		}
		x.channels = append(x.channels, Channel{
			Name:     stream.Name,
			StreamID: stream.ID,
			Logo:     stream.Icon,
			Type:     ChannelType(stream.Type),
			Category: categoryName,
			EPGID:    stream.EPGChannelID,
		})
	}

	return nil
}

func (x *XtreamCodes) Configuration() Configuration {
	return x.BaseConfig
}
