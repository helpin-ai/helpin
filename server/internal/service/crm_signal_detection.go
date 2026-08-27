package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmtext"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	repeatedThreadSignalWindow    = 24 * time.Hour
	minimumDetectedConfidence     = 0.6
	signalEvidenceDetectorVersion = "verified-v3-en"
)

// SignalDetectionService uses LLM to detect buyer signals from various sources.
type SignalDetectionService struct {
	llmProvider    llm.Provider
	signalRepo     *repository.CRMSignalRepository
	summaryRefresh interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	}
	healthScoreRefresh interface {
		RefreshDealHealthScore(ctx context.Context, workspaceID, dealID string) (*model.CRMDealHealthScore, error)
	}
}

// NewSignalDetectionService creates a new signal detection service.
func NewSignalDetectionService(llmProvider llm.Provider, signalRepo *repository.CRMSignalRepository, summaryRefresh interface {
	RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
	RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
}) *SignalDetectionService {
	return &SignalDetectionService{
		llmProvider:    llmProvider,
		signalRepo:     signalRepo,
		summaryRefresh: summaryRefresh,
	}
}

// SetHealthScoreRefresh recalculates deal health when verified evidence changes.
func (s *SignalDetectionService) SetHealthScoreRefresh(refresh interface {
	RefreshDealHealthScore(ctx context.Context, workspaceID, dealID string) (*model.CRMDealHealthScore, error)
}) *SignalDetectionService {
	s.healthScoreRefresh = refresh
	return s
}

// DetectedSignal is the parsed LLM output for a single signal.
type DetectedSignal struct {
	SourceType   string  `json:"source_type"`
	SourceID     string  `json:"source_id"`
	SignalType   string  `json:"signal_type"`
	Summary      string  `json:"summary"`
	Confidence   float64 `json:"confidence"`
	RawEvidence  string  `json:"raw_evidence"`
	TimelineDate string  `json:"timeline_date,omitempty"`
}

