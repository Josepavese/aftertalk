# Aftertalk Development Workflow

## Panoramica

Protocollo di sviluppo sicuro con protezioni automatiche.

## Protezioni Automatiche

### Pre-commit Hook
Ogni commit attiva automaticamente il build di tutti i componenti:
- SDK (TypeScript)
- Test Platform (TypeScript)  
- Go Server

### Script di Build

```bash
./dev.sh              # Build + commit + push
```

## Workflow

```bash
git pull origin master
# ... modifiche ...
./dev.sh
```

## Protezioni

- File .env mai committati
- Branch: mai su master direttamente

## Comandi Utili

```bash
# Build
cd sdk && npm run build
cd test-platform && npm run build
go build -o bin/aftertalk ./cmd/aftertalk

# Server locale
AFTERTALK_JWT_SECRET="dev-secret" AFTERTALK_API_KEY="dev-api" \
AFTERTALK_DATABASE_PATH="./aftertalk.db" AFTERTALK_HTTP_PORT=8080 \
AFTERTALK_STT_PROVIDER=google AFTERTALK_LLM_PROVIDER=openai \
AFTERTALK_OPENAI_API_KEY=sk-test AFTERTALK_WS_PORT=8081 \
./bin/aftertalk
```
