# Data Model: Aftertalk Core

**Feature**: 001-aftertalk-core  
**Date**: 2026-03-04  
**Purpose**: Define data entities, relationships, and storage patterns

## Core Entities

### 1. Session

**Purpose**: Represents a conversational session with audio capture and processing

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique session identifier (callId) | UUID v4 format |
| status | Enum | Yes | Current session state | `active`, `ended`, `processing`, `completed`, `error` |
| created_at | Timestamp | Yes | Session start time | ISO 8601, timezone UTC |
| ended_at | Timestamp | No | Session end time | ISO 8601, timezone UTC, must be > created_at |
| participant_count | Integer | Yes | Number of participants | Min: 2, Max: configurable (default 2) |
| metadata | JSON | No | Additional session metadata | Max size: 1KB |

**State Transitions**:

```
[active] → [ended] → [processing] → [completed]
    ↓         ↓            ↓             ↓
  [error]  [error]      [error]       [error]
```

**Indexes**:
- Primary key: `id`
- Index: `status`, `created_at` (for querying active/completed sessions)
- Index: `created_at` (for time-based queries and cleanup)

**Storage**: SQLite table `sessions`

### 2. Participant

**Purpose**: Represents an actor in the conversation with abstract role

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique participant identifier | UUID v4 format |
| session_id | UUID | Yes | Reference to session | Foreign key to sessions.id |
| user_id | String | Yes | External user identifier | Max length: 255 chars |
| role | String | Yes | Abstract role identifier | Max length: 50 chars, lowercase, alphanumeric + underscore |
| token_jti | UUID | Yes | JWT token unique identifier | UUID v4 format, used for single-use validation |
| token_expires_at | Timestamp | Yes | Token expiration time | Must be > created_at, typically session duration + buffer |
| token_used | Boolean | Yes | Whether token has been used | Default: false, set to true on first connection |
| connected_at | Timestamp | No | When participant connected to Bot Recorder | ISO 8601, UTC |
| disconnected_at | Timestamp | No | When participant disconnected | ISO 8601, UTC, must be > connected_at |

**Relationships**:
- Many-to-one with Session (multiple participants per session)

**Constraints**:
- Unique: `(session_id, role)` - only one participant per role per session
- Unique: `token_jti` - prevent token reuse across sessions

**Indexes**:
- Primary key: `id`
- Foreign key: `session_id` references `sessions(id)` ON DELETE CASCADE
- Unique: `(session_id, role)`
- Unique: `token_jti`
- Index: `token_expires_at` (for cleanup expired tokens)

**Storage**: SQLite table `participants`

### 3. AudioStream

**Purpose**: Represents the audio stream from a participant (metadata only, no audio storage)

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique stream identifier | UUID v4 format |
| participant_id | UUID | Yes | Reference to participant | Foreign key to participants.id |
| codec | String | Yes | Audio codec | Fixed: `opus` |
| sample_rate | Integer | Yes | Audio sample rate | Fixed: 48000 (Opus default) |
| channels | Integer | Yes | Number of audio channels | Fixed: 1 (mono) |
| chunk_size_seconds | Float | Yes | Audio chunk duration | Range: 10.0 - 30.0 seconds |
| started_at | Timestamp | Yes | Stream start time | ISO 8601, UTC |
| ended_at | Timestamp | No | Stream end time | ISO 8601, UTC, must be > started_at |
| chunks_received | Integer | Yes | Number of audio chunks received | Default: 0 |
| status | Enum | Yes | Stream status | `receiving`, `ended`, `error` |

**Relationships**:
- Many-to-one with Participant (one stream per participant per session)

**Constraints**:
- Unique: `participant_id` - one stream per participant

**Indexes**:
- Primary key: `id`
- Foreign key: `participant_id` references `participants(id)` ON DELETE CASCADE
- Index: `status`, `started_at`

**Storage**: SQLite table `audio_streams`

**Note**: Actual audio data is processed in-memory and never persisted to disk/database.

### 4. Transcription

