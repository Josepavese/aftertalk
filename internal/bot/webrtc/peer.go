package webrtc

import (
	"context"
	"sync"
	"time"

	pionice "github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

type AudioTrackHandler func(sessionID, participantID, role string, payload []byte)

type Sample struct {
	Timestamp       time.Time
	Data            []byte
	Duration        time.Duration
	PacketTimestamp uint32
}

type Peer struct {
	ConnectedAt   time.Time
	PC            *webrtc.PeerConnection
	AudioTrack    *webrtc.TrackLocalStaticSample
	onAudio       AudioTrackHandler
	SessionID     string
	ParticipantID string
	Role          string
	mu            sync.RWMutex
	Connected     bool
}

func NewPeer(sessionID, participantID, role string, onAudio AudioTrackHandler, iceServers []config.ICEServerConfig, iceUDPPortMin, iceUDPPortMax uint16) (*Peer, error) {
	webrtcICEServers := make([]webrtc.ICEServer, 0, len(iceServers))
	for _, s := range iceServers {
		webrtcICEServers = append(webrtcICEServers, webrtc.ICEServer{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}
	cfg := webrtc.Configuration{
		ICEServers:         webrtcICEServers,
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
	}

	se := webrtc.SettingEngine{}
	se.SetICEMulticastDNSMode(pionice.MulticastDNSModeQueryAndGather)
	se.SetIncludeLoopbackCandidate(true)
	// Fix range of UDP ports so the firewall only needs to allow this range.
	// Defaults 49200–49209; override via webrtc.ice_udp_port_min/max in config.
	if iceUDPPortMin > 0 && iceUDPPortMax > 0 {
		_ = se.SetEphemeralUDPPortRange(iceUDPPortMin, iceUDPPortMax)
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))

	pc, err := api.NewPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	peer := &Peer{
		SessionID:     sessionID,
		ParticipantID: participantID,
		Role:          role,
		PC:            pc,
		Connected:     false,
		ConnectedAt:   time.Now(),
		onAudio:       onAudio,
	}

	if err := peer.setupAudioTrack(); err != nil {
		_ = pc.Close() //nolint:errcheck // best-effort cleanup on error path
		return nil, err
	}

	if err := peer.setupPeerConnection(); err != nil {
		_ = pc.Close() //nolint:errcheck // best-effort cleanup on error path
		return nil, err
	}

	return peer, nil
}

func (p *Peer) setupAudioTrack() error {
	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio", "pion",
	)
	if err != nil {
		return err
	}
	p.AudioTrack = audioTrack
	return nil
}

func (p *Peer) setupPeerConnection() error {
	_, err := p.PC.AddTrack(p.AudioTrack)
	if err != nil {
		return err
	}

	p.PC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		logging.Infof("ICE Connection State for %s: %s", p.ParticipantID, state.String())
		p.mu.Lock()
		p.Connected = (state == webrtc.ICEConnectionStateConnected || state == webrtc.ICEConnectionStateCompleted)
		p.mu.Unlock()
	})

	p.PC.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		logging.Infof("Received track from %s: %s", p.ParticipantID, tr.Kind())
		if tr.Kind() == webrtc.RTPCodecTypeAudio {
			go p.handleAudioTrack(tr)
		}
	})

	return nil
}

func (p *Peer) handleAudioTrack(tr *webrtc.TrackRemote) {
	defer func() { logging.Infof("Track closed for %s", p.ParticipantID) }()
	for {
		pkt, _, err := tr.ReadRTP()
		if err != nil {
			logging.Errorf("Error reading RTP: %v", err)
			return
		}
		if p.onAudio != nil {
			p.onAudio(p.SessionID, p.ParticipantID, p.Role, pkt.Payload)
		}
	}
}

func (p *Peer) WriteAudio(data []byte) error {
	if p.AudioTrack == nil {
		return nil
	}
	sample := Sample{
		Data:      data,
		Timestamp: time.Now(),
		Duration:  20 * time.Millisecond,
	}
	return writeSample(p.AudioTrack, sample)
}

func writeSample(t *webrtc.TrackLocalStaticSample, s Sample) error {
	type sampleWriter interface {
		WriteSample(interface{}) error
	}
	if sw, ok := interface{}(t).(sampleWriter); ok {
		type mediaSample struct {
			Timestamp       interface{}
			Duration        interface{}
			Data            []byte
			PacketTimestamp uint32
		}
		return sw.WriteSample(mediaSample{
			Data:            s.Data,
			Timestamp:       s.Timestamp,
			Duration:        s.Duration,
			PacketTimestamp: s.PacketTimestamp,
		})
	}
	return nil
}

func (p *Peer) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Connected
}

func (p *Peer) Close() error {
	if p.PC != nil {
		return p.PC.Close()
	}
	return nil
}

type Manager struct {
	peers         map[string]*Peer
	onAudio       AudioTrackHandler
	iceServers    []config.ICEServerConfig
	iceUDPPortMin uint16
	iceUDPPortMax uint16
	mu            sync.RWMutex
}

func NewManager(onAudio AudioTrackHandler, iceServers []config.ICEServerConfig, iceUDPPortMin, iceUDPPortMax uint16) *Manager {
	return &Manager{
		peers:         make(map[string]*Peer),
		onAudio:       onAudio,
		iceServers:    iceServers,
		iceUDPPortMin: iceUDPPortMin,
		iceUDPPortMax: iceUDPPortMax,
	}
}

func (m *Manager) CreatePeer(ctx context.Context, sessionID, participantID, role string) (*Peer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionID + ":" + participantID
	if _, exists := m.peers[key]; exists {
		return nil, ErrPeerAlreadyExists
	}
	peer, err := NewPeer(sessionID, participantID, role, m.onAudio, m.iceServers, m.iceUDPPortMin, m.iceUDPPortMax)
	if err != nil {
		return nil, err
	}
	m.peers[key] = peer
	logging.Infof("Created WebRTC peer for session=%s participant=%s", sessionID, participantID)

	// Close the PeerConnection when the caller's context is canceled (e.g. WebSocket disconnect).
	// RemovePeer (called by the signaling layer) will also close it, but that may happen later;
	// this goroutine ensures the PeerConnection is released promptly on abrupt disconnections.
	go func() {
		<-ctx.Done()
		if err := peer.PC.Close(); err != nil {
			logging.Warnf("ctx-driven peer close session=%s participant=%s: %v", sessionID, participantID, err)
		}
	}()

	return peer, nil
}

func (m *Manager) GetPeer(sessionID, participantID string) (*Peer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, exists := m.peers[sessionID+":"+participantID]
	return peer, exists
}

func (m *Manager) RemovePeer(sessionID, participantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionID + ":" + participantID
	if peer, exists := m.peers[key]; exists {
		if err := peer.Close(); err != nil {
			logging.Warnf("Error closing WebRTC peer: %v", err)
		}
		delete(m.peers, key)
		logging.Infof("Removed WebRTC peer for session=%s participant=%s", sessionID, participantID)
	}
}

func (m *Manager) GetPeersForSession(sessionID string) []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Peer
	for _, peer := range m.peers {
		if peer.SessionID == sessionID {
			result = append(result, peer)
		}
	}
	return result
}

func (m *Manager) CloseSessionPeers(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, peer := range m.peers {
		if peer.SessionID == sessionID {
			if err := peer.Close(); err != nil {
				logging.Warnf("Error closing WebRTC peer: %v", err)
			}
			delete(m.peers, key)
		}
	}
}

var ErrPeerAlreadyExists = &webrtcError{"peer already exists"}

type webrtcError struct{ msg string }

func (e *webrtcError) Error() string { return e.msg }
