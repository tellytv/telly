package providers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	m3u "github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

// This regex matches and extracts the following URLs.
// http://vapi.vaders.tv/play/dvr/${start}/123.ts?duration=3600&token=
// http://vapi.vaders.tv/play/123.ts?token=
// http://vapi.vaders.tv/play/vod/123.mp4.m3u8?token=
// http://vapi.vaders.tv/play/vod/123.avi.m3u8?token=
// http://vapi.vaders.tv/play/vod/123.mkv.m3u8?token=
var vadersURL = regexp.MustCompile(`/(vod/|dvr/\${start}/)?(\d+).(ts|.*.m3u8)\?(duration=\d+&)?token=`).FindAllStringSubmatch

// M3U: http://api.vaders.tv/vget?username=xxx&password=xxx&format=ts
// XMLTV: http://vaders.tv/p2.xml

type vader struct {
	BaseConfig Configuration

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
	return fmt.Sprintf("http://api.vaders.tv/vget?username=%s&password=%s&vod=%t&format=ts", v.BaseConfig.Username, v.BaseConfig.Password, v.BaseConfig.VideoOnDemand)
}

func (v *vader) EPGURL() string {
	return "http://vaders.tv/p2.xml.gz"
}

// ParseTrack matches the provided M3U track an XMLTV channel and returns a ProviderChannel.
func (v *vader) ParseTrack(track m3u.Track, channelMap map[string]xmltv.Channel) (*ProviderChannel, error) {
	streamURL := vadersURL(track.URI, -1)[0]

	vod := strings.Contains(streamURL[1], "vod")

	if v.BaseConfig.VideoOnDemand == false && vod {
		return nil, nil
	}

	channelID, channelIDErr := strconv.Atoi(streamURL[2])
	if channelIDErr != nil {
		return nil, channelIDErr
	}

	nameVal := track.Tags["tvg-name"]
	if v.BaseConfig.NameKey != "" {
		nameVal = track.Tags[v.BaseConfig.NameKey]
	}

	logoVal := track.Tags["tvg-logo"]
	if v.BaseConfig.LogoKey != "" {
		logoVal = track.Tags[v.BaseConfig.LogoKey]
	}

	pChannel := &ProviderChannel{
		Name:         nameVal,
		Logo:         logoVal,
		StreamURL:    track.URI,
		StreamID:     channelID,
		HD:           strings.Contains(strings.ToLower(track.Tags["tvg-name"]), "hd"),
		StreamFormat: streamURL[3],
		Track:        track,
		OnDemand:     vod,
	}

	if xmlChan, ok := channelMap[track.Tags["tvg-id"]]; ok {
		pChannel.EPGMatch = track.Tags["tvg-id"]
		pChannel.EPGChannel = &xmlChan

		for _, displayName := range xmlChan.DisplayNames {
			if channelNumberRegex(displayName.Value) {
				if chanNum, chanNumErr := strconv.Atoi(displayName.Value); chanNumErr == nil {
					pChannel.Number = chanNum
				}
			}
		}
	}

	favoriteTag := "tvg-id"

	if v.BaseConfig.FavoriteTag != "" {
		favoriteTag = v.BaseConfig.FavoriteTag
	}

	if _, ok := track.Tags[favoriteTag]; !ok {
		log.Panicf("The specified favorite tag (%s) doesn't exist on the track with URL %s", favoriteTag, track.URI)
		return nil, nil
	}

	pChannel.Favorite = contains(v.BaseConfig.Favorites, track.Tags[favoriteTag])

	return pChannel, nil
}

func (v *vader) ProcessProgramme(programme xmltv.Programme) *xmltv.Programme {
	isNew := false
	for idx, title := range programme.Titles {
		isNew = strings.HasSuffix(title.Value, " [New!]")
		programme.Titles[idx].Value = strings.Replace(title.Value, " [New!]", "", -1)
	}

	if isNew {
		programme.New = xmltv.ElementPresent(true)
	}

	return &programme
}

func (v *vader) Configuration() Configuration {
	return v.BaseConfig
}

func (v *vader) RegexKey() string {
	return "group-title"
}
