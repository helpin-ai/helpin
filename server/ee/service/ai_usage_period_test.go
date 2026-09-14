//go:build ee

package service

import (
	"testing"
	"time"
)

func TestNextAIUsageBoundaryUsesRenewalAnniversaryAndClampsMonthEnd(t *testing.T) {
	anchor := time.Date(2026, 1, 31, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		after, want time.Time
	}{
		{after: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), want: time.Date(2026, 2, 28, 9, 30, 0, 0, time.UTC)},
		{after: time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC), want: time.Date(2026, 3, 31, 9, 30, 0, 0, time.UTC)},
		{after: time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC), want: time.Date(2028, 2, 29, 9, 30, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		if got := NextAIUsageBoundary(anchor, test.after); !got.Equal(test.want) {
			t.Fatalf("NextAIUsageBoundary(%s) = %s, want %s", test.after, got, test.want)
		}
	}
}
