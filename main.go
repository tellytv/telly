package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jamesnetherton/m3u"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"errors"
	"os"
	"io"
)

var deviceXml string
var filterRegex *bool
var filterUkTv *bool
var m3uFile *string
var listenAddress *string
var logRequests *bool
var concurrentStreams *int
var useRegex *string

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
	m3uFile = flag.String("playlist", "iptv.m3u", "Location of playlist m3u file")
	logRequests = flag.Bool("logrequests", false, "Log any requests to telly")
	concurrentStreams = flag.Int("streams", 1, "Amount of concurrent streams allowed")
	useRegex = flag.String("useregex", ".*", "Use regex to filter for channels that you want. Basic example would be .*UK.*. When using this -uktv and -filterregex will NOT work")
	flag.Parse()
}

func log(level string, msg string) {
	fmt.Println("[telly] [" + level + "] " + msg)
}

func logRequest(r string) {
	if *logRequests {
		log("request", r)
	}
}

func downloadFile(url string, dest string) (error) {
	out, err := os.Create(dest)
	defer out.Close()
	if err != nil {
		return errors.New("Could not create file: " + dest + " ; " + err.Error())
	}

	log("info", "Downloading file " + url)
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return errors.New("Could not download file: " + err.Error())
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.New("Could not copy file: " + err.Error())
	}

	return nil
}

func buildChannels(usedTracks []m3u.Track, filterRegex *regexp.Regexp) ([]LineupItem) {
	lineup := make([]LineupItem, 0)
	gn := 10000

	for _, track := range usedTracks {

		parsedTrack := filterRegex.FindStringSubmatch(track.Name)
		var finalName string
		if len(parsedTrack) == 0 {
			// TODO: Find other ways of parsing it
			finalName = track.Name
		} else {
			finalName = parsedTrack[0]
			finalName = strings.Replace(finalName, "tvg-name=\"", "", -1)
			finalName = strings.Replace(finalName, "\" tvg", "", -1)
		}
		lu := LineupItem{
			GuideNumber: strconv.Itoa(gn),
			GuideName:   finalName,
			URL:         track.URI,
		}

		lineup = append(lineup, lu)

		gn = gn + 1
	}

	return lineup
}

func main() {
	usedTracks := make([]m3u.Track, 0)

	if *m3uFile == "iptv.m3u" {
		log("warning", "using default m3u option, 'iptv.m3u'. launch telly with the -playlist=yourfile.m3u option to change this!")
	} else {
		if strings.HasPrefix(strings.ToLower(*m3uFile), "http") {
			tempFilename := os.TempDir() + "/" + "telly.m3u"

			err := downloadFile(*m3uFile, tempFilename)
			if err != nil {
				log("error", err.Error())
				os.Exit(1)
			}

			*m3uFile = tempFilename
			defer os.Remove(tempFilename)
		}
	}

	log("info", "Reading m3u file " + *m3uFile + "...")
	playlist, err := m3u.Parse(*m3uFile)
	if err != nil {
		log("error", "unable to read m3u file, error below")
		log("error", "m3u files need to have specific formats, see the github page for more information")
		log("error", "future versions of telly will attempt to parse this better")
		panic(err)
	}

	episodeRegex, _ := regexp.Compile("S\\d{1,3}E\\d{1,3}")
	twentyFourSevenRegex, _ := regexp.Compile("24/7")
	ukTv, _ := regexp.Compile("UK")

	showNameRegex, _ := regexp.Compile("tvg-name=\"(.*)\" tvg")

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

	log("info", "found" + strconv.Itoa(len(usedTracks)) + "channels")

	log("info", "creating discovery data")
	discoveryData := DiscoveryData{
		FriendlyName:    "telly",
		Manufacturer:    "Silicondust",
		ModelNumber:     "HDHR-2US",
		FirmwareName:    "hdhomeruntc_atsc",
		TunerCount:      *concurrentStreams,
		FirmwareVersion: "20150826",
		DeviceID:        "12345678",
		DeviceAuth:      "telly123",
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

	http.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/discover.json")
		json.NewEncoder(w).Encode(discoveryData)
	})

	http.HandleFunc("/lineup_status.json", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/lineup_status.json")
		json.NewEncoder(w).Encode(lineupStatus)
	})

	http.HandleFunc("/lineup.post", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/lineup.post")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/")
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(deviceXml))
	})

	http.HandleFunc("/device.xml", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/device.xml")
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(deviceXml))
	})

	log("info", "Building lineup")
	lineupItems := buildChannels(usedTracks, showNameRegex)

	http.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		logRequest("/lineup.json")
		json.NewEncoder(w).Encode(lineupItems)

	})

	log("info", "listening on " + *listenAddress)
	http.ListenAndServe(*listenAddress, nil)
}
