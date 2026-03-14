package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Josepavese/aftertalk/internal/bot/webrtc"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

// RTCConfigHandler serves GET /v1/rtc-config.
//
// It delegates entirely to the ICEProvider PAL interface — it has no knowledge
// of whether the credentials come from an embedded TURN server, Twilio, Xirsys,
// Metered, or a static list. The frontend SDK receives the same JSON shape
// regardless of the backing provider.
//
// Security: endpoint requires API key auth (TURN credentials are sensitive).
type RTCConfigHandler struct {
	provider webrtc.ICEProvider
	ttl      int // seconds; default 86400
}

// NewRTCConfigHandler builds the handler.
// provider must not be nil; use webrtc.NewStaticProvider for a no-op fallback.
func NewRTCConfigHandler(cfg *config.Config, provider webrtc.ICEProvider) *RTCConfigHandler {
	ttl := cfg.WebRTC.TURN.AuthTTL
	if ttl <= 0 {
		ttl = 86400
	}
	return &RTCConfigHandler{provider: provider, ttl: ttl}
}

type rtcConfigResponse struct {
	ICEServers []webrtc.ICEServer `json:"ice_servers"`
	TTL        int                `json:"ttl"`
	Provider   string             `json:"provider"`
}

// ServeHTTP handles GET /v1/rtc-config.
func (h *RTCConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	servers, err := h.provider.GetICEServers(r.Context(), h.ttl)
	if err != nil {
		logging.Errorf("rtc-config: get ice servers (%s): %v", h.provider.Name(), err)
		http.Error(w, "failed to get ICE servers", http.StatusInternalServerError)
		return
	}

	logging.Infof("rtc-config: served %d ICE servers via %s to %s",
		len(servers), h.provider.Name(), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rtcConfigResponse{
		ICEServers: servers,
		TTL:        h.ttl,
		Provider:   h.provider.Name(),
	}); err != nil {
		logging.Errorf("rtc-config: encode response: %v", err)
	}
}
