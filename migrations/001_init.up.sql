-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('active', 'ended', 'processing', 'completed', 'error')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,
    participant_count INTEGER NOT NULL CHECK (participant_count >= 2),
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_status_created ON sessions(status, created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at);

-- Participants table
CREATE TABLE IF NOT EXISTS participants (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL,
    token_jti TEXT NOT NULL UNIQUE,
    token_expires_at TEXT NOT NULL,
    token_used INTEGER NOT NULL DEFAULT 0,
    connected_at TEXT,
    disconnected_at TEXT,
    UNIQUE(session_id, role)
);

CREATE INDEX IF NOT EXISTS idx_participants_session ON participants(session_id);
CREATE INDEX IF NOT EXISTS idx_participants_token_expires ON participants(token_expires_at);

-- Audio streams table
CREATE TABLE IF NOT EXISTS audio_streams (
    id TEXT PRIMARY KEY,
    participant_id TEXT NOT NULL UNIQUE REFERENCES participants(id) ON DELETE CASCADE,
    codec TEXT NOT NULL DEFAULT 'opus',
    sample_rate INTEGER NOT NULL DEFAULT 48000,
    channels INTEGER NOT NULL DEFAULT 1,
    chunk_size_seconds REAL NOT NULL CHECK (chunk_size_seconds BETWEEN 10.0 AND 30.0),
    started_at TEXT NOT NULL,
    ended_at TEXT,
    chunks_received INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('receiving', 'ended', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_audio_streams_status ON audio_streams(status, started_at);

-- Transcriptions table
CREATE TABLE IF NOT EXISTS transcriptions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    segment_index INTEGER NOT NULL,
    role TEXT NOT NULL,
    start_ms INTEGER NOT NULL CHECK (start_ms >= 0),
    end_ms INTEGER NOT NULL CHECK (end_ms > start_ms),
    text TEXT NOT NULL,
    confidence REAL CHECK (confidence BETWEEN 0.0 AND 1.0),
    provider TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'ready', 'error')),
    UNIQUE(session_id, segment_index)
);

CREATE INDEX IF NOT EXISTS idx_transcriptions_session ON transcriptions(session_id, start_ms);
CREATE INDEX IF NOT EXISTS idx_transcriptions_status ON transcriptions(status);

-- Minutes table
CREATE TABLE IF NOT EXISTS minutes (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    themes TEXT NOT NULL,
    contents_reported TEXT NOT NULL,
    professional_interventions TEXT NOT NULL,
    progress_issues TEXT NOT NULL,
    next_steps TEXT NOT NULL,
    citations TEXT NOT NULL,
    generated_at TEXT NOT NULL DEFAULT (datetime('now')),
    delivered_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'delivered', 'error')),
    provider TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_minutes_status ON minutes(status, generated_at);

-- Minutes history table
CREATE TABLE IF NOT EXISTS minutes_history (
    id TEXT PRIMARY KEY,
    minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    edited_at TEXT NOT NULL DEFAULT (datetime('now')),
    edited_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_minutes_history ON minutes_history(minutes_id, version);

-- Webhook events table
CREATE TABLE IF NOT EXISTS webhook_events (
    id TEXT PRIMARY KEY,
    minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    payload_hash TEXT NOT NULL UNIQUE,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    delivered_at TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_webhook_status ON webhook_events(status, created_at);

-- Processing queue table
CREATE TABLE IF NOT EXISTS processing_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL CHECK (job_type IN ('transcription', 'minutes')),
    session_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_processing_queue_status ON processing_queue(status, created_at);
