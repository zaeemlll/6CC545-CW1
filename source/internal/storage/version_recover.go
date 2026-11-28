package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Store) recoverVersionedLocked(ctx context.Context) error {
	s.recoveryRequired = true

	if s.versionStateDir == "" || s.nodeID == "" {
		return errors.New("versioned storage is not initialised")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	stateInfo, err := os.Lstat(s.versionStateDir)
	if err != nil {
		return err
	}
	if stateInfo.Mode()&os.ModeSymlink != 0 || !stateInfo.IsDir() {
		return errors.New("version-state directory is invalid")
	}

	markerPath := filepath.Join(s.versionStateDir, versionMarkerName)
	manifestPath := filepath.Join(s.versionStateDir, versionManifestName)
	journalPath := filepath.Join(s.versionStateDir, versionJournalName)

	_, markerErr := os.Lstat(markerPath)
	markerExists := markerErr == nil
	if markerErr != nil && !errors.Is(markerErr, os.ErrNotExist) {
		return markerErr
	}

	_, manifestErr := os.Lstat(manifestPath)
	manifestExists := manifestErr == nil
	if manifestErr != nil && !errors.Is(manifestErr, os.ErrNotExist) {
		return manifestErr
	}

	_, journalErr := os.Lstat(journalPath)
	if journalErr == nil {
		if !markerExists || !manifestExists {
			return errors.New("unfinished versioned commit has incomplete state")
		}
		if err := loadVersionMarker(s.versionStateDir, s.nodeID); err != nil {
			return err
		}

		journal, err := loadVersionJournal(s.versionStateDir, s.maxBytes)
		if err != nil {
			return err
		}

		stagePath := filepath.Join(s.root, journal.TempName)
		destination := filepath.Join(s.root, journal.Metadata.Name)

		stageValid, err := verifyVersionJournalFile(ctx, stagePath, journal.Metadata)
		if err != nil {
			return err
		}

		if stageValid {
			if err := os.Rename(stagePath, destination); err != nil {
				return err
			}
			if err := syncDirectory(s.root); err != nil {
				return err
			}
		} else {
			if _, err := os.Lstat(stagePath); err == nil {
				return ErrIntegrity
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			destinationValid, err := verifyVersionJournalFile(ctx, destination, journal.Metadata)
			if err != nil {
				return err
			}
			if !destinationValid {
				return ErrIntegrity
			}
		}

		clock, files, err := loadVersionManifest(s.versionStateDir, s.maxBytes)
		if err != nil {
			return err
		}
		nextFiles := make(map[string]Metadata, len(files)+1)
		for name, metadata := range files {
			nextFiles[name] = metadata
		}
		nextFiles[journal.Metadata.Name] = journal.Metadata
		if journal.Metadata.Version > clock {
			clock = journal.Metadata.Version
		}
		if err := saveVersionManifest(s.versionStateDir, clock, nextFiles, s.maxBytes); err != nil {
			return err
		}
		if err := os.Remove(journalPath); err != nil {
			return err
		}
		if err := syncDirectory(s.versionStateDir); err != nil {
			return err
		}
	} else if !errors.Is(journalErr, os.ErrNotExist) {
		return journalErr
	}
	if markerExists {
		if err := loadVersionMarker(s.versionStateDir, s.nodeID); err != nil {
			return err
		}
		if !manifestExists {
			return errors.New("version manifest is missing after initialisation")
		}
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}

	diskFiles := make(map[string]Metadata)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		name := entry.Name()
		path := filepath.Join(s.root, name)

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("storage contains a symbolic link")
		}

		if name == versionStateDirName {
			if !info.IsDir() {
				return errors.New("CW2 state entry is not a directory")
			}
			continue
		}

		if strings.HasPrefix(name, ".w07-tmp-") ||
			strings.HasPrefix(name, versionStagePrefix) {
			if !info.Mode().IsRegular() {
				return errors.New("invalid temporary storage entry")
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}

		if err := ValidateName(name); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("storage contains a non-regular file")
		}
		if info.Size() < 0 || info.Size() > s.maxBytes {
			return ErrTooLarge
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		openedInfo, err := file.Stat()
		if err != nil {
			file.Close()
			return err
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			file.Close()
			return ErrIntegrity
		}

		hasher := sha256.New()
		written, readErr := io.Copy(hasher, contextReader{ctx: ctx, r: file})
		closeErr := file.Close()

		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		afterInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if afterInfo.Mode()&os.ModeSymlink != 0 ||
			!afterInfo.Mode().IsRegular() ||
			!os.SameFile(info, afterInfo) ||
			afterInfo.Size() != info.Size() ||
			!afterInfo.ModTime().Equal(info.ModTime()) ||
			written != info.Size() {
			return ErrIntegrity
		}

		diskFiles[name] = Metadata{
			Name:         name,
			Size:         written,
			SHA256:       hex.EncodeToString(hasher.Sum(nil)),
			ModifiedUnix: afterInfo.ModTime().Unix(),
		}
	}

	if len(diskFiles) > maxVersionedFiles {
		return ErrInvalidVersionMetadata
	}

	var clock uint64
	var recovered map[string]Metadata

	if manifestExists {
		clock, recovered, err = loadVersionManifest(s.versionStateDir, s.maxBytes)
		if err != nil {
			return err
		}
		if len(recovered) != len(diskFiles) {
			return errors.New("version manifest does not match stored files")
		}

		for name, disk := range diskFiles {
			metadata, ok := recovered[name]
			if !ok || metadata.Size != disk.Size || metadata.SHA256 != disk.SHA256 {
				return errors.New("stored file does not match version manifest")
			}
		}
	} else {
		if markerExists {
			return errors.New("version manifest is missing after initialisation")
		}

		recovered = make(map[string]Metadata, len(diskFiles))
		for _, name := range sortedDiskNames(diskFiles) {
			clock++
			metadata := diskFiles[name]
			metadata.Version = clock
			metadata.WriterID = s.nodeID
			recovered[name] = metadata
		}

		if err := saveVersionManifest(s.versionStateDir, clock, recovered, s.maxBytes); err != nil {
			return err
		}
	}

	if !markerExists {
		if err := saveVersionMarker(s.versionStateDir, s.nodeID); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	s.files = recovered
	s.versionClock = clock
	s.recoveryRequired = false
	return nil
}

func sortedDiskNames(files map[string]Metadata) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// verifyVersionJournalFile checks a file against an interrupted commit record.
func verifyVersionJournalFile(ctx context.Context, path string, expected Metadata) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, ErrIntegrity
	}
	if info.Size() != expected.Size {
		return false, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return false, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return false, ErrIntegrity
	}

	hasher := sha256.New()
	written, readErr := io.Copy(hasher, contextReader{ctx: ctx, r: file})
	closeErr := file.Close()
	if readErr != nil {
		return false, readErr
	}
	if closeErr != nil {
		return false, closeErr
	}

	afterInfo, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if afterInfo.Mode()&os.ModeSymlink != 0 ||
		!afterInfo.Mode().IsRegular() ||
		!os.SameFile(info, afterInfo) ||
		afterInfo.Size() != info.Size() ||
		!afterInfo.ModTime().Equal(info.ModTime()) ||
		written != expected.Size {
		return false, ErrIntegrity
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}
	return hex.EncodeToString(hasher.Sum(nil)) == expected.SHA256, nil
}
