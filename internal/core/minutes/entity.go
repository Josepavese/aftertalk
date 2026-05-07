package minutes

import (
	"encoding/json"
	"strings"
	"time"
)

type MinutesStatus string

const (
	MinutesStatusPending   MinutesStatus = "pending"
	MinutesStatusReady     MinutesStatus = "ready"
	MinutesStatusDelivered MinutesStatus = "delivered"
	MinutesStatusError     MinutesStatus = "error"
)

// Minutes holds the structured output of a session, with flexible sections
// defined by the session template. Sections is a map from section key
// (e.g. "themes", "contents_reported") to the raw JSON value for that section.
// Citations are always present as a typed slice.
type Minutes struct {
	GeneratedAt     time.Time                  `json:"generated_at"`
	Sections        map[string]json.RawMessage `json:"sections"`
	DeliveredAt     *time.Time                 `json:"delivered_at,omitempty"`
	ID              string                     `json:"id"`
	SessionID       string                     `json:"session_id"`
	TemplateID      string                     `json:"template_id"`
	Status          MinutesStatus              `json:"status"`
	Provider        string                     `json:"provider"`
	Summary         Summary                    `json:"summary"`
	LLMUsage        LLMUsageSummary            `json:"llm_usage,omitempty"`
	Citations       []Citation                 `json:"citations"`
	QualityWarnings []string                   `json:"quality_warnings,omitempty"`
	Version         int                        `json:"version"`
}

// Summary is the high-level synopsis of the whole conversation.
type Summary struct {
	Overview string  `json:"overview"`
	Phases   []Phase `json:"phases"`
}

// Phase is one chronological stage in the conversation.
type Phase struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
}

func (s Summary) IsZero() bool {
	return strings.TrimSpace(s.Overview) == "" && len(s.Phases) == 0
}

// Citation is a verbatim quote from the transcript.
type Citation struct {
	Text        string `json:"text"`
	Role        string `json:"role"`
	TimestampMs int    `json:"timestamp_ms"`
}

type LLMUsageSummary struct {
	CostCredits      float64 `json:"cost_credits,omitempty"`
	Calls            int     `json:"calls,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	ReasoningTokens  int     `json:"reasoning_tokens,omitempty"`
	CachedTokens     int     `json:"cached_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
}

// contentBlob is the JSON structure stored in the DB content column.
type contentBlob struct {
	Summary         Summary                    `json:"summary"`
	Sections        map[string]json.RawMessage `json:"sections"`
	Citations       []Citation                 `json:"citations"`
	LLMUsage        LLMUsageSummary            `json:"llm_usage,omitempty"`
	QualityWarnings []string                   `json:"quality_warnings,omitempty"`
}

func NewMinutes(id, sessionID, templateID string) *Minutes {
	return &Minutes{
		ID:          id,
		SessionID:   sessionID,
		TemplateID:  templateID,
		Version:     1,
		Summary:     Summary{Phases: []Phase{}},
		Sections:    map[string]json.RawMessage{},
		Citations:   []Citation{},
		GeneratedAt: time.Now().UTC(),
		Status:      MinutesStatusPending,
	}
}

func (m *Minutes) IncrementVersion() {
	m.Version++
}

func (m *Minutes) MarkReady() {
	m.Status = MinutesStatusReady
}

func (m *Minutes) MarkDelivered() {
	now := time.Now().UTC()
	m.DeliveredAt = &now
	m.Status = MinutesStatusDelivered
}

func (m *Minutes) MarkError() {
	m.Status = MinutesStatusError
}

// MarshalContent serializes Sections+Citations into a single JSON blob for DB storage.
func (m *Minutes) MarshalContent() (string, error) {
	blob := contentBlob{
		Summary:         m.Summary,
		Sections:        m.Sections,
		Citations:       m.Citations,
		LLMUsage:        m.LLMUsage,
		QualityWarnings: m.QualityWarnings,
	}
	b, err := json.Marshal(blob)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalContent restores Sections and Citations from the DB JSON blob.
func (m *Minutes) UnmarshalContent(raw string) error {
	var blob contentBlob
	if err := json.Unmarshal([]byte(raw), &blob); err != nil {
		return err
	}
	if blob.Sections == nil {
		blob.Sections = map[string]json.RawMessage{}
	}
	if blob.Summary.Phases == nil {
		blob.Summary.Phases = []Phase{}
	}
	if blob.Citations == nil {
		blob.Citations = []Citation{}
	}
	if blob.QualityWarnings == nil {
		blob.QualityWarnings = []string{}
	}
	m.Summary = blob.Summary
	m.Sections = blob.Sections
	m.Citations = blob.Citations
	m.LLMUsage = blob.LLMUsage
	m.QualityWarnings = blob.QualityWarnings
	return nil
}

type MinutesHistory struct {
	EditedAt  time.Time `json:"edited_at"`
	ID        string    `json:"id"`
	MinutesID string    `json:"minutes_id"`
	Content   string    `json:"content"`
	EditedBy  string    `json:"edited_by,omitempty"`
	Version   int       `json:"version"`
}

func NewMinutesHistory(id, minutesID string, version int, content string) *MinutesHistory {
	return &MinutesHistory{
		ID:        id,
		MinutesID: minutesID,
		Version:   version,
		Content:   content,
		EditedAt:  time.Now().UTC(),
	}
}

func (h *MinutesHistory) SetEditedBy(editedBy string) {
	h.EditedBy = editedBy
}
