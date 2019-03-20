// Package m3uplus provides a M3U Plus parser.
package m3uplus

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// Playlist is a type that represents an m3u playlist containing 0 or more tracks
type Playlist struct {
	Tracks []Track
}

// Track represents an m3u track
type Track struct {
	Name       string
	Length     float64
	URI        string
	Tags       map[string]string
	Raw        string
	LineNumber int
}

// UnmarshalTags will decode the Tags map into a struct containing fields with `m3u` tags matching map keys.
func (t *Track) UnmarshalTags(v interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "m3u",
		Result:  &v,
	})
	if err != nil {
		return err
	}

	return decoder.Decode(t.Tags)
}

// Decode parses an m3u playlist in the given io.Reader and returns a Playlist
func Decode(r io.Reader) (*Playlist, error) {
	playlist := &Playlist{}
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	if decErr := decode(playlist, buf); decErr != nil {
		return nil, decErr
	}

	return playlist, nil
}

func decode(playlist *Playlist, buf *bytes.Buffer) error {
	var eof bool
	var line string
	var err error

	lineNum := 0

	for !eof {
		lineNum = lineNum + 1
		if line, err = buf.ReadString('\n'); err == io.EOF {
			eof = true
		} else if err != nil {
			return err
		}

		if lineNum == 1 && !strings.HasPrefix(strings.TrimSpace(line), "#EXTM3U") {
			return fmt.Errorf("malformed M3U provided")
		}

		if err = decodeLine(playlist, line, lineNum); err != nil {
			return err
		}
	}
	return nil
}

func decodeLine(playlist *Playlist, line string, lineNumber int) error {
	line = strings.TrimSpace(line)

	switch {
	case strings.HasPrefix(line, "#EXTINF:"):
		track := Track{
			Raw:        line,
			LineNumber: lineNumber,
		}

		track.Length, track.Name, track.Tags = decodeInfoLine(line)

		playlist.Tracks = append(playlist.Tracks, track)

	case strings.HasPrefix(line, "http") || strings.HasPrefix(line, "udp"):
		playlist.Tracks[len(playlist.Tracks)-1].URI = line
	}

	return nil
}

var infoRegex = regexp.MustCompile(`([^\s="]+)=(?:"(.*?)"|(\d+))(?:,([.*^,]))?|#EXTINF:(-?\d*\s*)|,(.*)`)

func decodeInfoLine(line string) (float64, string, map[string]string) {
	matches := infoRegex.FindAllStringSubmatch(line, -1)
	var err error
	durationFloat := 0.0
	durationStr := strings.TrimSpace(matches[0][len(matches[0])-2])
	if durationStr != "-1" && len(durationStr) > 0 {
		if durationFloat, err = strconv.ParseFloat(durationStr, 64); err != nil {
			panic(fmt.Errorf("Duration parsing error: %s", err))
		}
	}

	titleIndex := len(matches) - 1
	title := matches[titleIndex][len(matches[titleIndex])-1]

	keyMap := make(map[string]string)

	for _, match := range matches[1 : len(matches)-1] {
		val := match[2]
		if val == "" { // If empty string find a number in [3]
			val = match[3]
		}
		keyMap[strings.ToLower(match[1])] = val
	}

	return durationFloat, title, keyMap
}
