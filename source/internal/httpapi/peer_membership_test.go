package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
)

func peerMembershipTestSetup(t *testing.T) (*membership.Table, *state.Registry, http.HandlerFunc) {
	t.Helper()

	table, err := membership.New(8, 4*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: "https://node-b:9444",
		GRPCAddress: "node-b:9443",
		Status:      "unknown",
	}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	return table, registry, PeerMembership(table, registry)
}

func peerMembershipRequest(body string, certificate *x509.Certificate) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/peer/membership",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	if certificate != nil {
		request.TLS = &tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{{certificate}},
		}
	}

	return request
}

func TestPeerMembershipRejectsMissingVerifiedCertificate(t *testing.T) {
	_, _, handler := peerMembershipTestSetup(t)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, peerMembershipRequest(
		`{"id":"node-b","address":"https://node-b:9444","incarnation":1,"sequence":1}`,
		nil,
	))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestPeerMembershipRejectsUnregisteredPeer(t *testing.T) {
	_, _, handler := peerMembershipTestSetup(t)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, peerMembershipRequest(
		`{"id":"node-c","address":"https://node-c:9444","incarnation":1,"sequence":1}`,
		peerFileTestCertificate(t, "node-c"),
	))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPeerMembershipRejectsAddressMismatch(t *testing.T) {
	table, _, handler := peerMembershipTestSetup(t)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, peerMembershipRequest(
		`{"id":"node-b","address":"https://node-c:9444","incarnation":1,"sequence":1}`,
		peerFileTestCertificate(t, "node-b"),
	))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if len(table.Snapshot()) != 0 {
		t.Fatal("address mismatch changed membership state")
	}
}

func TestPeerMembershipAcceptsAnnouncementAndRejectsReplay(t *testing.T) {
	table, _, handler := peerMembershipTestSetup(t)

	body := `{"id":"node-b","address":"https://node-b:9444","incarnation":1,"sequence":1}`
	certificate := peerFileTestCertificate(t, "node-b")

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, peerMembershipRequest(body, certificate))

	if first.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d; body: %s",
			first.Code, http.StatusAccepted, first.Body.String())
	}

	peers := table.Snapshot()
	if len(peers) != 1 || peers[0].ID != "node-b" {
		t.Fatalf("membership snapshot = %+v, want node-b", peers)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, peerMembershipRequest(body, certificate))

	if second.Code != http.StatusConflict {
		t.Fatalf("replay status = %d, want %d; body: %s",
			second.Code, http.StatusConflict, second.Body.String())
	}
	if len(table.Snapshot()) != 1 {
		t.Fatal("rejected replay changed membership state")
	}
}

func TestPeerMembershipRejectsWrongMethod(t *testing.T) {
	_, _, handler := peerMembershipTestSetup(t)

	request := httptest.NewRequest(http.MethodGet, "/v1/peer/membership", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
