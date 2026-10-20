package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/state"
)

func TestWeek05_RemoteFilesValidation(t *testing.T) {
	h := RemoteFiles(state.NewRegistry(), "node-a", time.Second)

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"wrong method", http.MethodPost, "/v1/remote-files?peer_id=node-b", http.StatusMethodNotAllowed},
		{"missing peer", http.MethodGet, "/v1/remote-files", http.StatusBadRequest},
		{"invalid peer", http.MethodGet, "/v1/remote-files?peer_id=bad%20id", http.StatusBadRequest},
		{"unknown peer", http.MethodGet, "/v1/remote-files?peer_id=node-b", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.name == "wrong method" && rec.Header().Get("Allow") != "GET" {
				t.Fatalf("Allow header = %q, want GET", rec.Header().Get("Allow"))
			}
		})
	}
}
