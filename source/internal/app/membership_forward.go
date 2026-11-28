package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
)

func forwardMembershipToPeers(
	ctx context.Context,
	registry *state.Registry,
	nodeID string,
	announcement membership.Announcement,
	c config.Config,
) error {
	if ctx == nil {
		return errors.New("membership forwarding context is nil")
	}
	if registry == nil {
		return errors.New("peer registry is nil")
	}
	if c.SecurityMode != "required" {
		return errors.New("membership forwarding requires mutual TLS")
	}
	if !config.ValidID(nodeID) ||
		announcement.ID != nodeID ||
		announcement.Incarnation == 0 ||
		announcement.Sequence == 0 ||
		!config.ValidPeerHTTPSURL(announcement.Address) {
		return errors.New("invalid local membership announcement")
	}

	body, err := json.Marshal(announcement)
	if err != nil {
		return fmt.Errorf("encode membership announcement: %w", err)
	}

	var failures []error

	for _, peer := range registry.Snapshot() {
		if peer.ID == nodeID {
			continue
		}

		if err := peer.Validate(); err != nil {
			failures = append(failures, fmt.Errorf("invalid peer %s: %w", peer.ID, err))
			continue
		}
		if !config.ValidPeerHTTPSURL(peer.HTTPAddress) {
			failures = append(failures, fmt.Errorf("peer %s does not have a valid HTTPS address", peer.ID))
			continue
		}

		tlsConfig, err := security.ClientTLS(
			c.TLSCertFile,
			c.TLSKeyFile,
			c.CAFile,
			peer.ID,
		)
		if err != nil {
			failures = append(failures, fmt.Errorf("configure TLS for peer %s: %w", peer.ID, err))
			continue
		}

		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client := &http.Client{
			Timeout:   c.Timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		requestCtx, cancel := context.WithTimeout(ctx, c.Timeout)
		targetURL := strings.TrimRight(peer.HTTPAddress, "/") + "/v1/peer/membership"

		req, err := http.NewRequestWithContext(
			requestCtx,
			http.MethodPost,
			targetURL,
			bytes.NewReader(body),
		)
		if err != nil {
			cancel()
			transport.CloseIdleConnections()
			failures = append(failures, fmt.Errorf("create membership request for peer %s: %w", peer.ID, err))
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			transport.CloseIdleConnections()
			failures = append(failures, fmt.Errorf("send membership announcement to peer %s: %w", peer.ID, err))
			continue
		}

		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		cancel()
		transport.CloseIdleConnections()

		if resp.StatusCode != http.StatusAccepted {
			failures = append(failures, fmt.Errorf(
				"peer %s rejected membership announcement with HTTP %d",
				peer.ID,
				resp.StatusCode,
			))
		}
	}

	return errors.Join(failures...)
}

// runMembershipAnnouncements periodically announces this node to its enrolled peers.
func runMembershipAnnouncements(
	ctx context.Context,
	registry *state.Registry,
	nodeID string,
	address string,
	table *membership.Table,
	c config.Config,
) {
	if table == nil {
		return
	}

	incarnation := table.Incarnation()
	if incarnation == 0 {
		log.Printf("membership announcements stopped: local incarnation is zero")
		return
	}

	var sequence uint64

	send := func() bool {
		if sequence == ^uint64(0) {
			log.Printf("membership announcements stopped: sequence exhausted")
			return false
		}
		sequence++

		announcement := membership.Announcement{
			ID:          nodeID,
			Address:     address,
			Incarnation: incarnation,
			Sequence:    sequence,
		}

		if err := forwardMembershipToPeers(ctx, registry, nodeID, announcement, c); err != nil {
			log.Printf("membership announcement failed: %v", err)
		}
		return true
	}

	if !send() {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}