**Purpose**: Represents the text conversion of audio with timestamps and role assignment

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique transcription identifier | UUID v4 format |
| session_id | UUID | Yes | Reference to session | Foreign key to sessions.id |
| segment_index | Integer | Yes | Order of segment in conversation | Sequential, starting from 0 |
| role | String | Yes | Role of speaker | Copied from participant.role |
| start_ms | Integer | Yes | Start time relative to session start | Milliseconds since session start, >= 0 |
| end_ms | Integer | Yes | End time relative to session start | Milliseconds since session start, must be > start_ms |
| text | Text | Yes | Transcribed text content | Max length: 10KB per segment |
| confidence | Float | No | STT confidence score | Range: 0.0 - 1.0 |
| provider | String | Yes | STT provider used | e.g., `google`, `aws`, `azure` |
| created_at | Timestamp | Yes | Transcription creation time | ISO 8601, UTC |
| status | Enum | Yes | Transcription status | `pending`, `processing`, `ready`, `error` |

**Relationships**:
- Many-to-one with Session (multiple transcription segments per session)

**Constraints**:
- Unique: `(session_id, segment_index)` - prevent duplicate segments
- Append-only: No UPDATE or DELETE operations allowed (enforced by database permissions)

**Indexes**:
- Primary key: `id`
- Foreign key: `session_id` references `sessions(id)` ON DELETE CASCADE
- Unique: `(session_id, segment_index)`
- Index: `session_id`, `start_ms` (for ordering by time)
- Index: `status` (for querying pending/ready transcriptions)

**Storage**: SQLite table `transcriptions`

**Retention Policy**: Configurable retention period (default: 90 days), automatic cleanup via scheduled job.

### 5. Minutes

**Purpose**: Represents the structured summary of the conversation with citations

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique minutes identifier | UUID v4 format |
| session_id | UUID | Yes | Reference to session | Foreign key to sessions.id |
| version | Integer | Yes | Version number for edits | Starts at 1, increments on edit |
| themes | JSONB | Yes | Main themes discussed | Array of strings |
| contents_reported | JSONB | Yes | Content reported by participants | Array of objects with text and timestamp |
| professional_interventions | JSONB | Yes | Professional's interventions | Array of objects with text and timestamp |
| progress_issues | JSONB | Yes | Progress and issues identified | Object with `progress` and `issues` arrays |
| next_steps | JSONB | Yes | Next steps and action items | Array of strings |
| citations | JSONB | Yes | Timestamped citations | Array of objects `{timestamp_ms, text, role}` |
| generated_at | Timestamp | Yes | Minutes generation time | ISO 8601, UTC |
| delivered_at | Timestamp | No | Webhook delivery time | ISO 8601, UTC |
| status | Enum | Yes | Minutes status | `pending`, `ready`, `delivered`, `error` |
| provider | String | Yes | LLM provider used | e.g., `openai`, `anthropic`, `azure` |

**Relationships**:
- One-to-one with Session (one minutes document per session)

**Constraints**:
- Unique: `session_id` - one minutes per session
- Version tracking: Previous versions stored in `minutes_history` table

**Indexes**:
- Primary key: `id`
- Foreign key: `session_id` references `sessions(id)` ON DELETE CASCADE
- Unique: `session_id`
- Index: `status`, `generated_at`

**Storage**: SQLite table `minutes`

### 6. MinutesHistory

**Purpose**: Stores previous versions of minutes for audit trail

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique history entry identifier | UUID v4 format |
| minutes_id | UUID | Yes | Reference to minutes | Foreign key to minutes.id |
| version | Integer | Yes | Version number | Copied from minutes.version |
| content | JSONB | Yes | Full minutes content at this version | Complete minutes JSON |
| edited_at | Timestamp | Yes | When this version was created | ISO 8601, UTC |
| edited_by | String | No | Who made the edit (if applicable) | User identifier from application layer |

**Relationships**:
- Many-to-one with Minutes

**Indexes**:
- Primary key: `id`
- Foreign key: `minutes_id` references `minutes(id)` ON DELETE CASCADE
- Index: `minutes_id`, `version` (for retrieving version history)

**Storage**: SQLite table `minutes_history`

**Retention Policy**: Same as transcriptions (configurable, default 90 days).

### 7. WebhookEvent

**Purpose**: Tracks webhook delivery attempts and status for idempotency

