package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
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

type GuideSourceChannel struct {
	ID             int             `db:"id"`
	GuideID        int             `db:"guide_id"`
	XMLTVID        string          `db:"xmltv_id"`
	DisplayNames   json.RawMessage `db:"display_names"`
	URLs           json.RawMessage `db:"urls"`
	Icons          json.RawMessage `db:"icons"`
	ChannelNumber  string          `db:"channel_number"`
	HighDefinition bool            `db:"hd" json:"HD"`
	ImportedAt     *time.Time      `db:"imported_at"`

	GuideSource     *GuideSource
	GuideSourceName string
}

// GuideSourceChannelAPI contains all methods for the User struct
type GuideSourceChannelAPI interface {
	InsertGuideSourceChannel(channelStruct GuideSourceChannel) (*GuideSourceChannel, error)
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
  G.display_names,
  G.urls,
  G.icons,
  G.channel_number,
  G.hd,
  G.imported_at
  FROM guide_source_channel G`

// InsertGuideSourceChannel inserts a new GuideSourceChannel into the database.
func (db *GuideSourceChannelDB) InsertGuideSourceChannel(channelStruct GuideSourceChannel) (*GuideSourceChannel, error) {
	channel := GuideSourceChannel{}
	res, err := db.SQL.NamedExec(`
    INSERT INTO guide_source_channel (guide_id, xmltv_id, display_names, urls, icons, channel_number, hd)
    VALUES (:guide_id, :xmltv_id, :display_names, :urls, :icons, :channel_number, :hd)`, channelStruct)
	if err != nil {
		return &channel, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &channel, rowIDErr
	}
	err = db.SQL.Get(&channel, "SELECT * FROM guide_source_channel WHERE id = $1", rowID)
	return &channel, err
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
