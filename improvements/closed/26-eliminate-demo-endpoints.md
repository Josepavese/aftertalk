# Improvement 26: Eliminazione endpoint demo — demo gira sull'SDK

## Stato: APERTO

## Proposta

Eliminare gli endpoint custom `/demo/config` e `/test/start` dal server core.
La demo UI (`cmd/test-ui`) diventa un client puro che usa esclusivamente l'SDK
TypeScript e gli endpoint `/v1/*` stabili, come farebbe qualsiasi integratore reale.

---

## Situazione attuale

Il server espone tre categorie di endpoint:

```
/v1/*         → API pubblica stabile, versionata, con API key
/demo/config  → metadata pubblici per la demo UI (no auth, non versionato)
/test/start   → join/crea sessione da room code (auth condizionale, non versionato)
/             → serve staticamente la demo UI (dist/index.html)
```

La test UI (`cmd/test-ui/src/main.ts`) **usa già l'SDK** (`@aftertalk/sdk`) per
WebRTC, sessioni, minuti e polling. Ma fa ancora due chiamate raw fuori dall'SDK:
- `fetch('/demo/config')` → per caricare templates e profili STT/LLM all'avvio
- `fetch('/test/start', { method: 'POST' })` → per join/crea stanza da room code

---

## Problemi degli endpoint demo

### `/demo/config` — non dovrebbe esistere

1. **Non è sotto `/v1/`** → non segue il versionamento, può rompersi senza notice
2. **Duplica parzialmente `/v1/config`** → due sorgenti di verità per templates e
   default_template_id
3. **Root cause dei bug in improvement 24 e 25**: la PHP SDK usa `/demo/config` invece
   di `/v1/config`, e `sttProfiles`/`llmProfiles` non sono su `/v1/config`
4. **Espone `api_key`** in demo mode → pattern pericoloso anche se limitato a dev
5. **È pubblico** (no auth), ma per una ragione legittima: la demo UI ha bisogno dei
   templates prima che l'utente inserisca la API key

### `/test/start` — logica utile, nome e posizione sbagliati

`/test/start` implementa un meccanismo **room code → sessione**:
- Dato un codice stanza + nome + ruolo, crea una sessione la prima volta
- I partecipanti successivi ottengono il loro token dalla stessa sessione
- Il ruolo è esclusivo (due persone non possono avere lo stesso ruolo nella stanza)
- Gestisce riconnessione (stesso nome + ruolo → stesso token)
- TTL 3 ore sulla cache delle stanze

Questa logica è genuinamente utile — non solo per la demo, ma per qualsiasi
integratore che voglia un flusso "entra nella stanza" invece del flusso esplicito
"crea sessione → distribuisci token". Il problema non è la funzionalità ma il nome
(`/test/start`) e la collocazione fuori da `/v1/`.

---

## Soluzione proposta

### Step 1 — Estendere `/v1/config` (server)

Aggiungere i profili STT/LLM a `/v1/config` e renderlo **pubblico** (spostarlo fuori
dal gruppo `apiKeyMiddleware`):

```go
// internal/api/server.go — spostare fuori dal gruppo protetto
r.Get("/v1/config", func(w http.ResponseWriter, req *http.Request) {
    sttProfiles := make([]string, 0, len(cfg.STT.Profiles))
    for name := range cfg.STT.Profiles {
        sttProfiles = append(sttProfiles, name)
    }
    llmProfiles := make([]string, 0, len(cfg.LLM.Profiles))
    for name := range cfg.LLM.Profiles {
        llmProfiles = append(llmProfiles, name)
    }
    response.OK(w, map[string]interface{}{
        "templates":           cfg.Templates,
        "default_template_id": defaultTemplateID,
        "stt_profiles":        sttProfiles,
        "llm_profiles":        llmProfiles,
        "default_stt_profile": cfg.STT.DefaultProfile,
        "default_llm_profile": cfg.LLM.DefaultProfile,
    })
})
```

