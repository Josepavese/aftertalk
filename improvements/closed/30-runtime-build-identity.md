# 30 - Runtime build identity and deploy verification

## Status

Implemented in May 2026.

## Context

During a downstream production update on 2026-05-04, the installed Aftertalk binary was upgraded from an older build to the current rolling `edge` release.

The upgrade could be verified only by comparing file metadata:

- old binary mtime: 2026-03-25
- old binary sha256: different from the current release asset
- new release tag: `edge`
- new release commit: `e68b0fbed27c44aeb1eef855d1d6aa6c24dbaef5`
- new binary sha256: matching the downloaded `edge` asset

However, the runtime health endpoint returned the same application version before and after the upgrade:

```json
{"status":"ok","version":"1.0.0"}
```

This happens because `internal/version/version.txt` is embedded at build time and currently remains fixed at `1.0.0`. The release workflow builds binaries with:

```bash
go build -trimpath -ldflags="-s -w"
```

No commit SHA, release tag, build timestamp, or build channel is injected into the binary.

## Problem

The runtime does not expose a unique build identity.

This makes production deploy validation unnecessarily fragile:

- `/v1/health` cannot prove which binary is running.
- Rolling `edge` releases are indistinguishable at runtime when `version.txt` does not change.
- Operators must compare filesystem hashes or mtimes on the server.
- Support/debug reports cannot reliably correlate a running instance with a Git commit.
- Rollback verification is harder because both old and new binaries may report the same semantic version.

The issue is not limited to `edge`: any release process that forgets to bump `version.txt` will produce a binary that reports stale version metadata.

## Root Cause

The current version model conflates product/API semantic version with deployable build identity.

`version.Current` answers only "what semantic version is embedded in `version.txt`", but operationally we also need to answer:

- which Git commit produced this binary?
- which release tag/channel was used (`edge`, `vX.Y.Z`, manual build)?
- when was the binary built?
- is the running binary from a clean CI build or a local/dev build?

## Proposed Fix

Introduce explicit build metadata in `internal/version`.

Suggested fields:

- `Version`: semantic version from `version.txt`
- `Commit`: Git SHA injected at build time
- `Tag`: release tag or channel (`edge`, `vX.Y.Z`, `dev`)
- `BuildTime`: UTC build timestamp
- `BuildSource`: optional source (`github-actions`, `local`, `unknown`)

The release workflow should inject these values via `-ldflags -X`, for example:

```bash
go build -trimpath \
  -ldflags="-s -w \
    -X github.com/Josepavese/aftertalk/internal/version.Commit=${GITHUB_SHA} \
    -X github.com/Josepavese/aftertalk/internal/version.Tag=${TAG} \
    -X github.com/Josepavese/aftertalk/internal/version.BuildTime=${BUILD_TIME} \
    -X github.com/Josepavese/aftertalk/internal/version.BuildSource=github-actions" \
  -o "$output" ./cmd/aftertalk
```

For local builds, defaults should be explicit and non-misleading:

```json
{
  "version": "1.0.0",
  "commit": "dev",
  "tag": "dev",
  "build_time": "",
  "build_source": "local"
}
```

## API Contract

Keep `/v1/health` lightweight, but include enough metadata to verify a deploy.

Recommended response:

```json
{
  "status": "ok",
  "version": "1.0.0",
  "commit": "e68b0fbed27c44aeb1eef855d1d6aa6c24dbaef5",
  "tag": "edge",
  "build_time": "2026-04-13T21:32:00Z"
}
```

Alternative: add `GET /v1/version` and keep `/v1/health` unchanged. If this path is chosen, deploy scripts must use `/v1/version` for post-install verification.

## CLI Contract

Add a non-server version command:

```bash
aftertalk --version
```

Expected output should include at least:

```text
aftertalk 1.0.0 edge e68b0fbed27c44aeb1eef855d1d6aa6c24dbaef5
```

This allows deploy scripts and operators to verify the binary before starting/restarting the service.

## Installer / Deploy Impact

Installer and deploy wrappers should log the runtime build identity after restart.

Recommended verification flow:

1. Download release asset.
2. Record downloaded asset sha256.
3. Install binary atomically.
4. Restart service.
5. Query `/v1/health` or `/v1/version`.
6. Fail deploy if the runtime commit/tag does not match the requested release.

For rolling `edge`, the deploy script should resolve the remote tag SHA before install and compare it against runtime `commit` after restart.

## Acceptance Criteria

- Release binaries expose semantic version, commit, tag/channel, and build time.
- `/v1/health` or `/v1/version` returns the same build metadata.
- `aftertalk --version` returns the same build metadata without starting the server.
- GitHub release workflow injects metadata for both server and installer binaries.
- Local builds have clear `dev`/`local` defaults, never stale CI metadata.
- Deploy verification can assert that the running process matches the requested release.
- Tests cover default build metadata and HTTP serialization.

## Priority

High for operational reliability.

The product can run correctly without this, but production support cannot conclusively prove which build is serving traffic through the application API.
