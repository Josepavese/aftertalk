package minutes

import "time"

type MinutesStatus string

const (
	MinutesStatusPending   MinutesStatus = "pending"
	MinutesStatusReady     MinutesStatus = "ready"
	MinutesStatusDelivered MinutesStatus = "delivered"
	MinutesStatusError     MinutesStatus = "error"
)

type Minutes struct {
	ID                        string        `json:"id"`
	SessionID                 string        `json:"session_id"`
	Version                   int           `json:"version"`
	Themes                    []string      `json:"themes"`
	ContentsReported          []ContentItem `json:"contents_reported"`
	ProfessionalInterventions []ContentItem `json:"professional_interventions"`
	ProgressIssues            Progress      `json:"progress_issues"`
	NextSteps                 []string      `json:"next_steps"`
	Citations                 []Citation    `json:"citations"`
	GeneratedAt               time.Time     `json:"generated_at"`
	DeliveredAt               *time.Time    `json:"delivered_at,omitempty"`
	Status                    MinutesStatus `json:"status"`
	Provider                  string        `json:"provider"`
}

type ContentItem struct {
	Text      string `json:"text"`
	Timestamp int    `json:"timestamp,omitempty"`
}

type Progress struct {
	Progress []string `json:"progress"`
	Issues   []string `json:"issues"`
}

type Citation struct {
	TimestampMs int    `json:"timestamp_ms"`
	Text        string `json:"text"`
	Role        string `json:"role"`
}

func NewMinutes(id, sessionID string) *Minutes {
	return &Minutes{
		ID:                        id,
		SessionID:                 sessionID,
		Version:                   1,
		Themes:                    make([]string, 0),
		ContentsReported:          make([]ContentItem, 0),
		ProfessionalInterventions: make([]ContentItem, 0),
		ProgressIssues:            Progress{Progress: make([]string, 0), Issues: make([]string, 0)},
		NextSteps:                 make([]string, 0),
		Citations:                 make([]Citation, 0),
		GeneratedAt:               time.Now().UTC(),
		Status:                    MinutesStatusPending,
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

type MinutesHistory struct {
	ID        string    `json:"id"`
	MinutesID string    `json:"minutes_id"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	EditedAt  time.Time `json:"edited_at"`
	EditedBy  string    `json:"edited_by,omitempty"`
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
