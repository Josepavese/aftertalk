# 17 — Installer: Completezza dei Check e Configurabilità

## Problemi Identificati

### A. Step `90-verify` troppo superficiale

`verify.go` fa solo GET `/v1/health` e controlla HTTP 200. L'install può "passare"
anche se STT e LLM non sono raggiungibili. In produzione questo significa:

- Aftertalk si avvia ✓
- Prima sessione con audio → STT fallisce silenziosamente
- Nessun avviso operativo

### B. Whisper language hardcoded italiano

`whisper.go:125` nel systemd unit template:
```
Environment=WHISPER_LANGUAGE=it
```
Il campo `WhisperLanguage` non esiste in `InstallConfig`. Chi installa per inglese
deve editare manualmente `/etc/systemd/system/aftertalk-whisper.service` dopo l'install.

### C. Installer non chiede lingua STT nel prompt

`prompt.go` chiede `WhisperModel` e `WhisperURL` ma non la lingua. Utenti internazionali
non sanno che il default è italiano.

### D. Ollama model verification incompleta

Dopo `ollama pull`, il step non verifica che il modello sia effettivamente disponibile.
Se il pull fallisce a metà (disco pieno, timeout rete), il marker `.ok` viene scritto
lo stesso perché il passo successivo non controlla lo stato del modello.

### E. Step `00-prereqs` non installa ffmpeg

`prereqs.go` installa le dipendenze base ma non `ffmpeg`. Se non viene eliminata la
dipendenza da ffmpeg (vedi improvement #16), bisogna aggiungere ffmpeg ai prereqs.
**Nota**: Se viene implementato improvement #16, questo punto decade.

---

## Modifiche richieste

### 1. `cmd/installer/steps/verify.go` — Aggiungere health check STT e LLM

Dopo la verifica `/v1/health`, aggiungere:

```go
// Verifica whisper-local se configurato
if cfg.STTProvider == "whisper-local" {
    whisperURL := cfg.WhisperURL + "/v1/models"  // endpoint standard OpenAI-compat
    // GET con timeout 5s, solo warn se non risponde (potrebbe ancora avviarsi)
    if err := checkEndpoint(ctx, whisperURL, 5*time.Second); err != nil {
        log.Warn(fmt.Sprintf("whisper-local at %s not reachable: %v (check aftertalk-whisper service)", cfg.WhisperURL, err))
    } else {
        log.Info("whisper-local reachable ✓")
    }
}

// Verifica ollama se configurato
if cfg.LLMProvider == "ollama" {
    ollamaURL := cfg.OllamaURL + "/api/tags"
    if err := checkEndpoint(ctx, ollamaURL, 5*time.Second); err != nil {
        log.Warn(fmt.Sprintf("ollama at %s not reachable: %v (check ollama service)", cfg.OllamaURL, err))
    } else {
        log.Info("ollama reachable ✓")
    }
}
```

I check STT/LLM sono `Warn` non `Error`: non bloccano l'install (potrebbero ancora
avviarsi), ma avvertono l'operatore.

### 2. `cmd/installer/config/config.go` + `env.go` + `prompt.go` — Aggiungere WhisperLanguage

In `InstallConfig`:
```go
WhisperLanguage string // default: "it"
```

In `Default()`:
```go
WhisperLanguage: "it",
```

In `prompt.go` (sotto WhisperModel):
```go
cfg.WhisperLanguage = ask("Whisper language (it|en|fr|de|es|auto)", cfg.WhisperLanguage)
```

In `whisper.go` systemd unit template:
```
Environment=WHISPER_LANGUAGE={{.Language}}
```

### 3. `cmd/installer/steps/ollama.go` — Verificare modello dopo pull

Dopo `ollama pull`, eseguire `ollama list` e verificare che il modello sia presente:

```go
// Verifica che il modello sia nella lista dopo il pull
checkCmd := exec.CommandContext(ctx, "ollama", "list")
out, err := checkCmd.Output()
if err != nil || !strings.Contains(string(out), strings.Split(model, ":")[0]) {
    return fmt.Errorf("ollama model %s not found after pull — disk full?", model)
}
log.Info(fmt.Sprintf("model verified in ollama list: %s", model))
```

### 4. `cmd/installer/steps/prereqs.go` — Aggiungere ffmpeg (condizionale)

Solo se non viene rimossa la dipendenza ffmpeg (altrimenti questo punto è N/A):

```go
// In apt packages list per Linux:
"ffmpeg",
```

---

## Impatto

- Operatore informato immediatamente se STT/LLM non partono post-install
- Support per deployment multi-lingua senza edit manuale systemd
- Nessun caso "installazione completata ma non funziona" silenzioso

---

## Status: closed

Implementato:
- `config.go`: campo `WhisperLanguage string` + default `"it"`
- `env.go`: `WHISPER_LANGUAGE` in `FromEnvMap` e `WriteEnvFile`
- `prompt.go`: domanda `Whisper language (it|en|fr|de|es|auto)` nel wizard
- `whisper.go`: `{{.Language}}` in systemd unit e macOS plist (era `it` hardcoded)
- `ollama.go`: verifica `ollama list` dopo pull; errore esplicito se modello mancante
- `verify.go`: `checkEndpoint` helper + warn se whisper-local/ollama non raggiungibili post-install
- Punto E (ffmpeg nei prereqs) non necessario: rimosso da improvement #16

**Analisi avvocato del diavolo post-implementazione:**
- `goto` in `verify.go` rimosso: refactoring in `waitAftertalkHealthy` + `checkDependencies`
- `WhisperLanguage` propagato correttamente a `whisper_server.py` (letto via `os.environ.get("WHISPER_LANGUAGE", "")`)
- `ollama list` check usa `SplitN(model,":",2)[0]` — corretto anche per modelli senza tag
- Test aggiunti: `cmd/installer/config/config_test.go` e `cmd/installer/steps/verify_test.go` (13 test, httptest, roundtrip env file)
