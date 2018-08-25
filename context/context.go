// Package context provides Telly specific context functions like SQLite access, along with initialized API clients and other packages such as models.
package context

import (
	ctx "context"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/models"
)

// CContext is a context struct that gets passed around the application.
type CContext struct {
	API    *models.APICollection
	Ctx    ctx.Context
	Lineup *models.Lineup
	Log    *logrus.Logger
	Tuners map[int]chan bool

	RawSQL *sqlx.DB
}

// Copy returns a cloned version of the input CContext minus the User and Device fields.
func (cc *CContext) Copy() *CContext {
	return &CContext{
		API:    cc.API,
		Ctx:    cc.Ctx,
		Lineup: cc.Lineup,
		Log:    cc.Log,
		Tuners: cc.Tuners,
		RawSQL: cc.RawSQL,
	}
}

// NewCContext returns an initialized CContext struct
func NewCContext() (*CContext, error) {

	theCtx := ctx.Background()

	log := &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.InfoLevel,
	}

	gooseLog := &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}

	sql, dbErr := sqlx.Open("sqlite3", viper.GetString("database.file"))
	if dbErr != nil {
		log.WithError(dbErr).Panicln("Unable to open database")
	}

	sql.Exec(`PRAGMA foreign_keys = ON;`)

	log.Debugln("Checking migrations status and running any required migrations...")

	goose.SetLogger(gooseLog)

	if dialectErr := goose.SetDialect("sqlite3"); dialectErr != nil {
		log.WithError(dialectErr).Panicln("error setting migrations dialect")
	}

	if statusErr := goose.Status(sql.DB, "./migrations"); statusErr != nil {
		log.WithError(statusErr).Panicln("error getting migrations status")
	}

	api := models.NewAPICollection(theCtx, sql)

	// lineup := models.NewLineup()

	// if scanErr := lineup.Scan(); scanErr != nil {
	// 	log.WithError(scanErr).Panicln("Error scanning lineup!")
	// }

	tuners := make(map[int]chan bool)

	context := &CContext{
		API: api,
		Ctx: theCtx,
		Log: log,
		// Lineup: lineup,
		Tuners: tuners,
		RawSQL: sql,
	}

	log.Debugln("Context: Context build complete")

	return context, nil
}
