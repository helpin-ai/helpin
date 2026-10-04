package model

import (
	"testing"
	"time"
)

func TestNextContentSyncUsesDailyUTCSlot(t *testing.T) {
	for _, tc := range []struct{ now, want string }{
		{"2026-10-01T01:59:59Z", "2026-10-01T02:00:00Z"},
		{"2026-10-01T02:00:00Z", "2026-10-02T02:00:00Z"},
		{"2026-10-01T23:00:00-04:00", "2026-10-03T02:00:00Z"},
	} {
		now, err := time.Parse(time.RFC3339, tc.now)
		if err != nil {
			t.Fatal(err)
		}
		if got := NextContentSourceAutoSync(now).Format(time.RFC3339); got != tc.want {
			t.Fatalf("next(%s)=%s, want %s", tc.now, got, tc.want)
		}
	}
}
