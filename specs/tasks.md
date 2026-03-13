# Tasks: Aftertalk Core

**Input**: Design documents from `/specs/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md, contracts/

**Tests**: Not explicitly requested in feature specification, so test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Single Go project**: `cmd/`, `internal/`, `pkg/` at repository root
- All paths relative to repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Initialize Go module with `go mod init github.com/your-org/aftertalk`
- [ ] T002 Create directory structure per plan.md (cmd/, internal/, pkg/, migrations/)
- [ ] T003 [P] Create Makefile with build, test, run, migrate targets
- [ ] T004 [P] Create Dockerfile for single-stage Go build
- [ ] T005 [P] Create docker-compose.yml for local development
- [ ] T006 [P] Create .env.example with configuration template
- [ ] T007 [P] Create .gitignore for Go project
- [ ] T008 [P] Create README.md with project overview

**Checkpoint**: Project skeleton ready, can proceed to Phase 2

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T009 Create configuration structs in internal/config/config.go
- [ ] T010 Implement configuration loader in internal/config/loader.go (env + YAML + defaults)
- [ ] T011 Create SQLite database connection in internal/storage/sqlite/db.go (modernc.org/sqlite)
- [ ] T012 Create in-memory cache in internal/storage/cache/cache.go (session state, tokens, queues)
- [ ] T013 Create database migrations in migrations/001_init.up.sql (sessions, participants, transcriptions, minutes tables)
- [ ] T014 Create database migrations in migrations/001_init.down.sql (rollback)
- [ ] T015 [P] Implement structured logging in internal/logging/logger.go (zap)
- [ ] T016 [P] Create JWT utilities in pkg/jwt/jwt.go (validation, parsing)
- [ ] T017 [P] Create audio utilities in pkg/audio/opus.go (Opus decoding)
- [ ] T018 [P] Create audio utilities in pkg/audio/pcm.go (PCM conversion)
- [ ] T019 Create base repository interface in internal/core/repository.go
- [ ] T020 Implement graceful shutdown in cmd/aftertalk/main.go

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - WebRTC Audio Capture (Priority: P1) 🎯 MVP

**Goal**: Bot Recorder receives separate audio streams per participant with server-side timestamps

**Independent Test**: Start a WebRTC session and verify that the Bot Recorder receives separate audio streams for each participant with correct server-side timestamps

### Implementation for User Story 1

- [ ] T021 [P] [US1] Create Session entity in internal/core/session/entity.go
- [ ] T022 [P] [US1] Create Participant entity in internal/core/session/entity.go
- [ ] T023 [P] [US1] Create AudioStream entity in internal/core/session/entity.go
- [ ] T024 [US1] Implement SessionRepository in internal/core/session/repository.go
- [ ] T025 [US1] Implement SessionService in internal/core/session/service.go (business logic)
- [ ] T026 [US1] Implement WebSocket server in internal/bot/server.go (gorilla/websocket)
- [ ] T027 [US1] Implement JWT authentication in internal/bot/auth.go (token validation)
- [ ] T028 [US1] Implement Pion peer connection in internal/bot/peer.go (WebRTC server-side)
- [ ] T029 [US1] Implement audio processing in internal/bot/audio.go (Opus → PCM, chunking)
- [ ] T030 [US1] Implement server-side timestamping in internal/bot/timestamp.go (monotonic clock)
- [ ] T031 [US1] Implement session audio tracking in internal/bot/session.go (track multiple streams)
- [ ] T032 [US1] Create session creation endpoint in internal/api/handler/session.go (POST /v1/sessions)
- [ ] T033 [US1] Create session retrieval endpoint in internal/api/handler/session.go (GET /v1/sessions/{id})
- [ ] T034 [US1] Implement authentication middleware in internal/api/middleware/auth.go (API key + JWT)
- [ ] T035 [US1] Implement logging middleware in internal/api/middleware/logging.go
- [ ] T036 [US1] Implement recovery middleware in internal/api/middleware/recovery.go (panic handling)
- [ ] T037 [US1] Create HTTP server in internal/api/server.go (chi router setup)
- [ ] T038 [US1] Create health check endpoint in internal/api/handler/health.go (GET /v1/health, GET /v1/ready)
- [ ] T039 [US1] Wire up dependencies in cmd/aftertalk/main.go (DI setup)

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Automatic Transcription with Verified Roles (Priority: P2)

**Goal**: System automatically transcribes audio with verified roles and produces structured segments

**Independent Test**: Send pre-recorded audio to the Bot Recorder and verify that the produced transcription contains segments with role, timestamp, text and confidence score

### Implementation for User Story 2

- [ ] T040 [P] [US2] Create Transcription entity in internal/core/transcription/entity.go
- [ ] T041 [P] [US2] Create TranscriptionSegment entity in internal/core/transcription/entity.go
- [ ] T042 [US2] Implement TranscriptionRepository in internal/core/transcription/repository.go (append-only)
- [ ] T043 [US2] Define STTProvider interface in internal/ai/stt/provider.go
- [ ] T044 [US2] Implement Google STT client in internal/ai/stt/google.go (HTTP client + API)
- [ ] T045 [US2] Implement AWS Transcribe client in internal/ai/stt/aws.go (HTTP client + API)
- [ ] T046 [US2] Implement Azure Speech client in internal/ai/stt/azure.go (HTTP client + API)
- [ ] T047 [US2] Implement STT provider factory in internal/ai/stt/factory.go (config-based selection)
- [ ] T048 [US2] Implement TranscriptionService in internal/core/transcription/service.go (orchestration)
- [ ] T049 [US2] Implement retry logic in internal/ai/stt/retry.go (exponential backoff)
- [ ] T050 [US2] Create transcription retrieval endpoint in internal/api/handler/transcription.go (GET /v1/sessions/{id}/transcriptions)
- [ ] T051 [US2] Integrate transcription pipeline in internal/bot/session.go (trigger on session end)
- [ ] T052 [US2] Add error handling for STT failures in internal/ai/stt/errors.go (custom error types)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Structured AI Minutes Generation (Priority: P3)

**Goal**: System processes the transcription and produces structured minutes with temporal citations

**Independent Test**: Provide a complete transcription to the AI module and verify that the produced minutes contain all mandatory fields: Main themes, Reported contents, Professional interventions, Progress/issues, Next steps, Citations with timestamps

### Implementation for User Story 3

- [ ] T053 [P] [US3] Create Minutes entity in internal/core/minutes/entity.go
- [ ] T054 [P] [US3] Create MinutesHistory entity in internal/core/minutes/entity.go
- [ ] T055 [US3] Implement MinutesRepository in internal/core/minutes/repository.go
- [ ] T056 [US3] Define LLMProvider interface in internal/ai/llm/provider.go
- [ ] T057 [US3] Implement OpenAI client in internal/ai/llm/openai.go (HTTP client + JSON mode)
- [ ] T058 [US3] Implement Anthropic client in internal/ai/llm/anthropic.go (HTTP client)
- [ ] T059 [US3] Implement Azure OpenAI client in internal/ai/llm/azure.go (HTTP client)
- [ ] T060 [US3] Implement LLM provider factory in internal/ai/llm/factory.go (config-based selection)
- [ ] T061 [US3] Create prompt templates in internal/ai/llm/prompts.go (minutes generation template)
- [ ] T062 [US3] Implement MinutesService in internal/core/minutes/service.go (orchestration)
- [ ] T063 [US3] Implement AI pipeline in internal/ai/pipeline.go (STT + LLM orchestration)
- [ ] T064 [US3] Implement retry logic for LLM failures in internal/ai/llm/retry.go
- [ ] T065 [US3] Create minutes retrieval endpoint in internal/api/handler/minutes.go (GET /v1/sessions/{id}/minutes)
- [ ] T066 [US3] Create webhook client in pkg/webhook/client.go (POST to backend)
- [ ] T067 [US3] Integrate webhook notification in internal/core/minutes/service.go (on completion)
- [ ] T068 [US3] Add error handling for LLM failures in internal/ai/llm/errors.go

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: User Story 4 - Professional Review and Editing of Minutes (Priority: P4)

**Goal**: The professional can view the minutes, consult timestamps and edit the text

**Independent Test**: Provide complete minutes to the professional's interface and verify that they can view them, click timestamps, edit the text and save changes

### Implementation for User Story 4

- [ ] T069 [US4] Implement minutes update endpoint in internal/api/handler/minutes.go (PUT /v1/sessions/{id}/minutes)
- [ ] T070 [US4] Implement version tracking in internal/core/minutes/service.go (increment version on edit)
- [ ] T071 [US4] Create minutes history retrieval endpoint in internal/api/handler/minutes.go (GET /v1/sessions/{id}/minutes/versions)
- [ ] T072 [US4] Implement minutes history repository in internal/core/minutes/repository.go (save history)
- [ ] T073 [US4] Add input validation in internal/api/handler/minutes.go (validate minutes content)

**Checkpoint**: All user stories complete with full functionality

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T074 [P] Add Prometheus metrics in internal/metrics/metrics.go (sessions, transcriptions, minutes counters)
- [ ] T075 [P] Add pprof endpoints for profiling in cmd/aftertalk/main.go (debug endpoints)
- [ ] T076 [P] Create JSON response helpers in internal/api/response/json.go
- [ ] T077 [P] Add CORS middleware in internal/api/middleware/cors.go
- [ ] T078 [P] Implement context propagation throughout request lifecycle
- [ ] T079 Add comprehensive error handling with proper HTTP status codes
- [ ] T080 Implement request ID tracking for distributed tracing
- [ ] T081 Add database connection health checks in internal/api/handler/health.go
- [ ] T082 [P] Create Kubernetes deployment manifests in infra/kubernetes/deployment.yaml
- [ ] T083 [P] Create Kubernetes service manifests in infra/kubernetes/service.yaml
- [ ] T084 [P] Create Kubernetes HPA manifest in infra/kubernetes/hpa.yaml
- [ ] T085 Add structured error responses with error codes
- [ ] T086 Implement rate limiting middleware in internal/api/middleware/ratelimit.go
- [ ] T087 Add configuration validation at startup
- [ ] T088 Create development documentation in docs/development.md
- [ ] T089 Create deployment documentation in docs/deployment.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3 → P4)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) — Uses Session entities from US1 but independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) — Uses Transcription entities from US2 but independently testable
- **User Story 4 (P4)**: Can start after Foundational (Phase 2) — Uses Minutes entities from US3 but independently testable

### Within Each User Story

- Entities before services
- Services before endpoints
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- Entities within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all entities for User Story 1 together:
Task: "Create Session entity in internal/core/session/entity.go"
Task: "Create Participant entity in internal/core/session/entity.go"
Task: "Create AudioStream entity in internal/core/session/entity.go"

# Then sequentially:
Task: "Implement SessionRepository in internal/core/session/repository.go"
Task: "Implement SessionService in internal/core/session/service.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently (WebRTC audio capture working)
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP! WebRTC audio capture)
3. Add User Story 2 → Test independently → Deploy/Demo (Audio transcription)
4. Add User Story 3 → Test independently → Deploy/Demo (Minutes generation)
5. Add User Story 4 → Test independently → Deploy/Demo (Professional UI)
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (WebRTC Audio Capture)
   - Developer B: User Story 2 (Transcription) — can mock audio input
   - Developer C: User Story 3 (Minutes Generation) — can mock transcription
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
- Tests are not included as they were not explicitly requested in the feature specification
