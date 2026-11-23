package security

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

var ErrUnauthenticated = errors.New("unauthenticated")
var ErrForbidden = errors.New("forbidden")

func loadCertificate(certFile, keyFile string) (tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return tls.Certificate{}, errors.New("TLS certificate and key paths are required")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load TLS certificate and key: %w", err)
	}

	if len(cert.Certificate) == 0 {
		return tls.Certificate{}, errors.New("TLS certificate chain is empty")
	}

	return cert, nil
}

func loadTrustPool(caFile string) (*x509.CertPool, error) {
	if caFile == "" {
		return nil, errors.New("CA certificate path is required")
	}

	pemData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(pemData); !ok {
		return nil, errors.New("CA file contains no valid certificates")
	}

	return pool, nil
}

func ServerTLS(certFile, keyFile, caFile string, requireClient bool) (*tls.Config, error) {
	cert, err := loadCertificate(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	pool, err := loadTrustPool(caFile)
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
	}

	if requireClient {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}

func ClientTLS(certFile, keyFile, caFile, serverName string) (*tls.Config, error) {
	if serverName == "" {
		return nil, errors.New("TLS server name is required")
	}

	cert, err := loadCertificate(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	pool, err := loadTrustPool(caFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   serverName,
	}, nil
}
