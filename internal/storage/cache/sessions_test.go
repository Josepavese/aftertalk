package cache

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSessionCache(t *testing.T) {
	sc := NewSessionCache()
	assert.NotNil(t, sc)
	assert.NotNil(t, sc.Cache)
}

func TestSessionCache_SetAndGetSession(t *testing.T) {
	sc := NewSessionCache()

	state := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", state, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "session-123", retrieved.SessionID)
	assert.Equal(t, "active", retrieved.Status)
	assert.Equal(t, 5, retrieved.ParticipantCount)
	assert.Equal(t, 3, retrieved.ActiveParticipants)
}

func TestSessionCache_GetSessionNonExistent(t *testing.T) {
	sc := NewSessionCache()

	_, exists := sc.GetSession("nonexistent")
	assert.False(t, exists)
}

func TestSessionCache_SetAndGetSessionWithTTL(t *testing.T) {
	sc := NewSessionCache()

	state := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", state, 100*time.Millisecond)

	_, exists := sc.GetSession("session-123")
	assert.True(t, exists)

	time.Sleep(150 * time.Millisecond)

	_, exists = sc.GetSession("session-123")
	assert.False(t, exists)
}

func TestSessionCache_DeleteSession(t *testing.T) {
	sc := NewSessionCache()

	state := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", state, 1*time.Hour)

	_, exists := sc.GetSession("session-123")
	assert.True(t, exists)

	sc.DeleteSession("session-123")

	_, exists = sc.GetSession("session-123")
	assert.False(t, exists)
}

func TestSessionCache_MultipleSessions(t *testing.T) {
	sc := NewSessionCache()

	sessions := []*SessionState{
		{
			SessionID:          "session-1",
			Status:             "active",
			StartedAt:          time.Now(),
			ParticipantCount:   2,
			ActiveParticipants: 1,
		},
		{
			SessionID:          "session-2",
			Status:             "paused",
			StartedAt:          time.Now(),
			ParticipantCount:   4,
			ActiveParticipants: 2,
		},
		{
			SessionID:          "session-3",
			Status:             "completed",
			StartedAt:          time.Now(),
			ParticipantCount:   10,
			ActiveParticipants: 0,
		},
	}

	for _, session := range sessions {
		sc.SetSession(session.SessionID, session, 1*time.Hour)
	}

	for _, session := range sessions {
		retrieved, exists := sc.GetSession(session.SessionID)
		assert.True(t, exists)
		assert.Equal(t, session.SessionID, retrieved.SessionID)
		assert.Equal(t, session.Status, retrieved.Status)
		assert.Equal(t, session.ParticipantCount, retrieved.ParticipantCount)
		assert.Equal(t, session.ActiveParticipants, retrieved.ActiveParticipants)
	}
}

func TestSessionCache_UpdateSession(t *testing.T) {
	sc := NewSessionCache()

	session := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", session, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.Equal(t, "active", retrieved.Status)

	retrieved.Status = "paused"
	retrieved.ActiveParticipants = 2

	sc.SetSession("session-123", retrieved, 1*time.Hour)

	updated, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.Equal(t, "paused", updated.Status)
	assert.Equal(t, 2, updated.ActiveParticipants)
}

func TestSessionCache_EmptySessionID(t *testing.T) {
	sc := NewSessionCache()

	_, exists := sc.GetSession("")
	assert.True(t, exists)
}

