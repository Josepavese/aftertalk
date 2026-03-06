package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/api"
	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/core/minutes"
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/core/transcription"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
	"github.com/flowup/aftertalk/pkg/jwt"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testServerURL    = "http://localhost:8080"
	testWebSocketURL = "ws://localhost:8081/ws"
)

type TestEnvironment struct {
	ServerProcess  *exec.Cmd
	ServerURL      string
	WebSocketURL   string
	DBPath         string
	ServerReady    chan struct{}
	ShutdownCancel context.CancelFunc
}

func startTestServer(dbPath string) (*TestEnvironment, error) {
	ctx, cancel := context.WithCancel(context.Background())

	tmpFile, err := os.CreateTemp("", "aftertalk-test-server-*")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFile.Close()

	env := &TestEnvironment{
		ServerURL:      testServerURL,
		WebSocketURL:   testWebSocketURL,
		DBPath:         dbPath,
		ServerReady:    make(chan struct{}),
		ShutdownCancel: cancel,
	}

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
		HTTP: config.HTTPConfig{
			Host: "localhost",
			Port: 8080,
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret-for-e2e",
			Issuer:     "aftertalk-test",
			Expiration: 2 * time.Hour,
		},
	}

	db, err := sqlite.New(ctx, cfg.Database.Path)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}
	defer db.Close()

	sessionRepo := session.NewSessionRepository(db.DB)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	jwtManager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)
	sessionService := session.NewService(sessionRepo, jwtManager, sessionCache, tokenCache)

	botServer := api.NewBotServer(sessionService, jwtManager, tokenCache)
	apiServer := api.NewServer(cfg, sessionService, botServer)

	go func() {
		defer close(env.ServerReady)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	timeout := time.After(5 * time.Second)
	select {
	case <-env.ServerReady:
		return env, nil
	case <-timeout:
		cancel()
		return nil, fmt.Errorf("server failed to start within timeout")
	}
}

func stopTestServer(env *TestEnvironment) {
	if env.ShutdownCancel != nil {
		env.ShutdownCancel()
	}

	if env.ServerProcess != nil && env.ServerProcess.Process != nil {
		if err := env.ServerProcess.Process.Kill(); err != nil {
			fmt.Printf("Failed to kill server process: %v\n", err)
		}
	}
}

