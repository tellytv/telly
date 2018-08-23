package models

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

var (
	log = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}

	safeStringsRegex = regexp.MustCompile(`(?m)(username|password|token)=[\w=]+(&?)`)

	stringSafer = func(input string) string {
		ret := input
		if strings.HasPrefix(input, "username=") {
			ret = "username=REDACTED"
		} else if strings.HasPrefix(input, "password=") {
			ret = "password=REDACTED"
		} else if strings.HasPrefix(input, "token=") {
			ret = "token=bm90Zm9yeW91" // "notforyou"
		}
		if strings.HasSuffix(input, "&") {
			return fmt.Sprintf("%s&", ret)
		}
		return ret
	}
)

// APICollection is a struct containing all models.
type APICollection struct {
	GuideSource        GuideSourceAPI
	GuideSourceChannel GuideSourceChannelAPI
	Lineup             LineupAPI
	LineupChannel      LineupChannelAPI
	VideoSource        VideoSourceAPI
	VideoSourceTrack   VideoSourceTrackAPI
}

// NewAPICollection returns an initialized APICollection struct.
func NewAPICollection(ctx context.Context, db *sqlx.DB) *APICollection {
	api := &APICollection{}

	api.GuideSource = newGuideSourceDB(db, api)
	api.GuideSourceChannel = newGuideSourceChannelDB(db, api)
	api.Lineup = newLineupDB(db, api)
	api.LineupChannel = newLineupChannelDB(db, api)
	api.VideoSource = newVideoSourceDB(db, api)
	api.VideoSourceTrack = newVideoSourceTrackDB(db, api)
	return api
}
