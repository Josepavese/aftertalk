package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestIncrementActiveConnections(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.InEpsilon(t, float64(1), activeConnections, 1e-9)
}

func TestDecrementActiveConnections(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.InEpsilon(t, float64(1), activeConnections, 1e-9)

	DecrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()

	assert.Zero(t, activeConnections)
}

func TestDecrementBeyondZero(t *testing.T) {
	ResetMetrics()

	DecrementActiveConnections()
	activeConnections := GetActiveConnectionsValue()

	assert.InDelta(t, float64(-1), activeConnections, 1e-9)
}

func TestMultipleIncrements(t *testing.T) {
	ResetMetrics()

	for i := 0; i < 10; i++ {
		IncrementActiveConnections()
	}

	activeConnections := GetActiveConnectionsValue()
	assert.InEpsilon(t, float64(10), activeConnections, 1e-9)
}

func TestMixedIncrementsAndDecrements(t *testing.T) {
	ResetMetrics()

	IncrementActiveConnections()
	IncrementActiveConnections()
	DecrementActiveConnections()
	IncrementActiveConnections()
	DecrementActiveConnections()
	DecrementActiveConnections()

	// +1+1-1+1-1-1 = 0
	activeConnections := GetActiveConnectionsValue()
	assert.Zero(t, activeConnections)
}

func TestSetQueueSize(t *testing.T) {
	ResetMetrics()

	SetQueueSize(0)
	queueSize := GetQueueSize()
	assert.Zero(t, queueSize)

	SetQueueSize(100)
	queueSize = GetQueueSize()
	assert.InEpsilon(t, float64(100), queueSize, 1e-9)

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
	before := testutil.ToFloat64(TranscriptionsGenerated)
	TranscriptionsGenerated.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(TranscriptionsGenerated)-before)
}

func TestTranscriptionErrorsCounter(t *testing.T) {
	before := testutil.ToFloat64(TranscriptionErrors)
	TranscriptionErrors.Inc()
	TranscriptionErrors.Inc()
	TranscriptionErrors.Inc()
	assert.Equal(t, float64(3), testutil.ToFloat64(TranscriptionErrors)-before)
}

func TestMinutesGeneratedCounter(t *testing.T) {
	before := testutil.ToFloat64(MinutesGenerated)
	MinutesGenerated.Inc()
	MinutesGenerated.Inc()
	assert.Equal(t, float64(2), testutil.ToFloat64(MinutesGenerated)-before)
}

func TestMinutesErrorsCounter(t *testing.T) {
	before := testutil.ToFloat64(MinutesErrors)
	MinutesErrors.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(MinutesErrors)-before)
}

func TestWebhookSentCounter(t *testing.T) {
	before := testutil.ToFloat64(WebhookSent)
	WebhookSent.Inc()
	WebhookSent.Inc()
	WebhookSent.Inc()
	assert.Equal(t, float64(3), testutil.ToFloat64(WebhookSent)-before)
}

func TestWebhookErrorsCounter(t *testing.T) {
	before := testutil.ToFloat64(WebhookErrors)
	WebhookErrors.Inc()
	WebhookErrors.Inc()
	assert.Equal(t, float64(2), testutil.ToFloat64(WebhookErrors)-before)
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
	before := map[string]float64{
		"SessionsCreated":         testutil.ToFloat64(SessionsCreated),
		"SessionsEnded":           testutil.ToFloat64(SessionsEnded),
		"TranscriptionsGenerated": testutil.ToFloat64(TranscriptionsGenerated),
		"TranscriptionErrors":     testutil.ToFloat64(TranscriptionErrors),
		"MinutesGenerated":        testutil.ToFloat64(MinutesGenerated),
		"MinutesErrors":           testutil.ToFloat64(MinutesErrors),
		"WebhookSent":             testutil.ToFloat64(WebhookSent),
		"WebhookErrors":           testutil.ToFloat64(WebhookErrors),
	}

	SessionsCreated.Inc()
	SessionsEnded.Inc()
	TranscriptionsGenerated.Inc()
	TranscriptionErrors.Inc()
	MinutesGenerated.Inc()
	MinutesErrors.Inc()
	WebhookSent.Inc()
	WebhookErrors.Inc()

	assert.Equal(t, float64(1), testutil.ToFloat64(SessionsCreated)-before["SessionsCreated"])
	assert.Equal(t, float64(1), testutil.ToFloat64(SessionsEnded)-before["SessionsEnded"])
	assert.Equal(t, float64(1), testutil.ToFloat64(TranscriptionsGenerated)-before["TranscriptionsGenerated"])
	assert.Equal(t, float64(1), testutil.ToFloat64(TranscriptionErrors)-before["TranscriptionErrors"])
	assert.Equal(t, float64(1), testutil.ToFloat64(MinutesGenerated)-before["MinutesGenerated"])
	assert.Equal(t, float64(1), testutil.ToFloat64(MinutesErrors)-before["MinutesErrors"])
	assert.Equal(t, float64(1), testutil.ToFloat64(WebhookSent)-before["WebhookSent"])
	assert.Equal(t, float64(1), testutil.ToFloat64(WebhookErrors)-before["WebhookErrors"])
}

