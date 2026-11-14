package events

import "testing"

// This checks supplied allocation only; it is not CW2-AT1 evidence.
func TestScaffoldAllocation(t *testing.T) {
	if _, err := New(0); err == nil {
		t.Fatal("zero capacity accepted")
	}
	b, err := New(2)
	if err != nil || b == nil {
		t.Fatalf("New: %v", err)
	}
}
