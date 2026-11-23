package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/state"
)

// PeerAnnounce receives event metadata from a registered peer over mTLS.
func PeerAnnounce(bus *events.Bus, registry *state.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		if bus == nil || registry == nil {
			ErrorJSON(w, http.StatusServiceUnavailable, "peer_service_unavailable")
			return
		}

		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 ||
			len(r.TLS.VerifiedChains[0]) == 0 {
			ErrorJSON(w, http.StatusUnauthorized, "verified_peer_certificate_required")
			return
		}

		leaf := r.TLS.VerifiedChains[0][0]
		if len(leaf.URIs) != 1 {
			ErrorJSON(w, http.StatusForbidden, "invalid_peer_identity")
			return
		}

		identity := leaf.URIs[0].String()
		const identityPrefix = "spiffe://cw2.local/"
		if !strings.HasPrefix(identity, identityPrefix) {
			ErrorJSON(w, http.StatusForbidden, "invalid_peer_identity")
			return
		}

		peerID := strings.TrimPrefix(identity, identityPrefix)
		if !config.ValidID(peerID) || identity != identityPrefix+peerID {
			ErrorJSON(w, http.StatusForbidden, "invalid_peer_identity")
			return
		}

		if _, err := registry.Get(peerID); err != nil {
			ErrorJSON(w, http.StatusForbidden, "unregistered_peer")
			return
		}

		var event events.Event
		if err := DecodeOne(w, r, &event); err != nil {
			RejectInput(w, err)
			return
		}

		if event.Origin != peerID {
			ErrorJSON(w, http.StatusForbidden, "event_origin_mismatch")
			return
		}

		delivery, err := bus.Publish(event)
		if err != nil {
			if errors.Is(err, events.ErrClosed) {
				ErrorJSON(w, http.StatusServiceUnavailable, "event_bus_unavailable")
				return
			}
			ErrorJSON(w, http.StatusBadRequest, "invalid_event")
			return
		}

		if delivery.Duplicate {
			WriteJSON(w, http.StatusOK, delivery)
			return
		}

		WriteJSON(w, http.StatusAccepted, delivery)
	}
}
