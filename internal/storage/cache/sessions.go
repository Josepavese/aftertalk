package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	errQueueClosed = errors.New("queue is closed")
	errQueueFull   = errors.New("queue is full")
)

type SessionState struct {
	StartedAt          time.Time `json:"started_at"`
	SessionID          string    `json:"session_id"`
	Status             string    `json:"status"`
	ParticipantCount   int       `json:"participant_count"`
	ActiveParticipants int       `json:"active_participants"`
	STTProfile         string    `json:"stt_profile,omitempty"`
	LLMProfile         string    `json:"llm_profile,omitempty"`
}

type SessionCache struct {
	*Cache
}

func NewSessionCache() *SessionCache {
	return &SessionCache{
		Cache: New(),
	}
}

func (sc *SessionCache) SetSession(sessionID string, state *SessionState, ttl time.Duration) {
	sc.Set(fmt.Sprintf("session:%s:state", sessionID), state, ttl)
}

func (sc *SessionCache) GetSession(sessionID string) (*SessionState, bool) {
	val, exists := sc.Get(fmt.Sprintf("session:%s:state", sessionID))
	if !exists {
		return nil, false
	}

	state, ok := val.(*SessionState)
	if !ok {
		return nil, false
	}

	return state, true
}

func (sc *SessionCache) DeleteSession(sessionID string) {
	sc.Delete(fmt.Sprintf("session:%s:state", sessionID))
}

type TokenCache struct {
	*Cache
}

func NewTokenCache() *TokenCache {
	return &TokenCache{
		Cache: New(),
	}
}

func (tc *TokenCache) SetToken(jti string, sessionID string, ttl time.Duration) {
	tc.Set(fmt.Sprintf("token:%s", jti), sessionID, ttl)
}

func (tc *TokenCache) GetToken(jti string) (string, bool) {
	val, exists := tc.Get(fmt.Sprintf("token:%s", jti))
	if !exists {
		return "", false
	}

	sessionID, ok := val.(string)
	if !ok {
		return "", false
	}

	return sessionID, true
}

func (tc *TokenCache) DeleteToken(jti string) {
	tc.Delete(fmt.Sprintf("token:%s", jti))
}

func (tc *TokenCache) UseToken(jti string, sessionID string) bool {
	if _, exists := tc.GetToken(jti); exists {
		return false
	}
	tc.SetToken(jti, sessionID, 24*time.Hour)
	return true
}

type ProcessingQueue struct {
	jobs      chan Job
	quit      chan struct{}
	closeOnce sync.Once
}

type Job struct {
	Type      string
	SessionID string
	Payload   json.RawMessage
}

func NewProcessingQueue(workers int) *ProcessingQueue {
	if workers < 0 {
		workers = 0
	}
	q := &ProcessingQueue{
		jobs: make(chan Job, workers*2),
		quit: make(chan struct{}),
	}
	return q
}

func (q *ProcessingQueue) Enqueue(job Job) error {
	select {
	case <-q.quit:
		return errQueueClosed
	default:
	}
	select {
	case q.jobs <- job:
		return nil
	default:
		return errQueueFull
	}
}

func (q *ProcessingQueue) Dequeue() (Job, bool) {
	select {
	case <-q.quit:
		return Job{}, false
	default:
	}
	select {
	case job := <-q.jobs:
		return job, true
	case <-q.quit:
		return Job{}, false
	}
}

func (q *ProcessingQueue) Close() {
	q.closeOnce.Do(func() {
		close(q.quit)
	})
}

func (q *ProcessingQueue) Size() int {
	select {
	case <-q.quit:
		return 0
	default:
		return len(q.jobs)
	}
}

type AudioBuffer struct {
	StartTime     time.Time
	ParticipantID string
	Role          string
	Data          []byte
	Frames        [][]byte
	DurationMs    int
}

type AudioBufferCache struct {
	*Cache

	mu sync.RWMutex
}

func NewAudioBufferCache() *AudioBufferCache {
	return &AudioBufferCache{
		Cache: New(),
	}
}

func (abc *AudioBufferCache) GetBuffer(sessionID, participantID string) (*AudioBuffer, bool) {
	abc.mu.RLock()
	defer abc.mu.RUnlock()

	key := fmt.Sprintf("session:%s:participant:%s:audio", sessionID, participantID)
	val, exists := abc.Get(key)
	if !exists {
		return nil, false
	}

	buffer, ok := val.(*AudioBuffer)
	if !ok {
		return nil, false
	}

	return buffer, true
}

func (abc *AudioBufferCache) SetBuffer(sessionID, participantID string, buffer *AudioBuffer, ttl time.Duration) {
	abc.mu.Lock()
	defer abc.mu.Unlock()

	key := fmt.Sprintf("session:%s:participant:%s:audio", sessionID, participantID)
	abc.Set(key, buffer, ttl)
}

func (abc *AudioBufferCache) AppendToBuffer(sessionID, participantID, role string, chunk []byte, chunkDurationMs int) (*AudioBuffer, bool) {
	abc.mu.Lock()
	defer abc.mu.Unlock()

	key := fmt.Sprintf("session:%s:participant:%s:audio", sessionID, participantID)

	var buffer *AudioBuffer
	if existing, exists := abc.Get(key); exists {
		buf, ok := existing.(*AudioBuffer)
		if !ok {
			buf = &AudioBuffer{Data: make([]byte, 0), StartTime: time.Now(), ParticipantID: participantID}
		}
		buffer = buf
	} else {
		buffer = &AudioBuffer{
			Data:          make([]byte, 0, len(chunk)*3),
			StartTime:     time.Now(),
			ParticipantID: participantID,
			Role:          role,
		}
	}

	buffer.Data = append(buffer.Data, chunk...)
	frameCopy := make([]byte, len(chunk))
	copy(frameCopy, chunk)
	buffer.Frames = append(buffer.Frames, frameCopy)
	buffer.DurationMs += chunkDurationMs

	abc.Set(key, buffer, 30*time.Minute)

	return buffer, len(buffer.Data) > 0
}

func (abc *AudioBufferCache) ClearBuffer(sessionID, participantID string) {
	abc.mu.Lock()
	defer abc.mu.Unlock()

	key := fmt.Sprintf("session:%s:participant:%s:audio", sessionID, participantID)
	abc.Delete(key)
}

func (abc *AudioBufferCache) GetAllParticipantBuffers(sessionID string) map[string]*AudioBuffer {
	abc.mu.RLock()
	defer abc.mu.RUnlock()

	result := make(map[string]*AudioBuffer)
	prefix := fmt.Sprintf("session:%s:participant:", sessionID)

	abc.Cache.cache.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}

		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			if buffer, ok := value.(*AudioBuffer); ok {
				result[buffer.ParticipantID] = buffer
			}
		}
		return true
	})

	return result
}
