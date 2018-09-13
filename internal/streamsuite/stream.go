package streamsuite

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tellytv/telly/internal/metrics"
	"github.com/tellytv/telly/internal/models"
)

var (
	log = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}
)

const (
	// BufferSize is the size of the content buffer we will use.
	BufferSize = 1024 * 8
)

// Stream describes a single active video stream in telly.
type Stream struct {
	UUID      string
	Channel   *models.LineupChannel
	StreamURL string

	Transport   StreamTransport
	StartTime   *time.Time
	PromLabels  []string
	StopNow     chan bool `json:"-"`
	LastWroteAt *time.Time

	streamData io.ReadCloser
}

// Start will mark the stream as playing and begin playback.
func (s *Stream) Start(c *gin.Context) {
	ctx := c.Request.Context()

	now := time.Now()
	s.StartTime = &now
	metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Inc()

	log.Infoln("Transcoding stream via", s.Transport.Type())
	sd, streamErr := s.Transport.Start(ctx, s.StreamURL)
	if streamErr != nil {
		if httpErr, ok := streamErr.(httpError); ok {
			c.AbortWithError(httpErr.StatusCode, httpErr)
			return
		}
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("error when starting streaming via %s: %s", s.Transport.Type(), streamErr))
		return
	}

	s.streamData = sd

	clientGone := c.Writer.CloseNotify()

	for key, value := range s.Transport.Headers() {
		c.Writer.Header()[key] = value
	}

	buffer := make([]byte, BufferSize)

	writer := wrappedWriter{c.Writer}

forLoop:
	for {
		select {
		case <-s.StopNow:
			break forLoop
		case <-clientGone:
		case <-ctx.Done():
			log.Debugln("Stream client is disconnected, returning!")
			break forLoop
		default:
			n, err := s.streamData.Read(buffer)

			if n == 0 {
				log.Debugln("Read 0 bytes from stream source, returning")
				break forLoop
			}

			if err != nil {
				log.WithError(err).Errorln("Received error while reading from stream source")
				break forLoop
			}

			data := buffer[:n]
			if _, respWriteErr := writer.Write(data); respWriteErr != nil {
				if respWriteErr == io.EOF || respWriteErr == io.ErrUnexpectedEOF || respWriteErr == io.ErrClosedPipe {
					log.Debugln("CAUGHT IO ERR")
				}
				log.WithError(respWriteErr).Errorln("Error while writing to connected stream client")
				break forLoop
			}
			c.Writer.Flush()
		}
	}

	s.Stop()

}

// Stop will tear down the stream.
func (s *Stream) Stop() {
	metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Dec()

	if closeErr := s.streamData.Close(); closeErr != nil {
		log.WithError(closeErr).Errorf("error when closing stream via %s", s.Transport.Type())
		return
	}

	if stopErr := s.Transport.Stop(); stopErr != nil {
		log.WithError(stopErr).Errorf("error when cleaning up stream via %s", s.Transport.Type())
		return
	}
}

type wrappedWriter struct {
	writer io.Writer
}

func (w wrappedWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		// Filter out broken pipe (user pressed "stop") errors
		if nErr, ok := err.(*net.OpError); ok {
			if nErr.Err == syscall.EPIPE {
				return n, nil
			}
		}
	}
	return n, err
}
