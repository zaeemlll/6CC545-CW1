package app

import (
	"context"
	"errors"
	"log"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/peerfiles"
	"example.edu/6cc545/cw1/internal/rpcfiles"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
	"example.edu/6cc545/cw1/internal/syncstate"
)

const (
	reconciliationInterval = 5 * time.Second
	reconciliationTimeout  = 30 * time.Second
)

func runReconciliation(
	ctx context.Context,
	c config.Config,
	store *storage.Store,
	registry *state.Registry,
	table *membership.Table,
	validator security.TokenValidator,
) {
	if ctx == nil || store == nil || registry == nil || table == nil ||
		c.SecurityMode != "required" {
		log.Printf("reconciliation worker not started: secure configuration and dependencies are required")
		return
	}

	reconcile := func() {
		if err := reconcileOnce(ctx, c, store, registry, table, validator); err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Printf("reconciliation cycle failed: %v", err)
		}
	}

	reconcile()

	ticker := time.NewTicker(reconciliationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}

func reconcileOnce(
	ctx context.Context,
	c config.Config,
	store *storage.Store,
	registry *state.Registry,
	table *membership.Table,
	validator security.TokenValidator,
) error {
	if ctx == nil || store == nil || registry == nil || table == nil {
		return errors.New("reconciliation dependencies are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.SecurityMode != "required" || !store.VersioningEnabled() {
		return errors.New("reconciliation requires secure versioned storage")
	}
	if _, err := store.Snapshot(); err != nil {
		return errors.New("reconciliation storage is not ready: " + err.Error())
	}

	eligiblePeers := selectReconciliationPeers(
		registry.Snapshot(),
		table.ActiveSnapshot(time.Now()),
	)

	for _, peer := range eligiblePeers {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !reconciliationPeerIsActive(peer, registry, table, time.Now()) {
			continue
		}

		token, err := validator.IssueReaderToken(c.ID, 30*time.Second)
		if err != nil {
			log.Printf("reconciliation: issue reader token for peer %s: %v", peer.ID, err)
			continue
		}

		response, err := rpcfiles.FetchWithToken(
			ctx,
			peer.GRPCAddress,
			peer.ID,
			c.ID,
			c.Timeout,
			token,
			c,
		)
		if err != nil {
			log.Printf("reconciliation: fetch metadata from peer %s: %v", peer.ID, err)
			continue
		}
		if response == nil || response.GetNodeId() != peer.ID {
			log.Printf("reconciliation: reject metadata response from peer %s: node identity mismatch", peer.ID)
			continue
		}

		localMetadata, err := store.Snapshot()
		if err != nil {
			return err
		}

		local := make([]syncstate.Versioned, 0, len(localMetadata))
		for _, metadata := range localMetadata {
			local = append(local, syncstate.Versioned{Metadata: metadata})
		}

		remote := make([]syncstate.Versioned, 0, len(response.GetFiles()))
		validResponse := true
		for _, file := range response.GetFiles() {
			if file == nil {
				log.Printf("reconciliation: reject metadata from peer %s: nil file entry", peer.ID)
				validResponse = false
				break
			}

			remote = append(remote, syncstate.Versioned{
				Metadata: storage.Metadata{
					Name:         file.GetName(),
					Size:         file.GetSize(),
					SHA256:       file.GetSha256(),
					ModifiedUnix: file.GetModifiedUnix(),
					Version:      file.GetVersion(),
					WriterID:     file.GetWriterId(),
				},
			})
		}
		if !validResponse {
			continue
		}

		plan, err := syncstate.Plan(local, remote)
		if err != nil {
			log.Printf("reconciliation: invalid metadata from peer %s: %v", peer.ID, err)
			continue
		}

		for _, item := range plan {
			if err := ctx.Err(); err != nil {
				return err
			}

			_, err := peerfiles.DownloadExpected(
				ctx,
				peer.HTTPAddress,
				peer.ID,
				item.Name,
				store,
				reconciliationTimeout,
				c,
				item.Metadata,
			)
			if err != nil {
				log.Printf(
					"reconciliation: download %s from peer %s failed: %v",
					item.Name,
					peer.ID,
					err,
				)
			}
		}
	}

	return ctx.Err()
}

// selectReconciliationPeers selects registered peers with active matching
// membership announcements and an online registry status.
// reconciliationPeerIsActive rechecks registry and membership state before
// contacting a peer selected from an earlier snapshot.
func reconciliationPeerIsActive(
	peer model.Peer,
	registry *state.Registry,
	table *membership.Table,
	now time.Time,
) bool {
	current, err := registry.Get(peer.ID)
	if err != nil || current.Status != "online" ||
		current.HTTPAddress != peer.HTTPAddress ||
		current.GRPCAddress != peer.GRPCAddress {
		return false
	}

	for _, member := range table.ActiveSnapshot(now) {
		if member.ID == peer.ID && member.Address == current.HTTPAddress {
			return true
		}
	}

	return false
}

func selectReconciliationPeers(
	registered []model.Peer,
	active []membership.Peer,
) []model.Peer {
	activeByID := make(map[string]membership.Peer, len(active))
	for _, member := range active {
		activeByID[member.ID] = member
	}

	selected := make([]model.Peer, 0, len(registered))
	for _, peer := range registered {
		member, enrolled := activeByID[peer.ID]
		if !enrolled || peer.Status != "online" ||
			member.Address != peer.HTTPAddress {
			continue
		}
		selected = append(selected, peer)
	}

	return selected
}
