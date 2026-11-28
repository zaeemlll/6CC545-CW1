//go:build acceptance

package membership

import (
	"errors"
	"testing"
	"time"
)

func TestMembershipReplayExpiryRestart(t *testing.T) {
	x, _ := New(8, 3*time.Second)
	now := time.Unix(1800000000, 0)
	a := Announcement{ID: "node-b", Address: "https://node-b:9444", Incarnation: 1, Sequence: 1}
	if e := x.Accept(a, "node-b", now); e != nil {
		t.Fatal(e)
	}
	if e := x.Accept(a, "node-b", now.Add(2*time.Second)); !errors.Is(e, ErrReplay) {
		t.Fatalf("replay: %v", e)
	}
	expired := x.Expire(now.Add(3 * time.Second))
	if len(expired) != 1 || expired[0] != "node-b" || len(x.Snapshot()) != 0 {
		t.Fatalf("replay refreshed liveness: %v", expired)
	}
	if e := x.Accept(a, "node-b", now.Add(4*time.Second)); !errors.Is(e, ErrReplay) {
		t.Fatalf("expired replay: %v", e)
	}
	a.Incarnation = 2
	a.Sequence = 1
	if e := x.Accept(a, "node-b", now.Add(4*time.Second)); e != nil {
		t.Fatalf("new incarnation: %v", e)
	}
	saved := x.Watermarks()
	y, _ := New(8, 3*time.Second)
	if e := y.Restore(saved); e != nil {
		t.Fatal(e)
	}
	if len(y.Snapshot()) != 0 {
		t.Fatal("restore invented liveness")
	}
	if e := y.Accept(a, "node-b", now.Add(5*time.Second)); !errors.Is(e, ErrReplay) {
		t.Fatalf("restart lost watermark: %v", e)
	}
}
func TestIdentityAndCapacity(t *testing.T) {
	x, _ := New(1, time.Second)
	now := time.Now()
	a := Announcement{ID: "node-b", Address: "https://node-b:9444", Incarnation: 1, Sequence: 1}
	if e := x.Accept(a, "node-c", now); !errors.Is(e, ErrIdentity) {
		t.Fatalf("identity mismatch: %v", e)
	}
	if e := x.Accept(a, "node-b", now); e != nil {
		t.Fatal(e)
	}
	a.ID = "node-c"
	a.Address = "https://node-c:9444"
	if e := x.Accept(a, "node-c", now); !errors.Is(e, ErrCapacity) {
		t.Fatalf("capacity: %v", e)
	}
}
