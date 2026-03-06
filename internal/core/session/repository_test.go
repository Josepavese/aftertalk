package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			ended_at TEXT,
			participant_count INTEGER NOT NULL,
			metadata TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create sessions table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE participants (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			token_jti TEXT NOT NULL,
			token_expires_at TEXT NOT NULL,
			token_used INTEGER NOT NULL DEFAULT 0,
			connected_at TEXT,
			disconnected_at TEXT,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create participants table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE audio_streams (
			id TEXT PRIMARY KEY,
			participant_id TEXT NOT NULL,
			codec TEXT NOT NULL,
			sample_rate INTEGER NOT NULL,
			channels INTEGER NOT NULL,
			chunk_size_seconds REAL NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			chunks_received INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create audio_streams table: %v", err)
	}

	return db
}

func TestSessionRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	session := NewSession("test-session", 2)
	session.CreatedAt = now

	err := repo.Create(ctx, session)

	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO sessions (id, status, created_at, participant_count) VALUES (?, ?, ?, ?)`,
		"test-session-2", StatusActive, now.Format(time.RFC3339), 3)
	if err != nil {
		t.Fatalf("failed to create second session: %v", err)
	}

	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name: "Create session successfully",
			session: &Session{
				ID:               "session-1",
				Status:           StatusActive,
				CreatedAt:        now,
				ParticipantCount: 2,
			},
			wantErr: false,
		},
		{
			name: "Create session with ended_at",
			session: &Session{
				ID:               "session-2",
				Status:           StatusEnded,
				CreatedAt:        now,
				EndedAt:          &now,
				ParticipantCount: 1,
			},
			wantErr: false,
		},
		{
			name: "Create session with metadata",
			session: &Session{
				ID:               "session-3",
				Status:           StatusActive,
				CreatedAt:        now,
				ParticipantCount: 2,
				Metadata:         "test metadata",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSessionRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Now()
	testCases := []struct {
		name          string
		sessionID     string
		setupSession  bool
		expectedError bool
	}{
		{
			name:          "Get existing session",
			sessionID:     "session-1",
			setupSession:  true,
			expectedError: false,
		},
		{
			name:          "Get non-existent session",
			sessionID:     "non-existent",
			setupSession:  false,
			expectedError: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupSession {
				session := NewSession(tt.sessionID, 2)
				session.CreatedAt = now
				if err := repo.Create(ctx, session); err != nil {
					t.Fatalf("failed to setup test session: %v", err)
				}
			}

			session, err := repo.GetByID(ctx, tt.sessionID)
			if (err != nil) != tt.expectedError {
				t.Errorf("GetByID() error = %v, wantErr %v", err, tt.expectedError)
			}

			if !tt.expectedError && session == nil {
				t.Fatal("expected session, got nil")
			}

			if session != nil {
				if session.ID != tt.sessionID {
					t.Errorf("expected ID %s, got %s", tt.sessionID, session.ID)
				}

				if session.Status != StatusActive {
					t.Errorf("expected status %s, got %s", StatusActive, session.Status)
				}
			}
		})
	}
}

func TestSessionRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	session := NewSession("session-1", 2)
	session.CreatedAt = now
	session.End()
	session.StartProcessing()
	session.Complete()

	err := repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.Status = StatusEnded
	session.ParticipantCount = 3
	session.Metadata = "updated metadata"
	now = time.Now().UTC()
	session.EndedAt = &now

	err = repo.Update(ctx, session)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	updatedSession, err := repo.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() failed: %v", err)
	}

	if updatedSession.Status != StatusEnded {
		t.Errorf("expected status %s, got %s", StatusEnded, updatedSession.Status)
	}

	if updatedSession.ParticipantCount != 3 {
		t.Errorf("expected participant count 3, got %d", updatedSession.ParticipantCount)
	}

	if updatedSession.Metadata != "updated metadata" {
		t.Errorf("expected metadata 'updated metadata', got %s", updatedSession.Metadata)
	}

	if updatedSession.EndedAt == nil {
		t.Error("ended_at should be set")
	}
}

func TestSessionRepository_CreateParticipant(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	session := NewSession("session-1", 2)
	session.CreatedAt = now

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	participant := NewParticipant("participant-1", "session-1", "user-1", "moderator", "token-jti-1", now.Add(2*time.Hour))
	participant.ConnectedAt = &now

	err := repo.CreateParticipant(ctx, participant)
	if err != nil {
		t.Fatalf("CreateParticipant() failed: %v", err)
	}

	retrieved, err := repo.GetParticipantByJTI(ctx, "token-jti-1")
	if err != nil {
		t.Fatalf("GetParticipantByJTI() failed: %v", err)
	}

	if retrieved.ID != "participant-1" {
		t.Errorf("expected ID %s, got %s", "participant-1", retrieved.ID)
	}

	if retrieved.SessionID != "session-1" {
		t.Errorf("expected session_id %s, got %s", "session-1", retrieved.SessionID)
	}

	if retrieved.UserID != "user-1" {
		t.Errorf("expected user_id %s, got %s", "user-1", retrieved.UserID)
	}

	if retrieved.Role != "moderator" {
		t.Errorf("expected role %s, got %s", "moderator", retrieved.Role)
	}

	if retrieved.TokenJTI != "token-jti-1" {
		t.Errorf("expected token_jti %s, got %s", "token-jti-1", retrieved.TokenJTI)
	}

	if retrieved.TokenUsed {
		t.Error("token_used should be false")
	}

	if retrieved.ConnectedAt == nil {
		t.Error("connected_at should be set")
	}
}

func TestSessionRepository_GetParticipantByJTI(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	session := NewSession("session-1", 2)
	session.CreatedAt = now

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	participant := NewParticipant("participant-1", "session-1", "user-1", "moderator", "token-jti-1", now.Add(2*time.Hour))
	participant.ConnectedAt = &now

	if err := repo.CreateParticipant(ctx, participant); err != nil {
		t.Fatalf("failed to create participant: %v", err)
	}

	tests := []struct {
		name          string
		jti           string
		expectedError bool
	}{
		{
			name:          "Get participant by valid JTI",
			jti:           "token-jti-1",
			expectedError: false,
		},
		{
			name:          "Get participant by invalid JTI",
			jti:           "invalid-token-jti",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			participant, err := repo.GetParticipantByJTI(ctx, tt.jti)
			if (err != nil) != tt.expectedError {
				t.Errorf("GetParticipantByJTI() error = %v, wantErr %v", err, tt.expectedError)
			}

			if !tt.expectedError && participant == nil {
				t.Fatal("expected participant, got nil")
			}
		})
	}
}

