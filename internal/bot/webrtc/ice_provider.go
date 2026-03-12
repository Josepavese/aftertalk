// Package webrtc provides WebRTC infrastructure including ICE server management.
//
// ICEProvider is the Platform Abstraction Layer (PAL) for TURN/STUN credential
// provisioning. All callers (RTCConfigHandler, BotServer) consume only this
// interface; the concrete implementation is selected once at startup by the
// factory NewICEProvider.
//
//	Logic (RTCConfigHandler) → ICEProvider interface → concrete provider
//	                                                    ├── EmbeddedProvider  (pion/turn self-hosted)
//	                                                    ├── TwilioProvider    (Twilio NTS)
//	                                                    ├── XirsysProvider    (Xirsys)
//	                                                    ├── MeteredProvider   (Metered.ca)
//	                                                    └── StaticProvider    (config ice_servers list)
package webrtc

import (
	"context"
	"fmt"
	"time"

	"github.com/flowup/aftertalk/internal/config"
	"github.com/flowup/aftertalk/internal/logging"
)

// ICEServer is the normalised wire format understood by RTCPeerConnection.
// Every provider translates its proprietary response into this struct.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEProvider is the middleware contract for any source of ICE server credentials.
// Implementations must be safe for concurrent use.
type ICEProvider interface {
	// Name returns the provider identifier (used in logs and metrics).
	Name() string

	// GetICEServers returns a fresh list of ICE servers valid for the given ttl.
	// ctx governs the HTTP round-trip or credential derivation.
	// ttl is a hint; providers may return shorter-lived credentials.
	GetICEServers(ctx context.Context, ttl int) ([]ICEServer, error)
}

// NewICEProvider is the single routing point (factory) that selects the provider
// based on config. No caller outside this package needs to know the concrete type.
func NewICEProvider(cfg *config.WebRTCConfig, turnServer *TURNServer) (ICEProvider, error) {
	switch cfg.ICEProviderName {
	case "twilio":
		if cfg.Twilio.AccountSID == "" || cfg.Twilio.AuthToken == "" {
			return nil, fmt.Errorf("ice provider: twilio requires account_sid and auth_token")
		}
		p := NewTwilioProvider(cfg.Twilio.AccountSID, cfg.Twilio.AuthToken)
		logging.Infof("ICE provider: twilio (account=%s...)", cfg.Twilio.AccountSID[:min(8, len(cfg.Twilio.AccountSID))])
		return p, nil

	case "xirsys":
		if cfg.Xirsys.Ident == "" || cfg.Xirsys.Secret == "" || cfg.Xirsys.Channel == "" {
			return nil, fmt.Errorf("ice provider: xirsys requires ident, secret, and channel")
		}
		p := NewXirsysProvider(cfg.Xirsys.Ident, cfg.Xirsys.Secret, cfg.Xirsys.Channel)
		logging.Infof("ICE provider: xirsys (ident=%s, channel=%s)", cfg.Xirsys.Ident, cfg.Xirsys.Channel)
		return p, nil

	case "metered":
		if cfg.Metered.AppName == "" || cfg.Metered.APIKey == "" {
			return nil, fmt.Errorf("ice provider: metered requires app_name and api_key")
		}
		p := NewMeteredProvider(cfg.Metered.AppName, cfg.Metered.APIKey)
		logging.Infof("ICE provider: metered (app=%s)", cfg.Metered.AppName)
		return p, nil

	case "embedded":
		if turnServer == nil {
			return nil, fmt.Errorf("ice provider: embedded requires an active TURN server (webrtc.turn.enabled must be true)")
		}
		logging.Infof("ICE provider: embedded TURN (%s)", turnServer.Addr())
		return NewEmbeddedProvider(turnServer, cfg.ICEServers), nil

	case "", "static":
		logging.Infof("ICE provider: static (%d servers configured)", len(cfg.ICEServers))
		return NewStaticProvider(cfg.ICEServers), nil

	default:
		return nil, fmt.Errorf("ice provider: unknown provider %q (valid: twilio, xirsys, metered, embedded, static)", cfg.ICEProviderName)
	}
}

// min is a local helper (Go 1.21 has builtin min, but we target 1.22 anyway).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// defaultHTTPTimeout is used by all provider HTTP clients.
const defaultHTTPTimeout = 10 * time.Second
