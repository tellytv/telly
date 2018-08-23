package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/xmltv"
)

func xmlTV(cc *context.CContext, c *gin.Context) {
	// FIXME: Move this outside of the function stuff.
	epg := &xmltv.TV{
		GeneratorInfoName: "telly",
		GeneratorInfoURL:  "https://github.com/tellytv/telly",
	}

	// FIXME: Not actually a lineup...
	// lineup := &models.SQLLineup{}

	// lineups, lineupsErr := cc.API.Lineup.GetEnabledLineups(true)
	// if lineupsErr != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, lineupsErr)
	// 	return
	// }

	// for _, channel := range lineup.Channels {
	// 	if channel.ProviderChannel.EPGChannel != nil {
	// 		epg.Channels = append(epg.Channels, *channel.ProviderChannel.EPGChannel)
	// 		epg.Programmes = append(epg.Programmes, channel.ProviderChannel.EPGProgrammes...)
	// 	}
	// }

	sort.Slice(epg.Channels, func(i, j int) bool {
		return epg.Channels[i].LCN < epg.Channels[j].LCN
	})

	buf, marshallErr := xml.MarshalIndent(epg, "", "\t")
	if marshallErr != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error marshalling EPG to XML"))
	}
	c.Data(http.StatusOK, "application/xml", []byte(xml.Header+`<!DOCTYPE tv SYSTEM "xmltv.dtd">`+"\n"+string(buf)))
}
