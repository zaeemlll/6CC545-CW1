package peerfiles

import (
	"context"
	"encoding/hex"
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
	if source.VersioningEnabled() {
		request.Header.Set("X-File-Version", strconv.FormatUint(metadata.Version, 10))
		request.Header.Set("X-File-Writer-ID", metadata.WriterID)
	}
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
	return download(ctx, peerURL, peerID, name, destination, timeout, c, nil)
}

// DownloadExpected verifies the HTTPS response against metadata obtained earlier
// from the peer before allowing the file to be committed.
func DownloadExpected(
	ctx context.Context,
	peerURL string,
	peerID string,
	name string,
	destination *storage.Store,
	timeout time.Duration,
	c config.Config,
	expected storage.Metadata,
) (storage.Metadata, error) {
	return download(ctx, peerURL, peerID, name, destination, timeout, c, &expected)
}

func download(
	ctx context.Context,
	peerURL string,
	peerID string,
	name string,
	destination *storage.Store,
	timeout time.Duration,
	c config.Config,
	expected *storage.Metadata,
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
	if expected != nil {
		if !destination.VersioningEnabled() {
			return storage.Metadata{}, errors.New("expected metadata requires versioned destination storage")
		}
		if expected.Name != name || expected.Size < 0 ||
			expected.Version == 0 || !config.ValidID(expected.WriterID) ||
			len(expected.SHA256) != 64 {
			return storage.Metadata{}, errors.New("invalid expected file metadata")
		}
		if _, err := hex.DecodeString(expected.SHA256); err != nil {
			return storage.Metadata{}, errors.New("invalid expected file digest")
		}
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
	if _, err := hex.DecodeString(digest); err != nil {
		return storage.Metadata{}, errors.New("peer returned an invalid file digest")
	}
	if expected != nil &&
		(size != expected.Size || !strings.EqualFold(digest, expected.SHA256)) {
		return storage.Metadata{}, errors.New("downloaded file content metadata differs from advertised metadata")
	}

	var metadata storage.Metadata
	if destination.VersioningEnabled() {
		version, parseErr := strconv.ParseUint(response.Header.Get("X-File-Version"), 10, 64)
		writerID := response.Header.Get("X-File-Writer-ID")
		if parseErr != nil || version == 0 || !config.ValidID(writerID) {
			return storage.Metadata{}, errors.New("peer returned invalid file version metadata")
		}
		if expected != nil &&
			(version != expected.Version || writerID != expected.WriterID) {
			return storage.Metadata{}, errors.New("downloaded file version metadata differs from advertised metadata")
		}

		metadata, err = destination.PutVersioned(opCtx, name, response.Body, storage.Metadata{
			Name: name, Size: size, SHA256: digest,
			Version: version, WriterID: writerID,
		})
	} else {
		metadata, err = destination.Put(opCtx, name, response.Body, size, digest)
	}
	if err != nil {
		return storage.Metadata{}, fmt.Errorf("verify and store downloaded file: %w", err)
	}

	return metadata, nil
}
