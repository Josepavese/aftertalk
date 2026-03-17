# 16 — Audio Pipeline: Rimozione ffmpeg, Pure Go

## Problema

`whisper_local.go:128` chiama `audio.DecodeFramesToWAVffmpeg` che lancia un processo
`exec.Command("ffmpeg", ...)` per ogni chunk audio trascritto. Questo significa:

- ffmpeg deve essere installato su ogni macchina (dipendenza non dichiarata)
- Ogni chunk = 1 processo OS → overhead significativo sotto carico
- Se ffmpeg manca → trascrizioni silenziose falliscono completamente

**Il paradosso**: `pkg/audio/opus.go` ha già `DecodeFramesToWAV` che fa esattamente la stessa
cosa in pure Go usando `kazopus`, senza CGO, senza processi esterni. Semplicemente non viene usata.

---

## Modifiche richieste

### 1. `internal/ai/stt/whisper_local.go:128`

```go
// PRIMA (ffmpeg)
wav, err := audio.DecodeFramesToWAVffmpeg(ctx, audioData.Frames, 16000)

// DOPO (pure Go)
wav, err := audio.DecodeFramesToWAV(audioData.Frames, 16000)
```

Rimuovere anche l'import `context` se non più usato in quel punto.

### 2. `pkg/audio/opus.go:69-76` — Migliorare resampling

Il downsampling 48kHz → 16kHz attuale è naive (ogni 3° campione), introduce aliasing.
Sostituire con box filter (media di 3 campioni consecutivi) — pure Go, no CGO:

```go
// Sostituire:
for i := 0; i < len(allPCM); i += 3 {
    downsampled = append(downsampled, allPCM[i])
}

// Con (box filter anti-aliasing):
for i := 0; i+2 < len(allPCM); i += 3 {
    avg := (int32(allPCM[i]) + int32(allPCM[i+1]) + int32(allPCM[i+2])) / 3
    downsampled = append(downsampled, int16(avg))
}
```

### 3. `pkg/audio/ogg_opus.go` — Rimuovere `DecodeFramesToWAVffmpeg`

La funzione `DecodeFramesToWAVffmpeg` (righe 91-109) diventa dead code dopo il fix #1.
Rimuoverla. Se `EncodeOggOpus` non è usata altrove, rimuovere l'intero file.

Verificare:
```bash
grep -rn "DecodeFramesToWAVffmpeg\|EncodeOggOpus\|writeOggPage\|oggCRC" --include="*.go" .
```

### 4. `pkg/audio/pcm.go:53-55` — Rimuovere dead code

`decodeOpus` e il suo errore `errOpusDecodingNotImplemented` esistono ma non sono mai chiamati
(superati da `opus.go`). Rimuovere:
- `decodeOpus()` (righe 53-55)
- `errOpusDecodingNotImplemented` (riga 12)
- `PCMConverter.ConvertToPCM()` se chiama solo `decodeOpus`

### 5. `pkg/audio/opus.go:119-121` — Rimuovere `OpusEncoder.Encode`

```go
func (e *OpusEncoder) Encode(pcmData []int16) ([]byte, error) {
    return nil, errOpusEncodingNotSupported  // mai implementato, mai chiamato
}
```

Dead code. Se `OpusEncoder` non è usato altrove, rimuovere anche la struct.

### 6. `internal/ai/stt/whisper_local.go:133-138` — Rimuovere debug WAV dump

Il file debug `/tmp/aftertalk_debug_*.wav` viene scritto **sempre** in produzione:
```go
dbgPath := fmt.Sprintf("/tmp/aftertalk_debug_%s.wav", audioData.SessionID)
if writeErr := os.WriteFile(dbgPath, wav, 0600); writeErr != nil { ... }
logging.Infof("WhisperLocal: wav_bytes=%d frames_decoded=%d debug_wav=%s", ...)
```

Rimuovere completamente o proteggere con `if cfg.Debug { ... }`.

---

## Impatto

| Prima | Dopo |
|-------|------|
| Richiede ffmpeg installato | Zero dipendenze esterne per audio |
| 1 processo OS per chunk | Tutto in-process Go |
| Aliasing nel downsampling | Box filter, qualità STT migliore |
| /tmp pieno di WAV in produzione | Nessun file debug |

## Verifica

```bash
go test ./pkg/audio/... -v
go test ./internal/ai/stt/... -v
which ffmpeg  # non deve essere richiesto
```
