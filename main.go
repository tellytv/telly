package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ssdp "github.com/koron/go-ssdp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
	"github.com/tombowditch/telly/m3u"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var log = logrus.New()

func buildLineup(directMode bool, baseURL string, usedTracks []Track) []LineupItem {
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
				log.WithError(err).Panicln("Error when sending SSDP heartbeat")
			}
		}
	}
}

func advertiseSSDP(baseAddress, deviceName, deviceUUID string) (*ssdp.Advertiser, error) {
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

	go sendAlive(adv)

	return adv, nil
}

func main() {

	var opts = config{}

	// Discovery flags
	kingpin.Flag("discovery.deviceid", "8 digits. Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICEID)").Envar("TELLY_DISCOVERY_DEVICEID").Default("12345678").IntVar(&opts.DeviceID)
	kingpin.Flag("discovery.friendlyname", "Name exposed via discovery. Useful if you are running two instances of telly and want to differentiate between them $(TELLY_DISCOVERY_FRIENDLYNAME)").Envar("TELLY_DISCOVERY_FRIENDLYNAME").Default("telly").StringVar(&opts.FriendlyName)
	kingpin.Flag("discovery.deviceauth", "Only change this if you know what you're doing $(TELLY_DISCOVERY_DEVICEAUTH)").Envar("TELLY_DISCOVERY_DEVICEAUTH").Default("telly123").StringVar(&opts.DeviceAuth)
	kingpin.Flag("discovery.ssdp", "Turn on SSDP announcement of telly to the local network $(TELLY_DISCOVERY_SSDP)").Envar("TELLY_DISCOVERY_SSDP").Default("true").BoolVar(&opts.SSDP)

	// Regex/filtering flags
	kingpin.Flag("filter.filterregex", "Use regex to attempt to strip out bogus channels (SxxExx, 24/7 channels, etc) $(TELLY_FILTER_FILTERREGEX)").Envar("TELLY_FILTER_FILTERREGEX").Default("false").BoolVar(&opts.FilterRegex)
	kingpin.Flag("filter.uktv-preset", "Only index channels with 'UK' in the name. $(TELLY_FILTER_UKTV_PRESET)").Envar("TELLY_FILTER_UKTV_PRESET").Default("false").BoolVar(&opts.FilterUKTV)
	kingpin.Flag("filter.useregex", "Use regex to filter for channels that you want. Basic example would be .*UK.*. When using this --filter.uktv-preset and --filter.filterregex will NOT work. $(TELLY_FILTER_USEREGEX)").Envar("TELLY_FILTER_USEREGEX").Default(".*").StringVar(&opts.UseRegex)

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

	if opts.BaseAddress.IP.IsUnspecified() {
		log.Panicln("Your base URL is set to 0.0.0.0, this will not work. Please use the --web.base-address option and set it to the (local) IP address telly is running on")
	}

	if opts.ListenAddress.IP.IsUnspecified() && opts.BaseAddress.IP.IsLoopback() {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	usedTracks := make([]Track, 0)

	var m3uReader io.Reader

	if opts.M3UPath == "iptv.m3u" {
		log.Warnln("using default m3u option, 'iptv.m3u'. launch telly with the --iptv.playlist=yourfile.m3u option to change this!")
	}

	if strings.HasPrefix(strings.ToLower(opts.M3UPath), "http") {
		log.Debugf("Downloading M3U from %s", opts.M3UPath)
		resp, err := http.Get(opts.M3UPath)
		if err != nil {
			log.WithError(err).Panicln("could not download M3U")
		}
		defer resp.Body.Close()

		m3uReader = resp.Body
	} else {
		log.Debugf("Reading M3U file %s...", opts.M3UPath)

		m3uFile, m3uReadErr := os.Open(opts.M3UPath)
		if m3uReadErr != nil {
			log.WithError(m3uReadErr).Panicln("unable to open local M3U file")
		}
		m3uReader = m3uFile
	}

	playlist, err := m3u.Decode(m3uReader)
	if err != nil {
		log.WithError(err).Panicln("unable to parse m3u file")
	}

	episodeRegex, _ := regexp.Compile("S\\d{1,3}E\\d{1,3}")
	twentyFourSevenRegex, _ := regexp.Compile("24/7")
	ukTv, _ := regexp.Compile("UK")

	userRegex, _ := regexp.Compile(opts.UseRegex)

	for _, oldTrack := range playlist.Tracks {
		track := Track{Track: oldTrack}
		if unmarshalErr := oldTrack.UnmarshalTags(&track); unmarshalErr != nil {
			log.WithError(unmarshalErr).Panicln("Error when unmarshalling tags to Track")
		}
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

	log.Debugln("Building lineup")
	lineupItems := buildLineup(opts.DirectMode, opts.BaseAddress.String(), usedTracks)

	if !opts.FilterRegex {
		log.Warnln("telly is not attempting to strip out unneeded channels, please use the flag --filter.filterregex if telly returns too many channels")
	}

	if !opts.FilterUKTV {
		log.Warnln("telly is currently not filtering for only uk television. if you would like it to, please use the flag --filter.uktv-preset")
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
	deviceXML := discoveryData.UPNP()

	log.Debugln("creating webserver routes")

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	if opts.LogRequests {
		router.Use(ginrus())
	}

	router.GET("/", func(c *gin.Context) {
		c.XML(http.StatusOK, deviceXML)
	})

	router.GET("/discover.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, discoveryData)
	})

	router.GET("/lineup_status.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, lineupStatus)
	})

	router.GET("/lineup.post", func(c *gin.Context) {
		// empty
	})

	router.GET("/device.xml", func(c *gin.Context) {
		c.XML(http.StatusOK, deviceXML)
	})

	router.GET("/lineup.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, lineupItems)
	})

	router.GET("/stream/", func(c *gin.Context) {
		u, _ := url.Parse(c.Request.RequestURI)
		uriPart := strings.Replace(u.Path, "/stream/", "", 1)
		log.Debugf("Parsing URI %s to %s", c.Request.RequestURI, uriPart)

		decodedStreamURI, decodeErr := base64.StdEncoding.DecodeString(uriPart)
		if decodeErr != nil {
			log.WithError(err).Errorf("Invalid base64: %s", uriPart)
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		log.Debugln("Redirecting to:", string(decodedStreamURI))
		c.Redirect(http.StatusMovedPermanently, string(decodedStreamURI))
	})

	if opts.SSDP {
		log.Debugln("advertising telly service on network")
		_, ssdpErr := advertiseSSDP(opts.BaseAddress.String(), opts.FriendlyName, opts.DeviceUUID)
		if ssdpErr != nil {
			log.WithError(ssdpErr).Warnln("telly cannot advertise over ssdp")
		}
	}

	log.Infof("Listening and serving HTTP on %s", opts.ListenAddress)
	if err := router.Run(opts.ListenAddress.String()); err != nil {
		log.WithError(err).Panicln("Error starting up web server")
	}
}

func ginrus() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		end = end.UTC()

		logFields := logrus.Fields{
			"status":    c.Writer.Status(),
			"method":    c.Request.Method,
			"path":      path,
			"ipAddress": c.ClientIP(),
			"latency":   latency,
			"userAgent": c.Request.UserAgent(),
			"time":      end.Format(time.RFC3339),
		}

		entry := log.WithFields(logFields)

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			entry.Error(c.Errors.String())
		} else {
			entry.Info()
		}
	}
}
