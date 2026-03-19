# Improvement 23: Distribuzione automatizzata degli SDK via GitHub Actions

## Stato: APERTO

---

## Contesto e Motivazione

Aftertalk dispone di tre SDK ufficiali che coprono ecosistemi di linguaggi diversi:

| SDK | Package name | Registry | Directory attuale |
|-----|-------------|----------|-------------------|
| TypeScript | `@aftertalk/sdk` | npm | `sdk/` (mono-repo) |
| PHP | `aftertalk/aftertalk-php` | Packagist/Composer | da creare (improvement 13) |
| Go | modulo Go pubblico | pkg.go.dev / GitHub | da creare |

Oggi **nessuno dei tre SDK viene pubblicato automaticamente**. La pipeline CI esistente
(`.github/workflows/ci.yml` e `.github/workflows/release.yml`) si occupa esclusivamente
del server Go: build binari cross-platform, Docker, GitHub Release. Gli SDK sono esclusi da ogni
automazione.

Questo crea tre rischi operativi concreti:

1. **Dimenticanza di pubblicazione**: una nuova versione del server viene taggata ma l'SDK
   TypeScript rimane alla `1.0.0` su npm, con API non aggiornate.
2. **Deriva di versione**: ogni SDK segue un ciclo di release manuale diverso, senza
   corrispondenza garantita con la versione del server.
3. **Sicurezza**: i token di pubblicazione (NPM_TOKEN, Packagist API key) finiscono in
   `.env` locali degli sviluppatori invece di risiedere esclusivamente nei GitHub Secrets.

L'obiettivo di questo improvement è definire una strategia completa di distribuzione automatizzata:
scelta del layout di repository, pipeline GitHub Actions per ciascun registro, strategia di
versioning e gestione dei segreti.

---

## Raccomandazione: Strategia di Repository

### Opzione A — Mono-repo con tag per-SDK (RACCOMANDATA)

Tutti e tre gli SDK vivono nel repository principale (`github.com/Josepavese/aftertalk`),
ciascuno in una propria sotto-directory. Il trigger di pubblicazione è un tag Git con prefisso:

```
sdk/ts/v1.2.0      → pubblica @aftertalk/sdk@1.2.0 su npm
sdk/php/v1.2.0     → pubblica aftertalk/aftertalk-php@1.2.0 su Packagist
sdk/go/v1.2.0      → tag del modulo Go (pkg.go.dev lo indicizza automaticamente)
```

**Vantaggi rispetto a repo separati:**

- Un solo luogo per aprire issue, PRs e code review che coinvolgono server + SDK insieme
- Le modifiche API del server e l'aggiornamento degli SDK possono stare nello stesso commit/PR
- Un solo `CHANGELOG.md` coordinato (o tre sezioni separate)
- I GitHub Secrets sono in un solo repository
- Non serve sincronizzare più repository quando si aggiorna un tipo (es. `Minutes.Sections`)
- GitHub Actions `on.push.paths` permette di eseguire solo il workflow rilevante in base
  ai file modificati

**Svantaggi:**

- I consumatori degli SDK ricevono notifiche GitHub da un repository più "rumoroso" che include
  anche il server Go
- Il Go module path diventa `github.com/Josepavese/aftertalk/sdk/go` (path lungo) oppure
  il sotto-modulo va in `sdk/go/go.mod` con module path autonomo — entrambi gestibili
- Packagist richiede un `composer.json` in root o un path esplicito: con path `sdk/php/`
  è necessario configurare il VCS repository su Packagist oppure usare un webhook

**Alternativa scartata — Repository separati:**

Repository indipendenti (`aftertalk-ts-sdk`, `aftertalk-php-sdk`, `aftertalk-go-sdk`) semplificano
il Go module path e l'indicizzazione Packagist, ma frammentano il contesto di sviluppo.
Ogni fix cross-layer richiede PR in due repo, e la sincronizzazione dei tipi API diventa
manuale. Il guadagno tecnico non giustifica l'overhead operativo nella fase attuale del progetto.

---

## Struttura Directory Consigliata

