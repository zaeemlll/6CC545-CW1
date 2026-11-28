package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/membership"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
)

func newReconcileTestState(t *testing.T, versioned, recoverStore bool) (*storage.Store, *state.Registry, *membership.Table) {
	t.Helper()

	store, err := storage.New(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if versioned {
		if err := store.EnableVersioning("node-a"); err != nil {
			t.Fatal(err)
		}
		if recoverStore {
			if err := store.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
	}

	table, err := membership.New(8, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	return store, state.NewRegistry(), table
}

func TestReconcileOnceRejectsInsecureConfiguration(t *testing.T) {
	store, registry, table := newReconcileTestState(t, false, false)

	err := reconcileOnce(
		context.Background(), config.Config{}, store, registry, table,
		security.TokenValidator{},
	)
	if err == nil || !strings.Contains(err.Error(), "secure versioned storage") {
		t.Fatalf("expected secure-storage error, got %v", err)
	}
}

func TestReconcileOnceRejectsUnversionedStorage(t *testing.T) {
	store, registry, table := newReconcileTestState(t, false, false)

	err := reconcileOnce(
		context.Background(),
		config.Config{SecurityMode: "required"},
		store, registry, table, security.TokenValidator{},
	)
	if err == nil || !strings.Contains(err.Error(), "secure versioned storage") {
		t.Fatalf("expected versioned-storage error, got %v", err)
	}
}

func TestReconcileOnceRejectsStorageBeforeRecovery(t *testing.T) {
	store, registry, table := newReconcileTestState(t, true, false)

	err := reconcileOnce(
		context.Background(),
		config.Config{SecurityMode: "required"},
		store, registry, table, security.TokenValidator{},
	)
	if err == nil || !strings.Contains(err.Error(), "storage is not ready") {
		t.Fatalf("expected storage-readiness error, got %v", err)
	}
}

func TestReconcileOnceAcceptsRecoveredStorageWithoutPeers(t *testing.T) {
	store, registry, table := newReconcileTestState(t, true, true)

	err := reconcileOnce(
		context.Background(),
		config.Config{SecurityMode: "required"},
		store, registry, table, security.TokenValidator{},
	)
	if err != nil {
		t.Fatalf("expected empty reconciliation cycle to succeed, got %v", err)
	}
}

func TestReconcileOnceHonoursCancelledContext(t *testing.T) {
	store, registry, table := newReconcileTestState(t, true, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := reconcileOnce(
		ctx,
		config.Config{SecurityMode: "required"},
		store, registry, table, security.TokenValidator{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestSelectReconciliationPeersRequiresActiveMatchingOnlineMembership(t *testing.T) {
	registered := []model.Peer{
		{ID: "node-a", HTTPAddress: "https://node-a:9444", GRPCAddress: "node-a:9091", Status: "online"},
		{ID: "node-b", HTTPAddress: "https://node-b:9444", GRPCAddress: "node-b:9091", Status: "unknown"},
		{ID: "node-c", HTTPAddress: "https://node-c:9444", GRPCAddress: "node-c:9091", Status: "offline"},
		{ID: "node-d", HTTPAddress: "https://node-d:9444", GRPCAddress: "node-d:9091", Status: "online"},
		{ID: "node-e", HTTPAddress: "https://node-e:9444", GRPCAddress: "node-e:9091", Status: "online"},
	}
	active := []membership.Peer{
		{Announcement: membership.Announcement{ID: "node-a", Address: "https://node-a:9444"}},
		{Announcement: membership.Announcement{ID: "node-b", Address: "https://node-b:9444"}},
		{Announcement: membership.Announcement{ID: "node-c", Address: "https://node-c:9444"}},
		{Announcement: membership.Announcement{ID: "node-d", Address: "https://wrong-node:9444"}},
	}

	selected := selectReconciliationPeers(registered, active)
	if len(selected) != 1 || selected[0].ID != "node-a" ||
		selected[0].GRPCAddress != "node-a:9091" {
		t.Fatalf("unexpected eligible peers: %+v", selected)
	}
}

func TestSelectReconciliationPeersReturnsEmptyWithoutMembership(t *testing.T) {
	registered := []model.Peer{
		{ID: "node-b", HTTPAddress: "https://node-b:9444", GRPCAddress: "node-b:9091", Status: "online"},
	}
	if selected := selectReconciliationPeers(registered, nil); len(selected) != 0 {
		t.Fatalf("expected no eligible peers, got %+v", selected)
	}
}

func TestReconciliationPeerIsActive(t *testing.T) {
	now := time.Now()
	registered := model.Peer{
		ID: "node-b", HTTPAddress: "https://node-b:9444",
		GRPCAddress: "node-b:9091", Status: "online",
	}

	newState := func(ttl time.Duration) (*state.Registry, *membership.Table) {
		t.Helper()
		registry := state.NewRegistry()
		if err := registry.Add(registered); err != nil {
			t.Fatal(err)
		}
		table, err := membership.New(8, ttl)
		if err != nil {
			t.Fatal(err)
		}
		announcement := membership.Announcement{
			ID: "node-b", Address: registered.HTTPAddress,
			Incarnation: 1, Sequence: 1,
		}
		if err := table.Accept(announcement, "node-b", now); err != nil {
			t.Fatal(err)
		}
		return registry, table
	}

	t.Run("active matching peer", func(t *testing.T) {
		registry, table := newState(time.Minute)
		if !reconciliationPeerIsActive(registered, registry, table, now) {
			t.Fatal("expected active matching peer to be accepted")
		}
	})

	t.Run("expired membership", func(t *testing.T) {
		registry, table := newState(time.Second)
		if reconciliationPeerIsActive(registered, registry, table, now.Add(time.Second)) {
			t.Fatal("expected expired membership to be rejected")
		}
	})

	t.Run("offline registry status", func(t *testing.T) {
		registry, table := newState(time.Minute)
		if err := registry.SetStatus("node-b", "offline"); err != nil {
			t.Fatal(err)
		}
		if reconciliationPeerIsActive(registered, registry, table, now) {
			t.Fatal("expected offline peer to be rejected")
		}
	})

	t.Run("missing registry peer", func(t *testing.T) {
		registry := state.NewRegistry()
		table, err := membership.New(8, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if reconciliationPeerIsActive(registered, registry, table, now) {
			t.Fatal("expected unregistered peer to be rejected")
		}
	})

	t.Run("changed registered address", func(t *testing.T) {
		registry, table := newState(time.Minute)
		changed := registered
		changed.HTTPAddress = "https://other-node:9444"
		if reconciliationPeerIsActive(changed, registry, table, now) {
			t.Fatal("expected changed HTTP address to be rejected")
		}
		changed = registered
		changed.GRPCAddress = "other-node:9091"
		if reconciliationPeerIsActive(changed, registry, table, now) {
			t.Fatal("expected changed gRPC address to be rejected")
		}
	})
}
