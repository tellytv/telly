package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	ssdp "github.com/koron/go-ssdp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	m3u "github.com/tombowditch/telly-m3u-parser"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var log = logrus.New()

type config struct {
	DeviceID          int
	DeviceUUID        string
	FilterRegex       bool
	FilterUKTV        bool
	M3UPath           string
	ConcurrentStreams int
	UseRegex          string
	DeviceAuth        string
	FriendlyName      string
	TempPath          string
	DirectMode        bool
	SSDP              bool

	LogRequests bool
	LogLevel    string

	ListenAddress *net.TCPAddr
	BaseAddress   *net.TCPAddr
}

// DiscoveryData contains data about telly to expose in the HDHomeRun format for Plex detection.
type DiscoveryData struct {
	FriendlyName    string
	Manufacturer    string
	ModelNumber     string
	FirmwareName    string
	TunerCount      int
	FirmwareVersion string
	DeviceID        string
	DeviceAuth      string
	BaseURL         string
	LineupURL       string
}

// LineupStatus exposes the status of the channel lineup.
type LineupStatus struct {
	ScanInProgress int
	ScanPossible   int
	Source         string
	SourceList     []string
}

// LineupItem is a single channel found in the playlist.
type LineupItem struct {
	GuideNumber string
	GuideName   string
	URL         string
}

func logRequestHandler(logRequests bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logRequests {
			log.Debugf("%s -> %s %s", r.RemoteAddr, r.Method, r.RequestURI)

			if r.Method == "POST" {
				r.ParseForm()
				log.Debugln("POST body:", r.Form.Encode())
			}
		}

		next.ServeHTTP(w, r)

	})
}

func downloadFile(url string, dest string) error {
	out, err := os.Create(dest)
	defer out.Close()
	if err != nil {
		return fmt.Errorf("could not create file: %s %s", dest, err.Error())
	}

	log.Debugf("Downloading file %s to %s", url, dest)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("could not download: %s", err.Error())
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("could not download to output file: %s", err.Error())
	}

	return nil
}

