package httpapi

import (
	"net/http"

	"example.edu/6cc545/cw1/internal/storage"
)

// StoredFiles returns the metadata for committed files.
func StoredFiles(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		if store == nil {
			ErrorJSON(w, http.StatusInternalServerError, "storage_unavailable")
			return
		}

		files, err := store.Snapshot()
		if err != nil {
			ErrorJSON(w, http.StatusInternalServerError, "storage_read_failed")
			return
		}

		WriteJSON(w, http.StatusOK, files)
	}
}
