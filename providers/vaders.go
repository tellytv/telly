package providers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tombowditch/telly/m3u"
)

// M3U: http://api.vaders.tv/vget?username=xxx&password=xxx&format=ts
// XMLTV: http://vaders.tv/p2.xml

type vader struct {
	provider Configuration

	Token string `json:"-"`
}

func newVaders(config *Configuration) (Provider, error) {
	tok, tokErr := json.Marshal(config)
	if tokErr != nil {
		return nil, tokErr
	}

	return &vader{*config, base64.StdEncoding.EncodeToString(tok)}, nil
}

func (v *vader) Name() string {
	return "Vaders.tv"
}

func (v *vader) PlaylistURL() string {
	return fmt.Sprintf("http://api.vaders.tv/vget?username=%s&password=%s&vod=%t&format=ts", v.provider.Username, v.provider.Password, v.provider.VideoOnDemand)
}

func (v *vader) EPGURL() string {
	return "http://vaders.tv/p2.xml"
}

func (v *vader) ParseLine(line m3u.Track) (*ProviderChannel, error) {
	streamURL := channelNumberExtractor(line.URI, -1)[0]
	channelID, channelIDErr := strconv.Atoi(streamURL[1])
	if channelIDErr != nil {
		return nil, channelIDErr
	}

	// http://vapi.vaders.tv/play/dvr/${start}/TSID.ts?duration=3600&token=
	// http://vapi.vaders.tv/play/TSID.ts?token=
	// http://vapi.vaders.tv/play/vod/VODID.mp4.m3u8?token=
	// http://vapi.vaders.tv/play/vod/VODID.avi.m3u8?token=
	// http://vapi.vaders.tv/play/vod/VODID.mkv.m3u8?token=

	return &ProviderChannel{
		Name:         line.Tags["tvg-name"],
		Logo:         line.Tags["tvg-logo"],
		StreamURL:    line.URI,
		InternalID:   channelID,
		HD:           strings.Contains(strings.ToLower(line.Tags["tvg-name"]), "hd"),
		StreamFormat: streamURL[2],
	}, nil
}

func (v *vader) AuthenticatedStreamURL(channel *ProviderChannel) string {
	return fmt.Sprintf("http://vapi.vaders.tv/play/%d.ts?token=%s", channel.InternalID, v.Token)
}

func (v *vader) MatchPlaylistKey() string {
	return "tvg-id"
}
