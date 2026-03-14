package transcription

import "time"

type TranscriptionStatus string

const (
	StatusPending    TranscriptionStatus = "pending"
	StatusProcessing TranscriptionStatus = "processing"
	StatusReady      TranscriptionStatus = "ready"
	StatusError      TranscriptionStatus = "error"
)

type Transcription struct {
	CreatedAt    time.Time           `json:"created_at"`
	ID           string              `json:"id"`
	SessionID    string              `json:"session_id"`
	Role         string              `json:"role"`
	Text         string              `json:"text"`
	Provider     string              `json:"provider"`
	Status       TranscriptionStatus `json:"status"`
	SegmentIndex int                 `json:"segment_index"`
	StartMs      int                 `json:"start_ms"`
	EndMs        int                 `json:"end_ms"`
	Confidence   float64             `json:"confidence,omitempty"`
}

func NewTranscription(id, sessionID string, segmentIndex int, role string, startMs, endMs int, text string) *Transcription {
	return &Transcription{
		ID:           id,
		SessionID:    sessionID,
		SegmentIndex: segmentIndex,
		Role:         role,
		StartMs:      startMs,
		EndMs:        endMs,
		Text:         text,
		Provider:     "",
		CreatedAt:    time.Now().UTC(),
		Status:       StatusPending,
	}
}

func (t *Transcription) SetConfidence(confidence float64) {
	t.Confidence = confidence
}

func (t *Transcription) SetProvider(provider string) {
	t.Provider = provider
}

func (t *Transcription) MarkProcessing() {
	t.Status = StatusProcessing
}

func (t *Transcription) MarkReady() {
	t.Status = StatusReady
}

func (t *Transcription) MarkError() {
	t.Status = StatusError
}