```
aftertalk/
├── sdk/
│   ├── ts/                    # @aftertalk/sdk (TypeScript)
│   │   ├── package.json
│   │   ├── tsup.config.ts
│   │   └── src/
│   ├── php/                   # aftertalk/aftertalk-php (Composer)
│   │   ├── composer.json
│   │   └── src/
│   └── go/                    # aftertalk-go (Go module)
│       ├── go.mod             # module github.com/Josepavese/aftertalk/sdk/go
│       └── aftertalk.go
├── .github/
│   └── workflows/
│       ├── ci.yml             # esistente — test server Go
│       ├── release.yml        # esistente — build binari
│       ├── sdk-ts-publish.yml # NUOVO — pubblica su npm
│       ├── sdk-php-publish.yml# NUOVO — pubblica su Packagist
│       └── sdk-go-tag.yml     # NUOVO — valida e crea tag Go module
└── ...
```

> Nota: attualmente `sdk/` contiene direttamente il codice TypeScript (senza sotto-directory `ts/`).
> La migrazione verso `sdk/ts/` è opzionale ma consigliata per coerenza prima di aggiungere
> PHP e Go. In alternativa, mantenere `sdk/` per il TS e aggiungere `sdk/php/` e `sdk/go/`.

---

## Strategia di Versioning

### Semantic Versioning obbligatorio

Tutti gli SDK seguono [semver](https://semver.org/):
`MAJOR.MINOR.PATCH` con `-alpha.N`, `-beta.N`, `-rc.N` per le pre-release.

### Disaccoppiamento server/SDK

La versione degli SDK **non deve essere accoppiata** alla versione del server. Il server
può rilasciare la `v2.3.0` senza che gli SDK debbano cambiare major version, a meno che
non ci siano breaking change nell'API pubblica.

### Strumento raccomandato: `release-please`

[`release-please`](https://github.com/googleapis/release-please) di Google è lo strumento
più adatto per questo scenario perché:

- Supporta mono-repo con package multipli (`release-please-config.json`)
- Genera automaticamente il `CHANGELOG.md` per sezione da Conventional Commits
- Crea PR di rilascio che aggiornano `package.json`, `composer.json` e file di versione Go
- Si integra nativamente con GitHub Actions
- Produce tag Git (`sdk/ts/v1.2.0`) pronti per triggerare i workflow di pubblicazione

Configurazione `release-please-config.json` alla root:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    "sdk/ts": {
      "release-type": "node",
      "package-name": "@aftertalk/sdk",
      "changelog-path": "sdk/ts/CHANGELOG.md",
      "tag-name": "sdk/ts/v${version}"
    },
    "sdk/php": {
      "release-type": "php",
      "package-name": "aftertalk/aftertalk-php",
      "changelog-path": "sdk/php/CHANGELOG.md",
      "tag-name": "sdk/php/v${version}"
    },
    "sdk/go": {
      "release-type": "go",
      "package-name": "aftertalk-go",
      "changelog-path": "sdk/go/CHANGELOG.md",
      "tag-name": "sdk/go/v${version}"
    }
  },
  "separate-pull-requests": true
}
```

File `.release-please-manifest.json` (traccia la versione corrente per ogni package):

```json
{
  "sdk/ts": "1.0.0",
  "sdk/php": "0.1.0",
  "sdk/go": "0.1.0"
}
```

### Conventional Commits

Gli sviluppatori devono usare il formato Conventional Commits nel messaggio di commit.
`release-please` usa questi prefissi per determinare il tipo di bump:

| Prefisso commit | Bump semver | Esempio |
|----------------|-------------|---------|
| `feat:` | MINOR | `feat(sdk/ts): add streaming minutes support` |
| `fix:` | PATCH | `fix(sdk/php): correct webhook HMAC verification` |
| `feat!:` o `BREAKING CHANGE:` | MAJOR | `feat(sdk/go)!: remove deprecated Session.Roles field` |
| `chore:`, `docs:`, `test:` | nessun bump | — |

---

## Pipeline GitHub Actions

### 1. release-please — PR di rilascio automatiche

```yaml
# .github/workflows/release-please.yml
name: Release Please

on:
  push:
    branches: [master]

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
          token: ${{ secrets.GITHUB_TOKEN }}
```

Questo workflow crea (o aggiorna) una PR "Release: sdk/ts v1.2.0" ogni volta che vengono
pushati commit sul branch `master`. La PR può essere approvata e mergiata manualmente, e al
merge `release-please` crea automaticamente il tag Git corretto.

---

### 2. Pubblicazione TypeScript SDK su npm

```yaml
# .github/workflows/sdk-ts-publish.yml
name: Publish TypeScript SDK to npm

