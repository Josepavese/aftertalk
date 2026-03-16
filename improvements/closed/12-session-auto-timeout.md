# Improvement 12: Auto-Timeout Sessione (MaxDuration)

## Stato: COMPLETATO

## Contesto

Il campo `SessionConfig.MaxDuration` **esiste già** in `internal/config/config.go:215` con valore
di default `2h`:

```go
// internal/config/config.go:214
type SessionConfig struct {
    MaxDuration               time.Duration `koanf:"max_duration"`
    MaxParticipantsPerSession int           `koanf:"max_participants_per_session"`
}
// default: MaxDuration = 2 * time.Hour  (config.go:382)
```

La config è correttamente strutturata (SSOT rispettato). Il problema è che `MaxDuration` non è
mai letto da nessun service o goroutine: è un campo orfano che non produce alcun effetto a runtime.

Questo fu identificato come parte dell'improvement 09-code-quality-bugs ma non ancora risolto.

### Caso d'uso che ha evidenziato il problema

MondoPsicologi vuole che le sessioni si chiudano automaticamente dopo 1 ora e 10 minuti se i
partecipanti dimenticano di terminare la chiamata. Senza auto-timeout, la sessione rimane
`active` indefinitamente, accumulando audio in buffer e tenendo aperto il WebRTC peer.
Il backend PHP dovrebbe fare polling per forzare la chiusura — che è lavoro inutile e fragile.

### Principio SSOT applicato

`MaxDuration` è già il punto di verità per questa configurazione. Non va creato un nuovo campo,
non va hardcodato alcun valore: si tratta solo di collegare la config esistente al service.

---

## Analisi dell'implementazione

### Opzioni valutate

**Opzione A — goroutine per sessione (timer attivo)**
All'atto di `CreateSession`, si lancia una goroutine che dorme per `MaxDuration` poi chiama
`EndSession`. Il timer viene cancellato se la sessione viene chiusa prima.

- Pro: reattivo, nessun delay
- Contro: N goroutine dormienti per N sessioni; se il processo si riavvia, i timer sono persi
  e le sessioni pre-esistenti non vengono mai chiuse.

**Opzione B — background sweep periodico (raccoglitore)**
Un'unica goroutine controlla periodicamente (es. ogni 5 minuti) le sessioni `active` con
`created_at + MaxDuration < now` e le chiude.

- Pro: sopravvive ai restart (usa il DB come stato), scala meglio con N sessioni
- Contro: latenza di chiusura pari al periodo di sweep (accettabile)

**Opzione B è la scelta corretta** per i seguenti motivi:
1. Il DB è già SSOT per lo stato delle sessioni — il criterio di scadenza è computabile dalla
   colonna `created_at` già esistente.
2. I timer in-memory si perdono al restart del processo; il DB non si perde.
3. Una goroutine è più semplice da testare e monitorare di N goroutine dormienti.

### Struttura proposta

```go
// internal/core/session/service.go

// StartSessionReaper avvia il background sweep. Va chiamato da main.go dopo
// la creazione del service. Termina quando ctx è cancellato.
func (s *Service) StartSessionReaper(ctx context.Context) {
    if s.cfg.MaxDuration == 0 {
        return // disabilitato via config
    }
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                s.reapExpiredSessions(ctx)
            }
        }
    }()
}

func (s *Service) reapExpiredSessions(ctx context.Context) {
    sessions, err := s.repo.ListActive(ctx)  // query WHERE status = 'active'
    if err != nil { ... }
    for _, sess := range sessions {
        if time.Since(sess.CreatedAt) > s.cfg.MaxDuration {
            logging.Infof("auto-closing session %s (exceeded MaxDuration %s)", sess.ID, s.cfg.MaxDuration)
            if err := s.EndSession(ctx, sess.ID); err != nil {
                logging.Errorf("auto-close failed for session %s: %v", sess.ID, err)
            }
        }
    }
}
```

### Punti di integrazione

1. `session.Service` deve ricevere `cfg.Session` (o solo `MaxDuration`) nel costruttore.
   Attualmente `NewService` non riceve `SessionConfig` — va aggiunto.

2. `internal/core/session/repository.go` deve esporre `ListActive(ctx) ([]*Session, error)` se
   non esiste già. Verificare: esiste già `List` con filtro status? Se sì, riusare.

3. In `cmd/aftertalk/main.go`, dopo la wiring del service:
   ```go
   sessionSvc.StartSessionReaper(ctx) // ctx = context derivato da os.Signal
   ```

4. Il periodo di sweep (5 minuti) può diventare anch'esso un campo config se necessario.
   Per ora è un dettaglio implementativo, non una variabile operativa.

---

## Configurazione

Nessuna modifica al config schema. Solo collegare `cfg.Session.MaxDuration` al service.

```yaml
# .aftertalk.yaml (già disponibile)
session:
  max_duration: 1h10m     # MondoPsicologi usa 70 minuti
  max_participants_per_session: 2
```

`max_duration: 0` disabilita il reaper (comportamento attuale — nessuna regressione).

---

## Documentazione da aggiornare

Quando questa feature è implementata, aggiornare obbligatoriamente:

- **`docs/wiki/configuration.md`** — documentare `session.max_duration` con descrizione del
  comportamento, valore di default (2h), e come disabilitarlo (0).
- **`docs/wiki/session-lifecycle.md`** (o equivalente) — aggiungere il paragrafo sul
  "session reaper": come funziona, che usa il DB come stato, latenza di chiusura.
- **`README.md`** — aggiungere nella tabella config la riga `session.max_duration`.
- **`internal/config/config.go`** — aggiungere commento GoDoc a `MaxDuration` che esplicita
  il comportamento (`0 = disabled`).
- **SDK JS/TS** — nessun impatto diretto (la chiusura è server-side).
- **SDK PHP** — nessun impatto diretto; nota nella guida all'integrazione che le sessioni
  vengono auto-chiuse dopo `MaxDuration` anche senza chiamare `endSession()` esplicitamente.
