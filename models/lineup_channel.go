package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// LineupChannelDB is a struct containing initialized the SQL connection as well as the APICollection.
type LineupChannelDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newLineupChannelDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *LineupChannelDB {
	db := &LineupChannelDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *LineupChannelDB) tableName() string {
	return "lineup_channels"
}

type LineupChannel struct {
	ID             int        `db:"id"`
	Title          string     `db:"title"`
	ChannelNumber  string     `db:"channel_number"`
	VideoTrackID   string     `db:"video_track_id"`
	GuideChannelID string     `db:"guide_channel_id"`
	HighDefinition bool       `db:"hd"`
	Favorite       bool       `db:"favorite"`
	CreatedAt      *time.Time `db:"created_at"`
}

// LineupChannelAPI contains all methods for the User struct
type LineupChannelAPI interface {
	InsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error)
	DeleteLineupChannel(channelID string) (*LineupChannel, error)
	UpdateLineupChannel(channelID, description string) (*LineupChannel, error)
	GetLineupChannelByID(id string) (*LineupChannel, error)
}

const baseLineupChannelQuery string = `
SELECT
  C.id,
  C.title,
  C.channel_number,
  C.video_track_id,
  C.guide_channel_id,
  C.favorite,
  C.hd,
  C.created_at
  FROM lineup_channels C`

// InsertLineupChannel inserts a new LineupChannel into the database.
func (db *LineupChannelDB) InsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error) {
	channel := LineupChannel{}
	rows, err := db.SQL.NamedQuery(`
    INSERT INTO lineup_channels (title, channel_number, video_track_id, guide_channel_id, favorite, hd)
    VALUES (:title, :channel_number, :video_track_id, :guide_channel_id, :favorite, :hd)
    RETURNING *`, channelStruct)
	if err != nil {
		return &channel, err
	}
	for rows.Next() {
		err := rows.StructScan(&channel)
		if err != nil {
			return &channel, err
		}
	}
	return &channel, nil
}

// GetLineupChannelByID returns a single LineupChannel for the given ID.
func (db *LineupChannelDB) GetLineupChannelByID(id string) (*LineupChannel, error) {
	var channel LineupChannel
	err := db.SQL.Get(&channel, fmt.Sprintf(`%s WHERE G.id = $1`, baseLineupChannelQuery), id)
	return &channel, err
}

// DeleteLineupChannel marks a channel with the given ID as deleted.
func (db *LineupChannelDB) DeleteLineupChannel(channelID string) (*LineupChannel, error) {
	channel := LineupChannel{}
	err := db.SQL.Get(&channel, `DELETE FROM lineup_channels WHERE id = $1`, channelID)
	return &channel, err
}

// UpdateLineupChannel updates a channel.
func (db *LineupChannelDB) UpdateLineupChannel(channelID, description string) (*LineupChannel, error) {
	channel := LineupChannel{}
	err := db.SQL.Get(&channel, `UPDATE lineup_channels SET description = $2 WHERE id = $1 RETURNING *`, channelID, description)
	return &channel, err
}
