package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	ssdp "github.com/koron/go-ssdp"
	"github.com/namsral/flag"
	log "github.com/sirupsen/logrus"
	m3u "github.com/tombowditch/telly-m3u-parser"
)

var deviceXml string
var filterRegex *bool
var filterUkTv *bool
var directMode *bool

var m3uPath *string
var listenAddress *string
var baseURL *string
var logRequests *bool
var concurrentStreams *int
var useRegex *string
var deviceId string
var deviceAuth *string
var friendlyName *string
var tempPath *string
var deviceUuid string
var noSsdp *bool

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

type LineupStatus struct {
	ScanInProgress int
	ScanPossible   int
	Source         string
	SourceList     []string
}

type LineupItem struct {
	GuideNumber string
	GuideName   string
	URL         string
}

func init() {
	flag.StringVar(&deviceId, "deviceid", "12345678", "8 characters, must be numbers. Only change this if you know what you're doing")
	deviceUuid = fmt.Sprintf("%s-AE2A-4E54-BBC9-33AF7D5D6A92", deviceId)
	filterRegex = flag.Bool("filterregex", false, "Use regex to attempt to strip out bogus channels (SxxExx, 24/7 channels, etc")
	filterUkTv = flag.Bool("uktv", false, "Only index channels with 'UK' in the name")
	listenAddress = flag.String("listen", "localhost:6077", "IP:Port to listen on")
	baseURL = flag.String("base", "localhost:6077", "example.com:port (useful with reverse proxy)")
	m3uPath = flag.String("playlist", "iptv.m3u", "Location of playlist m3u file")
	logRequests = flag.Bool("logrequests", false, "Log any requests to telly")
	concurrentStreams = flag.Int("streams", 1, "Amount of concurrent streams allowed")
	useRegex = flag.String("useregex", ".*", "Use regex to filter for channels that you want. Basic example would be .*UK.*. When using this -uktv and -filterregex will NOT work")
	deviceAuth = flag.String("deviceauth", "telly123", "Only change this if you know what you're doing")
	friendlyName = flag.String("friendlyname", "telly", "Useful if you are running two instances of telly and want to differentiate between them.")
	tempPath = flag.String("temp", fmt.Sprintf("%s/telly.m3u", os.TempDir()), "Where telly will temporarily store the downloaded playlist file.")
	directMode = flag.Bool("direct", false, "Does not encode the stream URL and redirect to the correct one.")
	noSsdp = flag.Bool("nossdp", false, "Turn off SSDP")
	flag.Parse()
}

func logRequestHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *logRequests {
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

func buildChannels(usedTracks []m3u.Track) []LineupItem {
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
		fullTrackUri := track.URI
		if !*directMode {
			trackUri := base64.StdEncoding.EncodeToString([]byte(track.URI))
			fullTrackUri = fmt.Sprintf("http://%s/stream/%s", *baseURL, trackUri)
		}

		if strings.Contains(track.URI, ".m3u8") {
			log.Warnln("your .m3u contains .m3u8's. Plex has either stopped supporting m3u8 or it is a bug in a recent version - please use .ts! telly will automatically convert these in a future version. See telly github issue #108")
		}

		lu := LineupItem{
			GuideNumber: strconv.Itoa(gn),
			GuideName:   finalName,
			URL:         fullTrackUri,
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

func advertiseSSDP(deviceName string, deviceUUID string) (*ssdp.Advertiser, error) {
	log.Debugf("Advertising telly as %s (%s)", deviceName, deviceUUID)

	adv, err := ssdp.Advertise(
		"upnp:rootdevice",
		fmt.Sprintf("uuid:%s::upnp:rootdevice", deviceUUID),
		fmt.Sprintf("http://%s/device.xml", *listenAddress),
		deviceName,
		1800)

	if err != nil {
		return nil, err
	}

	go sendAlive(adv)

	return adv, nil
}

func base64StreamHandler(w http.ResponseWriter, r *http.Request, base64StreamUrl string) {
	decodedStreamURI, err := base64.StdEncoding.DecodeString(base64StreamUrl)

	if err != nil {
		log.Errorf("Invalid base64: %s: %s", base64StreamUrl, err.Error())
		w.WriteHeader(400)
		return
	}

	log.Debugln("Redirecting to:", string(decodedStreamURI))
	http.Redirect(w, r, string(decodedStreamURI), 301)
}

func main() {
	tellyVersion := "v0.6.2"
	log.Debugln("booting telly", tellyVersion)
	usedTracks := make([]m3u.Track, 0)

	if *m3uPath == "iptv.m3u" {
		log.Warnln("using default m3u option, 'iptv.m3u'. launch telly with the -playlist=yourfile.m3u option to change this!")
	} else {
		if strings.HasPrefix(strings.ToLower(*m3uPath), "http") {
			tempFilename := *tempPath

			err := downloadFile(*m3uPath, tempFilename)
			if err != nil {
				log.Errorln(err.Error())
				os.Exit(1)
			}

			*m3uPath = tempFilename
			defer os.Remove(tempFilename)
		}
	}

	log.Debugf("Reading m3u file %s...", *m3uPath)
	playlist, err := m3u.Parse(*m3uPath)
	if err != nil {
		log.Errorln("unable to read m3u file, error below")
		panic(err)
	}

	episodeRegex, _ := regexp.Compile("S\\d{1,3}E\\d{1,3}")
	twentyFourSevenRegex, _ := regexp.Compile("24/7")
	ukTv, _ := regexp.Compile("UK")

	userRegex, _ := regexp.Compile(*useRegex)

	for _, track := range playlist.Tracks {
		if *useRegex == ".*" {
			if *filterRegex && *filterUkTv {
				if !episodeRegex.MatchString(track.Name) {
					if !twentyFourSevenRegex.MatchString(track.Name) {
						if ukTv.MatchString(track.Name) {
							usedTracks = append(usedTracks, track)
						}
					}
				}
			} else if *filterRegex && !*filterUkTv {
				if !episodeRegex.MatchString(track.Name) {
					if !twentyFourSevenRegex.MatchString(track.Name) {
						usedTracks = append(usedTracks, track)
					}
				}

			} else if !*filterRegex && *filterUkTv {
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

	if !*filterRegex {
		log.Warnln("telly is not attempting to strip out unneeded channels, please use the flag -filterregex if telly returns too many channels")
	}

	if !*filterUkTv {
		log.Warnln("telly is currently not filtering for only uk television. if you would like it to, please use the flag -uktv")
	}

	channelCount := len(usedTracks)

	log.Infof("found %d channels", channelCount)

	if channelCount > 420 {
		log.Warnln("telly has loaded more than 420 channels. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You have been warned!")
	}

	*friendlyName = fmt.Sprintf("HDHomerun (%s)", *friendlyName)

	log.Debugln("creating discovery data")
	discoveryData := DiscoveryData{
		FriendlyName:    *friendlyName,
		Manufacturer:    "Silicondust",
		ModelNumber:     "HDTC-2US",
		FirmwareName:    "hdhomeruntc_atsc",
		TunerCount:      *concurrentStreams,
		FirmwareVersion: "20150826",
		DeviceID:        deviceId,
		DeviceAuth:      *deviceAuth,
		BaseURL:         fmt.Sprintf("http://%s", *baseURL),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", *baseURL),
	}

	log.Debugln("creating lineup status")
	lineupStatus := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}

	log.Debugln("creating device xml")
	deviceXml = `<root xmlns="urn:schemas-upnp-org:device-1-0">
    <specVersion>
        <major>1</major>
        <minor>0</minor>
    </specVersion>
    <URLBase>$BaseURL</URLBase>
    <device>
        <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
        <friendlyName>$FriendlyName</friendlyName>
        <manufacturer>$Manufacturer</manufacturer>
        <modelName>$ModelNumber</modelName>
        <modelNumber>$ModelNumber</modelNumber>
        <serialNumber></serialNumber>
        <UDN>uuid:$DeviceID</UDN>
    </device>
</root>`

	deviceXml = strings.Replace(deviceXml, "$BaseURL", discoveryData.BaseURL, -1)
	deviceXml = strings.Replace(deviceXml, "$FriendlyName", discoveryData.FriendlyName, -1)
	deviceXml = strings.Replace(deviceXml, "$Manufacturer", discoveryData.Manufacturer, -1)
	deviceXml = strings.Replace(deviceXml, "$ModelNumber", discoveryData.ModelNumber, -1)
	deviceXml = strings.Replace(deviceXml, "$DeviceID", discoveryData.DeviceID, -1)

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
		w.Write([]byte(deviceXml))
	})

	h.HandleFunc("/device.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(deviceXml))
	})

	log.Debugln("Building lineup")
	lineupItems := buildChannels(usedTracks)

	h.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineupItems)
	})

	h.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		u, _ := url.Parse(r.RequestURI)
		uriPart := strings.Replace(u.Path, "/stream/", "", 1)
		log.Debugf("Parsing URI %s to %s", r.RequestURI, uriPart)
		base64StreamHandler(w, r, uriPart)
	})

	if strings.Contains(*baseURL, "0.0.0.0") {
		log.Errorln("Your base URL is set to 0.0.0.0, this will not work")
		log.Errorln("Please use the -base option and set it to the (local) IP address telly is running on")
	}

	if strings.Contains(*listenAddress, "0.0.0.0") && strings.Contains(*baseURL, "localhost") {
		log.Warnln("You are listening on all interfaces but your base URL is localhost (meaning Plex will try and load localhost to access your streams) - is this intended?")
	}

	if !*noSsdp {
		log.Debugln("advertising telly service on network")
		_, err2 := advertiseSSDP(*friendlyName, deviceUuid)
		if err2 != nil {
			log.WithError(err2).Warnln("telly cannot advertise over ssdp")
		}
	}

	log.Infof("listening on %s", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, logRequestHandler(h)); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}
}
