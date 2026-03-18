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
	CreatedAt        time.Time     `json:"created_at"`
	EndedAt          *time.Time    `json:"ended_at,omitempty"`
	ID               string        `json:"id"`
	Status           SessionStatus `json:"status"`
	TemplateID       string        `json:"template_id,omitempty"`
	Metadata         string        `json:"metadata,omitempty"`
	ParticipantCount int           `json:"participant_count"`
	// STTProfile and LLMProfile name the provider profiles to use for this session.
	// Empty string means "use the registry default".
	STTProfile string `json:"stt_profile,omitempty"`
	LLMProfile string `json:"llm_profile,omitempty"`
}

func NewSession(id string, participantCount int, templateID, sttProfile, llmProfile string) *Session {
	return &Session{
		ID:               id,
		Status:           StatusActive,
		CreatedAt:        time.Now().UTC(),
		ParticipantCount: participantCount,
		TemplateID:       templateID,
		STTProfile:       sttProfile,
		LLMProfile:       llmProfile,
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
	TokenExpiresAt time.Time  `json:"token_expires_at"`
	ConnectedAt    *time.Time `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	UserID         string     `json:"user_id"`
	Role           string     `json:"role"`
	TokenJTI       string     `json:"token_jti"`
	TokenUsed      bool       `json:"token_used"`
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
	StartedAt        time.Time         `json:"started_at"`
	EndedAt          *time.Time        `json:"ended_at,omitempty"`
	ID               string            `json:"id"`
	ParticipantID    string            `json:"participant_id"`
	Codec            string            `json:"codec"`
	Status           AudioStreamStatus `json:"status"`
	SampleRate       int               `json:"sample_rate"`
	Channels         int               `json:"channels"`
	ChunkSizeSeconds float64           `json:"chunk_size_seconds"`
	ChunksReceived   int               `json:"chunks_received"`
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