// DetectSignals analyzes source payloads and detects buyer signals.
func (s *SignalDetectionService) DetectSignals(ctx context.Context, payloads []model.SignalSourcePayload) ([]model.CRMBuyerSignal, error) {
	if len(payloads) == 0 {
		return nil, nil
	}
	if s.llmProvider == nil {
		return nil, fmt.Errorf("LLM provider not configured")
	}

	// Build the analysis prompt
	payloadJSON, err := json.Marshal(payloads)
	if err != nil {
		return nil, fmt.Errorf("marshal payloads: %w", err)
	}
	payloadsByKey := make(map[string]model.SignalSourcePayload, len(payloads))
	for _, candidate := range payloads {
		payloadsByKey[signalSourceKey(candidate.SourceType, candidate.SourceID)] = candidate
	}

	primaryPayload := payloads[0]
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    primaryPayload.WorkspaceID,
		FeatureKey:     BillingFeatureCRMSignalDetection,
		IdempotencyKey: aiUsagePayloadIdempotencyKey(payloadJSON, primaryPayload.WorkspaceID, BillingFeatureCRMSignalDetection, signalEvidenceDetectorVersion, primaryPayload.SourceType, primaryPayload.SourceID, derefString(primaryPayload.SourceThreadID)),
		Metadata: map[string]interface{}{
			"source_type":  primaryPayload.SourceType,
			"source_id":    primaryPayload.SourceID,
			"thread_id":    derefString(primaryPayload.SourceThreadID),
			"source_count": len(payloads),
		},
		RequireComplete:    true,
		RetryInvalidOutput: true,
		ValidateResponse: func(response *llm.ChatResponse) error {
			if response == nil || strings.TrimSpace(response.Content) == "" {
				return fmt.Errorf("buyer signal model returned an empty response")
			}
			detected, parseErr := parseDetectedSignals(response.Content)
			if parseErr != nil {
				return parseErr
			}
			for _, item := range detected {
				if len(payloads) == 1 {
					if strings.TrimSpace(item.SourceType) == "" {
						item.SourceType = payloads[0].SourceType
					}
					if strings.TrimSpace(item.SourceID) == "" {
						item.SourceID = payloads[0].SourceID
					}
				}
				_, knownSource := payloadsByKey[signalSourceKey(item.SourceType, item.SourceID)]
				if !knownSource || !validDetectedSignalType(item.SignalType) {
					return fmt.Errorf("buyer signal model returned an unknown source or signal type")
				}
				if languageErr := validateEnglishCRMNarrative(item.Summary); languageErr != nil {
					return languageErr
				}
			}
			return nil
		},
		Chat: llm.ChatRequest{
			SystemPrompt: signalDetectionSystemPrompt,
			Messages: []llm.Message{
				{Role: "user", Content: string(payloadJSON)},
			},
			Temperature: 0.1,
			MaxTokens:   4096,
			JSONMode:    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM signal detection: %w", err)
	}

	detected, err := parseDetectedSignals(resp.Content)
	if err != nil {
		slog.Error("failed to parse signal detection response", "error", err, "content", resp.Content)
		return nil, err
	}

	keptTypesByKey := make(map[string][]string, len(payloads))
	reconcileSource := make(map[string]bool, len(payloads))
	for _, candidate := range payloads {
		key := signalSourceKey(candidate.SourceType, candidate.SourceID)
		keptTypesByKey[key] = nil
		reconcileSource[key] = true
	}

	// Create buyer signal records against the exact source declared by the model.
	var signals []model.CRMBuyerSignal
	for _, d := range detected {
		if len(payloads) == 1 {
			if strings.TrimSpace(d.SourceType) == "" {
				d.SourceType = payloads[0].SourceType
			}
			if strings.TrimSpace(d.SourceID) == "" {
				d.SourceID = payloads[0].SourceID
			}
		}
		key := signalSourceKey(d.SourceType, d.SourceID)
		payload, knownSource := payloadsByKey[key]
		if !knownSource || !validDetectedSignalType(d.SignalType) {
			slog.InfoContext(ctx, "skipping buyer signal with unknown source or type", "source_type", d.SourceType, "source_id", d.SourceID, "signal_type", d.SignalType)
			continue
		}
		if d.Confidence < minimumDetectedConfidence {
			continue
		}
		evidence, verified := verifiedSignalEvidence(payload, d.RawEvidence)
		if !verified {
			reconcileSource[key] = false
			slog.InfoContext(ctx, "skipping unverified buyer signal evidence", "workspace_id", payload.WorkspaceID, "source_type", payload.SourceType, "source_id", payload.SourceID, "signal_type", d.SignalType)
			continue
		}

		if payload.SourceThreadID != nil && *payload.SourceThreadID != "" {
			exists, err := s.signalRepo.HasRecentSignalForThread(ctx, payload.WorkspaceID, *payload.SourceThreadID, d.SignalType, payload.SourceID, time.Now().Add(-repeatedThreadSignalWindow))
			if err != nil {
				return nil, err
			}
			if exists {
				slog.Info("skipping repeated thread-level buyer signal", "workspace_id", payload.WorkspaceID, "thread_id", *payload.SourceThreadID, "signal_type", d.SignalType)
				continue
			}
		}

		metadata := buildSignalMetadata(payload)
		if d.SignalType == model.CRMSignalTimelineSignal && strings.TrimSpace(d.TimelineDate) != "" {
			parsedDate, parseErr := time.Parse("2006-01-02", strings.TrimSpace(d.TimelineDate))
			if parseErr != nil {
				slog.InfoContext(ctx, "ignoring invalid buyer signal timeline date", "workspace_id", payload.WorkspaceID, "source_id", payload.SourceID)
			} else {
				metadata["timeline_date"] = parsedDate.Format("2006-01-02")
			}
		}

		signal := model.CRMBuyerSignal{
			WorkspaceID:     payload.WorkspaceID,
			ContactID:       payload.ContactID,
			DealID:          payload.DealID,
			CompanyID:       payload.CompanyID,
			SignalType:      d.SignalType,
			SourceType:      payload.SourceType,
			SourceID:        &payload.SourceID,
			SourceThreadID:  payload.SourceThreadID,
			Summary:         d.Summary,
			EvidenceExcerpt: &evidence,
			Metadata:        metadata,
			Confidence:      d.Confidence,
			DetectedAt:      time.Now(),
		}

		created, err := s.signalRepo.CreateSignalIfAbsent(ctx, &signal)
		if err != nil {
			slog.Error("failed to store detected signal", "error", err, "signal_type", d.SignalType)
			continue
		}
		if !created {
			keptTypesByKey[key] = append(keptTypesByKey[key], d.SignalType)
			continue
		}

		keptTypesByKey[key] = append(keptTypesByKey[key], d.SignalType)
		s.requestSummaryRefresh(ctx, signal)

		signals = append(signals, signal)
	}
	for key, payload := range payloadsByKey {
		if !reconcileSource[key] {
			continue
		}
		if err := s.signalRepo.ReconcileAutomatedSignalsForSource(ctx, payload.WorkspaceID, payload.SourceType, payload.SourceID, keptTypesByKey[key]); err != nil {
			return signals, err
		}
	}

	slog.Info("signal detection complete", "payloads", len(payloads), "signals_detected", len(signals))
	return signals, nil
}

