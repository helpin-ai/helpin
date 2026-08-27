package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCalculateDealHealthScoreUsesStageRecencyAndDecayedSignals(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	deal := &model.CRMDeal{
		ID: "deal-1", WorkspaceID: "workspace-1", UpdatedAt: now,
		Stage: &model.CRMPipelineStage{StageType: model.CRMStageTypeOpen, Probability: 50},
	}
	signals := []model.CRMBuyerSignal{
		{SignalType: model.CRMSignalBuyingIntent, SourceType: model.CRMSignalSourceEmail, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
		{SignalType: model.CRMSignalBudgetSignal, SourceType: model.CRMSignalSourceEmail, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
	}

	score, factors := calculateDealHealthScore(deal, signals, now)
	if score != 82 {
		t.Fatalf("score = %d, want 82", score)
	}
	if factors["compound_signal_boost"] != float64(0) || factors["signal_count"] != 2 {
		t.Fatalf("factors = %#v", factors)
	}
}

func TestCalculateDealHealthScoreBoostsIndependentDomains(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	deal := &model.CRMDeal{UpdatedAt: now, Stage: &model.CRMPipelineStage{StageType: model.CRMStageTypeOpen, Probability: 50}}
	signals := []model.CRMBuyerSignal{
		{SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainConversation, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
		{SignalType: model.CRMSignalBudgetSignal, SignalDomain: model.CRMSignalDomainWebBehavior, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
	}
	_, factors := calculateDealHealthScore(deal, signals, now)
	if factors["independent_domains"] != 2 || factors["compound_signal_boost"] == float64(0) {
		t.Fatalf("cross-domain factors = %#v", factors)
	}
}

func TestCalculateDealHealthScorePinsClosedOutcomes(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		stageType string
		want      int
	}{
		{stageType: model.CRMStageTypeWon, want: 100},
		{stageType: model.CRMStageTypeLost, want: 0},
	} {
		deal := &model.CRMDeal{UpdatedAt: now, Stage: &model.CRMPipelineStage{StageType: test.stageType}}
		score, _ := calculateDealHealthScore(deal, nil, now)
		if score != test.want {
			t.Fatalf("stage %s score = %d, want %d", test.stageType, score, test.want)
		}
	}
}
