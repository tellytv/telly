package main

import (
	"encoding/json"
	fflag "flag"
	"fmt"
	"net"
	"os"

	"github.com/prometheus/common/version"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/api"
	"github.com/tellytv/telly/commands"
	"github.com/tellytv/telly/context"
	"github.com/tellytv/telly/utils"
)

var (
	namespace            = "telly"
	namespaceWithVersion = fmt.Sprintf("%s %s", namespace, version.Version)
	log                  = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}
)

func main() {

	// Web flags
	flag.StringP("web.listen-address", "l", "localhost:6077", "Address to listen on for web interface, API and telemetry $(TELLY_WEB_LISTEN_ADDRESS)")

	// Log flags
	flag.String("log.level", logrus.InfoLevel.String(), "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal] $(TELLY_LOG_LEVEL)")
	flag.Bool("log.requests", false, "Log HTTP requests $(TELLY_LOG_REQUESTS)")

	// Misc flags
	flag.StringP("config.file", "c", "", "Path to your config file. If not set, configuration is searched for in the current working directory, $HOME/.telly/ and /etc/telly/. If provided, it will override all other arguments and environment variables. $(TELLY_CONFIG_FILE)")
	flag.StringP("database.file", "d", "./telly.db", "Path to the SQLite3 database. If not set, defaults to telly.db. $(TELLY_DATABASE_FILE)")
	flag.Bool("version", false, "Show application version")

	flag.CommandLine.AddGoFlagSet(fflag.CommandLine)

	flag.Parse()
	if bindErr := viper.BindPFlags(flag.CommandLine); bindErr != nil {
		log.WithError(bindErr).Panicln("error binding flags to viper")
	}

	if flag.Lookup("version").Changed {
		fmt.Println(version.Print(namespace))
		os.Exit(0)
	}

	if flag.Lookup("config.file").Changed {
		viper.SetConfigFile(flag.Lookup("config.file").Value.String())
	} else {
		viper.SetConfigName("telly.config")
		viper.AddConfigPath("/etc/telly/")
		viper.AddConfigPath("$HOME/.telly")
		viper.AddConfigPath(".")
		viper.SetEnvPrefix(namespace)
		viper.AutomaticEnv()
	}

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.WithError(err).Panicln("fatal error while reading config file:")
		}
	}

	level, parseLevelErr := logrus.ParseLevel(viper.GetString("log.level"))
	if parseLevelErr != nil {
		log.WithError(parseLevelErr).Panicln("error setting log level!")
	}
	log.SetLevel(level)

	log.Infoln("telly is preparing to go live", version.Info())
	log.Debugln("Build context", version.BuildContext())

	validateConfig()

	viper.Set("discovery.device-friendly-name", fmt.Sprintf("HDHomerun (%s)", viper.GetString("discovery.device-friendly-name")))
	viper.Set("discovery.device-uuid", fmt.Sprintf("%d-AE2A-4E54-BBC9-33AF7D5D6A92", viper.GetInt("discovery.device-id")))

	if log.Level == logrus.DebugLevel {
		js, jsErr := json.MarshalIndent(viper.AllSettings(), "", "    ")
		if jsErr != nil {
			log.WithError(jsErr).Panicln("error marshal indenting viper config to JSON")
		}
		log.Debugf("Loaded configuration %s", js)
	}

	cc, err := context.NewCContext()
	if err != nil {
		log.WithError(err).Panicln("Couldn't create context")
	}

	lineups, lineupsErr := cc.API.Lineup.GetEnabledLineups(true)
	if lineupsErr != nil {
		log.WithError(lineupsErr).Panicln("Error getting all enabled lineups")
	}

	c := cron.New()

	for _, lineup := range lineups {
		api.StartTuner(cc, &lineup)

		// videoProviders := make(map[int]string)
		guideProviders := make(map[int]string)
		for _, channel := range lineup.Channels {
			// videoProviders[channel.VideoTrack.VideoSource.ID] = channel.VideoTrack.VideoSource.UpdateFrequency
			guideProviders[channel.GuideChannel.GuideSource.ID] = channel.GuideChannel.GuideSource.UpdateFrequency
		}

		// for videoProviderID, updateFrequencey := range videoProviders {
		// 	c.AddFunc(updateFrequencey, func() { commands.StartFireVideoUpdates(cc, videoProviderID) })
		// }

		for guideProviderID, updateFrequencey := range guideProviders {
			c.AddFunc(updateFrequencey, func() { commands.StartFireGuideUpdates(cc, guideProviderID) })
		}
	}

	c.Start()
	log.Infof("CRON ENTRIES %+v", c.Entries())

	api.ServeAPI(cc)
}

func validateConfig() {
	var addrErr error
	if _, addrErr = net.ResolveTCPAddr("tcp", viper.GetString("web.listenaddress")); addrErr != nil {
		log.WithError(addrErr).Panic("Error when parsing Listen address, please check the address and try again.")
		return
	}

	if _, addrErr = net.ResolveTCPAddr("tcp", viper.GetString("web.base-address")); addrErr != nil {
		log.WithError(addrErr).Panic("Error when parsing Base addresses, please check the address and try again.")
		return
	}

	if utils.GetTCPAddr("web.base-address").IP.IsUnspecified() {
		log.Panicln("base URL is set to 0.0.0.0, this will not work. please use the --web.baseaddress option and set it to the (local) ip address telly is running on.")
	}

	if utils.GetTCPAddr("web.listenaddress").IP.IsUnspecified() && utils.GetTCPAddr("web.base-address").IP.IsLoopback() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}
}
