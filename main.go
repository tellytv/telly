package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/namsral/flag"
	"github.com/tombowditch/telly-m3u-parser"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"github.com/AlekSi/xmltv"
	"encoding/xml"
	"io/ioutil"
)

var deviceXml string
var filterRegex *bool
var filterUkTv *bool

//TODO: remove m3uFileOld in next release (deprecated)
var m3uFileOld *string
var m3uPath *string
var listenAddress *string
var logRequests *bool
var concurrentStreams *int
var useRegex *string
var deviceId *string
var deviceAuth *string
var friendlyName *string
var tempPath *string
var xmlTvFile *string

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
	filterRegex = flag.Bool("filterregex", false, "Use regex to attempt to strip out bogus channels (SxxExx, 24/7 channels, etc")
	filterUkTv = flag.Bool("uktv", false, "Only index channels with 'UK' in the name")
	listenAddress = flag.String("listen", "localhost:6077", "IP:Port to listen on")
	//TODO: remove m3uFileOld in next release (deprecated)
	m3uFileOld = flag.String("file", "", "Filepath of the playlist m3u file (DEPRECATED, use -playlist instead)")
	m3uPath = flag.String("playlist", "iptv.m3u", "Location of playlist m3u file")
	logRequests = flag.Bool("logrequests", false, "Log any requests to telly")
	concurrentStreams = flag.Int("streams", 1, "Amount of concurrent streams allowed")
	useRegex = flag.String("useregex", ".*", "Use regex to filter for channels that you want. Basic example would be .*UK.*. When using this -uktv and -filterregex will NOT work")
	deviceId = flag.String("deviceid", "12345678", "8 characters, must be numbers. Only change this if you know what you're doing")
	deviceAuth = flag.String("deviceauth", "telly123", "Only change this if you know what you're doing")
	friendlyName = flag.String("friendlyname", "telly", "Useful if you are running two instances of telly and want to differentiate between them.")
	tempPath = flag.String("temp", os.TempDir()+"/telly.m3u", "Where telly will temporarily store the downloaded playlist file.")
	xmlTvFile = flag.String("xmltv", "xmltv.xml", "XMLTV file where the EPG data is stored")
	flag.Parse()
}



func log(level string, msg string) {
	fmt.Println("[telly] [" + level + "] " + msg)
}

func logRequestHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *logRequests {
			log("request", r.RemoteAddr+" -> "+r.Method+" "+r.RequestURI)

			if r.Method == "POST" {
				r.ParseForm()
				log("request", "POST body: "+r.Form.Encode())
			}
		}

		next.ServeHTTP(w, r)

	})
}

/*
	Returns a map with "Channel Name" => "Channel Id" mapping
 */
func getChannelMappings(reader io.Reader) (map[string]string, error) {
	decoder := xml.NewDecoder(reader)

	tvSetup := new(xmltv.Tv)

	if err := decoder.Decode(tvSetup); err != nil {
		return nil, errors.New("Could not decode xmltv programme: " + err.Error())
	}

	channelMap := make(map[string]string)

	for _, tvChann := range tvSetup.Channels {
		for _, displayName := range tvChann.DisplayNames {
			channelMap[displayName] = tvChann.Id
		}
	}

	return channelMap, nil
}

func downloadFile(url string, dest string) error {
	out, err := os.Create(dest)
	defer out.Close()
	if err != nil {
		return errors.New("Could not create file: " + dest + " ; " + err.Error())
	}

	log("info", "Downloading file "+url+" to "+dest)
	resp, err := http.Get(url)
	if err != nil {
		return errors.New("Could not download: " + err.Error())
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.New("Could not download to output file: " + err.Error())
	}

	return nil
}

