package metrics

import (
	"sync"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncrementActiveConnections(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.Equal(t, float64(1), activeConnections)
}

func TestDecrementActiveConnections(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.Equal(t, float64(1), activeConnections)

	DecrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()

	assert.Equal(t, float64(0), activeConnections)
}

func TestDecrementBeyondZero(t *testing.T) {
	ResetMetrics()

	DecrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.Equal(t, float64(-1), activeConnections)
}

func TestMultipleIncrements(t *testing.T) {
	ResetMetrics()

	for i := 0; i < 10; i++ {
		IncrementActiveConnections()
	}

	activeConnections := GetActiveConnectionsValue()
	assert.Equal(t, float64(10), activeConnections)
}

func TestMixedIncrementsAndDecrements(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	IncrementActiveConnections()
	DecrementActiveConnections()
	IncrementActiveConnections()
	DecrementActiveConnections()
	DecrementActiveConnections()

	activeConnections := GetActiveConnectionsValue()
	assert.Equal(t, float64(-1), activeConnections)
}

func TestSetQueueSize(t *testing.T) {
	ResetMetrics()

	SetQueueSize(0)
	queueSize := GetQueueSize()
	assert.Equal(t, float64(0), queueSize)

	SetQueueSize(100)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(100), queueSize)

	SetQueueSize(999)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(999), queueSize)

	SetQueueSize(1)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(1), queueSize)
}

func TestSessionsCreatedCounter(t *testing.T) {
	ResetMetrics()

	for i := 0; i < 5; i++ {
		SessionsCreated.Inc()
	}

	// Counter increments successfully
	assert.NotPanics(t, func() {
		SessionsCreated.Inc()
	})
}

func TestSessionsEndedCounter(t *testing.T) {
	ResetMetrics()

	SessionsEnded.Inc()
	SessionsEnded.Inc()

	assert.NotPanics(t, func() {
		SessionsEnded.Inc()
	})
}

func TestTranscriptionsGeneratedCounter(t *testing.T) {
	ResetMetrics()

	TranscriptionsGenerated.Inc()

	assert.Equal(t, float64(1), TranscriptionsGenerated)
}

func TestTranscriptionErrorsCounter(t *testing.T) {
	ResetMetrics()

	TranscriptionErrors.Inc()
	TranscriptionErrors.Inc()
	TranscriptionErrors.Inc()

	assert.Equal(t, float64(3), TranscriptionErrors)
}

func TestMinutesGeneratedCounter(t *testing.T) {
	ResetMetrics()

	MinutesGenerated.Inc()
	MinutesGenerated.Inc()

	assert.Equal(t, float64(2), MinutesGenerated)
}

func TestMinutesErrorsCounter(t *testing.T) {
	ResetMetrics()

	MinutesErrors.Inc()

	assert.Equal(t, float64(1), MinutesErrors)
}

func TestWebhookSentCounter(t *testing.T) {
	ResetMetrics()

	WebhookSent.Inc()
	WebhookSent.Inc()
	WebhookSent.Inc()

	assert.Equal(t, float64(3), WebhookSent)
}

func TestWebhookErrorsCounter(t *testing.T) {
	ResetMetrics()

	WebhookErrors.Inc()
	WebhookErrors.Inc()

	assert.Equal(t, float64(2), WebhookErrors)
}

func TestRequestDurationHistogram(t *testing.T) {
	ResetMetrics()

	RequestDuration.WithLabelValues("GET", "/api/sessions", "200").Observe(0.1)
	RequestDuration.WithLabelValues("GET", "/api/sessions", "404").Observe(0.2)
	RequestDuration.WithLabelValues("POST", "/api/sessions", "200").Observe(0.3)
	RequestDuration.WithLabelValues("DELETE", "/api/sessions", "500").Observe(0.4)
}

func TestRequestDurationMultipleObservations(t *testing.T) {
	ResetMetrics()

	histogram := RequestDuration.WithLabelValues("GET", "/api/sessions", "200")

	for i := 0; i < 10; i++ {
		histogram.Observe(float64(i) * 0.1)
	}
}

func TestAllCountersIndependence(t *testing.T) {
	ResetMetrics()

	SessionsCreated.Inc()
	SessionsEnded.Inc()
	TranscriptionsGenerated.Inc()
	TranscriptionErrors.Inc()
	MinutesGenerated.Inc()
	MinutesErrors.Inc()
	WebhookSent.Inc()
	WebhookErrors.Inc()

	assert.Equal(t, float64(1), SessionsCreated)
	assert.Equal(t, float64(1), SessionsEnded)
	assert.Equal(t, float64(1), TranscriptionsGenerated)
	assert.Equal(t, float64(1), TranscriptionErrors)
	assert.Equal(t, float64(1), MinutesGenerated)
	assert.Equal(t, float64(1), MinutesErrors)
	assert.Equal(t, float64(1), WebhookSent)
	assert.Equal(t, float64(1), WebhookErrors)
}

