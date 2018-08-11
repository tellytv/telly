package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	ssdp "github.com/koron/go-ssdp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	"github.com/tombowditch/telly/m3u"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	log  = logrus.New()
	opts = config{}

	exposedChannels = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "exposed_channels_total",
			Help: "Number of exposed channels.",
		},
	)
)

func main() {

	// Discovery flags
	kingpin.Flag("discovery.deviceid", "8 digits used to uniquely identify the device. $(TELLY_DISCOVERY_DEVICEID)").Envar("TELLY_DISCOVERY_DEVICEID").Default("12345678").IntVar(&opts.DeviceID)
	kingpin.Flag("discovery.friendlyname", "Name exposed via discovery. Useful if you are running two instances of telly and want to differentiate between them $(TELLY_DISCOVERY_FRIENDLYNAME)").Envar("TELLY_DISCOVERY_FRIENDLYNAME").Default("telly").StringVar(&opts.FriendlyName)
	kingpin.Flag("discovery.deviceauth", "Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICEAUTH)").Envar("TELLY_DISCOVERY_DEVICEAUTH").Default("telly123").Hidden().StringVar(&opts.DeviceAuth)
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
	kingpin.Flag("iptv.playlist", "Location of playlist M3U file. Can be on disk or a URL. $(TELLY_IPTV_PLAYLIST)").Envar("TELLY_IPTV_PLAYLIST").Default("iptv.m3u").StringVar(&opts.M3UPath)
	kingpin.Flag("iptv.streams", "Number of concurrent streams allowed $(TELLY_IPTV_STREAMS)").Envar("TELLY_IPTV_STREAMS").Default("1").IntVar(&opts.ConcurrentStreams)
	kingpin.Flag("iptv.direct", "Does not encode the stream URL and redirect to the correct one $(TELLY_IPTV_DIRECT)").Envar("TELLY_IPTV_DIRECT").Default("false").BoolVar(&opts.DirectMode)

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

	opts.DeviceUUID = fmt.Sprintf("%d-AE2A-4E54-BBC9-33AF7D5D6A92", opts.DeviceID)

	if opts.BaseAddress.IP.IsUnspecified() {
		log.Panicln("base URL is set to 0.0.0.0, this will not work. please use the --web.base-address option and set it to the (local) ip address telly is running on.")
	}

	if opts.ListenAddress.IP.IsUnspecified() && opts.BaseAddress.IP.IsLoopback() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	if opts.M3UPath == "iptv.m3u" {
		log.Warnln("using default m3u option, 'iptv.m3u'. launch telly with the --iptv.playlist=yourfile.m3u option to change this!")
	}

	m3uReader, readErr := getM3U(opts)
	if readErr != nil {
		log.WithError(readErr).Panicln("error getting m3u")
	}

	playlist, err := m3u.Decode(m3uReader)
	if err != nil {
		log.WithError(err).Panicln("unable to parse m3u file")
	}

	channels, filterErr := filterTracks(playlist.Tracks)
	if filterErr != nil {
		log.WithError(filterErr).Panicln("error during filtering of channels, check your regex and try again")
	}

	log.Debugln("Building lineup")

	opts.lineup = buildLineup(opts, channels)

	channelCount := len(channels)
	exposedChannels.Set(float64(channelCount))
	log.Infof("found %d channels", channelCount)

	if channelCount > 420 {
		log.Warnln("telly has loaded more than 420 channels. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You have been warned!")
	}

	opts.FriendlyName = fmt.Sprintf("HDHomerun (%s)", opts.FriendlyName)

	serve(opts)
}

func buildLineup(opts config, channels []Track) []LineupItem {
	lineup := make([]LineupItem, 0)
	gn := 10000

	for _, track := range channels {

		var finalName string
		if track.TvgName == "" {
			finalName = track.Name
		} else {
			finalName = track.TvgName
		}

		// base64 url
		fullTrackURI := track.URI
		if !opts.DirectMode {
			trackURI := base64.StdEncoding.EncodeToString([]byte(track.URI))
			fullTrackURI = fmt.Sprintf("http://%s/stream/%s", opts.BaseAddress.String(), trackURI)
		}

		if strings.Contains(track.URI, ".m3u8") {
			log.Warnln("your .m3u contains .m3u8's. Plex has either stopped supporting m3u8 or it is a bug in a recent version - please use .ts! telly will automatically convert these in a future version. See telly github issue #108")
		}

		lu := LineupItem{
			GuideNumber: strconv.Itoa(gn),
			GuideName:   finalName,
			URL:         fullTrackURI,
		}

		lineup = append(lineup, lu)

		gn = gn + 1
	}

	return lineup
}

func setupSSDP(baseAddress, deviceName, deviceUUID string) (*ssdp.Advertiser, error) {
	log.Debugf("Advertising telly as %s (%s)", deviceName, deviceUUID)

	adv, err := ssdp.Advertise(
		"upnp:rootdevice",
		fmt.Sprintf("uuid:%s::upnp:rootdevice", deviceUUID),
		fmt.Sprintf("http://%s/device.xml", baseAddress),
		deviceName,
		1800)

	if err != nil {
		return nil, err
	}

	go func(advertiser *ssdp.Advertiser) {
		aliveTick := time.Tick(15 * time.Second)

		for {
			select {
			case <-aliveTick:
				if err := advertiser.Alive(); err != nil {
					log.WithError(err).Panicln("error when sending ssdp heartbeat")
				}
			}
		}
	}(adv)

	return adv, nil
}

func getM3U(opts config) (io.Reader, error) {
	if strings.HasPrefix(strings.ToLower(opts.M3UPath), "http") {
		log.Debugf("Downloading M3U from %s", opts.M3UPath)
		resp, err := http.Get(opts.M3UPath)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return resp.Body, nil
	}

	log.Debugf("Reading M3U file %s...", opts.M3UPath)

	return os.Open(opts.M3UPath)
}

func filterTracks(tracks []*m3u.Track) ([]Track, error) {
	allowedTracks := make([]Track, 0)

	for _, oldTrack := range tracks {
		track := Track{Track: oldTrack}
		if unmarshalErr := oldTrack.UnmarshalTags(&track); unmarshalErr != nil {
			return nil, unmarshalErr
		}

		if opts.Regex.MatchString(track.Name) == opts.RegexInclusive {
			allowedTracks = append(allowedTracks, track)
		}
	}

	return allowedTracks, nil
}
