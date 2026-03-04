package metrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SessionsCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_sessions_created_total",
		Help: "Total number of sessions created",
	})

	SessionsEnded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_sessions_ended_total",
		Help: "Total number of sessions ended",
	})

	TranscriptionsGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_transcriptions_generated_total",
		Help: "Total number of transcriptions generated",
	})

	TranscriptionErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_transcription_errors_total",
		Help: "Total number of transcription errors",
	})

	MinutesGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_minutes_generated_total",
		Help: "Total number of minutes generated",
	})

	MinutesErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_minutes_errors_total",
		Help: "Total number of minutes generation errors",
	})

	WebhookSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_webhook_sent_total",
		Help: "Total number of webhooks sent",
	})

	WebhookErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aftertalk_webhook_errors_total",
		Help: "Total number of webhook errors",
	})

	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aftertalk_active_connections",
		Help: "Number of active WebSocket connections",
	})

	ProcessingQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aftertalk_processing_queue_size",
		Help: "Current size of processing queue",
	})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aftertalk_request_duration_seconds",
		Help:    "Duration of HTTP requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	activeConnections int64
)

func IncrementActiveConnections() {
	atomic.AddInt64(&activeConnections, 1)
	ActiveConnections.Set(float64(atomic.LoadInt64(&activeConnections)))
}

func DecrementActiveConnections() {
	atomic.AddInt64(&activeConnections, -1)
	ActiveConnections.Set(float64(atomic.LoadInt64(&activeConnections)))
}

func SetQueueSize(size int) {
	ProcessingQueueSize.Set(float64(size))
}