func buildChannels(xmltv string, usedTracks []m3u.Track) []LineupItem {
	lineup := make([]LineupItem, 0)
	gn := -1

	chanMap, err := getChannelMappings(strings.NewReader(xmltv))
	if err != nil {
		log("info", "Disabling XMLTV channel matching because of: " + err.Error())
		gn = 10000
	}

	for _, track := range usedTracks {

		var channName string
		if track.TvgName == "" {
			channName = track.Name
		} else {
			channName = track.TvgName
		}

		var channNum string
		if gn == -1 {
			for _, chanMapName := range chanMap {
				if strings.Contains(strings.ToLower(chanMapName), strings.ToLower(channName)) {
					channNum = chanMap[chanMapName]
					break;
				}
			}

			if channNum == "" {
				log("info", "No XMLTV entry found for channel: " + channName + ", defaulting to index")
				channNum = strconv.Itoa(gn)
				gn += 1
			}
		} else {
			channNum = strconv.Itoa(gn)
			gn += 1
		}

		lu := LineupItem{
			GuideNumber: channNum,
			GuideName:   channName,
			URL:         track.URI,
		}

		lineup = append(lineup, lu)
	}

	return lineup
}

func main() {
	tellyVersion := "v0.4.5"
	log("info", "booting telly "+tellyVersion)
	usedTracks := make([]m3u.Track, 0)

	// TODO: remove m3uFileOld
	if *m3uFileOld != "" {
		log("error", "argument -file is deprecated, use -playlist instead")
		os.Exit(1)
	}

	if *m3uPath == "iptv.m3u" {
		log("warning", "using default m3u option, 'iptv.m3u'. launch telly with the -playlist=yourfile.m3u option to change this!")
	} else {
		if strings.HasPrefix(strings.ToLower(*m3uPath), "http") {
			tempFilename := *tempPath

			err := downloadFile(*m3uPath, tempFilename)
			if err != nil {
				log("error", err.Error())
				os.Exit(1)
			}

			*m3uPath = tempFilename
			defer os.Remove(tempFilename)
		}
	}

	log("info", "Reading m3u file " + *m3uPath+"...")
	playlist, err := m3u.Parse(*m3uPath)
	if err != nil {
		log("error", "unable to read m3u file, error below")
		log("error", "m3u files need to have specific formats, see the github page for more information")
		log("error", "future versions of telly will attempt to parse this better")
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
		log("warning", "telly is not attempting to strip out unneeded channels, please use the flag -filterregex if telly returns too many channels")
	}

	if !*filterUkTv {
		log("info", "telly is currently not filtering for only uk television. if you would like it to, please use the flag -uktv")
	}

	channelCount := len(usedTracks)

	log("info", "found "+strconv.Itoa(channelCount)+" channels")

	if channelCount > 420 {
		fmt.Println("")
		fmt.Println("* * * * * * * * * * *")
		log("warning", "telly has loaded more than 420 channels. Plex does not deal well with more than this amount and will more than likely hang when trying to fetch channels. You have been warned!")
		fmt.Println("* * * * * * * * * * *")
		fmt.Println("")
	}

	log("info", "creating discovery data")
	discoveryData := DiscoveryData{
		FriendlyName:    *friendlyName,
		Manufacturer:    "Silicondust",
		ModelNumber:     "HDHR-2US",
		FirmwareName:    "hdhomeruntc_atsc",
		TunerCount:      *concurrentStreams,
		FirmwareVersion: "20150826",
		DeviceID:        *deviceId,
		DeviceAuth:      *deviceAuth,
		BaseURL:         fmt.Sprintf("http://%s", *listenAddress),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", *listenAddress),
	}

	log("info", "creating lineup status")
	lineupStatus := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Cable"},
	}

	log("info", "creating device xml")
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

	log("info", "creating webserver routes")

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

	var xmlEpg string
	if *xmlTvFile != "" {
		log("info", "Trying to load xmltv")
		xmlEpgBytes, err := ioutil.ReadFile(*xmlTvFile)
		if err != nil {
			log("error", "Skipping XMLTV: " + err.Error())
		}
		xmlEpg = string(xmlEpgBytes)
	}

	log("info", "Building lineup")
	lineupItems := buildChannels(xmlEpg, usedTracks)

	h.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(lineupItems)

	})

	log("info", "listening on "+*listenAddress)
	if err := http.ListenAndServe(*listenAddress, logRequestHandler(h)); err != nil {
		log("error", err.Error())
		os.Exit(1)
	}
}
