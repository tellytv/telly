package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tellytv/telly/internal/providers"
)

// VideoSourceDB is a struct containing initialized the SQL connection as well as the APICollection.
type VideoSourceDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newVideoSourceDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *VideoSourceDB {
	db := &VideoSourceDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *VideoSourceDB) tableName() string {
	return "video_source"
}

type VideoSource struct {
	ID         int        `db:"id"          json:"id,omitempty"`
	Name       string     `db:"name"        json:"name,omitempty"`
	Provider   string     `db:"provider"    json:"provider,omitempty"`
	Username   string     `db:"username"    json:"username,omitempty"`
	Password   string     `db:"password"    json:"password,omitempty"`
	M3UURL     string     `db:"m3u_url"     json:"m3uURL,omitempty"`
	MaxStreams int        `db:"max_streams" json:"maxStreams,omitempty"`
	ImportedAt *time.Time `db:"imported_at" json:"importedAt,omitempty"`

	Tracks []VideoSourceTrack `db:"tracks"  json:"tracks,omitempty"`
}

func (v *VideoSource) ProviderConfiguration() *providers.Configuration {
	return &providers.Configuration{
		Name:     v.Name,
		Provider: v.Provider,
		Username: v.Username,
		Password: v.Password,
		M3U:      v.M3UURL,
	}
}

// VideoSourceAPI contains all methods for the User struct
type VideoSourceAPI interface {
	InsertVideoSource(videoSourceStruct VideoSource) (*VideoSource, error)
	DeleteVideoSource(videoSourceID string) (*VideoSource, error)
	UpdateVideoSource(videoSourceID, description string) (*VideoSource, error)
	GetVideoSourceByID(id string) (*VideoSource, error)
	GetAllVideoSources(includeTracks bool) ([]VideoSource, error)
}

const baseVideoSourceQuery string = `
SELECT
  V.id,
  V.name,
  V.provider,
  V.username,
  V.password,
  V.m3u_url,
  V.max_streams,
  V.imported_at
  FROM video_source V`

// InsertVideoSource inserts a new VideoSource into the database.
func (db *VideoSourceDB) InsertVideoSource(videoSourceStruct VideoSource) (*VideoSource, error) {
	videoSource := VideoSource{}
	res, err := db.SQL.NamedExec(`
    INSERT INTO video_source (name, provider, username, password, m3u_url, max_streams)
    VALUES (:name, :provider, :username, :password, :m3u_url, :max_streams);`, videoSourceStruct)
	if err != nil {
		return &videoSource, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &videoSource, rowIDErr
	}
	err = db.SQL.Get(&videoSource, "SELECT * FROM video_source WHERE id = $1", rowID)
	return &videoSource, err
}

// GetVideoSourceByID returns a single VideoSource for the given ID.
func (db *VideoSourceDB) GetVideoSourceByID(id string) (*VideoSource, error) {
	var videoSource VideoSource
	err := db.SQL.Get(&videoSource, fmt.Sprintf(`%s WHERE V.id = $1`, baseVideoSourceQuery), id)
	return &videoSource, err
}

// DeleteVideoSource marks a videoSource with the given ID as deleted.
func (db *VideoSourceDB) DeleteVideoSource(videoSourceID string) (*VideoSource, error) {
	videoSource := VideoSource{}
	err := db.SQL.Get(&videoSource, `DELETE FROM video_source WHERE id = $1`, videoSourceID)
	return &videoSource, err
}

// UpdateVideoSource updates a videoSource.
func (db *VideoSourceDB) UpdateVideoSource(videoSourceID, description string) (*VideoSource, error) {
	videoSource := VideoSource{}
	err := db.SQL.Get(&videoSource, `UPDATE video_source SET description = $2 WHERE id = $1 RETURNING *`, videoSourceID, description)
	return &videoSource, err
}

// GetAllVideoSources returns all video sources in the database.
func (db *VideoSourceDB) GetAllVideoSources(includeTracks bool) ([]VideoSource, error) {
	sources := make([]VideoSource, 0)
	err := db.SQL.Select(&sources, baseVideoSourceQuery)
	if includeTracks {
		newSources := make([]VideoSource, 0)
		for _, source := range sources {
			allTracks, tracksErr := db.Collection.VideoSourceTrack.GetTracksForVideoSource(source.ID)
			if tracksErr != nil {
				return nil, tracksErr
			}
			source.Tracks = append(source.Tracks, allTracks...)
			newSources = append(newSources, source)
		}
		return newSources, nil
	}
	return sources, err
}
