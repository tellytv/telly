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
	return "lineup_channel"
}

type LineupChannel struct {
	ID             int        `db:"id"`
	LineupID       int        `db:"lineup_id"`
	Title          string     `db:"title"`
	ChannelNumber  string     `db:"channel_number"`
	VideoTrackID   int        `db:"video_track_id"`
	GuideChannelID int        `db:"guide_channel_id"`
	HighDefinition bool       `db:"hd" json:"HD"`
	Favorite       bool       `db:"favorite"`
	CreatedAt      *time.Time `db:"created_at"`

	VideoTrack   *VideoSourceTrack
	GuideChannel *GuideSourceChannel
	HDHR         *HDHomeRunLineupItem

	lineup *SQLLineup
}

func (l *LineupChannel) Fill(api *APICollection) {
	gChannel, gChannelErr := api.GuideSourceChannel.GetGuideSourceChannelByID(l.GuideChannelID, true)
	if gChannelErr != nil {
		log.WithError(gChannelErr).Panicln("error getting channel during LineupChannel fill")
		return
	}
	l.GuideChannel = gChannel
	vTrack, vTrackErr := api.VideoSourceTrack.GetVideoSourceTrackByID(l.VideoTrackID, true)
	if vTrackErr != nil {
		log.WithError(vTrackErr).Panicln("error getting track during LineupChannel fill")
		return
	}
	l.VideoTrack = vTrack
	l.HDHR = l.HDHomeRunLineupItem()
}

func (l *LineupChannel) HDHomeRunLineupItem() *HDHomeRunLineupItem {
	return &HDHomeRunLineupItem{
		DRM:         ConvertibleBoolean(false),
		GuideName:   l.Title,
		GuideNumber: l.ChannelNumber,
		Favorite:    ConvertibleBoolean(l.Favorite),
		HD:          ConvertibleBoolean(l.HighDefinition),
		URL:         fmt.Sprintf("http://%s:%d/auto/v%s", l.lineup.DiscoveryAddress, l.lineup.Port, l.ChannelNumber),
	}
}

// LineupChannelAPI contains all methods for the User struct
type LineupChannelAPI interface {
	InsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error)
	DeleteLineupChannel(channelID int) (*LineupChannel, error)
	UpdateLineupChannel(channelID int, description string) (*LineupChannel, error)
	GetLineupChannelByID(id int) (*LineupChannel, error)
	GetChannelsForLineup(lineupID int, expanded bool) ([]LineupChannel, error)
}

const baseLineupChannelQuery string = `
SELECT
  C.id,
  C.lineup_id,
  C.title,
  C.channel_number,
  C.video_track_id,
  C.guide_channel_id,
  C.favorite,
  C.hd,
  C.created_at
  FROM lineup_channel C`

// InsertLineupChannel inserts a new LineupChannel into the database.
func (db *LineupChannelDB) InsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error) {
	channel := LineupChannel{}
	res, err := db.SQL.NamedExec(`
    INSERT INTO lineup_channel (lineup_id, title, channel_number, video_track_id, guide_channel_id, favorite, hd)
    VALUES (:lineup_id, :title, :channel_number, :video_track_id, :guide_channel_id, :favorite, :hd)`, channelStruct)
	if err != nil {
		return &channel, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &channel, rowIDErr
	}
	err = db.SQL.Get(&channel, "SELECT * FROM lineup_channel WHERE id = $1", rowID)
	return &channel, err
}

// GetLineupChannelByID returns a single LineupChannel for the given ID.
func (db *LineupChannelDB) GetLineupChannelByID(id int) (*LineupChannel, error) {
	var channel LineupChannel
	err := db.SQL.Get(&channel, fmt.Sprintf(`%s WHERE C.id = $1`, baseLineupChannelQuery), id)
	if err != nil {
		return nil, err
	}

	// Need to get the address and port number to properly fill
	lineup, lineupErr := db.Collection.Lineup.GetLineupByID(channel.LineupID, false)
	if lineupErr != nil {
		return nil, lineupErr
	}

	channel.lineup = lineup
	channel.Fill(db.Collection)

	return &channel, err
}

// DeleteLineupChannel marks a channel with the given ID as deleted.
func (db *LineupChannelDB) DeleteLineupChannel(channelID int) (*LineupChannel, error) {
	channel := LineupChannel{}
	err := db.SQL.Get(&channel, `DELETE FROM lineup_channel WHERE id = $1`, channelID)
	return &channel, err
}

// UpdateLineupChannel updates a channel.
func (db *LineupChannelDB) UpdateLineupChannel(channelID int, description string) (*LineupChannel, error) {
	channel := LineupChannel{}
	err := db.SQL.Get(&channel, `UPDATE lineup_channel SET description = $2 WHERE id = $1 RETURNING *`, channelID, description)
	return &channel, err
}

// GetChannelsForLineup returns a slice of LineupChannels for the given lineup ID.
func (db *LineupChannelDB) GetChannelsForLineup(lineupID int, expanded bool) ([]LineupChannel, error) {
	channels := make([]LineupChannel, 0)
	err := db.SQL.Select(&channels, fmt.Sprintf(`%s WHERE C.lineup_id = $1`, baseLineupChannelQuery), lineupID)
	if err != nil {
		return nil, err
	}
	if expanded {
		// Need to get the address and port number to properly fill
		lineup, lineupErr := db.Collection.Lineup.GetLineupByID(lineupID, false)
		if lineupErr != nil {
			return nil, lineupErr
		}
		for idx, channel := range channels {
			channel.lineup = lineup
			channel.Fill(db.Collection)
			channels[idx] = channel
		}
	}
	return channels, nil
}
