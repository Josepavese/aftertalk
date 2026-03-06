package bot

import (
	"sync"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/stretchr/testify/assert"
)

func TestNewSessionManager(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.sessions)
	assert.Equal(t, 0, len(manager.sessions))
}

func TestSessionManager_CreateSession(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	sessionID := "test-session-id"
	activeSession := manager.CreateSession(sessionID)

	assert.NotNil(t, activeSession)
	assert.Equal(t, sessionID, activeSession.ID)
	assert.NotNil(t, activeSession.Timestamp)
	assert.NotNil(t, activeSession.AudioBuffers)
	assert.NotNil(t, activeSession.CreatedAt)
	assert.Equal(t, 0, len(activeSession.AudioBuffers))

	// Test creating same session again
	sameSession := manager.CreateSession(sessionID)
	assert.NotNil(t, sameSession)
	assert.Equal(t, activeSession.ID, sameSession.ID)
	assert.Equal(t, activeSession.Timestamp, sameSession.Timestamp)
}

func TestSessionManager_CreateSession_Multiple(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	sessions := make(map[string]*ActiveSession)

	for i := 0; i < 5; i++ {
		sessionID := "session-" + string(rune('a'+i))
		activeSession := manager.CreateSession(sessionID)
		sessions[sessionID] = activeSession
	}

	assert.Equal(t, 5, len(manager.sessions))

	for sessionID, expectedSession := range sessions {
		actualSession, exists := manager.GetSession(sessionID)
		assert.True(t, exists)
		assert.NotNil(t, actualSession)
		assert.Equal(t, expectedSession.ID, actualSession.ID)
	}
}

func TestSessionManager_CreateSession_UniqueIDs(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	sessionIDs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		sessionID := manager.CreateSession("").ID
		assert.False(t, sessionIDs[sessionID])
		sessionIDs[sessionID] = true
	}

	assert.Equal(t, 100, len(manager.sessions))
}

func TestSessionManager_GetSession(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	// Get non-existent session
	session, exists := manager.GetSession("non-existent")
	assert.False(t, exists)
	assert.Nil(t, session)

	// Create session and get it
	sessionID := "test-session"
	manager.CreateSession(sessionID)
	session, exists = manager.GetSession(sessionID)
	assert.True(t, exists)
	assert.NotNil(t, session)
	assert.Equal(t, sessionID, session.ID)
}

func TestSessionManager_GetSession_Multiple(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	for i := 0; i < 10; i++ {
		sessionID := "session-" + string(rune('a'+i))
		manager.CreateSession(sessionID)
	}

	for i := 0; i < 10; i++ {
		sessionID := "session-" + string(rune('a'+i))
		session, exists := manager.GetSession(sessionID)
		assert.True(t, exists)
		assert.NotNil(t, session)
		assert.Equal(t, sessionID, session.ID)
	}
}

func TestSessionManager_GetSession_NotExists(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	_, exists := manager.GetSession("non-existent")
	assert.False(t, exists)
}

func TestSessionManager_EndSession(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	sessionID := "test-session"
	manager.CreateSession(sessionID)

	err := manager.EndSession(testing.Background(), sessionID)
	assert.NoError(t, err)

	// Verify session is removed
	_, exists := manager.GetSession(sessionID)
	assert.False(t, exists)
}