**Fields**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| id | UUID | Yes | Unique event identifier | UUID v4 format |
| minutes_id | UUID | Yes | Reference to minutes | Foreign key to minutes.id |
| webhook_url | String | Yes | Target webhook URL | Valid URL format |
| payload_hash | String | Yes | SHA-256 hash of payload | For idempotency checking |
| attempt_number | Integer | Yes | Delivery attempt count | Starts at 1 |
| status | Enum | Yes | Delivery status | `pending`, `delivered`, `failed` |
| delivered_at | Timestamp | No | Successful delivery time | ISO 8601, UTC |
| error_message | Text | No | Error message if failed | Max length: 1KB |
| created_at | Timestamp | Yes | Event creation time | ISO 8601, UTC |

**Relationships**:
- Many-to-one with Minutes (multiple delivery attempts possible)

**Constraints**:
- Unique: `payload_hash` - prevent duplicate deliveries

**Indexes**:
- Primary key: `id`
- Foreign key: `minutes_id` references `minutes(id)` ON DELETE CASCADE
- Unique: `payload_hash`
- Index: `status`, `created_at` (for querying pending events)

**Storage**: SQLite table `webhook_events`

## In-Memory Cache Structures

SQLite is the primary database for all persistent data. For performance, we use in-memory Go caches for:

### 1. Session State Cache

**Structure**: In-memory map with TTL

**Purpose**: Fast access to active session state

**Fields**:
- `status`: Current session status
- `started_at`: Session start timestamp
- `participant_count`: Number of participants
- `active_participants`: Number of currently connected participants

**TTL**: Session duration + 1 hour (auto-cleanup via Go time.Timer)

### 2. Token Tracking Cache

**Structure**: In-memory map with TTL

**Purpose**: Track used JWT tokens for single-use validation

**Key**: `jti` (JWT ID)

**Value**: `session_id`

**TTL**: Same as JWT expiration time

### 3. Audio Stream Buffer

**Structure**: In-memory buffer (chan []byte)

**Purpose**: Temporary storage for audio chunks before processing

**Cleanup**: After transcription completes

### 4. Processing Queue

**Structure**: Go channel with worker pool

**Purpose**: Job queues for async processing

**Workers**: Configurable number of goroutines for parallel processing

## Data Flow

```
1. Session Creation
   Backend → Core API: Create session with participants
   Core API → SQLite: Insert session + participants
   Core API → In-memory cache: Initialize session state

2. Audio Capture
   Client → Bot Recorder: WebRTC connection with JWT
   Bot Recorder → In-memory cache: Validate token (check jti)
   Bot Recorder → In-memory buffer: Store audio chunks temporarily
   Bot Recorder → SQLite: Create audio_stream record

3. Transcription
   Bot Recorder → Processing queue: Push transcription job
   AI Pipeline ← Processing queue: Pull job
   AI Pipeline → STT Provider: Send audio
   STT Provider → AI Pipeline: Return transcription
   AI Pipeline → SQLite: Insert transcription segments
   AI Pipeline → In-memory cache: Update session state

4. Minutes Generation
   AI Pipeline → Processing queue: Push minutes job
   AI Pipeline ← Processing queue: Pull job
   AI Pipeline → LLM Provider: Send transcription + prompt
   LLM Provider → AI Pipeline: Return structured minutes
   AI Pipeline → SQLite: Insert minutes
   AI Pipeline → Backend: Webhook notification

5. Minutes Retrieval/Editing
   Professional → Backend: Request minutes
   Backend → Core API: Get minutes
   Core API → SQLite: Retrieve minutes
   Professional → Backend: Edit minutes
   Backend → Core API: Update minutes (increment version)
   Core API → SQLite: Update minutes, insert history
```

## Migration Strategy

### Initial Schema

