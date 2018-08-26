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
	safeStringsRegex = regexp.MustCompile(`(?m)(username|password|token)=[\w=]+(&?)`)

	stringSafer = func(input string) string {
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

func GetTCPAddr(key string) *net.TCPAddr {
	addr, addrErr := net.ResolveTCPAddr("tcp", viper.GetString(key))
	if addrErr != nil {
		panic(fmt.Errorf("error parsing address %s: %s", viper.GetString(key), addrErr))
	}
	return addr
}

func GetM3U(path string, cacheFiles bool) (*m3uplus.Playlist, error) {
	// safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)

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

func GetXMLTV(path string, cacheFiles bool) (*xmltv.TV, error) {
	// safePath := safeStringsRegex.ReplaceAllStringFunc(path, stringSafer)

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
		return nil, fmt.Errorf("error when closing xml reader", closeXMLErr)
	}

	return tvSetup, nil
}

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

func Contains(s []string, e string) bool {
	for _, ss := range s {
		if e == ss {
			return true
		}
	}
	return false
}
