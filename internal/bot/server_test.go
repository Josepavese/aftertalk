package bot

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)

	server := NewServer(sessionService, jwtManager, tokenCache)

	assert.NotNil(t, server)
	assert.NotNil(t, server.connections)
	assert.Equal(t, 0, len(server.connections))
}

func TestServer_HandleWebSocket(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	t.Run("ValidToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws?token="+token, nil)
		w := httptest.NewRecorder()

		server.HandleWebSocket(w, req)

		assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
	})

	t.Run("NoToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws", nil)
		w := httptest.NewRecorder()

		server.HandleWebSocket(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "token required")
	})

	t.Run("InvalidToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws?token=invalid-token", nil)
		w := httptest.NewRecorder()

		server.HandleWebSocket(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid token")
	})
}

func TestServer_HandleWebSocket_Concurrent(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/ws?token="+token, nil)
			w := httptest.NewRecorder()
			server.HandleWebSocket(w, req)
			assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
		}()
	}

	wg.Wait()
}

func TestServer_GetConnection(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	w := httptest.NewRecorder()

	server.HandleWebSocket(w, req)

	conn, exists := server.GetConnection("test-user")
	assert.True(t, exists)
	assert.NotNil(t, conn)
	assert.Equal(t, "test-user", conn.ParticipantID)
	assert.Equal(t, "test-session-id", conn.SessionID)
	assert.Equal(t, "host", conn.Role)
}

func TestServer_GetConnection_NotExists(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	conn, exists := server.GetConnection("non-existent-user")
	assert.False(t, exists)
	assert.Nil(t, conn)
}

func TestServer_Broadcast(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	w := httptest.NewRecorder()
	server.HandleWebSocket(w, req)

	message := []byte("test message")
	err = server.Broadcast("test-session-id", message)

	assert.NoError(t, err)
}

func TestServer_Broadcast_MultipleSessions(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token1, jti1, err := jwtManager.Generate("session-1", "user-1", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti1, "session-1", 2*time.Hour)

	token2, jti2, err := jwtManager.Generate("session-2", "user-2", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti2, "session-2", 2*time.Hour)

	req1 := httptest.NewRequest("GET", "/ws?token="+token1, nil)
	w1 := httptest.NewRecorder()
	server.HandleWebSocket(w1, req1)

	req2 := httptest.NewRequest("GET", "/ws?token="+token2, nil)
	w2 := httptest.NewRecorder()
	server.HandleWebSocket(w2, req2)

	err = server.Broadcast("session-1", []byte("message to session 1"))
	assert.NoError(t, err)

	err = server.Broadcast("session-2", []byte("message to session 2"))
	assert.NoError(t, err)
}

func TestServer_Broadcast_EmptySession(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	err := server.Broadcast("non-existent-session", []byte("test message"))

	assert.NoError(t, err)
}

func TestServer_Broadcast_All(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token1, jti1, err := jwtManager.Generate("test-session-id", "user-1", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti1, "test-session-id", 2*time.Hour)

	token2, jti2, err := jwtManager.Generate("test-session-id", "user-2", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti2, "test-session-id", 2*time.Hour)

	req1 := httptest.NewRequest("GET", "/ws?token="+token1, nil)
	w1 := httptest.NewRecorder()
	server.HandleWebSocket(w1, req1)

	req2 := httptest.NewRequest("GET", "/ws?token="+token2, nil)
	w2 := httptest.NewRecorder()
	server.HandleWebSocket(w2, req2)

	err = server.Broadcast("test-session-id", []byte("broadcast message"))
	assert.NoError(t, err)
}

func TestServer_Close(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	w := httptest.NewRecorder()
	server.HandleWebSocket(w, req)

	err = server.Close()
	assert.NoError(t, err)

	conn, exists := server.GetConnection("test-user")
	assert.False(t, exists)
	assert.Nil(t, conn)
}

func TestServer_Close_MultipleConnections(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token1, jti1, err := jwtManager.Generate("test-session-id", "user-1", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti1, "test-session-id", 2*time.Hour)

	token2, jti2, err := jwtManager.Generate("test-session-id", "user-2", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti2, "test-session-id", 2*time.Hour)

	req1 := httptest.NewRequest("GET", "/ws?token="+token1, nil)
	w1 := httptest.NewRecorder()
	server.HandleWebSocket(w1, req1)

	req2 := httptest.NewRequest("GET", "/ws?token="+token2, nil)
	w2 := httptest.NewRecorder()
	server.HandleWebSocket(w2, req2)

	err = server.Close()
	assert.NoError(t, err)

	conn1, exists1 := server.GetConnection("user-1")
	conn2, exists2 := server.GetConnection("user-2")

	assert.False(t, exists1)
	assert.False(t, exists2)
	assert.Nil(t, conn1)
	assert.Nil(t, conn2)
}

func TestServer_ReadPump(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	w := httptest.NewRecorder()

	server.HandleWebSocket(w, req)

	conn, exists := server.GetConnection("test-user")
	require.True(t, exists)
	require.NotNil(t, conn)

	time.Sleep(100 * time.Millisecond)

	assert.NoError(t, server.Close())
}

func TestServer_WritePump(t *testing.T) {
	repo := session.NewSessionRepository(nil)
	jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)
	tokenCache := cache.NewTokenCache()
	sessionService := session.NewService(repo, jwtManager, cache.NewSessionCache(), tokenCache)
	server := NewServer(sessionService, jwtManager, tokenCache)

	token, jti, err := jwtManager.Generate("test-session-id", "test-user", "host")
	require.NoError(t, err)
	tokenCache.SetToken(jti, "test-session-id", 2*time.Hour)

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	w := httptest.NewRecorder()

	server.HandleWebSocket(w, req)

	time.Sleep(100 * time.Millisecond)

	assert.NoError(t, server.Close())
}