func TestSessionManager_EndSession_NotExists(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	err := manager.EndSession(testing.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionManager_EndSession_Multiple(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	sessionIDs := []string{"session-1", "session-2", "session-3"}

	for _, sessionID := range sessionIDs {
		manager.CreateSession(sessionID)
	}

	for _, sessionID := range sessionIDs {
		err := manager.EndSession(testing.Background(), sessionID)
		assert.NoError(t, err)

		_, exists := manager.GetSession(sessionID)
		assert.False(t, exists)
	}
}

func TestActiveSession_AddAudioChunk(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add audio chunks
	data := []byte("test audio data")
	activeSession.AddAudioChunk("participant-1", data, 100)
	activeSession.AddAudioChunk("participant-2", data, 200)
	activeSession.AddAudioChunk("participant-1", data, 300)

	// Verify audio buffers
	buffers := activeSession.GetAudioBuffers()
	assert.Equal(t, 2, len(buffers))

	participant1Data, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 2, len(participant1Data))
	assert.Equal(t, participant1Data[0].Data, data)
	assert.Equal(t, participant1Data[1].Data, data)

	participant2Data, exists := buffers["participant-2"]
	assert.True(t, exists)
	assert.Equal(t, 1, len(participant2Data))
	assert.Equal(t, participant2Data[0].Data, data)
}

func TestActiveSession_AddAudioChunk_EmptyData(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add audio chunk with empty data
	activeSession.AddAudioChunk("participant-1", []byte{}, 100)
	activeSession.AddAudioChunk("participant-1", []byte{}, 200)

	buffers := activeSession.GetAudioBuffers()
	participant1Data, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 2, len(participant1Data))
	assert.Equal(t, len(participant1Data[0].Data), 0)
	assert.Equal(t, len(participant1Data[1].Data), 0)
}

func TestActiveSession_AddAudioChunk_SmallDuration(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add audio chunk with very small duration
	activeSession.AddAudioChunk("participant-1", []byte("data"), 1)
	activeSession.AddAudioChunk("participant-1", []byte("data"), 2)
	activeSession.AddAudioChunk("participant-1", []byte("data"), 3)

	buffers := activeSession.GetAudioBuffers()
	participant1Data, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 3, len(participant1Data))
}

func TestActiveSession_GetAudioBuffers(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add some audio chunks
	for i := 0; i < 5; i++ {
		data := []byte("chunk-" + string(rune('a'+i)))
		activeSession.AddAudioChunk("participant-1", data, int64(i*100))
	}

	buffers := activeSession.GetAudioBuffers()
	assert.Equal(t, 1, len(buffers))

	participantData, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 5, len(participantData))

	for i := 0; i < 5; i++ {
		assert.Equal(t, []byte("chunk-"+string(rune('a'+i))), participantData[i].Data)
		assert.Equal(t, int64(i*100), participantData[i].Timestamp)
	}
}

func TestActiveSession_GetAudioBuffers_Empty(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	buffers := activeSession.GetAudioBuffers()
	assert.Equal(t, 0, len(buffers))
}

func TestActiveSession_GetAudioBuffers_MultipleParticipants(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add audio chunks from multiple participants
	for i := 0; i < 3; i++ {
		data := []byte("participant-1-" + string(rune('a'+i)))
		activeSession.AddAudioChunk("participant-1", data, int64(i*100))
	}

	for i := 0; i < 3; i++ {
		data := []byte("participant-2-" + string(rune('a'+i)))
		activeSession.AddAudioChunk("participant-2", data, int64(i*100))
	}

	buffers := activeSession.GetAudioBuffers()
	assert.Equal(t, 2, len(buffers))

	participant1Data, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 3, len(participant1Data))

	participant2Data, exists := buffers["participant-2"]
	assert.True(t, exists)
	assert.Equal(t, 3, len(participant2Data))
}

func TestActiveSession_GetAudioBuffers_NonExistentParticipant(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	buffers := activeSession.GetAudioBuffers()
	_, exists := buffers["non-existent-participant"]
	assert.False(t, exists)
}

func TestActiveSession_MultipleAdditions(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add chunks multiple times
	for i := 0; i < 10; i++ {
		data := []byte("data-" + string(rune('a'+i)))
		activeSession.AddAudioChunk("participant-1", data, int64(i*100))
	}

	buffers := activeSession.GetAudioBuffers()
	participant1Data, exists := buffers["participant-1"]
	assert.True(t, exists)
	assert.Equal(t, 10, len(participant1Data))
}

func TestActiveSession_GetAudioBuffers_AfterEnd(t *testing.T) {
	sessionID := "test-session"
	manager := NewSessionManager(nil)
	activeSession := manager.CreateSession(sessionID)

	// Add some chunks
	activeSession.AddAudioChunk("participant-1", []byte("data1"), 100)
	activeSession.AddAudioChunk("participant-1", []byte("data2"), 200)

	// Get buffers before end
	buffers := activeSession.GetAudioBuffers()
	assert.Equal(t, 1, len(buffers))
	assert.Equal(t, 2, len(buffers["participant-1"]))
}

