# Improvement 25: Bugfix TypeScript SDK (`@aftertalk/sdk`)

## Stato: APERTO

## Contesto

Analisi devil's advocate del TS SDK (improvement 04) ha identificato bug funzionali che
causano 404 a runtime, incongruenze tra wiki e implementazione, e campi mai popolati.
I bug più gravi riguardano `MinutesAPI` e `TranscriptionsAPI` che usano endpoint
inesistenti nel server.

---

## BUG 1 — CRITICO: `MinutesAPI` usa endpoint inesistenti (3 metodi su 4)

### Problema

`sdk/ts/src/api/minutes.ts` usa path session-scoped che **non esistono nel server**:

```typescript
getBySession(sessionId)  → GET  /v1/sessions/${sessionId}/minutes          // 404
update(sessionId, ...)   → PUT  /v1/sessions/${sessionId}/minutes          // 404
getVersions(sessionId)   → GET  /v1/sessions/${sessionId}/minutes/versions // 404
```

Il server monta minutes a `/v1/minutes` (non sotto `/sessions`):

```go
// internal/api/server.go
r.Mount("/minutes", minutesHandler.Routes())
// Routes: GET /, GET /{id}, PUT /{id}, DELETE /{id}, GET /{id}/versions
```

Gli endpoint reali sono:
- `GET /v1/minutes?session_id={id}` → ottieni minuta per sessione
- `PUT /v1/minutes/{minutesId}` → aggiorna minuta (richiede minutesId, non sessionId)
- `GET /v1/minutes/{minutesId}/versions` → storia versioni

Questo è confermato anche dal bug fix del test-ui nella sessione precedente, dove
`fetchMinutes()` era stato fixato da `/v1/sessions/{id}/minutes` a `/v1/minutes?session_id={id}`.

### Problema aggiuntivo: `update` ha il parametro sbagliato

Il metodo `update(sessionId, request)` prende `sessionId` ma dovrebbe prendere `minutesId`.
Il PUT sul server richiede un minutes ID nel path (`PUT /v1/minutes/{id}`). Usare un
session ID come se fosse un minutes ID produce 404 o, peggio, un false positive se i
due ID coincidessero per caso.

### Fix completo di `sdk/ts/src/api/minutes.ts`

```typescript
export class MinutesAPI {
  constructor(private readonly http: HttpClient) {}

  /** GET /v1/minutes?session_id={sessionId} */
  async getBySession(sessionId: string): Promise<Minutes> {
    return this.http.get<Minutes>(`/v1/minutes?session_id=${encodeURIComponent(sessionId)}`);
  }

  /** GET /v1/minutes/{minutesId} */
  async get(minutesId: string): Promise<Minutes> {
    return this.http.get<Minutes>(`/v1/minutes/${minutesId}`);
  }

  /** PUT /v1/minutes/{minutesId}  — header X-User-Id per tracciare l'editor */
  async update(
    minutesId: string,
    request: UpdateMinutesRequest,
    userId?: string,
  ): Promise<Minutes> {
    const headers = userId ? { 'X-User-Id': userId } : {};
    return this.http.put<Minutes>(`/v1/minutes/${minutesId}`, request, { headers });
  }

  /** GET /v1/minutes/{minutesId}/versions */
  async getVersions(minutesId: string): Promise<MinutesVersion[]> {
    return this.http.get<MinutesVersion[]>(`/v1/minutes/${minutesId}/versions`);
  }

  /** DELETE /v1/minutes/{minutesId} */
  async delete(minutesId: string): Promise<void> {
    return this.http.delete<void>(`/v1/minutes/${minutesId}`);
  }
}
```

**Nota sul `userId`**: il server legge l'editor dall'header `X-User-Id` (non dal body).
`HttpClient.request()` accetta già `headers` in `RequestOptions` — nessuna modifica
necessaria all'HttpClient.

### Aggiornamento tipi necessario

`UpdateMinutesRequest` va aggiornato in `types.ts` rimuovendo eventuali riferimenti a
`userId` nel body (il campo non viene letto dal server):

```typescript
export interface UpdateMinutesRequest {
  sections?: Record<string, unknown>;
  notes?: string;
  // userId va passato come terzo param di update(), non nel body
}
```

---

## BUG 2 — CRITICO: `TranscriptionsAPI` usa endpoint inesistente

