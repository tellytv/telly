package utils

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"github.com/tellytv/telly/internal/m3uplus"
	"github.com/tellytv/telly/internal/xmltv"
)

var (
	// SafeStringsRegex will match any usernames, passwords or tokens in a string.
	SafeStringsRegex = regexp.MustCompile(`(?m)(username|password|token)=[\w=]+(&?)`)

	// StringSafer will replace sensitive values (username, password and token) with safed values.
	StringSafer = func(input string) string {
		ret := input
		if strings.HasPrefix(input, "username=") {
			ret = "username=REDACTED"
		} else if strings.HasPrefix(input, "password=") {
			ret = "password=REDACTED"
		} else if strings.HasPrefix(input, "token=") {
			ret = "token=bm90Zm9yeW91" // "notforyou"
		}
		if strings.HasSuffix(input, "&") {
			return fmt.Sprintf("%s&", ret)
		}
		return ret
	}
)

// GetTCPAddr attempts to convert a string found via viper to a net.TCPAddr. Will panic on error.
func GetTCPAddr(key string) *net.TCPAddr {
	addr, addrErr := net.ResolveTCPAddr("tcp", viper.GetString(key))
	if addrErr != nil {
		panic(fmt.Errorf("error parsing address %s: %s", viper.GetString(key), addrErr))
	}
	return addr
}

// GetM3U is a helper function to download/open and parse a M3U Plus file.
func GetM3U(path string, cacheFiles bool) (*m3uplus.Playlist, error) {
	// safePath := SafeStringsRegex.ReplaceAllStringFunc(path, StringSafer)

	file, _, err := GetFile(path, cacheFiles)
	if err != nil {
		return nil, fmt.Errorf("error while opening m3u file: %s", err)
	}

	rawPlaylist, decodeErr := m3uplus.Decode(file)
	if decodeErr != nil {
		return nil, fmt.Errorf("error while decoding m3u file: %s", decodeErr)
	}

	if closeM3UErr := file.Close(); closeM3UErr != nil {
		return nil, fmt.Errorf("error when closing m3u reader: %s", closeM3UErr)
	}

	return rawPlaylist, nil
}

// GetXMLTV is a helper function to download/open and parse a XMLTV file.
func GetXMLTV(path string, cacheFiles bool) (*xmltv.TV, error) {
	// safePath := SafeStringsRegex.ReplaceAllStringFunc(path, StringSafer)

	file, _, err := GetFile(path, cacheFiles)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(file)
	tvSetup := new(xmltv.TV)
	if err := decoder.Decode(tvSetup); err != nil {
		return nil, fmt.Errorf("could not decode xmltv programme: %s", err)
	}

	if closeXMLErr := file.Close(); closeXMLErr != nil {
		return nil, fmt.Errorf("error when closing xml reader: %s", closeXMLErr)
	}

	return tvSetup, nil
}

// GetFile is a helper function to download/open and parse a file.
func GetFile(path string, cacheFiles bool) (io.ReadCloser, string, error) {
	transport := "disk"

	if strings.HasPrefix(strings.ToLower(path), "http") {

		transport = "http"

		req, reqErr := http.NewRequest("GET", path, nil)
		if reqErr != nil {
			return nil, transport, reqErr
		}

		// For whatever reason, some providers only allow access from a "real" User-Agent.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/68.0.3440.106 Safari/537.36")

		resp, err := http.Get(path)
		if err != nil {
			return nil, transport, err
		}

		if strings.HasSuffix(strings.ToLower(path), ".gz") || resp.Header.Get("Content-Type") == "application/x-gzip" {
			// log.Infof("File (%s) is gzipp'ed, ungzipping now, this might take a while", path)
			gz, gzErr := gzip.NewReader(resp.Body)
			if gzErr != nil {
				return nil, transport, gzErr
			}

			if cacheFiles {
				return writeFile(path, transport, gz)
			}

			return gz, transport, nil
		}

		if cacheFiles {
			return writeFile(path, transport, resp.Body)
		}

		return resp.Body, transport, nil
	}

	file, fileErr := os.Open(path)
	if fileErr != nil {
		return nil, transport, fileErr
	}

	return file, transport, nil
}

