package main

import "testing"

func TestCoverageRebuildCommandDefaultsToDryRunAndRequiresWorkspace(t *testing.T) {
	if _, err := parseOptions(nil); err == nil {
		t.Fatal("expected workspace requirement")
	}
	options, err := parseOptions([]string{"--workspace-id", "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if options.apply || options.resumeID != "" || options.rollbackID != "" {
		t.Fatalf("options = %+v", options)
	}
}

func TestCoverageRebuildCommandRejectsConflictingMutationModes(t *testing.T) {
	if _, err := parseOptions([]string{"--workspace-id", "ws-1", "--apply", "--rollback", "audit-1"}); err == nil {
		t.Fatal("expected conflicting modes to fail")
	}
}