func parseDetectedSignals(content string) ([]DetectedSignal, error) {
	var detected []DetectedSignal
	if err := llm.UnmarshalResponse(content, &detected); err == nil {
		return detected, nil
	}
	var wrapper struct {
		Signals []DetectedSignal `json:"signals"`
	}
	if err := llm.UnmarshalResponse(content, &wrapper); err != nil {
		return nil, fmt.Errorf("parse detection response: %w", err)
	}
	return wrapper.Signals, nil
}

func signalSourceKey(sourceType, sourceID string) string {
	return strings.TrimSpace(sourceType) + ":" + strings.TrimSpace(sourceID)
}

func validDetectedSignalType(signalType string) bool {
	switch strings.TrimSpace(signalType) {
	case model.CRMSignalBuyingIntent, model.CRMSignalObjection, model.CRMSignalCompetitorMention,
		model.CRMSignalBudgetSignal, model.CRMSignalTimelineSignal, model.CRMSignalChampionSignal,
		model.CRMSignalRiskSignal:
		return true
	default:
		return false
	}
}

const signalDetectionSystemPrompt = `You are a sales intelligence analyst. Analyze the provided communication data and detect buyer signals.

Signal types to detect:
1. "buying_intent" — prospect shows interest in purchasing, asks about pricing, requests demos, mentions evaluating solutions
2. "objection" — prospect raises concerns, pushes back on features/price/timeline, mentions barriers
3. "competitor_mention" — prospect mentions competing products/vendors by name or alludes to alternatives
4. "budget_signal" — prospect discusses budget availability, approval processes, funding timelines
5. "timeline_signal" — prospect mentions deadlines, implementation timelines, urgency
6. "champion_signal" — internal advocate emerges, someone pushes for adoption, refers colleagues
7. "risk_signal" — signs of deal risk: going silent, mentioning organizational changes, deprioritization

For each detected signal, provide:
- source_type: the exact source_type from the input item containing the evidence
- source_id: the exact source_id from the input item containing the evidence
- signal_type: one of the 7 types above
- summary: concise 1-2 sentence description of the signal, written in English
- confidence: float 0.0-1.0 (0.9+ = very clear signal, 0.7-0.9 = likely signal, 0.5-0.7 = possible signal)
- raw_evidence: the specific text/quote that indicates this signal
- timeline_date: for timeline_signal only, the explicit calendar date normalized as YYYY-MM-DD; omit it when the evidence does not state a calendar date

Return a JSON array of detected signals. If no signals are detected, return an empty array [].
Only detect signals that are clearly present — avoid false positives. Be conservative with confidence scores.
Never infer a timeline_date from vague phrases such as "soon", "next quarter", or "in a few weeks".
Treat inbound customer language as primary evidence. Do not interpret the seller's outbound pitch, internal task state, or deal-stage movement as a buyer signal.
For calendar sources, attendee response status and event cancellation are factual evidence; do not treat the organizer's meeting title or description alone as buyer intent.
Write generated summaries in English even when the source evidence is in another language. Keep raw_evidence as an exact excerpt in its original language.

Example response:
[
  {"source_type": "email", "source_id": "source UUID from input", "signal_type": "buying_intent", "summary": "Prospect asked about enterprise pricing and requested a demo call", "confidence": 0.92, "raw_evidence": "Can you send me the pricing for your enterprise plan? We'd like to schedule a demo next week."},
  {"source_type": "email", "source_id": "source UUID from input", "signal_type": "budget_signal", "summary": "Budget approved for Q2 tooling purchase", "confidence": 0.85, "raw_evidence": "Our team has budget approved for Q2 to invest in a new project management tool."}
]`

