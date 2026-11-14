package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"example.edu/6cc545/cw1/internal/events"
	"example.edu/6cc545/cw1/internal/state"
)

func forwardEventToPeers(
	ctx context.Context,
	registry *state.Registry,
	nodeID string,
	event events.Event,
) error {
	if ctx == nil {
		return fmt.Errorf("forward context is nil")
	}
	if registry == nil {
		return fmt.Errorf("peer registry is nil")
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	for _, peer := range registry.Snapshot() {
		// Do not send an event back to this node.
		if peer.ID == nodeID {
			continue
		}

		// Check the peer details before using its address.
		if err := peer.Validate(); err != nil {
			return fmt.Errorf("invalid peer %s: %w", peer.ID, err)
		}

		url := "http://" + peer.HTTPAddress + "/v1/events"

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			url,
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf("create event request for peer %s: %w", peer.ID, err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("send event to peer %s: %w", peer.ID, err)
		}

		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted &&
			resp.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"peer %s rejected event with HTTP %d",
				peer.ID,
				resp.StatusCode,
			)
		}
	}

	return nil
}
