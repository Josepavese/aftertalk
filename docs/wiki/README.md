# Aftertalk Wiki

Operational documentation verified against source code.

## Table of Contents

| Page | Contents |
|---|---|
| [installation.md](installation.md) | Requirements, installer, first run |
| [configuration.md](configuration.md) | All configuration parameters with real defaults |
| [rest-api.md](rest-api.md) | Every endpoint with complete curl examples |
| [sdk.md](sdk.md) | TypeScript SDK: quickstart, WebRTC, polling |
| [webhook.md](webhook.md) | Push vs notify_pull, HMAC verification, examples |
| [templates.md](templates.md) | Session templates: built-in and custom |
| [architecture.md](architecture.md) | Internal flows, audio→minutes pipeline |

---

## In 30 seconds

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash

# Configure (copy template)
cp .env.example .env && nano .env   # set JWT_SECRET, API_KEY, LLM_PROVIDER

# Start
./bin/aftertalk
```

The demo UI is available at `http://localhost:8080` if `AFTERTALK_DEMO_ENABLED=true`.
