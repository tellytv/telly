//Package metrics provides Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

var (
	// ExposedChannels tracks the total number of exposed channels
	ExposedChannels = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "telly",
			Subsystem: "tuner",
			Name:      "channels_total",
			Help:      "Number of exposed channels.",
		},
		[]string{"lineup_name"},
	)
	// ActiveStreams tracks the realtime number of active streams.
	ActiveStreams = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "telly",
			Subsystem: "tuner",
			Name:      "active_total",
			Help:      "Number of active streams. Only activated if ffmpeg is enabled.",
		},
		[]string{"lineup_name"},
	)
	// StreamTime reports the total amount of time streamed since startup.
	StreamTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "telly",
			Subsystem: "tuner",
			Name:      "stream_time",
			Help:      "Amount of stream time in seconds.",
		},
		[]string{"lineup_name", "channel_name", "channel_number"},
	)
)

func init() {
	version.NewCollector("telly")
	prometheus.MustRegister(ExposedChannels)
	prometheus.MustRegister(ActiveStreams)
	prometheus.MustRegister(StreamTime)
}
