package rpcfiles

import (
	"errors"

	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/storage"
)

// StorageCatalog adapts the authoritative storage store to the
// catalogue interface used by the existing gRPC service.
type StorageCatalog struct {
	store *storage.Store
}

// NewStorageCatalog creates a catalogue backed by a valid storage store.
func NewStorageCatalog(store *storage.Store) (StorageCatalog, error) {
	if store == nil {
		return StorageCatalog{}, errors.New("storage store is required")
	}

	return StorageCatalog{store: store}, nil
}

// Snapshot preserves the original catalogue interface for existing callers.
func (c StorageCatalog) Snapshot() []model.FileMetadata {
	files, _ := c.SnapshotWithError()
	return files
}

// SnapshotWithError returns committed metadata without hiding storage errors.
func (c StorageCatalog) SnapshotWithError() ([]model.FileMetadata, error) {
	if c.store == nil {
		return nil, errors.New("storage store is unavailable")
	}

	metadata, err := c.store.Snapshot()
	if err != nil {
		return nil, err
	}

	files := make([]model.FileMetadata, 0, len(metadata))
	for _, file := range metadata {
		files = append(files, model.FileMetadata{
			Name:         file.Name,
			Size:         file.Size,
			SHA256:       file.SHA256,
			ModifiedUnix: file.ModifiedUnix,
			Version:      file.Version,
			WriterID:     file.WriterID,
		})
	}

	return files, nil
}
