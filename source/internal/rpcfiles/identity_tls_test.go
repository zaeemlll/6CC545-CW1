//go:build acceptance

package rpcfiles

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func writeIdentityTLSFixtures(t *testing.T) (string, string, string, string, string) {
	t.Helper()

	dir := t.TempDir()
	now := time.Now()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "CW2 test CA"},
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

	writeNode := func(id string, serial int64) (string, string) {
		t.Helper()

		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		identityURI, err := url.Parse("spiffe://cw2.local/" + id)
		if err != nil {
			t.Fatal(err)
		}

		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: id},
			DNSNames:     []string{id},
			URIs:         []*url.URL{identityURI},
			NotBefore:    now.Add(-time.Hour),
			NotAfter:     now.Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
				x509.ExtKeyUsageClientAuth,
			},
		}

		certDER, err := x509.CreateCertificate(rand.Reader, template, caTemplate, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}

		certPath := filepath.Join(dir, id+".crt")
		if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
			Type: "CERTIFICATE", Bytes: certDER,
		}), 0600); err != nil {
			t.Fatal(err)
		}

		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}

		keyPath := filepath.Join(dir, id+".key")
		if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
			Type: "EC PRIVATE KEY", Bytes: keyDER,
		}), 0600); err != nil {
			t.Fatal(err)
		}

		return certPath, keyPath
	}

	nodeACert, nodeAKey := writeNode("node-a", 2)
	nodeBCert, nodeBKey := writeNode("node-b", 3)

	return nodeACert, nodeAKey, nodeBCert, nodeBKey, caPath
}

func TestWeek09_GRPCMutualTLSAndIdentity(t *testing.T) {
	nodeACert, nodeAKey, nodeBCert, nodeBKey, ca := writeIdentityTLSFixtures(t)

	serverTLS, err := security.ServerTLS(nodeBCert, nodeBKey, ca, true)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(serverTLS)),
		grpc.MaxRecvMsgSize(4096),
		grpc.MaxSendMsgSize(65536),
	)
	pb.RegisterFileServiceServer(server, &Service{
		NodeID:              "node-b",
		Catalog:             state.NewCatalog(),
		RequirePeerIdentity: true,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	defer func() {
		server.Stop()
		_ = listener.Close()
		<-done
	}()

	clientConfig := config.Config{
		SecurityMode: "required",
		TLSCertFile:  nodeACert,
		TLSKeyFile:   nodeAKey,
		CAFile:       ca,
	}

	_, err = Fetch(context.Background(), listener.Addr().String(), "node-b", "node-a", time.Second, clientConfig)
	if err != nil {
		t.Fatalf("trusted node-a request failed: %v", err)
	}

	_, err = Fetch(context.Background(), listener.Addr().String(), "node-b", "node-c", time.Second, clientConfig)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("identity mismatch status = %v, want PermissionDenied; error: %v",
			status.Code(err), err)
	}

	untrustedCert, untrustedKey := writeUntrustedClientTLSFixture(t)
	untrustedConfig := clientConfig
	untrustedConfig.TLSCertFile = untrustedCert
	untrustedConfig.TLSKeyFile = untrustedKey

	_, err = Fetch(
		context.Background(),
		listener.Addr().String(),
		"node-b",
		"node-a",
		time.Second,
		untrustedConfig,
	)
	if err == nil {
		t.Fatal("untrusted client certificate was accepted")
	}
}

func writeUntrustedClientTLSFixture(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	now := time.Now()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Untrusted test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	_, err = x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	identityURI, err := url.Parse("spiffe://cw2.local/node-a")
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(11),
		Subject:      pkix.Name{CommonName: "node-a"},
		DNSNames:     []string{"node-a"},
		URIs:         []*url.URL{identityURI},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caTemplate, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	certPath := filepath.Join(dir, "untrusted-node.crt")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: certDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	keyPath := filepath.Join(dir, "untrusted-node.key")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "EC PRIVATE KEY", Bytes: keyDER,
	}), 0600); err != nil {
		t.Fatal(err)
	}

	return certPath, keyPath
}