func TestSessionCache_SessionIDAlreadyExists(t *testing.T) {
	sc := NewSessionCache()

	session := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", session, 1*time.Hour)

	sc.SetSession("session-123", session, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
}

func TestSessionCache_DeleteAndAddSameSession(t *testing.T) {
	sc := NewSessionCache()

	session := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", session, 1*time.Hour)
	sc.DeleteSession("session-123")

	sc.SetSession("session-123", session, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.Equal(t, "session-123", retrieved.SessionID)
}

func TestSessionCache_MultipleSessionsWithDifferentStates(t *testing.T) {
	sc := NewSessionCache()

	sessions := map[string]*SessionState{
		"session-1": {
			SessionID:          "session-1",
			Status:             "pending",
			StartedAt:          time.Now(),
			ParticipantCount:   0,
			ActiveParticipants: 0,
		},
		"session-2": {
			SessionID:          "session-2",
			Status:             "active",
			StartedAt:          time.Now(),
			ParticipantCount:   3,
			ActiveParticipants: 2,
		},
		"session-3": {
			SessionID:          "session-3",
			Status:             "paused",
			StartedAt:          time.Now(),
			ParticipantCount:   2,
			ActiveParticipants: 1,
		},
	}

	for id, session := range sessions {
		sc.SetSession(id, session, 1*time.Hour)
	}

	for id, session := range sessions {
		retrieved, exists := sc.GetSession(id)
		assert.True(t, exists)
		assert.Equal(t, id, retrieved.SessionID)
		assert.Equal(t, session.Status, retrieved.Status)
	}
}

func TestSessionCache_SessionStateJSONSerialization(t *testing.T) {
	sc := NewSessionCache()

	state := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          time.Now().UTC(),
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", state, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)

	jsonData, err := json.Marshal(retrieved)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var decoded SessionState
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, state.SessionID, decoded.SessionID)
	assert.Equal(t, state.Status, decoded.Status)
	assert.Equal(t, state.ParticipantCount, decoded.ParticipantCount)
	assert.Equal(t, state.ActiveParticipants, decoded.ActiveParticipants)
}

func TestSessionCache_TimezoneAwareTimes(t *testing.T) {
	sc := NewSessionCache()

	now := time.Now()
	state := &SessionState{
		SessionID:          "session-123",
		Status:             "active",
		StartedAt:          now,
		ParticipantCount:   5,
		ActiveParticipants: 3,
	}

	sc.SetSession("session-123", state, 1*time.Hour)

	retrieved, exists := sc.GetSession("session-123")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, now, retrieved.StartedAt)
}

func TestNewTokenCache(t *testing.T) {
	tc := NewTokenCache()
	assert.NotNil(t, tc)
	assert.NotNil(t, tc.Cache)
}

func TestTokenCache_SetAndGetToken(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)

	retrieved, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.Equal(t, sessionID, retrieved)
}

func TestTokenCache_GetTokenNonExistent(t *testing.T) {
	tc := NewTokenCache()

	_, exists := tc.GetToken("nonexistent-jti")
	assert.False(t, exists)
}

func TestTokenCache_SetAndGetTokenWithTTL(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 100*time.Millisecond)

	_, exists := tc.GetToken(jti)
	assert.True(t, exists)

	time.Sleep(150 * time.Millisecond)

	_, exists = tc.GetToken(jti)
	assert.False(t, exists)
}

func TestTokenCache_DeleteToken(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)

	_, exists := tc.GetToken(jti)
	assert.True(t, exists)

	tc.DeleteToken(jti)

	_, exists = tc.GetToken(jti)
	assert.False(t, exists)
}

func TestTokenCache_MultipleTokens(t *testing.T) {
	tc := NewTokenCache()

	tokens := []struct {
		jti       string
		sessionID string
	}{
		{"jti-1", "session-1"},
		{"jti-2", "session-2"},
		{"jti-3", "session-3"},
	}

	for _, token := range tokens {
		tc.SetToken(token.jti, token.sessionID, 1*time.Hour)
	}

	for _, token := range tokens {
		retrieved, exists := tc.GetToken(token.jti)
		assert.True(t, exists)
		assert.Equal(t, token.sessionID, retrieved)
	}
}

func TestTokenCache_UpdateToken(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)

	retrieved, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.Equal(t, sessionID, retrieved)

	newSessionID := "session-789"
	tc.SetToken(jti, newSessionID, 1*time.Hour)

	updated, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.Equal(t, newSessionID, updated)
}

func TestTokenCache_EmptyJTI(t *testing.T) {
	tc := NewTokenCache()

	tc.SetToken("", "session-123", 1*time.Hour)

	_, exists := tc.GetToken("")
	assert.True(t, exists)
}