on:
  push:
    tags:
      - 'sdk/ts/v*'

jobs:
  publish:
    name: Build & Publish @aftertalk/sdk
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write   # necessario per npm provenance (SLSA)

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          registry-url: 'https://registry.npmjs.org'
          cache: 'npm'
          cache-dependency-path: sdk/ts/package-lock.json

      - name: Estrai versione dal tag
        id: version
        run: |
          TAG="${GITHUB_REF#refs/tags/sdk/ts/v}"
          echo "version=${TAG}" >> "$GITHUB_OUTPUT"

      - name: Installa dipendenze
        working-directory: sdk/ts
        run: npm ci

      - name: Esegui test
        working-directory: sdk/ts
        run: npm test

      - name: Typecheck
        working-directory: sdk/ts
        run: npm run typecheck

      - name: Aggiorna versione in package.json
        working-directory: sdk/ts
        run: npm version "${{ steps.version.outputs.version }}" --no-git-tag-version

      - name: Build
        working-directory: sdk/ts
        run: npm run build

      - name: Pubblica su npm
        working-directory: sdk/ts
        run: npm publish --provenance --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

**Note:**
- `--provenance` aggiunge la firma SLSA Level 2 al pacchetto npm (richiede `id-token: write`)
- `--access public` è obbligatorio per i package con scope (`@aftertalk/sdk`)
- Il `NPM_TOKEN` deve essere un **Automation token** (non Legacy), tipo `Publish`

---

### 3. Pubblicazione PHP SDK su Packagist

Packagist non ha un meccanismo di push diretto: indicizza automaticamente un repository
GitHub via webhook. Il workflow GitHub Actions si occupa di **notificare Packagist**
che c'è un nuovo tag, e Packagist esegue l'indicizzazione autonomamente.

```yaml
# .github/workflows/sdk-php-publish.yml
name: Publish PHP SDK to Packagist

on:
  push:
    tags:
      - 'sdk/php/v*'

jobs:
  notify-packagist:
    name: Notifica Packagist del nuovo rilascio
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: '8.2'
          tools: composer

      - name: Estrai versione dal tag
        id: version
        run: |
          TAG="${GITHUB_REF#refs/tags/sdk/php/v}"
          echo "version=${TAG}" >> "$GITHUB_OUTPUT"

      - name: Installa dipendenze
        working-directory: sdk/php
        run: composer install --no-dev --prefer-dist --optimize-autoloader

      - name: Esegui test PHPUnit
        working-directory: sdk/php
        run: composer test

      - name: Valida composer.json
        working-directory: sdk/php
        run: composer validate --strict

      - name: Notifica Packagist (trigger re-indicizzazione)
        run: |
          curl -s -X POST \
            "https://packagist.org/api/update-package?username=${{ secrets.PACKAGIST_USERNAME }}&apiToken=${{ secrets.PACKAGIST_API_TOKEN }}" \
            -d "repository[url]=https://github.com/Josepavese/aftertalk" \
            -o /tmp/packagist_response.json
          cat /tmp/packagist_response.json
          # Fallisce il job se Packagist risponde con errore
          if grep -q '"status":"error"' /tmp/packagist_response.json; then
            echo "ERRORE: Packagist non ha accettato la notifica"
            exit 1
          fi
```

**Configurazione Packagist (da fare una sola volta):**

