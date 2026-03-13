---
name: wiki-regenerate
description: Regenerate or review the Aftertalk project wiki in docs/wiki/. Use when the wiki needs updating after code changes, when accuracy of existing wiki pages needs to be verified, or when new features need to be documented. The process is engineering-rigorous: reverse-engineer the code function-by-function, save scratch notes in /tmp/wiki-notes/, write wiki pages ONLY from verified facts, create improvement files for bugs found, then clean up scratch notes. Trigger when user says "aggiorna wiki", "rigenera wiki", "review wiki", or "documenta questa feature nel wiki".
---

# Wiki Regenerate

Reverse-engineer the codebase and produce verified documentation. Never write wiki content from assumptions — only from code you have read.

## Phases

### Phase 1 — Code scan (build scratch notes)

Create `/tmp/wiki-notes/` scratch directory. For each package/file you read, save notes in `/tmp/wiki-notes/<package>.md`:
- What each exported function does (verified from code, not comments)
- Config fields with their defaults (from `Default()` functions, not docs)
- Bugs or inconsistencies noticed (save to `/tmp/wiki-notes/bugs.md`)

Read in this order:
1. `internal/config/config.go` + `loader.go` — all config fields and defaults
2. `cmd/aftertalk/main.go` — DI wiring, DB migrations, startup
3. `internal/api/server.go` + `handler/` — all routes and handlers
4. `internal/core/*/service.go` — business logic flows
5. `internal/ai/stt/` + `internal/ai/llm/` — provider contracts and implementations
6. `pkg/webhook/client.go` — webhook delivery
7. `sdk/src/` — SDK public surface

### Phase 2 — Write wiki (from scratch notes only)

Wiki lives in `docs/wiki/`. Pages to maintain:

| File | Content |
|---|---|
| `README.md` | Index + 30-second quickstart |
| `installation.md` | Installer, manual build, first run, Docker |
| `configuration.md` | Every config field with verified defaults |
| `rest-api.md` | Every endpoint with curl examples |
| `sdk.md` | SDK quickstart, WebRTC, polling |
| `webhook.md` | Push vs notify_pull, HMAC verification |
| `templates.md` | Built-in templates, custom templates, section types |
| `architecture.md` | Directory structure, flows, data model |

**Rules**:
- Include a fact only if you read the code that proves it
- For config defaults: verify from `Default()` in `config.go`, not from docs
- For endpoints: verify from handler code, not from existing docs
- For flows: trace the actual call chain

### Phase 3 — Create improvement files

For each bug found in `bugs.md`:
- Create `improvements/09-<topic>.md` (or next available number)
- Include: file path, severity, current code snippet, proposed fix
- Update `improvements/README.md` under "Open" section

### Phase 4 — Cleanup

Delete `/tmp/wiki-notes/` (scratch notes served their purpose).

### Phase 5 — Commit

```
docs(wiki): regenerate from code scan
```

List changed wiki pages and improvement files in the commit body.

---

## Partial Review

To review a single page without full regeneration:
1. Identify which packages the page covers
2. Read only those source files
3. Compare each claim in the page against the code
4. Edit in place, noting what changed

## Bug Note Template

When adding to `/tmp/wiki-notes/bugs.md`:

```markdown
## Bug: <short title>
**File**: path/to/file.go (line N approx)
**Code**: `<snippet>`
**Problem**: ...
**Fix**: ...
```