// ChunkStringSlice will return a slice of slice of strings for the given chunkSize.
func ChunkStringSlice(sl []string, chunkSize int) [][]string {
	var divided [][]string

	for i := 0; i < len(sl); i += chunkSize {
		end := i + chunkSize

		if end > len(sl) {
			end = len(sl)
		}

		divided = append(divided, sl[i:end])
	}
	return divided
}

func writeFile(path, transport string, reader io.ReadCloser) (io.ReadCloser, string, error) {
	// buf := new(bytes.Buffer)
	// buf.ReadFrom(reader)
	// buf.Bytes()
	return reader, transport, nil
}

// Contains returns true if the given element "e" is found inside the slice of strings "s".
func Contains(s []string, e string) bool {
	for _, ss := range s {
		if e == ss {
			return true
		}
	}
	return false
}

// GetStringMapKeys returns a slice of strings for the keys of a map.
func GetStringMapKeys(s map[string]struct{}) []string {
	keys := make([]string, 0)
	for key := range s {
		keys = append(keys, key)
	}
	return keys
}

// From https://github.com/stoewer/go-strcase

// KebabCase converts a string into kebab case.
func KebabCase(s string) string {
	return lowerDelimiterCase(s, '-')
}

// SnakeCase converts a string into snake case.
func SnakeCase(s string) string {
	return lowerDelimiterCase(s, '_')
}

// isLower checks if a character is lower case. More precisely it evaluates if it is
// in the range of ASCII character 'a' to 'z'.
func isLower(ch rune) bool {
	return ch >= 'a' && ch <= 'z'
}

// toLower converts a character in the range of ASCII characters 'A' to 'Z' to its lower
// case counterpart. Other characters remain the same.
func toLower(ch rune) rune {
	if ch >= 'A' && ch <= 'Z' {
		return ch + 32
	}
	return ch
}

// isLower checks if a character is upper case. More precisely it evaluates if it is
// in the range of ASCII characters 'A' to 'Z'.
func isUpper(ch rune) bool {
	return ch >= 'A' && ch <= 'Z'
}

// toLower converts a character in the range of ASCII characters 'a' to 'z' to its lower
// case counterpart. Other characters remain the same.
func toUpper(ch rune) rune {
	if ch >= 'a' && ch <= 'z' {
		return ch - 32
	}
	return ch
}

// isSpace checks if a character is some kind of whitespace.
func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// isDelimiter checks if a character is some kind of whitespace or '_' or '-'.
func isDelimiter(ch rune) bool {
	return ch == '-' || ch == '_' || isSpace(ch)
}

// lowerDelimiterCase converts a string into snake_case or kebab-case depending on
// the delimiter passed in as second argument.
func lowerDelimiterCase(s string, delimiter rune) string {
	s = strings.TrimSpace(s)
	buffer := make([]rune, 0, len(s)+3)

	var prev rune
	var curr rune
	for _, next := range s {
		if isDelimiter(curr) {
			if !isDelimiter(prev) {
				buffer = append(buffer, delimiter)
			}
		} else if isUpper(curr) {
			if isLower(prev) || (isUpper(prev) && isLower(next)) {
				buffer = append(buffer, delimiter)
			}
			buffer = append(buffer, toLower(curr))
		} else if curr != 0 {
			buffer = append(buffer, curr)
		}
		prev = curr
		curr = next
	}

	if len(s) > 0 {
		if isUpper(curr) && isLower(prev) && prev != 0 {
			buffer = append(buffer, delimiter)
		}
		buffer = append(buffer, toLower(curr))
	}

	return string(buffer)
}

// PadNumberWithZeros will pad the given value integer with 0's until expectedLength is met.
func PadNumberWithZeros(value int, expectedLength int) string {
	padded := fmt.Sprintf("%02d", value)
	valLength := CountDigits(value)
	if valLength != expectedLength {
		repeatLength := expectedLength - valLength
		if repeatLength < 0 {
			repeatLength = 0
		}
		return fmt.Sprintf("%s%d", strings.Repeat("0", repeatLength), value)
	}
	return padded
}

// CountDigits will count the number of digits in an integer.
func CountDigits(i int) int {
	count := 0
	if i == 0 {
		count = 1
	}
	for i != 0 {
		i /= 10
		count = count + 1
	}
	return count
}
