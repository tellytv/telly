package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// LineupDB is a struct containing initialized the SQL connection as well as the APICollection.
type LineupDB struct {
	SQL        *sqlx.DB
	Collection *APICollection
}

func newLineupDB(
	SQL *sqlx.DB,
	Collection *APICollection,
) *LineupDB {
	db := &LineupDB{
		SQL:        SQL,
		Collection: Collection,
	}
	return db
}

func (db *LineupDB) tableName() string {
	return "lineup"
}

type SQLLineup struct {
	ID          int        `db:"id"`
	Name        string     `db:"name"`
	ChannelsStr string     `db:"channels"`
	CreatedAt   *time.Time `db:"created_at"`
}

// LineupAPI contains all methods for the User struct
type LineupAPI interface {
	InsertLineup(lineupStruct SQLLineup) (*SQLLineup, error)
	DeleteLineup(lineupID string) (*SQLLineup, error)
	UpdateLineup(lineupID, description string) (*SQLLineup, error)
	GetLineupByID(id string) (*SQLLineup, error)
}

const baseLineupQuery string = `
SELECT
  L.id,
  L.name,
  L.channels,
  L.created_at
  FROM lineups L`

// InsertLineup inserts a new Lineup into the database.
func (db *LineupDB) InsertLineup(lineupStruct SQLLineup) (*SQLLineup, error) {
	lineup := SQLLineup{}
	rows, err := db.SQL.NamedQuery(`
    INSERT INTO lineups (name, channels, created_at)
    VALUES (name, :channels, :created_at)
    RETURNING *`, lineupStruct)
	if err != nil {
		return &lineup, err
	}
	for rows.Next() {
		err := rows.StructScan(&lineup)
		if err != nil {
			return &lineup, err
		}
	}
	return &lineup, nil
}

// GetLineupByID returns a single Lineup for the given ID.
func (db *LineupDB) GetLineupByID(id string) (*SQLLineup, error) {
	var lineup SQLLineup
	err := db.SQL.Get(&lineup, fmt.Sprintf(`%s WHERE L.id = $1`, baseLineupQuery), id)
	return &lineup, err
}

// DeleteLineup marks a lineup with the given ID as deleted.
func (db *LineupDB) DeleteLineup(lineupID string) (*SQLLineup, error) {
	lineup := SQLLineup{}
	err := db.SQL.Get(&lineup, `DELETE FROM lineups WHERE id = $1`, lineupID)
	return &lineup, err
}

// UpdateLineup updates a lineup.
func (db *LineupDB) UpdateLineup(lineupID, description string) (*SQLLineup, error) {
	lineup := SQLLineup{}
	err := db.SQL.Get(&lineup, `UPDATE lineups SET description = $2 WHERE id = $1 RETURNING *`, lineupID, description)
	return &lineup, err
}
