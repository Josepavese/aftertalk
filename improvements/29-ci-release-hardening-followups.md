# 29 - CI/Release Hardening Follow-ups

## Context

Durante il ciclo di rilascio verso pubblico sono emersi diversi problemi in pipeline (`CI Pipeline`, `Release`) e nello smoke test container.
Questo documento raccoglie:

- problemi reali osservati
- workaround già applicati per sbloccare il rilascio
- miglioramenti da implementare per robustezza strutturale

## 1) Smoke test: SQLite path non scrivibile nel container

- Problema osservato:
  - crash in avvio container con errore SQLite (`unable to open database file`)
  - path usato nello smoke test non coerente con utente non-root runtime
- Workaround applicato:
  - immagine Docker aggiornata con directory dati dedicata e permessi espliciti (`/var/lib/aftertalk`)
  - smoke test allineato su path DB scrivibile
- Miglioramento robusto:
  - definire e documentare ufficialmente `AFTERTALK_DATABASE_PATH` per runtime container
  - aggiungere test automatico di permission/writability nel boot (fallimento con messaggio esplicito)

## 2) Smoke test: header API key incoerente

- Problema osservato:
  - request smoke su endpoint protetti con header `X-API-Key`, ma middleware valido su `Authorization: Bearer ...`
  - risultato: `401 Unauthorized`
- Workaround applicato:
  - workflow aggiornato su `Authorization: Bearer <api-key>` per health/ready/session endpoints protetti
- Miglioramento robusto:
  - scegliere uno standard unico di auth header e documentarlo in modo univoco su README/wiki/tests
  - opzionale: supportare in middleware anche `X-API-Key` solo per backward compatibility controllata

## 3) Smoke test: check `/metrics` non allineato al router

- Problema osservato:
  - step smoke bloccante su `/metrics`, endpoint non esposto nel profilo runtime corrente (`404`)
- Workaround applicato:
  - check metrics reso non bloccante (best-effort) per non interrompere release su endpoint opzionale
- Miglioramento robusto:
  - decidere se `/metrics` è parte del contratto runtime default
  - se sì: esporlo sempre e renderlo test bloccante
  - se no: rimuovere check da smoke e coprirlo in test dedicato profilo observability

## 4) CI coverage: comando non valido

- Problema osservato:
  - uso di `go tool cover -total` non valido nel flusso CI
- Workaround applicato:
  - merge coverage + report con `go tool cover -func`
- Miglioramento robusto:
  - introdurre script versionato (`scripts/ci/coverage.sh`) per evitare regressioni su comandi shell sparsi

## 5) Security job SARIF: permessi/action non allineati

- Problema osservato:
  - upload SARIF falliva per permessi insufficienti/action non aggiornata
- Workaround applicato:
  - aggiunto permesso `security-events: write`
  - aggiornato upload SARIF a `github/codeql-action/upload-sarif@v4`
- Miglioramento robusto:
  - definire baseline di permessi minimi richiesti per ogni job sicurezza
  - audit periodico delle action versionate (pinned + changelog review)

## 6) Docker build: toolchain Go non coerente

- Problema osservato:
  - builder image Go non allineata alla versione minima richiesta da `go.mod`
- Workaround applicato:
  - aggiornato builder a `golang:1.25.8-alpine`
- Miglioramento robusto:
  - centralizzare la versione Go in un solo punto (es. arg o file gestito) e riusarla in CI + Dockerfile

## 7) Dockerfile: reference a risorsa assente

- Problema osservato:
  - `COPY` di directory non presente (`migrations`) causava failure build
- Workaround applicato:
  - rimosso `COPY` obsoleto
- Miglioramento robusto:
  - aggiungere controllo pre-build che validi le `COPY` path dichiarate

## 8) Smoke container: immagine non disponibile tramite pull locale

- Problema osservato:
  - step smoke tentava `docker pull` di tag non pubblicato nel registry
- Workaround applicato:
  - passaggio ad artifact image (`docker load`) prodotto dal job build
- Miglioramento robusto:
  - standardizzare pattern build->artifact->smoke come unico path CI

## 9) Workflow condition su secrets: validazione fragile

- Problema osservato:
  - condizione job-level basata su secrets creava validazioni non affidabili
- Workaround applicato:
  - gating spostato a livello step/output
- Miglioramento robusto:
  - mantenere policy: no logica complessa su `if` di job, preferire step espliciti con output tracciabili

## 10) Warning Actions: deprecazione runtime Node 20

- Problema osservato:
  - warning GitHub Actions su passaggio a Node 24 (deadline giugno 2026)
- Workaround applicato:
  - nessun workaround tecnico immediato (non bloccante oggi)
- Miglioramento robusto:
  - backlog dedicato per aggiornare tutte le action a versioni compatibili Node 24 prima della deadline

## Priorita consigliata

1. Definizione contratto auth header unico (item #2)
2. Decisione ufficiale su `/metrics` runtime contract (item #3)
3. Centralizzazione versione Go + script coverage CI (item #4, #6)
4. Backlog aggiornamento action Node 24 (item #10)

