# Improvement: Installer — SSOT & PAL Compliance

## Verdetto Avvocato del Diavolo

**L'asserzione "semplice installer totalmente configurabile in SSOT scritto con approccio PAL" è PARZIALMENTE vera.**

Il sistema è funzionale e impressionante nella portata, ma presenta lacune strutturali rispetto ai principi dichiarati.

---

## Gaps Identificati

### 1. Violazione SSOT — Valori Hardcoded nel Codice Go

**Problema**: La `config.yaml` è il SSOT dichiarato, ma esistono valori hardcoded sparsi nel codice che dovrebbero provenire dalla config.

| Valore Hardcoded | File | Riga | Dovrebbe Stare In |
|---|---|---|---|
| `stun:stun.l.google.com:19302` | `internal/bot/webrtc/peer.go:37` | 37 | `config.yaml → webrtc.stun_servers` |
| Timeout WebSocket `30s` | `internal/bot/webrtc/signaling.go` | varie | `config.yaml → webrtc.timeout` |
| Chunk size default `15s` | `internal/core/session/service.go` | varie | già in config ma non sempre rispettato |
| `chunkSizeMs = 15000` | `internal/bot/webrtc/peer.go` | costante | `config.yaml → processing.chunk_size_ms` |
| `buffer = 100` (transcription chan) | `internal/core/session/service.go` | varie | `config.yaml → processing.buffer_size` |
| `MaxRetries: 3` LLM | `internal/core/minutes/service.go:39` | 39 | `config.yaml → llm.retry.max_retries` |
| Timeout minutes generation | main.go | costanti | `config.yaml → processing.timeouts` |

**Fix Richiesto**:
```yaml
# config.yaml — aggiungere sezione:
webrtc:
  stun_servers:
    - stun:stun.l.google.com:19302
  websocket_timeout: 30s
  ice_gathering_timeout: 10s

processing:
  chunk_size_ms: 15000
  transcription_buffer_size: 100
  llm_retry:
    max_retries: 3
    initial_backoff: 1s
    max_backoff: 10s
```

---

### 2. Violazione PAL — Installer Non Modulare

**Problema**: `install.sh` è un file monolitico di 413 righe che mescola responsabilità:
- Platform detection (già estratta parzialmente in `_platform.sh`, ma non completamente)
- Go installation logic
- Python/pip management
- Ollama management
- Binary build
- Config generation
- CLI wrapper generation

**Violazione PAL**: Il "Logic Layer" (cosa installare) è mescolato con il "Provider Layer" (come installarlo su ogni OS).

**Fix Richiesto**: Separare in moduli distinti:

```
scripts/
├── install.sh              # Orchestrator (Logic Layer)
├── _platform.sh            # Platform detection (Middleware)
├── providers/
│   ├── _go.sh              # Go installation provider
│   ├── _python.sh          # Python/pip provider
│   ├── _ollama.sh          # Ollama provider
│   └── _whisper.sh         # Whisper server provider
└── steps/
    ├── _config.sh          # Config generation step
    ├── _binary.sh          # Build step
    └── _cli.sh             # CLI wrapper generation step
```

---

### 3. Mancanza SSOT — Versioning Installer vs Codice

**Problema**: La versione hardcoded nell'installer (`Aftertalk Installer v1.0`) non è sincronizzata con nessun file di versione nel progetto.

```bash
# install.sh:85
echo "  ║     Aftertalk Installer v1.0      ║"
```

Non esiste un `VERSION` file, né un `version.go`, né un campo in `go.mod` che faccia da SSOT per la versione. La versione esiste solo nella stringa del banner.

**Fix Richiesto**:
```
# Creare /version.txt (o leggere da git tag)
1.0.0

# install.sh legge da questo file o da git tag:
VERSION=$(cat "$AFTERTALK_SRC/version.txt" 2>/dev/null || git -C "$AFTERTALK_SRC" describe --tags 2>/dev/null || echo "dev")
```

---

### 4. Mancanza SSOT — Defaults Duplicati tra Installer e Config Struct Go

**Problema**: I valori default esistono in **due posti**:
1. `scripts/install.sh` (genera `config.yaml` con defaults hardcoded)
2. `internal/config/config.go` (struct Go con tag `default:...` via koanf)

Se si cambia un default nel codice Go, l'installer non lo saprà e genererà un `config.yaml` con il vecchio valore.

