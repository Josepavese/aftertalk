# Aftertalk Real Audio Test Suite

Test completo con **audio parlato reale** per validare il sistema Aftertalk end-to-end.

## 🎯 Cos'è Questo Test?

Questo test utilizza **file audio realistici** (sintetici ma con pattern vocali reali) per:
- ✅ Validare che lo streaming WebSocket funzioni correttamente
- ✅ Verificare che il sistema STT trascrive effettivamente
- ✅ Confermare che l'AI genera minutes strutturati
- ✅ Testare l'intero flusso: Audio → Trascrizione → Minutes

## 🏗️ Architettura del Test

```
┌─────────────────────────────────────────────────────────────┐
│                    Test Environment                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐                                       │
│  │  Audio Files     │  WAV 16kHz PCM Mono                   │
│  │  (Generati)      │  ~33 secondi ciascuno                 │
│  └────────┬─────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌──────────────────┐     WebSocket      ┌─────────────────┐│
│  │  Client VM       │◄──────────────────►│                 ││
│  │  (Maria Rossi)   │     Audio          │   Aftertalk     ││
│  └──────────────────┘                    │   Server        ││
│                                          │                 ││
│  ┌──────────────────┐     WebSocket      │   • Sessioni    ││
│  │  Professional VM │◄──────────────────►│   • STT         ││
│  │  (Operatore)     │     Audio          │   • Minutes AI  ││
│  └──────────────────┘                    └─────────────────┘│
│           │                              ▲                  │
│           └──────────────────────────────┘                  │
│              Trascrizioni & Minutes                         │
└─────────────────────────────────────────────────────────────┘
```

## 📁 Struttura

```
test/
├── audio/
│   └── samples/
│       ├── client_conversation.wav        # Audio cliente (~33s)
│       ├── professional_conversation.wav  # Audio operatore (~33s)
│       ├── client_transcription.txt       # Trascrizione attesa
│       └── professional_transcription.txt # Trascrizione attesa
├── scripts/
│   ├── generate_audio.go                  # Generatore audio sintetico
│   ├── setup_vms.sh                       # Setup VMs con nido
│   ├── stream_audio.sh                    # Script streaming audio
│   ├── streamer.go                        # Client WebSocket Go
│   └── run_real_audio_test.sh            # Orchestratore principale
└── results/                               # Risultati test
    ├── session_info.json
    ├── transcriptions.json
    ├── minutes.json
    ├── metrics.txt
    └── validation_report.md
```

## 🚀 Utilizzo

### Modalità 1: Test Locale (Veloce)

Per testare con il server già in esecuzione sulla tua macchina:

```bash
# 1. Avvia il server
export API_KEY="real-audio-test-api-key-2024"
export JWT_SECRET="test-secret-real-audio-test-do-not-use-in-production-12345"
go run ./cmd/aftertalk

# 2. In un altro terminale, esegui il test
./test/scripts/run_real_audio_test.sh --local
```

**Vantaggi:**
- ⚡ Veloce (~1 minuto)
- 🔄 Facile iterazione
- 🐛 Facile debugging

### Modalità 2: Test con VMs (Completo)

Per testare in un ambiente isolato con nido:

```bash
# Esegui tutto automatico
./test/scripts/run_real_audio_test.sh
```

Questo comando:
1. Crea 3 VMs (Server, Client, Professional)
2. Deploya Aftertalk sul server
3. Genera audio di test
4. Crea una sessione con 2 partecipanti
5. Streamma audio da entrambi i client
6. Attende trascrizione e minutes
7. Genera report di validazione
8. Opzionalmente elimina le VMs

**Vantaggi:**
- 🏝️ Ambiente isolato e pulito
- 🌐 Test di rete realistici
- 📊 Test di performance
- 🔄 Replicabile su qualsiasi macchina

## 📝 Contenuto Audio

### Dialogo Cliente (Maria Rossi)

```
1. "Buongiorno, sono Maria Rossi. Ho urgente bisogno di aiuto."
2. "Il mio problema riguarda il mio account che non riesco ad accedere da tre giorni."
3. "Ho provato a reimpostare la password ma non ricevo l'email."
4. "Questo è molto importante perché devo accedere ai documenti per domani."
5. "Sì, ho controllato anche nello spam ma non c'è nulla."
6. "Il mio indirizzo email è maria.rossi@email.com."
7. "Ah, capisco! Proverò subito. Grazie mille per l'aiuto!"
```

### Dialogo Operatore (Marco)

```
1. "Buongiorno Maria, sono l'operatore Marco. Come posso aiutarla oggi?"
2. "Capisco la sua frustrazione. Verifichiamo subito cosa sta succedendo."
3. "Posso chiederle se ha controllato la cartella dello spam?"
4. "Ho capito. Controllo immediatamente il suo account nel sistema."
5. "Ho trovato il problema! L'email era bloccata. La sblocco ora."
6. "Perfetto, ho sbloccato l'indirizzo. Ora dovrebbe ricevere l'email entro due minuti."
7. "Prego Maria! Se ha altri problemi non esiti a contattarci. Buona giornata!"
```

## 🔧 Generazione Audio

L'audio è generato sinteticamente ma realisticamente:

```bash
# Rigenera audio (se necessario)
go run test/scripts/generate_audio.go test/audio/samples
```

L'algoritmo:
1. Usa formanti vocali (multiple sine waves)
2. Simula sillabe con envelope variabile
3. Aggiunge rumore realistico
4. Output: WAV 16kHz 16-bit mono