func TestGetActiveConnectionsValue(t *testing.T) {
	ResetMetrics()

	// Should return 0 when no connections
	activeConnections := GetActiveConnectionsValue()
	assert.Equal(t, float64(0), activeConnections)

	// Increment once
	IncrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.Equal(t, float64(1), activeConnections)

	// Increment again
	IncrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.Equal(t, float64(2), activeConnections)

	// Decrement
	DecrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.Equal(t, float64(1), activeConnections)
}

func TestGetQueueSize(t *testing.T) {
	ResetMetrics()

	// Should return 0 when queue is empty
	queueSize := GetQueueSize()
	assert.Equal(t, float64(0), queueSize)

	// Set queue size
	SetQueueSize(10)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(10), queueSize)

	// Clear queue
	SetQueueSize(0)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(0), queueSize)
}

func TestConcurrentActiveConnections(t *testing.T) {
	ResetMetrics()

	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncrementActiveConnections()
		}()
	}

	// Concurrent decrements
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			DecrementActiveConnections()
		}()
	}

	wg.Wait()

	activeConnections := GetActiveConnectionsValue()
	assert.Equal(t, float64(50), activeConnections)
}

func TestConcurrentQueueSizeUpdates(t *testing.T) {
	ResetMetrics()

	var wg sync.WaitGroup

	// Concurrent queue size updates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			SetQueueSize(n)
		}(i)
	}

	wg.Wait()

	queueSize := GetQueueSize()
	assert.Equal(t, float64(99), queueSize)
}

func TestConcurrentCounterIncrements(t *testing.T) {
	ResetMetrics()

	var wg sync.WaitGroup

	// Concurrent increments on all counters
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SessionsCreated.Inc()
			SessionsEnded.Inc()
			TranscriptionsGenerated.Inc()
			TranscriptionErrors.Inc()
			MinutesGenerated.Inc()
			MinutesErrors.Inc()
			WebhookSent.Inc()
			WebhookErrors.Inc()
		}()
	}

	wg.Wait()

	assert.Equal(t, float64(100), SessionsCreated)
	assert.Equal(t, float64(100), SessionsEnded)
	assert.Equal(t, float64(100), TranscriptionsGenerated)
	assert.Equal(t, float64(100), TranscriptionErrors)
	assert.Equal(t, float64(100), MinutesGenerated)
	assert.Equal(t, float64(100), MinutesErrors)
	assert.Equal(t, float64(100), WebhookSent)
	assert.Equal(t, float64(100), WebhookErrors)
}

func TestResetMetrics(t *testing.T) {
	ResetMetrics()

	// All metrics should start at 0 or default values
	assert.Equal(t, float64(0), GetActiveConnectionsValue())
	assert.Equal(t, float64(0), GetQueueSize())

	SessionsCreated.Inc()
	SessionsEnded.Inc()

	SessionsCreated.Inc()
	SessionsEnded.Inc()

	SessionsCreated.Inc()
	SessionsEnded.Inc()

	assert.Equal(t, float64(3), GetActiveConnectionsValue())
	assert.Equal(t, float64(3), GetQueueSize())

	ResetMetrics()

	assert.Equal(t, float64(0), GetActiveConnectionsValue())
	assert.Equal(t, float64(0), GetQueueSize())
}

func TestRequestDurationWithMultipleLabels(t *testing.T) {
	ResetMetrics()

	RequestDuration.WithLabelValues("POST", "/api/sessions/create", "201").Observe(0.5)
	RequestDuration.WithLabelValues("GET", "/api/sessions/{id}", "200").Observe(1.0)
	RequestDuration.WithLabelValues("PUT", "/api/sessions/{id}", "200").Observe(0.75)
	RequestDuration.WithLabelValues("DELETE", "/api/sessions/{id}", "204").Observe(0.3)
}

func TestHistogramInvariants(t *testing.T) {
	ResetMetrics()

	histogram := RequestDuration.WithLabelValues("GET", "/api/sessions", "200")

	// After first observation, count should be 1
	histogram.Observe(0.1)
	// Note: Can't directly verify histogram values without prometheus API access
	// This test ensures the API doesn't panic
}

func TestCounterZeroInitial(t *testing.T) {
	ResetMetrics()

	for _, counter := range []struct {
		name string
		val  prometheus.Counter
	}{
		{"SessionsCreated", SessionsCreated},
		{"SessionsEnded", SessionsEnded},
		{"TranscriptionsGenerated", TranscriptionsGenerated},
		{"TranscriptionErrors", TranscriptionErrors},
		{"MinutesGenerated", MinutesGenerated},
		{"MinutesErrors", MinutesErrors},
		{"WebhookSent", WebhookSent},
		{"WebhookErrors", WebhookErrors},
	} {
		t.Run(counter.name, func(t *testing.T) {
			assert.Equal(t, float64(0), counter.val)
		})
	}
}

func TestGaugeZeroInitial(t *testing.T) {
	ResetMetrics()

	assert.Equal(t, float64(0), GetActiveConnectionsValue())
	assert.Equal(t, float64(0), GetQueueSize())
}
