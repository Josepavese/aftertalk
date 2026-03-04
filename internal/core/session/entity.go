package session

import "time"

type SessionStatus string

const (
	StatusActive     SessionStatus = "active"
	StatusEnded      SessionStatus = "ended"
	StatusProcessing SessionStatus = "processing"
	StatusCompleted  SessionStatus = "completed"
	StatusError      SessionStatus = "error"
)

type Session struct {
	ID               string        `json:"id"`
	Status           SessionStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	EndedAt          *time.Time    `json:"ended_at,omitempty"`
	ParticipantCount int           `json:"participant_count"`
	Metadata         string        `json:"metadata,omitempty"`
}

func NewSession(id string, participantCount int) *Session {
	return &Session{
		ID:               id,
		Status:           StatusActive,
		CreatedAt:        time.Now().UTC(),
		ParticipantCount: participantCount,
	}
}

func (s *Session) End() {
	now := time.Now().UTC()
	s.EndedAt = &now
	s.Status = StatusEnded
}

func (s *Session) StartProcessing() {
	s.Status = StatusProcessing
}

func (s *Session) Complete() {
	s.Status = StatusCompleted
}

func (s *Session) Fail() {
	s.Status = StatusError
}

type Participant struct {
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	UserID         string     `json:"user_id"`
	Role           string     `json:"role"`
	TokenJTI       string     `json:"token_jti"`
	TokenExpiresAt time.Time  `json:"token_expires_at"`
	TokenUsed      bool       `json:"token_used"`
	ConnectedAt    *time.Time `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
}

func NewParticipant(id, sessionID, userID, role, tokenJTI string, tokenExpiresAt time.Time) *Participant {
	return &Participant{
		ID:             id,
		SessionID:      sessionID,
		UserID:         userID,
		Role:           role,
		TokenJTI:       tokenJTI,
		TokenExpiresAt: tokenExpiresAt,
		TokenUsed:      false,
	}
}

func (p *Participant) Connect() {
	now := time.Now().UTC()
	p.ConnectedAt = &now
	p.TokenUsed = true
}

func (p *Participant) Disconnect() {
	now := time.Now().UTC()
	p.DisconnectedAt = &now
}

func (p *Participant) IsTokenValid() bool {
	return !p.TokenUsed && time.Now().Before(p.TokenExpiresAt)
}

type AudioStreamStatus string

const (
	StreamStatusReceiving AudioStreamStatus = "receiving"
	StreamStatusEnded     AudioStreamStatus = "ended"
	StreamStatusError     AudioStreamStatus = "error"
)

type AudioStream struct {
	ID               string            `json:"id"`
	ParticipantID    string            `json:"participant_id"`
	Codec            string            `json:"codec"`
	SampleRate       int               `json:"sample_rate"`
	Channels         int               `json:"channels"`
	ChunkSizeSeconds float64           `json:"chunk_size_seconds"`
	StartedAt        time.Time         `json:"started_at"`
	EndedAt          *time.Time        `json:"ended_at,omitempty"`
	ChunksReceived   int               `json:"chunks_received"`
	Status           AudioStreamStatus `json:"status"`
}

func NewAudioStream(id, participantID string, chunkSizeSeconds float64) *AudioStream {
	return &AudioStream{
		ID:               id,
		ParticipantID:    participantID,
		Codec:            "opus",
		SampleRate:       48000,
		Channels:         1,
		ChunkSizeSeconds: chunkSizeSeconds,
		StartedAt:        time.Now().UTC(),
		ChunksReceived:   0,
		Status:           StreamStatusReceiving,
	}
}

func (a *AudioStream) AddChunk() {
	a.ChunksReceived++
}

func (a *AudioStream) End() {
	now := time.Now().UTC()
	a.EndedAt = &now
	a.Status = StreamStatusEnded
}

func (a *AudioStream) Fail() {
	a.Status = StreamStatusError
}
