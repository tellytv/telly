package main

import (
	"encoding/json"
	fflag "flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/prometheus/common/version"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tellytv/telly/internal/api"
	"github.com/tellytv/telly/internal/commands"
	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/models"
	"github.com/tellytv/telly/internal/utils"
)

var (
	namespace = "telly"
	log       = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}
)

func main() {

	// Discovery flags
	flag.String("discovery.device-id", "12345678", "8 alpha-numeric characters used to uniquely identify the device. $(TELLY_DISCOVERY_DEVICE_ID)")
	flag.String("discovery.device-friendly-name", "telly", "Name exposed via discovery. Useful if you are running two instances of telly and want to differentiate between them $(TELLY_DISCOVERY_DEVICE_FRIENDLY_NAME)")
	flag.String("discovery.device-auth", "telly123", "Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICE_AUTH)")
	flag.String("discovery.device-manufacturer", "Silicondust", "Manufacturer exposed via discovery. $(TELLY_DISCOVERY_DEVICE_MANUFACTURER)")
	flag.String("discovery.device-model-number", "HDTC-2US", "Model number exposed via discovery. $(TELLY_DISCOVERY_DEVICE_MODEL_NUMBER)")
	flag.String("discovery.device-firmware-name", "hdhomeruntc_atsc", "Firmware name exposed via discovery. $(TELLY_DISCOVERY_DEVICE_FIRMWARE_NAME)")
	flag.String("discovery.device-firmware-version", "20150826", "Firmware version exposed via discovery. $(TELLY_DISCOVERY_DEVICE_FIRMWARE_VERSION)")
	flag.Bool("discovery.ssdp", true, "Turn on SSDP announcement of telly to the local network $(TELLY_DISCOVERY_SSDP)")

	// Regex/filtering flags
	flag.Bool("filter.regex-inclusive", false, "Whether the provided regex is inclusive (whitelisting) or exclusive (blacklisting). If true (--filter.regex-inclusive), only channels matching the provided regex pattern will be exposed. If false (--no-filter.regex-inclusive), only channels NOT matching the provided pattern will be exposed. $(TELLY_FILTER_REGEX_INCLUSIVE)")
	flag.String("filter.regex", ".*", "Use regex to filter for channels that you want. A basic example would be .*UK.*. $(TELLY_FILTER_REGEX)")

	// Web flags
	flag.StringP("web.listen-address", "l", ":6077", "Address to listen on for web interface, API and telemetry $(TELLY_WEB_LISTEN_ADDRESS)")

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
		viper.SetConfigName("config")
		viper.AddConfigPath("/etc/telly/")
		viper.AddConfigPath("$HOME/.telly")
		viper.AddConfigPath("/telly") // Docker exposes this as a volume
		viper.AddConfigPath(".")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	viper.Set("discovery.device-uuid", fmt.Sprintf("%s-AE2A-4E54-BBC9-33AF7D5D6A92", viper.GetString("discovery.device-id")))

	if log.Level == logrus.DebugLevel {
		js, jsErr := json.MarshalIndent(viper.AllSettings(), "", "    ")
		if jsErr != nil {
			log.WithError(jsErr).Panicln("error marshal indenting viper config to JSON")
		}
		log.Debugf("Loaded configuration %s", js)
	}

	cc, err := context.NewCContext(log)
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

		videoProviders := make(map[int]*models.VideoSource)
		guideProviders := make(map[int]*models.GuideSource)
		for _, channel := range lineup.Channels {
			videoProviders[channel.VideoTrack.VideoSource.ID] = channel.VideoTrack.VideoSource
			guideProviders[channel.GuideChannel.GuideSource.ID] = channel.GuideChannel.GuideSource
		}

		for _, videoSource := range videoProviders {
			if videoSource.UpdateFrequency == "" {
				continue
			}
			commands.StartFireVideoUpdates(cc, videoSource)
			if addErr := c.AddFunc(videoSource.UpdateFrequency, func() { commands.StartFireVideoUpdates(cc, videoSource) }); addErr != nil {
				log.WithError(addErr).Errorln("error when adding video source to scheduled background jobs")
			}
		}

		for _, guideSource := range guideProviders {
			if guideSource.UpdateFrequency == "" {
				continue
			}
			commands.StartFireGuideUpdates(cc, guideSource)
			if addErr := c.AddFunc(guideSource.UpdateFrequency, func() { commands.StartFireGuideUpdates(cc, guideSource) }); addErr != nil {
				log.WithError(addErr).Errorln("error when adding guide source to scheduled background jobs")
			}
		}
	}

	c.Start()

	api.ServeAPI(cc)
}

func validateConfig() {
	if !(viper.IsSet("source")) {
		log.Warnln("There is no source element in the configuration, the config file is likely missing.")
	}

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