func TestTokenCache_JTIAlreadyExists(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)

	tc.SetToken(jti, sessionID, 1*time.Hour)

	retrieved, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
}

func TestTokenCache_DeleteAndAddSameJTI(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)
	tc.DeleteToken(jti)

	tc.SetToken(jti, sessionID, 1*time.Hour)

	retrieved, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.Equal(t, sessionID, retrieved)
}

func TestTokenCache_MultipleTokensWithDifferentSessions(t *testing.T) {
	tc := NewTokenCache()

	jtiSessions := map[string]string{
		"jti-1": "session-1",
		"jti-2": "session-2",
		"jti-3": "session-3",
	}

	for jti, sessionID := range jtiSessions {
		tc.SetToken(jti, sessionID, 1*time.Hour)
	}

	for jti, expectedSessionID := range jtiSessions {
		retrieved, exists := tc.GetToken(jti)
		assert.True(t, exists)
		assert.Equal(t, expectedSessionID, retrieved)
	}
}

func TestTokenCache_GetJTIFromSessionCacheKey(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"
	sessionID := "session-456"

	tc.SetToken(jti, sessionID, 1*time.Hour)

	retrieved, exists := tc.GetToken(jti)
	assert.True(t, exists)
	assert.Equal(t, sessionID, retrieved)
}

func TestTokenCache_MappingOneJTIToMultipleSessions(t *testing.T) {
	tc := NewTokenCache()

	jti := "jti-123"

	tc.SetToken(jti, "session-1", 1*time.Hour)
	_, exists1 := tc.GetToken(jti)
	assert.True(t, exists1)

	tc.SetToken(jti, "session-2", 1*time.Hour)
	_, exists2 := tc.GetToken(jti)
	assert.True(t, exists2)
}

func TestTokenCache_MappingMultipleJTIsToSameSession(t *testing.T) {
	tc := NewTokenCache()

	sessionID := "session-456"

	tc.SetToken("jti-1", sessionID, 1*time.Hour)
	tc.SetToken("jti-2", sessionID, 1*time.Hour)
	tc.SetToken("jti-3", sessionID, 1*time.Hour)

	retrieved1, exists1 := tc.GetToken("jti-1")
	assert.True(t, exists1)
	assert.Equal(t, sessionID, retrieved1)

	retrieved2, exists2 := tc.GetToken("jti-2")
	assert.True(t, exists2)
	assert.Equal(t, sessionID, retrieved2)

	retrieved3, exists3 := tc.GetToken("jti-3")
	assert.True(t, exists3)
	assert.Equal(t, sessionID, retrieved3)
}

func TestTokenCache_DifferentSessionsForDifferentJTIs(t *testing.T) {
	tc := NewTokenCache()

	jti1 := "jti-123"
	session1 := "session-1"
	jti2 := "jti-456"
	session2 := "session-2"

	tc.SetToken(jti1, session1, 1*time.Hour)
	tc.SetToken(jti2, session2, 1*time.Hour)

	retrieved1, exists1 := tc.GetToken(jti1)
	assert.True(t, exists1)
	assert.Equal(t, session1, retrieved1)

	retrieved2, exists2 := tc.GetToken(jti2)
	assert.True(t, exists2)
	assert.Equal(t, session2, retrieved2)

	assert.NotEqual(t, retrieved1, retrieved2)
}

func TestTokenCache_TokenExpired(t *testing.T) {
	tc := NewTokenCache()

	tc.SetToken("jti-123", "session-456", 100*time.Millisecond)

	_, exists := tc.GetToken("jti-123")
	assert.True(t, exists)

	time.Sleep(150 * time.Millisecond)

	_, exists = tc.GetToken("jti-123")
	assert.False(t, exists)
}

func TestTokenCache_DeleteNonExistentToken(t *testing.T) {
	tc := NewTokenCache()

	tc.SetToken("jti-123", "session-456", 1*time.Hour)
	tc.DeleteToken("jti-456")

	_, exists := tc.GetToken("jti-123")
	assert.True(t, exists)
}
