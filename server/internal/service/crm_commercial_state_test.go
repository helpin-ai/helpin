package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestApplyCommercialStatePatchPreservesOmittedAndClearsNull(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	current := model.JSONB{"plan_key": "growth", "seats_purchased": float64(20)}
	result, changed, err := ApplyCommercialStatePatch(
		current, now.Add(-time.Hour), model.JSONB{"plan_key": nil}, now, now,
	)
	if err != nil || !changed {
		t.Fatalf("ApplyCommercialStatePatch changed=%v err=%v", changed, err)
	}
	if _, exists := result["plan_key"]; exists {
		t.Fatal("explicit null must clear plan_key")
	}
	if result["seats_purchased"] != float64(20) {
		t.Fatalf("omitted seats_purchased = %v", result["seats_purchased"])
	}
}

func TestApplyCommercialStatePatchRejectsFutureAndTimestampConflict(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	if _, _, err := ApplyCommercialStatePatch(nil, time.Time{}, model.JSONB{}, now.Add(6*time.Minute), now); err == nil {
		t.Fatal("expected future-skew rejection")
	}
	if _, _, err := ApplyCommercialStatePatch(
		model.JSONB{"plan_key": "growth"}, now, model.JSONB{"plan_key": "enterprise"}, now, now,
	); err == nil {
		t.Fatal("expected same-timestamp conflict")
	}
}

func TestApplyCommercialStatePatchClassifiesStaleEvidenceForHistory(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	result, changed, err := ApplyCommercialStatePatch(
		model.JSONB{"plan_key": "growth"}, now, model.JSONB{"plan_key": "starter"}, now.Add(-time.Hour), now,
	)
	if !errors.Is(err, errCommercialStateStale) || changed || result["plan_key"] != "starter" {
		t.Fatalf("stale patch result=%#v changed=%v err=%v", result, changed, err)
	}
}

func TestUsageDeclineAndAnomalyGuard(t *testing.T) {
	if !IsUsageDecline(
		[]float64{50, 50, 50, 50, 50, 90, 90},
		[]float64{100, 100, 100, 100, 100, 100, 100},
	) {
		t.Fatal("expected five-of-seven decline")
	}
	if ShouldSuppressUsageDeclineBatch(25, 9) {
		t.Fatal("small workspaces must not be batch-suppressed")
	}
	if !ShouldSuppressUsageDeclineBatch(100, 35) {
		t.Fatal("expected large-workspace anomaly suppression")
	}
}

func TestWorkspaceLocalMidnightUsesWorkspaceTimezone(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 30, 0, 0, time.UTC)
	losAngeles, err := workspaceLocalMidnightUTC(now, "America/Los_Angeles")
	if err != nil || !losAngeles.Equal(time.Date(2026, 8, 27, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("Los Angeles midnight=%s err=%v", losAngeles, err)
	}
	sydney, err := workspaceLocalMidnightUTC(now, "Australia/Sydney")
	if err != nil || !sydney.Equal(time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("Sydney midnight=%s err=%v", sydney, err)
	}
}

func TestCommercialStateLeaseIsWorkspaceScoped(t *testing.T) {
	first := commercialStateLeaseKey("workspace-1")
	second := commercialStateLeaseKey("workspace-2")
	if first == second || first != "commercial_state_sync:workspace-1" {
		t.Fatalf("workspace lease keys are not isolated: %q %q", first, second)
	}
}

func TestCapacityConditionTransitionUsesHysteresis(t *testing.T) {
	emit, armed := CapacityConditionTransition(true, 85, 100)
	if !emit || armed {
		t.Fatalf("threshold crossing emit=%v armed=%v", emit, armed)
	}
	emit, armed = CapacityConditionTransition(false, 82, 100)
	if emit || armed {
		t.Fatalf("condition should remain disarmed emit=%v armed=%v", emit, armed)
	}
	emit, armed = CapacityConditionTransition(false, 80, 100)
	if emit || !armed {
		t.Fatalf("recovery should re-arm emit=%v armed=%v", emit, armed)
	}
}

func TestCommercialCandidateOriginGateRechecksPersistedIdentityMethod(t *testing.T) {
	browser := &model.CRMSignalRuleCandidate{
		RuleKey:                model.CRMSignalRulePaymentFailed,
		EvidenceIdentityMethod: model.IdentityMethodSignedWidget,
		Metadata:               model.JSONB{"event_type": "payment_failed"},
	}
	if commercialCandidateOriginAllowed(browser) {
		t.Fatal("browser-origin reserved event must not produce an observation")
	}
	server := *browser
	server.EvidenceIdentityMethod = model.IdentityMethodServerEvent
	if !commercialCandidateOriginAllowed(&server) {
		t.Fatal("server_event identity method must satisfy the Go origin recheck")
	}
	missingType := server
	missingType.Metadata = model.JSONB{}
	if commercialCandidateOriginAllowed(&missingType) {
		t.Fatal("a server-only rule without event_type must fail closed")
	}
}

func TestCommercialStateMaterializerRejectsBrowserOriginBeforeStorage(t *testing.T) {
	svc := NewCRMSignalService(nil, nil)
	_, err := svc.MaterializeCompanyCommercialState(
		context.Background(), "workspace", "company", "event", model.IdentityMethodSignedWidget,
		model.JSONB{"plan_key": "growth"}, time.Now(), time.Now(),
	)
	if err == nil {
		t.Fatal("browser-origin commercial state must be rejected before repository access")
	}
}
