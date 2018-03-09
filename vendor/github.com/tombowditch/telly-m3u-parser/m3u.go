package m3u

import (
	"bufio"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Playlist is a type that represents an m3u playlist containing 0 or more tracks
type Playlist struct {
	Tracks []Track
}

// Track represents an m3u track
type Track struct {
	Name    string
	Length  int
	URI     string
	TvgID   string
	TvgName string
}

// Parse parses an m3u playlist with the given file name and returns a Playlist
func Parse(fileName string) (playlist Playlist, err error) {
	f, err := os.Open(fileName)
	if err != nil {
		err = errors.New("Unable to open playlist file")
		return
	}
	defer f.Close()

	onFirstLine := true
	scanner := bufio.NewScanner(f)

	tvgNameRegex, _ := regexp.Compile("tvg-name=\"([^\"]+)\"")
	tvgIDRegex, _ := regexp.Compile("tvg-id=\"([^\"]+)\"")

	for scanner.Scan() {
		line := scanner.Text()
		if onFirstLine && !strings.HasPrefix(line, "#EXTM3U") {
			err = errors.New("Invalid m3u file format. Expected #EXTM3U file header")
			return
		}

		onFirstLine = false

		if strings.HasPrefix(line, "#EXTINF") {
			line := strings.Replace(line, "#EXTINF:", "", -1)
			// At this point the line will be something like "1 xxxxxxx"
			// We need "1, xxxxxx"
			temporaryInfo := strings.Split(line, " ")
			tempLength := temporaryInfo[0] // This is "1"
			if !strings.HasSuffix(tempLength, ",") {
				// We don't have a comma so we need to add it
				line = line[len(tempLength):]
				line = tempLength + ", " + line
			}
			trackInfo := strings.Split(line, ",")
			if len(trackInfo) < 2 {
				err = errors.New("Invalid m3u file format. Expected EXTINF metadata to contain track length and name data")
				return
			}
			length, parseErr := strconv.Atoi(trackInfo[0])
			if parseErr != nil {
				err = errors.New("Unable to parse length. Line: " + line)
				return
			}
			trackName := strings.Join(trackInfo[1:], " ")
			tvgName := ""
			tvgID := ""

			nameFind := tvgNameRegex.FindStringSubmatch(trackName)
			if len(nameFind) != 0 {
				tvgName = nameFind[0]
				tvgName = strings.Replace(tvgName, "tvg-name=\"", "", -1)
				tvgName = strings.Replace(tvgName, "\"", "", -1)
			}

			idFind := tvgIDRegex.FindStringSubmatch(trackName)
			if len(idFind) != 0 {
				tvgID = idFind[0]
				tvgID = strings.Replace(tvgID, "tvg-id=\"", "", -1)
				tvgID = strings.Replace(tvgID, "\"", "", -1)

			}

			track := &Track{trackName, length, "", tvgID, tvgName}
			playlist.Tracks = append(playlist.Tracks, *track)
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if len(playlist.Tracks) == 0 {
			err = errors.New("URI provided for playlist with no tracks")
			return
		} else {
			playlist.Tracks[len(playlist.Tracks)-1].URI = line
		}
	}

	return playlist, nil
}
