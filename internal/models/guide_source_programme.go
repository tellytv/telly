package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tellytv/telly/internal/xmltv"
	squirrel "gopkg.in/Masterminds/squirrel.v1"
)

// GuideSourceProgrammeDB is a struct containing initialized the SQL connection as well as the APICollection.
// Why is it spelled like this instead of "program"? Matches XMLTV spec which this code is based on.
type GuideSourceProgrammeDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newGuideSourceProgrammeDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *GuideSourceProgrammeDB {
	db := &GuideSourceProgrammeDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *GuideSourceProgrammeDB) tableName() string {
	return "guide_source_programme"
}

// GuideSourceProgramme is a single programme available in a guide providers lineup.
type GuideSourceProgramme struct {
	GuideID      int             `db:"guide_id"`
	Channel      string          `db:"channel"`
	ProviderData json.RawMessage `db:"provider_data"`
	StartTime    *time.Time      `db:"start"`
	EndTime      *time.Time      `db:"end"`
	Date         *time.Time      `db:"date,omitempty"`
	Data         json.RawMessage `db:"data"`
	ImportedAt   *time.Time      `db:"imported_at"`

	XMLTV *xmltv.Programme `json:"-"`
}

// GuideSourceProgrammeAPI contains all methods for the User struct
type GuideSourceProgrammeAPI interface {
	InsertGuideSourceProgramme(guideID int, programme xmltv.Programme, providerData interface{}) (*GuideSourceProgramme, error)
	DeleteGuideSourceProgrammesForChannel(channelID string) error
	UpdateGuideSourceProgramme(programmeID string, providerData interface{}) error
	GetGuideSourceProgrammeByID(id int) (*GuideSourceProgramme, error)
	GetProgrammesForActiveChannels() ([]GuideSourceProgramme, error)
	GetProgrammesForChannel(channelID string) ([]GuideSourceProgramme, error)
	GetProgrammesForGuideID(guideSourceID int) ([]GuideSourceProgramme, error)
}

// nolint
const baseGuideSourceProgrammeQuery string = `
SELECT
  G.guide_id,
  G.channel,
  G.provider_data,
  G.start,
  G.end,
  G.date,
  G.data,
  G.imported_at
  FROM guide_source_programme G`

// InsertGuideSourceProgramme inserts a new GuideSourceProgramme into the database.
func (db *GuideSourceProgrammeDB) InsertGuideSourceProgramme(guideID int, programme xmltv.Programme, providerData interface{}) (*GuideSourceProgramme, error) {
	programmeJSON, programmeMarshalErr := json.Marshal(programme)
	if programmeMarshalErr != nil {
		return nil, fmt.Errorf("error when marshalling xmltv.Programme for use in guide_source_programme insert: %s", programmeMarshalErr)
	}

	providerDataJSON, providerDataJSONErr := json.Marshal(providerData)
	if providerDataJSONErr != nil {
		return nil, fmt.Errorf("error when marshalling providerData for use in guide_source_programme insert: %s", providerDataJSONErr)
	}

	date := time.Time(programme.Date)
	insertingProgramme := GuideSourceProgramme{
		GuideID:      guideID,
		Channel:      programme.Channel,
		ProviderData: providerDataJSON,
		StartTime:    &programme.Start.Time,
		EndTime:      &programme.Stop.Time,
		Date:         &date,
		Data:         programmeJSON,
	}

	res, err := db.SQL.NamedExec(`
    INSERT OR REPLACE INTO guide_source_programme (guide_id, channel, provider_data, start, end, date, data)
    VALUES (:guide_id, :channel, :provider_data, :start, :end, :date, :data)`, insertingProgramme)
	if err != nil {
		return nil, fmt.Errorf("error when inserting guide_source_programme row: %s", err)
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return nil, fmt.Errorf("error when getting last inserted row id during guide_source_programme insert: %s", rowIDErr)
	}
	outputProgramme := GuideSourceProgramme{}
	if getErr := db.SQL.Get(&outputProgramme, "SELECT * FROM guide_source_programme WHERE rowid = $1", rowID); getErr != nil {
		return nil, fmt.Errorf("error when selecting newly inserted row during guide_source_programme insert: %s", getErr)
	}
	if unmarshalErr := json.Unmarshal(outputProgramme.Data, &outputProgramme.XMLTV); unmarshalErr != nil {
		return nil, fmt.Errorf("error when unmarshalling json.RawMessage to xmltv.Programme during guide_source_programme insert: %s", unmarshalErr)
	}
	return &outputProgramme, nil
}

