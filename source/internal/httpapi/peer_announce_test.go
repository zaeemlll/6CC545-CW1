package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.edu/6cc545/cw1/internal/events"
)

func TestPeerAnnounceRejectsMissingCertificate(t *testing.T) {
	bus, err := events.New(8)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	_, registry, _ := peerFileTestHandler(t)
	handler := PeerAnnounce(bus, registry)

	request := httptest.NewRequest(http.MethodPost, "/v1/peer/announce", strings.NewReader(
		`{"id":"event-1","kind":"file.added","origin":"node-b","name":"report.txt"}`,
	))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestPeerAnnounceRejectsUnregisteredPeer(t *testing.T) {
	bus, err := events.New(8)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	_, registry, _ := peerFileTestHandler(t)
	handler := PeerAnnounce(bus, registry)

	request := httptest.NewRequest(http.MethodPost, "/v1/peer/announce", strings.NewReader(
		`{"id":"event-1","kind":"file.added","origin":"node-c","name":"report.txt"}`,
	))
	request.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-c"),
		}},
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPeerAnnounceRejectsMismatchedEventOrigin(t *testing.T) {
	bus, err := events.New(8)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	_, registry, _ := peerFileTestHandler(t)
	handler := PeerAnnounce(bus, registry)

	request := httptest.NewRequest(http.MethodPost, "/v1/peer/announce", strings.NewReader(
		`{"id":"event-1","kind":"file.added","origin":"node-a","name":"report.txt"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPeerAnnounceAcceptsRegisteredPeerEvent(t *testing.T) {
	bus, err := events.New(8)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	subscription, err := bus.Subscribe("file.added")
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Cancel()

	_, registry, _ := peerFileTestHandler(t)
	handler := PeerAnnounce(bus, registry)

	request := httptest.NewRequest(http.MethodPost, "/v1/peer/announce", strings.NewReader(
		`{"id":"event-1","kind":"file.added","origin":"node-b","name":"report.txt"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusAccepted, response.Body.String())
	}

	select {
	case event := <-subscription.C:
		if event.ID != "event-1" || event.Origin != "node-b" {
			t.Fatalf("unexpected event: %+v", event)
		}
	default:
		t.Fatal("accepted event was not delivered to subscriber")
	}
}