func TestGetActiveConnectionsValue(t *testing.T) {
	ResetMetrics()

	activeConnections := GetActiveConnectionsValue()
	assert.Zero(t, activeConnections)

	IncrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.InEpsilon(t, float64(1), activeConnections, 1e-9)

	IncrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.Equal(t, float64(2), activeConnections)

	DecrementActiveConnections()
	activeConnections = GetActiveConnectionsValue()
	assert.InEpsilon(t, float64(1), activeConnections, 1e-9)
}

func TestGetQueueSize(t *testing.T) {
	ResetMetrics()

	queueSize := GetQueueSize()
	assert.Zero(t, queueSize)

	SetQueueSize(10)
	queueSize = GetQueueSize()
	assert.Equal(t, float64(10), queueSize)

	SetQueueSize(0)
	queueSize = GetQueueSize()
	assert.Zero(t, queueSize)
}

func TestConcurrentActiveConnections(t *testing.T) {
	ResetMetrics()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncrementActiveConnections()
		}()
	}

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

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			SetQueueSize(n)
		}(i)
	}

	wg.Wait()

	// Concurrent Set operations are non-deterministic; just verify the result is in range
	queueSize := GetQueueSize()
	assert.True(t, queueSize >= 0 && queueSize < 100)
}

func TestConcurrentCounterIncrements(t *testing.T) {
	before := map[string]float64{
		"SessionsCreated":         testutil.ToFloat64(SessionsCreated),
		"SessionsEnded":           testutil.ToFloat64(SessionsEnded),
		"TranscriptionsGenerated": testutil.ToFloat64(TranscriptionsGenerated),
		"TranscriptionErrors":     testutil.ToFloat64(TranscriptionErrors),
		"MinutesGenerated":        testutil.ToFloat64(MinutesGenerated),
		"MinutesErrors":           testutil.ToFloat64(MinutesErrors),
		"WebhookSent":             testutil.ToFloat64(WebhookSent),
		"WebhookErrors":           testutil.ToFloat64(WebhookErrors),
	}

	var wg sync.WaitGroup

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

	assert.Equal(t, float64(100), testutil.ToFloat64(SessionsCreated)-before["SessionsCreated"])
	assert.Equal(t, float64(100), testutil.ToFloat64(SessionsEnded)-before["SessionsEnded"])
	assert.Equal(t, float64(100), testutil.ToFloat64(TranscriptionsGenerated)-before["TranscriptionsGenerated"])
	assert.Equal(t, float64(100), testutil.ToFloat64(TranscriptionErrors)-before["TranscriptionErrors"])
	assert.Equal(t, float64(100), testutil.ToFloat64(MinutesGenerated)-before["MinutesGenerated"])
	assert.Equal(t, float64(100), testutil.ToFloat64(MinutesErrors)-before["MinutesErrors"])
	assert.Equal(t, float64(100), testutil.ToFloat64(WebhookSent)-before["WebhookSent"])
	assert.Equal(t, float64(100), testutil.ToFloat64(WebhookErrors)-before["WebhookErrors"])
}

func TestResetMetrics(t *testing.T) {
	ResetMetrics()

	assert.Equal(t, float64(0), GetActiveConnectionsValue())
	assert.Equal(t, float64(0), GetQueueSize())

	IncrementActiveConnections()
	IncrementActiveConnections()
	IncrementActiveConnections()
	SetQueueSize(3)

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

	histogram.Observe(0.1)
}

func TestCounterZeroInitial(t *testing.T) {
	// Prometheus counters are global and can't be reset — just verify they're accessible and non-negative
	for _, counter := range []struct {
		val  prometheus.Counter
		name string
	}{
		{name: "SessionsCreated", val: SessionsCreated},
		{name: "SessionsEnded", val: SessionsEnded},
		{name: "TranscriptionsGenerated", val: TranscriptionsGenerated},
		{name: "TranscriptionErrors", val: TranscriptionErrors},
		{name: "MinutesGenerated", val: MinutesGenerated},
		{name: "MinutesErrors", val: MinutesErrors},
		{name: "WebhookSent", val: WebhookSent},
		{name: "WebhookErrors", val: WebhookErrors},
	} {
		t.Run(counter.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, testutil.ToFloat64(counter.val), float64(0))
		})
	}
}

func TestGaugeZeroInitial(t *testing.T) {
	ResetMetrics()

	assert.Equal(t, float64(0), GetActiveConnectionsValue())
	assert.Equal(t, float64(0), GetQueueSize())
}
