package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type SessionState struct {
	SessionID          string    `json:"session_id"`
	Status             string    `json:"status"`
	StartedAt          time.Time `json:"started_at"`
	ParticipantCount   int       `json:"participant_count"`
	ActiveParticipants int       `json:"active_participants"`
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
	mu   sync.Mutex
	jobs chan Job
	quit chan struct{}
}

type Job struct {
	Type      string
	SessionID string
	Payload   json.RawMessage
}

func NewProcessingQueue(workers int) *ProcessingQueue {
	q := &ProcessingQueue{
		jobs: make(chan Job, workers*2),
		quit: make(chan struct{}),
	}
	return q
}

func (q *ProcessingQueue) Enqueue(job Job) error {
	select {
	case q.jobs <- job:
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

func (q *ProcessingQueue) Dequeue() (Job, bool) {
	select {
	case job := <-q.jobs:
		return job, true
	case <-q.quit:
		return Job{}, false
	}
}

func (q *ProcessingQueue) Close() {
	close(q.quit)
	close(q.jobs)
}

func (q *ProcessingQueue) Size() int {
	return len(q.jobs)
}
