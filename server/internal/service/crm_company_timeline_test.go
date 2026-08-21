package service

import (
	"testing"
	"time"
)

func TestCRMCompanyTimelineCursorRoundTrip(t *testing.T) {
	want := crmCompanyTimelineCursor{
		Version: 1,
		At:      time.Date(2026, time.August, 21, 10, 30, 0, 123, time.UTC),
		ID:      "task:activity-42",
	}
	encoded, err := encodeCompanyTimelineCursor(want)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	got, err := decodeCompanyTimelineCursor(encoded)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if got.Version != want.Version || !got.At.Equal(want.At) || got.ID != want.ID {
		t.Fatalf("decoded cursor = %#v, want %#v", got, want)
	}
}

func TestCRMCompanyTimelineCursorRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"not-base64", "e30"} {
		if _, err := decodeCompanyTimelineCursor(value); err == nil {
			t.Fatalf("expected cursor %q to be rejected", value)
		}
	}
}
