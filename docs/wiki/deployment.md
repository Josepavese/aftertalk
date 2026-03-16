# Deployment Guide

This guide covers deploying Aftertalk to a Linux server using the `deploy/` system.

---

## Prerequisites

- **Local machine**: Go 1.22+, `rsync`, `ssh`
- **Target server**: Ubuntu 22.04+ with SSH access and `sudo` privileges
- **DNS**: a domain pointing to the target (required for HTTPS/WSS)

---

## Quick Start

```bash
cp deploy/config/deploy.config.example.json deploy/config/prod.deploy.config.json
# edit prod.deploy.config.json — fill in host, credentials, API keys
cd deploy && go run . -config config/prod.deploy.config.json
```

The deployer runs an interactive TUI, streams install progress, and confirms success with smoke health checks.

---

## Config Reference

All fields live in `*.deploy.config.json`. Omitted fields use the listed defaults.

### `target` block

| Field | Type | Default | Description |
|---|---|---|---|
| `host` | string | — | SSH target, e.g. `user@192.168.1.10` |
| `port` | int | `22` | SSH port |
| `deploy_dir` | string | `/tmp/aftertalk-deploy` | Remote staging directory |

### `app` block

| Field | Type | Default | Description |
|---|---|---|---|
| `http_port` | int | `8080` | Port Aftertalk listens on. Change if occupied. |
| `tls_cert_file` | string | `""` | Absolute path to TLS cert on the target. Leave empty when behind a reverse proxy. |
| `tls_key_file` | string | `""` | Absolute path to TLS private key on the target. |
| `apache_vhost_conf` | string | `""` | Absolute path to the SSL vhost file to patch for ProxyPass. Leave empty to skip step `80-apache-proxy`. |
| `jwt_secret` | string | — | JWT signing secret |
| `api_key` | string | — | API key for `/v1/*` endpoints |
| `stt_provider` | string | `""` | `google`, `aws`, `azure`, or empty for stub |
| `llm_provider` | string | `""` | `openai`, `anthropic`, `azure`, or empty for stub |
| `webhook_url` | string | `""` | Webhook delivery URL |
| `webhook_mode` | string | `push` | `push` or `notify_pull` |

### Top-level fields

| Field | Type | Default | Description |
|---|---|---|---|
| `skip_firewall` | bool | `false` | Skip step `50-firewall`. Set to `true` on production servers with existing ufw rules. |
| `reset_steps` | []string | `[]` | List of step names (e.g. `["30-config", "40-install-binary"]`) to force re-run even if their `.ok` marker exists. |

See `deploy/config/deploy.config.example.json` for the full annotated example.

---

## Install Steps

Steps run in order. Each step is idempotent: it writes a `.ok` marker in `/opt/aftertalk/.state/install/` on success, and is skipped on subsequent runs unless explicitly reset.

| Step | Name | What it does |
|---|---|---|
| `00` | `prereqs` | Installs apt packages (`sqlite3`, `ufw`, etc.) |
| `20` | `service-user` | Creates `aftertalk` system user and directory layout under `/opt/aftertalk/` |
| `30` | `config` | Generates `/opt/aftertalk/aftertalk.yaml` from `install.env` |
| `40` | `install-binary` | Copies pre-built binary to `/opt/aftertalk/bin/aftertalk` |
| `50` | `firewall` | Adds ufw rules to allow the configured port. Skipped if `SKIP_FIREWALL=1` or `skip_firewall: true`. |
| `70` | `verify` | Post-install checks: binary executable, config present, service responds |
| `80` | `apache-proxy` | Injects ProxyPass into the SSL vhost (skipped if `apache_vhost_conf` not set) |

### Re-running individual steps

Add the step names to `reset_steps` in your config:

```json
"reset_steps": ["30-config", "40-install-binary"]
```

On the next run, those steps will re-execute regardless of their `.ok` marker.

---

## TLS Options

### Option A: Reverse proxy (Apache or nginx)

The recommended approach for most production deployments. Aftertalk listens on plain HTTP; the proxy handles TLS termination.

Leave `tls_cert_file` and `tls_key_file` empty in your deploy config.

**Apache example** — the install step `80-apache-proxy` generates and activates this automatically when `apache_vhost_conf` is set:

```apache
# /etc/apache2/conf-available/aftertalk-proxy.conf
<Location /aftertalk/>
    ProxyPass        http://127.0.0.1:8080/
    ProxyPassReverse http://127.0.0.1:8080/
</Location>

<Location /aftertalk/signaling>
    ProxyPass        ws://127.0.0.1:8080/signaling
    ProxyPassReverse ws://127.0.0.1:8080/signaling
</Location>
```

The step appends a single `Include /etc/apache2/conf-available/aftertalk-proxy.conf` line to the `:443` block of the target vhost. It is idempotent — running again will not add duplicate lines. The existing vhost is never overwritten.

**Client URLs:**
```
https://yourdomain.com/aftertalk/...
wss://yourdomain.com/aftertalk/signaling?token=eyJ...
```

**To remove:** delete `aftertalk-proxy.conf` and remove the `Include` line from the vhost, then reload Apache.

### Option B: Native TLS (standalone, no proxy)

Set the TLS fields in your deploy config:

```json
"app": {
  "tls_cert_file": "/etc/aftertalk/certs/cert.pem",
  "tls_key_file":  "/etc/aftertalk/certs/key.pem"
}
```

This writes the following to `aftertalk.yaml`:

```yaml
tls:
  cert_file: /etc/aftertalk/certs/cert.pem
  key_file:  /etc/aftertalk/certs/key.pem
```

Behavior at startup:
- Both files exist → server starts HTTPS/WSS (`ListenAndServeTLS`).
- Files missing → server exits with an explicit error. It never silently falls back to HTTP.
- Fields empty → plain HTTP/WS.

**Client URLs:**
```
https://yourdomain.com/...
wss://yourdomain.com/signaling?token=eyJ...
```

---

## Firewall Notes

Step `50-firewall` only **adds** ufw rules. It never resets or modifies existing rules, so it is safe on servers with pre-existing firewall configuration.

To skip it entirely (e.g. you manage firewall rules externally):

```json
"skip_firewall": true
```

Or set `SKIP_FIREWALL=1` in `install.env` before running the installer manually.

---

## Smoke Health Verification

After install, the deployer calls:

```
GET https://yourdomain.com/v1/health  → {"status":"ok"}
GET https://yourdomain.com/v1/ready   → {"status":"ready"}
```

If either check fails, the TUI reports the failure and exits non-zero. The service may still be running — check `journalctl -u aftertalk` on the target.

---

## Directory Layout (on target)

```
/opt/aftertalk/
├── bin/aftertalk          # binary
├── aftertalk.yaml         # generated config (from install.env)
├── aftertalk.db           # SQLite database
├── .state/
│   └── install/           # .ok markers per step
└── logs/                  # optional log output
```

The `aftertalk` systemd service runs as the `aftertalk` system user with `WorkingDirectory=/opt/aftertalk`.
