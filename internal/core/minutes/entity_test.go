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
	minutes := NewMinutes("test-id", "session-123")

	assert.Equal(t, "test-id", minutes.ID)
	assert.Equal(t, "session-123", minutes.SessionID)
	assert.Equal(t, 1, minutes.Version)
	assert.NotNil(t, minutes.GeneratedAt)
	assert.Equal(t, MinutesStatusPending, minutes.Status)
	assert.Empty(t, minutes.Themes)
	assert.Empty(t, minutes.ContentsReported)
	assert.Empty(t, minutes.ProfessionalInterventions)
	assert.Empty(t, minutes.ProgressIssues.Progress)
	assert.Empty(t, minutes.ProgressIssues.Issues)
	assert.Empty(t, minutes.NextSteps)
	assert.Empty(t, minutes.Citations)
}

func TestMinutesMethods(t *testing.T) {
	minutes := NewMinutes("test-id", "session-123")

	t.Run("IncrementVersion", func(t *testing.T) {
		minutes.IncrementVersion()
		assert.Equal(t, 2, minutes.Version)

		minutes.IncrementVersion()
		assert.Equal(t, 3, minutes.Version)
	})

	t.Run("MarkReady", func(t *testing.T) {
		minutes.MarkReady()
		assert.Equal(t, MinutesStatusReady, minutes.Status)
		assert.NotZero(t, minutes.GeneratedAt)
	})

	t.Run("MarkDelivered", func(t *testing.T) {
		var before time.Time
		if minutes.DeliveredAt != nil {
			before = *minutes.DeliveredAt
		}

		minutes.MarkDelivered()
		assert.Equal(t, MinutesStatusDelivered, minutes.Status)
		assert.NotNil(t, minutes.DeliveredAt)
		assert.NotZero(t, minutes.DeliveredAt)

		if !before.IsZero() {
			assert.True(t, minutes.DeliveredAt.Sub(before) >= 0)
		}
	})

	t.Run("MarkError", func(t *testing.T) {
		minutes.MarkError()
		assert.Equal(t, MinutesStatusError, minutes.Status)
	})
}

func TestContentItem(t *testing.T) {
	item := ContentItem{
		Text:      "Test text",
		Timestamp: 1000,
	}

	assert.Equal(t, "Test text", item.Text)
	assert.Equal(t, 1000, item.Timestamp)
}

