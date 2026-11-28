//go:build acceptance

package app

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
)

func writeForwardingTLSFixtures(t *testing.T) (string, string, string) {
	t.Helper()

	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "forwarding test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	caPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: caDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	nodeTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "node-b"},
		DNSNames:     []string{"node-b"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	nodeDER, err := x509.CreateCertificate(rand.Reader, nodeTemplate, caTemplate, &nodeKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	certPath := filepath.Join(dir, "node.crt")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: nodeDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(nodeKey)
	if err != nil {
		t.Fatal(err)
	}

	keyPath := filepath.Join(dir, "node.key")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "EC PRIVATE KEY", Bytes: keyDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	return certPath, keyPath, caPath
}

func TestForwardEventToPeersUsesMutualTLS(t *testing.T) {
	cert, key, ca := writeForwardingTLSFixtures(t)

	serverTLS, err := security.ServerTLS(cert, key, ca, true)
	if err != nil {
		t.Fatal(err)
	}

	var received events.Event
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/peer/announce" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode event: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: peerURL,
		GRPCAddress: "node-b:9443",
		Status:      "online",
	}); err != nil {
		t.Fatal(err)
	}

	event := events.Event{
		ID:     "secure-event-1",
		Kind:   "file.added",
		Origin: "node-a",
		Name:   "notes.txt",
	}

	err = forwardEventToPeers(
		context.Background(),
		registry,
		"node-a",
		event,
		config.Config{
			SecurityMode: "required",
			TLSCertFile:  cert,
			TLSKeyFile:   key,
			CAFile:       ca,
		},
	)
	if err != nil {
		t.Fatalf("secure event forwarding failed: %v", err)
	}

	if received != event {
		t.Fatalf("received event = %+v, want %+v", received, event)
	}
}

func TestForwardMembershipToPeersUsesMutualTLS(t *testing.T) {
	cert, key, ca := writeForwardingTLSFixtures(t)

	serverTLS, err := security.ServerTLS(cert, key, ca, true)
	if err != nil {
		t.Fatal(err)
	}

	expected := membership.Announcement{
		ID:          "node-a",
		Address:     "https://node-a:9444",
		Incarnation: 3,
		Sequence:    7,
	}

	var received membership.Announcement
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/peer/membership" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
			t.Error("request did not present a verified client certificate")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode announcement: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: peerURL,
		GRPCAddress: "node-b:9443",
		Status:      "online",
	}); err != nil {
		t.Fatal(err)
	}

	err = forwardMembershipToPeers(
		context.Background(),
		registry,
		"node-a",
		expected,
		config.Config{
			SecurityMode: "required",
			TLSCertFile:  cert,
			TLSKeyFile:   key,
			CAFile:       ca,
			Timeout:      2 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("secure membership forwarding failed: %v", err)
	}

	if received != expected {
		t.Fatalf("received announcement = %+v, want %+v", received, expected)
	}
}
