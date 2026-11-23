//go:build acceptance

package app

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/config"
)

func writePublicHTTPSTestFixtures(t *testing.T) (string, string, string) {
	t.Helper()

	dir := t.TempDir()
	now := time.Now()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "public HTTPS test CA"},
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
		SerialNumber: big.NewInt(11),
		Subject:      pkix.Name{CommonName: "node-a"},
		DNSNames:     []string{"localhost", "node-a"},
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

func publicHTTPSTestAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func TestWeek09_PublicHTTPSLifecycle(t *testing.T) {
	cert, key, ca := writePublicHTTPSTestFixtures(t)

	jwtKeyPath := filepath.Join(t.TempDir(), "jwt.key")
	jwtKey := []byte("local-test-only-32-byte-randomness-not-for-deployment")
	if err := os.WriteFile(jwtKeyPath, jwtKey, 0600); err != nil {
		t.Fatal(err)
	}

	httpAddress := publicHTTPSTestAddress(t)
	grpcAddress := publicHTTPSTestAddress(t)
	peerAddress := publicHTTPSTestAddress(t)

	c := config.Config{
		ID:                "node-a",
		HTTP:              httpAddress,
		GRPC:              grpcAddress,
		GRPCAdvertise:     "node-a:9443",
		HTTPAdvertise:     "https://node-a:8443",
		PeerHTTP:          peerAddress,
		PeerHTTPAdvertise: "https://node-a:9444",
		TLSCertFile:       cert,
		TLSKeyFile:        key,
		CAFile:            ca,
		JWTKeyFile:        jwtKeyPath,
		JWTIssuer:         "cw2-lab",
		JWTAudience:       "cw2-api",
		SecurityMode:      "required",
		Timeout:           2 * time.Second,
		DataDir:           filepath.Join(t.TempDir(), "node-data"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(ctx, c)
	}()
	t.Cleanup(func() {
		cancel()
	})

	roots := x509.NewCertPool()
	caPEM, err := os.ReadFile(ca)
	if err != nil {
		t.Fatal(err)
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("could not load test CA certificate")
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				ServerName: "localhost",
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	baseURL := "https://" + httpAddress
	var healthResponse *http.Response
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		healthResponse, err = client.Get(baseURL + "/healthz")
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("public HTTPS endpoint did not start: %v", err)
	}
	defer healthResponse.Body.Close()

	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthResponse.StatusCode, http.StatusOK)
	}

	statusResponse, err := client.Get(baseURL + "/v1/status")
	if err != nil {
		t.Fatalf("request protected status endpoint: %v", err)
	}
	defer statusResponse.Body.Close()

	if statusResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status endpoint returned %d, want %d", statusResponse.StatusCode, http.StatusUnauthorized)
	}

	privateURL := "https://" + peerAddress + "/v1/peer/announce"
	privateResponse, privateErr := client.Get(privateURL)
	if privateErr == nil {
		privateResponse.Body.Close()
		t.Fatal("private peer HTTPS accepted a client without a certificate")
	}

	cancel()
	select {
	case runErr := <-runDone:
		if runErr != nil {
			t.Fatalf("application shutdown: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal(fmt.Sprintf("application did not stop within the shutdown deadline"))
	}
}
