# 08 — Repository Cleanup: World-Class Standards

## Objective

Transform the repo from "well-engineered but messy" to "maniacally clean reference repository".
Every file must have a reason to exist, every directory a clear boundary, every convention applied uniformly.

---

## Analysis: Identified Problems

### CRITICAL — Build artifacts & incomplete .gitignore

| File/Dir | Problem | Fix |
|---|---|---|
| `/aftertalk` (27MB binary) | Binary committed to root | Add `/aftertalk` to .gitignore |
| `coverage.out` | Coverage report committed | Add `coverage.out`, `coverage.html`, `*.out` to .gitignore |
| `.claude/` | IDE settings not ignored (untracked) | Add `.claude/` to .gitignore |

### HIGH — Directory structure and test organization

| Problem | Detail | Fix |
|---|---|---|
| AI tests in separate directory | `internal/ai/stt/tests/` and `internal/ai/llm/tests/` use a different pattern from the rest of the project | Move tests into the package directory |
| Nested orphan directories | `internal/ai/stt/internal/ai/stt/_test` and `internal/ai/llm/internal/ai/llm/_test` | Delete |
| Unused `migrations/` folder | Two `.sql` files not used (migrations are inline in `main.go`) | Delete |
| Test summary files at root | `E2E_TESTS_SUMMARY.md`, `TEST_SUMMARY.md`, `TESTING_SUMMARY.md`, `INTEGRATION_TESTS_SUMMARY.md`, `PERFORMANCE_TEST_SUMMARY.md` clutter the root | Move to `docs/` |
| Two doc folders | `doc/` (philosophy, 2 files) and `docs/` (technical, 5 files) | Merge into `docs/` |
| `run-tests.sh`, `run_performance_tests.sh` at root | Test scripts scattered in root | Move to `scripts/` |
| `dev.sh` at root | Development script in root | Move to `scripts/` |
| `aftertalk_test.yaml` at root | Test fixture in root | Move to `testdata/` or delete if unused |

### MEDIUM — Root files and conventions

| Problem | Detail | Fix |
|---|---|---|
| Redundant test names | `entity_transcription_test.go`, `repository_repository_test.go`, `service_service_test.go` | Rename to `entity_test.go`, `repository_test.go`, `service_test.go` |
| `WORKFLOW.md` and `DEVELOPMENT_PROTOCOL.md` at root | Non-standard process documentation in root | Merge into `CONTRIBUTING.md` at root (standard) |
| `AGENTS.md` at root | AI agent documentation in root | Move to `docs/` or `.agent/` |
| Empty `opencode.json` | Empty, unused config file | Delete |
| Makefile with non-existent path `./e2e/run_tests.sh` | Causes error on `make test-e2e` | Fix target or remove reference |

### LOW — Missing project standards

| Missing file | Reason |
|---|---|
| `LICENSE` | README mentions MIT but no LICENSE file present |
| `CHANGELOG.md` | Best practice for tracking versions and breaking changes |
| `CONTRIBUTING.md` | Replaces/absorbs WORKFLOW.md and DEVELOPMENT_PROTOCOL.md |

---

## Execution Plan

### Step 1: Fix .gitignore
Add missing entries for binary, coverage, and IDE files.

### Step 2: Remove orphan files and directories
- Delete `/migrations/` (unused)
- Delete nested orphan directories in `internal/ai/`
- Delete empty `opencode.json`

### Step 3: Consolidate documentation
- Move `doc/*.md` → `docs/`
- Move `E2E_TESTS_SUMMARY.md`, `TEST_SUMMARY.md`, `TESTING_SUMMARY.md`, `INTEGRATION_TESTS_SUMMARY.md`, `PERFORMANCE_TEST_SUMMARY.md` → `docs/`
- Move `AGENTS.md` → `docs/`
- Merge `WORKFLOW.md` + `DEVELOPMENT_PROTOCOL.md` → `CONTRIBUTING.md` (root, standard GitHub)

### Step 4: Reorganize AI tests
- Move `internal/ai/stt/tests/*.go` → `internal/ai/stt/` (package `stt_test`)
- Move `internal/ai/llm/tests/*.go` → `internal/ai/llm/` (package `llm_test`)
- Delete empty `tests/` directories

### Step 5: Rename redundant test files
- `internal/core/transcription/entity_transcription_test.go` → `entity_test.go`
- `internal/core/transcription/repository_repository_test.go` → `repository_test.go`
- `internal/core/transcription/service_service_test.go` → `service_test.go`

### Step 6: Move scripts from root
- `run-tests.sh` → `scripts/run-tests.sh`
- `run_performance_tests.sh` → `scripts/run-performance-tests.sh`
- `dev.sh` → `scripts/dev.sh`
- Update references in `Makefile`

### Step 7: Add missing standard files
- Create `LICENSE` (MIT)
- Create `CHANGELOG.md` (with current version)
- Create `CONTRIBUTING.md` (merging WORKFLOW + DEVELOPMENT_PROTOCOL)

### Step 8: Fix Makefile
- Fix `test-e2e` target (non-existent path)
- Update paths for moved scripts

### Step 9: Fix aftertalk_test.yaml
- Check if used by any test
- If yes, move to `testdata/`; if no, delete

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
├── docs/                        # ALL technical documentation
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
├── improvements/                # Improvement tracking
│   ├── closed/                  # Completati
│   └── README.md
├── internal/                    # Private packages (unchanged structurally)
├── pkg/                         # Public packages
├── scripts/                     # ALL scripts
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
├── specs/                       # Project specifications
├── testdata/                    # Shared test fixtures
│   └── aftertalk_test.yaml
├── .env.example                 # Template configurazione
├── .env.test                    # Config test (tracked)
├── .env.test.clean              # Config test clean (tracked)
├── .gitignore                   # Complete
├── .golangci.yml                # Linter config
├── CHANGELOG.md                 # Version history ← NEW
├── CONTRIBUTING.md              # Development guide ← NEW (merge WORKFLOW+DEV_PROTOCOL)
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE                      # MIT ← NEW
├── Makefile                     # Fixed paths
└── README.md                    # Root documentation
```

---

## Impact

- **Root**: from 27 files → 14 files (only standard project files)
- **Documentation**: unified in `docs/`, no duplication
- **Tests**: uniform convention across the entire project
- **Scripts**: all in `scripts/`, clean Makefile
- **Artifacts**: never committed thanks to complete .gitignore
- **GitHub standards**: LICENSE, CHANGELOG, CONTRIBUTING present
