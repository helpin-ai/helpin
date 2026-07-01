package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const repeatedThreadSignalWindow = 24 * time.Hour

// SignalDetectionService uses LLM to detect buyer signals from various sources.
type SignalDetectionService struct {
	llmProvider    llm.Provider
	signalRepo     *repository.CRMSignalRepository
	summaryRefresh interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
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

// DetectedSignal is the parsed LLM output for a single signal.
type DetectedSignal struct {
	SignalType  string  `json:"signal_type"`
	Summary     string  `json:"summary"`
	Confidence  float64 `json:"confidence"`
	RawEvidence string  `json:"raw_evidence"`
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

	payload := payloads[0]
	ctx = WithAIUsageMetering(ctx, AIUsageMeteringContext{
		WorkspaceID:    payload.WorkspaceID,
		FeatureKey:     BillingFeatureCRMSignalDetection,
		IdempotencyKey: aiUsageIdempotencyKey(payload.WorkspaceID, BillingFeatureCRMSignalDetection, payload.SourceType, payload.SourceID, derefString(payload.SourceThreadID)),
		Metadata: map[string]interface{}{
			"source_type": payload.SourceType,
			"source_id":   payload.SourceID,
			"thread_id":   derefString(payload.SourceThreadID),
		},
	})
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: signalDetectionSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: string(payloadJSON)},
		},
		Temperature: 0.1,
		MaxTokens:   4096,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM signal detection: %w", err)
	}

	// Parse response
	var detected []DetectedSignal
	if err := llm.UnmarshalResponse(resp.Content, &detected); err != nil {
		// Try wrapping in case response has a wrapper object
		var wrapper struct {
			Signals []DetectedSignal `json:"signals"`
		}
		if err2 := llm.UnmarshalResponse(resp.Content, &wrapper); err2 != nil {
			slog.Error("failed to parse signal detection response", "error", err, "content", resp.Content)
			return nil, fmt.Errorf("parse detection response: %w", err)
		}
		detected = wrapper.Signals
	}

	// Create buyer signal records.
	var signals []model.CRMBuyerSignal
	for _, d := range detected {
		if d.Confidence < 0.3 {
			continue // Skip very low confidence
		}

		if payload.SourceThreadID != nil && *payload.SourceThreadID != "" {
			exists, err := s.signalRepo.HasRecentSignalForThread(ctx, payload.WorkspaceID, *payload.SourceThreadID, d.SignalType, time.Now().Add(-repeatedThreadSignalWindow))
			if err != nil {
				return nil, err
			}
			if exists {
				slog.Info("skipping repeated thread-level buyer signal", "workspace_id", payload.WorkspaceID, "thread_id", *payload.SourceThreadID, "signal_type", d.SignalType)
				continue
			}
		}

		signal := model.CRMBuyerSignal{
			WorkspaceID:     payload.WorkspaceID,
			ContactID:       payload.ContactID,
			DealID:          payload.DealID,
			SignalType:      d.SignalType,
			SourceType:      payload.SourceType,
			SourceID:        &payload.SourceID,
			SourceThreadID:  payload.SourceThreadID,
			Summary:         d.Summary,
			EvidenceExcerpt: optionalExcerpt(d.RawEvidence),
			Metadata:        buildSignalMetadata(payload),
			Confidence:      d.Confidence,
			DetectedAt:      time.Now(),
		}

		created, err := s.signalRepo.CreateSignalIfAbsent(ctx, &signal)
		if err != nil {
			slog.Error("failed to store detected signal", "error", err, "signal_type", d.SignalType)
			continue
		}
		if !created {
			continue
		}

		s.requestSummaryRefresh(ctx, signal)

		signals = append(signals, signal)
	}

	slog.Info("signal detection complete", "payloads", len(payloads), "signals_detected", len(signals))
	return signals, nil
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
- signal_type: one of the 7 types above
- summary: concise 1-2 sentence description of the signal
- confidence: float 0.0-1.0 (0.9+ = very clear signal, 0.7-0.9 = likely signal, 0.5-0.7 = possible signal)
- raw_evidence: the specific text/quote that indicates this signal

Return a JSON array of detected signals. If no signals are detected, return an empty array [].
Only detect signals that are clearly present — avoid false positives. Be conservative with confidence scores.

Example response:
[
  {"signal_type": "buying_intent", "summary": "Prospect asked about enterprise pricing and requested a demo call", "confidence": 0.92, "raw_evidence": "Can you send me the pricing for your enterprise plan? We'd like to schedule a demo next week."},
  {"signal_type": "budget_signal", "summary": "Budget approved for Q2 tooling purchase", "confidence": 0.85, "raw_evidence": "Our team has budget approved for Q2 to invest in a new project management tool."}
]`

func optionalExcerpt(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 500 {
		value = value[:500]
	}
	return &value
}

func buildSignalMetadata(payload model.SignalSourcePayload) model.JSONB {
	metadata := model.JSONB{
		"message_direction": payload.Direction,
		"participant_count": len(payload.Participants),
		"ingestion_version": "phase1a",
	}
	if payload.SourceThreadExternalID != nil && *payload.SourceThreadExternalID != "" {
		metadata["thread_external_id"] = *payload.SourceThreadExternalID
	}
	return metadata
}

func (s *SignalDetectionService) requestSummaryRefresh(ctx context.Context, signal model.CRMBuyerSignal) {
	if s == nil || s.summaryRefresh == nil {
		return
	}
	if signal.ContactID != nil && *signal.ContactID != "" {
		if err := s.summaryRefresh.RequestContactRefresh(ctx, signal.WorkspaceID, *signal.ContactID); err != nil {
			slog.ErrorContext(ctx, "failed to request contact summary refresh from detected signal", "error", err, "workspace_id", signal.WorkspaceID, "contact_id", *signal.ContactID, "signal_id", signal.ID)
		}
	}
	if signal.DealID != nil && *signal.DealID != "" {
		if err := s.summaryRefresh.RequestDealRefresh(ctx, signal.WorkspaceID, *signal.DealID); err != nil {
			slog.ErrorContext(ctx, "failed to request deal summary refresh from detected signal", "error", err, "workspace_id", signal.WorkspaceID, "deal_id", *signal.DealID, "signal_id", signal.ID)
		}
	}
}
