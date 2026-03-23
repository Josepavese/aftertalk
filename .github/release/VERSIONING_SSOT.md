# Versioning SSOT

Canonical source of truth for versioning and release rules in this repository.

## Scope

This SSOT governs SDK release versioning (`sdk/ts`, `sdk/php`) and release hygiene.

## Canonical Files

- `./.github/release-please/config.json`
- `./.github/release-please/manifest.json`
- `./sdk/ts/package.json`
- `./sdk/php/composer.json`
- `./docs/style/tone-of-voice.md` (release copy style)

If version information conflicts across files, resolve in this order:

1. `manifest.json`
2. package metadata (`package.json`, `composer.json`)
3. changelog entries

## Version Policy (SemVer Constraint)

Default rule: bump `x.y.z` by increasing **only `z`**.

- Allowed by default: `x.y.(z+1)`
- Forbidden by default: `x+1.*.*`, `x.(y+1).*`
- `x` or `y` can change only with explicit maintainer instruction.

## Release Hygiene Rules

Releases must not include AI traces.

- Do not include AI co-author trailers in commit messages.
- Ignore `.agent/` and `.claude/` from release scope.
- Never stage workflow-local helper files from `.agent/`.

Pre-release checks (mandatory):

1. `go test ./...`
2. SDK checks when relevant: `cd sdk/ts && npm test` (if TS changed)
3. SDK checks when relevant: `cd sdk/php && composer test` or package checks (if PHP changed)
4. Working tree must be clean for release-related files.
5. All completed changes must be committed before release.
6. Never release partial/incomplete work.

Post-release (mandatory):

7. Monitor GitHub Actions until all release-related workflows are green.
8. On failure: troubleshoot -> implement fix -> re-test -> commit -> release again.
9. Repeat until full green status is reached.

## AI-Trace Guard Commands

```bash
# 1) No AI co-author trailers in current history slice
 git log --format='%H%n%B%n---' -n 200 | rg -n -i 'co-authored-by:.*(claude|ai|anthropic)'

# 2) Ensure .agent/.claude are not staged
 git diff --cached --name-only | rg -n '^(\.agent/|\.claude/)'
```

Both commands must return no matches before release.

## Tone of Voice for Release Notes

Release messages and notes must follow:

- `./docs/style/tone-of-voice.md`

Use direct, technical, verifiable language. Avoid hype and generic marketing phrasing.
