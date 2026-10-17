package model

import (
	"encoding/hex"
	"errors"
	"example.edu/6cc545/cw1/internal/config"
	"strings"
)

type Peer struct {
	ID          string `json:"id"`
	HTTPAddress string `json:"http_address"`
	GRPCAddress string `json:"grpc_address"`
	Status      string `json:"status"`
}
type FileMetadata struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	ModifiedUnix int64  `json:"modified_unix"`
}

func (p Peer) Validate() error {
	if !config.ValidID(p.ID) {
		return errors.New("invalid peer ID")
	}
	if !config.ValidAddress(p.HTTPAddress) {
		return errors.New("invalid HTTP address")
	}
	if !config.ValidAddress(p.GRPCAddress) {
		return errors.New("invalid gRPC address")
	}

	switch p.Status {
	case "unknown", "online", "offline":
		return nil
	default:
		return errors.New("invalid peer status")
	}
}

func (f FileMetadata) Validate() error {
	if f.Name == "" || len(f.Name) > 128 ||
		f.Name == "." || f.Name == ".." ||
		strings.ContainsAny(f.Name, `/\:`) ||
		strings.ContainsRune(f.Name, 0) {
		return errors.New("invalid file name")
	}

	if f.Size < 0 {
		return errors.New("invalid file size")
	}
	if f.ModifiedUnix < 0 {
		return errors.New("invalid modification time")
	}
	if len(f.SHA256) != 64 {
		return errors.New("invalid SHA-256 hash")
	}
	if _, err := hex.DecodeString(f.SHA256); err != nil {
		return errors.New("invalid SHA-256 hash")
	}

	return nil
}

var _ = hex.DecodeString
var _ = strings.ContainsAny
var _ = config.ValidID