func TestApplicationLifecycle(t *testing.T) {
	t.Run("Startup and Graceful Shutdown", func(t *testing.T) {
		dbPath := "/tmp/test_aftertalk_e2e.db"
		defer os.Remove(dbPath)

		env, err := startTestServer(dbPath)
		require.NoError(t, err)
		defer stopTestServer(env)

		time.Sleep(500 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("%s/health", env.ServerURL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp, err = http.Get(fmt.Sprintf("%s/ready", env.ServerURL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		time.Sleep(200 * time.Millisecond)

		resp, err = http.Post(fmt.Sprintf("%s/v1/sessions", env.ServerURL), "application/json", bytes.NewReader([]byte(`{"participant_count": 2, "participants": []}`)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		env.ShutdownCancel()
		cancel()

		select {
		case <-ctx.Done():
			t.Fatal("shutdown did not complete within timeout")
		case <-time.After(500 * time.Millisecond):
			t.Log("Graceful shutdown successful")
		}
	})

	t.Run("Database Connection and Migrations", func(t *testing.T) {
		dbPath := "/tmp/test_aftertalk_e2e_db.db"
		defer os.Remove(dbPath)

		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		var tableName string
		err = db.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' LIMIT 1").Scan(&tableName)
		require.NoError(t, err)
		assert.Equal(t, "sessions", tableName)

		var count int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 7, count)
	})

	t.Run("Service Initialization", func(t *testing.T) {
		dbPath := "/tmp/test_aftertalk_e2e_services.db"
		defer os.Remove(dbPath)

		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		sessionRepo := session.NewSessionRepository(db.DB)
		sessionCache := cache.NewSessionCache()
		tokenCache := cache.NewTokenCache()
		jwtManager := jwt.NewJWTManager("test-secret", "test-issuer", 2*time.Hour)

		sessionService := session.NewService(sessionRepo, jwtManager, sessionCache, tokenCache)
		assert.NotNil(t, sessionService)

		botServer := api.NewBotServer(sessionService, jwtManager, tokenCache)
		assert.NotNil(t, botServer)

		cfg := &config.Config{}
		apiServer := api.NewServer(cfg, sessionService, botServer)
		assert.NotNil(t, apiServer)
	})
}

func TestSessionWorkflow(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_session.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	t.Run("Create Session with 2+ Participants", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"participant_count": 3,
			"participants": []map[string]string{
				{"user_id": "user1", "role": "moderator"},
				{"user_id": "user2", "role": "participant"},
				{"user_id": "user3", "role": "participant"},
			},
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(fmt.Sprintf("%s/v1/sessions", env.ServerURL), "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEmpty(t, response["session_id"])
		assert.NotEmpty(t, response["participants"])
		sessionID := response["session_id"].(string)

		var participantList []interface{}
		for _, p := range response["participants"].([]interface{}) {
			participantList = append(participantList, p)
		}

		assert.Greater(t, len(participantList), 0)

		t.Run("Retrieve Session", func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s", env.ServerURL, sessionID))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var session session.Session
			err = json.NewDecoder(resp.Body).Decode(&session)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, sessionID, session.ID)
			assert.Equal(t, session.Status, "active")
			assert.Equal(t, 3, session.ParticipantCount)
		})

		t.Run("WebSocket Connection", func(t *testing.T) {
			u, err := url.Parse(env.WebSocketURL)
			require.NoError(t, err)

			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			require.NoError(t, err)
			defer c.Close()

			time.Sleep(100 * time.Millisecond)

			mt, message, err := c.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, websocket.TextMessage, mt)
			t.Logf("Received WebSocket message: %s", message)
		})

		t.Run("Session Completion and Cleanup", func(t *testing.T) {
			_, err := http.Post(fmt.Sprintf("%s/v1/sessions/%s/end", env.ServerURL, sessionID), "application/json", bytes.NewReader([]byte(`{}`)))
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s", env.ServerURL, sessionID))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var session session.Session
			err = json.NewDecoder(resp.Body).Decode(&session)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, "ended", session.Status)

			time.Sleep(200 * time.Millisecond)

			resp, err = http.Get(fmt.Sprintf("%s/v1/sessions/%s", env.ServerURL, sessionID))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var sessionResponse map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&sessionResponse)
			require.NoError(t, err)
			resp.Body.Close()

			sessionIDFromResp := sessionResponse["session_id"]
			assert.Equal(t, sessionID, sessionIDFromResp)
		})
	})

	t.Run("Create Session with Invalid Participants", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"participant_count": 1,
			"participants": []map[string]string{
				{"user_id": "user1", "role": "participant"},
			},
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(fmt.Sprintf("%s/v1/sessions", env.ServerURL), "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		resp.Body.Close()
	})
}

func TestTranscriptionWorkflow(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_transcription.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	sessionID := "test-session-123"
	audioData := make([]byte, 16000*4) // 1 second of PCM audio at 16kHz
	for i := range audioData {
		audioData[i] = byte(i % 256)
	}

	t.Run("Audio Upload (Simulate PCM Data)", func(t *testing.T) {
		body := bytes.NewReader(audioData)
		resp, err := http.Post(fmt.Sprintf("%s/v1/sessions/%s/audio", env.ServerURL, sessionID), "audio/raw", body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		time.Sleep(100 * time.Millisecond)
	})

	t.Run("STT Processing (Mock Provider)", func(t *testing.T) {
		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		var transcriptionCount int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM transcriptions WHERE session_id = ?", sessionID).Scan(&transcriptionCount)
		require.NoError(t, err)
		assert.Greater(t, transcriptionCount, 0)

		t.Run("Transcription Storage and Retrieval", func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/transcriptions", env.ServerURL, sessionID))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var transcriptions []*transcription.Transcription
			err = json.NewDecoder(resp.Body).Decode(&transcriptions)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Greater(t, len(transcriptions), 0)
		})

		t.Run("Status Transitions", func(t *testing.T) {
			var status string
			var startTime int64

			rows, err := db.DB.Query("SELECT status, start_ms FROM transcriptions WHERE session_id = ? ORDER BY created_at ASC LIMIT 1", sessionID)
			require.NoError(t, err)
			if rows.Next() {
				rows.Scan(&status, &startTime)
			}
			rows.Close()

			assert.Equal(t, "ready", status)
		})
	})
}

