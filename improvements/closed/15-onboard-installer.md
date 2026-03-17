# Improvement 15 — Standalone Onboard Installer

## Status: closed

## Problem

The current deployment story has a split personality:

- `deploy/` (SSH orchestrator + Python install agent) — works but requires Python on the target,
  is not PAL, and is not runnable without the SSH orchestrator.
- `scripts/install.sh` — exists but only handles binary download, not server configuration.

The install steps (`deploy/install/*.sh`) are idempotent and self-contained, but they can only
be triggered remotely via the Python HTTP agent on port 9977. There is no way to run them
directly on the target without the orchestrator.

## Proposed Architecture

Two independent, composable systems:

```
┌─────────────────────────────────────┐   SSH   ┌──────────────────────────────────────┐
│  SSH Orchestrator (local, Go)       │ ──────► │  Onboard Installer (target, Go)      │
│  deploy/                            │         │  /opt/aftertalk/installer            │
│  - preflight TUI                    │         │  - interactive SSOT setup            │
│  - cross-compile + rsync            │         │  - idempotent steps                  │
│  - launch installer via SSH         │         │  - runnable standalone               │
│  - stream progress                  │         │  - HTTP API for remote orchestration │
└─────────────────────────────────────┘         └──────────────────────────────────────┘
                                                          ▲
                                                          │ also callable directly
                                                 aftertalk-installer --configure
```

## Tasks

### 1. Convert Python install agent → Go

Replace `deploy/install_agent.py` (Python HTTP agent on port 9977) with a compiled Go binary.

**Motivation:**
- Python dependency on the target is not guaranteed (and is a runtime failure mode).
- Go binary is self-contained, cross-platform (PAL), and can be cross-compiled locally and
  rsynced to the target along with the aftertalk binary.
- A Go agent can share types with the orchestrator (same module or separate cmd).

**Interface to preserve** (backwards-compatible):
- `GET /status` — running/idle
- `GET /events` — SSE stream of install log lines
- `POST /run` — trigger `install/run.sh`, stream output via `/events`

**Suggested location**: `cmd/installer/main.go` (separate `go build` target).

### 2. Make onboard installer standalone

Currently `install/run.sh` is only invoked by the remote agent. It should also be runnable
directly on the target without any orchestrator involvement.

**Changes:**
- Add an interactive mode: `aftertalk-installer --configure` prompts for all SSOT values
  and writes `install.env` locally (no SSH needed).
- Add a non-interactive mode: `aftertalk-installer --env /path/to/install.env` reads the
  env file and runs all steps (same as remote agent, but invoked locally).
- Both modes run the same idempotent step scripts.

**Interactive prompts (SSOT):**
- HTTP port (default: 8080)
- API key
- JWT secret + expiry
- STT provider + credentials
- LLM provider + API key + model
- Webhook URL + mode (push / notify_pull) + secret + pull base URL
- TLS: cert file + key file (optional; if set, enables native HTTPS)
- Apache proxy: vhost conf path (optional; if set, injects ProxyPass idempotently)

### 3. Add binary installer (like `scripts/install.sh`)

Create `scripts/install-server.sh` — a one-liner that:
1. Downloads the latest `aftertalk-linux-amd64` binary from GitHub Releases.
2. Downloads the `aftertalk-installer` binary.
3. Places both in `/usr/local/bin/` (or `~/.aftertalk/bin/`).
4. Optionally runs `aftertalk-installer --configure` interactively.

This gives operators a path to install and configure Aftertalk on a fresh server without
any local tooling or SSH orchestrator.

```bash
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install-server.sh | bash
```

### 4. Update SSH orchestrator to use Go installer

Once the Go installer binary exists, update `deploy/deploy.go`:
- Replace `python3 install_agent.py` launch with `./aftertalk-installer --agent` (HTTP mode).
- Cross-compile `cmd/installer` as part of the build step (alongside the main binary).
- rsync both binaries to the target.

## Out of Scope

- The SSH orchestrator (`deploy/deploy.go`) itself is intentionally NOT published (gitignored,
  contains secrets). The onboard installer is the public-facing artifact.
- Windows / macOS server installs — the installer targets Linux servers only (systemd, Apache).
  Cross-platform here means: the installer binary itself compiles on any OS, but its steps
  target Linux.

## Acceptance Criteria

- [ ] `aftertalk-installer --configure` runs interactively on a fresh Ubuntu 24.04 server
      with no Python, no SSH orchestrator, and produces a working Aftertalk installation.
- [ ] `aftertalk-installer --env install.env` is idempotent (re-running skips completed steps).
- [ ] `deploy/deploy.go` uses the Go installer instead of Python agent.
- [ ] `scripts/install-server.sh` bootstraps the full installation in one curl | bash.
