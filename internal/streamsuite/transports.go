package streamsuite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/prometheus/common/version"
)

type StreamTransport interface {
	Type() string
	Headers() http.Header
	Start(streamURL string) (io.ReadCloser, error)
	Stop() error
}

type FFMPEG struct {
	run *exec.Cmd
}

func (f FFMPEG) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Type())
}

func (f FFMPEG) Type() string {
	return "FFMPEG"
}

func (f FFMPEG) Headers() http.Header {
	return nil
}

func (f FFMPEG) Start(streamURL string) (io.ReadCloser, error) {
	log.Infoln("Transcoding stream with ffmpeg")
	f.run = exec.Command("ffmpeg", "-re", "-i", streamURL, "-codec", "copy", "-f", "mpegts", "-tune", "zerolatency", "pipe:1")
	streamData, stdErr := f.run.StdoutPipe()
	if stdErr != nil {
		return nil, stdErr
	}

	if startErr := f.run.Start(); startErr != nil {
		return nil, startErr
	}

	return streamData, nil
}

func (f FFMPEG) Stop() error {
	return f.run.Process.Kill()
}

type HTTP struct {
	req  *http.Request
	resp *http.Response
}

func (h HTTP) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Type())
}

func (h HTTP) Type() string {
	return "HTTP"
}

func (h HTTP) Headers() http.Header {
	if h.resp == nil {
		return nil
	}
	return h.resp.Header
}

func (h HTTP) Start(streamURL string) (io.ReadCloser, error) {
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

func (h HTTP) Stop() error {
	return nil
}
