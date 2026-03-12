package webrtc

import (
	"context"

	"github.com/flowup/aftertalk/internal/config"
)

// StaticProvider returns the ICE servers verbatim from the config file.
// This is the zero-config fallback: point at Google STUN or any self-configured
// STUN/TURN URLs without making any external API call.
type StaticProvider struct {
	servers []ICEServer
}

func NewStaticProvider(cfgServers []config.ICEServerConfig) *StaticProvider {
	servers := make([]ICEServer, len(cfgServers))
	for i, s := range cfgServers {
		servers[i] = ICEServer{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		}
	}
	return &StaticProvider{servers: servers}
}

func (p *StaticProvider) Name() string { return "static" }

func (p *StaticProvider) GetICEServers(_ context.Context, _ int) ([]ICEServer, error) {
	return p.servers, nil
}
