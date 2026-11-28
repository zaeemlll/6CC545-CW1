package membership

import (
	"testing"
	"time"
)

func TestScaffoldAllocation(t *testing.T) {
	x, e := New(8, 3*time.Second)
	if e != nil || x == nil {
		t.Fatal(e)
	}
	if _, e = New(0, time.Second); e == nil {
		t.Fatal("zero max")
	}
}
