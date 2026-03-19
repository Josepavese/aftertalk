# Improvement 24: Bugfix PHP SDK (`aftertalk/aftertalk-php`)

## Stato: APERTO

## Contesto

Analisi devil's advocate del PHP SDK (improvement 13) ha identificato bug funzionali,
incongruenze con il server e problemi di robustezza. Alcuni causano silently-wrong behaviour
(il codice gira ma fa la cosa sbagliata), altri rompono feature intere.

I bug sono raggruppati per severità e includono la fix esatta da applicare.

---

## BUG 1 — CRITICO: `ConfigApi` usa `/demo/config` invece di `/v1/config`

### Problema

`ConfigApi::getConfig()` chiama `/demo/config`:

```php
// src/Api/ConfigApi.php:20
$data = $this->http->get('/demo/config');
```

`/demo/config` è un endpoint per il test UI, non un'API pubblica stabile:
- Non è sotto `/v1/` → non seguirà il versionamento API
- È pensato per essere consumato dal browser, non da backend PHP
- Restituisce `api_key` quando `Demo.Enabled=true` (mai in produzione, ma è un segnale)

Il vero endpoint pubblico è `/v1/config` (richiede API key). Il problema reale è che
`/v1/config` **non espone** `stt_profiles` / `llm_profiles` — quelli li ha solo `/demo/config`.

### Root cause

Il server ha due endpoint che si sovrappongono parzialmente:

| Campo | `/v1/config` | `/demo/config` |
|-------|-------------|----------------|
| `templates` | ✅ | ✅ |
| `default_template_id` | ✅ | ✅ |
| `stt_profiles` | ❌ | ✅ |
| `llm_profiles` | ❌ | ✅ |
| `default_stt_profile` | ❌ | ✅ |
| `default_llm_profile` | ❌ | ✅ |
| `api_key` (solo demo mode) | ❌ | ✅ |

### Fix

**Lato server** (priorità): estendere `/v1/config` in `internal/api/server.go` per includere i profili:

```go
// internal/api/server.go — r.Get("/config", ...)
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
```

**Lato SDK** (conseguente): cambiare `ConfigApi::getConfig()` a `/v1/config`:

```php
// src/Api/ConfigApi.php
$data = $this->http->get('/v1/config');  // era /demo/config
```

---

## BUG 2 — CRITICO: `ServerConfig::fromArray()` usa chiavi JSON sbagliate

### Problema

Il server restituisce da `/demo/config`:
```json
{
  "default_stt_profile": "local",
  "default_llm_profile": "local"
}
```

Ma `ServerConfig::fromArray()` cerca:
```php
// src/Dto/ServerConfig.php:29-31
sttDefaultProfile:  $data['stt_default_profile'] ?? null,   // ← chiave invertita
llmDefaultProfile:  $data['llm_default_profile'] ?? null,   // ← chiave invertita
```

`stt_default_profile` ≠ `default_stt_profile`. Risultato: `sttDefaultProfile` e
`llmDefaultProfile` sono sempre `null` indipendentemente dalla configurazione del server.

### Fix

Allineare le chiavi in `ServerConfig::fromArray()` a quelle restituite dal server:

```php
// src/Dto/ServerConfig.php
sttDefaultProfile:  $data['default_stt_profile'] ?? null,   // era stt_default_profile
llmDefaultProfile:  $data['default_llm_profile'] ?? null,   // era llm_default_profile
```

**Nota**: dopo il BUG 1 fix, il server dovrà usare le stesse chiavi su `/v1/config`.
Scegliere un formato canonico e usarlo ovunque — preferibilmente `default_stt_profile`
(coerente con `default_template_id` già esistente).

---

## BUG 3 — CRITICO: `MinutesApi::update()` invia `user_id` nel body, ma il server lo legge dall'header

### Problema

`MinutesApi::update()` mette `user_id` nel body JSON:

```php
// src/Api/MinutesApi.php:39-43
$body = array_filter([
    'sections' => $sections,
    'notes'    => $notes,
    'user_id'  => $userId,   // ← body JSON
], fn($v) => $v !== null);
```

Il server `UpdateMinutes` legge l'editor dall'header HTTP, **non dal body**:

```go
// internal/api/handler/minutes.go:107
editedBy := r.Header.Get("X-User-Id")
if editedBy == "" {
    editedBy = "unknown"
}
```

Risultato: il `$userId` passato dall'SDK viene **ignorato silenziosamente**. Il server
registra sempre `"unknown"` come editor nella history delle versioni.

### Fix

Inviare `user_id` come header HTTP in `HttpClient::put()`, oppure aggiungere un metodo
`putWithUserId()` dedicato. La soluzione più pulita è passare header extra al `HttpClient`:

