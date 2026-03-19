# aftertalk/aftertalk-php

PHP SDK for the [Aftertalk](https://github.com/Josepavese/aftertalk) REST API.

Covers the **server-side** integration path: creating sessions, generating participant tokens,
ending sessions, and verifying/parsing webhook deliveries.

> For frontend WebRTC integration use the TypeScript SDK `@aftertalk/sdk`.

## Requirements

- PHP 8.1+
- Any PSR-18 HTTP client (`guzzlehttp/guzzle`, `symfony/http-client`, etc.)

## Installation

```bash
composer require aftertalk/aftertalk-php guzzlehttp/guzzle
```

## Quick start

```php
use Aftertalk\AftertalkClient;

$client = new AftertalkClient(
    baseUrl:       'https://aftertalk.yourserver.com',
    apiKey:        $_ENV['AFTERTALK_API_KEY'],
    webhookSecret: $_ENV['AFTERTALK_WEBHOOK_SECRET'],
);

// 1. Create a session
$session = $client->sessions->create(
    templateId:       'therapy',
    participantCount: 2,
    participants: [
        ['user_id' => 'doc_456', 'role' => 'terapeuta'],
        ['user_id' => 'pat_789', 'role' => 'paziente'],
    ],
    metadata:   json_encode(['appointment_id' => 'appt_123']),
    sttProfile: 'cloud',   // optional — falls back to server default
    llmProfile: 'cloud',   // optional — falls back to server default
);

// $session->id, $session->templateId, $session->sttProfile, $session->createdAt

// 2. Hand the JWT token to the frontend (never log or expose to other parties)
$therapistToken = collect($session->participants)
    ->firstWhere('userId', 'doc_456')
    ->token;

// 3. End the session (triggers transcription + minute generation)
$client->sessions->end($session->id);
```

## API Reference

### `$client->sessions`

| Method | Description |
|--------|-------------|
| `create(templateId, participantCount, participants, metadata?, sttProfile?, llmProfile?)` | Create session, returns `Session` DTO |
| `get(sessionId)` | Get session by ID |
| `list(status?, limit?, offset?)` | List sessions (returns `Session[]`) |
| `end(sessionId)` | End session — idempotent |
| `delete(sessionId)` | Delete session (must be ended first) |

### `$client->minutes`

| Method | Description |
|--------|-------------|
| `getBySession(sessionId)` | Get minutes for a session |
| `get(minutesId)` | Get minutes by ID |
| `update(minutesId, sections, notes?, userId?)` | Update sections (saves to history) |
| `getVersions(minutesId)` | Get edit history |
| `delete(minutesId)` | Delete minutes |

### `$client->transcriptions`

| Method | Description |
|--------|-------------|
| `listBySession(sessionId, limit?, offset?)` | List transcriptions |

### `$client->config`

| Method | Description |
|--------|-------------|
| `getConfig()` | Returns templates, STT/LLM profile names and defaults |

### `$client->webhook`

| Method | Description |
|--------|-------------|
| `verify(body, signatureHeader)` | Throws `WebhookSignatureException` on failure |
| `verifySignature(body, signatureHeader)` | Returns bool |
| `parsePayload(body)` | Returns `MinutesPayload` or `NotificationPayload` |

## Provider profiles (STT/LLM)

When the server is configured with multiple provider profiles (e.g. `local` and `cloud`),
you can select them per session:

```php
// Discover available profiles
$config = $client->config->getConfig();
// $config->sttProfiles       → ['local', 'cloud']
// $config->sttDefaultProfile → 'local'

// Use cloud providers for this session
$session = $client->sessions->create(
    templateId:       'therapy',
    participantCount: 2,
    participants:     [...],
    sttProfile:       'cloud',   // Groq whisper-large-v3
    llmProfile:       'cloud',   // OpenRouter / minimax
);
```

## Webhook verification

```php
// Laravel example
use Aftertalk\Webhook\MinutesPayload;
use Aftertalk\Webhook\NotificationPayload;

Route::post('/webhooks/aftertalk', function (Request $request) use ($client) {
    $body      = $request->getContent();
    $signature = $request->header('X-Aftertalk-Signature');

    // Throws WebhookSignatureException on failure
    $client->webhook->verify($body, $signature);

    $payload = $client->webhook->parsePayload($body);

    if ($payload instanceof MinutesPayload) {
        $meta          = $payload->decodedMetadata();
        $appointmentId = $meta['appointment_id'] ?? null;

        // Store / notify
        dispatch(new ProcessAftertalkMinutes(
            sessionId:     $payload->sessionId,
            appointmentId: $appointmentId,
            minutes:       $payload->minutes,
        ));
    }

    if ($payload instanceof NotificationPayload) {
        // notify_pull mode: pull later via $payload->retrieveUrl
        dispatch(new PullAftertalkMinutes($payload->retrieveUrl, $payload->expiresAt));
    }

    return response()->noContent();
});
```

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

## Security

- **Never** expose the API key to the browser or mobile clients.
- **Never** set `metadata` from user-controlled input (it is stored verbatim and sent in webhooks).
- Always verify the webhook signature before processing the payload.

## Testing

```bash
composer install
vendor/bin/phpunit
```