`/v1/config` è già commentato come "public metadata (templates, version) without API key
exposure" — era già l'intenzione, ma era rimasto accidentalmente dentro il gruppo protetto.

### Step 2 — Rinominare `/test/start` → `POST /v1/rooms/join`

Promuovere la room logic a endpoint ufficiale versionato:

```
POST /v1/rooms/join
```

Request body (invariata):
```json
{
  "code":        "stanza-abc",
  "name":        "Dott. Rossi",
  "role":        "terapeuta",
  "template_id": "therapy",    // opzionale
  "stt_profile": "cloud",      // opzionale
  "llm_profile": "local"       // opzionale
}
```

Response (invariata):
```json
{
  "session_id": "uuid",
  "token":      "eyJ..."
}
```

La `roomCache` con TTL e mutex rimane nel server — è logica stateful e concurrency-safe
che non può stare nel frontend. Semplicemente diventa un endpoint stabile.

**Autenticazione**: richiedere API key (come oggi con `apiKeyMiddleware`).
La demo UI può includere la API key nel payload o nell'header dopo che l'utente la digita.

### Step 3 — Eliminare `/demo/config` (server)

Una volta che `/v1/config` è pubblico e completo, `/demo/config` non serve più.
Rimuovere il route handler, il campo `Demo.Enabled` dalla config, e tutti i riferimenti.

```go
// DA ELIMINARE in internal/api/server.go:
r.Get("/demo/config", func(...) { ... })  // ~30 righe

// DA ELIMINARE in internal/config/config.go:
Demo struct {
    Enabled bool `koanf:"enabled" yaml:"enabled"`
}
```

### Step 4 — Aggiornare la demo UI (`cmd/test-ui/src/main.ts`)

La `main.ts` usa già l'SDK. Sostituire le due chiamate raw rimanenti:

**Prima (fetch raw):**
```typescript
// chiamata a /demo/config
const res = await fetch(`${API}/demo/config`);
const cfg = await res.json();
```

**Dopo (SDK):**
```typescript
const cfg = await client.config.getConfig();
// ora ritorna templates, sttProfiles, llmProfiles, default*
```

**Prima (fetch raw a /test/start):**
```typescript
const res = await fetch(`${API}/test/start`, {
  method: 'POST',
  body: JSON.stringify({ code, name, role, template_id, stt_profile, llm_profile }),
  headers: { 'X-API-Key': apiKey, 'Content-Type': 'application/json' },
});
const { session_id, token } = await res.json();
```

