package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
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

type VideoSourceTrack struct {
	ID             int             `db:"id"`
	VideoSourceID  int             `db:"video_source_id"`
	Name           string          `db:"name"`
	Tags           json.RawMessage `db:"tags"`
	RawLine        string          `db:"raw_line"`
	StreamURL      string          `db:"stream_url"`
	HighDefinition bool            `db:"hd"`
	ImportedAt     *time.Time      `db:"imported_at"`
}

// VideoSourceTrackAPI contains all methods for the User struct
type VideoSourceTrackAPI interface {
	InsertVideoSourceTrack(trackStruct VideoSourceTrack) (*VideoSourceTrack, error)
	DeleteVideoSourceTrack(trackID string) (*VideoSourceTrack, error)
	UpdateVideoSourceTrack(trackID, description string) (*VideoSourceTrack, error)
	GetVideoSourceTrackByID(id string) (*VideoSourceTrack, error)
	GetTracksForVideoSource(videoSourceID int) ([]VideoSourceTrack, error)
}

const baseVideoSourceTrackQuery string = `
SELECT
  T.id,
  T.video_source_id,
  T.name,
  T.tags,
  T.raw_line,
  T.stream_url,
  T.hd,
  T.imported_at
  FROM video_source_track T`

// InsertVideoSourceTrack inserts a new VideoSourceTrack into the database.
func (db *VideoSourceTrackDB) InsertVideoSourceTrack(trackStruct VideoSourceTrack) (*VideoSourceTrack, error) {
	track := VideoSourceTrack{}
	res, err := db.SQL.NamedExec(`
    INSERT INTO video_source_track (video_source_id, name, tags, raw_line, stream_url, hd)
    VALUES (:video_source_id, :name, :tags, :raw_line, :stream_url, :hd);`, trackStruct)
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
func (db *VideoSourceTrackDB) GetVideoSourceTrackByID(id string) (*VideoSourceTrack, error) {
	var track VideoSourceTrack
	err := db.SQL.Get(&track, fmt.Sprintf(`%s WHERE T.id = $1`, baseVideoSourceTrackQuery), id)
	return &track, err
}

// DeleteVideoSourceTrack marks a track with the given ID as deleted.
func (db *VideoSourceTrackDB) DeleteVideoSourceTrack(trackID string) (*VideoSourceTrack, error) {
	track := VideoSourceTrack{}
	err := db.SQL.Get(&track, `DELETE FROM video_source_track WHERE id = $1`, trackID)
	return &track, err
}

// UpdateVideoSourceTrack updates a track.
func (db *VideoSourceTrackDB) UpdateVideoSourceTrack(trackID, description string) (*VideoSourceTrack, error) {
	track := VideoSourceTrack{}
	err := db.SQL.Get(&track, `UPDATE video_source_track SET description = $2 WHERE id = $1 RETURNING *`, trackID, description)
	return &track, err
}

// GetTracksForVideoSource returns a slice of VideoSourceTracks for the given video source ID.
func (db *VideoSourceTrackDB) GetTracksForVideoSource(videoSourceID int) ([]VideoSourceTrack, error) {
	tracks := make([]VideoSourceTrack, 0)
	err := db.SQL.Select(&tracks, fmt.Sprintf(`%s WHERE T.video_source_id = $1`, baseVideoSourceTrackQuery), videoSourceID)
	return tracks, err
}
