package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/xmltv"
)

func xmlTV(cc *context.CContext, c *gin.Context) {
	epg := &xmltv.TV{
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
			epg.Channels = append(epg.Channels, xmltv.Channel{
				ID:           strconv.Itoa(channel.ID),
				DisplayNames: []xmltv.CommonElement{xmltv.CommonElement{Value: channel.Title}},
				LCN:          channel.ChannelNumber,
			})
		}
	}

	for _, programme := range programmes {
		programme.XMLTV.Channel = strconv.Itoa(epgMatchMap[programme.Channel])
		epg.Programmes = append(epg.Programmes, *programme.XMLTV)
	}

	sort.Slice(epg.Channels, func(i, j int) bool {
		return epg.Channels[i].LCN < epg.Channels[j].LCN
	})

	buf, marshallErr := xml.MarshalIndent(epg, "", "\t")
	if marshallErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling EPG to XML"))
	}
	c.Data(http.StatusOK, "application/xml", []byte(xml.Header+`<!DOCTYPE tv SYSTEM "xmltv.dtd">`+"\n"+string(buf)))
}