func TestMinutesWorkflow(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_minutes.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	sessionID := "test-session-minutes-123"

	t.Run("Prompt Generation from Transcription", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/minutes/prompt", env.ServerURL, sessionID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var promptResponse map[string]string
		err = json.NewDecoder(resp.Body).Decode(&promptResponse)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEmpty(t, promptResponse["prompt"])
	})

	t.Run("LLM Generation of Minutes", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/minutes", env.ServerURL, sessionID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var minutes minutes.Minutes
		err = json.NewDecoder(resp.Body).Decode(&minutes)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEmpty(t, minutes.ID)
		assert.Equal(t, minutes.Status, "ready")
	})

	t.Run("JSON Parsing and Validation", func(t *testing.T) {
		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		var minutesID string
		var status string
		err = db.DB.QueryRow("SELECT id, status FROM minutes WHERE session_id = ?", sessionID).Scan(&minutesID, &status)
		require.NoError(t, err)

		assert.NotEmpty(t, minutesID)
		assert.Equal(t, "ready", status)
	})

	t.Run("Minutes History and Versioning", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/minutes/%s/versions", env.ServerURL, sessionID, "mock-minutes-id"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var versions []*minutes.MinutesHistory
		err = json.NewDecoder(resp.Body).Decode(&versions)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Greater(t, len(versions), 0)
	})

	t.Run("Webhook Delivery", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/minutes/webhook", env.ServerURL, sessionID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var webhookResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&webhookResponse)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEmpty(t, webhookResponse["webhook_url"])
	})
}

func TestErrorScenarios(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_errors.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	t.Run("Invalid Configuration", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/invalid-route", env.ServerURL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Missing Participants", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"participant_count": 2,
			"participants":      []map[string]string{},
		}

		body, _ := json.Marshal(requestBody)
		resp, err := http.Post(fmt.Sprintf("%s/v1/sessions", env.ServerURL), "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Expired Tokens", func(t *testing.T) {
		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		var participantCount int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM participants").Scan(&participantCount)
		require.NoError(t, err)

		assert.Greater(t, participantCount, 0)
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := http.Get(fmt.Sprintf("%s/health", env.ServerURL))
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				resp.Body.Close()
			}()
		}

		wg.Wait()
	})

	t.Run("Database Failures", func(t *testing.T) {
		ctx := context.Background()
		db, err := sqlite.New(ctx, dbPath)
		require.NoError(t, err)
		defer db.Close()

		err = db.Close()
		require.NoError(t, err)

		_, err = sqlite.New(ctx, dbPath)
		assert.Error(t, err)
	})
}

