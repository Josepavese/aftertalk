# Improvement 16 — Minutes Generation Robustness

## Status: COMPLETED

## Context

Devil's advocate analysis of the minutes generation pipeline revealed five bugs ranging from
data loss on restart to silent failure states invisible to operators.

---

## Bug 1 — Sessions stuck in `processing` after restart (data loss)

**Severity: Critical**

When `EndSession` is called, the session status is written to `processing` in the DB and a
fire-and-forget goroutine is launched to run `processRemainingAudio` + `generateMinutesForSession`.
If the process is restarted (deploy, crash, OOM, systemd restart) while the goroutine is running:

- The goroutine dies silently.
- The session remains `processing` forever — no minutes are ever generated.
- The webhook is never delivered.
- The session reaper (improvement #12) ignores `processing` sessions (it only sweeps `active`).
- There is no recovery mechanism at boot.

**Fix**: At startup, query for sessions with `status = 'processing'` and re-trigger
`generateMinutesForSession` for each. This is safe because `processRemainingAudio` is idempotent
(it reads from the audio buffer cache, which will be empty after restart, so only the minutes
generation step runs — which is also idempotent since `GenerateMinutes` creates a new record
only if one does not already exist for the session, or replaces the existing `error` record).

A `recoverProcessingSessions` goroutine launched at boot is sufficient.

---

## Bug 2 — Race condition between transcription queue and minutes generation

**Severity: Medium**

`generateMinutesForSession` waits for the transcription channel to drain with a busy-wait:

```go
deadline := time.Now().Add(30 * time.Second)
for len(s.transcribeCh) > 0 && time.Now().Before(deadline) {
    time.Sleep(100 * time.Millisecond)
}
```

Two issues:

1. **Channel empty ≠ processing done.** A job can be dequeued from the channel but not yet
   persisted to DB. The minutes generation reads transcriptions from DB while the consumer
   goroutine is still writing — minutes are generated with incomplete data.

2. **Arbitrary 30-second timeout.** If the STT provider is slow (Google Speech timeout is
   up to 90s), the deadline expires and minutes are generated from partial transcriptions,
   with no warning to the operator.

**Fix**: Replace the busy-wait with a `sync.WaitGroup` (`transcribeWg`) incremented on
`transcribeCh <- job` and decremented inside `processTranscriptionQueue` after the DB write.
`generateMinutesForSession` calls `transcribeWg.Wait()` instead of the polling loop.
No timeout needed: the transcription goroutine already has per-job timeouts.

---

## Bug 3 — `defer cancel()` inside a loop creates context accumulation

**Severity: Low**

In `processRemainingAudio`:

```go
for participantID, buffer := range buffers {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()   // runs at function return, not at loop iteration end
    ...
}
```

With N participants, N contexts with 5-minute timeouts are created and all deferred until the
function returns. This is a correctness issue (contexts are not cancelled promptly) and a
potential goroutine leak if a STT provider spawns goroutines tied to the context.

**Fix**: Wrap the loop body in a closure or call `cancel()` explicitly before `continue`/end
of iteration.

---

## Bug 4 — Session status never reaches `error` on failure

**Severity: High**

`generateMinutesForSession` logs errors and returns early, but the session status remains
`processing` indefinitely. `session.Complete()` is only called on success. There is no
`session.MarkError()` / `session.Fail()` call.

Consequence: operators cannot distinguish "still processing" from "permanently failed" by
looking at the DB. Alerting on `processing` sessions older than N minutes is the only workaround
— but it requires external tooling and produces false positives during normal processing.

**Fix**: On any unrecoverable error in `generateMinutesForSession`, set `session.status = 'error'`
and persist. Use the existing `StatusError` constant (already defined in `entity.go`).

---

## Bug 5 — Webhook delivery goroutine lost on restart (without retrier)

**Severity: Low (mitigated in production)**

After minutes are saved to DB (`status = ready`), the webhook is dispatched in a nested goroutine:

```go
go s.deliverWebhook(sessionID, m, sessCtx)
```

This goroutine lives inside a goroutine. If the process dies after the DB write but before
the webhook HTTP call completes:

- **With `webhookRetrier` configured** (production with `WEBHOOK_URL` set): the event is already
  enqueued in the `webhook_events` DB table before the goroutine — safe. ✓
- **Without `webhookRetrier`** (no `WEBHOOK_URL` or direct delivery mode): the webhook is lost
  with no record in DB. The minutes exist in DB but were never delivered.

This is already mitigated in production (the retrier is always configured when a URL is set).
It remains a footgun for operators who configure a URL without understanding the retrier
dependency.

**Fix**: Log a startup warning when `WEBHOOK_URL` is set but `webhookRetrier` is nil.
Optionally, refuse to start (or at least warn loudly) in this configuration.

---

## Implementation Plan

### Phase 1 — Critical (data loss)

1. **Bug 4**: Add `session.MarkError()` in `generateMinutesForSession` on failure. (1 line)
2. **Bug 1**: Add `recoverProcessingSessions()` called at startup in `main.go`.
   - Query `SELECT id FROM sessions WHERE status = 'processing'`
   - Re-launch `go generateMinutesForSession(id)` for each

### Phase 2 — Correctness

3. **Bug 2**: Replace busy-wait with `sync.WaitGroup` (`transcribeWg`) in `Service`.
   - `transcribeWg.Add(1)` before each `transcribeCh <- job`
   - `transcribeWg.Done()` inside `processTranscriptionQueue` after each DB write
   - `transcribeWg.Wait()` at the start of `generateMinutesForSession`

4. **Bug 3**: Fix `defer cancel()` in loop in `processRemainingAudio`.

### Phase 3 — Observability

5. **Bug 5**: Startup warning when webhook URL is set but retrier is nil.

---

## Files to change

- `internal/core/session/service.go` — main changes (WaitGroup, error status, reaper)
- `internal/core/session/entity.go` — verify `StatusError` exists; add `MarkError()` if not
- `cmd/aftertalk/main.go` — call `recoverProcessingSessions` at boot

## Documentation to update

- `docs/wiki/architecture.md` — update session lifecycle diagram with error status and recovery
- `docs/wiki/configuration.md` — no changes needed (no new config fields)
