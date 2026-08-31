package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func validExternalEvidenceType(value string) bool {
	switch value {
	case model.CRMExternalEvidenceFunding, model.CRMExternalEvidenceHiring,
		model.CRMExternalEvidenceJobChange, model.CRMExternalEvidenceTechnology,
		model.CRMExternalEvidenceLeadership, model.CRMExternalEvidenceThirdPartyIntent:
		return true
	default:
		return false
	}
}

func validSignalDomain(value string) bool {
	switch value {
	case model.CRMSignalDomainConversation, model.CRMSignalDomainWebBehavior,
		model.CRMSignalDomainProductUsage, model.CRMSignalDomainSupport,
		model.CRMSignalDomainDelivery, model.CRMSignalDomainRelationship,
		model.CRMSignalDomainMarket:
		return true
	default:
		return false
	}
}

func validSignalPolarity(value string) bool {
	return value == model.CRMSignalPolarityPositive || value == model.CRMSignalPolarityNegative || value == model.CRMSignalPolarityNeutral
}

func (s *CRMSignalService) IngestExternalEvidence(ctx context.Context, req model.IngestCRMSignalExternalEvidenceRequest) (*model.CRMSignalExternalEvidence, bool, error) {
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.ProviderEvidenceID = strings.TrimSpace(req.ProviderEvidenceID)
	req.RuleKey = strings.TrimSpace(req.RuleKey)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.WorkspaceID == "" || req.Provider == "" || req.ProviderEvidenceID == "" || req.RuleKey == "" || req.RuleVersion < 1 || req.Summary == "" {
		return nil, false, fmt.Errorf("workspace, provider, evidence ID, rule version, and summary are required")
	}
	if len(req.Provider) > 64 || strings.ContainsAny(req.Provider, " /\\") {
		return nil, false, fmt.Errorf("invalid provider")
	}
	if req.RuleKey != model.CRMSignalRuleExternalEvidence {
		return nil, false, fmt.Errorf("external evidence must use the external provider rule")
	}
	if !validExternalEvidenceType(req.EvidenceType) || !validDetectedSignalType(req.SignalType) || req.SignalDomain != model.CRMSignalDomainMarket || !validSignalPolarity(req.Polarity) {
		return nil, false, fmt.Errorf("invalid normalized evidence dimensions")
	}
	if req.ContactID == nil && req.DealID == nil && req.CompanyID == nil {
		return nil, false, fmt.Errorf("external evidence must target a CRM contact, company, or deal")
	}
	if req.ObservedAt.IsZero() || req.ObservedAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return nil, false, fmt.Errorf("valid observed_at is required")
	}
	config, err := s.signalRepo.GetSignalRuleConfigVersion(ctx, req.WorkspaceID, req.RuleKey, req.RuleVersion)
	if err != nil {
		return nil, false, err
	}
	if config == nil {
		return nil, false, fmt.Errorf("signal rule version not found")
	}
	if !config.Enabled {
		return nil, false, fmt.Errorf("signal rule version is disabled")
	}
	if err := s.signalRepo.ValidateSignalEntityScope(ctx, req.WorkspaceID, req.ContactID, req.DealID, req.CompanyID); err != nil {
		return nil, false, err
	}
	identityMethod := "external_provider:" + req.Provider
	identityTrust := model.IdentityTrustProbabilistic
	evidence := &model.CRMSignalExternalEvidence{
		WorkspaceID: req.WorkspaceID, Provider: req.Provider, ProviderEvidenceID: req.ProviderEvidenceID,
		EvidenceType: req.EvidenceType, RuleKey: req.RuleKey, RuleVersion: req.RuleVersion,
		SignalType: req.SignalType, SignalDomain: req.SignalDomain, Polarity: req.Polarity,
		Summary: req.Summary, EvidenceExcerpt: req.EvidenceExcerpt, SourceURL: req.SourceURL,
		ContactID: req.ContactID, DealID: req.DealID, CompanyID: req.CompanyID,
		IdentityMethod: identityMethod, IdentityTrust: identityTrust,
		Provenance: model.JSONB(req.Provenance), ObservedAt: req.ObservedAt.UTC(),
	}
	fingerprint := sha256.Sum256([]byte(strings.Join([]string{req.Provider, req.ProviderEvidenceID, req.RuleKey, fmt.Sprint(req.RuleVersion)}, "\x00")))
	ruleKey, ruleVersion := req.RuleKey, req.RuleVersion
	windowStart, windowEnd := req.ObservedAt.UTC(), req.ObservedAt.UTC().Add(time.Nanosecond)
	signal := &model.CRMSignal{
		WorkspaceID: req.WorkspaceID, ContactID: req.ContactID, DealID: req.DealID, CompanyID: req.CompanyID,
		SignalType: req.SignalType, SourceType: model.CRMSignalSourceExternal,
		Summary: req.Summary, EvidenceExcerpt: req.EvidenceExcerpt,
		Metadata:   model.JSONB{"provider": req.Provider, "evidence_type": req.EvidenceType, "source_url": signalStringValue(req.SourceURL), "provenance": req.Provenance, "shadow_mode": config.ShadowMode},
		Confidence: 1, DetectedAt: req.ObservedAt.UTC(), DetectorKind: model.CRMSignalDetectorRuleDerived,
		SignalDomain: req.SignalDomain, Polarity: req.Polarity, RuleKey: &ruleKey, RuleVersion: &ruleVersion,
		WindowStartedAt: &windowStart, WindowEndedAt: &windowEnd,
		EvidenceIdentityMethod: identityMethod, EvidenceIdentityTrust: identityTrust,
		EvidenceFingerprint: fmt.Sprintf("%x", fingerprint[:]),
	}
	created, err := s.signalRepo.IngestExternalEvidence(ctx, evidence, signal)
	if err != nil {
		return nil, false, err
	}
	if created {
		s.requestSummaryRefresh(ctx, signal)
	}
	return evidence, created, nil
}
