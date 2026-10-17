package state

import (
	"errors"
	"sort"
	"sync"

	"example.edu/6cc545/cw1/internal/model"
)

type Catalog struct {
	mu    sync.RWMutex
	files []model.FileMetadata
}

func NewCatalog() *Catalog {
	return &Catalog{files: []model.FileMetadata{}}
}

func (c *Catalog) Replace(input []model.FileMetadata) error {
	candidate := make([]model.FileMetadata, len(input))
	seen := make(map[string]bool, len(input))

	for i, file := range input {
		if err := file.Validate(); err != nil {
			return err
		}

		if seen[file.Name] {
			return errors.New("duplicate file name")
		}

		seen[file.Name] = true
		candidate[i] = file
	}

	sort.Slice(candidate, func(i, j int) bool {
		return candidate[i].Name < candidate[j].Name
	})

	c.mu.Lock()
	c.files = candidate
	c.mu.Unlock()

	return nil
}

func (c *Catalog) Snapshot() []model.FileMetadata {
	c.mu.RLock()
	files := make([]model.FileMetadata, len(c.files))
	copy(files, c.files)
	c.mu.RUnlock()

	return files
}