### Problema

```typescript
// sdk/ts/src/api/transcriptions.ts:17
GET /v1/sessions/${sessionId}/transcriptions   // 404 — non esiste
```

Il server monta transcriptions a `/v1/transcriptions`:

```go
// internal/api/server.go
r.Mount("/transcriptions", transcriptionHandler.Routes())
// Routes: GET / (con ?session_id=), GET /{id}
```

Endpoint corretto: `GET /v1/transcriptions?session_id={id}`.

### Fix di `sdk/ts/src/api/transcriptions.ts`

```typescript
export class TranscriptionsAPI {
  constructor(private readonly http: HttpClient) {}

  /** GET /v1/transcriptions?session_id={sessionId} */
  async listBySession(
    sessionId: string,
    filters?: TranscriptionFilters,
  ): Promise<PaginatedResponse<Transcription>> {
    const params = new URLSearchParams({ session_id: sessionId });
    if (filters?.limit !== undefined) params.set('limit', String(filters.limit));
    if (filters?.offset !== undefined) params.set('offset', String(filters.offset));
    return this.http.get<PaginatedResponse<Transcription>>(`/v1/transcriptions?${params}`);
  }

  /** GET /v1/transcriptions/{id} */
  async get(transcriptionId: string): Promise<Transcription> {
    return this.http.get<Transcription>(`/v1/transcriptions/${transcriptionId}`);
  }
}
```

Aggiungere `Transcription` come tipo singolo al metodo (il server espone `GET /v1/transcriptions/{id}`).

---

## BUG 3 — CRITICO: `sttProfiles`/`llmProfiles` in `ServerConfig` mai popolati

### Problema

In `sdk/ts/src/types.ts` sono stati aggiunti i campi:

```typescript
export interface ServerConfig {
  templates: Template[];
  defaultTemplateId: string;
  sttProfiles?: string[];
  sttDefaultProfile?: string;
  llmProfiles?: string[];
  llmDefaultProfile?: string;
}
```

Ma `ConfigAPI.getServerConfig()` chiama `GET /v1/config` che restituisce **solo**
`templates` e `default_template_id`. I nuovi campi saranno sempre `undefined`.

Il server ha i profili su `/demo/config` ma non su `/v1/config`. Usare `/demo/config`
nell'SDK non è corretto (endpoint non versionato, pensato per test UI).

### Root cause condivisa con PHP SDK (BUG 1 di improvement 24)

La fix richiede prima un'estensione del server a `/v1/config`. Vedere improvement 24,
BUG 1 per i dettagli della modifica Go.

### Fix lato SDK (dopo fix server)

`ConfigAPI.getServerConfig()` non richiede modifiche una volta che il server espone i
campi. Tuttavia il nome del metodo è incoerente con `client.config.getConfig()` citato
nella wiki. Rinominare per consistenza:

```typescript
// sdk/ts/src/api/config.ts
async getConfig(): Promise<ServerConfig> {         // era getServerConfig()
  return this.http.get<ServerConfig>('/v1/config');
}
```

Aggiornare anche `client.ts` e la wiki.

**Attenzione ai key name**: il server restituisce `default_stt_profile` (snake_case).
Il TS SDK usa camelCase nei tipi (`sttDefaultProfile`). Verificare che la deserializzazione
JSON → TypeScript faccia il mapping corretto, oppure allineare i nomi nel tipo:

```typescript
// Opzione A: mantenere i tipi camelCase e mappare in ConfigAPI
async getConfig(): Promise<ServerConfig> {
  const raw = await this.http.get<{
    templates: Template[];
    default_template_id: string;
    stt_profiles?: string[];
    llm_profiles?: string[];
    default_stt_profile?: string;
    default_llm_profile?: string;
  }>('/v1/config');

  return {
    templates:          raw.templates,
    defaultTemplateId:  raw.default_template_id,
    sttProfiles:        raw.stt_profiles,
    llmProfiles:        raw.llm_profiles,
    sttDefaultProfile:  raw.default_stt_profile,
    llmDefaultProfile:  raw.default_llm_profile,
  };
}

// Opzione B: usare snake_case nel tipo ServerConfig (coerente con il resto dei tipi webhook)
export interface ServerConfig {
  templates:            Template[];
  default_template_id:  string;
  stt_profiles?:        string[];
  llm_profiles?:        string[];
  default_stt_profile?: string;
  default_llm_profile?: string;
}
```

