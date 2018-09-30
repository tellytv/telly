// Package context provides Telly specific context functions like SQLite access, along with initialized API clients and other packages such as models.
package context

import (
	ctx "context"
	"os"

	"github.com/gobuffalo/packr"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // the SQLite driver
	"github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/internal/guideproviders"
	"github.com/tellytv/telly/internal/models"
	"github.com/tellytv/telly/internal/streamsuite"
	"github.com/tellytv/telly/internal/videoproviders"
)

// CContext is a context struct that gets passed around the application.
type CContext struct {
	API                  *models.APICollection
	Ctx                  ctx.Context
	GuideSourceProviders map[int]guideproviders.GuideProvider
	Log                  *logrus.Logger
	Streams              map[string]*streamsuite.Stream
	Tuners               map[int]chan bool
	VideoSourceProviders map[int]videoproviders.VideoProvider

	RawSQL *sqlx.DB
}

// Copy returns a cloned version of the input CContext minus the User and Device fields.
func (cc *CContext) Copy() *CContext {
	return &CContext{
		API:                  cc.API,
		Ctx:                  cc.Ctx,
		GuideSourceProviders: cc.GuideSourceProviders,
		Log:                  cc.Log,
		RawSQL:               cc.RawSQL,
		Streams:              cc.Streams,
		Tuners:               cc.Tuners,
		VideoSourceProviders: cc.VideoSourceProviders,
	}
}

// NewCContext returns an initialized CContext struct
func NewCContext(log *logrus.Logger) (*CContext, error) {

	if log == nil {
		log = &logrus.Logger{
			Out: os.Stderr,
			Formatter: &logrus.TextFormatter{
				FullTimestamp: true,
			},
			Hooks: make(logrus.LevelHooks),
			Level: logrus.DebugLevel,
		}
	}

	theCtx := ctx.Background()

	sql, dbErr := sqlx.Open("sqlite3", viper.GetString("database.file"))
	if dbErr != nil {
		log.WithError(dbErr).Panicln("Unable to open database")
	}

	if _, execErr := sql.Exec(`PRAGMA foreign_keys = ON;`); execErr != nil {
		log.WithError(execErr).Panicln("error enabling foreign keys")
	}

	log.Debugln("Checking migrations status and running any required migrations...")

	migrate.SetTable("migrations")

	migrations := &migrate.PackrMigrationSource{
		Box: packr.NewBox("../../migrations"),
	}

	numMigrations, upErr := migrate.Exec(sql.DB, "sqlite3", migrations, migrate.Up)
	if upErr != nil {
		log.WithError(upErr).Panicln("error migrating database to newer version")
	}
	if numMigrations > 0 {
		log.Debugf("successfully applied %d migrations to database", numMigrations)
	}

	api := models.NewAPICollection(sql, log)

	tuners := make(map[int]chan bool)

	guideSources, guideSourcesErr := api.GuideSource.GetAllGuideSources(false)
	if guideSourcesErr != nil {
		log.WithError(guideSourcesErr).Panicln("error initializing video sources")
	}

	guideSourceProvidersMap := make(map[int]guideproviders.GuideProvider)

	for _, guideSource := range guideSources {
		providerCfg := guideSource.ProviderConfiguration()
		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			log.WithError(providerErr).Panicln("error initializing provider")
		}
		guideSourceProvidersMap[guideSource.ID] = provider
	}

	videoSources, videoSourcesErr := api.VideoSource.GetAllVideoSources(false)
	if videoSourcesErr != nil {
		log.WithError(videoSourcesErr).Panicln("error initializing video sources")
	}

	videoSourceProvidersMap := make(map[int]videoproviders.VideoProvider)

	for _, videoSource := range videoSources {
		log.Infof("Initializing video source %s (%s)", videoSource.Name, videoSource.Provider)
		providerCfg := videoSource.ProviderConfiguration()
		provider, providerErr := providerCfg.GetProvider()
		if providerErr != nil {
			log.WithError(providerErr).Panicln("error initializing provider")
		}
		videoSourceProvidersMap[videoSource.ID] = provider
	}

	context := &CContext{
		API:                  api,
		Ctx:                  theCtx,
		GuideSourceProviders: guideSourceProvidersMap,
		Log:                  log,
		RawSQL:               sql,
		Streams:              make(map[string]*streamsuite.Stream),
		Tuners:               tuners,
		VideoSourceProviders: videoSourceProvidersMap,
	}

	log.Debugln("Context: Context build complete")

	return context, nil
}