**Dopo (SDK — aggiungere `RoomsAPI` all'SDK):**
```typescript
const { sessionId, token } = await client.rooms.join({
  code, name, role, templateId, sttProfile, llmProfile,
});
```

### Step 5 — Aggiungere `RoomsAPI` all'SDK TypeScript

```typescript
// sdk/ts/src/api/rooms.ts
export interface JoinRoomRequest {
  code:        string;
  name:        string;
  role:        string;
  templateId?: string;
  sttProfile?: string;
  llmProfile?: string;
}

export interface JoinRoomResponse {
  sessionId: string;
  token:     string;
}

export class RoomsAPI {
  constructor(private readonly http: HttpClient) {}

  /**
   * Join or create a room session by code.
   * Creates the session the first time; subsequent participants get their own token.
   * Role is exclusive: two participants cannot share the same role.
   */
  async join(request: JoinRoomRequest): Promise<JoinRoomResponse> {
    const raw = await this.http.post<{ session_id: string; token: string }>(
      '/v1/rooms/join',
      {
        code:        request.code,
        name:        request.name,
        role:        request.role,
        template_id: request.templateId,
        stt_profile: request.sttProfile,
        llm_profile: request.llmProfile,
      },
    );
    return { sessionId: raw.session_id, token: raw.token };
  }
}
```

Aggiungere `rooms: RoomsAPI` a `AftertalkClient`.

### Step 6 — (Opzionale) PHP SDK: aggiungere `RoomsApi`

```php
// sdk/php/src/Api/RoomsApi.php
class RoomsApi
{
    public function join(
        string  $code,
        string  $name,
        string  $role,
        ?string $templateId = null,
        ?string $sttProfile = null,
        ?string $llmProfile = null,
    ): array {  // ['sessionId' => ..., 'token' => ...]
        $data = $this->http->post('/v1/rooms/join', array_filter([
            'code'        => $code,
            'name'        => $name,
            'role'        => $role,
            'template_id' => $templateId,
            'stt_profile' => $sttProfile,
            'llm_profile' => $llmProfile,
        ], fn($v) => $v !== null));

        return [
            'sessionId' => $data['session_id'],
            'token'     => $data['token'],
        ];
    }
}
```

---

## Architettura risultante

```
Server:
  GET  /v1/config         → pubblico, templates + profili STT/LLM
  POST /v1/rooms/join     → room code → session_id + token (API key richiesta)
  ...tutti gli altri /v1/*

Demo UI (cmd/test-ui):
  → usa @aftertalk/sdk esclusivamente
  → nessuna fetch() raw
  → è un example di come usare l'SDK, non un caso speciale

SDK TS:
  client.config.getConfig()   → GET /v1/config
  client.rooms.join(...)      → POST /v1/rooms/join
  client.sessions.*           → GET/POST /v1/sessions/*
  ...

SDK PHP:
  $client->config->getConfig()  → GET /v1/config
  $client->rooms->join(...)     → POST /v1/rooms/join
  $client->sessions->*          → GET/POST /v1/sessions/*
  ...
```

---

## Benefici

| Beneficio | Dettaglio |
|-----------|-----------|
| **Elimina bug PHP SDK #1 e TS SDK #3** | `/v1/config` diventa la sorgente unica di verità |
| **Test UI dimostra l'SDK** | Un integratore che legge la demo capisce esattamente come usare l'SDK |
| **Room join diventa API stabile** | Utile per integratori (MondoPsicologi potrebbe usarlo per sessioni walk-in) |
| **Meno codice server** | ~60 righe di endpoint demo eliminati, `Demo.Enabled` config rimossa |
| **Un solo endpoint config** | Nessuna duplicazione tra `/demo/config` e `/v1/config` |

---

## Ordine di implementazione

```
1. Server: estendere + rendere pubblico /v1/config
2. Server: rinominare /test/start → POST /v1/rooms/join
3. Server: eliminare /demo/config e Demo.Enabled config
4. SDK TS: aggiungere RoomsAPI, aggiornare ConfigAPI.getConfig()
5. SDK PHP: aggiornare ConfigApi (/v1/config), aggiungere RoomsApi
6. Demo UI: rimuovere fetch raw, usare solo SDK
7. Fix TS bug #1 e #2 (MinutesAPI + TranscriptionsAPI) — da improvement 25
8. Fix PHP bug #2 e #3 (chiavi JSON + user_id header) — da improvement 24
```

Steps 1-3 sono prerequisiti per tutto il resto.
Steps 4-6 possono avanzare in parallelo dopo step 1-3.
Steps 7-8 sono indipendenti e possono andare in parallelo da subito.

---

## Note

- **La `roomCache` rimane server-side**: è stateful con mutex e TTL. Non può
  stare nel frontend (ogni browser avrebbe una cache separata). È logica
  corretta per un server.
- **`/v1/config` pubblico vs protetto**: renderlo pubblico è intenzionale —
  serve per il bootstrap della UI prima che l'utente fornisca la API key. Non
  espone dati sensibili (solo template e nomi di profili, non credenziali).
- **Retrocompatibilità `/demo/config`**: se ci sono client esistenti che usano
  `/demo/config`, aggiungere un redirect temporaneo a `/v1/config` prima di
  rimuoverlo definitivamente.

---

## Task

> I task sono ordinati per dipendenza. Server prima, poi SDK, poi demo UI.

### Server (`internal/api/server.go`)

- [ ] **[server]** Estendere `GET /v1/config` con `stt_profiles`, `llm_profiles`, `default_stt_profile`, `default_llm_profile` (stessa logica di `/demo/config` righe 185-200)
- [ ] **[server]** Spostare `r.Get("/config", ...)` fuori dal gruppo `apiKeyMiddleware` → diventa pubblico
- [ ] **[server]** Aggiungere route `POST /v1/rooms/join` (promuovere logica di `/test/start`): spostare il handler nel gruppo `/v1`, rinominare path
- [ ] **[server]** Eliminare `r.Get("/demo/config", ...)` (~30 righe, righe 182-208)
- [ ] **[server]** Eliminare `r.With(apiKeyMiddleware).Post("/test/start", ...)` (~80 righe, righe 212-288)
- [ ] **[server]** Eliminare `roomTTL`, `roomEntry`, `roomCache`, `errRoleTaken`, `getOrCreate` dal file (righe 33-88) — spostarli in un file dedicato `internal/api/rooms.go` per pulizia
- [ ] **[config]** Eliminare `Demo struct { Enabled bool }` da `internal/config/config.go`
- [ ] **[config]** Eliminare tutti i riferimenti a `cfg.Demo.Enabled` nel codebase

### SDK TypeScript (`sdk/ts/`)

- [ ] **[sdk-ts]** Creare `sdk/ts/src/api/rooms.ts` con `RoomsAPI` e `JoinRoomRequest`/`JoinRoomResponse`
- [ ] **[sdk-ts]** Aggiungere `rooms: RoomsAPI` a `AftertalkClient` in `sdk/ts/src/client.ts`
- [ ] **[sdk-ts]** Aggiornare `ConfigAPI.getServerConfig()` → `getConfig()`, aggiungere mapping profili STT/LLM (dipende da task server)
- [ ] **[sdk-ts]** Esportare `RoomsAPI`, `JoinRoomRequest`, `JoinRoomResponse` da `sdk/ts/src/index.ts`
- [ ] **[sdk-ts]** `npm run build` e verificare dist aggiornato

### SDK PHP (`sdk/php/`)

- [ ] **[sdk-php]** Creare `sdk/php/src/Api/RoomsApi.php` con metodo `join()`
- [ ] **[sdk-php]** Aggiungere `public readonly RoomsApi $rooms` ad `AftertalkClient`
- [ ] **[sdk-php]** Aggiornare `ConfigApi::getConfig()` a `/v1/config` (dipende da task server)
- [ ] **[sdk-php]** Correggere chiavi JSON in `ServerConfig::fromArray()` per i profili (vedi improvement 24 BUG 2)

### Demo UI (`cmd/test-ui/`)

- [ ] **[ui]** Sostituire `fetch('/demo/config')` con `client.config.getConfig()` in `src/main.ts`
- [ ] **[ui]** Sostituire `fetch('/test/start', { method: 'POST' })` con `client.rooms.join(...)` in `src/main.ts`
- [ ] **[ui]** Verificare che non rimangano altre chiamate `fetch()` raw in `src/main.ts` non coperte dall'SDK
- [ ] **[ui]** `npm run build` in `cmd/test-ui/`, aggiornare `dist/`

### Wiki e documentazione

- [ ] **[docs]** Aggiornare `docs/wiki/sdk.md`: aggiungere sezione `client.rooms.join()`
- [ ] **[docs]** Aggiornare `docs/wiki/sdk.md`: aggiornare sezione `client.config` → `getConfig()` con nuovi campi profili
- [ ] **[docs]** Aggiornare `docs/wiki/sdk-php.md`: aggiungere sezione `$client->rooms->join()`
- [ ] **[docs]** Aggiornare `docs/wiki/configuration.md` (se esiste): rimuovere riferimenti a `Demo.Enabled`

### Test

- [ ] **[test]** Aggiungere test per `RoomsAPI.join()` nel TS SDK (mock fetch, verifica path `/v1/rooms/join`)
- [ ] **[test]** Aggiungere test per `RoomsApi::join()` nel PHP SDK (mock PSR-18, verifica body)
- [ ] **[test]** Aggiungere test server per `POST /v1/rooms/join` (verifica role conflict → 409, verifica riconnessione)
