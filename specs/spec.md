# Feature Specification: Aftertalk Core

**Feature Branch**: `001-aftertalk-core`
**Created**: 2026-03-04
**Status**: Draft
**Input**: User description: "Aftertalk Core - AI module to automatically generate end-of-session minutes from WebRTC conversations with transcription and AI summarization"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - WebRTC Audio Capture (Priority: P1)

The system intercepts the audio of an ongoing WebRTC session and routes it to a dedicated component for transcription, keeping the audio streams of different participants separate.

**Why this priority**: Audio capture is the fundamental prerequisite for any Aftertalk functionality. Without this capability, neither transcription nor minutes generation is possible. It is the foundation on which everything else is built.

**Independent Test**: Can be tested independently by starting a WebRTC session and verifying that the Bot Recorder receives separate audio streams for each participant with correct server-side timestamps.

**Acceptance Scenarios**:

1. **Given** an active WebRTC session between two participants, **When** the participants speak, **Then** the Bot Recorder receives two separate audio streams (one per participant) with role identification (professional/patient)
2. **Given** a WebRTC session with valid JWT tokens, **When** the Bot Recorder receives the streams, **Then** timestamps are assigned server-side with a monotonic clock relative to session start
3. **Given** a WebRTC session with an expired or already-used token, **When** a participant attempts to connect to the Bot Recorder, **Then** the connection is rejected

---

### User Story 2 - Automatic Transcription with Verified Roles (Priority: P2)

The system automatically transcribes the received audio into text, correctly assigning a role to each segment of conversation.

**Why this priority**: Transcription is the second fundamental step after audio capture. It is required to generate the structured minutes. Without correct transcription with roles, minutes cannot be generated.

**Independent Test**: Can be tested by sending pre-recorded audio to the Bot Recorder and verifying that the produced transcription contains segments with role, timestamp, text and confidence score.

**Acceptance Scenarios**:

1. **Given** audio streams received by the Bot Recorder, **When** the session ends, **Then** the system produces a structured transcription with segments `{callId, role, start_ms, end_ms, text, confidence}`
2. **Given** an ongoing transcription, **When** an STT error occurs, **Then** the system automatically retries before marking the status as ERROR
3. **Given** a completed transcription, **When** the backend requests it, **Then** the data is available in append-only format without in-place modification

---

### User Story 3 - Structured AI Minutes Generation (Priority: P3)

The system processes the transcription and produces structured minutes with temporal citations, main themes, professional interventions and next steps.

**Why this priority**: Minutes are the final value for the professional user. They are the product that reduces cognitive load and improves traceability. They come after capture and transcription.

**Independent Test**: Can be tested by providing a complete transcription to the AI module and verifying that the produced minutes contain all mandatory fields: Main themes, Patient-reported contents, Therapist interventions, Progress/issues, Next steps, Citations with timestamps.

**Acceptance Scenarios**:

1. **Given** a complete transcription with verified roles, **When** the system processes the transcription, **Then** it produces structured minutes with verifiable temporal citations
2. **Given** minutes being generated, **When** an LLM error occurs, **Then** the system automatically retries before marking the status as ERROR
3. **Given** generated minutes, **When** the backend requests them via webhook, **Then** delivery is idempotent with status `READY → DELIVERED`

---

### User Story 4 - Professional Review and Editing of Minutes (Priority: P4)

The professional can view the generated minutes, consult clickable timestamps, and edit the text before final save.

**Why this priority**: Human interaction with the minutes is essential for the Human-in-the-loop principle, but it is the last step after the system has captured, transcribed and generated the minutes.

**Independent Test**: Can be tested by providing complete minutes to the professional's interface and verifying that they can view them, click timestamps to review key points, edit the text and save changes.

**Acceptance Scenarios**:

1. **Given** minutes generated and delivered to the backend, **When** the professional accesses the session list, **Then** they see the minutes status: in progress / ready / error
2. **Given** ready minutes, **When** the professional opens them, **Then** they can view the full text, click timestamps to jump to key points, and edit the text
3. **Given** minutes edited by the professional, **When** they save changes, **Then** the edited version replaces the original but the version history is preserved

---

### Edge Cases

