package models

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
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

var log = &logrus.Logger{}

// NewAPICollection returns an initialized APICollection struct.
func NewAPICollection(db *sqlx.DB, logger *logrus.Logger) *APICollection {
	log = logger
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
