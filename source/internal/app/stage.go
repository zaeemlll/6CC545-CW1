package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
)

func CheckStage(c config.Config) error {
	if c.TCP != "" || c.UDP != "" || c.GRPC != "" {
		return errors.New("Week 3: TCP, UDP and gRPC are not enabled yet")
	}
	return nil
}

func loadSeed(c *state.Catalog, path string) error {
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open seed file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	if len(data) > 65536 {
		return errors.New("seed file exceeds 65536 bytes")
	}

	var files []model.FileMetadata
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("decode seed file: %w", err)
	}

	if err := c.Replace(files); err != nil {
		return fmt.Errorf("validate seed file: %w", err)
	}

	return nil
}

func Wire(ctx context.Context, c config.Config, mux *http.ServeMux) (func(), error) {
	_ = ctx

	status := state.NewStatus()
	registry := state.NewRegistry()
	catalog := state.NewCatalog()

	if err := loadSeed(catalog, c.Seed); err != nil {
		return nil, err
	}

	mux.HandleFunc("/v1/status", httpapi.StatusResource(status))
	mux.HandleFunc("/v1/peers", httpapi.Peers(registry))
	mux.HandleFunc("/v1/files", httpapi.Files(catalog))

	return func() {}, nil
}
