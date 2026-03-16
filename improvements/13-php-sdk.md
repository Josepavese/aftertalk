# Improvement 13: PHP SDK (`aftertalk/aftertalk-php`)

## Stato: APERTO

## Contesto

Aftertalk dispone già di un SDK TypeScript (`@aftertalk/sdk`, improvement 04) che copre il lato
frontend: WebRTC signaling, poller minute, client HTTP. Manca il lato **backend server-to-server**,
che per molti integratori è PHP (Laravel, Symfony, WordPress, piattaforme telemedicina).

Il caso d'uso primario che ha motivato questa richiesta è l'integrazione con MondoPsicologi:
il loro backend PHP deve creare sessioni con metadata, generare token JWT per i partecipanti,
terminare le sessioni e ricevere/verificare i webhook. Fare tutto questo con `curl` grezzo è
fragile e non documentato.

Il PHP SDK deve essere **affiancato** all'SDK JS/TS, documentato nello stesso modo, e vivere
in un repository dedicato (o come package Composer pubblicato su Packagist).

### Analogia con il JS SDK

| Responsabilità | SDK JS/TS (`@aftertalk/sdk`) | SDK PHP (`aftertalk/aftertalk-php`) |
|---|---|---|
| Creare sessioni | ✅ SessionsAPI | ✅ da implementare |
| Generare token partecipanti | ✅ SessionsAPI | ✅ da implementare |
| Terminare sessione | ✅ SessionsAPI | ✅ da implementare |
| Connessione WebRTC | ✅ WebRTCConnection | ❌ non applicabile (server-side) |
| Polling minute | ✅ MinutesPoller | 🔶 opzionale (di solito push/webhook) |
| Verificare webhook HMAC | ❌ non applicabile (frontend) | ✅ da implementare |
| Deserializzare payload webhook | ❌ non applicabile | ✅ da implementare |

---

## Struttura del repository

```
aftertalk-php/
├── composer.json
├── README.md
├── src/
│   ├── AftertalkClient.php      # Entry point pubblico
│   ├── Config.php               # Configurazione (base_url, api_key, webhook_secret, timeout)
│   ├── Http/
│   │   └── HttpClient.php       # Wrapper Guzzle/cURL con retry, timeout, header API key
│   ├── Api/
│   │   ├── SessionsApi.php      # createSession, getSession, endSession, listSessions
│   │   ├── MinutesApi.php       # getMinutes, updateMinutes, getMinutesVersions
│   │   └── TranscriptionsApi.php
│   ├── Webhook/
│   │   ├── WebhookHandler.php   # verifySignature(), parsePayload()
│   │   ├── MinutesPayload.php   # DTO: session_id, minutes, metadata, participants, timestamp
│   │   └── NotificationPayload.php # DTO: session_id, retrieve_url, metadata, expires_at
│   ├── Dto/
│   │   ├── Session.php          # id, status, template_id, metadata, created_at
│   │   ├── Participant.php      # id, user_id, role, token_jti
│   │   ├── Minutes.php          # id, session_id, sections, citations, template_id
│   │   └── ParticipantSummary.php
│   └── Exception/
│       ├── AftertalkException.php
│       ├── AuthException.php    # 401
│       ├── NotFoundException.php # 404
│       └── WebhookSignatureException.php
└── tests/
    ├── Unit/
    │   ├── WebhookHandlerTest.php
    │   └── SessionsApiTest.php
    └── Integration/ (opzionale, con mock HTTP)
```

---

## API pubblica principale

