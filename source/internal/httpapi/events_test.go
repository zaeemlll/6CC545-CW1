package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.edu/6cc545/cw1/internal/events"
)

func TestEventsHandler(t *testing.T) {
	bus, err := events.New(4)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	sub, err := bus.Subscribe("file.added")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Cancel()

	handler := Events(bus)

	tests := []struct {
		name   string
		method string
		body   string
		media  string
		want   int
	}{
		{
			name:   "valid event",
			method: http.MethodPost,
			body:   `{"id":"event-1","kind":"file.added","origin":"node-a","name":"notes.txt"}`,
			media:  "application/json",
			want:   http.StatusAccepted,
		},
		{
			name:   "duplicate event",
			method: http.MethodPost,
			body:   `{"id":"event-1","kind":"file.added","origin":"node-a","name":"notes.txt"}`,
			media:  "application/json",
			want:   http.StatusOK,
		},
		{
			name:   "invalid event",
			method: http.MethodPost,
			body:   `{"id":"","kind":"file.added","origin":"node-a","name":"notes.txt"}`,
			media:  "application/json",
			want:   http.StatusBadRequest,
		},
		{
			name:   "unknown field",
			method: http.MethodPost,
			body:   `{"id":"event-2","kind":"file.added","origin":"node-a","name":"notes.txt","extra":true}`,
			media:  "application/json",
			want:   http.StatusBadRequest,
		},
		{
			name:   "wrong media type",
			method: http.MethodPost,
			body:   `{}`,
			media:  "text/plain",
			want:   http.StatusUnsupportedMediaType,
		},
		{
			name:   "wrong method",
			method: http.MethodGet,
			want:   http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(
				tc.method,
				"/v1/events",
				strings.NewReader(tc.body),
			)
			if tc.media != "" {
				req.Header.Set("Content-Type", tc.media)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf(
					"want HTTP %d, got %d: %s",
					tc.want,
					rec.Code,
					rec.Body.String(),
				)
			}
		})
	}

	select {
	case got := <-sub.C:
		if got.ID != "event-1" || got.Name != "notes.txt" {
			t.Fatalf("unexpected delivered event: %+v", got)
		}
	default:
		t.Fatal("valid event was not delivered to subscriber")
	}

	select {
	case got := <-sub.C:
		t.Fatalf("duplicate event was delivered again: %+v", got)
	default:
	}
}

func TestEventsHandlerClosedBus(t *testing.T) {
	bus, err := events.New(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/events",
		strings.NewReader(
			`{"id":"event-1","kind":"file.added","origin":"node-a","name":"notes.txt"}`,
		),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	Events(bus).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want HTTP 503, got %d: %s", rec.Code, rec.Body.String())
	}
}
