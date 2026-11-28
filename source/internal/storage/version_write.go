package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PutLocalVersioned stores a local file and assigns its next logical version.
func (s *Store) PutLocalVersioned(
	ctx context.Context,
	name string,
	r io.Reader,
	size int64,
	digest string,
) (Metadata, error) {
	if s == nil {
		return Metadata{}, errors.New("invalid storage configuration")
	}

	s.mu.RLock()
	nodeID := s.nodeID
	enabled := s.versioned
	s.mu.RUnlock()

	if !enabled || nodeID == "" {
		return Metadata{}, errors.New("versioned storage is not enabled")
	}

	metadata := Metadata{
		Name:         name,
		Size:         size,
		SHA256:       strings.ToLower(digest),
		ModifiedUnix: time.Now().Unix(),
		Version:      1,
		WriterID:     nodeID,
	}

	return s.putVersioned(ctx, name, r, metadata, true)
}

// PutVersioned stores verified content while preserving its supplied version metadata.
func (s *Store) PutVersioned(
	ctx context.Context,
	name string,
	r io.Reader,
	metadata Metadata,
) (Metadata, error) {
	return s.putVersioned(ctx, name, r, metadata, false)
}

func (s *Store) putVersioned(
	ctx context.Context,
	name string,
	r io.Reader,
	metadata Metadata,
	local bool,
) (Metadata, error) {
	if s == nil || s.maxBytes < 1 {
		return Metadata{}, errors.New("invalid storage configuration")
	}
	if ctx == nil {
		return Metadata{}, errors.New("operation context is required")
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if err := ValidateName(name); err != nil {
		return Metadata{}, err
	}
	if r == nil {
		return Metadata{}, errors.New("source reader is required")
	}
	if metadata.Name != name {
		return Metadata{}, ErrInvalidVersionMetadata
	}
	if err := validateVersionedMetadata(metadata, s.maxBytes); err != nil {
		return Metadata{}, err
	}

	s.mu.RLock()
	enabled := s.versioned
	stateDir := s.versionStateDir
	root := s.root
	s.mu.RUnlock()

	if !enabled || stateDir == "" {
		return Metadata{}, errors.New("versioned storage is not enabled")
	}

	temp, err := os.CreateTemp(root, versionStagePrefix+"*")
	if err != nil {
		return Metadata{}, err
	}
	tempName := filepath.Base(temp.Name())
	tempPath := filepath.Join(root, tempName)
	defer os.Remove(tempPath)

	hasher := sha256.New()
	written, err := io.Copy(
		io.MultiWriter(temp, hasher),
		io.LimitReader(contextReader{ctx: ctx, r: r}, metadata.Size),
	)
	if err != nil {
		_ = temp.Close()
		return Metadata{}, err
	}
	if written != metadata.Size {
		_ = temp.Close()
		return Metadata{}, ErrIntegrity
	}

	var extra [1]byte
	n, extraErr := io.ReadFull(contextReader{ctx: ctx, r: r}, extra[:])
	if n != 0 || extraErr != io.EOF {
		_ = temp.Close()
		if extraErr != nil && extraErr != io.EOF {
			return Metadata{}, extraErr
		}
		return Metadata{}, ErrIntegrity
	}

	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if actualDigest != metadata.SHA256 {
		_ = temp.Close()
		return Metadata{}, ErrIntegrity
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return Metadata{}, err
	}
	if err := temp.Close(); err != nil {
		return Metadata{}, err
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.versioned || s.versionStateDir != stateDir || s.root != root {
		return Metadata{}, errors.New("versioned storage configuration changed")
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if s.recoveryRequired {
		return Metadata{}, errors.New("versioned storage requires recovery")
	}

	if local {
		if metadata.WriterID != s.nodeID || s.versionClock == ^uint64(0) {
			return Metadata{}, ErrInvalidVersionMetadata
		}
		metadata.Version = s.versionClock + 1
	} else if metadata.Version > s.versionClock {
		// A remote logical version may advance this node's local clock.
	}

	if err := validateVersionedMetadata(metadata, s.maxBytes); err != nil {
		return Metadata{}, err
	}

	if !local {
		if current, exists := s.files[name]; exists {
			switch {
			case metadata.Version < current.Version:
				return Metadata{}, ErrVersionConflict
			case metadata.Version == current.Version && metadata.WriterID < current.WriterID:
				return Metadata{}, ErrVersionConflict
			case metadata.Version == current.Version && metadata.WriterID == current.WriterID:
				if metadata.Size != current.Size || metadata.SHA256 != current.SHA256 {
					return Metadata{}, ErrVersionEquivocation
				}
				return current, nil
			}
		}
	}

	journal := versionJournal{
		Format:   versionStateFormat,
		TempName: tempName,
		Metadata: metadata,
	}
	if err := saveVersionJournal(stateDir, journal, s.maxBytes); err != nil {
		return Metadata{}, err
	}
	s.recoveryRequired = true

	destination := filepath.Join(root, name)
	if err := os.Rename(tempPath, destination); err != nil {
		return Metadata{}, err
	}
	if err := syncDirectory(root); err != nil {
		return Metadata{}, err
	}

	nextFiles := make(map[string]Metadata, len(s.files)+1)
	for existingName, existingMetadata := range s.files {
		nextFiles[existingName] = existingMetadata
	}
	nextFiles[name] = metadata

	nextClock := s.versionClock
	if metadata.Version > nextClock {
		nextClock = metadata.Version
	}
	if err := saveVersionManifest(stateDir, nextClock, nextFiles, s.maxBytes); err != nil {
		return Metadata{}, err
	}

	if err := os.Remove(filepath.Join(stateDir, versionJournalName)); err != nil {
		return Metadata{}, err
	}
	if err := syncDirectory(stateDir); err != nil {
		return Metadata{}, err
	}

	s.files = nextFiles
	s.versionClock = nextClock
	s.recoveryRequired = false
	return metadata, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
