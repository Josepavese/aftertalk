# Contributing to Aftertalk

## Development Setup

```bash
# Clone and install SDK dependencies
git clone https://github.com/Josepavese/aftertalk
cd aftertalk
cd sdk && npm install && cd ..
```

## Build

```bash
# Go server
go build -o bin/aftertalk ./cmd/aftertalk

# TypeScript SDK
cd sdk && npm run build

# All via Makefile
make build
```

## Development Workflow

```bash
git pull origin master
# ... make changes ...
scripts/dev.sh    # Build + commit + push
```

### Running the server locally

```bash
AFTERTALK_JWT_SECRET="dev-secret" \
AFTERTALK_API_KEY="dev-api" \
AFTERTALK_DATABASE_PATH="./aftertalk.db" \
AFTERTALK_HTTP_PORT=8080 \
AFTERTALK_STT_PROVIDER=stub \
AFTERTALK_LLM_PROVIDER=stub \
./bin/aftertalk
```

Or copy `.env.example` to `.env` and run `make dev`.

## Testing

```bash
make test                  # All tests
make test-unit             # Unit tests only
make test-integration      # Integration tests
make test-coverage         # Coverage report

# SDK tests
cd sdk && npm test
```

## Code Standards

### Go
- `gofmt` + `golangci-lint` before committing (enforced by CI)
- Every exported symbol must have a Go doc comment
- Tests co-located with source (`{file}_test.go`)

### TypeScript
- ESLint + Prettier via `cd sdk && npm run lint`
- Tests co-located with source (`{file}.test.ts`)

## Documentation Style

- Follow [docs/style/tone-of-voice.md](docs/style/tone-of-voice.md) for README/docs copy.

## Architecture Principles

- **PAL** (Platform Abstraction Layer): business logic consumes only interfaces; providers implement them. See `.agent/skills/platform-abstraction-layer/`
- **SSOT** (Single Source of Truth): one canonical location per piece of data. See `.agent/skills/single-source-of-truth/`

## Commit Conventions

Semantic commits:

```
feat(scope): description
fix(scope): description
docs(scope): description
test(scope): description
refactor(scope): description
chore(scope): description
```

## Release Versioning

- Follow [./.github/release/VERSIONING_SSOT.md](.github/release/VERSIONING_SSOT.md) as the canonical release/version source of truth.

## Rules

- Never commit directly to `master` — use `feature/*` or `fix/*` branches
- Never commit `.env` files (`.gitignore` enforces this)
- Build must pass before committing (pre-commit hook)
- Never commit more than ~5 files without an intermediate commit