1. Accedere a [packagist.org](https://packagist.org) con l'account `aftertalk`
2. "Submit" → URL: `https://github.com/Josepavese/aftertalk`
3. Impostare il **path** del `composer.json` come `sdk/php/composer.json` nel campo
   "Package root" (disponibile solo per repository mono-repo)
4. Opzionalmente abilitare il **GitHub Service Hook** automatico per evitare di affidarsi
   solo alla chiamata API

**Contenuto di `sdk/php/composer.json` (estratto rilevante):**

```json
{
  "name": "aftertalk/aftertalk-php",
  "description": "Official PHP SDK for Aftertalk API",
  "type": "library",
  "license": "MIT",
  "require": {
    "php": ">=8.1",
    "guzzlehttp/guzzle": "^7.0"
  },
  "autoload": {
    "psr-4": {
      "Aftertalk\\": "src/"
    }
  },
  "scripts": {
    "test": "vendor/bin/phpunit tests/"
  }
}
```

---

### 4. Go SDK — tag e indicizzazione pkg.go.dev

I moduli Go non richiedono upload su un registro centralizzato: pkg.go.dev indicizza
automaticamente qualsiasi tag Git nel formato `vX.Y.Z` su un repository GitHub pubblico.
Il workflow deve solo validare il modulo e garantire che il tag sia corretto.

```yaml
# .github/workflows/sdk-go-tag.yml
name: Validate & Tag Go SDK

on:
  push:
    tags:
      - 'sdk/go/v*'

jobs:
  validate:
    name: Valida modulo Go SDK
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true
          cache-dependency-path: sdk/go/go.sum

      - name: Scarica dipendenze
        working-directory: sdk/go
        run: go mod download

      - name: Esegui test
        working-directory: sdk/go
        run: go test -v -race ./...

      - name: go vet
        working-directory: sdk/go
        run: go vet ./...

      - name: Verifica che il modulo sia importabile
        run: |
          TAG="${GITHUB_REF#refs/tags/sdk/go/v}"
          MODULE_PATH="github.com/Josepavese/aftertalk/sdk/go"
          echo "Modulo: ${MODULE_PATH}@v${TAG}"

      - name: Forza indicizzazione su pkg.go.dev (opzionale)
        run: |
          TAG="${GITHUB_REF#refs/tags/sdk/go/v}"
          curl -sf "https://proxy.golang.org/github.com/Josepavese/aftertalk/sdk/go/@v/v${TAG}.info" || true
          echo "Il modulo sarà indicizzato da proxy.golang.org entro pochi minuti"
```

**Struttura `sdk/go/go.mod`:**

```
module github.com/Josepavese/aftertalk/sdk/go

go 1.22

require (
    // dipendenze del solo SDK, nessuna del server
)
```

**Uso dal lato consumer:**

```bash
go get github.com/Josepavese/aftertalk/sdk/go@v1.2.0
```

---

## Gestione dei Segreti (GitHub Secrets)

Tutti i token di pubblicazione devono essere configurati come **repository secrets** in
`Settings → Secrets and variables → Actions` del repository `Josepavese/aftertalk`.

| Secret name | Valore | Dove usato |
|-------------|--------|------------|
| `NPM_TOKEN` | Automation token da npmjs.com | `sdk-ts-publish.yml` |
| `PACKAGIST_USERNAME` | Username account packagist.org | `sdk-php-publish.yml` |
| `PACKAGIST_API_TOKEN` | API token da packagist.org → Profile → API Tokens | `sdk-php-publish.yml` |

`GITHUB_TOKEN` è già disponibile automaticamente e non richiede configurazione manuale.

### Rotazione e audit dei token

- I token NPM devono avere scope **"Automation"** e non scadere automaticamente, ma devono
  essere ruotati ogni 12 mesi. Impostare un reminder nel calendario.
- I token Packagist non scadono per default: abilitare la scadenza a 365 giorni
  se l'interfaccia lo permette.
- **Mai committare token in file di configurazione** (`.npmrc`, `auth.json`, `.env`).
  Verificare che `.gitignore` escluda questi file.

### Principio del minimo privilegio

| Token | Scope minimi necessari |
|-------|----------------------|
| NPM_TOKEN | `Publish` solo per `@aftertalk/sdk`, nessun accesso a delete o settings |
| PACKAGIST_API_TOKEN | Update packages — non necessita accesso admin |

---

## Workflow Completo End-to-End

```
1. Sviluppatore crea PR con commit tipo "feat(sdk/ts): add streaming support"
2. PR mergiata su master
3. release-please.yml esegue → crea/aggiorna PR "Release: sdk/ts v1.1.0"
4. Maintainer revede la PR di rilascio, approva e mergia
5. release-please crea automaticamente il tag "sdk/ts/v1.1.0"
6. sdk-ts-publish.yml si triggera → test → build → npm publish
7. @aftertalk/sdk@1.1.0 appare su npmjs.com entro ~1 minuto
```

```
Per PHP:
4b. La PR di rilascio PHP aggiorna composer.json version e CHANGELOG
5b. release-please crea "sdk/php/v1.1.0"
6b. sdk-php-publish.yml → test PHPUnit → notifica Packagist
7b. aftertalk/aftertalk-php@1.1.0 indicizzato su Packagist entro ~5 minuti
```

```
Per Go:
5c. release-please crea "sdk/go/v1.1.0"
6c. sdk-go-tag.yml → go test → go vet → ping proxy.golang.org
7c. Modulo disponibile su pkg.go.dev entro ~15 minuti
```

---

## Trade-off e Alternative

### Alternative a `release-please`

| Strumento | Pro | Contro |
|-----------|-----|--------|
| **release-please** (raccomandato) | Mono-repo support nativo, integrazione GitHub Actions, PR review umana prima del tag | Richiede Conventional Commits, configurazione JSON da mantenere |
| **semantic-release** | Molto flessibile, plugin npm | Mono-repo support tramite plugin terzi, configurazione più complessa |
| **changesets** | Ottimo per mono-repo Node.js | Nativo solo per npm; PHP/Go richiedono script custom |
| **Tag manuale** | Zero setup | Errore umano garantito nel tempo, nessuna automazione changelog |

### Mono-repo vs Repo separati — dettaglio per registro

**npm**: Il mono-repo con `sdk/ts/` funziona perfettamente. npm pubblica il contenuto
di `sdk/ts/package.json` e `sdk/ts/dist/` senza necessità di conoscere la struttura
del resto del repository.

**Packagist**: Supporta mono-repo tramite il campo "Package root path" nell'interfaccia
di submit. Se Packagist non supportasse la configurazione del path, si potrebbe usare
un repository separato `aftertalk-php` (mirror o sottomodulo). Questa complessità è
accettabile solo come ultima risorsa.

**Go modules**: I Go module in sotto-directory di un mono-repo sono un pattern standard
e supportato (es. `google.golang.org/grpc` fa la stessa cosa). Il module path
`github.com/Josepavese/aftertalk/sdk/go` è valido e pkg.go.dev lo indicizza correttamente.
L'unico vincolo è che il tag Git deve includere il path completo del sotto-modulo:
`sdk/go/v1.2.0`, non semplicemente `v1.2.0`.

### Rischi da mitigare

| Rischio | Mitigazione |
|---------|-------------|
| Tag errato che pubblica versione sbagliata | Branch protection su `master` + richiesta di PR per ogni merge; `release-please` non crea tag direttamente ma tramite PR approvata |
| Token npm/Packagist compromesso | Scope minimi, rotazione annuale, alert via GitHub Security Advisory |
| Test falliti pubblicano comunque | I workflow di pubblicazione hanno `go test`/`npm test`/`phpunit` come step obbligatorio prima di `publish` |
| Versione SDK non allineata con server | Documentare nel `CONTRIBUTING.md` il processo; considerare un check automatico di compatibilità API nei test E2E |

---

## Checklist di Implementazione

- [ ] Spostare il codice TypeScript da `sdk/` a `sdk/ts/` (o adattare i path nei workflow)
- [ ] Creare `sdk/php/` con `composer.json` (improvement 13)
- [ ] Creare `sdk/go/` con `go.mod` e modulo Go autonomo
- [ ] Creare `release-please-config.json` e `.release-please-manifest.json` alla root
- [ ] Aggiungere `.github/workflows/release-please.yml`
- [ ] Aggiungere `.github/workflows/sdk-ts-publish.yml`
- [ ] Aggiungere `.github/workflows/sdk-php-publish.yml`
- [ ] Aggiungere `.github/workflows/sdk-go-tag.yml`
- [ ] Creare account npm per `@aftertalk` scope, generare `NPM_TOKEN` Automation
- [ ] Registrare `aftertalk/aftertalk-php` su Packagist, generare `PACKAGIST_API_TOKEN`
- [ ] Configurare i GitHub Secrets nel repository
- [ ] Aggiornare `CONTRIBUTING.md` con il processo Conventional Commits
- [ ] Testare il flusso completo con una pre-release (`v1.0.0-rc.1`) prima della prima release stabile

---

## Dipendenze con Altri Improvement

- **Improvement 13 (PHP SDK)**: deve essere completato prima di poter attivare `sdk-php-publish.yml`
- **Improvement 04 (JS/TS SDK)**: già in stato avanzato (`sdk/` esiste con package.json) — il workflow npm può essere attivato prima degli altri due
