# Installation

## Requisiti

- Linux / macOS (Windows via WSL)
- Go 1.22+ (solo per build da sorgente)
- Nessun altro requisito — SQLite è embedded, TURN è embedded opzionale

## Installer automatico

```bash
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

L'installer:
1. Rileva Go, Python, Whisper (se presenti)
2. Builda il binario in `bin/aftertalk`
3. Crea `.env` da `.env.example` se non esiste

Per vedere cosa fa prima di eseguirlo:

```bash
curl -fsSL .../install.sh | less
```

## Build manuale

```bash
git clone https://github.com/Josepavese/aftertalk
cd aftertalk
go build -o bin/aftertalk ./cmd/aftertalk
```

## Primo avvio

```bash
cp .env.example .env
# Modifica almeno: AFTERTALK_JWT_SECRET, AFTERTALK_API_KEY, AFTERTALK_LLM_PROVIDER
./bin/aftertalk
```

Al primo avvio il server:
- Crea il database SQLite in `./aftertalk.db` (path configurabile)
- Esegue le migrazioni inline (nessun file SQL separato)
- Avvia su `0.0.0.0:8080`

Verifica:
```bash
curl http://localhost:8080/v1/health
# → {"status":"ok"}
```

## Docker

```bash
docker build -t aftertalk:latest .
docker-compose up -d
```

Il `docker-compose.yml` incluso monta un volume per il database.

## Aggiornamento

```bash
git pull && go build -o bin/aftertalk ./cmd/aftertalk
# Le migrazioni vengono eseguite automaticamente all'avvio
```
