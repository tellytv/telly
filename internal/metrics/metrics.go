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
			Subsystem: "channels",
			Name:      "total",
			Help:      "Number of exposed channels.",
		},
		[]string{"lineup_name", "video_source_name", "video_source_provider"},
	)
	// ActiveStreams tracks the realtime number of active streams.
	ActiveStreams = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "telly",
			Subsystem: "channels",
			Name:      "active",
			Help:      "Number of active streams.",
		},
		[]string{"lineup_name", "video_source_name", "video_source_provider", "channel_name", "stream_transport"},
	)
	// PausedStreams tracks the realtime number of paused streams.
	PausedStreams = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "telly",
			Subsystem: "channels",
			Name:      "paused",
			Help:      "Number of paused streams.",
		},
		[]string{"lineup_name", "video_source_name", "video_source_provider", "channel_name", "stream_transport"},
	)
	// StreamPlayingTime reports the total amount of time streamed since startup.
	StreamPlayingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "telly",
			Subsystem: "channels",
			Name:      "playing_time",
			Help:      "Amount of stream playing time in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.1, 1.5, 5),
		},
		[]string{"lineup_name", "video_source_name", "video_source_provider", "channel_name", "stream_transport"},
	)
	// StreamPausedTime reports the total amount of time streamed since startup.
	StreamPausedTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "telly",
			Subsystem: "channels",
			Name:      "paused_time",
			Help:      "Amount of stream paused time in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.1, 1.5, 5),
		},
		[]string{"lineup_name", "video_source_name", "video_source_provider", "channel_name", "stream_transport"},
	)
)

// nolint
func init() {
	prometheus.MustRegister(version.NewCollector("telly"))
	prometheus.MustRegister(ExposedChannels)
	prometheus.MustRegister(ActiveStreams)
	prometheus.MustRegister(PausedStreams)
	prometheus.MustRegister(StreamPlayingTime)
	prometheus.MustRegister(StreamPausedTime)
}
