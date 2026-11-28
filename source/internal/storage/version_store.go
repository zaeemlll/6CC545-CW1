package storage

import (
	"errors"

	"example.edu/6cc545/cw1/internal/config"
)

// EnableVersioning activates persistent file versions for this node.
// It must be called before Recover and before the store serves requests.
func (s *Store) EnableVersioning(nodeID string) error {
	if s == nil || s.maxBytes < 1 || s.root == "" {
		return errors.New("invalid storage configuration")
	}
	if !config.ValidID(nodeID) {
		return errors.New("valid node ID is required for versioned storage")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.versioned {
		if s.nodeID == nodeID {
			return nil
		}
		return errors.New("versioning is already enabled for another node")
	}

	stateDir, err := prepareVersionStateDir(s.root)
	if err != nil {
		return err
	}

	s.nodeID = nodeID
	s.versionStateDir = stateDir
	s.recoveryRequired = true
	s.versioned = true

	return nil
}

// VersioningEnabled reports whether durable versioning is enabled for this store.
func (s *Store) VersioningEnabled() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.versioned
}