L'opzione A è preferibile: mantiene la convenzione camelCase già usata nel resto di
`types.ts` (`sessionId`, `templateId`, ecc.) e isola il mapping in un unico punto.

---

## BUG 4 — MODERATO: Wiki documenta `client.config.getConfig()` ma il metodo è `getServerConfig()`

### Problema

`docs/wiki/sdk.md`, sezione "Per-session STT/LLM provider profiles" (appena aggiunta):

```typescript
const { sttProfiles, sttDefaultProfile, ... } = await client.config.getConfig();
```

Il metodo reale in `ConfigAPI` è `getServerConfig()`. `getConfig()` non esiste → TypeError
a runtime. Stesso errore nel quick start della wiki:

```typescript
// docs/wiki/sdk.md — sezione client.config
const { templates, defaultTemplateId } = await client.config.getConfig();
// ← getConfig() non esiste, il metodo è getServerConfig()
```

### Fix

Due step:
1. Rinominare il metodo in `ConfigAPI` da `getServerConfig()` a `getConfig()` (vedi BUG 3)
2. Aggiornare tutti i riferimenti nella wiki

---

## BUG 5 — MODERATO: `MinutesAPI.update()` — wiki, firma e server inconsistenti

### Problema a tre livelli:

| Livello | Stato |
|---------|-------|
| Wiki `sdk.md` | `update(minutesId, { sections }, userId?)` — tre param |
| Tipo `UpdateMinutesRequest` | `sections?`, `notes?` — no `userId` |
| Implementazione attuale | `update(sessionId, request)` — sessionId errato, no userId |
| Server | legge editor da `X-User-Id` header, ignora body |

### Fix

La fix del BUG 1 (firma con `minutesId` + `userId` come terzo param) risolve anche
questo bug. Aggiornare la wiki per riflettere la firma corretta:

```typescript
// wiki corretta
await client.minutes.update(minutesId, { sections: { themes: ['ansia'] } }, 'doc_456');
//                   ^^^^^^^^          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^   ^^^^^^^^^
//                   minutes ID        corpo aggiornamento                 userId (header)
```

---

## BUG 6 — MINORE: Typo `AfterthalkClient` (double-h) non fixato

### Problema

Il punto di ingresso esporta `AfterthalkClient` (due `h`) invece di `AftertalkClient`.
Il bug è documentato nella wiki come "known issue" ma non è fixato.

```typescript
// sdk/ts/src/client.ts
export class AftertalkClient { ... }  // ← il class name è corretto internamente

// sdk/ts/src/index.ts — verificare cosa viene esportato
```

### Fix

Verificare `index.ts` per capire se il typo è nell'export o nel class name.
Se nell'export, rinominare il re-export. Se nel class name, rinominare la classe
e aggiungere un alias deprecato per backward compat:

```typescript
// index.ts — backward compat durante la transizione
export { AftertalkClient } from './client.js';
/** @deprecated Use AftertalkClient */
export { AftertalkClient as AfterthalkClient } from './client.js';
```

Rimuovere il "Known Issues" dalla wiki dopo il fix.

---

## Ordine di fix consigliato

I bug 1, 2, 3 dipendono in parte dalla stessa fix server (estendere `/v1/config`).
Il bug 1 e 2 invece sono risolvibili **subito**, indipendentemente dal server.

```
Priorità 1 (fix subito, nessuna dipendenza server):
  → BUG 1: fix MinutesAPI endpoint + firma update()
  → BUG 2: fix TranscriptionsAPI endpoint

Priorità 2 (richiede fix server su /v1/config):
  → BUG 3: sttProfiles/llmProfiles nel ServerConfig

Priorità 3 (cleanup conseguente):
  → BUG 4: rinominare getServerConfig() → getConfig() + wiki
  → BUG 5: wiki + firma update()

Priorità 4 (opzionale):
  → BUG 6: typo AfterthalkClient
```

## Riepilogo

