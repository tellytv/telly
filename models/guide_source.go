package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tellytv/telly/internal/providers"
)

// GuideSourceDB is a struct containing initialized the SQL connection as well as the APICollection.
type GuideSourceDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newGuideSourceDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *GuideSourceDB {
	db := &GuideSourceDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *GuideSourceDB) tableName() string {
	return "guide_source"
}

type GuideSource struct {
	ID         int        `db:"id"`
	Name       string     `db:"name"`
	Provider   string     `db:"provider"`
	Username   string     `db:"username"`
	Password   string     `db:"password"`
	URL        string     `db:"xmltv_url"`
	ImportedAt *time.Time `db:"imported_at"`

	Channels []GuideSourceChannel `db:"-"`
}

func (g *GuideSource) ProviderConfiguration() *providers.Configuration {
	return &providers.Configuration{
		Name:     g.Name,
		Provider: g.Provider,
		Username: g.Username,
		Password: g.Password,
		EPG:      g.URL,
	}
}

// GuideSourceAPI contains all methods for the User struct
type GuideSourceAPI interface {
	InsertGuideSource(guideSourceStruct GuideSource) (*GuideSource, error)
	DeleteGuideSource(guideSourceID int) (*GuideSource, error)
	UpdateGuideSource(guideSourceID int, description string) (*GuideSource, error)
	GetGuideSourceByID(id int) (*GuideSource, error)
	GetAllGuideSources(includeChannels bool) ([]GuideSource, error)
}

const baseGuideSourceQuery string = `
SELECT
  G.id,
  G.name,
  G.provider,
  G.username,
  G.password,
  G.xmltv_url,
  G.imported_at
  FROM guide_source G`

// InsertGuideSource inserts a new GuideSource into the database.
func (db *GuideSourceDB) InsertGuideSource(guideSourceStruct GuideSource) (*GuideSource, error) {
	guideSource := GuideSource{}
	res, err := db.SQL.NamedExec(`
    INSERT INTO guide_source (name, provider, username, password, xmltv_url)
    VALUES (:name, :provider, :username, :password, :xmltv_url);`, guideSourceStruct)
	if err != nil {
		return &guideSource, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &guideSource, rowIDErr
	}
	err = db.SQL.Get(&guideSource, "SELECT * FROM guide_source WHERE id = $1", rowID)
	return &guideSource, err
}

// GetGuideSourceByID returns a single GuideSource for the given ID.
func (db *GuideSourceDB) GetGuideSourceByID(id int) (*GuideSource, error) {
	var guideSource GuideSource
	err := db.SQL.Get(&guideSource, fmt.Sprintf(`%s WHERE G.id = $1`, baseGuideSourceQuery), id)
	return &guideSource, err
}

// DeleteGuideSource marks a guideSource with the given ID as deleted.
func (db *GuideSourceDB) DeleteGuideSource(guideSourceID int) (*GuideSource, error) {
	guideSource := GuideSource{}
	err := db.SQL.Get(&guideSource, `DELETE FROM guide_source WHERE id = $1`, guideSourceID)
	return &guideSource, err
}

// UpdateGuideSource updates a guideSource.
func (db *GuideSourceDB) UpdateGuideSource(guideSourceID int, description string) (*GuideSource, error) {
	guideSource := GuideSource{}
	err := db.SQL.Get(&guideSource, `UPDATE guide_source SET description = $2 WHERE id = $1 RETURNING *`, guideSourceID, description)
	return &guideSource, err
}

// GetAllGuideSources returns all video sources in the database.
func (db *GuideSourceDB) GetAllGuideSources(includeChannels bool) ([]GuideSource, error) {
	sources := make([]GuideSource, 0)
	err := db.SQL.Select(&sources, baseGuideSourceQuery)
	if includeChannels {
		newSources := make([]GuideSource, 0)
		for _, source := range sources {
			allChannels, channelsErr := db.Collection.GuideSourceChannel.GetChannelsForGuideSource(source.ID)
			if channelsErr != nil {
				return nil, channelsErr
			}
			source.Channels = append(source.Channels, allChannels...)
			newSources = append(newSources, source)
		}
		return newSources, nil
	}
	return sources, err
}
