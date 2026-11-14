package httpapi

import (
	"errors"
	"net/http"

	"example.edu/6cc545/cw1/internal/events"
)

// Events receives event metadata from another node.
func Events(bus *events.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		var event events.Event
		if err := DecodeOne(w, r, &event); err != nil {
			RejectInput(w, err)
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
