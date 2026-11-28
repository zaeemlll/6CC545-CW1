package membership

import (
	"testing"
	"time"
)

func TestActiveSnapshotFiltersExpiredPeersAndSorts(t *testing.T) {
	table, err := New(8, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1800000000, 0)
	announcements := []Announcement{
		{ID: "node-c", Address: "https://node-c:9444", Incarnation: 1, Sequence: 1},
		{ID: "node-a", Address: "https://node-a:9444", Incarnation: 1, Sequence: 1},
		{ID: "node-b", Address: "https://node-b:9444", Incarnation: 1, Sequence: 1},
	}
	seen := []time.Time{
		now.Add(-time.Second),
		now.Add(-2 * time.Second),
		now.Add(-5 * time.Second),
	}

	for i, announcement := range announcements {
		if err := table.Accept(announcement, announcement.ID, seen[i]); err != nil {
			t.Fatal(err)
		}
	}

	active := table.ActiveSnapshot(now)
	if len(active) != 2 {
		t.Fatalf("expected 2 active peers, got %d: %+v", len(active), active)
	}
	if active[0].ID != "node-a" || active[1].ID != "node-c" {
		t.Fatalf("active peers are not sorted or filtered correctly: %+v", active)
	}

	// ActiveSnapshot filters without taking ownership of expiry deletion.
	if got := len(table.Snapshot()); got != 3 {
		t.Fatalf("ActiveSnapshot unexpectedly deleted table entries: got %d", got)
	}
}

func TestActiveSnapshotIncludesPeerJustBeforeExpiry(t *testing.T) {
	table, err := New(8, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1800000000, 0)
	announcement := Announcement{
		ID: "node-b", Address: "https://node-b:9444",
		Incarnation: 1, Sequence: 1,
	}
	if err := table.Accept(announcement, announcement.ID, now.Add(-5*time.Second+time.Nanosecond)); err != nil {
		t.Fatal(err)
	}

	active := table.ActiveSnapshot(now)
	if len(active) != 1 || active[0].ID != "node-b" {
		t.Fatalf("peer just before expiry should remain active: %+v", active)
	}
}
