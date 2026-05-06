# Mondopsicologi minutes stuck in processing after generation timeout

Date: 2026-05-06
Project: Mondopsicologi production

## Incident

A Premium cloud video session ended around 12:49 Europe/Rome and the
application kept showing "transcription and minutes generation in progress" for
more than three hours.

Internal Aftertalk session:

- Session ID: `77794a9f-640a-4348-a7c9-04e9b7e30554`
- Application event token: `eh3y5es82xlbr47rksbdbl`
- Template: `therapy`
- Transcriptions: present and ready, about 240 segments
- Minutes row: present but left with placeholder content

## Observed State

The Aftertalk service DB had:

- `sessions.status = processing`
- `minutes.status = pending`
- `minutes.provider = ollama`
- placeholder minutes content only

The application DB had no ready minutes payload, so the frontend kept polling a
work-in-progress state.

## Logs

At session end, Aftertalk started local generation instead of the expected cloud
Premium path:

- `Generating minutes for session ...`
- `Generating minutes batch 1/10`
- `Ollama: generating with model qwen2.5:3b`

The run then failed around batch 3/10 with a context timeout. The important
framework issue is that the same canceled context appears to be reused while
marking the session/minutes as failed, so the DB update also fails and the
session remains stuck in `processing` rather than moving to `error`.

After the Mondopsicologi integration bug was fixed, forcing cloud recovery did
select the cloud profile, but the configured cloud LLM API key on that VPS was
invalid/placeholder and returned 401. In that path Aftertalk did mark the
session as `error` correctly.

## Root Causes To Address In Aftertalk

1. Failure-state writes must not use the already-expired generation context.
   Use a fresh bounded context for `failSession` / minutes error updates so a
   generation timeout cannot leave the session stuck forever.

2. Startup or health checks should validate configured cloud profiles enough to
   catch placeholder or invalid API keys before production usage. A configured
   profile that cannot authenticate should be visible as unhealthy.

3. The service should expose an explicit retry/regenerate operation for ended
   sessions with ready transcriptions, instead of requiring manual SQLite state
   edits and another `/end` call.

4. Add stuck-processing detection for sessions/minutes older than the configured
   timeout, with clear logs and a recoverable error state.

5. Local Mondopsicologi fallback should use `qwen3.5:2b`. The VPS has been
   updated and old `qwen2.5` models were removed, but Aftertalk defaults and
   installer examples should be reviewed by the framework team before changing
   global defaults.

6. Webhook delivery should be easier to inspect and replay when minutes are
   generated but not reflected in the integrating application.

## Immediate Production Mitigation Performed

On the Mondopsicologi VPS:

- Backed up `/opt/aftertalk/aftertalk.db` and related WAL/SHM files.
- Backed up `/opt/aftertalk/aftertalk.yaml`.
- Pulled `qwen3.5:2b` with Ollama.
- Updated `/opt/aftertalk/aftertalk.yaml` local LLM model to `qwen3.5:2b`.
- Added `processing.minutes_generation_timeout: "20m"` for local minutes
  generation.
- Restarted `aftertalk.service`.
- Removed old `qwen2.5:3b` and `qwen2.5:7b` models from the VPS.
- Re-queued the affected session locally for minutes generation.

## Requested Framework Follow-Up

Please treat this as a framework reliability issue, not only as a tenant
configuration problem. The tenant integration bug caused the wrong profile to be
selected, but Aftertalk should still never leave sessions indefinitely in
`processing` after its own generation timeout.
