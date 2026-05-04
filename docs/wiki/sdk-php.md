# PHP SDK (`aftertalk/aftertalk-php`)

Server-side PHP client for the Aftertalk REST API.
Covers session management, participant token generation, and webhook verification/parsing.

> For frontend WebRTC integration use the TypeScript SDK — see [sdk.md](sdk.md).

## Installation

```bash
composer require aftertalk/aftertalk-php guzzlehttp/guzzle
```

Requires: PHP 8.1+, any PSR-18 HTTP client.

## Quick start

```php
use Aftertalk\AftertalkClient;

$client = new AftertalkClient(
    baseUrl:       'https://aftertalk.yourserver.com',
    apiKey:        $_ENV['AFTERTALK_API_KEY'],
    webhookSecret: $_ENV['AFTERTALK_WEBHOOK_SECRET'],
);

// 1. Create a session (server-side only — never from the browser)
$session = $client->sessions->create(
    templateId:       'therapy',
    participantCount: 2,
    participants: [
        ['user_id' => 'doc_456', 'role' => 'terapeuta'],
        ['user_id' => 'pat_789', 'role' => 'paziente'],
    ],
    metadata: json_encode(['appointment_id' => 'appt_123']),
);

// 2. Return JWT tokens to the frontend (one per participant)
foreach ($session->participants as $p) {
    // $p->userId, $p->role, $p->token (JWT — pass to WebRTC frontend)
}

// 3. End the session when the call is over
$client->sessions->end($session->id);
```

## Per-session STT/LLM profiles

When the server has multiple provider profiles configured, you can select them at session
creation time:

```php
// Discover what the server supports
$config = $client->config->getConfig();
// $config->sttProfiles       → ['local', 'cloud']
// $config->sttDefaultProfile → 'local'
// $config->llmProfiles       → ['local', 'cloud']
// $config->llmDefaultProfile → 'local'

// Force cloud providers for this session
$session = $client->sessions->create(
    templateId:       'therapy',
    participantCount: 2,
    participants:     [...],
    sttProfile:       'cloud',   // e.g. Groq whisper-large-v3
    llmProfile:       'cloud',   // e.g. OpenRouter minimax-m2.7
);
// $session->sttProfile → 'cloud'
// $session->llmProfile → 'cloud'
```

Profiles are defined server-side in `stt.profiles` / `llm.profiles`. When omitted, the
server falls back to `stt.default_profile` / `llm.default_profile`.

## Webhook handling

Aftertalk signs every POST with HMAC-SHA256 (`X-Aftertalk-Signature: sha256=<hex>`).

### Push mode (`webhook_mode: push`)

The full minutes JSON is delivered in the request body:

```php
// routes/webhooks.php (Laravel)
Route::post('/webhooks/aftertalk', function (Request $request) use ($client) {
    $body      = $request->getContent();
    $signature = $request->header('X-Aftertalk-Signature');

    $client->webhook->verify($body, $signature); // throws on failure

    $payload = $client->webhook->parsePayload($body);
    // $payload is MinutesPayload

    $meta          = $payload->decodedMetadata(); // decoded JSON or null
    $appointmentId = $meta['appointment_id'] ?? null;

    // $payload->sessionId
    // $payload->minutes->sections['themes']
    // $payload->minutes->citations[0]->text, ->timestampMs
    // $payload->participants[0]->userId, ->role

    return response()->noContent();
});
```

### Notify-pull mode (`webhook_mode: notify_pull`)

Only a signed retrieval URL is sent — no clinical data in transit:

```php
$payload = $client->webhook->parsePayload($body);
// $payload is NotificationPayload

// $payload->retrieveUrl  — single-use URL to GET full minutes
// $payload->expiresAt    — expiry timestamp (RFC3339)
// $payload->sessionMetadata, $payload->participants

// Pull full minutes when ready
$minutes = $client->minutes->getBySession($payload->sessionId);
```

## Idempotent session creation (anti race-condition)

```php
function getOrCreateAftertalkSession(string $appointmentId, Appointment $appt): string
{
    $db->execute('INSERT OR IGNORE INTO appointment_calls (appointment_id) VALUES (?)', [$appointmentId]);
    $row = $db->fetchOne('SELECT aftertalk_session_id FROM appointment_calls WHERE appointment_id = ?', [$appointmentId]);

    if ($row['aftertalk_session_id'] === null) {
        $session = $aftertalk->sessions->create(
            templateId:       'therapy',
            participantCount: 2,
            participants:     [
                ['user_id' => $appt->doctorId,  'role' => 'terapeuta'],
                ['user_id' => $appt->patientId, 'role' => 'paziente'],
            ],
            metadata: json_encode(['appointment_id' => $appointmentId, 'doctor_id' => $appt->doctorId]),
        );
        $db->execute(
            'UPDATE appointment_calls SET aftertalk_session_id = ? WHERE appointment_id = ?',
            [$session->id, $appointmentId],
        );
        return $session->id;
    }

    return $row['aftertalk_session_id'];
}
```

## DTOs

| Class | Fields |
|-------|--------|
| `Session` | `id`, `status`, `participantCount`, `participants[]`, `templateId?`, `metadata?`, `sttProfile?`, `llmProfile?`, `createdAt`, `updatedAt`, `endedAt?` |
| `Participant` | `id`, `userId`, `role`, `token`, `connectedAt?`, `audioStreamId?` |
| `Minutes` | `id`, `sessionId`, `status`, `summary`, `sections`, `citations[]`, `version`, `generatedAt`, `templateId?`, `provider?` |
| `Citation` | `text`, `role`, `timestampMs` |
| `ParticipantSummary` | `userId`, `role` (compact, for webhook payloads) |
| `ServerConfig` | `templates[]`, `defaultTemplateId`, `sttProfiles[]`, `sttDefaultProfile?`, `llmProfiles[]`, `llmDefaultProfile?` |

## Exceptions

| Exception | When |
|-----------|------|
| `AftertalkException` | Base class (HTTP error, generic) |
| `AuthException` | HTTP 401 / 403 — invalid API key |
| `NotFoundException` | HTTP 404 — session / minutes not found |
| `WebhookSignatureException` | HMAC mismatch on incoming webhook |

## Custom HTTP client

```php
use GuzzleHttp\Client;
use GuzzleHttp\Psr7\HttpFactory;

$factory = new HttpFactory();
$client  = new AftertalkClient(
    baseUrl:        'https://...',
    apiKey:         '...',
    httpClient:     new Client(['timeout' => 10]),
    requestFactory: $factory,
    streamFactory:  $factory,
);
```

Any PSR-18 (`psr/http-client`) + PSR-17 (`psr/http-factory`) implementation works.
Without explicit injection the SDK auto-discovers via `php-http/discovery` or falls
back to Guzzle if available.

## Security checklist

- Never expose the `apiKey` to the browser or mobile clients
- Never derive `metadata` from user-controlled input
- Always verify the webhook signature before processing the payload
- Store `webhookSecret` in environment variables, not source code