func TestProgress(t *testing.T) {
	progress := Progress{
		Progress: []string{"Progress 1", "Progress 2"},
		Issues:   []string{"Issue 1", "Issue 2"},
	}

	assert.Equal(t, 2, len(progress.Progress))
	assert.Equal(t, 2, len(progress.Issues))
	assert.Equal(t, "Progress 1", progress.Progress[0])
	assert.Equal(t, "Issue 1", progress.Issues[0])
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

func TestMinutesJSONSerialization(t *testing.T) {
	minutes := NewMinutes("test-id", "session-123")
	minutes.Themes = []string{"Theme 1", "Theme 2"}
	minutes.ContentsReported = []ContentItem{
		{Text: "Content 1", Timestamp: 100},
		{Text: "Content 2", Timestamp: 200},
	}
	minutes.ProfessionalInterventions = []ContentItem{
		{Text: "Intervention 1", Timestamp: 300},
	}
	minutes.ProgressIssues = Progress{
		Progress: []string{"Progress item 1"},
		Issues:   []string{"Issue 1"},
	}
	minutes.NextSteps = []string{"Step 1", "Step 2"}
	minutes.Citations = []Citation{
		{TimestampMs: 1000, Text: "Quote 1", Role: "client"},
		{TimestampMs: 2000, Text: "Quote 2", Role: "professional"},
	}
	minutes.MarkReady()

	jsonData, err := json.Marshal(minutes)
	require.NoError(t, err)
	require.NotEmpty(t, jsonData)

	var decoded Minutes
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, minutes.ID, decoded.ID)
	assert.Equal(t, minutes.SessionID, decoded.SessionID)
	assert.Equal(t, minutes.Version, decoded.Version)
	assert.Equal(t, minutes.Status, decoded.Status)
	assert.Equal(t, minutes.Provider, decoded.Provider)

	assert.Equal(t, len(minutes.Themes), len(decoded.Themes))
	assert.Equal(t, minutes.Themes[0], decoded.Themes[0])

	assert.Equal(t, len(minutes.ContentsReported), len(decoded.ContentsReported))
	assert.Equal(t, minutes.ContentsReported[0].Text, decoded.ContentsReported[0].Text)
	assert.Equal(t, minutes.ContentsReported[0].Timestamp, decoded.ContentsReported[0].Timestamp)

	assert.Equal(t, len(minutes.ProfessionalInterventions), len(decoded.ProfessionalInterventions))
	assert.Equal(t, minutes.ProfessionalInterventions[0].Text, decoded.ProfessionalInterventions[0].Text)

	assert.Equal(t, len(minutes.ProgressIssues.Progress), len(decoded.ProgressIssues.Progress))
	assert.Equal(t, minutes.ProgressIssues.Progress[0], decoded.ProgressIssues.Progress[0])

	assert.Equal(t, len(minutes.ProgressIssues.Issues), len(decoded.ProgressIssues.Issues))
	assert.Equal(t, minutes.ProgressIssues.Issues[0], decoded.ProgressIssues.Issues[0])

	assert.Equal(t, len(minutes.NextSteps), len(decoded.NextSteps))
	assert.Equal(t, minutes.NextSteps[0], decoded.NextSteps[0])

	assert.Equal(t, len(minutes.Citations), len(decoded.Citations))
	assert.Equal(t, minutes.Citations[0].TimestampMs, decoded.Citations[0].TimestampMs)
	assert.Equal(t, minutes.Citations[0].Text, decoded.Citations[0].Text)
	assert.Equal(t, minutes.Citations[0].Role, decoded.Citations[0].Role)
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
	minutes := NewMinutes("test-id", "session-123")

	assert.Equal(t, MinutesStatusPending, minutes.Status)

	minutes.MarkReady()
	assert.Equal(t, MinutesStatusReady, minutes.Status)

	minutes.MarkDelivered()
	assert.Equal(t, MinutesStatusDelivered, minutes.Status)
	assert.NotNil(t, minutes.DeliveredAt)
	assert.NotZero(t, minutes.DeliveredAt)

	minutes.MarkError()
	assert.Equal(t, MinutesStatusError, minutes.Status)
}

func TestMinutesMultipleVersions(t *testing.T) {
	minutes := NewMinutes("test-id", "session-123")
	minutes.Themes = []string{"Theme 1"}

	minutes.IncrementVersion()
	minutes.Themes = []string{"Theme 1", "Theme 2"}

	minutes.IncrementVersion()
	minutes.Themes = []string{"Theme 1", "Theme 2", "Theme 3"}

	assert.Equal(t, 3, minutes.Version)
	assert.Equal(t, "Theme 3", minutes.Themes[2])
}

func TestContentItemWithNoTimestamp(t *testing.T) {
	item := ContentItem{
		Text: "Item without timestamp",
	}

	assert.Equal(t, "Item without timestamp", item.Text)
	assert.Equal(t, 0, item.Timestamp)
}

func TestProgressEmpty(t *testing.T) {
	progress := Progress{
		Progress: []string{},
		Issues:   []string{},
	}

	assert.Equal(t, 0, len(progress.Progress))
	assert.Equal(t, 0, len(progress.Issues))
}

func TestCitationWithZeroTimestamp(t *testing.T) {
	citation := Citation{
		TimestampMs: 0,
		Text:        "Zero timestamp citation",
		Role:        "system",
	}

	assert.Equal(t, 0, citation.TimestampMs)
	assert.Equal(t, "Zero timestamp citation", citation.Text)
	assert.Equal(t, "system", citation.Role)
}
