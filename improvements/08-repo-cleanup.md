# 08 — Repository Cleanup: World-Class Standards

## Obiettivo

Trasformare il repo da "ben ingegnerizzato ma disordinato" a "repo di riferimento maniacalmente ordinato".
Ogni file deve avere una ragione d'essere, ogni directory un confine chiaro, ogni convenzione applicata uniformemente.

---

## Analisi: Problemi Identificati

### CRITICI — Build artifacts & .gitignore incompleto

| File/Dir | Problema | Fix |
|---|---|---|
| `/aftertalk` (27MB binary) | Binario committato nella root | Add `/aftertalk` to .gitignore |
| `coverage.out` | Coverage report committato | Add `coverage.out`, `coverage.html`, `*.out` to .gitignore |
| `.claude/` | IDE settings non ignorati (untracked) | Add `.claude/` to .gitignore |

### ALTI — Struttura directory e organizzazione test

| Problema | Dettaglio | Fix |
|---|---|---|
| Test AI in directory separata | `internal/ai/stt/tests/` e `internal/ai/llm/tests/` usano pattern diverso dal resto del progetto | Spostare i test nella directory del package |
| Directory orfane annidate | `internal/ai/stt/internal/ai/stt/_test` e `internal/ai/llm/internal/ai/llm/_test` | Eliminare |
| Cartella `migrations/` inutilizzata | Due file `.sql` non usati (le migrazioni sono inline in `main.go`) | Eliminare |
| File di riepilogo test alla root | `E2E_TESTS_SUMMARY.md`, `TEST_SUMMARY.md`, `TESTING_SUMMARY.md`, `INTEGRATION_TESTS_SUMMARY.md`, `PERFORMANCE_TEST_SUMMARY.md` clutter la root | Spostare in `docs/` |
| Due cartelle doc | `doc/` (filosofia, 2 file) e `docs/` (tecnica, 5 file) | Unificare in `docs/` |
| `run-tests.sh`, `run_performance_tests.sh` alla root | Script di test sparsi nella root | Spostare in `scripts/` |
| `dev.sh` alla root | Script di sviluppo nella root | Spostare in `scripts/` |
| `aftertalk_test.yaml` alla root | Fixture di test nella root | Spostare in `testdata/` o eliminare se inutilizzata |

### MEDI — File root e convenzioni

| Problema | Dettaglio | Fix |
|---|---|---|
| Nomi test ridondanti | `entity_transcription_test.go`, `repository_repository_test.go`, `service_service_test.go` | Rinominare a `entity_test.go`, `repository_test.go`, `service_test.go` |
| `WORKFLOW.md` e `DEVELOPMENT_PROTOCOL.md` alla root | Documentazione di processo non standard nella root | Unire in `CONTRIBUTING.md` e spostare in root (standard) |
| `AGENTS.md` alla root | Documentazione agente AI nella root | Spostare in `docs/` o `.agent/` |
| `opencode.json` vuoto | File di config vuoto e inutilizzato | Eliminare |
| Makefile con path `./e2e/run_tests.sh` inesistente | Causa errore su `make test-e2e` | Fix target o rimuovere reference |

### BASSI — Standard di progetto mancanti

| File mancante | Motivo |
|---|---|
| `LICENSE` | README cita MIT ma nessun file LICENSE presente |
| `CHANGELOG.md` | Best practice per tracciare versioni e breaking changes |
| `CONTRIBUTING.md` | Sostituisce/assorbe WORKFLOW.md e DEVELOPMENT_PROTOCOL.md |

---

## Piano di Esecuzione

### Step 1: Fix .gitignore
Aggiungere entries mancanti per binary, coverage, IDE files.

### Step 2: Rimozione file e directory orfane
- Eliminare `/migrations/` (non usato)
- Eliminare directory orfane annidate in `internal/ai/`
- Eliminare `opencode.json` vuoto

### Step 3: Consolidamento documentazione
- Spostare `doc/*.md` → `docs/`
- Spostare `E2E_TESTS_SUMMARY.md`, `TEST_SUMMARY.md`, `TESTING_SUMMARY.md`, `INTEGRATION_TESTS_SUMMARY.md`, `PERFORMANCE_TEST_SUMMARY.md` → `docs/`
- Spostare `AGENTS.md` → `docs/`
- Unire `WORKFLOW.md` + `DEVELOPMENT_PROTOCOL.md` → `CONTRIBUTING.md` (root, standard GitHub)

