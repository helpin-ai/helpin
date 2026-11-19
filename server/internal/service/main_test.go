package service

import (
	"os"
	"testing"
	"time"
)

// TestMain pins the local time zone to UTC. The tests run on SQLite, which
// stores timestamps as text and compares them as strings, so a time written
// in local time (time.Now()) and one written in UTC sort wrongly whenever the
// machine is not on UTC. CI and production both run in UTC; this makes a
// developer machine in any other zone behave the same.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}
