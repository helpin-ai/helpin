package service

import "testing"

func TestCoverageRolloutPolicyResolvesWorkspaceOverrides(t *testing.T) {
	policy, err := NewCoverageRolloutPolicy("shadow", `{"ws-read":"v2_read","ws-off":"disabled"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.ModeForWorkspace("ws-default"); got != CoverageRolloutShadow {
		t.Fatalf("default mode = %q", got)
	}
	if got := policy.ModeForWorkspace("ws-read"); got != CoverageRolloutV2Read {
		t.Fatalf("override mode = %q", got)
	}
	if policy.Evaluate("ws-off", CoverageRolloutMetrics{}).CaptureEnabled {
		t.Fatal("disabled workspace must not capture")
	}
}

func TestCoverageRolloutPolicyRejectsInvalidModes(t *testing.T) {
	if _, err := NewCoverageRolloutPolicy("launch_everywhere", ""); err == nil {
		t.Fatal("expected invalid default mode to fail")
	}
	if _, err := NewCoverageRolloutPolicy("shadow", `{"ws-1":"launch"}`); err == nil {
		t.Fatal("expected invalid workspace override to fail")
	}
}

func TestCoverageRolloutGuardrailsPauseCutoverButPreserveCapture(t *testing.T) {
	precision := 0.94
	tests := []struct {
		name    string
		metrics CoverageRolloutMetrics
	}{
		{"terminal failure rate", CoverageRolloutMetrics{AttemptCount: 100, TerminalFailureCount: 6}},
		{"duplicate idempotency", CoverageRolloutMetrics{DuplicateIdempotencyViolations: 1}},
		{"cross workspace", CoverageRolloutMetrics{CrossWorkspaceInvariantViolations: 1}},
		{"assignment precision", CoverageRolloutMetrics{AutoAttachPrecision: &precision}},
	}
	policy, err := NewCoverageRolloutPolicy("v2_write", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := policy.Evaluate("ws-1", test.metrics)
			if !decision.CaptureEnabled || decision.AssignmentEnabled || decision.ReadV2Enabled || decision.WriteV2Enabled || len(decision.PauseReasons) == 0 {
				t.Fatalf("guardrail decision = %+v", decision)
			}
		})
	}
}

func TestCoverageRolloutModesExposeOnlyTheirIntendedCapabilities(t *testing.T) {
	tests := []struct {
		mode                         string
		capture, assign, read, write bool
	}{
		{"disabled", false, false, false, false},
		{"shadow", true, true, false, false},
		{"v2_read", true, true, true, false},
		{"v2_write", true, true, true, true},
	}
	for _, test := range tests {
		policy, err := NewCoverageRolloutPolicy(test.mode, "")
		if err != nil {
			t.Fatal(err)
		}
		decision := policy.Evaluate("ws-1", CoverageRolloutMetrics{})
		if decision.CaptureEnabled != test.capture || decision.AssignmentEnabled != test.assign || decision.ReadV2Enabled != test.read || decision.WriteV2Enabled != test.write {
			t.Fatalf("mode %s decision = %+v", test.mode, decision)
		}
	}
}
