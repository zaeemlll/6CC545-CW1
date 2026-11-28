package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"example.edu/6cc545/cw1/internal/config"
)

const (
	versionStateDirName = ".cw2-state"
	versionManifestName = "files.json"
	versionJournalName  = "file-commit.json"
)

// prepareVersionStateDir creates and validates the private version-state directory.
func prepareVersionStateDir(root string) (string, error) {
	dir := filepath.Join(root, versionStateDirName)

	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}

	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("version-state path must be a directory, not a symlink")
	}

	return filepath.Abs(dir)
}

// readVersionJSON reads one bounded JSON value from a regular state file.
func readVersionJSON(path string, destination any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("version-state file must be a regular file")
	}
	if info.Size() < 0 || info.Size() > maxVersionStateBytes {
		return errors.New("version-state file exceeds size limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, openedInfo) {
		return errors.New("version-state file changed while opening")
	}

	data, err := io.ReadAll(io.LimitReader(file, maxVersionStateBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxVersionStateBytes {
		return errors.New("version-state file exceeds size limit")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("version-state file contains extra JSON data")
		}
		return err
	}

	return nil
}

// writeVersionJSON atomically replaces a bounded JSON state file.
func writeVersionJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maxVersionStateBytes {
		return errors.New("version-state file exceeds size limit")
	}

	dir := filepath.Dir(path)
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return errors.New("version-state directory is invalid")
	}

	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("version-state destination must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	temp, err := os.CreateTemp(dir, ".cw2-state-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempName, path); err != nil {
		return err
	}

	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()

	return directory.Sync()
}

// saveVersionManifest validates and atomically saves the committed file catalogue.
func saveVersionManifest(dir string, clock uint64, files map[string]Metadata, maxBytes int64) error {
	if len(files) > maxVersionedFiles {
		return ErrInvalidVersionMetadata
	}

	manifest := versionManifest{
		Format: versionStateFormat,
		Clock:  clock,
		Files:  sortedVersionMetadata(files),
	}

	if _, err := validateVersionManifest(manifest, maxBytes); err != nil {
		return err
	}

	return writeVersionJSON(filepath.Join(dir, versionManifestName), manifest)
}

// loadVersionManifest reads and validates the committed file catalogue.
func loadVersionManifest(dir string, maxBytes int64) (uint64, map[string]Metadata, error) {
	var manifest versionManifest

	if err := readVersionJSON(filepath.Join(dir, versionManifestName), &manifest); err != nil {
		return 0, nil, err
	}

	files, err := validateVersionManifest(manifest, maxBytes)
	if err != nil {
		return 0, nil, err
	}

	return manifest.Clock, files, nil
}

// saveVersionJournal validates and atomically saves an interrupted commit record.
func saveVersionJournal(dir string, journal versionJournal, maxBytes int64) error {
	if err := validateVersionJournal(journal, maxBytes); err != nil {
		return err
	}

	return writeVersionJSON(filepath.Join(dir, versionJournalName), journal)
}

// loadVersionJournal reads and validates an interrupted commit record.
func loadVersionJournal(dir string, maxBytes int64) (versionJournal, error) {
	var journal versionJournal

	if err := readVersionJSON(filepath.Join(dir, versionJournalName), &journal); err != nil {
		return versionJournal{}, err
	}
	if err := validateVersionJournal(journal, maxBytes); err != nil {
		return versionJournal{}, err
	}

	return journal, nil
}

const versionMarkerName = "versioning.json"

// saveVersionMarker records the node identity for an initialised version store.
func saveVersionMarker(dir, nodeID string) error {
	if !config.ValidID(nodeID) {
		return errors.New("invalid version marker node ID")
	}

	return writeVersionJSON(filepath.Join(dir, versionMarkerName), versionMarker{
		Format: versionStateFormat,
		NodeID: nodeID,
	})
}

// loadVersionMarker verifies the format and node identity of an existing marker.
func loadVersionMarker(dir, nodeID string) error {
	var marker versionMarker

	if err := readVersionJSON(filepath.Join(dir, versionMarkerName), &marker); err != nil {
		return err
	}
	if marker.Format != versionStateFormat || !config.ValidID(marker.NodeID) {
		return errors.New("invalid version initialisation marker")
	}
	if marker.NodeID != nodeID {
		return errors.New("version marker belongs to another node")
	}

	return nil
}
