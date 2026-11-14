package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
)

func TestForwardEventToPeers(t *testing.T) {
	var received events.Event

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode event: %v", err)
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	registry := state.NewRegistry()

	peer := model.Peer{
		ID:          "node-b",
		HTTPAddress: server.Listener.Addr().String(),
		GRPCAddress: "127.0.0.1:9092",
		Status:      "online",
	}

	if err := registry.Add(peer); err != nil {
		t.Fatal(err)
	}

	event := events.Event{
		ID:     "event-1",
		Kind:   "file.added",
		Origin: "node-a",
		Name:   "notes.txt",
	}

	if err := forwardEventToPeers(
		context.Background(),
		registry,
		"node-a",
		event,
	); err != nil {
		t.Fatalf("forward event: %v", err)
	}

	if received != event {
		t.Fatalf("received event = %+v, want %+v", received, event)
	}
}

func TestForwardEventSkipsLocalNode(t *testing.T) {
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	registry := state.NewRegistry()

	peer := model.Peer{
		ID:          "node-a",
		HTTPAddress: server.Listener.Addr().String(),
		GRPCAddress: "127.0.0.1:9091",
		Status:      "online",
	}

	if err := registry.Add(peer); err != nil {
		t.Fatal(err)
	}

	event := events.Event{
		ID:     "event-2",
		Kind:   "file.added",
		Origin: "node-a",
		Name:   "diagram.txt",
	}

	if err := forwardEventToPeers(
		context.Background(),
		registry,
		"node-a",
		event,
	); err != nil {
		t.Fatalf("forward event: %v", err)
	}

	if requests != 0 {
		t.Fatalf("expected no HTTP requests to the local node, got %d", requests)
	}
}

func TestForwardEventRejectsUnsuccessfulPeerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	registry := state.NewRegistry()

	peer := model.Peer{
		ID:          "node-b",
		HTTPAddress: server.Listener.Addr().String(),
		GRPCAddress: "127.0.0.1:9092",
		Status:      "online",
	}

	if err := registry.Add(peer); err != nil {
		t.Fatal(err)
	}

	event := events.Event{
		ID:     "event-3",
		Kind:   "file.added",
		Origin: "node-a",
		Name:   "notes.txt",
	}

	err := forwardEventToPeers(
		context.Background(),
		registry,
		"node-a",
		event,
	)

	if err == nil {
		t.Fatal("expected an error for HTTP 503 response")
	}
}
