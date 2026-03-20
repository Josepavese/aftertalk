# Improvement 28: README d'impatto — primo colpo d'occhio decisivo

## Stato: APERTO

---

## Obiettivo

Un developer o un potenziale integratore apre il repo per la prima volta.
Nei **10 secondi successivi** deve aver capito:

1. Cosa fa Aftertalk (una frase)
2. Perché è diverso (tre punti visivi)
3. Come iniziare (tre comandi)

Se dopo 10 secondi scorre ancora cercando "ma quindi cos'è?" — il README ha fallito.

---

## Goal / Non-Goal

| Goal | Non-Goal |
|------|----------|
| Comunicare il valore in 10 secondi | Spiegare tutto l'architettura |
| Fare venire voglia di provarlo | Essere un tutorial |
| Linkare wiki, SDK, guide | Duplicare il contenuto del wiki |
| Mostrare output reale (screenshot) | Descrivere ogni feature |
| Essere denso ma non lungo | Elencare ogni parametro di config |
| Funzionare bene come anteprima GitHub | Sostituire la documentazione |

---

## Tono di voce

**Parole chiave**: diretto, tecnico, concreto, fiducioso — mai promozionale.

```
✅  "Aftertalk intercepts WebRTC audio and generates structured session minutes."
✅  "Zero audio stored. Minutes delivered via webhook."
✅  "PHP backend + TypeScript frontend, connected in 20 lines."

❌  "Revolutionary AI-powered meeting intelligence platform"
❌  "Seamlessly integrates with your existing workflow"
❌  "Enterprise-grade privacy-first solution"
```

**Regola d'oro**: se una frase potrebbe stare nel marketing di qualsiasi SaaS, va eliminata.
Ogni frase deve contenere qualcosa di specifico e verificabile su Aftertalk.

**Lingua**: inglese. Tutti i README di progetti tecnici aperti sono in inglese.

---

## Asset visivi necessari (da creare prima di scrivere il README)

### 1. Screenshot demo UI — PRIORITÀ ALTA

Una singola immagine `docs/assets/demo-screenshot.png` che mostri:
- La sezione "Minutes" del test UI con una minuta reale completata
- Sezioni visibili: Temi, Contenuti riportati, Prossimi passi
- Citation con timestamp
- Status "ready" con template "Seduta Terapeutica"

**Come ottenerla**: aprire `https://app.mondopsicologi.it/aftertalk/` con la minuta
della sessione completata, fare screenshot dell'area minutes. Ritagliare a ~1200×700px.
Salvare in `docs/assets/demo-screenshot.png`.

**Alternativa**: mockup clean in Figma/Sketch se lo screenshot reale ha dati sensibili.

### 2. Diagramma architettura — PRIORITÀ MEDIA

`docs/assets/architecture.png` — versione grafica del diagramma ASCII già in
`docs/wiki/integration-guide.md`, resa in forma visiva.

Tre blocchi colorati in fila:
```
[Browser + TS SDK] → [PHP Backend] → [Aftertalk Server]
                                          ↓
                                   [Webhook → Minutes]
```
Stile: minimal, sfondo scuro o bianco, font monospace, bordi sottili.
Tool: qualsiasi (Excalidraw, draw.io, Mermaid rendered, Figma).

### 3. Badge row — PRIORITÀ BASSA (generati automaticamente)

```markdown
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)
![CI](https://github.com/Josepavese/aftertalk/actions/workflows/ci.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/Josepavese/aftertalk)
```

**Regola**: massimo 4-5 badge. Niente badge decorativi inutili (stars, forks, ecc.).

---

## Struttura del README

### Sezione 1 — Hero (above the fold)

```
# Aftertalk

[4 badge]

**WebRTC session recorder → structured AI minutes, delivered via webhook.**

[screenshot demo-screenshot.png — larghezza 100%, cliccabile verso /aftertalk/]
```

Nessun altro testo. Lo screenshot parla da solo.

---

### Sezione 2 — Cosa fa (3 righe, 0 sottosezioni)

```markdown
Aftertalk sits alongside your WebRTC calls. It captures audio server-side,
transcribes with STT (Whisper · Google · AWS · Azure), generates structured
minutes with an LLM (OpenAI · Anthropic · Ollama), and delivers them to your
backend via webhook — all without storing audio.
```

**Una** sola riga aggiuntiva per la privacy/sicurezza:
```markdown
> No audio is ever persisted. Minutes are editable. Humans stay in the loop.
```

---

### Sezione 3 — Quick Start (3 comandi, niente tutorial)

```markdown
## Quick Start

```bash
# Install (Linux / macOS)
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash

# Configure
cp ~/.aftertalk/aftertalk.yaml.example ~/.aftertalk/aftertalk.yaml
# edit: API_KEY, JWT_SECRET, LLM_PROVIDER

