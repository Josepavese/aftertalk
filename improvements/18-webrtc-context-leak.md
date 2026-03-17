# 18 — WebRTC: Context Leak e Goroutine Dangling

## Problema

`internal/bot/webrtc/signaling.go` usa `context.TODO()` per la creazione dei peer WebRTC
in due punti:

```go
// signaling.go:160 — handleJoin
peer, err := s.manager.CreatePeer(context.TODO(), sessionID, participantID, role)

// signaling.go:207 — handleOffer
peer, err := s.manager.CreatePeer(context.TODO(), sessionID, participantID, role)
```

Il commento `//nolint:contextcheck` sopprime il linter invece di correggere il problema.

### Conseguenze

1. **Goroutine leak**: se il client WebSocket disconnette bruscamente (tab chiuso, rete
   caduta), la goroutine `handleMessages` viene cancellata ma il `Peer` creato con
   `context.TODO()` non riceve alcun signal di cancellazione. La PeerConnection Pion
   rimane aperta fino al timeout ICE naturale (~30s).

2. **Resource leak**: ogni disconnessione brusca lascia una PeerConnection zombie per
   ~30s che tiene risorse ICE e DTLS aperte.

3. **Tracciabilità**: con `context.TODO()` non c'è modo di associare traces/spans al
   lifecycle della peer connection.

---

## Modifiche richieste

### `internal/bot/webrtc/signaling.go`

La `handleMessages` goroutine ha già un context dal proprio WebSocket lifecycle.
Passarlo ai `CreatePeer`:

```go
// handleJoin deve ricevere il context dalla goroutine chiamante
func (s *SignalingServer) handleJoin(ctx context.Context, cw *connWriter, sessionID, participantID, role string) {
    peer, err := s.manager.CreatePeer(ctx, sessionID, participantID, role)
    // ...
}

// handleOffer idem
func (s *SignalingServer) handleOffer(ctx context.Context, cw *connWriter, msg signalingMessage) {
    peer, err := s.manager.CreatePeer(ctx, sessionID, participantID, role)
    // ...
}
```

Il context della connessione WebSocket deve essere derivato dal context di `handleMessages`
con un timeout ragionevole per la vita di una sessione:

```go
func (s *SignalingServer) handleMessages(ctx context.Context, cw *connWriter, ...) {
    // ctx già cancellato quando il WS si chiude — passarlo ai figli
    s.handleJoin(ctx, cw, sessionID, participantID, role)
    // oppure
    s.handleOffer(ctx, cw, msg)
}
```

### `internal/bot/webrtc/peer.go` — Verificare che CreatePeer rispetti ctx

Verificare che `Manager.CreatePeer` passi il context a `webrtc.NewAPI` o almeno
alla PeerConnection per il cleanup. Se Pion non supporta cancellation sulla
PeerConnection direttamente, è sufficiente che `CreatePeer` registri un cleanup
su `ctx.Done()`:

```go
func (m *Manager) CreatePeer(ctx context.Context, ...) (*Peer, error) {
    peer, err := newPeer(...)
    if err != nil { return nil, err }

    // Cleanup automatico quando il context viene cancellato
    go func() {
        <-ctx.Done()
        peer.PC.Close()
        m.removePeer(sessionID, participantID)
    }()

    return peer, nil
}
```

---

## Test

Simulare una disconnessione brusca con `curl --max-time 1` su `/signaling` e verificare
con `go tool pprof` che le goroutine di WebRTC vengano terminate entro il timeout.

## Impatto

- Eliminazione goroutine/resource leak su disconnessione brusca
- Comportamento corretto sotto stress (molte sessioni parallele)
- Rimozione del `//nolint:contextcheck` sleazy
