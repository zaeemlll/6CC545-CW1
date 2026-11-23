package peerfiles

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
)

func writePeerTransferCertificate(t *testing.T, dir, name, identity string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, serial int64) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	if identity != "" {
		uri, err := url.Parse("spiffe://cw2.local/" + identity)
		if err != nil {
			t.Fatal(err)
		}
		template.URIs = []*url.URL{uri}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	certPath := filepath.Join(dir, name+".crt")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: certDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, name+".key")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "EC PRIVATE KEY", Bytes: keyDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	return certPath, keyPath
}

func peerTransferTLSConfig(t *testing.T) (config.Config, string, string, string, string) {
	t.Helper()

	dir := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "peer transfer test CA"},
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

	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: caDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	serverCert, serverKey := writePeerTransferCertificate(t, dir, "server", "", ca, caKey, 2)
	clientCert, clientKey := writePeerTransferCertificate(t, dir, "client", "node-a", ca, caKey, 3)

	return config.Config{
		SecurityMode: "required",
		TLSCertFile:  clientCert,
		TLSKeyFile:   clientKey,
		CAFile:       caPath,
	}, serverCert, serverKey, clientCert, caPath
}

func TestPeerFileUploadAndDownloadUseMutualTLS(t *testing.T) {
	cfg, serverCert, serverKey, _, caPath := peerTransferTLSConfig(t)

	serverStore, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	clientStore, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	downloadStore, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("secure peer transfer")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	if _, err := clientStore.Put(context.Background(), "report.txt", bytes.NewReader(content), int64(len(content)), digest); err != nil {
		t.Fatal(err)
	}

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-a",
		HTTPAddress: "127.0.0.1:8081",
		GRPCAddress: "127.0.0.1:9091",
		Status:      "online",
	}); err != nil {
		t.Fatal(err)
	}

	serverTLS, err := security.ServerTLS(serverCert, serverKey, caPath, true)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(httpapi.PeerFiles(serverStore, registry, 1024))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	uploaded, err := Upload(context.Background(), peerURL, "localhost", "report.txt", clientStore, 3*time.Second, cfg)
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if uploaded.Size != int64(len(content)) || uploaded.SHA256 != digest {
		t.Fatalf("uploaded metadata = %+v", uploaded)
	}

	downloaded, err := Download(context.Background(), peerURL, "localhost", "report.txt", downloadStore, 3*time.Second, cfg)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if downloaded.Size != int64(len(content)) || downloaded.SHA256 != digest {
		t.Fatalf("downloaded metadata = %+v", downloaded)
	}

	got, _, err := downloadStore.Open("report.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	actual, err := io.ReadAll(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(content) {
		t.Fatalf("downloaded content = %q, want %q", actual, content)
	}
}

func TestPeerFileTransferRejectsInsecureConfigurationAndInvalidNames(t *testing.T) {
	cfg := config.Config{SecurityMode: "required"}

	if _, _, err := newClient("http://node-b:9444", "node-b", time.Second, cfg); err == nil {
		t.Fatal("accepted an insecure HTTP peer URL")
	}
	if _, _, err := newClient("https://node-b:9444", "node-c", time.Second, cfg); err == nil {
		t.Fatal("accepted a peer URL with a different hostname")
	}
	if err := storage.ValidateName("../secret.txt"); err == nil {
		t.Fatal("accepted an invalid file name")
	}
}
