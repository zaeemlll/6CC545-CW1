package syncstate

import (
	"encoding/hex"
	"errors"
	"strings"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/storage"
)

var ErrNotImplemented = errors.New("TODO W10: conflict ordering not implemented")
var ErrEquivocation = errors.New("same writer and version disagree")

type Versioned struct {
	storage.Metadata
}

func validateVersioned(v Versioned) error {
	if storage.ValidateName(v.Name) != nil {
		return errors.New("invalid file name")
	}
	if v.Size < 0 || v.Version == 0 || !config.ValidID(v.WriterID) {
		return errors.New("invalid versioned metadata")
	}
	if len(v.SHA256) != 64 {
		return errors.New("invalid SHA-256 hash")
	}
	if _, err := hex.DecodeString(v.SHA256); err != nil {
		return errors.New("invalid SHA-256 hash")
	}
	return nil
}

// Compare returns -1 if a loses, +1 if a wins, and 0 for identical content.
func Compare(a, b Versioned) (int, error) {
	if err := validateVersioned(a); err != nil {
		return 0, err
	}
	if err := validateVersioned(b); err != nil {
		return 0, err
	}

	if a.Name != b.Name {
		return 0, errors.New("cannot compare different file names")
	}

	if a.Version > b.Version {
		return 1, nil
	}
	if a.Version < b.Version {
		return -1, nil
	}

	if a.WriterID > b.WriterID {
		return 1, nil
	}
	if a.WriterID < b.WriterID {
		return -1, nil
	}

	if a.Size != b.Size || !strings.EqualFold(a.SHA256, b.SHA256) {
		return 0, ErrEquivocation
	}

	return 0, nil
}
