# Technical Specification

## AI Module for Automatic Session Minutes

Document for **architects and developers**. Language is intentionally operational and prescriptive.

---

## 1. Purpose

Implement a module that:

* intercepts **only the audio** of WebRTC sessions,
* generates a **textual transcription with verified roles (professional/patient)**,
* produces a **structured AI minutes** at the end of the session,
* makes the minutes **viewable and editable** by the professional.

The system **does NOT**:

* record or permanently store audio,
* perform automatic diagnoses,
* expose transcriptions or minutes to the patient.

---

## 2. Architectural constraints

* PeerJS used **exclusively for signaling**
* WebRTC media **P2P between professional and patient**
* Dedicated third peer (Bot Recorder) for audio reception
* Application backend = source of truth for roles and sessions
* All AI operations are **post-session (batch)**

---

## 3. Actors and services

### 3.1 Professional client

* Web browser
* P2P WebRTC audio/video
* WebRTC audio toward Bot Recorder

### 3.2 Patient client

* Web browser
* P2P WebRTC audio/video
* WebRTC audio toward Bot Recorder

### 3.3 Application backend

* User and role management
* Session creation (`callId`)
* Signed token issuance
* Session end signaling

### 3.4 Bot Recorder / Transcriber

* Server-side WebRTC peer
* Audio reception
* Server-side timestamping
* STT integration
* Temporary transcription persistence

---

## 4. Identity, roles and security

### 4.1 Session token

The backend issues a JWT for each participant:

```
{
  callId: string,
  userId: string,
  role: "doctor" | "patient",
  exp: timestamp,
  jti: uuid
}
```

Rules:

* one stream per `(callId, role)`
* token verified by the Bot Recorder
* expired or reused tokens → connection rejected

---

## 5. Session flow (technical sequence)

1. Backend creates `callId`
2. Backend issues signed tokens
3. Client starts P2P call (PeerJS)
4. Client opens WebRTC audio toward Bot Recorder
5. Bot validates token
6. Bot receives audio and assigns timestamps
7. Audio processed in chunks
8. At session end, backend sends `SESSION_END`
9. Bot finalizes transcription
10. Bot invokes AI minutes pipeline

---

## 6. Audio management

### 6.1 Audio characteristics

* Codec: Opus (input)
* Internal conversion: PCM mono 16kHz
* Chunking: 10–30 seconds

### 6.2 Timestamps

* Server-side monotonic clock
* Timestamps relative to session start
* No client-side timestamps

---

## 7. Transcription (STT)

### 7.1 Mode

* Batch STT (post-processing)
* Configurable cloud provider

### 7.2 STT output

```
{
  callId,
  role,
  start_ms,
  end_ms,
  text,
  confidence
}
```

Persistence:

* append-only
* no in-place modification

---

## 8. AI Minutes (LLM)

### 8.1 Pre-processing

* Sort by timestamp
* Transcription chunking (2–5 min)

### 8.2 Prompting

The prompt must:

* prohibit diagnoses
* prohibit non-explicit inferences
* require structured output
* include citations with timestamps

### 8.3 Minutes output

Mandatory structure:

* Main themes
* Patient-reported contents
* Therapist interventions
* Progress / issues
* Next steps
* Citations (with timestamps)

---

## 9. Backend delivery

* Minutes are sent via application webhook
* The webhook must be idempotent
* Status: `READY → DELIVERED`

---

## 10. Frontend – Professional UI

### 10.1 Session list view

* AI status: in progress / ready / error

### 10.2 Minutes view

* Editable text
* Clickable timestamps
* Manual save

---

## 11. Error handling

* STT failure → retry
* LLM failure → retry
* Timeout → `ERROR` status

---

## 12. Retention and cleanup

* Audio: **never persisted**
* Transcription: configurable retention
* Logs: without sensitive content

---

## 13. Non-goals

* Automatic emotional tone analysis
* Clinical diagnosis
* Patient access to the minutes

---

## 14. Definition of Done

* Correct transcription with roles
* Minutes generated and visible
* Editing working
* No persistent audio
* Security audit passed
