# Improvement 14: Guida all'Integrazione — Pattern MondoPsicologi

## Stato: APERTO

## Contesto

Questo improvement non riguarda modifiche al codice di Aftertalk ma la creazione di documentazione
di integrazione completa per il caso d'uso "piattaforma telemedicina con backend PHP e WebRTC
custom". MondoPsicologi è il caso concreto che ha generato la richiesta, ma la guida deve essere
scritta in modo generico per qualsiasi integratore con caratteristiche simili.

Il problema attuale è che un nuovo integratore deve capire da solo:
- Chi deve creare la sessione (server o client?) e perché
- Come evitare la race condition quando entrambi i peer entrano nella stanza
- Come associare i metadata al momento della creazione per ritrovarli nel webhook
- Come gestire la chiusura della sessione (esplicita vs auto-timeout)
- Come verificare e processare il webhook in modo sicuro

Questa conoscenza è distribuita nel codice, nella memoria del progetto, e nei report di analisi —
ma non è documentata in un posto solo, navigabile e linkabile.

### Prerequisiti da completare prima

Questa guida **dipende** dagli improvement 11 e 13:
- **Improvement 11** (metadata nel webhook) deve essere implementato per poter documentare
  il flusso end-to-end senza workaround.
- **Improvement 13** (PHP SDK) deve esistere per poter mostrare codice PHP concreto.

La guida può essere abbozzata prima, ma deve essere completata e pubblicata solo quando i
prerequisiti sono implementati.

---

## Struttura della guida da creare

### `docs/wiki/integration-guide.md` (o equivalente nella wiki)

#### 1. Architettura di riferimento

Diagramma testuale del flusso completo:

```
Frontend (browser) → PHP Backend → Aftertalk API
                                 ← JWT token
Frontend ← JWT token
Frontend → Aftertalk /signaling (WebRTC)
Frontend → PHP Backend (end call)
PHP Backend → Aftertalk POST /sessions/{id}/end
Aftertalk → PHP Backend (webhook: minutes + metadata)
PHP Backend → DB (associa minuta all'appuntamento)
```

#### 2. Regola fondamentale: chi crea la sessione

**Regola**: la sessione Aftertalk deve essere creata esclusivamente dal backend.
Mai dal frontend, mai da entrambi i peer.

Ragioni:
- L'API key Aftertalk non deve mai essere esposta al browser.
- I metadata (doctor_id, patient_id, appointment_id) devono essere verificati server-side.
- Il ruolo del partecipante deve essere assegnato dall'auth di sessione, non da input utente.
- La race condition (due peer che creano la stessa stanza) è gestibile solo con un lock DB
  server-side.

#### 3. Gestione della race condition

Il problema: dottore e paziente premono "Unisciti" contemporaneamente. Entrambi chiamano
il backend PHP. Il backend deve creare la sessione una volta sola.

Soluzione: `INSERT OR IGNORE` + get-or-create atomico sull'`appointment_id` come chiave
di idempotenza. Dettaglio del pattern con codice PHP nel README dell'SDK PHP.

#### 4. Assegnazione del ruolo

Il frontend non deve mai poter dichiarare il proprio ruolo. Il backend PHP conosce
il ruolo dell'utente autenticato (es. `user_id == appointment.doctor_id` → terapeuta).
Il JWT generato da Aftertalk codifica il ruolo al suo interno — non modificabile lato client.

#### 5. Flusso token e sicurezza

- Il frontend riceve solo il JWT del partecipante (breve TTL, monouso).
- Il frontend non riceve mai l'API key Aftertalk.
- Il JWT viene usato solo per `/signaling` (WebRTC) — non per chiamate REST.
- L'URL base di Aftertalk può essere esposta al frontend (è necessaria per il WebSocket).

#### 6. Chiusura sessione

Due modalità:
- **Esplicita**: il frontend segnala al backend PHP la fine della chiamata → PHP chiama
  `POST /v1/sessions/{id}/end`.
- **Auto-timeout**: Aftertalk chiude automaticamente dopo `session.max_duration`
  (improvement 12). Il backend PHP non deve fare nulla.

Consiglio: implementare entrambe. L'auto-timeout è una rete di sicurezza, non la via principale.

#### 7. Ricezione webhook e associazione metadata

Dopo la chiusura, Aftertalk elabora STT + LLM e consegna le minute via webhook.
Il payload include i metadata passati alla creazione (improvement 11).

Flusso nel backend PHP:
1. Ricevere `POST /api/webhooks/aftertalk`.
2. Verificare `X-Aftertalk-Signature` (HMAC-SHA256) prima di processare qualsiasi dato.
3. Leggere `payload.session_metadata.appointment_id`.
4. Associare le minute all'appuntamento nel DB.
5. Notificare il dottore.

#### 8. Modalità push vs notify_pull

- `push`: le minute complete sono nel body del webhook. Più semplice, meno sicuro per dati
  sensibili in transito.
- `notify_pull`: il webhook porta solo un URL firmato one-shot. Il backend PHP deve fare
  una GET per scaricare le minute. Più complesso, ma zero dati medici in transito nel webhook.

Raccomandazione per telemedicina: usare `notify_pull` con HMAC verification.

#### 9. Template terapia vs consulting

Aftertalk supporta template configurabili per la struttura delle minute.
Il template `therapy` (dottore/paziente) è quello corretto per MondoPsicologi.
Il `template_id` va passato alla creazione della sessione.

---

## Documentazione da aggiornare

Quando questa guida è pubblicata, aggiornare obbligatoriamente:

- **`README.md` (root)** — aggiungere link alla guida all'integrazione nella sezione
  "Documentation" o "Getting Started".
- **`docs/wiki/` index** — aggiungere voce `integration-guide.md` nell'indice della wiki.
- **`improvements/13-php-sdk.md`** — aggiornare il riferimento alla guida quando è pubblicata.
- **`improvements/11-webhook-metadata.md`** — linkare la guida come esempio d'uso dei metadata.
- **`@aftertalk/sdk` README** — aggiungere link alla guida per chi usa il JS SDK insieme a
  un backend PHP.
