package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/logging"
)

type SessionManager struct {
	sessions       map[string]*ActiveSession
	sessionService *session.Service
	mu             sync.RWMutex
}

type ActiveSession struct {
	ID           string
	Timestamp    *Timestamp
	AudioBuffers map[string]*AudioBuffer
	CreatedAt    time.Time
	mu           sync.RWMutex
}

func NewSessionManager(sessionService *session.Service) *SessionManager {
	return &SessionManager{
		sessions:       make(map[string]*ActiveSession),
		sessionService: sessionService,
	}
}

func (m *SessionManager) CreateSession(sessionID string) *ActiveSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, exists := m.sessions[sessionID]; exists {
		return s
	}

	activeSession := &ActiveSession{
		ID:           sessionID,
		Timestamp:    NewTimestamp(),
		AudioBuffers: make(map[string]*AudioBuffer),
		CreatedAt:    time.Now(),
	}

	m.sessions[sessionID] = activeSession
	logging.Infof("Active session created: %s", sessionID)

	return activeSession
}

func (m *SessionManager) GetSession(sessionID string) (*ActiveSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.sessions[sessionID]
	return s, exists
}

func (m *SessionManager) EndSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(m.sessions, sessionID)
	logging.Infof("Active session ended: %s", sessionID)

	return m.sessionService.EndSession(ctx, sessionID)
}

func (s *ActiveSession) AddAudioChunk(participantID string, data []byte, durationMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.AudioBuffers[participantID]; !exists {
		s.AudioBuffers[participantID] = NewAudioBuffer(1000)
	}

	chunk := AudioChunk{
		ParticipantID: participantID,
		Timestamp:     s.Timestamp.GetMonotonicTime(),
		Data:          data,
		Duration:      durationMs,
	}

	s.AudioBuffers[participantID].Add(chunk)
}

func (s *ActiveSession) GetAudioBuffers() map[string][]AudioChunk {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]AudioChunk)
	for participantID, buffer := range s.AudioBuffers {
		result[participantID] = buffer.GetAll()
	}

	return result
}
