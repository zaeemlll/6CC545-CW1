package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/storage"
)

var ErrNotImplemented = errors.New("TODO W10: verified transfer not implemented")

const transferTimeout = 30 * time.Second

type Request struct {
	URL, AllowedHost, Name string
	Size                   int64
	SHA256                 string
	Version                uint64
	WriterID               string
}

func Fetch(ctx context.Context, client *http.Client, store *storage.Store, r Request) (storage.Metadata, error) {
	if ctx == nil || client == nil || store == nil {
		return storage.Metadata{}, errors.New("context, HTTP client and storage are required")
	}
	if err := storage.ValidateName(r.Name); err != nil {
		return storage.Metadata{}, err
	}
	if r.Size < 0 {
		return storage.Metadata{}, storage.ErrIntegrity
	}
	if len(r.SHA256) != sha256.Size*2 {
		return storage.Metadata{}, storage.ErrIntegrity
	}
	if _, err := hex.DecodeString(r.SHA256); err != nil {
		return storage.Metadata{}, storage.ErrIntegrity
	}

	u, err := url.Parse(r.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" ||
		u.User != nil || u.Fragment != "" {
		return storage.Metadata{}, errors.New("invalid HTTPS transfer URL")
	}
	if r.AllowedHost == "" || !strings.EqualFold(u.Host, r.AllowedHost) {
		return storage.Metadata{}, errors.New("transfer host is not authorised")
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, transferTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return storage.Metadata{}, err
	}

	safeClient := *client
	safeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := safeClient.Do(req)
	if err != nil {
		return storage.Metadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return storage.Metadata{}, fmt.Errorf("unexpected transfer status: %d", resp.StatusCode)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != r.Size {
		return storage.Metadata{}, storage.ErrIntegrity
	}

	if store.VersioningEnabled() {
		if r.Version == 0 || !config.ValidID(r.WriterID) {
			return storage.Metadata{}, storage.ErrInvalidVersionMetadata
		}
		return store.PutVersioned(deadlineCtx, r.Name, resp.Body, storage.Metadata{
			Name:     r.Name,
			Size:     r.Size,
			SHA256:   strings.ToLower(r.SHA256),
			Version:  r.Version,
			WriterID: r.WriterID,
		})
	}

	return store.Put(deadlineCtx, r.Name, resp.Body, r.Size, r.SHA256)
}
