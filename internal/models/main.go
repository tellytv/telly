package models

import (
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
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
)

var sq = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar) // nolint

// APICollection is a struct containing all models.
type APICollection struct {
	GuideSource          GuideSourceAPI
	GuideSourceChannel   GuideSourceChannelAPI
	GuideSourceProgramme GuideSourceProgrammeAPI
	Lineup               LineupAPI
	LineupChannel        LineupChannelAPI
	VideoSource          VideoSourceAPI
	VideoSourceTrack     VideoSourceTrackAPI
}

// NewAPICollection returns an initialized APICollection struct.
func NewAPICollection(db *sqlx.DB) *APICollection {
	api := &APICollection{}

	api.GuideSource = newGuideSourceDB(db, api)
	api.GuideSourceChannel = newGuideSourceChannelDB(db, api)
	api.GuideSourceProgramme = newGuideSourceProgrammeDB(db, api)
	api.Lineup = newLineupDB(db, api)
	api.LineupChannel = newLineupChannelDB(db, api)
	api.VideoSource = newVideoSourceDB(db, api)
	api.VideoSourceTrack = newVideoSourceTrackDB(db, api)
	return api
}
