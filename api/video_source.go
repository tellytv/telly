package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/models"
)

func getVideoSources(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.VideoSource.GetAllVideoSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}
	c.JSON(http.StatusOK, sources)
}

func addVideoSource(cc *context.CContext, c *gin.Context) {
	var payload models.VideoSource
	if c.BindJSON(&payload) == nil {
		newProvider, providerErr := cc.API.VideoSource.InsertVideoSource(payload)
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		providerCfg := newProvider.ProviderConfiguration()

		log.Infof("providerCfg %+v", providerCfg)

		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			c.AbortWithError(http.StatusInternalServerError, providerErr)
			return
		}

		log.Infoln("Detected passed config is for provider", provider.Name())

		reader, m3uErr := models.GetM3U(provider.PlaylistURL(), false)
		if m3uErr != nil {
			log.WithError(m3uErr).Errorln("unable to get m3u file")
			c.AbortWithError(http.StatusBadRequest, m3uErr)
			return
		}

		rawPlaylist, err := m3uplus.Decode(reader)
		if err != nil {
			log.WithError(err).Errorln("unable to parse m3u file")
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		for _, track := range rawPlaylist.Tracks {
			marshalledTags, _ := json.Marshal(track.Tags)
			newTrack, newTrackErr := cc.API.VideoSourceTrack.InsertVideoSourceTrack(models.VideoSourceTrack{
				VideoSourceID: newProvider.ID,
				Name:          track.Name,
				Tags:          marshalledTags,
				RawLine:       track.Raw,
				StreamURL:     track.URI,
			})
			if newTrackErr != nil {
				log.WithError(newTrackErr).Errorln("Error creating new video source track!")
				c.AbortWithError(http.StatusInternalServerError, newTrackErr)
				return
			}
			newProvider.Tracks = append(newProvider.Tracks, *newTrack)
		}
		c.JSON(http.StatusOK, newProvider)
	}
}

func getAllTracks(cc *context.CContext, c *gin.Context) {
	sources, sourcesErr := cc.API.VideoSource.GetAllVideoSources(true)
	if sourcesErr != nil {
		c.AbortWithError(http.StatusInternalServerError, sourcesErr)
		return
	}

	tracks := make([]models.VideoSourceTrack, 0)

	for _, source := range sources {
		for _, track := range source.Tracks {
			track.VideoSourceName = source.Name
			tracks = append(tracks, track)
		}
	}

	c.JSON(http.StatusOK, tracks)
}