**Nota:** L'audio è sintetico ma ha pattern vocali realistici. Per test con vera voce umana, sostituire i file WAV.

## 📊 Output del Test

Dopo l'esecuzione, in `test/results/` troverai:

### 1. session_info.json
```json
{
  "session_id": "sess-xxx-xxx",
  "client_token": "eyJ...",
  "professional_token": "eyJ...",
  "created_at": "2025-03-04T12:00:00Z"
}
```

### 2. transcriptions.json
```json
[
  {
    "id": "trans-001",
    "session_id": "sess-xxx",
    "text": "Buongiorno, sono Maria Rossi...",
    "timestamp": "2025-03-04T12:00:05Z",
    "confidence": 0.95
  },
  ...
]
```

### 3. minutes.json
```json
{
  "id": "mins-001",
  "session_id": "sess-xxx",
  "themes": ["Problema accesso account", "Email non ricevuta"],
  "contents_reported": [...],
  "professional_interventions": [...],
  "next_steps": ["Verificare email in spam", "Contattare supporto"]
}
```

### 4. validation_report.md
Report markdown con:
- ✅/❌ Stato di ogni componente
- 📊 Metriche chiave
- 💡 Raccomandazioni

## 🎮 Esempio di Esecuzione

```bash
$ ./test/scripts/run_real_audio_test.sh --local

🎬 Aftertalk Real Audio Test
==============================

[INFO] Modalità locale (senza VMs)
[SUCCESS] Server locale rilevato

[INFO] Step 2: Generazione audio di test...
✅ Audio di test già presente

[INFO] Step 3: Creazione sessione...
✅ Sessione creata: sess-abc-123-def

[INFO] Step 4: Streaming audio dai partecipanti...
🎤 Aftertalk Audio Streamer
============================
   File: test/audio/samples/client_conversation.wav
   Role: client
   Session: sess-abc-123-def

🎵 Audio file: 1100000 bytes
✅ Connected to WebSocket: ws://localhost:8080/ws
📋 Session metadata sent
▶️  Starting stream: 1718 chunks
   📤 Progress: 50.0% (859/1718 chunks) - 17s
   📤 Progress: 100.0% (1718/1718 chunks) - 34s

✅ Stream completed!
   Chunks sent: 1718
   Bytes sent: 1099520
   Duration: 34s

[INFO] Step 5: Attesa elaborazione trascrizione...
[SUCCESS] Trovati 12 segmenti di trascrizione

📝 Esempio trascrizioni:
   - Buongiorno sono Maria Rossi
   - Ho urgente bisogno di aiuto
   - Il mio problema riguarda l'account

[INFO] Step 6: Generazione minutes...
[SUCCESS] Minutes generati

📋 Riassunto Minutes:
{
  "themes": ["Problema accesso", "Supporto tecnico"],
  "next_steps": ["Verificare email", "Reset password"]
}

[INFO] Step 7: Raccolta metriche...
📊 Metriche chiave:
   Sessioni create: 1
   Trascrizioni: 12
   Minutes: 1

# Aftertalk Real Audio Test - Validation Report

**Data:** 2025-03-04T12:05:00
**Session ID:** sess-abc-123-def
**Modalità:** Locale

## Test Summary

### Results
✅ **Trascrizione:** 12 segmenti generati
✅ **Minutes:** 3 temi identificati

## ✅ TEST PASSED

🎉 Test completato!

📁 Risultati salvati in: /home/user/aftertalk/test/results

Per visualizzare i risultati:
   cat test/results/validation_report.md
   cat test/results/transcriptions.json | jq .
   cat test/results/minutes.json | jq .
```

## 🔍 Troubleshooting

### Server non raggiungibile
```bash
# Verifica che il server sia in esecuzione
curl http://localhost:8080/v1/health

# Se non risponde, avvialo:
go run ./cmd/aftertalk
```

### STT non trascrive
- Verifica che `STT_PROVIDER` sia configurato
- Per test locali, il sistema usa placeholder se non configurato
- In produzione, configura Google/AWS/Azure STT

### LLM non genera minutes
- Verifica che `LLM_OPENAI_API_KEY` sia impostato
- Per test locali, il sistema può usare risposte mock

### VMs nido non si connettono
```bash
# Verifica stato VMs
nido ls
nido info aftertalk-server

# Riavvia se necessario
nido stop aftertalk-server
nido start aftertalk-server
```

## 📈 Test Scalabili

Per testare più sessioni contemporanee:

```bash
# Esegui 5 test in parallelo
for i in {1..5}; do
    ./test/scripts/run_real_audio_test.sh --local &
done
wait
```

## 🧪 Test con Audio Reale (Umano)

Per usare la tua voce:

1. Registra audio con il telefono o computer
2. Converti in formato richiesto:
   ```bash
   ffmpeg -i tuo_audio.mp3 \
     -ar 16000 -ac 1 -acodec pcm_s16le \
     test/audio/samples/client_conversation.wav
   ```
3. Aggiorna `client_transcription.txt` con la trascrizione attesa
4. Esegui il test

## 🔐 Sicurezza

⚠️ **Non usare in produzione:**
- API Key di test hardcoded
- JWT Secret di test
- Database SQLite in `/tmp/`

Per produzione:
- Usa variabili d'ambiente
- Database persistente
- Secrets management (Vault, etc.)

## 📚 Risorse

- [Documentazione Testing principale](../docs/testing.md)
- [Piano Test Reale con Nido](../docs/REAL_WORLD_TESTING.md)
- [Guida WebSocket](../docs/contracts/websocket.yaml)

---

**Ultimo aggiornamento:** 2025-03-04
