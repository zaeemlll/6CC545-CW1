package storage

import (
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"example.edu/6cc545/cw1/internal/config"
)

const (
	versionStateFormat   = 1
	maxVersionedFiles    = 4096
	maxVersionStateBytes = 2 << 20
)

var (
	ErrInvalidVersionMetadata = errors.New("invalid file version metadata")
	ErrVersionConflict        = errors.New("incoming file version loses conflict comparison")
	ErrVersionEquivocation    = errors.New("same writer and version disagree")
)

// versionManifest stores the synchronisation metadata for committed files.
type versionManifest struct {
	Format int        `json:"format"`
	Clock  uint64     `json:"clock"`
	Files  []Metadata `json:"files"`
}

const versionStagePrefix = ".cw2-stage-"

type versionJournal struct {
	Format   int      `json:"format"`
	TempName string   `json:"temp_name"`
	Metadata Metadata `json:"metadata"`
}

// validateVersionJournal ensures recovery can only use a safe staged filename.
func validateVersionJournal(journal versionJournal, maxBytes int64) error {
	if journal.Format != versionStateFormat ||
		!strings.HasPrefix(journal.TempName, versionStagePrefix) ||
		len(journal.TempName) == len(versionStagePrefix) ||
		strings.ContainsAny(journal.TempName, `/\\`) {
		return ErrInvalidVersionMetadata
	}

	suffix := strings.TrimPrefix(journal.TempName, versionStagePrefix)
	for _, char := range suffix {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-') {
			return ErrInvalidVersionMetadata
		}
	}

	return validateVersionedMetadata(journal.Metadata, maxBytes)
}

func validateVersionedMetadata(metadata Metadata, maxBytes int64) error {
	if ValidateName(metadata.Name) != nil ||
		metadata.Size < 0 ||
		metadata.Size > maxBytes ||
		len(metadata.SHA256) != 64 ||
		metadata.Version == 0 ||
		!config.ValidID(metadata.WriterID) {
		return ErrInvalidVersionMetadata
	}

	if metadata.SHA256 != strings.ToLower(metadata.SHA256) {
		return ErrInvalidVersionMetadata
	}

	if _, err := hex.DecodeString(metadata.SHA256); err != nil {
		return ErrInvalidVersionMetadata
	}

	return nil
}

func validateVersionManifest(manifest versionManifest, maxBytes int64) (map[string]Metadata, error) {
	if manifest.Format != versionStateFormat || len(manifest.Files) > maxVersionedFiles {
		return nil, ErrInvalidVersionMetadata
	}

	files := make(map[string]Metadata, len(manifest.Files))
	for _, metadata := range manifest.Files {
		if err := validateVersionedMetadata(metadata, maxBytes); err != nil {
			return nil, err
		}
		if metadata.Version > manifest.Clock {
			return nil, ErrInvalidVersionMetadata
		}
		if _, exists := files[metadata.Name]; exists {
			return nil, ErrInvalidVersionMetadata
		}
		files[metadata.Name] = metadata
	}

	return files, nil
}

func sortedVersionMetadata(files map[string]Metadata) []Metadata {
	result := make([]Metadata, 0, len(files))
	for _, metadata := range files {
		result = append(result, metadata)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// versionMarker records that persistent file versioning has been initialised.
type versionMarker struct {
	Format int    `json:"format"`
	NodeID string `json:"node_id"`
}
