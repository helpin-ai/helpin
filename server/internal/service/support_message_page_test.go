package service

import (
	"testing"
	"time"
)

func TestSupportMessageCursorRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 8, 18, 10, 15, 0, 123, time.UTC)
	cursor := encodeSupportMessageCursor(createdAt, "msg-20")

	decodedAt, decodedID, err := decodeSupportMessageCursor(cursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !decodedAt.Equal(createdAt) || decodedID != "msg-20" {
		t.Fatalf("decoded cursor = (%s, %q), want (%s, %q)", decodedAt, decodedID, createdAt, "msg-20")
	}
}

func TestSupportMessageCursorRejectsMalformedValues(t *testing.T) {
	for _, cursor := range []string{"not-base64", "e30", "eyJjcmVhdGVkX2F0IjoiYmFkIiwiaWQiOiJtc2cifQ"} {
		if _, _, err := decodeSupportMessageCursor(cursor); err == nil {
			t.Fatalf("decode cursor %q returned nil error", cursor)
		}
	}
}
