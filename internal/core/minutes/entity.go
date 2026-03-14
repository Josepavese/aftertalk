package minutes

import (
	"encoding/json"
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
	GeneratedAt time.Time                  `json:"generated_at"`
	Sections    map[string]json.RawMessage `json:"sections"`
	DeliveredAt *time.Time                 `json:"delivered_at,omitempty"`
	ID          string                     `json:"id"`
	SessionID   string                     `json:"session_id"`
	TemplateID  string                     `json:"template_id"`
	Status      MinutesStatus              `json:"status"`
	Provider    string                     `json:"provider"`
	Citations   []Citation                 `json:"citations"`
	Version     int                        `json:"version"`
}

// Citation is a verbatim quote from the transcript.
type Citation struct {
	Text        string `json:"text"`
	Role        string `json:"role"`
	TimestampMs int    `json:"timestamp_ms"`
}

// contentBlob is the JSON structure stored in the DB content column.
type contentBlob struct {
	Sections  map[string]json.RawMessage `json:"sections"`
	Citations []Citation                 `json:"citations"`
}

func NewMinutes(id, sessionID, templateID string) *Minutes {
	return &Minutes{
		ID:          id,
		SessionID:   sessionID,
		TemplateID:  templateID,
		Version:     1,
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
		Sections:  m.Sections,
		Citations: m.Citations,
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
	if blob.Citations == nil {
		blob.Citations = []Citation{}
	}
	m.Sections = blob.Sections
	m.Citations = blob.Citations
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