```php
// src/Http/HttpClient.php — aggiungere $headers param a put()
public function put(string $path, array $body = [], array $headers = []): array
{
    // ... costruzione request ...
    foreach ($headers as $name => $value) {
        $request = $request->withHeader($name, $value);
    }
    return $this->send($request);
}

// src/Api/MinutesApi.php — update() usa l'header
public function update(
    string  $minutesId,
    array   $sections,
    ?string $notes  = null,
    ?string $userId = null,
): Minutes {
    $body = array_filter([
        'sections' => $sections,
        'notes'    => $notes,
    ], fn($v) => $v !== null);

    $headers = $userId !== null ? ['X-User-Id' => $userId] : [];

    $data = $this->http->put("/v1/minutes/{$minutesId}", $body, $headers);
    return Minutes::fromArray($data);
}
```

---

## BUG 4 — MODERATO: `WebhookHandler` con secret vuoto non dà errore diagnostico

### Problema

```php
// src/AftertalkClient.php:85
$this->webhook = new WebhookHandler($webhookSecret ?? '');
```

Se `$webhookSecret` è `null` (dimenticato nel costruttore), l'handler viene creato con `''`.
Ogni chiamata a `verify()` calcolerà `hash_hmac('sha256', $body, '')` e lo confronterà
con la firma reale → sempre `WebhookSignatureException`. L'utente non capisce perché e
non riceve nessun messaggio utile.

### Fix

Due opzioni:

**Opzione A** — fail-fast nel costruttore di `WebhookHandler`:

```php
// src/Webhook/WebhookHandler.php
public function __construct(private readonly string $secret)
{
    if ($secret === '') {
        throw new \LogicException(
            'WebhookHandler requires a non-empty secret. ' .
            'Set webhookSecret in AftertalkClient constructor.'
        );
    }
}
```

**Opzione B** — non esporre `$client->webhook` se il secret non è configurato
(lazy init o `null` con controllo):

```php
// src/AftertalkClient.php
$this->webhook = $webhookSecret !== null
    ? new WebhookHandler($webhookSecret)
    : null;
```

L'opzione A è preferibile: fail-fast, messaggio chiaro, nessuna nullable da gestire.

---

## BUG 5 — MODERATO: Eccezioni PSR-18 di rete non wrappate in `AftertalkException`

### Problema

```php
// src/Http/HttpClient.php:95
$response = $this->client->sendRequest($request);
```

`ClientInterface::sendRequest()` può lanciare `Psr\Http\Client\ClientExceptionInterface`
per timeout, connessione rifiutata, DNS failure ecc. Queste eccezioni PSR-18 bubblano
fuori dall'SDK in modo raw: chi usa il PHP SDK deve gestire **due famiglie di eccezioni**
diverse senza saperlo.

Il TS SDK gestisce questo correttamente (wrappa in `AftertalkError` network_error/timeout).

### Fix

Wrappare `sendRequest()` in un try/catch nel metodo `send()`:

```php
// src/Http/HttpClient.php
private function send(\Psr\Http\Message\RequestInterface $request): array
{
    try {
        $response = $this->client->sendRequest($request);
    } catch (\Psr\Http\Client\ClientExceptionInterface $e) {
        throw new AftertalkException(
            'Network error: ' . $e->getMessage(),
            0,
            null,
            $e,
        );
    }
    // ... resto invariato
}
```

Aggiungere anche gestione del timeout (alcuni client PSR-18 lanciano un'eccezione
specifica, altri wrappano in `ClientException`):

```php
} catch (\Psr\Http\Client\NetworkExceptionInterface $e) {
    throw new AftertalkException('Network unreachable: ' . $e->getMessage(), 0, null, $e);
} catch (\Psr\Http\Client\RequestExceptionInterface $e) {
    throw new AftertalkException('Invalid request: ' . $e->getMessage(), 0, null, $e);
} catch (\Psr\Http\Client\ClientExceptionInterface $e) {
    throw new AftertalkException('HTTP client error: ' . $e->getMessage(), 0, null, $e);
}
```

---

## BUG 6 — MINORE: `resolveHttpDeps()` non valida injection parziale

### Problema

```php
// src/AftertalkClient.php:96
if ($httpClient !== null && $requestFactory !== null && $streamFactory !== null) {
    return [$httpClient, $requestFactory, $streamFactory];
}
// → se solo $httpClient è fornito, si entra in auto-discovery per factory
```

Se l'utente passa `$httpClient` ma dimentica `$requestFactory`, l'SDK usa auto-discovery
per le factory — che potrebbe scoprire factory incompatibili con il client iniettato (es.
Guzzle client + Symfony factory in certi scenari edge). Il silent fallback è pericoloso.

### Fix

Validare che l'injection sia "tutto o niente" (tutti e tre o nessuno):

