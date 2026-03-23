# Improvement 23: Distribuzione automatizzata degli SDK via GitHub Actions

## Stato: PARZIALMENTE IMPLEMENTATO

**Implementato (marzo 2026)**: TS SDK + PHP SDK workflows, release-please, fix ci.yml
**Bloccato**: Go SDK workflow → dipende da improvement #22 (Go SDK non ancora creato)

---

## Checklist di Implementazione

- [x] `sdk/ts/` al path corretto (era già presente)
- [x] `sdk/php/` con `composer.json` valido (era già presente, aggiunto `scripts.test`)
- [ ] `sdk/go/` con `go.mod` — **bloccato da improvement #22**
- [x] `.github/release-please/config.json`
- [x] `.github/release-please/manifest.json`
- [x] `.github/workflows/release-please.yml`
- [x] `.github/workflows/sdk-ts-publish.yml`
- [x] `.github/workflows/sdk-php-publish.yml`
- [ ] `.github/workflows/sdk-go-tag.yml` — **bloccato da improvement #22**
- [ ] Account npm `@aftertalk` + `NPM_TOKEN` GitHub Secret — **azione manuale richiesta**
- [ ] Registrazione Packagist + `PACKAGIST_USERNAME` + `PACKAGIST_API_TOKEN` — **azione manuale richiesta**
- [ ] CONTRIBUTING.md con processo Conventional Commits
- [ ] Test end-to-end con pre-release `v1.0.0-rc.1`

---

## Azioni manuali richieste (credenziali)

### 1. npm — `NPM_TOKEN`

```
1. Vai su npmjs.com → login/crea account
2. Crea organizzazione con scope @aftertalk (se non esiste)
3. Account → Access Tokens → Generate New Token → tipo "Automation"
4. GitHub repo → Settings → Secrets and variables → Actions
5. New secret: NPM_TOKEN = <token>
```

Il workflow `sdk-ts-publish.yml` si attiva automaticamente al tag `sdk/ts/v*`
e pubblica `@aftertalk/sdk` su npm con provenance SLSA Level 2.

### 2. Packagist — `PACKAGIST_USERNAME` + `PACKAGIST_API_TOKEN`

```
1. Vai su packagist.org → crea account "aftertalk"
2. Submit → URL: https://github.com/Josepavese/aftertalk
3. Nella configurazione mono-repo, imposta Package root path: sdk/php
4. Profile → API Tokens → crea token
5. GitHub Secrets:
   PACKAGIST_USERNAME = aftertalk
   PACKAGIST_API_TOKEN = <token>
```

Il workflow `sdk-php-publish.yml` valida il package, esegue PHPUnit,
poi notifica Packagist via API. Packagist re-indicizza entro ~5 minuti.

---

## File creati

### `.github/release-please/config.json`

Configura release-please per due package nel mono-repo:

```json
{
  "separate-pull-requests": true,
  "packages": {
    "sdk/ts":  { "release-type": "node", "tag-name": "sdk/ts/v${version}" },
    "sdk/php": { "release-type": "php",  "tag-name": "sdk/php/v${version}" }
  }
}
```

Quando viene mergiato un commit `feat:` o `fix:` su master, release-please
apre automaticamente una PR "Release: sdk/ts v1.1.0" con CHANGELOG aggiornato.
Al merge della PR crea il tag → il workflow di pubblicazione si attiva.

### `.github/workflows/release-please.yml`

Gira su ogni push a master. Crea/aggiorna le PR di rilascio per SDK.
Usa solo `GITHUB_TOKEN` (automatico, zero setup).

### `.github/workflows/sdk-ts-publish.yml`

Trigger: tag `sdk/ts/v*`
Steps: typecheck → test (vitest) → `npm version` → build (tsup) → `npm publish --provenance`
Richiede: `NPM_TOKEN` in GitHub Secrets

### `.github/workflows/sdk-php-publish.yml`

Trigger: tag `sdk/php/v*`
Steps: `composer validate --strict` → install → PHPUnit → notifica Packagist API
Richiede: `PACKAGIST_USERNAME` + `PACKAGIST_API_TOKEN` in GitHub Secrets

---

## Bug corretto: `ci.yml` branch `main` → `master`

Il CI esistente ascoltava su `main` e `develop`, ma il repo usa `master`.
Corretto in entrambi i trigger (`push.branches` e `pull_request.branches`)
e nella condizione `build-and-push` (`refs/heads/main` → `refs/heads/master`).

---

## Workflow end-to-end (quando le credenziali sono configurate)

```
1. Developer: git commit -m "feat(sdk/ts): add streaming minutes support"
2. Push su master → release-please.yml
3. release-please apre PR "Release: sdk/ts v1.1.0" con CHANGELOG
4. Maintainer approva e mergia la PR
5. release-please crea tag sdk/ts/v1.1.0
6. sdk-ts-publish.yml: typecheck → test → build → npm publish
7. @aftertalk/sdk@1.1.0 su npmjs.com entro ~1 minuto
```

---

## Cosa resta da fare (dipendenze esterne)

| Item | Dipendenza |
|------|-----------|
| `sdk-go-tag.yml` | Improvement #22 (Go SDK) |
| `sdk/go` in `.github/release-please/config.json` | Improvement #22 |
| `NPM_TOKEN` attivo | Azione manuale npmjs.com |
| Packagist registrato | Azione manuale packagist.org |

Quando improvement #22 è completato, aggiungere in `.github/release-please/config.json`:
```json
"sdk/go": {
  "release-type": "go",
  "tag-name": "sdk/go/v${version}"
}
```
E creare `.github/workflows/sdk-go-tag.yml` seguendo la specifica originale in questo file.

---

## Specifica originale completa

→ Vedi git history per il testo originale dell'improvement (ante marzo 2026).
