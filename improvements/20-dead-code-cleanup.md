# 20 — Cleanup: Dead Code e Inconsistenze Minori

## Dead Code da Rimuovere

### 1. `pkg/audio/pcm.go` — `decodeOpus` inutilizzabile

```go
// pcm.go:12 — errore mai restituito a codice reale
var errOpusDecodingNotImplemented = errors.New("opus decoding not yet implemented - requires opus library")

// pcm.go:53-55 — funzione sempre-errore
func decodeOpus(opusData []byte, sampleRate, channels int) ([]int16, error) {
    return nil, errOpusDecodingNotImplemented
}
```

`decodeOpus` è chiamata da `PCMConverter.ConvertToPCM` che è chiamata da... nessuno
nel codice di produzione. Il decoder Opus reale è in `opus.go` con kazopus.

**Fix**: Rimuovere `decodeOpus`, `errOpusDecodingNotImplemented`, e `PCMConverter.ConvertToPCM`.
Valutare se rimuovere tutta la struct `PCMConverter` se le sue altre funzioni non sono usate.

### 2. `pkg/audio/opus.go` — `OpusEncoder` non funzionante

```go
// opus.go:119-121
func (e *OpusEncoder) Encode(pcmData []int16) ([]byte, error) {
    return nil, errOpusEncodingNotSupported
}
```

L'encoder Opus non è mai implementato e non è chiamato da nessuno nel codice di produzione.

**Fix**: Rimuovere `OpusEncoder`, `NewOpusEncoder`, `errOpusEncodingNotSupported`.

### 3. `pkg/audio/ogg_opus.go` — `DecodeFramesToWAVffmpeg` (post improvement #16)

Dopo il fix in improvement #16, questa funzione diventa dead code.

**Fix**: Rimuovere la funzione. Verificare se `EncodeOggOpus`/`writeOggPage`/`oggCRC`
sono usate altrove; se no, rimuovere l'intero file.

```bash
grep -rn "EncodeOggOpus\|writeOggPage\|DecodeFramesToWAVffmpeg" --include="*.go" .
```

---

## Inconsistenze da Correggere

### 4. `pkg/audio/opus.go:12` — Errore orfano

```go
var errOpusEncodingNotSupported = errors.New("opus encoding requires external library - use github.com/hraban/opus")
```

Se si rimuove `OpusEncoder`, anche questo errore va rimosso.

### 5. `pkg/audio/pcm.go` — Import `math` orfano dopo cleanup

Se `ConvertToInt16` viene rimossa (usa `math.Max/Min`), rimuovere l'import `"math"`.

### 6. `cmd/installer/steps/whisper.go` — Debug log eccessivo in verify

```go
// whisper_local.go:138 (da rimuovere insieme al WAV dump)
logging.Infof("WhisperLocal: wav_bytes=%d frames_decoded=%d debug_wav=%s", len(wav), len(audioData.Frames), dbgPath)
```

Spostare a `logging.Debugf` o eliminare insieme al dump (vedi improvement #16).

---

## Verifica Finale

Dopo tutti i cleanup:

```bash
go build ./...              # nessun errore di compilazione
go vet ./...                # nessun warning
go test ./pkg/audio/... -v  # tutti i test passano
grep -rn "TODO\|FIXME\|HACK\|errOpus\|decodeOpus\|OpusEncoder" --include="*.go" .
```

L'ultimo grep deve tornare vuoto o solo commenti legittimi.
