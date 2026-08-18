package service

import (
	"errors"
	"testing"
	"time"
)

func TestDockRunCursorRoundTrip(t *testing.T) {
	wantTime := time.Date(2026, 8, 18, 12, 30, 0, 0, time.UTC)
	cursor := encodeDockRunCursor(wantTime, "run-30")
	gotTime, gotID, err := decodeDockRunCursor(cursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !gotTime.Equal(wantTime) || gotID != "run-30" {
		t.Fatalf("decoded cursor = (%v, %q), want (%v, %q)", gotTime, gotID, wantTime, "run-30")
	}
	if _, _, err := decodeDockRunCursor("bad-cursor"); !errors.Is(err, ErrDockRunInvalidCursor) {
		t.Fatalf("invalid cursor error = %v, want %v", err, ErrDockRunInvalidCursor)
	}
}