func buildChannels(directMode bool, baseURL string, usedTracks []m3u.Track) []LineupItem {
	lineup := make([]LineupItem, 0)
	gn := 10000

	for _, track := range usedTracks {

		var finalName string
		if track.TvgName == "" {
			finalName = track.Name
		} else {
			finalName = track.TvgName
		}

		// base64 url
		fullTrackURI := track.URI
		if !directMode {
			trackURI := base64.StdEncoding.EncodeToString([]byte(track.URI))
			fullTrackURI = fmt.Sprintf("http://%s/stream/%s", baseURL, trackURI)
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

func sendAlive(advertiser *ssdp.Advertiser) {
	aliveTick := time.Tick(15 * time.Second)

	for {
		select {
		case <-aliveTick:
			if err := advertiser.Alive(); err != nil {
				log.Errorln(err.Error())
				os.Exit(1)
			}
		}
	}
}

func advertiseSSDP(listenAddress, deviceName, deviceUUID string) (*ssdp.Advertiser, error) {
	log.Debugf("Advertising telly as %s (%s)", deviceName, deviceUUID)

	adv, err := ssdp.Advertise(
		"upnp:rootdevice",
		fmt.Sprintf("uuid:%s::upnp:rootdevice", deviceUUID),
		fmt.Sprintf("http://%s/device.xml", listenAddress),
		deviceName,
		1800)

	if err != nil {
		return nil, err
	}

	go sendAlive(adv)

	return adv, nil
}

func base64StreamHandler(w http.ResponseWriter, r *http.Request, base64StreamURL string) {
	decodedStreamURI, err := base64.StdEncoding.DecodeString(base64StreamURL)

	if err != nil {
		log.Errorf("Invalid base64: %s: %s", base64StreamURL, err.Error())
		w.WriteHeader(400)
		return
	}

	log.Debugln("Redirecting to:", string(decodedStreamURI))
	http.Redirect(w, r, string(decodedStreamURI), 301)
}

func main() {

	var (
		opts = config{}
	)

	// Discovery flags
	kingpin.Flag("discovery.deviceid", "8 digits. Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICEID)").Envar("TELLY_DISCOVERY_DEVICEID").Default("12345678").IntVar(&opts.DeviceID)
	kingpin.Flag("discovery.friendlyname", "Name exposed via discovery. Useful if you are running two instances of telly and want to differentiate between them $(TELLY_DISCOVERY_FRIENDLYNAME)").Envar("TELLY_DISCOVERY_FRIENDLYNAME").Default("telly").StringVar(&opts.FriendlyName)
	kingpin.Flag("discovery.deviceauth", "Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICEAUTH)").Envar("TELLY_DISCOVERY_DEVICEAUTH").Default("telly123").StringVar(&opts.DeviceAuth)
	kingpin.Flag("discovery.ssdp", "Turn on SSDP announcement of telly to the local network $(TELLY_DISCOVERY_SSDP)").Envar("TELLY_DISCOVERY_SSDP").Default("true").BoolVar(&opts.SSDP)

	// Regex/filtering flags
	kingpin.Flag("filter.filterregex", "Use regex to attempt to strip out bogus channels (SxxExx, 24/7 channels, etc) $(TELLY_FILTER_FILTERREGEX)").Envar("TELLY_FILTER_FILTERREGEX").Default("false").BoolVar(&opts.FilterRegex)
	kingpin.Flag("filter.uktv-preset", "Only index channels with 'UK' in the name. $(TELLY_FILTER_UKTV_PRESET)").Envar("TELLY_FILTER_UKTV_PRESET").Default("false").BoolVar(&opts.FilterUKTV)
	kingpin.Flag("filter.useregex", "Use regex to filter for channels that you want. Basic example would be .*UK.*. When using this -filter.uktv-preset and -filter.filterregex will NOT work. $(TELLY_FILTER_USEREGEX)").Envar("TELLY_FILTER_USEREGEX").Default(".*").StringVar(&opts.UseRegex)

	// Web flags
	kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry $(TELLY_WEB_LISTEN_ADDRESS)").Envar("TELLY_WEB_LISTEN_ADDRESS").Default("localhost:6077").TCPVar(&opts.ListenAddress)
	kingpin.Flag("web.base-address", "The address to expose via discovery. Useful with reverse proxy $(TELLY_WEB_BASE_ADDRESS)").Envar("TELLY_WEB_BASE_ADDRESS").Default("localhost:6077").TCPVar(&opts.BaseAddress)

	// Log flags
	kingpin.Flag("log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal] $(TELLY_LOG_LEVEL)").Envar("TELLY_LOG_LEVEL").Default(logrus.InfoLevel.String()).StringVar(&opts.LogLevel)
	kingpin.Flag("log.requests", "Log HTTP requests $(TELLY_LOG_REQUESTS)").Envar("TELLY_LOG_REQUESTS").Default("false").BoolVar(&opts.LogRequests)

	// IPTV flags
	kingpin.Flag("iptv.playlist", "Location of playlist m3u file. Can be on disk or a URL $(TELLY_IPTV_PLAYLIST)").Envar("TELLY_IPTV_PLAYLIST").Default("iptv.m3u").StringVar(&opts.M3UPath)
	kingpin.Flag("iptv.streams", "Amount of concurrent streams allowed $(TELLY_IPTV_STREAMS)").Envar("TELLY_IPTV_STREAMS").Default("1").IntVar(&opts.ConcurrentStreams)
	kingpin.Flag("iptv.direct", "Does not encode the stream URL and redirect to the correct one $(TELLY_IPTV_DIRECT)").Envar("TELLY_IPTV_DIRECT").Default("false").BoolVar(&opts.DirectMode)

	kingpin.Version(version.Print("telly"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting telly", version.Info())
	log.Infoln("Build context", version.BuildContext())

	level, parseLevelErr := logrus.ParseLevel(opts.LogLevel)
	if parseLevelErr != nil {
		log.WithError(parseLevelErr).Panicln("Error setting log level!")
	}
	log.SetLevel(level)

	opts.DeviceUUID = fmt.Sprintf("%d-AE2A-4E54-BBC9-33AF7D5D6A92", opts.DeviceID)

	usedTracks := make([]m3u.Track, 0)

	if opts.M3UPath == "iptv.m3u" {
		log.Warnln("using default m3u option, 'iptv.m3u'. launch telly with the -playlist=yourfile.m3u option to change this!")
	} else {
		if strings.HasPrefix(strings.ToLower(opts.M3UPath), "http") {
			// FIXME: Don't need a tempPath, just load into memory.
			tempFilename := "FIXME"

			err := downloadFile(opts.M3UPath, tempFilename)
			if err != nil {
				log.Errorln(err.Error())
				os.Exit(1)
			}

			opts.M3UPath = tempFilename
			defer os.Remove(tempFilename)
		}
	}

	log.Debugf("Reading m3u file %s...", opts.M3UPath)
	playlist, err := m3u.Parse(opts.M3UPath)
	if err != nil {
		log.Errorln("unable to read m3u file, error below")
		panic(err)
	}

	episodeRegex, _ := regexp.Compile("S\\d{1,3}E\\d{1,3}")
	twentyFourSevenRegex, _ := regexp.Compile("24/7")
	ukTv, _ := regexp.Compile("UK")

	userRegex, _ := regexp.Compile(opts.UseRegex)

	for _, track := range playlist.Tracks {
		if opts.UseRegex == ".*" {
			if opts.FilterRegex && opts.FilterUKTV {
				if !episodeRegex.MatchString(track.Name) {
					if !twentyFourSevenRegex.MatchString(track.Name) {
						if ukTv.MatchString(track.Name) {
							usedTracks = append(usedTracks, track)
						}
					}
				}
			} else if opts.FilterRegex && !opts.FilterUKTV {
				if !episodeRegex.MatchString(track.Name) {
					if !twentyFourSevenRegex.MatchString(track.Name) {
						usedTracks = append(usedTracks, track)
					}
				}

			} else if !opts.FilterRegex && opts.FilterUKTV {
				if ukTv.MatchString(track.Name) {
					usedTracks = append(usedTracks, track)
				}
			} else {
				usedTracks = append(usedTracks, track)
			}
		} else {
			// Use regex
			if userRegex.MatchString(track.Name) {
				usedTracks = append(usedTracks, track)
			}
		}
	}

	if !opts.FilterRegex {
		log.Warnln("telly is not attempting to strip out unneeded channels, please use the flag -filterregex if telly returns too many channels")
	}

	if !opts.FilterUKTV {
		log.Warnln("telly is currently not filtering for only uk television. if you would like it to, please use the flag -uktv")
	}

	channelCount := len(usedTracks)

	log.Infof("found %d channels", channelCount)

	if channelCount > 420 {
		log.Warnln("telly has loaded more than 420 channels. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You have been warned!")
	}

	opts.FriendlyName = fmt.Sprintf("HDHomerun (%s)", opts.FriendlyName)

	log.Debugln("creating discovery data")
	discoveryData := DiscoveryData{
		FriendlyName:    opts.FriendlyName,
		Manufacturer:    "Silicondust",
		ModelNumber:     "HDTC-2US",
		FirmwareName:    "hdhomeruntc_atsc",
		TunerCount:      opts.ConcurrentStreams,
		FirmwareVersion: "20150826",
		DeviceID:        strconv.Itoa(opts.DeviceID),
		DeviceAuth:      opts.DeviceAuth,
		BaseURL:         fmt.Sprintf("http://%s", opts.BaseAddress),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", opts.BaseAddress),
	}

	log.Debugln("creating lineup status")
	lineupStatus := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}

	log.Debugln("creating device xml")
	deviceXML := fmt.Sprintf(`<root xmlns="urn:schemas-upnp-org:device-1-0">
    <specVersion>
        <major>1</major>
        <minor>0</minor>
    </specVersion>
    <URLBase>%s</URLBase>
    <device>
        <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
        <friendlyName>%s</friendlyName>
        <manufacturer>%s</manufacturer>
        <modelName>%s</modelName>
        <modelNumber>%s</modelNumber>
        <serialNumber></serialNumber>
        <UDN>uuid:%s</UDN>
    </device>
</root>`, discoveryData.BaseURL, discoveryData.FriendlyName, discoveryData.Manufacturer, discoveryData.ModelNumber, discoveryData.ModelNumber, discoveryData.DeviceID)

	log.Debugln("creating webserver routes")

	h := http.NewServeMux()

	h.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discoveryData)
	})

	h.HandleFunc("/lineup_status.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineupStatus)
	})

	h.HandleFunc("/lineup.post", func(w http.ResponseWriter, r *http.Request) {
		// empty
	})

	h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(deviceXML))
	})

	h.HandleFunc("/device.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(deviceXML))
	})

	log.Debugln("Building lineup")
	lineupItems := buildChannels(opts.DirectMode, opts.BaseAddress.String(), usedTracks)

	h.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineupItems)
	})

	h.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		u, _ := url.Parse(r.RequestURI)
		uriPart := strings.Replace(u.Path, "/stream/", "", 1)
		log.Debugf("Parsing URI %s to %s", r.RequestURI, uriPart)
		base64StreamHandler(w, r, uriPart)
	})

	if opts.BaseAddress.IP.IsUnspecified() {
		log.Errorln("Your base URL is set to 0.0.0.0, this will not work")
		log.Errorln("Please use the -base option and set it to the (local) IP address telly is running on")
	}

	if opts.ListenAddress.IP.IsUnspecified() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	if opts.SSDP {
		log.Debugln("advertising telly service on network")
		_, ssdpErr := advertiseSSDP(opts.ListenAddress.String(), opts.FriendlyName, opts.DeviceUUID)
		if ssdpErr != nil {
			log.WithError(ssdpErr).Warnln("telly cannot advertise over ssdp")
		}
	}

	log.Infof("listening on %s", opts.ListenAddress)
	if err := http.ListenAndServe(opts.ListenAddress.String(), logRequestHandler(opts.LogRequests, h)); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}
}
