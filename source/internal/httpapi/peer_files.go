package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
)

// PeerFiles handles private file transfers between authenticated peers.
func PeerFiles(store *storage.Store, registry *state.Registry, maxBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil || registry == nil || maxBytes < 1 {
			ErrorJSON(w, http.StatusInternalServerError, "storage_unavailable")
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

		const prefix = "/v1/peer/files/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			ErrorJSON(w, http.StatusNotFound, "not_found")
			return
		}

		name := strings.TrimPrefix(r.URL.Path, prefix)
		if err := storage.ValidateName(name); err != nil {
			ErrorJSON(w, http.StatusBadRequest, "invalid_file_name")
			return
		}

		switch r.Method {
		case http.MethodGet:
			reader, metadata, err := store.Open(name)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					ErrorJSON(w, http.StatusNotFound, "file_not_found")
					return
				}
				log.Printf("peer file read failed peer=%s name=%s: %v", peerID, name, err)
				ErrorJSON(w, http.StatusInternalServerError, "storage_read_failed")
				return
			}
			defer reader.Close()

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
			w.Header().Set("X-File-Size", strconv.FormatInt(metadata.Size, 10))
			w.Header().Set("X-File-SHA256", metadata.SHA256)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusOK)

			if _, err := io.Copy(w, reader); err != nil {
				log.Printf("peer file response interrupted peer=%s name=%s: %v", peerID, name, err)
			}

		case http.MethodPost:
			size, err := strconv.ParseInt(r.Header.Get("X-File-Size"), 10, 64)
			if err != nil || size < 0 {
				ErrorJSON(w, http.StatusBadRequest, "invalid_file_size")
				return
			}
			if size > maxBytes {
				ErrorJSON(w, http.StatusRequestEntityTooLarge, "file_too_large")
				return
			}
			if r.ContentLength >= 0 && r.ContentLength != size {
				ErrorJSON(w, http.StatusBadRequest, "content_length_mismatch")
				return
			}

			body := http.MaxBytesReader(w, r.Body, maxBytes+1)
			metadata, err := store.Put(
				r.Context(),
				name,
				body,
				size,
				r.Header.Get("X-File-SHA256"),
			)
			if err != nil {
				var maxErr *http.MaxBytesError
				switch {
				case errors.Is(err, storage.ErrTooLarge), errors.As(err, &maxErr):
					ErrorJSON(w, http.StatusRequestEntityTooLarge, "file_too_large")
				case errors.Is(err, storage.ErrInvalidName):
					ErrorJSON(w, http.StatusBadRequest, "invalid_file_name")
				case errors.Is(err, storage.ErrIntegrity):
					ErrorJSON(w, http.StatusUnprocessableEntity, "file_integrity_failed")
				default:
					log.Printf("peer file upload failed peer=%s name=%s: %v", peerID, name, err)
					ErrorJSON(w, http.StatusInternalServerError, "storage_failed")
				}
				return
			}

			WriteJSON(w, http.StatusCreated, map[string]any{"file": metadata})

		default:
			w.Header().Set("Allow", "GET, POST")
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	}
}