```php
use Aftertalk\AftertalkClient;

$client = new AftertalkClient(
    baseUrl: 'https://aftertalk.yourserver.com',
    apiKey: env('AFTERTALK_API_KEY'),
    webhookSecret: env('AFTERTALK_WEBHOOK_SECRET'),
);

// --- CREAZIONE SESSIONE (server-side, mai da frontend) ---
$session = $client->sessions->create(
    templateId: 'therapy',
    participantCount: 2,
    metadata: [
        'appointment_id' => 'appt_123',
        'doctor_id'      => 'doc_456',
        'patient_id'     => 'pat_789',
    ],
);
// $session->id, $session->templateId, $session->createdAt

// --- TOKEN PARTECIPANTE (da restituire al frontend) ---
$token = $client->sessions->createParticipantToken(
    sessionId: $session->id,
    userId: 'doc_456',
    role: 'terapeuta',
);
// JWT monouso — il frontend lo usa per /signaling

// --- FINE SESSIONE (triggerato da frontend o da cron) ---
$client->sessions->end(sessionId: $session->id);

// --- RICEZIONE WEBHOOK (controller PHP) ---
// routes/webhooks.php
$handler = $client->webhook;

if (!$handler->verifySignature($requestBody, $signatureHeader)) {
    http_response_code(401);
    exit;
}

$payload = $handler->parsePayload($requestBody);
// $payload->sessionId
// $payload->sessionMetadata['appointment_id']  ← da improvement 11
// $payload->minutes->sections
// $payload->participants[0]->role
```

---

## Pattern get-or-create (anti race condition)

Il PHP SDK **non** deve implementare la logica get-or-create internamente (è logica di business
del chiamante, non del trasporto). Tuttavia la documentazione deve spiegare esplicitamente il
pattern da usare con l'idempotency dell'`appointment_id`:

```php
// Documentare questo pattern nella wiki e nel README del SDK:
function getOrCreateAfterlTalkSession(string $appointmentId, Appointment $appt): string {
    $db->execute('INSERT OR IGNORE INTO appointment_calls (appointment_id) VALUES (?)', [$appointmentId]);
    $row = $db->fetchOne('SELECT aftertalk_session_id FROM appointment_calls WHERE appointment_id = ?', [$appointmentId]);

    if ($row['aftertalk_session_id'] === null) {
        $session = $aftertalk->sessions->create(
            templateId: 'therapy',
            participantCount: 2,
            metadata: ['appointment_id' => $appointmentId, 'doctor_id' => $appt->doctorId, ...],
        );
        $db->execute('UPDATE appointment_calls SET aftertalk_session_id = ? WHERE appointment_id = ?',
            [$session->id, $appointmentId]);
        return $session->id;
    }
    return $row['aftertalk_session_id'];
}
```

---

## Dipendenze

- PHP ≥ 8.1 (readonly properties, enum, fibers)
- `guzzlehttp/guzzle` ^7.0 oppure PSR-18 `psr/http-client` (preferibile per interoperabilità)
- `psr/log` ^3.0 (logging opzionale)
- Nessuna dipendenza su framework (puro PHP — funziona con Laravel, Symfony, Slim, etc.)

---

## Documentazione da aggiornare

Quando l'SDK PHP è rilasciato, aggiornare obbligatoriamente:

- **`README.md` (root Aftertalk)** — aggiungere nella sezione SDK il link a `aftertalk-php`
  accanto a `@aftertalk/sdk`.
- **`docs/wiki/sdks.md`** (o equivalente) — aggiungere pagina dedicata PHP SDK con:
  installazione Composer, guida rapida, tabella metodi, gestione webhook, pattern
  get-or-create, esempi Laravel/Symfony.
- **`docs/wiki/integration-guide.md`** — sezione "Integrazione backend PHP" con il flusso
  completo MondoPsicologi-style (appointment → session → token → webhook).
- **`aftertalk-php/README.md`** — README del repository PHP: installazione, quickstart,
  tutti i metodi, esempi webhook, sicurezza (non esporre API key al frontend).
- **`@aftertalk/sdk` README** — aggiungere nota che per backend server-to-server esiste
  `aftertalk/aftertalk-php`.
- **`CHANGELOG.md`** (se presente) — voce per il rilascio del PHP SDK.
