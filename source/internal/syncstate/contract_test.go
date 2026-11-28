//go:build acceptance

package syncstate

import (
	"errors"
	"example.edu/6cc545/cw1/internal/storage"
	"strings"
	"testing"
)

func v(n uint64, w, h string) Versioned {
	return Versioned{Metadata: storage.Metadata{Name: "a.txt", Size: 3, SHA256: strings.Repeat(h, 64), Version: n, WriterID: w}}
}
func TestDeterministicOrdering(t *testing.T) {
	cases := []struct {
		a, b Versioned
		want int
	}{{v(2, "node-a", "a"), v(1, "node-z", "b"), 1}, {v(1, "node-a", "a"), v(1, "node-b", "b"), -1}, {v(1, "node-a", "a"), v(1, "node-a", "a"), 0}}
	for _, c := range cases {
		got, e := Compare(c.a, c.b)
		if e != nil || got != c.want {
			t.Fatalf("order %d %v want %d", got, e, c.want)
		}
		reverse, e := Compare(c.b, c.a)
		if e != nil || reverse != -c.want {
			t.Fatal("order not antisymmetric")
		}
	}
}
func TestInconsistentVersionRejected(t *testing.T) {
	if _, e := Compare(v(1, "node-a", "a"), v(1, "node-a", "b")); !errors.Is(e, ErrEquivocation) {
		t.Fatalf("equivocation %v", e)
	}
}
