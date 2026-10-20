package httpapi

import (
	"net/http"

	"example.edu/6cc545/cw1/internal/state"
)

func Messages(inbox *state.Inbox) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		WriteJSON(w, http.StatusOK, inbox.Snapshot())
	}
}
