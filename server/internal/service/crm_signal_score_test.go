package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSignalScoringKeepsConfidenceAndBusinessWeightSeparate(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	signal := model.CRMBuyerSignal{
		SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainConversation,
		Polarity: model.CRMSignalPolarityPositive, Confidence: 0.8,
		EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now,
	}
	defaultSignalScoringProfile().scoreSignal(&signal, now)
	if signal.BusinessPriority != 12 {
		t.Fatalf("business priority = %v, want 12", signal.BusinessPriority)
	}
	if signal.ScoreFactors["business_weight"] != float64(15) || signal.ScoreFactors["extraction_confidence"] != float64(0.8) {
		t.Fatalf("score factors conflate confidence and weight: %#v", signal.ScoreFactors)
	}
}

func TestComposeSignalStoriesBoostsOnlyIndependentDomains(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	companyID := "company-1"
	profile := defaultSignalScoringProfile()
	signals := []model.CRMBuyerSignal{
		{ID: "one", CompanyID: &companyID, AccountName: "Acme", SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainConversation, Polarity: model.CRMSignalPolarityPositive, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
		{ID: "two", CompanyID: &companyID, AccountName: "Acme", SignalType: model.CRMSignalBudgetSignal, SignalDomain: model.CRMSignalDomainWebBehavior, Polarity: model.CRMSignalPolarityPositive, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now},
	}
	for index := range signals {
		profile.scoreSignal(&signals[index], now)
	}
	stories := composeSignalStories(signals, profile, now)
	if len(stories) != 1 || stories[0].ScoreFactors["independent_domains"] != 2 {
		t.Fatalf("stories = %#v", stories)
	}
	if stories[0].ScoreFactors["compound_boost"] != float64(0.15) || len(stories[0].Signals) != 2 {
		t.Fatalf("cross-domain composition = %#v", stories[0])
	}
}

func TestSignalScoringDiscountsUntrustedIdentity(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	verified := model.CRMBuyerSignal{SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainWebBehavior, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now}
	anonymous := verified
	anonymous.EvidenceIdentityTrust = model.IdentityTrustUntrusted
	profile := defaultSignalScoringProfile()
	profile.scoreSignal(&verified, now)
	profile.scoreSignal(&anonymous, now)
	if anonymous.BusinessPriority >= verified.BusinessPriority || anonymous.Confidence != verified.Confidence {
		t.Fatalf("identity trust should affect priority only: verified=%#v anonymous=%#v", verified, anonymous)
	}
}
