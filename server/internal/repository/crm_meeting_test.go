package repository

import (
	"strings"
	"testing"
)

func TestMeetingCaptureLaunchLockKeyIsPostgresTextSafeAndUnambiguous(t *testing.T) {
	key := meetingCaptureLaunchLockKey(" workspace-1 ", " request-1 ")
	if strings.ContainsRune(key, '\x00') {
		t.Fatalf("lock key contains a PostgreSQL-incompatible NUL byte: %q", key)
	}
	if key != "11:workspace-1:request-1" {
		t.Fatalf("unexpected normalized lock key: %q", key)
	}
	if meetingCaptureLaunchLockKey("ab", "c") == meetingCaptureLaunchLockKey("a", "bc") {
		t.Fatal("lock key encoding is ambiguous")
	}
}
