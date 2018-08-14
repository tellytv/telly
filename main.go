package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	namespace = "telly"
	log       = logrus.New()
	opts      = config{}

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
	kingpin.Flag("discovery.device-id", "8 digits used to uniquely identify the device. $(TELLY_DISCOVERY_DEVICE_ID)").Envar("TELLY_DISCOVERY_DEVICE_ID").Default("12345678").IntVar(&opts.DeviceID)
	kingpin.Flag("discovery.device-friendly-name", "Name exposed via discovery. Useful if you are running two instances of telly and want to differentiate between them $(TELLY_DISCOVERY_DEVICE_FRIENDLY_NAME)").Envar("TELLY_DISCOVERY_DEVICE_FRIENDLY_NAME").Default("telly").StringVar(&opts.FriendlyName)
	kingpin.Flag("discovery.device-auth", "Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICE_AUTH)").Envar("TELLY_DISCOVERY_DEVICE_AUTH").Default("telly123").Hidden().StringVar(&opts.DeviceAuth)
	kingpin.Flag("discovery.device-manufacturer", "Manufacturer exposed via discovery. $(TELLY_DISCOVERY_DEVICE_MANUFACTURER)").Envar("TELLY_DISCOVERY_DEVICE_MANUFACTURER").Default("Silicondust").StringVar(&opts.Manufacturer)
	kingpin.Flag("discovery.device-model-number", "Model number exposed via discovery. $(TELLY_DISCOVERY_DEVICE_MODEL_NUMBER)").Envar("TELLY_DISCOVERY_DEVICE_MODEL_NUMBER").Default("HDTC-2US").StringVar(&opts.ModelNumber)
	kingpin.Flag("discovery.device-firmware-name", "Firmware name exposed via discovery. $(TELLY_DISCOVERY_DEVICE_FIRMWARE_NAME)").Envar("TELLY_DISCOVERY_DEVICE_FIRMWARE_NAME").Default("hdhomeruntc_atsc").StringVar(&opts.FirmwareName)
	kingpin.Flag("discovery.device-firmware-version", "Firmware version exposed via discovery. $(TELLY_DISCOVERY_DEVICE_FIRMWARE_VERSION)").Envar("TELLY_DISCOVERY_DEVICE_FIRMWARE_VERSION").Default("20150826").StringVar(&opts.FirmwareVersion)
	kingpin.Flag("discovery.ssdp", "Turn on SSDP announcement of telly to the local network $(TELLY_DISCOVERY_SSDP)").Envar("TELLY_DISCOVERY_SSDP").Default("true").BoolVar(&opts.SSDP)

	// Regex/filtering flags
	kingpin.Flag("filter.regex-inclusive", "Whether the provided regex is inclusive (whitelisting) or exclusive (blacklisting). If true (--filter.regex-inclusive), only channels matching the provided regex pattern will be exposed. If false (--no-filter.regex-inclusive), only channels NOT matching the provided pattern will be exposed. $(TELLY_FILTER_REGEX_MODE)").Envar("TELLY_FILTER_REGEX_MODE").Default("false").BoolVar(&opts.RegexInclusive)
	kingpin.Flag("filter.regex", "Use regex to filter for channels that you want. A basic example would be .*UK.*. $(TELLY_FILTER_REGEX)").Envar("TELLY_FILTER_REGEX").Default(".*").RegexpVar(&opts.Regex)

	// Web flags
	kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry $(TELLY_WEB_LISTEN_ADDRESS)").Envar("TELLY_WEB_LISTEN_ADDRESS").Default("localhost:6077").TCPVar(&opts.ListenAddress)
	kingpin.Flag("web.base-address", "The address to expose via discovery. Useful with reverse proxy $(TELLY_WEB_BASE_ADDRESS)").Envar("TELLY_WEB_BASE_ADDRESS").Default("localhost:6077").TCPVar(&opts.BaseAddress)

	// Log flags
	kingpin.Flag("log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal] $(TELLY_LOG_LEVEL)").Envar("TELLY_LOG_LEVEL").Default(logrus.InfoLevel.String()).StringVar(&opts.LogLevel)
	kingpin.Flag("log.requests", "Log HTTP requests $(TELLY_LOG_REQUESTS)").Envar("TELLY_LOG_REQUESTS").Default("false").BoolVar(&opts.LogRequests)

	// IPTV flags
	kingpin.Flag("iptv.playlist", "Location of playlist M3U file. Can be on disk or a URL. $(TELLY_IPTV_PLAYLIST)").Envar("TELLY_IPTV_PLAYLIST").Default("iptv.m3u").StringsVar(&opts.Playlists)
	kingpin.Flag("iptv.streams", "Number of concurrent streams allowed $(TELLY_IPTV_STREAMS)").Envar("TELLY_IPTV_STREAMS").Default("1").IntVar(&opts.ConcurrentStreams)
	kingpin.Flag("iptv.direct", "If true, stream URLs will not be obfuscated to hide them from Plex. $(TELLY_IPTV_DIRECT)").Envar("TELLY_IPTV_DIRECT").Default("false").BoolVar(&opts.DirectMode)
	kingpin.Flag("iptv.starting-channel", "The channel number to start exposing from. $(TELLY_IPTV_STARTING_CHANNEL)").Envar("TELLY_IPTV_STARTING_CHANNEL").Default("10000").IntVar(&opts.StartingChannel)

	kingpin.Version(version.Print("telly"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting telly", version.Info())
	log.Infoln("Build context", version.BuildContext())

	prometheus.MustRegister(version.NewCollector("telly"), exposedChannels)

	level, parseLevelErr := logrus.ParseLevel(opts.LogLevel)
	if parseLevelErr != nil {
		log.WithError(parseLevelErr).Panicln("error setting log level!")
	}
	log.SetLevel(level)

	opts.FriendlyName = fmt.Sprintf("HDHomerun (%s)", opts.FriendlyName)
	opts.DeviceUUID = fmt.Sprintf("%d-AE2A-4E54-BBC9-33AF7D5D6A92", opts.DeviceID)

	if opts.BaseAddress.IP.IsUnspecified() {
		log.Panicln("base URL is set to 0.0.0.0, this will not work. please use the --web.base-address option and set it to the (local) ip address telly is running on.")
	}

	if opts.ListenAddress.IP.IsUnspecified() && opts.BaseAddress.IP.IsLoopback() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	if len(opts.Playlists) == 1 && opts.Playlists[0] == "iptv.m3u" {
		log.Warnln("using default m3u option, 'iptv.m3u'. launch telly with the --iptv.playlist=yourfile.m3u option to change this!")
	}

	opts.lineup = NewLineup(opts)

	for _, playlistPath := range opts.Playlists {
		if addErr := opts.lineup.AddPlaylist(playlistPath); addErr != nil {
			log.WithError(addErr).Panicln("error adding new playlist to lineup")
		}
	}

	log.Infof("Loaded %d channels into the lineup", opts.lineup.FilteredTracksCount)

	if opts.lineup.FilteredTracksCount > 420 {
		log.Warnln("telly has loaded more than 420 channels into the lineup. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You have been warned!")
	}

	serve(opts)
}