**Esempio concreto**:
```bash
# install.sh:230
processing:
  chunkSizeMs: 15000   # hardcoded nell'installer
```
```go
// config.go — se un giorno cambia a 30000:
type ProcessingConfig struct {
    ChunkSizeMs int `koanf:"chunkSizeMs" default:"30000"`
}
```

**Fix Richiesto**: L'installer dovrebbe generare la config con i defaults estratti dal binario stesso:
```bash
# Dopo la build del binario:
"$AFTERTALK_BIN/aftertalk-server" --dump-config-defaults > "$CONFIG_FILE"
```
Questo richiede un flag `--dump-config-defaults` nel server Go che emette YAML con tutti i valori default.

---

### 5. Installer Windows Incompleto (PAL non bilanciata)

**Problema**: `install.ps1` esiste ma manca di parità funzionale con `install.sh`:
- Non gestisce `aftertalk update`
- Il processo management (PID files) usa approcci diversi tra Unix e Windows
- Non è testato in CI
- Manca `install.bat` come entry point (l'utente Windows finale non usa PowerShell direttamente)

**Fix Richiesto**:
- Entry point `install.bat` che lancia `install.ps1` correttamente
- Parità completa comandi CLI (`start`, `stop`, `status`, `update`, `logs`)
- Test CI per Windows (GitHub Actions `windows-latest`)

---

### 6. Nessuna Verifica Integrità dell'Installer

**Problema**: Il pattern `curl -fsSL ... | bash` è insicuro senza verifica della firma.

```bash
# Pattern attuale — vulnerabile a MITM:
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

**Fix Richiesto**:
```bash
# Pattern sicuro con verifica SHA256:
curl -fsSL https://releases.aftertalk.io/install.sh -o install.sh
curl -fsSL https://releases.aftertalk.io/install.sh.sha256 -o install.sh.sha256
sha256sum --check install.sh.sha256
bash install.sh
```

---

### 7. aftertalk CLI — Nessun Meccanismo di Health Recovery

**Problema**: Il comando `aftertalk start` avvia i processi ma non li monitora. Se whisper_server crasha dopo l'avvio, nessuno lo rileva.

**Fix Richiesto**: Sistema di supervisione dei processi:
- Integrazione con `systemd` (Linux) o `launchd` (macOS) per auto-restart
- Su Windows: Windows Service via NSSM o Task Scheduler
- Oppure: supervisord-style watchdog nel CLI wrapper

---

## Priorità di Intervento

| # | Gap | Impatto | Effort | Priorità |
|---|-----|---------|--------|----------|
| 1 | STUN hardcoded nel Go | Medio | Basso | **Alta** |
| 4 | Defaults duplicati installer/Go | Alto | Medio | **Alta** |
| 2 | Installer non modulare | Medio | Medio | **Media** |
| 3 | Versioning mancante | Basso | Basso | **Media** |
| 7 | Nessun health recovery | Alto | Alto | **Media** |
| 5 | Windows incompleto | Medio | Alto | **Bassa** |
| 6 | Sicurezza installer | Basso | Medio | **Bassa** |

---

## Passi di Implementazione

### Step 1 — Esternalizzare STUN in config (1-2h)

1. Aggiungere `WebRTCConfig` struct a `internal/config/config.go`
2. Aggiungere sezione `webrtc:` a config.yaml template nell'installer
3. Leggere STUN servers in `peer.go` da `cfg.WebRTC.STUNServers`
4. Test: verificare che WebRTC funzioni con STUN servers custom

### Step 2 — Aggiungere `--dump-config-defaults` (2-3h)

1. Aggiungere flag `--dump-defaults` al main.go
2. Quando attivo: stampare YAML con tutti i valori default da koanf e uscire
3. Modificare installer: dopo build, eseguire `aftertalk-server --dump-defaults > config.yaml`
4. Rimuovere blocco heredoc config da install.sh

### Step 3 — VERSION file (30min)

1. Creare `/version.txt` con valore corrente
2. Leggere in install.sh: `VERSION=$(cat "$AFTERTALK_SRC/version.txt")`
3. Leggere in main.go: `go:embed version.txt`
4. Esporre in `/v1/health` response: `{"status":"ok","version":"1.0.0"}`

### Step 4 — Modularizzare install.sh (3-4h)

1. Estrarre ogni blocco funzionale in `scripts/providers/_*.sh`
2. Ogni provider ha: `check_<tool>()`, `install_<tool>()`, `version_<tool>()`
3. `install.sh` diventa orchestratore puro che source i provider
4. Aggiungere test unitari shell (bats framework)
