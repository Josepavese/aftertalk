package session

import (
	"testing"
	"time"
)

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value SessionStatus
	}{
		{"StatusActive", StatusActive},
		{"StatusEnded", StatusEnded},
		{"StatusProcessing", StatusProcessing},
		{"StatusCompleted", StatusCompleted},
		{"StatusError", StatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) == "" {
				t.Error("Status constant should not be empty")
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	id := "test-session-id"

	session := NewSession(id, 2, "")

	if session.ID != id {
		t.Errorf("expected ID %s, got %s", id, session.ID)
	}

	if session.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, session.Status)
	}

	if session.ParticipantCount != 2 {
		t.Errorf("expected participant count 2, got %d", session.ParticipantCount)
	}

	if session.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}

	if session.EndedAt != nil {
		t.Error("ended_at should be nil for new session")
	}

	if session.Metadata != "" {
		t.Error("metadata should be empty for new session")
	}
}

func TestSessionEnd(t *testing.T) {
	now := time.Now().UTC()
	session := NewSession("test-id", 2, "")

	session.End()

	if session.Status != StatusEnded {
		t.Errorf("expected status %s, got %s", StatusEnded, session.Status)
	}

	if session.EndedAt == nil {
		t.Error("ended_at should be set after End()")
	}

	if session.EndedAt.Before(now) || session.EndedAt.After(now.Add(time.Second)) {
		t.Errorf("ended_at should be around %v", now)
	}

	if session.ParticipantCount != 2 {
		t.Errorf("participant count should remain unchanged: got %d", session.ParticipantCount)
	}
}

func TestSessionStartProcessing(t *testing.T) {
	session := NewSession("test-id", 2, "")

	session.StartProcessing()

	if session.Status != StatusProcessing {
		t.Errorf("expected status %s, got %s", StatusProcessing, session.Status)
	}
}

func TestSessionComplete(t *testing.T) {
	session := NewSession("test-id", 2, "")

	session.Complete()

	if session.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, session.Status)
	}
}

func TestSessionFail(t *testing.T) {
	session := NewSession("test-id", 2, "")

	session.Fail()

	if session.Status != StatusError {
		t.Errorf("expected status %s, got %s", StatusError, session.Status)
	}
}

func TestSessionUpdateStatus(t *testing.T) {
	session := NewSession("test-id", 2, "")

	session.End()
	session.StartProcessing()
	session.Complete()
	session.Fail()

	if session.Status != StatusError {
		t.Errorf("expected final status %s, got %s", StatusError, session.Status)
	}
}

func TestNewParticipant(t *testing.T) {
	now := time.Now().UTC()
	tokenExpiresAt := now.Add(2 * time.Hour)
	tokenJTI := "test-token-jti"

	participant := NewParticipant("test-id", "session-id", "user-id", "moderator", tokenJTI, tokenExpiresAt)

	if participant.ID != "test-id" {
		t.Errorf("expected ID %s, got %s", "test-id", participant.ID)
	}

	if participant.SessionID != "session-id" {
		t.Errorf("expected session_id %s, got %s", "session-id", participant.SessionID)
	}

	if participant.UserID != "user-id" {
		t.Errorf("expected user_id %s, got %s", "user-id", participant.UserID)
	}

	if participant.Role != "moderator" {
		t.Errorf("expected role %s, got %s", "moderator", participant.Role)
	}

	if participant.TokenJTI != tokenJTI {
		t.Errorf("expected token_jti %s, got %s", tokenJTI, participant.TokenJTI)
	}

	if !participant.TokenExpiresAt.Equal(tokenExpiresAt) {
		t.Errorf("expected token_expires_at %v, got %v", tokenExpiresAt, participant.TokenExpiresAt)
	}

	if participant.TokenUsed {
		t.Error("token_used should be false for new participant")
	}

	if participant.ConnectedAt != nil {
		t.Error("connected_at should be nil for new participant")
	}

	if participant.DisconnectedAt != nil {
		t.Error("disconnected_at should be nil for new participant")
	}
}

func TestParticipantConnect(t *testing.T) {
	now := time.Now().UTC()
	tokenExpiresAt := now.Add(2 * time.Hour)
	participant := NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", tokenExpiresAt)

	participant.Connect()

	if participant.TokenUsed != true {
		t.Error("token_used should be true after Connect()")
	}

	if participant.ConnectedAt == nil {
		t.Error("connected_at should be set after Connect()")
	}

	if participant.ConnectedAt.Before(now) || participant.ConnectedAt.After(now.Add(time.Second)) {
		t.Errorf("connected_at should be around %v", now)
	}
}

