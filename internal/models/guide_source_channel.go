package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tellytv/telly/internal/guideproviders"
	"github.com/tellytv/telly/internal/xmltv"
	squirrel "gopkg.in/Masterminds/squirrel.v1"
)

// GuideSourceChannelDB is a struct containing initialized the SQL connection as well as the APICollection.
type GuideSourceChannelDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newGuideSourceChannelDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *GuideSourceChannelDB {
	db := &GuideSourceChannelDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *GuideSourceChannelDB) tableName() string {
	return "guide_source_channel"
}

// GuideSourceChannel is a single channel in a guide providers lineup.
type GuideSourceChannel struct {
	ID           int             `db:"id"`
	GuideID      int             `db:"guide_id"`
	XMLTVID      string          `db:"xmltv_id"`
	ProviderData json.RawMessage `db:"provider_data"`
	Data         json.RawMessage `db:"data"`
	ImportedAt   *time.Time      `db:"imported_at"`

	GuideSource     *GuideSource
	GuideSourceName string
	XMLTV           *xmltv.Channel `json:"-"`
}

// GuideSourceChannelAPI contains all methods for the User struct
type GuideSourceChannelAPI interface {
	InsertGuideSourceChannel(guideID int, channel guideproviders.Channel, providerData interface{}) (*GuideSourceChannel, error)
	DeleteGuideSourceChannel(channelID int) (*GuideSourceChannel, error)
	UpdateGuideSourceChannel(XMLTVID string, providerData interface{}) error
	GetGuideSourceChannelByID(id int, expanded bool) (*GuideSourceChannel, error)
	GetChannelsForGuideSource(guideSourceID int) ([]GuideSourceChannel, error)
}

// nolint
const baseGuideSourceChannelQuery string = `
SELECT
  G.id,
  G.guide_id,
  G.xmltv_id,
  G.provider_data,
  G.data,
  G.imported_at
  FROM guide_source_channel G`

// InsertGuideSourceChannel inserts a new GuideSourceChannel into the database.
func (db *GuideSourceChannelDB) InsertGuideSourceChannel(guideID int, channel guideproviders.Channel, providerData interface{}) (*GuideSourceChannel, error) {
	channelJSON, channelJSONErr := json.Marshal(channel)
	if channelJSONErr != nil {
		return nil, fmt.Errorf("error when marshalling guideproviders.Channel for use in guide_source_channel insert: %s", channelJSONErr)
	}

	providerDataJSON, providerDataJSONErr := json.Marshal(providerData)
	if providerDataJSONErr != nil {
		return nil, fmt.Errorf("error when marshalling providerData for use in guide_source_programme insert: %s", providerDataJSONErr)
	}

	insertingChannel := GuideSourceChannel{
		GuideID:      guideID,
		XMLTVID:      channel.ID,
		Data:         channelJSON,
		ProviderData: providerDataJSON,
	}

	res, err := db.SQL.NamedExec(`
    INSERT OR REPLACE INTO guide_source_channel (guide_id, xmltv_id, data, provider_data)
    VALUES (:guide_id, :xmltv_id, :data, :provider_data)`, insertingChannel)
	if err != nil {
		return nil, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return nil, rowIDErr
	}
	outputChannel := GuideSourceChannel{}
	if getErr := db.SQL.Get(&outputChannel, "SELECT * FROM guide_source_channel WHERE id = $1", rowID); getErr != nil {
		return nil, getErr
	}
	if unmarshalErr := json.Unmarshal(outputChannel.Data, &outputChannel.XMLTV); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	return &outputChannel, err
}

// GetGuideSourceChannelByID returns a single GuideSourceChannel for the given ID.
func (db *GuideSourceChannelDB) GetGuideSourceChannelByID(id int, expanded bool) (*GuideSourceChannel, error) {
	var channel GuideSourceChannel
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_channel").Where(squirrel.Eq{"id": id}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Get(&channel, sql, args)
	if err != nil {
		return nil, err
	}
	if expanded {
		guide, guideErr := db.Collection.GuideSource.GetGuideSourceByID(channel.GuideID)
		if guideErr != nil {
			return nil, guideErr
		}
		channel.GuideSource = guide
	}
	return &channel, err
}

// DeleteGuideSourceChannel marks a channel with the given ID as deleted.
func (db *GuideSourceChannelDB) DeleteGuideSourceChannel(channelID int) (*GuideSourceChannel, error) {
	channel := GuideSourceChannel{}
	err := db.SQL.Get(&channel, `DELETE FROM guide_source_channel WHERE id = $1`, channelID)
	return &channel, err
}

// UpdateGuideSourceChannel updates a channel.
func (db *GuideSourceChannelDB) UpdateGuideSourceChannel(XMLTVID string, providerData interface{}) error {
	_, err := db.SQL.Exec(`UPDATE guide_source_channel SET provider_data = ? WHERE xmltv_id = ?`, providerData, XMLTVID)
	return err
}

// GetChannelsForGuideSource returns a slice of GuideSourceChannels for the given video source ID.
func (db *GuideSourceChannelDB) GetChannelsForGuideSource(guideSourceID int) ([]GuideSourceChannel, error) {
	channels := make([]GuideSourceChannel, 0)
	sql, args, sqlGenErr := squirrel.Select("*").From("guide_source_channel").Where(squirrel.Eq{"guide_id": guideSourceID}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Select(&channels, sql, args)
	return channels, err
}