| # | Bug | Severità | Metodo/File |
|---|-----|----------|-------------|
| 1 | MinutesAPI endpoint e firma sbagliati | CRITICO | `api/minutes.ts` |
| 2 | TranscriptionsAPI endpoint sbagliato | CRITICO | `api/transcriptions.ts` |
| 3 | sttProfiles mai popolati, `/v1/config` incompleto | CRITICO | `api/config.ts` + server |
| 4 | Wiki `getConfig()` vs `getServerConfig()` | MODERATO | `docs/wiki/sdk.md` |
| 5 | Inconsistenza `update()` wiki/firma/server | MODERATO | `api/minutes.ts` + wiki |
| 6 | Typo `AfterthalkClient` | MINORE | `index.ts` |

---

## Task

> **Prerequisito server**: i task marcati `[server]` devono precedere i task `[sdk]` corrispondenti.
> I task BUG 1 e BUG 2 non hanno prerequisiti server e si possono fare subito.

- [ ] **[sdk] BUG 1a** — Riscrivere `MinutesAPI.getBySession()`: `GET /v1/sessions/{id}/minutes` → `GET /v1/minutes?session_id={id}` (`sdk/ts/src/api/minutes.ts`)
- [ ] **[sdk] BUG 1b** — Riscrivere `MinutesAPI.update()`: cambiare firma da `update(sessionId, request)` a `update(minutesId, request, userId?)`, path `PUT /v1/minutes/{minutesId}`, inviare `userId` come header `X-User-Id` (`sdk/ts/src/api/minutes.ts`)
- [ ] **[sdk] BUG 1c** — Riscrivere `MinutesAPI.getVersions()`: `GET /v1/sessions/{id}/minutes/versions` → `GET /v1/minutes/{minutesId}/versions`, cambiare param da `sessionId` a `minutesId` (`sdk/ts/src/api/minutes.ts`)
- [ ] **[sdk] BUG 1d** — Aggiungere `MinutesAPI.get(minutesId)` per `GET /v1/minutes/{id}` (endpoint esistente, non esposto dall'SDK)
- [ ] **[sdk] BUG 2** — Riscrivere `TranscriptionsAPI.listBySession()`: `GET /v1/sessions/{id}/transcriptions` → `GET /v1/transcriptions?session_id={id}` (`sdk/ts/src/api/transcriptions.ts`)
- [ ] **[sdk] BUG 2b** — Aggiungere `TranscriptionsAPI.get(transcriptionId)` per `GET /v1/transcriptions/{id}` (endpoint esistente, non esposto dall'SDK)
- [ ] **[server] BUG 3 prereq** — Estendere `GET /v1/config` con `stt_profiles`, `llm_profiles`, `default_stt_profile`, `default_llm_profile`; spostarlo fuori da `apiKeyMiddleware` (vedi improvement 26 task server)
- [ ] **[sdk] BUG 3** — Rinominare `ConfigAPI.getServerConfig()` → `getConfig()`; aggiungere mapping snake_case → camelCase nella risposta per `sttProfiles`, `llmProfiles`, `sttDefaultProfile`, `llmDefaultProfile` (`sdk/ts/src/api/config.ts`)
- [ ] **[sdk] BUG 3b** — Verificare che `ServerConfig` in `types.ts` usi camelCase coerente; aggiungere i nuovi campi se mancanti
- [ ] **[sdk] BUG 4** — Aggiornare `docs/wiki/sdk.md`: sostituire `client.config.getConfig()` → `client.config.getConfig()` (ok dopo rename BUG 3), verificare tutti gli snippet della wiki contro la firma reale
- [ ] **[sdk] BUG 5** — Aggiornare firma `update()` nella wiki e in `UpdateMinutesRequest` (rimuovere `userId` dal tipo body, documentare come terzo param)
- [ ] **[sdk] BUG 6** — Verificare `sdk/ts/src/index.ts`: se il typo è nell'export aggiungere alias deprecato `AfterthalkClient`, rimuovere "Known Issues" dalla wiki dopo il fix
- [ ] **[test]** Aggiungere test per `MinutesAPI` che verifica i path corretti (usando `vitest` con mock fetch)
- [ ] **[test]** Aggiungere test per `TranscriptionsAPI.listBySession` che verifica `?session_id=` nel query string
- [ ] **[test]** Aggiungere test per `ConfigAPI.getConfig()` che verifica il mapping camelCase dei profili
- [ ] **[rebuild]** Dopo tutte le modifiche: `npm run build` in `sdk/ts/`, verificare che il dist rifletta i nuovi tipi