func TestParticipantDisconnect(t *testing.T) {
	now := time.Now().UTC()
	tokenExpiresAt := now.Add(2 * time.Hour)
	participant := NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", tokenExpiresAt)
	participant.Connect()

	participant.Disconnect()

	if participant.DisconnectedAt == nil {
		t.Error("disconnected_at should be set after Disconnect()")
	}

	if participant.DisconnectedAt.Before(now) || participant.DisconnectedAt.After(now.Add(time.Second)) {
		t.Errorf("disconnected_at should be around %v", now)
	}
}

func TestParticipantIsTokenValid(t *testing.T) {
	now := time.Now().UTC()
	validTokenExpiresAt := now.Add(1 * time.Hour)
	expiredTokenExpiresAt := now.Add(-1 * time.Hour)
	usedToken := NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", validTokenExpiresAt)
	usedToken.Connect()

	tests := []struct {
		name        string
		participant *Participant
		expected    bool
	}{
		{"Valid token", NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", validTokenExpiresAt), true},
		{"Expired token", NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", expiredTokenExpiresAt), false},
		{"Used token", usedToken, false},
		{"Zero expiration", NewParticipant("test-id", "session-id", "user-id", "participant", "token-jti", time.Time{}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.participant.IsTokenValid()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNewAudioStream(t *testing.T) {
	id := "test-stream-id"

	stream := NewAudioStream(id, "participant-id", 0.5)

	if stream.ID != id {
		t.Errorf("expected ID %s, got %s", id, stream.ID)
	}

	if stream.ParticipantID != "participant-id" {
		t.Errorf("expected participant_id %s, got %s", "participant-id", stream.ParticipantID)
	}

	if stream.Codec != "opus" {
		t.Errorf("expected codec %s, got %s", "opus", stream.Codec)
	}

	if stream.SampleRate != 48000 {
		t.Errorf("expected sample_rate 48000, got %d", stream.SampleRate)
	}

	if stream.Channels != 1 {
		t.Errorf("expected channels 1, got %d", stream.Channels)
	}

	if stream.ChunkSizeSeconds != 0.5 {
		t.Errorf("expected chunk_size_seconds 0.5, got %f", stream.ChunkSizeSeconds)
	}

	if stream.StartedAt.IsZero() {
		t.Error("started_at should be set")
	}

	if stream.ChunksReceived != 0 {
		t.Errorf("expected chunks_received 0, got %d", stream.ChunksReceived)
	}

	if stream.Status != StreamStatusReceiving {
		t.Errorf("expected status %s, got %s", StreamStatusReceiving, stream.Status)
	}

	if stream.EndedAt != nil {
		t.Error("ended_at should be nil for new stream")
	}
}

func TestAudioStreamAddChunk(t *testing.T) {
	stream := NewAudioStream("test-id", "participant-id", 0.5)

	if stream.ChunksReceived != 0 {
		t.Errorf("expected chunks_received 0, got %d", stream.ChunksReceived)
	}

	stream.AddChunk()
	if stream.ChunksReceived != 1 {
		t.Errorf("expected chunks_received 1 after AddChunk(), got %d", stream.ChunksReceived)
	}

	stream.AddChunk()
	stream.AddChunk()
	if stream.ChunksReceived != 3 {
		t.Errorf("expected chunks_received 3 after 3 AddChunk() calls, got %d", stream.ChunksReceived)
	}
}

func TestAudioStreamEnd(t *testing.T) {
	now := time.Now().UTC()
	stream := NewAudioStream("test-id", "participant-id", 0.5)

	stream.End()

	if stream.Status != StreamStatusEnded {
		t.Errorf("expected status %s, got %s", StreamStatusEnded, stream.Status)
	}

	if stream.EndedAt == nil {
		t.Error("ended_at should be set after End()")
	}

	if stream.EndedAt.Before(now) || stream.EndedAt.After(now.Add(time.Second)) {
		t.Errorf("ended_at should be around %v", now)
	}
}

func TestAudioStreamFail(t *testing.T) {
	stream := NewAudioStream("test-id", "participant-id", 0.5)

	stream.Fail()

	if stream.Status != StreamStatusError {
		t.Errorf("expected status %s, got %s", StreamStatusError, stream.Status)
	}
}

func TestAudioStreamStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value AudioStreamStatus
	}{
		{"StreamStatusReceiving", StreamStatusReceiving},
		{"StreamStatusEnded", StreamStatusEnded},
		{"StreamStatusError", StreamStatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) == "" {
				t.Error("Status constant should not be empty")
			}
		})
	}
}
