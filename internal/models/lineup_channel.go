package models

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	squirrel "gopkg.in/Masterminds/squirrel.v1"
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

// HDHomeRunLineupItem is a HDHomeRun specification compatible representation of a Track available in the lineup.
type HDHomeRunLineupItem struct {
	XMLName     xml.Name           `xml:"Program"    json:"-"`
	AudioCodec  string             `xml:",omitempty" json:",omitempty"`
	DRM         ConvertibleBoolean `xml:",omitempty" json:",omitempty"`
	Favorite    ConvertibleBoolean `xml:",omitempty" json:",omitempty"`
	GuideName   string             `xml:",omitempty" json:",omitempty"`
	GuideNumber string             `xml:",omitempty" json:",omitempty"`
	HD          ConvertibleBoolean `xml:",omitempty" json:",omitempty"`
	URL         string             `xml:",omitempty" json:",omitempty"`
	VideoCodec  string             `xml:",omitempty" json:",omitempty"`
}

// LineupChannel is a single channel available in a Lineup.
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

	VideoTrack   *VideoSourceTrack    `json:",omitempty"`
	GuideChannel *GuideSourceChannel  `json:",omitempty"`
	HDHR         *HDHomeRunLineupItem `json:",omitempty"`

	lineup *Lineup
}

func (l *LineupChannel) String() string {
	return fmt.Sprintf("channel: %s (ch#: %s, video source name: %s, video source provider type: %s)", l.Title, l.ChannelNumber, l.VideoTrack.VideoSource.Name, l.VideoTrack.VideoSource.Provider)
}

// Fill will insert Lineup, GuideChannel and VideoTrack into the LineupChannel.
func (l *LineupChannel) Fill(api *APICollection) {
	if l.lineup == nil {
		// Need to get the address and port number to properly fill
		lineup, lineupErr := api.Lineup.GetLineupByID(l.LineupID, false)
		if lineupErr != nil {
			log.WithError(lineupErr).Panicln("error getting lineup during LineupChannel fill")
			return
		}

		l.lineup = lineup
	}

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

// HDHomeRunLineupItem returns a HDHomeRunLineupItem for the LineupChannel.
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
	UpsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error)
	DeleteLineupChannel(channelID string) error
	UpdateLineupChannel(channelStruct LineupChannel) (*LineupChannel, error)
	GetLineupChannelByID(lineupID int, channelNumber string) (*LineupChannel, error)
	GetChannelsForLineup(lineupID int, expanded bool) ([]LineupChannel, error)
	GetEnabledChannelsForGuideProvider(providerID int) ([]LineupChannel, error)
	GetEnabledChannelsForVideoProvider(providerID int) ([]LineupChannel, error)
}

// nolint
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

// UpsertLineupChannel upserts a LineupChannel in the database.
func (db *LineupChannelDB) UpsertLineupChannel(channelStruct LineupChannel) (*LineupChannel, error) {
	if channelStruct.ID != 0 {
		return db.UpdateLineupChannel(channelStruct)
	}
	return db.InsertLineupChannel(channelStruct)
}

// GetLineupChannelByID returns a single LineupChannel for the given ID.
func (db *LineupChannelDB) GetLineupChannelByID(lineupID int, channelNumber string) (*LineupChannel, error) {
	var channel LineupChannel

	sql, args, sqlGenErr := squirrel.Select("*").From("lineup_channel").Where(squirrel.Eq{"lineup_id": lineupID, "channel_number": channelNumber}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}

	err := db.SQL.Get(&channel, sql, args)
	if err != nil {
		return nil, err
	}

	channel.Fill(db.Collection)

	return &channel, err
}

// DeleteLineupChannel marks a channel with the given ID as deleted.
func (db *LineupChannelDB) DeleteLineupChannel(channelID string) error {
	_, err := db.SQL.Exec(`DELETE FROM lineup_channel WHERE id = ?`, channelID)
	return err
}

// UpdateLineupChannel updates a channel.
func (db *LineupChannelDB) UpdateLineupChannel(channelStruct LineupChannel) (*LineupChannel, error) {
	channel := LineupChannel{}
	_, err := db.SQL.NamedExec(`UPDATE lineup_channel SET lineup_id = :lineup_id, title = :title, channel_number = :channel_number, video_track_id = :video_track_id, guide_channel_id = :guide_channel_id, favorite = :favorite, hd =:hd WHERE id = :id`, channelStruct)
	if err != nil {
		return &channel, err
	}
	err = db.SQL.Get(&channel, "SELECT * FROM lineup_channel WHERE id = $1", channelStruct.ID)
	return &channel, err
}

// GetChannelsForLineup returns a slice of LineupChannels for the given lineup ID.
func (db *LineupChannelDB) GetChannelsForLineup(lineupID int, expanded bool) ([]LineupChannel, error) {
	channels := make([]LineupChannel, 0)
	sql, args, sqlGenErr := squirrel.Select("*").From("lineup_channel").Where(squirrel.Eq{"lineup_id": lineupID}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Select(&channels, sql, args)
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

// GetEnabledChannelsForGuideProvider returns a slice of LineupChannels for the given guide provider ID.
func (db *LineupChannelDB) GetEnabledChannelsForGuideProvider(providerID int) ([]LineupChannel, error) {
	channels := make([]LineupChannel, 0)

	inQuery := squirrel.Select("id").From("guide_source_channel").Where(squirrel.Eq{"guide_id": providerID})

	// Using DebugSqlizer is unsafe but Squirrel doesn't support WHERE IN subqueries.
	sql, args, sqlGenErr := squirrel.Select("*").From("lineup_channel").Where(squirrel.Eq{"guide_channel_id": squirrel.DebugSqlizer(inQuery)}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}

	err := db.SQL.Select(&channels, sql, args)
	if err != nil {
		return nil, err
	}
	// Need to get the address and port number to properly fill
	lineup, lineupErr := db.Collection.Lineup.GetLineupByID(channels[0].LineupID, false)
	if lineupErr != nil {
		return nil, lineupErr
	}
	for idx, channel := range channels {
		channel.lineup = lineup
		channel.Fill(db.Collection)
		channels[idx] = channel
	}
	return channels, err
}

// GetEnabledChannelsForVideoProvider returns a slice of LineupChannels for the given video provider ID.
func (db *LineupChannelDB) GetEnabledChannelsForVideoProvider(providerID int) ([]LineupChannel, error) {
	channels := make([]LineupChannel, 0)

	inQuery := squirrel.Select("id").From("video_source_track").Where(squirrel.Eq{"video_source_id": providerID})

	// Using DebugSqlizer is unsafe but Squirrel doesn't support WHERE IN subqueries.
	sql, args, sqlGenErr := squirrel.Select("*").From("lineup_channel").Where(squirrel.Eq{"video_track_id": squirrel.DebugSqlizer(inQuery)}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}

	err := db.SQL.Select(&channels, sql, args)
	if err != nil {
		return nil, err
	}
	// Need to get the address and port number to properly fill
	lineup, lineupErr := db.Collection.Lineup.GetLineupByID(channels[0].LineupID, false)
	if lineupErr != nil {
		return nil, lineupErr
	}
	for idx, channel := range channels {
		channel.lineup = lineup
		channel.Fill(db.Collection)
		channels[idx] = channel
	}
	return channels, err
}
