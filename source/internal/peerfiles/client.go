package peerfiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/storage"
)

const peerFilesPath = "/v1/peer/files/"

func newClient(peerURL, peerID string, timeout time.Duration, c config.Config) (*http.Client, string, error) {
	if c.SecurityMode != "required" {
		return nil, "", errors.New("peer file transfer requires security mode")
	}
	if !config.ValidID(peerID) {
		return nil, "", errors.New("invalid peer ID")
	}
	if !config.ValidPeerHTTPSURL(peerURL) {
		return nil, "", errors.New("invalid peer HTTPS URL")
	}
	if timeout <= 0 {
		return nil, "", errors.New("invalid transfer timeout")
	}

	parsed, err := url.Parse(peerURL)
	if err != nil {
		return nil, "", err
	}
	if !strings.EqualFold(parsed.Hostname(), peerID) {
		return nil, "", errors.New("peer URL hostname does not match peer ID")
	}

	tlsConfig, err := security.ClientTLS(
		c.TLSCertFile,
		c.TLSKeyFile,
		c.CAFile,
		peerID,
	)
	if err != nil {
		return nil, "", fmt.Errorf("configure peer TLS: %w", err)
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	endpoint := strings.TrimRight(peerURL, "/") + peerFilesPath
	return client, endpoint, nil
}

// Upload sends a locally stored file to a peer over mutually authenticated HTTPS.
func Upload(
	ctx context.Context,
	peerURL string,
	peerID string,
	name string,
	source *storage.Store,
	timeout time.Duration,
	c config.Config,
) (storage.Metadata, error) {
	if ctx == nil {
		return storage.Metadata{}, errors.New("operation context is required")
	}
	if source == nil {
		return storage.Metadata{}, errors.New("source storage is required")
	}
	if err := storage.ValidateName(name); err != nil {
		return storage.Metadata{}, err
	}

	client, endpoint, err := newClient(peerURL, peerID, timeout, c)
	if err != nil {
		return storage.Metadata{}, err
	}
	defer client.CloseIdleConnections()

	content, metadata, err := source.Open(name)
	if err != nil {
		return storage.Metadata{}, err
	}
	defer content.Close()

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		opCtx,
		http.MethodPost,
		endpoint+name,
		content,
	)
	if err != nil {
		return storage.Metadata{}, err
	}
	request.ContentLength = metadata.Size
	request.Header.Set("X-File-Size", strconv.FormatInt(metadata.Size, 10))
	request.Header.Set("X-File-SHA256", metadata.SHA256)
	request.Header.Set("Content-Type", "application/octet-stream")

	response, err := client.Do(request)
	if err != nil {
		return storage.Metadata{}, fmt.Errorf("upload file to peer: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return storage.Metadata{}, fmt.Errorf("peer upload returned HTTP %d", response.StatusCode)
	}

	return metadata, nil
}

// Download retrieves a peer's file and stores it after size and SHA-256 verification.
func Download(
	ctx context.Context,
	peerURL string,
	peerID string,
	name string,
	destination *storage.Store,
	timeout time.Duration,
	c config.Config,
) (storage.Metadata, error) {
	if ctx == nil {
		return storage.Metadata{}, errors.New("operation context is required")
	}
	if destination == nil {
		return storage.Metadata{}, errors.New("destination storage is required")
	}
	if err := storage.ValidateName(name); err != nil {
		return storage.Metadata{}, err
	}

	client, endpoint, err := newClient(peerURL, peerID, timeout, c)
	if err != nil {
		return storage.Metadata{}, err
	}
	defer client.CloseIdleConnections()

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		opCtx,
		http.MethodGet,
		endpoint+name,
		nil,
	)
	if err != nil {
		return storage.Metadata{}, err
	}

	response, err := client.Do(request)
	if err != nil {
		return storage.Metadata{}, fmt.Errorf("download file from peer: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return storage.Metadata{}, fmt.Errorf("peer download returned HTTP %d", response.StatusCode)
	}

	size, err := strconv.ParseInt(response.Header.Get("X-File-Size"), 10, 64)
	if err != nil || size < 0 {
		return storage.Metadata{}, errors.New("peer returned an invalid file size")
	}

	if response.ContentLength >= 0 && response.ContentLength != size {
		return storage.Metadata{}, errors.New("peer response length does not match file size")
	}

	digest := response.Header.Get("X-File-SHA256")
	if len(digest) != 64 {
		return storage.Metadata{}, errors.New("peer returned an invalid file digest")
	}

	metadata, err := destination.Put(opCtx, name, response.Body, size, digest)
	if err != nil {
		return storage.Metadata{}, fmt.Errorf("verify and store downloaded file: %w", err)
	}

	return metadata, nil
}
