# Specifiche Tecniche

## Modulo AI per Minuta Automatica di Fine Seduta

Documento destinato **ai progettisti e sviluppatori**. Linguaggio volutamente operativo e prescrittivo.

---

## 1. Scopo

Implementare un modulo che:

* intercetti **solo l’audio** delle sedute WebRTC,
* generi una **trascrizione testuale con ruoli certi (medico/paziente)**,
* produca una **minuta AI strutturata** a fine seduta,
* renda la minuta **consultabile e modificabile** dal medico.

Il sistema **NON**:

* registra o conserva audio in modo persistente,
* effettua diagnosi automatiche,
* espone trascrizioni o minute al paziente.

---

## 2. Vincoli architetturali

* PeerJS usato **esclusivamente per signaling**
* Media WebRTC **P2P tra medico e paziente**
* Terzo peer dedicato (Bot Recorder) per ricezione audio
* Backend applicativo = source of truth per ruoli e sessioni
* Tutte le operazioni AI sono **post‑sessione (batch)**

---

## 3. Attori e servizi

### 3.1 Client Medico

* Browser Web
* WebRTC audio/video P2P
* WebRTC audio verso Bot Recorder

### 3.2 Client Paziente

* Browser Web
* WebRTC audio/video P2P
* WebRTC audio verso Bot Recorder

### 3.3 Backend Applicativo

* Gestione utenti e ruoli
* Creazione seduta (`callId`)
* Emissione token firmati
* Segnalazione fine seduta

### 3.4 Bot Recorder / Transcriber

* Peer WebRTC server-side
* Ricezione audio
* Timestamp server-side
* Integrazione STT
* Persistenza temporanea trascrizioni

---

## 4. Identità, ruoli e sicurezza

### 4.1 Token di sessione

Il backend emette un JWT per ogni partecipante:

```
{
  callId: string,
  userId: string,
  role: "doctor" | "patient",
  exp: timestamp,
  jti: uuid
}
```

Regole:

* un solo stream per `(callId, role)`
* token verificato dal Bot Recorder
* token scaduti o riusati → connessione rifiutata

---

## 5. Flusso di seduta (sequenza tecnica)

1. Backend crea `callId`
2. Backend emette token firmati
3. Client avvia call P2P (PeerJS)
4. Client apre WebRTC audio verso Bot Recorder
5. Bot valida token
6. Bot riceve audio e assegna timestamp
7. Audio processato in chunk
8. A fine seduta backend invia `SESSION_END`
9. Bot finalizza trascrizione
10. Bot invoca pipeline AI minuta

---

## 6. Gestione audio

### 6.1 Caratteristiche audio

* Codec: Opus (input)
* Conversione interna: PCM mono 16kHz
* Chunking: 10–30 secondi

### 6.2 Timestamp

* Clock monotonic server-side
* Timestamp relativi all’inizio seduta
* Niente timestamp client-side

---

## 7. Trascrizione (STT)

### 7.1 Modalità

* Batch STT (post-processing)
* Provider cloud configurabile

### 7.2 Output STT

```
{
  callId,
  role,
  start_ms,
  end_ms,
  text,
  confidence
}
```

Persistenza:

* append-only
* nessuna modifica in-place

---

## 8. Minuta AI (LLM)

### 8.1 Pre-processing

* Ordinamento per timestamp
* Chunking trascrizione (2–5 min)

### 8.2 Prompting

Il prompt deve:

* vietare diagnosi
* vietare inferenze non esplicite
* richiedere output strutturato
* includere citazioni con timestamp

### 8.3 Output minuta

Struttura obbligatoria:

* Temi principali
* Contenuti riportati dal paziente
* Interventi del terapeuta
* Progressi / criticità
* Next steps
* Citazioni (timestamp)

---

## 9. Consegna al backend

* La minuta è inviata via webhook applicativo
* Il webhook deve essere idempotente
* Stato: `READY → DELIVERED`

---

## 10. Frontend – UI Medico

### 10.1 Vista elenco sedute

* Stato AI: in corso / pronta / errore

### 10.2 Vista minuta

* Testo editabile
* Timestamp cliccabili
* Salvataggio manuale

---

## 11. Error handling

* STT failure → retry
* LLM failure → retry
* Timeout → stato `ERROR`

---

## 12. Retention e cleanup

* Audio: **mai persistito**
* Trascrizione: retention configurabile
* Log: senza contenuti sensibili

---

## 13. Non-goals

* Analisi emotiva automatica
* Diagnosi clinica
* Accesso paziente alla minuta

---

## 14. Definition of Done

* Trascrizione corretta con ruoli
* Minuta generata e visibile
* Editing funzionante
* Nessun audio persistente
* Audit sicurezza superato

---

Fine specifiche tecniche.
