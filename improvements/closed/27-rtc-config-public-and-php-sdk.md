# Improvement #27 — Public `/v1/rtc-config` + PHP SDK parity

## Contesto

Il pattern d'uso standard è:

```
PHP middleware (ha API key)
  → POST /v1/rooms/join  →  restituisce { sessionId, token } al frontend

Frontend browser (NO API key, solo JWT token)
  → AftertalkClient({ baseUrl })     // apiKey è optional nel TS SDK
  → client.config.getConfig()        // /v1/config è public ✅
  → client.connectWebRTC({ sessionId, token })  // autentica via ?token= JWT
```

Il token JWT è sufficiente per autenticare il WebSocket di signaling. L'API key non serve al frontend.

## Il problema

`connectWebRTC()` nel TS SDK (e il corrispondente flow PHP) chiama internamente
`GET /v1/rtc-config` per ottenere la lista ICE server prima di aprire il WebSocket.
Quell'endpoint è attualmente dentro l'`apiKeyMiddleware`:

```go
// server.go — dentro r.Group con apiKeyMiddleware
r.Get("/rtc-config", rtcHandler.ServeHTTP)
```

Un frontend senza API key riceve **401** prima ancora di aprire il WebSocket, rendendo
il pattern "PHP middleware + TS SDK frontend" inutilizzabile senza workaround.

Gli ICE server (STUN/TURN) **non sono segreti**: contengono indirizzi e porte pubblici.
Non c'è motivo architetturale per proteggerli con l'API key.

## Fix

### Server (Go) — 1 modifica

Spostare `GET /v1/rtc-config` **fuori** dall'`apiKeyMiddleware`, accanto a `GET /v1/config`:

```go
// Registrare accanto a /v1/config, fuori dal gruppo apiKeyMiddleware
if rtcHandler != nil {
    r.Get("/v1/rtc-config", rtcHandler.ServeHTTP)
}
```

Rimuovere il blocco corrispondente dal gruppo protetto in `server.go`.

### TS SDK — nessuna modifica

`ConfigAPI.getRTCConfig()` chiama già `/v1/rtc-config`. Funzionerà non appena
il server lo rende pubblico.

### PHP SDK — aggiungere parità con TS

Il PHP SDK non espone nessun metodo per ottenere gli ICE server. Aggiungere:

#### `sdk/php/src/Dto/RtcConfig.php` (nuovo)

```php
<?php
declare(strict_types=1);
namespace Aftertalk\Dto;

class RtcConfig
{
    /**
     * @param list<array{urls: string|list<string>, username?: string, credential?: string}> $iceServers
     */
    public function __construct(
        public readonly array $iceServers,
    ) {}

    public static function fromArray(array $data): self
    {
        return new self(
            iceServers: $data['ice_servers'] ?? [],
        );
    }
}
```

#### `sdk/php/src/Api/ConfigApi.php` — aggiungere metodo

```php
public function getRtcConfig(): RtcConfig
{
    $data = $this->http->get('/v1/rtc-config');
    return RtcConfig::fromArray($data);
}
```

## Risposta server

```json
{
  "ice_servers": [
    { "urls": ["stun:stun.l.google.com:19302"] },
    { "urls": ["turn:turn.example.com:3478"], "username": "user", "credential": "pass" }
  ],
  "ttl": 86400,
  "provider": "static"
}
```

Chiave confermata `ice_servers` (snake_case) da `rtcConfigResponse` in `internal/api/handler/rtc.go:37`.

## Task checklist

### Server
- [ ] Spostare `GET /v1/rtc-config` fuori dall'`apiKeyMiddleware` in `internal/api/server.go`
- [ ] Rimuovere il blocco corrispondente dal gruppo protetto
- [ ] Aggiornare `internal/api/integration_test.go`: aggiungere `/v1/rtc-config` alla lista dei route pubblici in `TestAPI_PublicRoutes_NoAuth`

### PHP SDK
- [ ] Creare `sdk/php/src/Dto/RtcConfig.php`
- [ ] Aggiungere `use Aftertalk\Dto\RtcConfig;` e metodo `getRtcConfig(): RtcConfig` in `sdk/php/src/Api/ConfigApi.php`
- [ ] Verificare la chiave JSON esatta del server (`ice_servers` vs `iceServers`) e allineare `fromArray()`

### TS SDK
- [ ] Nessuna modifica necessaria (già implementato)

### Verifica
- [ ] `go build ./...` pulito
- [ ] `go test ./internal/api/...` — il test `TestAPI_PublicRoutes_NoAuth` include `/v1/rtc-config`
- [ ] Test manuale: `new AftertalkClient({ baseUrl })` senza `apiKey` → `connectWebRTC()` non ritorna 401