// GetGuideSourceProgrammeByID returns a single GuideSourceProgramme for the given ID.
func (db *GuideSourceProgrammeDB) GetGuideSourceProgrammeByID(id int) (*GuideSourceProgramme, error) {
	var programme GuideSourceProgramme
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_programme").Where(squirrel.Eq{"id": id}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Get(&programme, sql, args...)
	if err != nil {
		return nil, err
	}
	return &programme, err
}

// DeleteGuideSourceProgrammesForChannel marks a programme with the given ID as deleted.
func (db *GuideSourceProgrammeDB) DeleteGuideSourceProgrammesForChannel(channelID string) error {
	_, err := db.SQL.Exec(`DELETE FROM guide_source_programme WHERE channel IN (SELECT xmltv_id FROM guide_source_channel WHERE id IN (SELECT guide_channel_id FROM lineup_channel WHERE id = ?))`, channelID)
	return err
}

// UpdateGuideSourceProgramme updates a programme.
func (db *GuideSourceProgrammeDB) UpdateGuideSourceProgramme(programmeID string, providerData interface{}) error {
	_, err := db.SQL.Exec(`UPDATE guide_source_programme SET provider_data = ? WHERE id = ?`, providerData, programmeID)
	return err
}

// GetProgrammesForActiveChannels returns a slice of GuideSourceProgrammes for actively assigned channels.
func (db *GuideSourceProgrammeDB) GetProgrammesForActiveChannels() ([]GuideSourceProgramme, error) {
	programmes := make([]GuideSourceProgramme, 0)
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_programme").Where("channel IN (SELECT xmltv_id FROM guide_source_channel WHERE id IN (SELECT guide_channel_id FROM lineup_channel))").OrderBy("start ASC").ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Select(&programmes, sql, args...)
	if err != nil {
		return nil, err
	}
	for idx, programme := range programmes {
		if unmarshalErr := json.Unmarshal(programme.Data, &programme.XMLTV); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		programmes[idx] = programme
	}
	return programmes, err
}

// GetProgrammesForChannel returns a slice of GuideSourceProgrammes for the given XMLTV channel ID.
func (db *GuideSourceProgrammeDB) GetProgrammesForChannel(channelID string) ([]GuideSourceProgramme, error) {
	programmes := make([]GuideSourceProgramme, 0)
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_programme").Where(squirrel.And{squirrel.Eq{"channel": channelID}, squirrel.GtOrEq{"start": "datetime('now')"}, squirrel.LtOrEq{"start": "datetime('now', '+6 hours')"}}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Select(&programmes, sql, args...)
	if err != nil {
		return nil, err
	}
	for idx, programme := range programmes {
		if unmarshalErr := json.Unmarshal(programme.Data, &programme.XMLTV); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		programmes[idx] = programme
	}
	return programmes, err
}

// GetProgrammesForGuideID returns a slice of GuideSourceProgrammes for the given guide ID.
func (db *GuideSourceProgrammeDB) GetProgrammesForGuideID(guideSourceID int) ([]GuideSourceProgramme, error) {
	programmes := make([]GuideSourceProgramme, 0)
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_programme").Where(squirrel.And{squirrel.Eq{"guide_id": guideSourceID}, squirrel.GtOrEq{"start": "datetime('now')"}, squirrel.LtOrEq{"start": "datetime('now', '+6 hours')"}}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Select(&programmes, sql, args...)
	if err != nil {
		return nil, err
	}
	for idx, programme := range programmes {
		if unmarshalErr := json.Unmarshal(programme.Data, &programme.XMLTV); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		programmes[idx] = programme
	}
	return programmes, err
}
