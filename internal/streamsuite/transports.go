package streamsuite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/prometheus/common/version"
)

// StreamTransport is a method to acquire a video source.
type StreamTransport interface {
	Type() string
	Headers() http.Header
	Start(streamURL string) (io.ReadCloser, error)
	Stop() error
}

// FFMPEG is a transport that uses FFMPEG to process the video stream.
type FFMPEG struct {
	run *exec.Cmd
}

// MarshalJSON returns the string type of transport.
func (f FFMPEG) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Type())
}

// Type describes the type of transport.
func (f FFMPEG) Type() string {
	return "FFMPEG"
}

// Headers returns HTTP headers to add to the outbound request, if any.
func (f FFMPEG) Headers() http.Header {
	return nil
}

// Start will begin the stream.
func (f FFMPEG) Start(streamURL string) (io.ReadCloser, error) {
	log.Infoln("Transcoding stream with ffmpeg")
	f.run = exec.Command("ffmpeg", "-re", "-i", streamURL, "-codec", "copy", "-f", "mpegts", "-tune", "zerolatency", "pipe:1") // nolint
	streamData, stdErr := f.run.StdoutPipe()
	if stdErr != nil {
		return nil, stdErr
	}

	if startErr := f.run.Start(); startErr != nil {
		return nil, startErr
	}

	return streamData, nil
}

// Stop kills the stream
func (f FFMPEG) Stop() error {
	return f.run.Process.Kill()
}

// HTTP is a transport that simply "restreams" the video from the source with a small buffer.
type HTTP struct {
	req  *http.Request
	resp *http.Response
}

// MarshalJSON returns the string type of transport.
func (h HTTP) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Type())
}

// Type describes the type of transport.
func (h HTTP) Type() string {
	return "HTTP"
}

// Headers returns HTTP headers to add to the outbound request, if any.
func (h HTTP) Headers() http.Header {
	if h.resp == nil {
		return nil
	}
	return h.resp.Header
}

// Start will begin the stream.
func (h *HTTP) Start(streamURL string) (io.ReadCloser, error) {
	streamReq, reqErr := http.NewRequest("GET", streamURL, nil)
	if reqErr != nil {
		return nil, reqErr
	}

	streamReq.Header.Set("User-Agent", fmt.Sprintf("telly/%s", version.Version))

	h.req = streamReq

	resp, respErr := http.DefaultClient.Do(streamReq)
	if respErr != nil {
		return nil, respErr
	}

	h.resp = resp

	if resp.StatusCode > 399 {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Stop kills the stream
func (h HTTP) Stop() error {
	return nil
}