```sql
-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,  -- UUID as string
    status TEXT NOT NULL CHECK (status IN ('active', 'ended', 'processing', 'completed', 'error')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),  -- ISO 8601 timestamp
    ended_at TEXT,
    participant_count INTEGER NOT NULL CHECK (participant_count >= 2),
    metadata TEXT  -- JSON as text
);

CREATE INDEX idx_sessions_status_created ON sessions(status, created_at);
CREATE INDEX idx_sessions_created ON sessions(created_at);

-- Participants table
CREATE TABLE participants (
    id TEXT PRIMARY KEY,  -- UUID as string
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL,
    token_jti TEXT NOT NULL UNIQUE,
    token_expires_at TEXT NOT NULL,
    token_used INTEGER NOT NULL DEFAULT 0,  -- SQLite uses INTEGER for boolean
    connected_at TEXT,
    disconnected_at TEXT,
    UNIQUE(session_id, role)
);

CREATE INDEX idx_participants_session ON participants(session_id);
CREATE INDEX idx_participants_token_expires ON participants(token_expires_at);

-- Audio streams table
CREATE TABLE audio_streams (
    id TEXT PRIMARY KEY,  -- UUID as string
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

CREATE INDEX idx_audio_streams_status ON audio_streams(status, started_at);

-- Transcriptions table
CREATE TABLE transcriptions (
    id TEXT PRIMARY KEY,  -- UUID as string
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

CREATE INDEX idx_transcriptions_session ON transcriptions(session_id, start_ms);
CREATE INDEX idx_transcriptions_status ON transcriptions(status);

-- Minutes table
CREATE TABLE minutes (
    id TEXT PRIMARY KEY,  -- UUID as string
    session_id TEXT NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    themes TEXT NOT NULL,  -- JSON array as text
    contents_reported TEXT NOT NULL,  -- JSON array as text
    professional_interventions TEXT NOT NULL,  -- JSON array as text
    progress_issues TEXT NOT NULL,  -- JSON object as text
    next_steps TEXT NOT NULL,  -- JSON array as text
    citations TEXT NOT NULL,  -- JSON array as text
    generated_at TEXT NOT NULL DEFAULT (datetime('now')),
    delivered_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'delivered', 'error')),
    provider TEXT NOT NULL
);

CREATE INDEX idx_minutes_status ON minutes(status, generated_at);

-- Minutes history table
CREATE TABLE minutes_history (
    id TEXT PRIMARY KEY,  -- UUID as string
    minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,  -- JSON as text
    edited_at TEXT NOT NULL DEFAULT (datetime('now')),
    edited_by TEXT
);

CREATE INDEX idx_minutes_history ON minutes_history(minutes_id, version);

-- Webhook events table
CREATE TABLE webhook_events (
    id TEXT PRIMARY KEY,  -- UUID as string
    minutes_id TEXT NOT NULL REFERENCES minutes(id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    payload_hash TEXT NOT NULL UNIQUE,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    delivered_at TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_webhook_status ON webhook_events(status, created_at);

-- Processing queue table (replaces Redis queues)
CREATE TABLE processing_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL CHECK (job_type IN ('transcription', 'minutes')),
    session_id TEXT NOT NULL,
    payload TEXT NOT NULL,  -- JSON as text
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_processing_queue_status ON processing_queue(status, created_at);
```

### SQLite Configuration

```sql
-- Enable foreign key constraints (must be done on each connection)
PRAGMA foreign_keys = ON;

-- Enable WAL mode for better concurrent read/write performance
PRAGMA journal_mode = WAL;

-- Set busy timeout for concurrent access
PRAGMA busy_timeout = 5000;  -- 5 seconds
```

## Backup and Retention

### Backup Strategy
- SQLite database is a single file: `aftertalk.db`
- Backup: Copy the file while database is not in active write transaction
- For online backup: Use SQLite's Online Backup API (go-sqlite3 supports this)
- Retention: Daily backups retained 30 days
- Cross-region backup replication (production only)

### Retention Policies
- Transcriptions: 90 days (configurable)
- Minutes: 90 days (configurable)
- Minutes history: Same as minutes
- Webhook events: 30 days
- Session metadata: 1 year (anonymized after 90 days)

### Cleanup Jobs
- Scheduled daily job to delete expired data
- Soft delete with audit trail
- GDPR compliance: Right to erasure via session ID deletion

### Advantages of SQLite
- **Zero dependencies**: No separate database server to install/maintain
- **Simple backup**: Just copy the `.db` file
- **Easy deployment**: Single binary + single database file
- **Development**: No Docker/services needed for local dev
- **Performance**: In-process communication, zero network latency
- **Cost**: No database server costs