### Step 4: Riorganizzazione test AI
- Spostare `internal/ai/stt/tests/*.go` → `internal/ai/stt/` (package `stt_test`)
- Spostare `internal/ai/llm/tests/*.go` → `internal/ai/llm/` (package `llm_test`)
- Eliminare directory `tests/` vuote

### Step 5: Rinomina test files ridondanti
- `internal/core/transcription/entity_transcription_test.go` → `entity_test.go`
- `internal/core/transcription/repository_repository_test.go` → `repository_test.go`
- `internal/core/transcription/service_service_test.go` → `service_test.go`

### Step 6: Spostamento script alla root
- `run-tests.sh` → `scripts/run-tests.sh`
- `run_performance_tests.sh` → `scripts/run-performance-tests.sh`
- `dev.sh` → `scripts/dev.sh`
- Aggiornare riferimenti in `Makefile`

### Step 7: Aggiunta file standard mancanti
- Creare `LICENSE` (MIT)
- Creare `CHANGELOG.md` (con versione corrente)
- Creare `CONTRIBUTING.md` (unendo WORKFLOW + DEVELOPMENT_PROTOCOL)

### Step 8: Fix Makefile
- Correggere `test-e2e` target (path inesistente)
- Aggiornare path per script spostati

### Step 9: Fix aftertalk_test.yaml
- Verificare se è usato da qualche test
- Se sì, spostare in `testdata/`; se no, eliminare

---

## Risultato Atteso

```
aftertalk/
├── .agent/                      # Claude Code agent skills
├── .github/workflows/ci.yml     # CI/CD
├── cmd/
│   ├── aftertalk/main.go        # Entry point
│   ├── demo/index.html          # Static demo
│   └── test-ui/                 # Test UI (TypeScript)
├── docs/                        # TUTTA la documentazione tecnica
│   ├── AGENTS.md
│   ├── DEPENDENCIES.md
│   ├── PERFORMANCE_TESTING.md
│   ├── PERFORMANCE_QUICKREF.md
│   ├── REAL_WORLD_TESTING.md
│   ├── testing.md
│   ├── filosofia_di_progetto.md
│   ├── idea.md
│   └── test-results/            # Report generati da test
│       ├── E2E_TESTS_SUMMARY.md
│       ├── INTEGRATION_TESTS_SUMMARY.md
│       ├── PERFORMANCE_TEST_SUMMARY.md
│       ├── TEST_SUMMARY.md
│       └── TESTING_SUMMARY.md
├── improvements/                # Tracking miglioramenti
│   ├── closed/                  # Completati
│   └── README.md
├── internal/                    # Private packages (unchanged structurally)
├── pkg/                         # Public packages
├── scripts/                     # TUTTI gli script
│   ├── dev.sh
│   ├── run-tests.sh
│   ├── run-performance-tests.sh
│   ├── install.sh
│   ├── install.ps1
│   ├── providers/
│   ├── steps/
│   ├── test_pipeline.py
│   └── whisper_server.py
├── sdk/                         # TypeScript SDK
├── specs/                       # Specifiche di progetto
├── testdata/                    # Fixture di test condivise
│   └── aftertalk_test.yaml
├── .env.example                 # Template configurazione
├── .env.test                    # Config test (tracked)
├── .env.test.clean              # Config test clean (tracked)
├── .gitignore                   # Completo
├── .golangci.yml                # Linter config
├── CHANGELOG.md                 # History versioni ← NEW
├── CONTRIBUTING.md              # Guida sviluppo ← NEW (merge WORKFLOW+DEV_PROTOCOL)
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE                      # MIT ← NEW
├── Makefile                     # Fixed paths
└── README.md                    # Root documentation
```

---

## Impatto

- **Root**: da 27 file → 14 file (solo file standard di progetto)
- **Documentazione**: unificata in `docs/`, nessuna duplicazione
- **Test**: convenzione uniforme in tutto il progetto
- **Script**: tutti in `scripts/`, Makefile pulito
- **Artifacts**: mai committati grazie a .gitignore completo
- **Standard GitHub**: LICENSE, CHANGELOG, CONTRIBUTING presenti
