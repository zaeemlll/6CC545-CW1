package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/state"
)

// PeerMembership accepts membership announcements from enrolled peers over mTLS.
func PeerMembership(table *membership.Table, registry *state.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		if table == nil || registry == nil {
			ErrorJSON(w, http.StatusServiceUnavailable, "membership_service_unavailable")
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

		enrolledPeer, err := registry.Get(peerID)
		if err != nil {
			ErrorJSON(w, http.StatusForbidden, "unregistered_peer")
			return
		}

		var announcement membership.Announcement
		if err := DecodeOne(w, r, &announcement); err != nil {
			RejectInput(w, err)
			return
		}

		if announcement.Address != enrolledPeer.HTTPAddress {
			ErrorJSON(w, http.StatusForbidden, "peer_address_mismatch")
			return
		}

		err = table.Accept(announcement, peerID, time.Now())
		if err != nil {
			switch {
			case errors.Is(err, membership.ErrReplay):
				ErrorJSON(w, http.StatusConflict, "membership_replay")
			case errors.Is(err, membership.ErrIdentity):
				ErrorJSON(w, http.StatusForbidden, "invalid_peer_identity")
			case errors.Is(err, membership.ErrCapacity):
				ErrorJSON(w, http.StatusServiceUnavailable, "membership_capacity_reached")
			default:
				ErrorJSON(w, http.StatusServiceUnavailable, "membership_update_failed")
			}
			return
		}

		if err := registry.SetStatus(peerID, "online"); err != nil {
			ErrorJSON(w, http.StatusServiceUnavailable, "peer_status_update_failed")
			return
		}

		WriteJSON(w, http.StatusAccepted, map[string]string{
			"status": "accepted",
		})
	}
}
