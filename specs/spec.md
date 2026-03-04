# Feature Specification: Aftertalk Core

**Feature Branch**: `001-aftertalk-core`  
**Created**: 2026-03-04  
**Status**: Draft  
**Input**: User description: "Aftertalk Core - modulo AI per generare automaticamente minute di fine seduta da conversazioni WebRTC con trascrizione e sintesi AI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Acquisizione Audio da Sessione WebRTC (Priority: P1)

Il sistema intercetta l'audio di una sessione WebRTC in corso e lo dirige verso un componente dedicato per la trascrizione, mantenendo separati gli stream audio dei diversi partecipanti.

**Why this priority**: L'acquisizione audio è il prerequisite fondamentale per qualsiasi funzionalità di Aftertalk. Senza questa capacità, non è possibile né trascrivere né generare minute. È la base su cui si costruisce tutto il resto.

**Independent Test**: Può essere testato indipendentemente avviando una sessione WebRTC e verificando che il Bot Recorder riceva stream audio separati per ogni partecipante con timestamp server-side corretti.

**Acceptance Scenarios**:

1. **Given** una sessione WebRTC attiva tra due partecipanti, **When** i partecipanti parlano, **Then** il Bot Recorder riceve due stream audio separati (uno per partecipante) con identificazione del ruolo (professionista/paziente)
2. **Given** una sessione WebRTC con token JWT validi, **When** il Bot Recorder riceve gli stream, **Then** i timestamp sono assegnati server-side con clock monotonic relativo all'inizio della seduta
3. **Given** una sessione WebRTC con token scaduto o già usato, **When** un partecipante tenta di connettersi al Bot Recorder, **Then** la connessione viene rifiutata

---

### User Story 2 - Trascrizione Automatica con Ruoli Certi (Priority: P2)

Il sistema trascrive automaticamente l'audio ricevuto in testo, assegnando correttamente il ruolo a ciascun segmento di conversazione.

**Why this priority**: La trascrizione è il secondo step fondamentale dopo l'acquisizione audio. È necessaria per poter generare la minuta strutturata. Senza trascrizione corretta con ruoli, la minuta non può essere generata.

**Independent Test**: Può essere testato inviando audio preregistrato al Bot Recorder e verificando che la trascrizione prodotta contenga segmenti con ruolo, timestamp, testo e confidence score.

**Acceptance Scenarios**:

1. **Given** stream audio ricevuti dal Bot Recorder, **When** la sessione termina, **Then** il sistema produce una trascrizione strutturata con segmenti `{callId, role, start_ms, end_ms, text, confidence}`
2. **Given** una trascrizione in corso, **When** si verifica un errore STT, **Then** il sistema ritenta automaticamente la trascrizione prima di marcare lo stato come ERROR
3. **Given** una trascrizione completata, **When** il backend la richiede, **Then** i dati sono disponibili in formato append-only senza modifiche in-place

---

### User Story 3 - Generazione Minuta AI Strutturata (Priority: P3)

Il sistema elabora la trascrizione e produce una minuta strutturata con citazioni temporali, temi principali, interventi del professionista e next steps.

**Why this priority**: La minuta è il valore finale per l'utente professionista. È il prodotto che riduce il carico cognitivo e migliora la tracciabilità. Viene dopo l'acquisizione e la trascrizione.

**Independent Test**: Può essere testato fornendo una trascrizione completa al modulo AI e verificando che la minuta prodotta contenga tutti i campi obbligatori: Temi principali, Contenuti riportati dal paziente, Interventi del terapeuta, Progressi/criticità, Next steps, Citazioni con timestamp.

**Acceptance Scenarios**:

1. **Given** una trascrizione completa con ruoli certi, **When** il sistema elabora la trascrizione, **Then** produce una minuta strutturata con citazioni temporali verificabili
2. **Given** una minuta in generazione, **When** si verifica un errore LLM, **Then** il sistema ritenta automaticamente prima di marcare lo stato come ERROR
3. **Given** una minuta generata, **When** il backend la richiede via webhook, **Then** la consegna è idempotente con stato `READY → DELIVERED`

---

### User Story 4 - Consultazione e Modifica Minuta da Parte del Professionista (Priority: P4)

Il professionista può visualizzare la minuta generata, consultare i timestamp cliccabili e modificare il testo prima del salvataggio definitivo.

**Why this priority**: L'interazione umana con la minuta è essenziale per il principio di Human-in-the-loop, ma è l'ultimo step dopo che il sistema ha acquisito, trascritto e generato la minuta.

**Independent Test**: Può essere testato fornendo una minuta completa all'interfaccia del professionista e verificando che possa visualizzare, cliccare sui timestamp per rivedere i punti chiave, modificare il testo e salvare le modifiche.

**Acceptance Scenarios**:

1. **Given** una minuta generata e consegnata al backend, **When** il professionista accede alla lista sedute, **Then** vede lo stato della minuta: in corso/pronta/errore
2. **Given** una minuta pronta, **When** il professionista la apre, **Then** può visualizzare il testo completo, cliccare sui timestamp per saltare ai punti chiave e modificare il testo
3. **Given** una minuta modificata dal professionista, **When** salva le modifiche, **Then** la versione modificata sostituisce quella originale ma la cronologia delle versioni è preservata

---

### Edge Cases

