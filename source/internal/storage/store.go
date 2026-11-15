// Package storage is an assessed single-process local file store wireframe.
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotImplemented = errors.New("operation not implemented yet")
var ErrInvalidName = errors.New("invalid portable file name")
var ErrIntegrity = errors.New("size or SHA-256 mismatch")
var ErrTooLarge = errors.New("file exceeds configured limit")

type Metadata struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	ModifiedUnix int64  `json:"modified_unix"`
}
type Store struct {
	mu       sync.RWMutex
	root     string
	maxBytes int64
	files    map[string]Metadata
}

// New is supplied directory allocation, not a persistence/recovery implementation.
// The directory must be owned exclusively by this process and not attacker-writable.
// Task 8.6 adds recovery of committed files after a restart.
func New(root string, maxBytes int64) (*Store, error) {
	if root == "" || maxBytes < 1 {
		return nil, errors.New("root and positive maxBytes required")
	}
	abs, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(abs, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("root must be a directory, not a symlink")
	}
	return &Store{root: abs, maxBytes: maxBytes, files: make(map[string]Metadata)}, nil
}
func ValidateName(name string) error {
	if len(name) < 1 || len(name) > 128 {
		return ErrInvalidName
	}

	isAlphaNumeric := func(c byte) bool {
		return (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9')
	}

	if !isAlphaNumeric(name[0]) ||
		strings.Contains(name, "..") ||
		strings.HasSuffix(name, ".") {
		return ErrInvalidName
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if !isAlphaNumeric(c) && c != '.' && c != '_' && c != '-' {
			return ErrInvalidName
		}
	}

	base := name
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}

	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true,
		"COM4": true, "COM5": true, "COM6": true,
		"COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true,
		"LPT4": true, "LPT5": true, "LPT6": true,
		"LPT7": true, "LPT8": true, "LPT9": true,
	}

	if reserved[strings.ToUpper(base)] {
		return ErrInvalidName
	}

	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (s *Store) Put(ctx context.Context, name string, r io.Reader, size int64, sha string) (Metadata, error) {
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
	if size < 0 {
		return Metadata{}, ErrIntegrity
	}
	if size > s.maxBytes {
		return Metadata{}, ErrTooLarge
	}
	if len(sha) != 64 {
		return Metadata{}, ErrIntegrity
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return Metadata{}, ErrIntegrity
	}

	temp, err := os.CreateTemp(s.root, ".w07-tmp-*")
	if err != nil {
		return Metadata{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	hasher := sha256.New()
	source := contextReader{ctx: ctx, r: r}

	written, err := io.Copy(
		io.MultiWriter(temp, hasher),
		io.LimitReader(source, size),
	)
	if err != nil {
		temp.Close()
		return Metadata{}, err
	}
	if written != size {
		temp.Close()
		return Metadata{}, ErrIntegrity
	}

	// Read one additional byte to reject content beyond the declared size.
	var extra [1]byte
	n, extraErr := io.ReadFull(source, extra[:])
	if n != 0 {
		temp.Close()
		return Metadata{}, ErrIntegrity
	}
	if extraErr != io.EOF {
		temp.Close()
		if extraErr != nil {
			return Metadata{}, extraErr
		}
		return Metadata{}, ErrIntegrity
	}

	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualSHA, sha) {
		temp.Close()
		return Metadata{}, ErrIntegrity
	}

	if err := ctx.Err(); err != nil {
		temp.Close()
		return Metadata{}, err
	}

	if err := temp.Close(); err != nil {
		return Metadata{}, err
	}

	metadata := Metadata{
		Name:         name,
		Size:         written,
		SHA256:       actualSHA,
		ModifiedUnix: time.Now().Unix(),
	}

	// The lock protects the final replacement and metadata update together.
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}

	destination := filepath.Join(s.root, name)
	if err := os.Rename(tempName, destination); err != nil {
		return Metadata{}, err
	}

	if s.files == nil {
		s.files = make(map[string]Metadata)
	}
	s.files[name] = metadata

	return metadata, nil
}
func (s *Store) Open(name string) (io.ReadCloser, Metadata, error) {
	if s == nil {
		return nil, Metadata{}, errors.New("invalid storage configuration")
	}
	if err := ValidateName(name); err != nil {
		return nil, Metadata{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, ok := s.files[name]
	if !ok {
		return nil, Metadata{}, os.ErrNotExist
	}

	content, err := os.ReadFile(filepath.Join(s.root, name))
	if err != nil {
		return nil, Metadata{}, err
	}

	if int64(len(content)) != metadata.Size {
		return nil, Metadata{}, ErrIntegrity
	}

	actualSHA := sha256.Sum256(content)
	if !strings.EqualFold(hex.EncodeToString(actualSHA[:]), metadata.SHA256) {
		return nil, Metadata{}, ErrIntegrity
	}

	return io.NopCloser(bytes.NewReader(content)), metadata, nil
}
func (s *Store) Snapshot() ([]Metadata, error) {
	if s == nil {
		return nil, errors.New("invalid storage configuration")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Metadata, 0, len(s.files))
	for _, metadata := range s.files {
		result = append(result, metadata)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}
func (s *Store) Recover(ctx context.Context) error {
	// TODO 8.6: rebuild committed metadata on restart, ignore/remove abandoned
	// temp files, revalidate names/sizes/hashes; do not follow symlinks. A clear
	// private-temp naming convention is needed. Never infer validity from name.
	return ErrNotImplemented
}