func TestTimestamp_GetMonotonicTime(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(50 * time.Millisecond)

	t1 := timestamp.GetMonotonicTime()
	assert.True(t, t1 > 0)
	assert.True(t, t1 >= 50)
}

func TestTimestamp_GetMonotonicTime_Increments(t *testing.T) {
	timestamp := NewTimestamp()

	t1 := timestamp.GetMonotonicTime()
	time.Sleep(10 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()
	t3 := timestamp.GetMonotonicTime()

	assert.Equal(t, t1, t2)
	assert.Equal(t, t2, t3)
	assert.True(t, t2 > t1)
}

func TestTimestamp_GetMonotonicTime_MultipleTimestamps(t *testing.T) {
	timestamp1 := NewTimestamp()
	timestamp2 := NewTimestamp()

	time.Sleep(100 * time.Millisecond)

	t1 := timestamp1.GetMonotonicTime()
	t2 := timestamp2.GetMonotonicTime()

	// Both should have the same start time
	assert.Equal(t, t1, t2)
}

func TestTimestamp_Reset(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(100 * time.Millisecond)

	t1 := timestamp.GetMonotonicTime()
	assert.True(t, t1 > 0)

	timestamp.Reset()

	time.Sleep(50 * time.Millisecond)

	t2 := timestamp.GetMonotonicTime()
	assert.Equal(t, t1, t2)
}

func TestTimestamp_Reset_MultipleTimes(t *testing.T) {
	timestamp := NewTimestamp()

	for i := 0; i < 3; i++ {
		time.Sleep(50 * time.Millisecond)
		t1 := timestamp.GetMonotonicTime()
		assert.True(t, t1 > 0)

		timestamp.Reset()
	}

	time.Sleep(50 * time.Millisecond)

	val := timestamp.GetMonotonicTime()
	assert.True(t, t > 0)
}

func TestAudioChunk_Struct(t *testing.T) {
	chunk := AudioChunk{
		ParticipantID: "participant-1",
		Timestamp:     100,
		Data:          []byte("test data"),
		Duration:      200,
	}

	assert.Equal(t, "participant-1", chunk.ParticipantID)
	assert.Equal(t, int64(100), chunk.Timestamp)
	assert.Equal(t, []byte("test data"), chunk.Data)
	assert.Equal(t, 200, chunk.Duration)
}

func TestAudioChunk_ZeroTimestamp(t *testing.T) {
	chunk := AudioChunk{
		ParticipantID: "participant-1",
		Timestamp:     0,
		Data:          []byte("test data"),
		Duration:      200,
	}

	assert.Equal(t, int64(0), chunk.Timestamp)
}

func TestAudioChunk_EmptyData(t *testing.T) {
	chunk := AudioChunk{
		ParticipantID: "participant-1",
		Timestamp:     100,
		Data:          []byte{},
		Duration:      200,
	}

	assert.Equal(t, []byte{}, chunk.Data)
}

func TestAudioChunk_ZeroDuration(t *testing.T) {
	chunk := AudioChunk{
		ParticipantID: "participant-1",
		Timestamp:     100,
		Data:          []byte("test data"),
		Duration:      0,
	}

	assert.Equal(t, 0, chunk.Duration)
}

func TestAudioChunk_MultipleChunks(t *testing.T) {
	chunks := []AudioChunk{
		{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200},
		{ParticipantID: "p2", Timestamp: 100, Data: []byte("data3"), Duration: 200},
	}

	assert.Equal(t, 3, len(chunks))
}

func TestSessionManager_ConcurrentSessions(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	sessionService := session.NewService(repo, nil, nil, nil)
	manager := NewSessionManager(sessionService)

	var wg sync.WaitGroup

	// Concurrent session creation
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := "session-" + string(rune('a'+id%26))
			manager.CreateSession(sessionID)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 26, len(manager.sessions))
}
