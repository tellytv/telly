package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	upnp "github.com/NebulousLabs/go-upnp/goupnp"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
	squirrel "gopkg.in/Masterminds/squirrel.v1"
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

// DiscoveryData contains data about telly to expose in the HDHomeRun format for Plex detection.
type DiscoveryData struct {
	FriendlyName    string
	Manufacturer    string
	ModelName       string
	ModelNumber     string
	FirmwareName    string
	TunerCount      int
	FirmwareVersion string
	DeviceID        string
	DeviceAuth      string
	BaseURL         string
	LineupURL       string
	DeviceUUID      string
}

// UPNP returns the UPNP representation of the DiscoveryData.
func (d *DiscoveryData) UPNP() upnp.RootDevice {
	return upnp.RootDevice{
		SpecVersion: upnp.SpecVersion{
			Major: 1, Minor: 0,
		},
		URLBaseStr: d.BaseURL,
		Device: upnp.Device{
			DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
			FriendlyName: fmt.Sprintf("HDHomerun (%s)", d.FriendlyName),
			Manufacturer: d.Manufacturer,
			ManufacturerURL: upnp.URLField{
				Str: "http://www.silicondust.com/",
			},
			ModelName:        d.ModelName,
			ModelNumber:      d.ModelNumber,
			ModelDescription: fmt.Sprintf("%s %s", d.ModelNumber, d.ModelName),
			ModelURL: upnp.URLField{
				Str: "http://www.silicondust.com/",
			},
			SerialNumber: d.DeviceID,
			UDN:          fmt.Sprintf("uuid:%s", strings.ToUpper(d.DeviceUUID)),
			PresentationURL: upnp.URLField{
				Str: "/",
			},
		},
	}
}

// Lineup describes a collection of channels exposed to the world with associated configuration.
type Lineup struct {
	ID               int        `db:"id"`
	Name             string     `db:"name"`
	SSDP             bool       `db:"ssdp"`
	ListenAddress    string     `db:"listen_address"`
	DiscoveryAddress string     `db:"discovery_address"`
	Port             int        `db:"port"`
	Tuners           int        `db:"tuners"`
	Manufacturer     string     `db:"manufacturer"`
	ModelName        string     `db:"model_name"`
	ModelNumber      string     `db:"model_number"`
	FirmwareName     string     `db:"firmware_name"`
	FirmwareVersion  string     `db:"firmware_version"`
	DeviceID         string     `db:"device_id"`
	DeviceAuth       string     `db:"device_auth"`
	DeviceUUID       string     `db:"device_uuid"`
	StreamTransport  string     `db:"stream_transport"`
	CreatedAt        *time.Time `db:"created_at"`

	Channels []LineupChannel
}

// GetDiscoveryData returns DiscoveryData for the Lineup.
func (s *Lineup) GetDiscoveryData() DiscoveryData {
	baseAddr := fmt.Sprintf("http://%s:%d", s.DiscoveryAddress, s.Port)
	return DiscoveryData{
		FriendlyName:    s.Name,
		Manufacturer:    s.Manufacturer,
		ModelName:       s.ModelName,
		ModelNumber:     s.ModelNumber,
		FirmwareName:    s.FirmwareName,
		TunerCount:      s.Tuners,
		FirmwareVersion: s.FirmwareVersion,
		DeviceID:        s.DeviceID,
		DeviceAuth:      s.DeviceAuth,
		BaseURL:         baseAddr,
		LineupURL:       fmt.Sprintf("%s/lineup.json", baseAddr),
		DeviceUUID:      s.DeviceUUID,
	}
}

// LineupAPI contains all methods for the User struct
type LineupAPI interface {
	InsertLineup(lineupStruct Lineup) (*Lineup, error)
	DeleteLineup(lineupID int) (*Lineup, error)
	UpdateLineup(lineupID int, description string) (*Lineup, error)
	GetLineupByID(id int, withChannels bool) (*Lineup, error)
	GetEnabledLineups(withChannels bool) ([]Lineup, error)
}

const baseLineupQuery string = `
SELECT
  L.id,
  L.name,
  L.ssdp,
  L.listen_address,
  L.discovery_address,
  L.port,
  L.tuners,
  L.manufacturer,
  L.model_name,
  L.model_number,
  L.firmware_name,
  L.firmware_version,
  L.device_id,
  L.device_auth,
  L.device_uuid,
  L.stream_transport,
  L.created_at
  FROM lineup L`

