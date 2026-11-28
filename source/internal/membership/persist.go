package membership

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"example.edu/6cc545/cw1/internal/config"
)

const maxMembershipStateBytes = 32 << 10

type savedMembership struct {
	Version          int              `json:"version"`
	LocalIncarnation uint64           `json:"local_incarnation,omitempty"`
	Peers            []savedWatermark `json:"peers"`
}

type savedWatermark struct {
	ID          string `json:"id"`
	Incarnation uint64 `json:"incarnation"`
	Sequence    uint64 `json:"sequence"`
}

func prepareStateDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("membership state path is required")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(absolute)
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(dir, 0700); err != nil {
			return "", err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("membership state path must be a real directory")
	}

	return absolute, nil
}

func loadWatermarks(path string, localIncarnation *uint64) (map[string][2]uint64, error) {
	result := make(map[string][2]uint64)

	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("membership state must be a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, errors.New("membership state file changed during validation")
	}
	if openedInfo.Size() < 0 || openedInfo.Size() > maxMembershipStateBytes {
		return nil, errors.New("membership state file exceeds size limit")
	}

	data, err := io.ReadAll(io.LimitReader(file, maxMembershipStateBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMembershipStateBytes {
		return nil, errors.New("membership state file exceeds size limit")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var saved savedMembership
	if err := decoder.Decode(&saved); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("membership state contains trailing data")
	}
	if saved.Version != 1 || len(saved.Peers) > 128 {
		return nil, errors.New("invalid membership state format")
	}

	if localIncarnation != nil {
		*localIncarnation = saved.LocalIncarnation
	}

	for _, item := range saved.Peers {
		if !config.ValidID(item.ID) || item.Incarnation == 0 || item.Sequence == 0 {
			return nil, errors.New("invalid saved membership watermark")
		}
		if _, exists := result[item.ID]; exists {
			return nil, errors.New("duplicate saved membership identity")
		}
		result[item.ID] = [2]uint64{item.Incarnation, item.Sequence}
	}

	return result, nil
}

func saveWatermarks(path string, watermarks map[string][2]uint64, localIncarnation uint64) error {
	if len(watermarks) > 128 {
		return ErrCapacity
	}

	ids := make([]string, 0, len(watermarks))
	for id, watermark := range watermarks {
		if !config.ValidID(id) || watermark[0] == 0 || watermark[1] == 0 {
			return errors.New("invalid membership watermark")
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	saved := savedMembership{
		Version:          1,
		LocalIncarnation: localIncarnation,
		Peers:            make([]savedWatermark, 0, len(ids)),
	}
	for _, id := range ids {
		watermark := watermarks[id]
		saved.Peers = append(saved.Peers, savedWatermark{
			ID:          id,
			Incarnation: watermark[0],
			Sequence:    watermark[1],
		})
	}

	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if len(data) > maxMembershipStateBytes {
		return errors.New("membership state file exceeds size limit")
	}

	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("membership state directory is invalid")
	}

	temp, err := os.CreateTemp(dir, ".membership-*.tmp")
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
