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
	"sync/atomic"

	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/storage"
)

type storedFileHandler struct {
	store    *storage.Store
	bus      *events.Bus
	nodeID   string
	maxBytes int64
	nextID   atomic.Uint64
}

// StoredFile handles uploads and downloads for one stored filename.
// The caller registers it at /v1/files/.
func StoredFile(store *storage.Store, bus *events.Bus, nodeID string, maxBytes int64) http.HandlerFunc {
	h := &storedFileHandler{
		store:    store,
		bus:      bus,
		nodeID:   nodeID,
		maxBytes: maxBytes,
	}
	return h.serve
}

func (h *storedFileHandler) serve(w http.ResponseWriter, r *http.Request) {
	const prefix = "/v1/files/"

	if h.store == nil || h.bus == nil || h.nodeID == "" || h.maxBytes < 1 {
		ErrorJSON(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}

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
	case http.MethodPost:
		h.upload(w, r, name)
	case http.MethodGet:
		h.download(w, r, name)
	default:
		w.Header().Set("Allow", "GET, POST")
		ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (h *storedFileHandler) upload(w http.ResponseWriter, r *http.Request, name string) {
	size, err := strconv.ParseInt(r.Header.Get("X-File-Size"), 10, 64)
	if err != nil || size < 0 {
		ErrorJSON(w, http.StatusBadRequest, "invalid_file_size")
		return
	}

	if size > h.maxBytes {
		ErrorJSON(w, http.StatusRequestEntityTooLarge, "file_too_large")
		return
	}

	if r.ContentLength >= 0 && r.ContentLength != size {
		ErrorJSON(w, http.StatusBadRequest, "content_length_mismatch")
		return
	}

	sha := r.Header.Get("X-File-SHA256")
	body := http.MaxBytesReader(w, r.Body, h.maxBytes+1)

	metadata, err := h.store.Put(r.Context(), name, body, size, sha)
	if err != nil {
		var maxErr *http.MaxBytesError

		switch {
		case errors.Is(err, storage.ErrTooLarge):
			ErrorJSON(w, http.StatusRequestEntityTooLarge, "file_too_large")
		case errors.As(err, &maxErr):
			ErrorJSON(w, http.StatusRequestEntityTooLarge, "file_too_large")
		case errors.Is(err, storage.ErrInvalidName):
			ErrorJSON(w, http.StatusBadRequest, "invalid_file_name")
		case errors.Is(err, storage.ErrIntegrity):
			ErrorJSON(w, http.StatusUnprocessableEntity, "file_integrity_failed")
		default:
			log.Printf("file upload failed name=%s: %v", name, err)
			ErrorJSON(w, http.StatusInternalServerError, "storage_failed")
		}
		return
	}

	event := events.Event{
		ID:     fmt.Sprintf("%s-file-%d", h.nodeID, h.nextID.Add(1)),
		Kind:   "file.added",
		Origin: h.nodeID,
		Name:   name,
	}

	delivery, publishErr := h.bus.Publish(event)

	response := map[string]any{
		"file":           metadata,
		"event_delivery": delivery,
	}

	if publishErr != nil {
		log.Printf("file committed but event publish failed name=%s: %v", name, publishErr)
		response["event_delivery_error"] = publishErr.Error()
	} else if delivery.Dropped > 0 {
		log.Printf("file committed but event notification dropped name=%s subscribers=%d",
			name, delivery.Dropped)
	}

	WriteJSON(w, http.StatusCreated, response)
}

func (h *storedFileHandler) download(w http.ResponseWriter, r *http.Request, name string) {
	reader, metadata, err := h.store.Open(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ErrorJSON(w, http.StatusNotFound, "file_not_found")
			return
		}
		if errors.Is(err, storage.ErrInvalidName) {
			ErrorJSON(w, http.StatusBadRequest, "invalid_file_name")
			return
		}

		log.Printf("file read failed name=%s: %v", name, err)
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
		log.Printf("file response interrupted name=%s: %v", name, err)
	}
}