// InsertLineup inserts a new Lineup into the database.
func (db *LineupDB) InsertLineup(lineupStruct Lineup) (*Lineup, error) {
	lineup := Lineup{}
	if lineupStruct.Manufacturer == "" {
		lineupStruct.Manufacturer = "Silicondust"
	}
	if lineupStruct.ModelName == "" {
		lineupStruct.ModelName = "HDHomeRun EXTEND"
	}
	if lineupStruct.ModelNumber == "" {
		lineupStruct.ModelNumber = "HDTC-2US"
	}
	if lineupStruct.FirmwareName == "" {
		lineupStruct.FirmwareName = "hdhomeruntc_atsc"
	}
	if lineupStruct.FirmwareVersion == "" {
		lineupStruct.FirmwareVersion = "20150826"
	}
	if lineupStruct.DeviceID == "" {
		bytes := make([]byte, 20)
		if _, err := rand.Read(bytes); err != nil {
			return &lineup, fmt.Errorf("error when generating random device id: %s", err)
		}
		lineupStruct.DeviceID = strings.ToUpper(hex.EncodeToString(bytes)[:8])
	}
	if lineupStruct.DeviceAuth == "" {
		lineupStruct.DeviceAuth = "telly"
	}
	if lineupStruct.DeviceUUID == "" {
		lineupStruct.DeviceUUID = uuid.Must(uuid.NewV4()).String()
	}
	if lineupStruct.StreamTransport == "" {
		lineupStruct.StreamTransport = "http"
	}
	res, err := db.SQL.NamedExec(`
    INSERT INTO lineup (name, ssdp, listen_address, discovery_address, port, tuners, manufacturer, model_name, model_number, firmware_name, firmware_version, device_id, device_auth, device_uuid, stream_transport)
    VALUES (:name, :ssdp, :listen_address, :discovery_address, :port, :tuners, :manufacturer, :model_name, :model_number, :firmware_name, :firmware_version, :device_id, :device_auth, :device_uuid, :stream_transport)`, lineupStruct)
	if err != nil {
		return &lineup, err
	}
	rowID, rowIDErr := res.LastInsertId()
	if rowIDErr != nil {
		return &lineup, rowIDErr
	}
	err = db.SQL.Get(&lineup, "SELECT * FROM lineup WHERE id = $1", rowID)
	return &lineup, err
}

// GetLineupByID returns a single Lineup for the given ID.
func (db *LineupDB) GetLineupByID(id int, withChannels bool) (*Lineup, error) {
	var lineup Lineup
	sql, args, sqlGenErr := squirrel.Select("*").From("lineup").Where(squirrel.Eq{"id": id}).ToSql()
	if sqlGenErr != nil {
		return nil, sqlGenErr
	}
	err := db.SQL.Get(&lineup, sql, args)
	if withChannels {
		channels, channelsErr := db.Collection.LineupChannel.GetChannelsForLineup(lineup.ID, true)
		if channelsErr != nil {
			return nil, channelsErr
		}
		lineup.Channels = channels
	}
	return &lineup, err
}

// DeleteLineup marks a lineup with the given ID as deleted.
func (db *LineupDB) DeleteLineup(lineupID int) (*Lineup, error) {
	lineup := Lineup{}
	err := db.SQL.Get(&lineup, `DELETE FROM lineup WHERE id = $1`, lineupID)
	return &lineup, err
}

// UpdateLineup updates a lineup.
func (db *LineupDB) UpdateLineup(lineupID int, description string) (*Lineup, error) {
	lineup := Lineup{}
	err := db.SQL.Get(&lineup, `UPDATE lineup SET description = $2 WHERE id = $1 RETURNING *`, lineupID, description)
	return &lineup, err
}

// GetEnabledLineups returns all enabled lineups in the database.
func (db *LineupDB) GetEnabledLineups(withChannels bool) ([]Lineup, error) {
	lineups := make([]Lineup, 0)
	err := db.SQL.Select(&lineups, baseLineupQuery)
	if withChannels {
		for idx, lineup := range lineups {
			channels, channelsErr := db.Collection.LineupChannel.GetChannelsForLineup(lineup.ID, true)
			if channelsErr != nil {
				return nil, channelsErr
			}
			lineup.Channels = channels
			lineups[idx] = lineup
		}
	}
	return lineups, err
}
