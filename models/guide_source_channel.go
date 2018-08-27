package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tellytv/telly/internal/guideproviders"
	"github.com/tellytv/telly/internal/xmltv"
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
	ID         int             `db:"id"`
	GuideID    int             `db:"guide_id"`
	XMLTVID    string          `db:"xmltv_id"`
	Data       json.RawMessage `db:"data"`
	ImportedAt *time.Time      `db:"imported_at"`

	GuideSource     *GuideSource
	GuideSourceName string
	XMLTV           *xmltv.Channel `json:"-"`
}

// GuideSourceChannelAPI contains all methods for the User struct
type GuideSourceChannelAPI interface {
	InsertGuideSourceChannel(guideID int, channel guideproviders.Channel) (*GuideSourceChannel, error)
	DeleteGuideSourceChannel(channelID int) (*GuideSourceChannel, error)
	UpdateGuideSourceChannel(channelID int, description string) (*GuideSourceChannel, error)
	GetGuideSourceChannelByID(id int, expanded bool) (*GuideSourceChannel, error)
	GetChannelsForGuideSource(guideSourceID int) ([]GuideSourceChannel, error)
}

const baseGuideSourceChannelQuery string = `
SELECT
  G.id,
  G.guide_id,
  G.xmltv_id,
  G.data,
  G.imported_at
  FROM guide_source_channel G`

// InsertGuideSourceChannel inserts a new GuideSourceChannel into the database.
func (db *GuideSourceChannelDB) InsertGuideSourceChannel(guideID int, channel guideproviders.Channel) (*GuideSourceChannel, error) {
	marshalled, marshalErr := json.Marshal(channel)
	if marshalErr != nil {
		return nil, marshalErr
	}

	insertingChannel := GuideSourceChannel{
		GuideID: guideID,
		XMLTVID: channel.ID,
		Data:    marshalled,
	}

	res, err := db.SQL.NamedExec(`
    INSERT INTO guide_source_channel (guide_id, xmltv_id, data)
    VALUES (:guide_id, :xmltv_id, :data)`, insertingChannel)
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
	err := db.SQL.Get(&channel, fmt.Sprintf(`%s WHERE G.id = $1`, baseGuideSourceChannelQuery), id)
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
func (db *GuideSourceChannelDB) UpdateGuideSourceChannel(channelID int, description string) (*GuideSourceChannel, error) {
	channel := GuideSourceChannel{}
	err := db.SQL.Get(&channel, `UPDATE guide_source_channel SET description = $2 WHERE id = $1 RETURNING *`, channelID, description)
	return &channel, err
}

// GetChannelsForGuideSource returns a slice of GuideSourceChannels for the given video source ID.
func (db *GuideSourceChannelDB) GetChannelsForGuideSource(guideSourceID int) ([]GuideSourceChannel, error) {
	channels := make([]GuideSourceChannel, 0)
	err := db.SQL.Select(&channels, fmt.Sprintf(`%s WHERE G.guide_id = $1`, baseGuideSourceChannelQuery), guideSourceID)
	return channels, err
}
