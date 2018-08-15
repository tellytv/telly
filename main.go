package main

import (
	"encoding/json"
	fflag "flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	opts = config{}

	exposedChannels = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "exposed_channels_total",
			Help: "Number of exposed channels.",
		},
	)

	safeStringsRegex = regexp.MustCompile(`(?m)(username|password|token)=[\w=]+(&?)`)

	stringSafer = func(input string) string {
		ret := input
		if strings.HasPrefix(input, "username=") {
			ret = "username=hunter1"
		} else if strings.HasPrefix(input, "password=") {
			ret = "password=hunter2"
		} else if strings.HasPrefix(input, "token=") {
			ret = "token=bm90Zm9yeW91" // "notforyou"
		}
		if strings.HasSuffix(input, "&") {
			return fmt.Sprintf("%s&", ret)
		}
		return ret
	}
)

func main() {

	// Discovery flags
	flag.Int("discovery.device-id", 12345678, "8 digits used to uniquely identify the device. $(TELLY_DISCOVERY_DEVICE_ID)")
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
	flag.String("web.listen-address", "localhost:6077", "Address to listen on for web interface and telemetry $(TELLY_WEB_LISTEN_ADDRESS)")
	flag.String("web.base-address", "localhost:6077", "The address to expose via discovery. Useful with reverse proxy $(TELLY_WEB_BASE_ADDRESS)")

	// Log flags
	flag.String("log.level", logrus.InfoLevel.String(), "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal] $(TELLY_LOG_LEVEL)")
	flag.Bool("log.requests", false, "Log HTTP requests $(TELLY_LOG_REQUESTS)")

	// IPTV flags
	flag.String("iptv.playlist", "", "Path to an M3U file on disk or at a URL. $(TELLY_IPTV_PLAYLIST)")
	flag.Int("iptv.streams", 1, "Number of concurrent streams allowed $(TELLY_IPTV_STREAMS)")
	flag.Int("iptv.starting-channel", 10000, "The channel number to start exposing from. $(TELLY_IPTV_STARTING_CHANNEL)")
	flag.Bool("iptv.xmltv-channels", true, "Use channel numbers discovered via XMLTV file, if provided. $(TELLY_IPTV_XMLTV_CHANNELS)")

	flag.CommandLine.AddGoFlagSet(fflag.CommandLine)
	flag.Parse()
	viper.BindPFlags(flag.CommandLine)
	viper.SetConfigName("telly.config") // name of config file (without extension)
	viper.AddConfigPath("/etc/telly/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.telly") // call multiple times to add many search paths
	viper.AddConfigPath(".")            // optionally look for config in the working directory
	viper.SetEnvPrefix(namespace)
	viper.AutomaticEnv()
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.WithError(err).Panicln("fatal error while reading config file:")
		}
	}

	log.Infoln("Starting telly", version.Info())
	log.Infoln("Build context", version.BuildContext())

	prometheus.MustRegister(version.NewCollector("telly"), exposedChannels)

	level, parseLevelErr := logrus.ParseLevel(viper.GetString("log.level"))
	if parseLevelErr != nil {
		log.WithError(parseLevelErr).Panicln("error setting log level!")
	}
	log.SetLevel(level)

	if log.Level == logrus.DebugLevel {
		js, _ := json.MarshalIndent(viper.AllSettings(), "", "    ")
		log.Debugf("Loaded configuration %s", js)
	}

	if viper.IsSet("filter.regexstr") {
		if _, regexErr := regexp.Compile(viper.GetString("filter.regex")); regexErr != nil {
			log.WithError(regexErr).Panicln("Error when compiling regex, is it valid?")
		}
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

	if GetTCPAddr("web.base-address").IP.IsUnspecified() {
		log.Panicln("base URL is set to 0.0.0.0, this will not work. please use the --web.baseaddress option and set it to the (local) ip address telly is running on.")
	}

	if GetTCPAddr("web.listenaddress").IP.IsUnspecified() && GetTCPAddr("web.base-address").IP.IsLoopback() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	viper.Set("discovery.device-friendly-name", fmt.Sprintf("HDHomerun (%s)", viper.GetString("discovery.device-friendly-name")))
	viper.Set("discovery.device-uuid", fmt.Sprintf("%d-AE2A-4E54-BBC9-33AF7D5D6A92", viper.GetInt("discovery.device-id")))

	if flag.Lookup("iptv.playlist").Changed {
		viper.Set("playlists.default.m3u", flag.Lookup("iptv.playlist").Value.String())
	}

	lineup := NewLineup()

	log.Infof("Loaded %d channels into the lineup", lineup.FilteredTracksCount)

	if lineup.FilteredTracksCount > 420 {
		log.Panicf("telly has loaded more than 420 channels (%d) into the lineup. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You must use regular expressions to filter out channels. You can also start another Telly instance.", lineup.FilteredTracksCount)
	}

	serve(lineup)
}
