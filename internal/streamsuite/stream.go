package streamsuite

import (
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/tellytv/telly/metrics"
	"github.com/tellytv/telly/models"
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
	BufferSize = 1024 * 8
)

type Stream struct {
	UUID      string
	Channel   *models.LineupChannel
	StreamURL string

	Transport   StreamTransport
	Paused      bool
	PausedAt    *time.Time
	StartTime   *time.Time
	PromLabels  []string
	PlayTimer   *prometheus.Timer `json:"-"`
	PauseTimer  *prometheus.Timer `json:"-"`
	StopNow     chan bool         `json:"-"`
	LastWroteAt *time.Time
}

func (s *Stream) Start(c *gin.Context) {
	now := time.Now()
	s.LastWroteAt = &now
	s.StartTime = &now
	metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Inc()

	s.PlayTimer = prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		metrics.StreamPlayingTime.WithLabelValues(s.PromLabels...).Observe(v)
	}))

	s.PauseTimer = prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		metrics.StreamPausedTime.WithLabelValues(s.PromLabels...).Observe(v)
	}))

	streamData, streamErr := s.Transport.Start(s.StreamURL)
	if streamErr != nil {
		log.WithError(streamErr).Errorf("Error when starting streaming via %s", s.Transport.Type())
		c.AbortWithError(http.StatusInternalServerError, streamErr)
		return
	}

	defer func() {
		if closeErr := streamData.Close(); closeErr != nil {
			log.WithError(closeErr).Errorf("error when closing stream via %s", s.Transport.Type())
			return
		}

		if stopErr := s.Transport.Stop(); stopErr != nil {
			log.WithError(stopErr).Errorf("error when cleaning up stream via %s", s.Transport.Type())
			return
		}
	}()

	clientGone := c.Writer.CloseNotify()

	go func() {
		for {
			// Keep the Prometheus timer updated
			if !s.Paused {
				s.PlayTimer.ObserveDuration()
			} else {
				s.PauseTimer.ObserveDuration()
			}

			// We wait at least 2 full seconds before declaring that a stream is paused.
			if time.Now().Sub(*s.LastWroteAt) > 2*time.Second {
				s.Pause()
			}
		}
	}()

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
			log.Debugln("Stream client is disconnected, returning!")
			s.Stop()
			break forLoop
		default:
			n, err := streamData.Read(buffer)

			if n == 0 {
				log.Debugln("Read 0 bytes from stream source, returning")
				s.Unpause(false)
				break forLoop
			}

			if err != nil {
				log.WithError(err).Errorln("Received error while reading from stream source")
				s.Unpause(false)
				break forLoop
			}

			now := time.Now()
			s.LastWroteAt = &now
			s.Unpause(true)

			data := buffer[:n]
			if _, respWriteErr := writer.Write(data); respWriteErr != nil {
				if respWriteErr == io.EOF || respWriteErr == io.ErrUnexpectedEOF || respWriteErr == io.ErrClosedPipe {
					log.Infoln("CAUGHT IO ERR")
				}
				log.WithError(respWriteErr).Errorln("Error while writing to connected stream client")
				break forLoop
			}
			c.Writer.Flush()
		}
	}

}

func (s *Stream) Pause() {
	if !s.Paused {
		s.Paused = true
		now := time.Now()
		s.PausedAt = &now
		metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Dec()
		metrics.PausedStreams.WithLabelValues(s.PromLabels...).Inc()
	}
}

func (s *Stream) Unpause(increaseActiveStreams bool) {
	if s.Paused {
		s.Paused = false
		s.PausedAt = nil
		metrics.PausedStreams.WithLabelValues(s.PromLabels...).Dec()
		if increaseActiveStreams {
			metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Inc()
		}
	}
}

func (s *Stream) Stop() {
	if s.Paused {
		metrics.PausedStreams.WithLabelValues(s.PromLabels...).Dec()
	} else {
		metrics.ActiveStreams.WithLabelValues(s.PromLabels...).Dec()
	}
	s.Paused = false
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
