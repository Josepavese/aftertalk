# Aftertalk

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-22c55e)
<!-- CI and Release badges: re-enable when repo is public -->
<!-- ![CI](https://github.com/Josepavese/aftertalk/actions/workflows/ci.yml/badge.svg?branch=master) -->
<!-- ![Release](https://img.shields.io/github/v/release/Josepavese/aftertalk?color=6366f1) -->

**WebRTC session recorder → structured AI minutes, delivered via webhook.**

> [!WARNING]
> **Alpha pre-release.** This project is under active development. APIs may change without notice and correct behaviour is not guaranteed. Not recommended for production use.

<!-- TODO: replace with real screenshot → docs/assets/demo-screenshot.png -->
![Aftertalk Demo](https://placehold.co/1200x630/0f172a/6366f1?font=montserrat&text=demo+screenshot+here)

---

Aftertalk sits alongside your WebRTC calls. It captures audio server-side,
transcribes with STT (Whisper · Google · AWS · Azure), generates structured
minutes with an LLM (OpenAI · Anthropic · Ollama), and delivers them to your
backend via webhook — all without storing audio.

Minutes generation is incremental: the server reduces the transcript batch by batch,
so smaller local models can work without seeing the whole conversation in one request.

> No audio is ever persisted. Minutes are always editable. Humans stay in the loop.

---

## Quick Start

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash

# Configure
nano ~/.aftertalk/config/config.yaml
# set: api.key, jwt.secret, stt.provider, llm.provider
# for OpenRouter/reasoning models, configure llm.profiles.<name>.request_timeout,
# llm.profiles.<name>.generation_timeout, and retry budgets

# Start
aftertalk start
```

Demo UI at `http://localhost:8080` · [Full installation guide](docs/wiki/installation.md) · [LLM timeout profiles](docs/wiki/configuration.md#llm-timeout-budget-cookbook)

---

## Integrate

The canonical pattern: **PHP backend holds the API key**, browser receives only a short-lived JWT room token.

<!-- TODO: replace with real diagram → docs/assets/architecture.png -->
![Architecture](https://placehold.co/900x200/0f172a/6366f1?font=montserrat&text=Browser+%2B+TS+SDK+%E2%86%92+PHP+Backend+%E2%86%92+Aftertalk+Server+%E2%86%92+Webhook)

**PHP backend** (privileged — API key stays here):

```php
$result = $aftertalk->rooms->join(
    code: $appointment->id,   // room code — idempotent, safe to call twice
    name: $user->displayName,
    role: 'therapist',        // determined server-side from your auth
);
// return $result['token'] to the browser — never the API key
```

**TypeScript frontend** (JWT only — no API key):

```typescript
const sdk = new AftertalkClient({ baseUrl: window.location.origin });
const conn = await sdk.connectWebRTC({ sessionId, token }); // token from PHP
conn.on('connected', () => console.log('streaming audio'));
```

→ [Full integration guide](docs/wiki/integration-guide.md)

---

## SDKs

| SDK | Install | Use case |
|-----|---------|----------|
| **TypeScript** [`@aftertalk/sdk`](sdk/ts/) | `npm i @aftertalk/sdk` | Browser — WebRTC streaming, minutes polling |
| **PHP** [`aftertalk/aftertalk-php`](sdk/php/) | `composer require aftertalk/aftertalk-php` | Server — sessions, webhook verification |

---

## Documentation

| | |
|--|--|
| [Installation](docs/wiki/installation.md) | Requirements, install modes (`local-ai` · `cloud` · `offline`), first run |
| [Configuration](docs/wiki/configuration.md) | All parameters with defaults, including LLM profile timeout/retry SSOT |
| [REST API](docs/wiki/rest-api.md) | Every endpoint with curl examples |
| [Integration Guide](docs/wiki/integration-guide.md) | PHP + TS full workflow, security model, race conditions |
| [Webhook](docs/wiki/webhook.md) | Push vs `notify_pull`, HMAC verification |
| [Templates](docs/wiki/templates.md) | `therapy`, `consulting`, custom session structures |
| [Architecture](docs/wiki/architecture.md) | Internal audio → minutes pipeline |
| [Tone of Voice](docs/style/tone-of-voice.md) | Editorial style for docs and project messaging |

---

MIT · [Josepavese](https://github.com/Josepavese)