func buildSignalMetadata(payload model.SignalSourcePayload) model.JSONB {
	content := signalEvidenceCorpus(payload)
	contentHash := sha256.Sum256([]byte(content))
	metadata := model.JSONB{
		"message_direction":   payload.Direction,
		"participant_count":   len(payload.Participants),
		"ingestion_version":   signalEvidenceDetectorVersion,
		"detector_version":    signalEvidenceDetectorVersion,
		"source_content_hash": fmt.Sprintf("%x", contentHash),
		"evidence_verified":   true,
	}
	if payload.SourceThreadExternalID != nil && *payload.SourceThreadExternalID != "" {
		metadata["thread_external_id"] = *payload.SourceThreadExternalID
	}
	return metadata
}

func verifiedSignalEvidence(payload model.SignalSourcePayload, rawEvidence string) (string, bool) {
	evidence := strings.TrimSpace(rawEvidence)
	if evidence == "" {
		return "", false
	}
	if len(evidence) > 500 {
		evidence = evidence[:500]
	}
	corpus := signalEvidenceCorpus(payload)
	normalizedEvidence := normalizedSignalText(evidence)
	return evidence, normalizedEvidence != "" && strings.Contains(corpus, normalizedEvidence)
}

func signalEvidenceCorpus(payload model.SignalSourcePayload) string {
	return normalizedSignalText(payload.Subject + " " + payload.Body + " " + payload.ThreadContext)
}

func normalizedSignalText(value string) string {
	return crmtext.Normalize(value)
}

func (s *SignalDetectionService) requestSummaryRefresh(ctx context.Context, signal model.CRMBuyerSignal) {
	if s == nil {
		return
	}
	if s.summaryRefresh != nil && signal.ContactID != nil && *signal.ContactID != "" {
		if err := s.summaryRefresh.RequestContactRefresh(ctx, signal.WorkspaceID, *signal.ContactID); err != nil {
			slog.ErrorContext(ctx, "failed to request contact summary refresh from detected signal", "error", err, "workspace_id", signal.WorkspaceID, "contact_id", *signal.ContactID, "signal_id", signal.ID)
		}
	}
	if signal.DealID != nil && *signal.DealID != "" {
		if s.summaryRefresh != nil {
			if err := s.summaryRefresh.RequestDealRefresh(ctx, signal.WorkspaceID, *signal.DealID); err != nil {
				slog.ErrorContext(ctx, "failed to request deal summary refresh from detected signal", "error", err, "workspace_id", signal.WorkspaceID, "deal_id", *signal.DealID, "signal_id", signal.ID)
			}
		}
		if s.healthScoreRefresh != nil {
			if _, err := s.healthScoreRefresh.RefreshDealHealthScore(ctx, signal.WorkspaceID, *signal.DealID); err != nil {
				slog.ErrorContext(ctx, "failed to refresh deal health from detected signal", "error", err, "workspace_id", signal.WorkspaceID, "deal_id", *signal.DealID, "signal_id", signal.ID)
			}
		}
	}
	companyRefresh, canRefreshCompany := s.summaryRefresh.(CompanySummaryRefreshRequester)
	if !canRefreshCompany {
		return
	}
	if signal.ContactID != nil && *signal.ContactID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectContact, *signal.ContactID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from detected contact signal", "error", err, "contact_id", *signal.ContactID)
		}
	}
	if signal.DealID != nil && *signal.DealID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectDeal, *signal.DealID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from detected deal signal", "error", err, "deal_id", *signal.DealID)
		}
	}
	if signal.CompanyID != nil && *signal.CompanyID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectCompany, *signal.CompanyID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from detected company signal", "error", err, "company_id", *signal.CompanyID)
		}
	}
}