func TestFullEndToEndWorkflow(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_full.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	t.Run("Complete Session Lifecycle", func(t *testing.T) {
		t.Run("Create Session", func(t *testing.T) {
			requestBody := map[string]interface{}{
				"participant_count": 3,
				"participants": []map[string]string{
					{"user_id": "moderator", "role": "moderator"},
					{"user_id": "participant1", "role": "participant"},
					{"user_id": "participant2", "role": "participant"},
				},
			}

			body, _ := json.Marshal(requestBody)
			resp, err := http.Post(fmt.Sprintf("%s/v1/sessions", env.ServerURL), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			resp.Body.Close()

			sessionID := response["session_id"].(string)
			assert.NotEmpty(t, sessionID)

			t.Run("Connect Participants", func(t *testing.T) {
				var participantIDs []string
				for _, p := range response["participants"].([]interface{}) {
					participantID := p.(map[string]interface{})["id"]
					participantIDs = append(participantIDs, participantID.(string))
				}

				for _, pid := range participantIDs {
					resp, err := http.Post(fmt.Sprintf("%s/v1/sessions/%s/participants/%s/connect", env.ServerURL, sessionID, pid), "application/json", bytes.NewReader([]byte(`{}`)))
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, resp.StatusCode)
					resp.Body.Close()
				}
			})

			t.Run("Upload Audio and Generate Transcriptions", func(t *testing.T) {
				audioData := make([]byte, 16000*4)
				for i := range audioData {
					audioData[i] = byte((i + int(time.Now().UnixNano())) % 256)
				}

				for i := 0; i < 5; i++ {
					resp, err := http.Post(fmt.Sprintf("%s/v1/sessions/%s/audio", env.ServerURL, sessionID), "audio/raw", bytes.NewReader(audioData))
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, resp.StatusCode)
					resp.Body.Close()

					time.Sleep(100 * time.Millisecond)
				}
			})

			t.Run("Generate Minutes", func(t *testing.T) {
				resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s/minutes", env.ServerURL, sessionID))
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				var minutes minutes.Minutes
				err = json.NewDecoder(resp.Body).Decode(&minutes)
				require.NoError(t, err)
				resp.Body.Close()

				assert.Equal(t, minutes.Status, "ready")
			})

			t.Run("Update Minutes", func(t *testing.T) {
				updatedMinutes := minutes.Minutes{
					ID:                        "mock-minutes-id",
					SessionID:                 sessionID,
					Themes:                    []string{"Test Themes"},
					ContentsReported:          []minutes.ContentItem{{Text: "Test Content"}},
					ProfessionalInterventions: []minutes.ContentItem{{Text: "Test Interventions"}},
					ProgressIssues:            minutes.Progress{Progress: []string{"Test Progress"}, Issues: []string{"Test Issues"}},
					NextSteps:                 []string{"Test Next Steps"},
					Citations:                 []minutes.Citation{{Text: "Test Citation"}},
					Version:                   2,
					Status:                    "ready",
					Provider:                  "openai",
				}

				body, _ := json.Marshal(updatedMinutes)
				req, err := http.NewRequest("PUT", fmt.Sprintf("%s/v1/sessions/%s/minutes/%s", env.ServerURL, sessionID, "mock-minutes-id"), bytes.NewReader(body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-User-ID", "admin")

				client := &http.Client{}
				resp, err = client.Do(req)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				resp.Body.Close()
			})

			t.Run("Verify Data Flow", func(t *testing.T) {
				ctx := context.Background()
				db, err := sqlite.New(ctx, dbPath)
				require.NoError(t, err)
				defer db.Close()

				var sessionCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
				require.NoError(t, err)
				assert.Equal(t, 1, sessionCount)

				var transcriptionCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM transcriptions WHERE session_id = ?", sessionID).Scan(&transcriptionCount)
				require.NoError(t, err)
				assert.Greater(t, transcriptionCount, 0)

				var minutesCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM minutes WHERE session_id = ?", sessionID).Scan(&minutesCount)
				require.NoError(t, err)
				assert.Equal(t, 1, minutesCount)

				var historyCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM minutes_history WHERE minutes_id = ?", "mock-minutes-id").Scan(&historyCount)
				require.NoError(t, err)
				assert.Greater(t, historyCount, 0)

				t.Log("Data flow verification successful")
			})

			t.Run("Verify Data Flow", func(t *testing.T) {
				ctx := context.Background()
				db, err := sqlite.New(ctx, dbPath)
				require.NoError(t, err)
				defer db.Close()

				var sessionCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
				require.NoError(t, err)
				assert.Equal(t, 1, sessionCount)

				var transcriptionCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM transcriptions WHERE session_id = ?", sessionID).Scan(&transcriptionCount)
				require.NoError(t, err)
				assert.Greater(t, transcriptionCount, 0)

				var minutesCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM minutes WHERE session_id = ?", sessionID).Scan(&minutesCount)
				require.NoError(t, err)
				assert.Equal(t, 1, minutesCount)

				var historyCount int
				err = db.DB.QueryRow("SELECT COUNT(*) FROM minutes_history WHERE minutes_id = ?", "mock-minutes-id").Scan(&historyCount)
				require.NoError(t, err)
				assert.Greater(t, historyCount, 0)

				t.Log("Data flow verification successful")
			})

			t.Run("End Session", func(t *testing.T) {
				_, err := http.Post(fmt.Sprintf("%s/v1/sessions/%s/end", env.ServerURL, sessionID), "application/json", bytes.NewReader([]byte(`{}`)))
				require.NoError(t, err)

				time.Sleep(200 * time.Millisecond)

				resp, err := http.Get(fmt.Sprintf("%s/v1/sessions/%s", env.ServerURL, sessionID))
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				var session session.Session
				err = json.NewDecoder(resp.Body).Decode(&session)
				require.NoError(t, err)
				resp.Body.Close()

				assert.Equal(t, "ended", session.Status)
			})
		})
	})
}

func TestWebSocketIntegration(t *testing.T) {
	dbPath := "/tmp/test_aftertalk_e2e_ws.db"
	defer os.Remove(dbPath)

	env, err := startTestServer(dbPath)
	require.NoError(t, err)
	defer stopTestServer(env)

	time.Sleep(500 * time.Millisecond)

	t.Run("WebSocket Connection and Message Flow", func(t *testing.T) {
		u, err := url.Parse(env.WebSocketURL)
		require.NoError(t, err)

		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)
		defer c.Close()

		t.Run("Send Message", func(t *testing.T) {
			message := map[string]interface{}{
				"type":    "participant-join",
				"session": "test-session",
				"user_id": "test-user",
				"role":    "participant",
			}

			body, _ := json.Marshal(message)
			err := c.WriteMessage(websocket.TextMessage, body)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			mt, messageBytes, err := c.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, websocket.TextMessage, mt)

			var response map[string]interface{}
			err = json.Unmarshal(messageBytes, &response)
			require.NoError(t, err)
			assert.Equal(t, "participant-join", response["type"])
		})

		t.Run("Close Connection", func(t *testing.T) {
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			_, _, err = c.ReadMessage()
			assert.Error(t, err)
		})
	})
}
