package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/guide_providers"
	"github.com/tellytv/telly/internal/xmltv"
)

func xmlTV(cc *context.CContext, c *gin.Context) {
	epg := &xmltv.TV{
		Date:              time.Now().Format("2006-01-02"),
		GeneratorInfoName: "telly",
		GeneratorInfoURL:  "https://github.com/tellytv/telly",
	}

	lineups, lineupsErr := cc.API.Lineup.GetEnabledLineups(true)
	if lineupsErr != nil {
		c.AbortWithError(http.StatusInternalServerError, lineupsErr)
		return
	}

	programmes, programmesErr := cc.API.GuideSourceProgramme.GetProgrammesForActiveChannels()
	if programmesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, programmesErr)
		return
	}

	epgMatchMap := make(map[string]int)

	for _, lineup := range lineups {
		for _, channel := range lineup.Channels {
			epgMatchMap[channel.GuideChannel.XMLTVID] = channel.ID

			var guideChannel guide_providers.Channel

			if jsonErr := json.Unmarshal(channel.GuideChannel.Data, &guideChannel); jsonErr != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error while unmarshalling lineupchannel to guide_providers.channel: %s", jsonErr))
				return
			}

			xChannel := guideChannel.XMLTV()

			displayNames := []xmltv.CommonElement{xmltv.CommonElement{Value: channel.Title}}
			displayNames = append(displayNames, xChannel.DisplayNames...)

			epg.Channels = append(epg.Channels, xmltv.Channel{
				ID:           strconv.Itoa(channel.ID),
				DisplayNames: displayNames,
				Icons:        xChannel.Icons,
				LCN:          channel.ChannelNumber,
			})
		}
	}

	for _, programme := range programmes {
		programme.XMLTV.Channel = strconv.Itoa(epgMatchMap[programme.Channel])
		epg.Programmes = append(epg.Programmes, *programme.XMLTV)
	}

	buf, marshallErr := xml.MarshalIndent(epg, "", "\t")
	if marshallErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling EPG to XML"))
	}
	c.Data(http.StatusOK, "application/xml", []byte(xml.Header+`<!DOCTYPE tv SYSTEM "xmltv.dtd">`+"\n"+string(buf)))
}
