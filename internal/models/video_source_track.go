package models

import (
	"time"

	"github.com/jmoiron/sqlx"
	squirrel "gopkg.in/Masterminds/squirrel.v1"
)

// VideoSourceTrackDB is a struct containing initialized the SQL connection as well as the APICollection.
type VideoSourceTrackDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newVideoSourceTrackDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *VideoSourceTrackDB {
	db := &VideoSourceTrackDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *VideoSourceTrackDB) tableName() string {
	return "video_source_track"
}

// VideoSourceTrack is a single stream available from a video source.
type VideoSourceTrack struct {
	ID            int        `db:"id"`
	VideoSourceID int        `db:"video_source_id"`
	Name          string     `db:"name"`
	StreamID      int        `db:"stream_id"`
	Logo          string     `db:"logo"`
	Type          string     `db:"type"`
	Category      string     `db:"category"`
	EPGID         string     `db:"epg_id"`
	ImportedAt    *time.Time `db:"imported_at"`

	VideoSource     *VideoSource
	VideoSourceName string
}

// VideoSourceTrackAPI contains all methods for the User struct
type VideoSourceTrackAPI interface {
	InsertVideoSourceTrack(trackStruct VideoSourceTrack) (*VideoSourceTrack, error)
	DeleteVideoSourceTrack(trackID int) (*VideoSourceTrack, error)
	UpdateVideoSourceTrack(providerID, trackID int, trackStruct VideoSourceTrack) error
	GetVideoSourceTrackByID(id int, expanded bool) (*VideoSourceTrack, error)
	GetTracksForVideoSource(videoSourceID int) ([]VideoSourceTrack, error)
}

// nolint
const baseVideoSourceTrackQuery string = `
SELECT
  T.id,
  T.video_source_id,
  T.name,
  T.stream_id,
  T.logo,
  T.type,
  T.category,
  T.epg_id,
  T.imported_at
  FROM video_source_track T`

// InsertVideoSourceTrack inserts a new VideoSourceTrack into the database.
func (db *VideoSourceTrackDB) InsertVideoSourceTrack(trackStruct VideoSourceTrack) (*VideoSourceTrack, error) {
	track := VideoSourceTrack{}
	res, err := db.SQL.NamedExec(`
    INSERT OR REPLACE INTO video_source_track (video_source_id, name, stream_id, logo, type, category, epg_id)
    VALUES (:video_source_id, :name, :stream_id, :logo, :type, :category, :epg_id);`, trackStruct)
	if err != nil {
		return &track, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &track, rowIDErr
	}
	err = db.SQL.Get(&track, "SELECT * FROM video_source_track WHERE id = $1", rowID)
	return &track, err
}

// GetVideoSourceTrackByID returns a single VideoSourceTrack for the given ID.
func (db *VideoSourceTrackDB) GetVideoSourceTrackByID(id int, expanded bool) (*VideoSourceTrack, error) {
	var track VideoSourceTrack

	sql, args, sqlGenErr := squirrel.Select("*").From("video_source_track").Where(squirrel.Eq{"id": id}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}

	err := db.SQL.Get(&track, sql, args...)
	if expanded {
		video, videoErr := db.Collection.VideoSource.GetVideoSourceByID(track.VideoSourceID)
		if videoErr != nil {
			return nil, videoErr
		}
		track.VideoSource = video
	}
	return &track, err
}

// DeleteVideoSourceTrack marks a track with the given ID as deleted.
func (db *VideoSourceTrackDB) DeleteVideoSourceTrack(trackID int) (*VideoSourceTrack, error) {
	track := VideoSourceTrack{}
	err := db.SQL.Get(&track, `DELETE FROM video_source_track WHERE id = $1`, trackID)
	return &track, err
}

// UpdateVideoSourceTrack updates a track.
func (db *VideoSourceTrackDB) UpdateVideoSourceTrack(providerID, streamID int, trackStruct VideoSourceTrack) error {
	_, err := db.SQL.Exec(`UPDATE video_source_track SET category = ?, epg_id = ? WHERE video_source_id = ? AND stream_id = ?`, trackStruct.Category, trackStruct.EPGID, providerID, streamID)
	return err
}

// GetTracksForVideoSource returns a slice of VideoSourceTracks for the given video source ID.
func (db *VideoSourceTrackDB) GetTracksForVideoSource(videoSourceID int) ([]VideoSourceTrack, error) {
	tracks := make([]VideoSourceTrack, 0)

	sql, args, sqlGenErr := squirrel.Select("*").From("video_source_track").Where(squirrel.Eq{"video_source_id": videoSourceID}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}

	err := db.SQL.Select(&tracks, sql, args...)
	return tracks, err
}
