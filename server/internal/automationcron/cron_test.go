package automationcron

import (
	"testing"
	"time"
)

func TestNextUsesUTCAndReturnsFollowingTick(t *testing.T) {
	after := time.Date(2026, time.August, 10, 12, 7, 0, 0, time.FixedZone("CEST", 2*60*60))

	next, err := Next("15 * * * *", after)
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	want := time.Date(2026, time.August, 10, 10, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("Next = %v, want %v", next, want)
	}
	if next.Location() != time.UTC {
		t.Fatalf("Next location = %v, want UTC", next.Location())
	}
}

func TestNextRejectsInvalidExpression(t *testing.T) {
	if _, err := Next("0 0 6 * * 1", time.Now()); err == nil {
		t.Fatal("expected invalid six-field expression to fail")
	}
}
