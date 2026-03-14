package webrtc

import (
	"context"
	"fmt"

	"github.com/Josepavese/aftertalk/internal/config"
)

// EmbeddedProvider generates TURN credentials for the in-process pion/turn server
// and prepends any additional static ICE servers from the config.
type EmbeddedProvider struct {
	turn    *TURNServer
	statics []ICEServer
}

func NewEmbeddedProvider(turn *TURNServer, statics []config.ICEServerConfig) *EmbeddedProvider {
	servers := make([]ICEServer, len(statics))
	for i, s := range statics {
		servers[i] = ICEServer{URLs: s.URLs, Username: s.Username, Credential: s.Credential}
	}
	return &EmbeddedProvider{turn: turn, statics: servers}
}

func (p *EmbeddedProvider) Name() string { return "embedded" }

func (p *EmbeddedProvider) GetICEServers(_ context.Context, ttl int) ([]ICEServer, error) {
	username, credential := p.turn.GenerateCredentials("client", ttl)
	addr := p.turn.Addr()

	turnEntry := ICEServer{
		URLs: []string{
			fmt.Sprintf("turn:%s", addr),
			fmt.Sprintf("turn:%s?transport=tcp", addr),
		},
		Username:   username,
		Credential: credential,
	}

	result := make([]ICEServer, 0, len(p.statics)+1)
	result = append(result, p.statics...)
	result = append(result, turnEntry)
	return result, nil
}