```php
$injected = array_filter([$httpClient, $requestFactory, $streamFactory], fn($v) => $v !== null);
if (count($injected) > 0 && count($injected) < 3) {
    throw new \InvalidArgumentException(
        'When injecting HTTP dependencies, all three must be provided: ' .
        '$httpClient, $requestFactory, $streamFactory.'
    );
}
```

---

## BUG 7 — MINORE: Stile inconsistente nel filtraggio null dei query param

### Problema

`SessionsApi::list()` passa i null a `HttpClient::get()` che li filtra internamente:

```php
$data = $this->http->get('/v1/sessions', [
    'status' => $status,  // può essere null
    'limit'  => $limit,   // può essere null
]);
```

`TranscriptionsApi::listBySession()` filtra prima di passare:

```php
$data = $this->http->get('/v1/transcriptions', array_filter([
    'session_id' => $sessionId,
    'limit'      => $limit,
], fn($v) => $v !== null));
```

Stesso risultato, stile diverso. Chi estende l'SDK non sa qual è il contratto di
`HttpClient::get()` riguardo ai null.

### Fix

Scegliere un'unica convenzione e documentarla nel phpdoc di `HttpClient::get()`:

```php
/**
 * @param array<string, scalar|null> $query  Null values are filtered out automatically.
 */
public function get(string $path, array $query = []): array
```

E usare sempre la forma senza pre-filtraggio nei chiamanti (delega a HttpClient).

---

## Riepilogo priorità di fix

| # | Bug | Severità | Impatto |
|---|-----|----------|---------|
| 1 | `ConfigApi` usa `/demo/config` | CRITICO | Endpoint sbagliato, non stabile |
| 2 | Chiavi JSON invertite in `ServerConfig` | CRITICO | Profili default sempre `null` |
| 3 | `user_id` in body invece che in header | CRITICO | Editor sempre `"unknown"` in history |
| 4 | `WebhookHandler` con secret vuoto | MODERATO | Errore non diagnosticabile |
| 5 | Eccezioni PSR-18 non wrappate | MODERATO | Due famiglie di eccezioni raw |
| 6 | Injection parziale non validata | MINORE | Silent incompatibility |
| 7 | Stile inconsistente null-filter | MINORE | Manutenibilità |

## Dipendenza con BUG 1

Il bug 1 richiede una modifica al **server** Go per estendere `/v1/config`.
Questa modifica sblocca anche il TS SDK (improvement 25).
Ordine consigliato di esecuzione:

1. Fix server: estendere `/v1/config`
2. Fix PHP SDK: bug 1, 2, 3 (critici)
3. Fix PHP SDK: bug 4, 5 (moderati)
4. Fix PHP SDK: bug 6, 7 (minori, opzionali)

---

## Task

- [ ] **[server]** Estendere `GET /v1/config` con `stt_profiles`, `llm_profiles`, `default_stt_profile`, `default_llm_profile` — prerequisito per BUG 1 e BUG 2 (`internal/api/server.go`)
- [ ] **[server]** Spostare `GET /v1/config` fuori dal gruppo `apiKeyMiddleware` (renderlo pubblico)
- [ ] **[php]** BUG 1 — Cambiare `ConfigApi::getConfig()` da `/demo/config` a `/v1/config` (`src/Api/ConfigApi.php`)
- [ ] **[php]** BUG 2 — Correggere chiavi JSON in `ServerConfig::fromArray()`: `stt_default_profile` → `default_stt_profile`, `llm_default_profile` → `default_llm_profile` (`src/Dto/ServerConfig.php`)
- [ ] **[php]** BUG 3 — Aggiungere parametro `array $headers = []` a `HttpClient::put()`, inviare `userId` come header `X-User-Id` invece che nel body (`src/Http/HttpClient.php`, `src/Api/MinutesApi.php`)
- [ ] **[php]** BUG 4 — Aggiungere check secret vuoto nel costruttore di `WebhookHandler` con `\LogicException` diagnostica (`src/Webhook/WebhookHandler.php`)
- [ ] **[php]** BUG 5 — Wrappare `$this->client->sendRequest()` in try/catch con gerarchia PSR-18 → `AftertalkException` (`src/Http/HttpClient.php`)
- [ ] **[php]** BUG 6 — Validare injection parziale in `resolveHttpDeps()`: tutti e tre o nessuno (`src/AftertalkClient.php`)
- [ ] **[php]** BUG 7 — Uniformare stile null-filter nei chiamanti: delegare sempre a `HttpClient::get()`, documentare nel phpdoc
- [ ] **[test]** Aggiornare `WebhookHandlerTest` per coprire il caso secret vuoto (aspettarsi `\LogicException`)
- [ ] **[test]** Aggiungere test per `SessionsApi::list()` con filtri null
- [ ] **[test]** Aggiungere test per `MinutesApi::update()` che verifica header `X-User-Id` (richiede mock dell'HttpClient con verifica degli header)