# Start
aftertalk start
```

→ Demo UI at `http://localhost:8080`
```

**Regola**: zero spiegazioni inline. Chi vuole capire va alla wiki.
Link finale: `→ [Full installation guide](docs/wiki/installation.md)`

---

### Sezione 4 — Come si integra (codice minimo, due colonne)

Questa è la sezione più importante per un integratore.
Due snippet side-by-side in unica sezione:

```markdown
## Integrate

**PHP backend** (holds API key) + **TypeScript frontend** (JWT only):

<!-- tabella 2 colonne con due code block -->
```

**PHP snippet** (6 righe):
```php
$result = $aftertalk->rooms->join(
    code: $appointment->id,
    name: $user->name,
    role: 'therapist',
);
// return $result['token'] to the browser
```

**TypeScript snippet** (4 righe):
```typescript
const sdk = new AftertalkClient({ baseUrl: origin });
const conn = await sdk.connectWebRTC({
  sessionId, token, // received from PHP backend
});
```

Link: `→ [Full integration guide](docs/wiki/integration-guide.md)`

---

### Sezione 5 — SDK (tabella 2 righe)

```markdown
## SDKs

| SDK | Install | Use case |
|-----|---------|----------|
| **TypeScript** `@aftertalk/sdk` | `npm i @aftertalk/sdk` | Browser — WebRTC, minutes polling |
| **PHP** `aftertalk/aftertalk-php` | `composer require aftertalk/aftertalk-php` | Server — sessions, webhook verification |
```

---

### Sezione 6 — Documentation (link table)

```markdown
## Documentation

| | |
|--|--|
| [Installation](docs/wiki/installation.md) | Requirements, install modes, first run |
| [Configuration](docs/wiki/configuration.md) | All parameters with defaults |
| [REST API](docs/wiki/rest-api.md) | Every endpoint with curl examples |
| [Integration Guide](docs/wiki/integration-guide.md) | PHP + TS full workflow, security model |
| [Webhook](docs/wiki/webhook.md) | Push vs notify_pull, HMAC verification |
| [Templates](docs/wiki/templates.md) | therapy, consulting, custom |
| [Architecture](docs/wiki/architecture.md) | Internal audio → minutes pipeline |
```

---

### Sezione 7 — License (una riga, in fondo)

```markdown
MIT — [Josepavese](https://github.com/Josepavese)
```

Niente contributing guide, niente code of conduct, niente changelog nel README.
Link nel footer solo se esistono: `· [Changelog](CHANGELOG.md)` ecc.

---

## Cosa NON mettere nel README

| Evitare | Perché |
|---------|--------|
| Lista features (10+ bullets) | Nessuno legge liste lunghe; le feature si scoprono usandolo |
| Sezione "Architecture" inline | C'è già in wiki/architecture.md — duplicare crea drift |
| Sezione "Contributing" | Va in CONTRIBUTING.md, non nel README |
| Sezione "License" lunga | Una riga è sufficiente |
| "Why Aftertalk?" | Marketing speak — lo screenshot mostra il perché |
| Stack tecnologico dettagliato | Interessante per chi contribuisce, non per chi integra |
| Roadmap / TODO inline | Crea aspettative false, non è aggiornata, va in Issues/Projects |
| Qualsiasi frase che inizia con "Easily", "Simply", "Just" | Condiscendente e vuoto |
| Tabella di confronto con competitor | Invecchia male e crea controversie |

---

## Dimensioni target

| Elemento | Limite |
|----------|--------|
| Tempo di lettura | < 2 minuti |
| Righe di testo puro (senza code, tabelle, immagini) | < 30 |
| Sezioni di primo livello (`##`) | ≤ 6 |
| Comandi nel quick start | ≤ 3 |
| Linee di codice totali nei snippet | ≤ 20 |

---

## Ordine di implementazione

1. **Crea gli asset** (`docs/assets/demo-screenshot.png`, opzionale `architecture.png`)
2. **Scrivi il README** seguendo la struttura sopra
3. **Test visivo**: apri il preview GitHub (o un renderer Markdown locale)
   e verifica la regola dei 10 secondi con qualcuno che non conosce il progetto
4. **Rimuovi** il README attuale (che è minimal/stub) e sostituisci
5. **Aggiorna** `docs/wiki/README.md` se necessario (è già linkato)

---

## Dipendenze

- Nessuna dipendenza tecnica (il README è solo Markdown)
- Asset visivi richiesti: almeno `demo-screenshot.png` (senza di lui manca il colpo d'occhio)
- Wiki già completata e linkata correttamente ✅
- SDK README (`sdk/php/README.md`) già presente ✅
- Integration guide già presente ✅