- Cosa succede se il Bot Recorder perde la connessione durante una sessione? La trascrizione parziale è preservata?
- Come gestisce il sistema una sessione con più di due partecipanti (es. terapia di coppia, gruppo)?
- Cosa succede se il provider STT o LLM è temporaneamente non disponibile?
- Come viene gestita una sessione molto breve (< 1 minuto) o molto lunga (> 2 ore)?
- Cosa succede se l'audio è di bassa qualità o con molto rumore di fondo?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Il sistema MUST intercettare l'audio delle sessioni WebRTC senza interferire con la comunicazione P2P tra i partecipanti
- **FR-002**: Il sistema MUST ricevere stream audio separati per ogni partecipante con identificazione del ruolo (professionista/paziente)
- **FR-003**: Il sistema MUST assegnare timestamp server-side con clock monotonic relativo all'inizio della seduta
- **FR-004**: Il sistema MUST convertire l'audio Opus in PCM mono 16kHz per l'elaborazione
- **FR-005**: Il sistema MUST processare l'audio in chunk di 10-30 secondi
- **FR-006**: Il sistema MUST trascrivere l'audio in testo utilizzando un provider STT cloud configurabile
- **FR-007**: Il sistema MUST produrre trascrizioni con struttura `{callId, role, start_ms, end_ms, text, confidence}`
- **FR-008**: Il sistema MUST persistere le trascrizioni in formato append-only senza modifiche in-place
- **FR-009**: Il sistema MUST generare minute strutturate con campi obbligatori: Temi principali, Contenuti riportati, Interventi del professionista, Progressi/criticità, Next steps, Citazioni con timestamp
- **FR-010**: Il sistema MUST vietare diagnosi automatiche e inferenze non esplicite nella minuta
- **FR-011**: Il sistema MUST consegnare la minuta al backend via webhook idempotente
- **FR-012**: Il sistema MUST permettere al professionista di visualizzare, consultare timestamp e modificare la minuta
- **FR-013**: Il sistema MUST garantire che l'audio NON sia mai persistito in modo permanente
- **FR-014**: Il sistema MUST verificare i token JWT per ogni connessione al Bot Recorder
- **FR-015**: Il sistema MUST rifiutare connessioni con token scaduti o già usati
- **FR-016**: Il sistema MUST consentire un solo stream per coppia `(callId, role)`

### Key Entities

- **Sessione (Session)**: Rappresenta una seduta conversazionale con identificativo univoco `callId`, partecipanti con ruoli, timestamp di inizio e fine, stato (active, ended, processing, completed, error)
- **Partecipante (Participant)**: Rappresenta un attore nella conversazione con identificativo `userId`, ruolo astratto (professionista/paziente o altri ruoli configurabili), token JWT di sessione
- **Stream Audio (AudioStream)**: Rappresenta il flusso audio di un partecipante con identificativo, codec (Opus), sample rate, chunk temporali, timestamp server-side
- **Trascrizione (Transcription)**: Rappresenta la conversione testo dell'audio con segmenti strutturati `{callId, role, start_ms, end_ms, text, confidence}`, timestamp di creazione, stato (pending, processing, ready, error)
- **Minuta (Minutes)**: Rappresenta la sintesi strutturata della conversazione con campi obbligatori (Temi, Contenuti, Interventi, Progressi, Next steps, Citazioni), timestamp di generazione, stato (pending, ready, delivered, error), versione (per modifiche)
- **Token di Sessione (SessionToken)**: Rappresenta le credenziali di accesso con JWT contenente `{callId, userId, role, exp, jti}`, stato (valid, used, expired, revoked)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Il sistema acquisisce audio da sessioni WebRTC con latenza < 500ms per l'inizio della ricezione dello stream
- **SC-002**: Il sistema trascrive audio con accuratezza > 85% (confidence score medio) in condizioni audio standard
- **SC-003**: Il sistema genera minute strutturate in < 5 minuti per sessioni di 1 ora
- **SC-004**: Il professionista può visualizzare una minuta pronta entro 10 minuti dalla fine della sessione
- **SC-005**: Il sistema gestisce fino a 100 sessioni concorrenti senza degradazione delle performance
- **SC-006**: Il sistema mantiene traccia di tutti i tentativi di retry per errori STT/LLM con log (senza contenuti sensibili)
- **SC-007**: Il professionista può modificare e salvare una minuta in < 2 minuti
- **SC-008**: Il sistema rifiuta il 100% delle connessioni con token non validi, scaduti o già usati
- **SC-009**: Il sistema non persiste MAI audio in modo permanente (verificabile tramite audit)
- **SC-010**: Il core rimane agnostico rispetto al dominio applicativo (verificabile tramite code review: nessun riferimento a termini di dominio specifico)

## Assumptions

- PeerJS è già configurato per il signaling WebRTC nel layer applicativo (MondoPsicologi)
- Il backend applicativo esiste già e può emettere token JWT firmati
- Il provider STT cloud supporta lingua italiana e inglese (configurabile)
- Il provider LLM cloud supporta prompt in italiano e inglese (configurabile)
- I partecipanti hanno browser Web moderni con supporto WebRTC
- La connessione internet ha bandwidth sufficiente per stream audio bidirezionali + stream verso Bot Recorder
- Il professionista ha familiarità con interfacce web per consultare e modificare documenti

## Out of Scope

- Analisi emotiva automatica del tono di voce o contenuto
- Diagnosi cliniche automatiche o suggerimenti terapeutici
- Accesso del paziente alla trascrizione o alla minuta
- Registrazione video della sessione
- Integrazione con sistemi di cartelle cliniche elettroniche (EHR)
- Riconoscimento vocale per comandi durante la sessione
- Traduzione automatica in tempo reale
- Analisi sentiment in tempo reale durante la sessione
