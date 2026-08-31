package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSignalScoringKeepsConfidenceAndBusinessWeightSeparate(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	signal := model.CRMSignal{
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

func TestSignalRescoringCannotChangeSnapshottedMeaning(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	signal := model.CRMSignal{
		SignalType: model.CRMSignalRiskSignal, SignalDomain: model.CRMSignalDomainProductUsage,
		CommercialMotion: model.CRMCommercialMotionRetention, Polarity: model.CRMSignalPolarityNegative,
		MeaningFingerprint: "immutable-meaning", BusinessWeightSnapshot: 17, HalfLifeDaysSnapshot: 11,
		Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now.Add(-24 * time.Hour),
	}
	profile := defaultSignalScoringProfile()
	profile.scoreSignal(&signal, now)
	firstType, firstMotion, firstPolarity, firstFingerprint := signal.SignalType, signal.CommercialMotion, signal.Polarity, signal.MeaningFingerprint
	profile.signalWeights[model.CRMSignalRiskSignal] = 99
	profile.halfLives[model.CRMSignalRiskSignal] = 1
	profile.scoreSignal(&signal, now.Add(24*time.Hour))
	if signal.SignalType != firstType || signal.CommercialMotion != firstMotion || signal.Polarity != firstPolarity || signal.MeaningFingerprint != firstFingerprint {
		t.Fatalf("rescoring changed immutable meaning: %#v", signal)
	}
	if signal.ScoreFactors["business_weight"] != float64(17) || signal.ScoreFactors["half_life_days"] != float64(11) {
		t.Fatalf("rescoring ignored detection-time snapshots: %#v", signal.ScoreFactors)
	}
}

func TestComposeSignalStoriesBoostsOnlyIndependentDomains(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	companyID := "company-1"
	profile := defaultSignalScoringProfile()
	signals := []model.CRMSignal{
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

func TestComposeSignalStoriesCorrelatesSignalsFromSupportConversations(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	companyID := "company-1"
	signals := make([]model.CRMSignal, 0, 6)
	for index, signedImpact := range []float64{8, 8, -8} {
		threadID := []string{"conversation-1", "conversation-2", "conversation-3"}[index]
		messageID := []string{"message-1", "message-2", "message-3"}[index]
		detectedAt := now.Add(-time.Duration(index) * time.Hour)
		signals = append(signals,
			model.CRMSignal{
				ID: "escalation-" + threadID, CompanyID: &companyID, AccountName: "Acme",
				SignalType: model.CRMSignalTimelineSignal, SignalDomain: model.CRMSignalDomainSupport,
				SourceType: model.CRMSignalSourceSupport, SourceID: &threadID,
				Polarity: model.CRMSignalPolarityNeutral, BusinessPriority: 11,
				DetectedAt: detectedAt,
			},
			model.CRMSignal{
				ID: "extracted-" + messageID, CompanyID: &companyID, AccountName: "Acme",
				SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainConversation,
				SourceType: model.CRMSignalSourceSupport, SourceID: &messageID, SourceThreadID: &threadID,
				Polarity: model.CRMSignalPolarityPositive, BusinessPriority: 8, SignedImpact: signedImpact,
				DetectedAt: detectedAt,
			},
		)
	}

	stories := composeSignalStories(signals, defaultSignalScoringProfile(), now)
	if len(stories) != 1 {
		t.Fatalf("stories=%#v, want one account story", stories)
	}
	story := stories[0]
	if len(story.Signals) != 6 || story.EvidenceSourceCount != 3 || story.ChangedEvidenceSourceCount != 3 {
		t.Fatalf("evidence counts=%#v", story)
	}
	if story.Priority != 33 || story.SignedImpact != 8 || story.Polarity != model.CRMSignalPolarityPositive {
		t.Fatalf("correlated composition priority=%v impact=%v polarity=%s", story.Priority, story.SignedImpact, story.Polarity)
	}
	if len(story.Domains) != 2 || story.ScoreFactors["independent_domains"] != 1 || story.ScoreFactors["compound_boost"] != float64(0) {
		t.Fatalf("domain composition=%#v", story.ScoreFactors)
	}
	if story.ScoreFactors["correlated_signal_count"] != 3 || story.ChangeSummary != "3 new conversations · 6 signals across 2 domains" {
		t.Fatalf("source correlation factors=%#v summary=%q", story.ScoreFactors, story.ChangeSummary)
	}
}

func TestSignalEvidenceSourceKeyFallsBackToFingerprint(t *testing.T) {
	first := model.CRMSignal{ID: "signal-1", SourceType: model.CRMSignalSourceWeb, EvidenceFingerprint: "evidence-1"}
	second := model.CRMSignal{ID: "signal-2", SourceType: model.CRMSignalSourceWeb, EvidenceFingerprint: "evidence-1"}
	if signalEvidenceSourceKey(first) != signalEvidenceSourceKey(second) {
		t.Fatal("matching fingerprints should identify one evidence source")
	}
	second.EvidenceFingerprint = ""
	if signalEvidenceSourceKey(first) == signalEvidenceSourceKey(second) {
		t.Fatal("a signal without source provenance should fall back to its own ID")
	}
}

func TestComposeSignalStoriesKeepsOpposingEvidenceInsideOneSource(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	companyID, sourceID := "company-1", "product-window-1"
	signals := []model.CRMSignal{
		{ID: "expansion", CompanyID: &companyID, SourceType: model.CRMSignalSourceProduct, SourceID: &sourceID,
			CommercialMotion: model.CRMCommercialMotionExpansion, Polarity: model.CRMSignalPolarityPositive,
			BusinessPriority: 10, SignedImpact: 10, DetectedAt: now},
		{ID: "risk", CompanyID: &companyID, SourceType: model.CRMSignalSourceProduct, SourceID: &sourceID,
			CommercialMotion: model.CRMCommercialMotionExpansion, Polarity: model.CRMSignalPolarityNegative,
			BusinessPriority: 9, SignedImpact: -9, DetectedAt: now},
	}
	stories := composeSignalStories(signals, defaultSignalScoringProfile(), now)
	if len(stories) != 1 || !stories[0].NeedsJudgment || stories[0].RecommendedActionKey != nil {
		t.Fatalf("same-source opposing evidence should require judgment: %#v", stories)
	}
	if stories[0].Priority != 10 {
		t.Fatalf("opposing evidence must not lower source-correlated priority: %#v", stories[0])
	}
}

func TestPaginateSignalLanesRanksAndPagesEachMotionIndependently(t *testing.T) {
	stories := make([]model.CRMSignalAccountStory, 0, 32)
	for index := 0; index < 30; index++ {
		stories = append(stories, model.CRMSignalAccountStory{ID: fmt.Sprintf("expansion-%d", index), CommercialMotion: model.CRMCommercialMotionExpansion})
	}
	for index := 0; index < 2; index++ {
		stories = append(stories, model.CRMSignalAccountStory{ID: fmt.Sprintf("retention-%d", index), CommercialMotion: model.CRMCommercialMotionRetention})
	}
	lanes, _ := paginateSignalLanes(stories, 1, 20, map[string]int{model.CRMCommercialMotionExpansion: 2})
	if len(lanes) != 2 || lanes[0].CommercialMotion != model.CRMCommercialMotionExpansion || lanes[0].Total != 30 || len(lanes[0].Data) != 10 || lanes[0].Page != 2 {
		t.Fatalf("expansion lane = %#v", lanes)
	}
	if lanes[1].CommercialMotion != model.CRMCommercialMotionRetention || lanes[1].Total != 2 || len(lanes[1].Data) != 2 || lanes[1].Page != 1 {
		t.Fatalf("retention lane = %#v", lanes)
	}
}

func TestSignalScoringDiscountsUntrustedIdentity(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	verified := model.CRMSignal{SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainWebBehavior, Confidence: 1, EvidenceIdentityTrust: model.IdentityTrustVerified, DetectedAt: now}
	anonymous := verified
	anonymous.EvidenceIdentityTrust = model.IdentityTrustUntrusted
	profile := defaultSignalScoringProfile()
	profile.scoreSignal(&verified, now)
	profile.scoreSignal(&anonymous, now)
	if anonymous.BusinessPriority >= verified.BusinessPriority || anonymous.Confidence != verified.Confidence {
		t.Fatalf("identity trust should affect priority only: verified=%#v anonymous=%#v", verified, anonymous)
	}
}

func TestNeutralSignalRanksWithoutCreatingMomentum(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	companyID := "company-1"
	ruleKey := model.CRMSignalRuleSessionDepthSpike
	signal := model.CRMSignal{
		ID: "deep-session", CompanyID: &companyID, AccountName: "Acme",
		SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainWebBehavior,
		Polarity: model.CRMSignalPolarityNeutral, Confidence: 1,
		EvidenceIdentityTrust: model.IdentityTrustUntrusted, DetectedAt: now, RuleKey: &ruleKey,
	}
	profile := defaultSignalScoringProfile()
	profile.scoreSignal(&signal, now)
	if signal.BusinessPriority <= 0 || signal.SignedImpact != 0 {
		t.Fatalf("neutral score priority=%v signed impact=%v, want ranked context with zero impact", signal.BusinessPriority, signal.SignedImpact)
	}
	stories := composeSignalStories([]model.CRMSignal{signal}, profile, now)
	if len(stories) != 1 || stories[0].Polarity != model.CRMSignalPolarityNeutral || stories[0].Priority <= 0 {
		t.Fatalf("neutral story = %#v", stories)
	}
}
