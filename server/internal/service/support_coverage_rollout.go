package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type CoverageRolloutMode string

const (
	CoverageRolloutDisabled CoverageRolloutMode = "disabled"
	CoverageRolloutShadow   CoverageRolloutMode = "shadow"
	CoverageRolloutV2Read   CoverageRolloutMode = "v2_read"
	CoverageRolloutV2Write  CoverageRolloutMode = "v2_write"
)

const (
	coverageRolloutFailureRateGate = 0.05
	coverageRolloutPrecisionGate   = 0.95
	coverageRolloutMinAttempts     = 20
)

type CoverageRolloutMetrics struct {
	AttemptCount                      int
	TerminalFailureCount              int
	DuplicateIdempotencyViolations    int
	CrossWorkspaceInvariantViolations int
	AutoAttachPrecision               *float64
}

type CoverageRolloutDecision struct {
	RequestedMode     CoverageRolloutMode `json:"requested_mode"`
	CaptureEnabled    bool                `json:"capture_enabled"`
	AssignmentEnabled bool                `json:"assignment_enabled"`
	ReadV2Enabled     bool                `json:"read_v2_enabled"`
	WriteV2Enabled    bool                `json:"write_v2_enabled"`
	PauseReasons      []string            `json:"pause_reasons,omitempty"`
}

type CoverageRolloutPolicy struct {
	defaultMode CoverageRolloutMode
	overrides   map[string]CoverageRolloutMode
}

func NewCoverageRolloutPolicy(defaultMode, workspaceOverridesJSON string) (*CoverageRolloutPolicy, error) {
	mode, err := parseCoverageRolloutMode(defaultMode)
	if err != nil {
		return nil, err
	}
	overrides := map[string]CoverageRolloutMode{}
	if strings.TrimSpace(workspaceOverridesJSON) != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(workspaceOverridesJSON), &raw); err != nil {
			return nil, fmt.Errorf("parse SUPPORT_COVERAGE_V2_WORKSPACE_MODES: %w", err)
		}
		for workspaceID, value := range raw {
			workspaceID = strings.TrimSpace(workspaceID)
			if workspaceID == "" {
				return nil, fmt.Errorf("coverage rollout workspace override has an empty workspace_id")
			}
			override, err := parseCoverageRolloutMode(value)
			if err != nil {
				return nil, fmt.Errorf("workspace %s: %w", workspaceID, err)
			}
			overrides[workspaceID] = override
		}
	}
	return &CoverageRolloutPolicy{defaultMode: mode, overrides: overrides}, nil
}

func parseCoverageRolloutMode(value string) (CoverageRolloutMode, error) {
	mode := CoverageRolloutMode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case CoverageRolloutDisabled, CoverageRolloutShadow, CoverageRolloutV2Read, CoverageRolloutV2Write:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported coverage rollout mode %q", value)
	}
}

func (p *CoverageRolloutPolicy) ModeForWorkspace(workspaceID string) CoverageRolloutMode {
	if p == nil {
		return CoverageRolloutV2Write
	}
	if mode, ok := p.overrides[strings.TrimSpace(workspaceID)]; ok {
		return mode
	}
	return p.defaultMode
}

func (p *CoverageRolloutPolicy) CaptureEnabled(workspaceID string) bool {
	return p.Evaluate(workspaceID, CoverageRolloutMetrics{}).CaptureEnabled
}

func (p *CoverageRolloutPolicy) Evaluate(workspaceID string, metrics CoverageRolloutMetrics) CoverageRolloutDecision {
	mode := p.ModeForWorkspace(workspaceID)
	decision := CoverageRolloutDecision{RequestedMode: mode}
	switch mode {
	case CoverageRolloutShadow:
		decision.CaptureEnabled = true
		decision.AssignmentEnabled = true
	case CoverageRolloutV2Read:
		decision.CaptureEnabled = true
		decision.AssignmentEnabled = true
		decision.ReadV2Enabled = true
	case CoverageRolloutV2Write:
		decision.CaptureEnabled = true
		decision.AssignmentEnabled = true
		decision.ReadV2Enabled = true
		decision.WriteV2Enabled = true
	}
	if !decision.CaptureEnabled {
		return decision
	}
	if metrics.AttemptCount >= coverageRolloutMinAttempts && float64(metrics.TerminalFailureCount)/float64(metrics.AttemptCount) > coverageRolloutFailureRateGate {
		decision.PauseReasons = append(decision.PauseReasons, "terminal_failure_rate")
	}
	if metrics.DuplicateIdempotencyViolations > 0 {
		decision.PauseReasons = append(decision.PauseReasons, "duplicate_idempotency_violation")
	}
	if metrics.CrossWorkspaceInvariantViolations > 0 {
		decision.PauseReasons = append(decision.PauseReasons, "cross_workspace_invariant")
	}
	if metrics.AutoAttachPrecision != nil && *metrics.AutoAttachPrecision < coverageRolloutPrecisionGate {
		decision.PauseReasons = append(decision.PauseReasons, "auto_attach_precision")
	}
	if len(decision.PauseReasons) > 0 {
		sort.Strings(decision.PauseReasons)
		decision.AssignmentEnabled = false
		decision.ReadV2Enabled = false
		decision.WriteV2Enabled = false
	}
	return decision
}