- What happens if the Bot Recorder loses connection during a session? Is the partial transcription preserved?
- How does the system handle a session with more than two participants (e.g. couples therapy, group)?
- What happens if the STT or LLM provider is temporarily unavailable?
- How is a very short session (< 1 minute) or very long session (> 2 hours) handled?
- What happens if the audio quality is poor or there is significant background noise?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST intercept audio from WebRTC sessions without interfering with P2P communication between participants
- **FR-002**: The system MUST receive separate audio streams for each participant with role identification (professional/patient)
- **FR-003**: The system MUST assign server-side timestamps with a monotonic clock relative to session start
- **FR-004**: The system MUST convert Opus audio to PCM mono 16kHz for processing
- **FR-005**: The system MUST process audio in chunks of 10-30 seconds
- **FR-006**: The system MUST transcribe audio into text using a configurable cloud STT provider
- **FR-007**: The system MUST produce transcriptions with structure `{callId, role, start_ms, end_ms, text, confidence}`
- **FR-008**: The system MUST persist transcriptions in append-only format without in-place modification
- **FR-009**: The system MUST generate structured minutes with mandatory fields: Main themes, Reported contents, Professional interventions, Progress/issues, Next steps, Citations with timestamps
- **FR-010**: The system MUST prohibit automatic diagnoses and non-explicit inferences in minutes
- **FR-011**: The system MUST deliver minutes to the backend via idempotent webhook
- **FR-012**: The system MUST allow the professional to view, consult timestamps and edit the minutes
- **FR-013**: The system MUST ensure audio is NEVER permanently stored
- **FR-014**: The system MUST verify JWT tokens for every connection to the Bot Recorder
- **FR-015**: The system MUST reject connections with expired or already-used tokens
- **FR-016**: The system MUST allow only one stream per `(callId, role)` pair

### Key Entities

- **Session**: Represents a conversational session with unique `callId`, participants with roles, start and end timestamps, status (active, ended, processing, completed, error)
- **Participant**: Represents an actor in the conversation with `userId`, abstract role (professional/patient or other configurable roles), session JWT token
- **AudioStream**: Represents a participant's audio stream with identifier, codec (Opus), sample rate, time chunks, server-side timestamps
- **Transcription**: Represents the audio-to-text conversion with structured segments `{callId, role, start_ms, end_ms, text, confidence}`, creation timestamp, status (pending, processing, ready, error)
- **Minutes**: Represents the structured conversation summary with mandatory fields (Themes, Contents, Interventions, Progress, Next steps, Citations), generation timestamp, status (pending, ready, delivered, error), version (for edits)
- **SessionToken**: Represents access credentials as a JWT containing `{callId, userId, role, exp, jti}`, status (valid, used, expired, revoked)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The system captures audio from WebRTC sessions with < 500ms latency for stream reception start
- **SC-002**: The system transcribes audio with > 85% accuracy (average confidence score) under standard audio conditions
- **SC-003**: The system generates structured minutes in < 5 minutes for 1-hour sessions
- **SC-004**: The professional can view ready minutes within 10 minutes of session end
- **SC-005**: The system handles up to 100 concurrent sessions without performance degradation
- **SC-006**: The system tracks all retry attempts for STT/LLM errors with logs (without sensitive content)
- **SC-007**: The professional can edit and save minutes in < 2 minutes
- **SC-008**: The system rejects 100% of connections with invalid, expired or already-used tokens
- **SC-009**: The system NEVER permanently stores audio (verifiable via audit)
- **SC-010**: The core remains agnostic to the application domain (verifiable via code review: no references to domain-specific terms)

## Assumptions

- PeerJS is already configured for WebRTC signaling in the application layer (MondoPsicologi)
- The application backend already exists and can issue signed JWT tokens
- The STT cloud provider supports Italian and English (configurable)
- The LLM cloud provider supports prompts in Italian and English (configurable)
- Participants have modern web browsers with WebRTC support
- Internet connection has sufficient bandwidth for bidirectional audio streams + stream toward Bot Recorder
- The professional is familiar with web interfaces for viewing and editing documents

## Out of Scope

- Automatic emotional tone analysis of voice or content
- Automatic clinical diagnoses or therapeutic suggestions
- Patient access to transcription or minutes
- Video recording of the session
- Integration with electronic health record (EHR) systems
- Voice recognition for commands during the session
- Real-time automatic translation
- Real-time sentiment analysis during the session
