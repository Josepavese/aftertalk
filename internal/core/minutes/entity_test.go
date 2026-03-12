package minutes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinutesStatusConstants(t *testing.T) {
	assert.Equal(t, MinutesStatus("pending"), MinutesStatusPending)
	assert.Equal(t, MinutesStatus("ready"), MinutesStatusReady)
	assert.Equal(t, MinutesStatus("delivered"), MinutesStatusDelivered)
	assert.Equal(t, MinutesStatus("error"), MinutesStatusError)
}

func TestNewMinutes(t *testing.T) {
	m := NewMinutes("test-id", "session-123", "tmpl-1")

	assert.Equal(t, "test-id", m.ID)
	assert.Equal(t, "session-123", m.SessionID)
	assert.Equal(t, "tmpl-1", m.TemplateID)
	assert.Equal(t, 1, m.Version)
	assert.NotZero(t, m.GeneratedAt)
	assert.Equal(t, MinutesStatusPending, m.Status)
	assert.Empty(t, m.Citations)
}

func TestMinutesMethods(t *testing.T) {
	m := NewMinutes("test-id", "session-123", "tmpl-1")

	t.Run("IncrementVersion", func(t *testing.T) {
		m.IncrementVersion()
		assert.Equal(t, 2, m.Version)

		m.IncrementVersion()
		assert.Equal(t, 3, m.Version)
	})

	t.Run("MarkReady", func(t *testing.T) {
		m.MarkReady()
		assert.Equal(t, MinutesStatusReady, m.Status)
		assert.NotZero(t, m.GeneratedAt)
	})

	t.Run("MarkDelivered", func(t *testing.T) {
		var before time.Time
		if m.DeliveredAt != nil {
			before = *m.DeliveredAt
		}

		m.MarkDelivered()
		assert.Equal(t, MinutesStatusDelivered, m.Status)
		assert.NotNil(t, m.DeliveredAt)
		assert.NotZero(t, m.DeliveredAt)

		if !before.IsZero() {
			assert.True(t, m.DeliveredAt.Sub(before) >= 0)
		}
	})

	t.Run("MarkError", func(t *testing.T) {
		m.MarkError()
		assert.Equal(t, MinutesStatusError, m.Status)
	})
}

func TestCitation(t *testing.T) {
	citation := Citation{
		TimestampMs: 123456,
		Text:        "Test quote",
		Role:        "client",
	}

	assert.Equal(t, 123456, citation.TimestampMs)
	assert.Equal(t, "Test quote", citation.Text)
	assert.Equal(t, "client", citation.Role)
}

func TestCitationWithZeroTimestamp(t *testing.T) {
	citation := Citation{
		TimestampMs: 0,
		Text:        "Zero timestamp citation",
		Role:        "system",
	}

	assert.Equal(t, 0, citation.TimestampMs)
}

func TestMinutesJSONSerialization(t *testing.T) {
	m := NewMinutes("test-id", "session-123", "tmpl-1")
	m.Sections = map[string]json.RawMessage{
		"themes": json.RawMessage(`["Theme 1","Theme 2"]`),
	}
	m.Citations = []Citation{
		{TimestampMs: 1000, Text: "Quote 1", Role: "client"},
	}
	m.MarkReady()

	jsonData, err := json.Marshal(m)
	require.NoError(t, err)
	require.NotEmpty(t, jsonData)

	var decoded Minutes
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, m.ID, decoded.ID)
	assert.Equal(t, m.SessionID, decoded.SessionID)
	assert.Equal(t, m.TemplateID, decoded.TemplateID)
	assert.Equal(t, m.Version, decoded.Version)
	assert.Equal(t, m.Status, decoded.Status)
	assert.Len(t, decoded.Citations, 1)
	assert.Equal(t, 1000, decoded.Citations[0].TimestampMs)
}

func TestNewMinutesHistory(t *testing.T) {
	history := NewMinutesHistory("history-id", "minutes-123", 1, "Test content")

	assert.Equal(t, "history-id", history.ID)
	assert.Equal(t, "minutes-123", history.MinutesID)
	assert.Equal(t, 1, history.Version)
	assert.Equal(t, "Test content", history.Content)
	assert.NotZero(t, history.EditedAt)
	assert.Empty(t, history.EditedBy)
}

func TestMinutesHistoryMethods(t *testing.T) {
	history := NewMinutesHistory("history-id", "minutes-123", 1, "Test content")

	history.SetEditedBy("user-123")

	assert.Equal(t, "user-123", history.EditedBy)
}

func TestMinutesHistoryJSONSerialization(t *testing.T) {
	history := NewMinutesHistory("history-id", "minutes-123", 2, "Test content")
	history.SetEditedBy("user-456")

	jsonData, err := json.Marshal(history)
	require.NoError(t, err)
	require.NotEmpty(t, jsonData)

	var decoded MinutesHistory
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, history.ID, decoded.ID)
	assert.Equal(t, history.MinutesID, decoded.MinutesID)
	assert.Equal(t, history.Version, decoded.Version)
	assert.Equal(t, history.Content, decoded.Content)
	assert.Equal(t, "user-456", decoded.EditedBy)
}

func TestMinutesStatusLifecycle(t *testing.T) {
	m := NewMinutes("test-id", "session-123", "tmpl-1")

	assert.Equal(t, MinutesStatusPending, m.Status)

	m.MarkReady()
	assert.Equal(t, MinutesStatusReady, m.Status)

	m.MarkDelivered()
	assert.Equal(t, MinutesStatusDelivered, m.Status)
	assert.NotNil(t, m.DeliveredAt)

	m.MarkError()
	assert.Equal(t, MinutesStatusError, m.Status)
}

func TestMinutesMultipleVersions(t *testing.T) {
	m := NewMinutes("test-id", "session-123", "tmpl-1")

	m.IncrementVersion()
	m.IncrementVersion()

	assert.Equal(t, 3, m.Version)
}
