package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DealAutomationService handles autonomous deal creation and progression.
type DealAutomationService struct {
	llmProvider  llm.Provider
	dealRepo     *repository.CRMDealRepository
	signalRepo   *repository.CRMSignalRepository
	suggestionRepo *repository.CRMSuggestionRepository
	contactRepo  *repository.CRMContactRepository
	assocRepo    *repository.CRMAssociationRepository
	autonomyRepo *repository.CRMAutonomyRepository
}

// NewDealAutomationService creates a new deal automation service.
func NewDealAutomationService(
	llmProvider llm.Provider,
	dealRepo *repository.CRMDealRepository,
	signalRepo *repository.CRMSignalRepository,
	suggestionRepo *repository.CRMSuggestionRepository,
	contactRepo *repository.CRMContactRepository,
	assocRepo *repository.CRMAssociationRepository,
	autonomyRepo *repository.CRMAutonomyRepository,
) *DealAutomationService {
	return &DealAutomationService{
		llmProvider:    llmProvider,
		dealRepo:       dealRepo,
		signalRepo:     signalRepo,
		suggestionRepo: suggestionRepo,
		contactRepo:    contactRepo,
		assocRepo:      assocRepo,
		autonomyRepo:   autonomyRepo,
	}
}

// GetAutonomySettings retrieves CRM autonomy settings for a workspace.
func (s *DealAutomationService) GetAutonomySettings(ctx context.Context, workspaceID string) model.CRMAutonomySettings {
	settings, err := s.autonomyRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil || settings == nil {
		return model.DefaultAutonomySettings()
	}
	return *settings
}

// UpdateAutonomySettings saves CRM autonomy settings.
func (s *DealAutomationService) UpdateAutonomySettings(ctx context.Context, workspaceID string, req model.CRMAutonomySettings) error {
	req.WorkspaceID = workspaceID
	return s.autonomyRepo.Upsert(ctx, &req)
}

// DealCreationInference is the LLM output for deal creation.
type DealCreationInference struct {
	ShouldCreate    bool     `json:"should_create"`
	DealName        string   `json:"deal_name"`
	EstimatedAmount *float64 `json:"estimated_amount"`
	SuggestedStage  string   `json:"suggested_stage"`
	Confidence      float64  `json:"confidence"`
	Reasoning       string   `json:"reasoning"`
}

// EvaluateDealCreation checks if signals warrant creating a new deal.
func (s *DealAutomationService) EvaluateDealCreation(ctx context.Context, workspaceID string, signals []model.CRMBuyerSignal) error {
	if s.llmProvider == nil || len(signals) == 0 {
		return nil
	}

	autonomy := s.GetAutonomySettings(ctx, workspaceID)
	if !autonomy.Enabled || !autonomy.AutoCreateDeals {
		return nil
	}

	// Group signals by contact
	contactSignals := map[string][]model.CRMBuyerSignal{}
	for _, sig := range signals {
		key := ""
		if sig.ContactID != nil {
			key = *sig.ContactID
		}
		contactSignals[key] = append(contactSignals[key], sig)
	}

	for contactID, sigs := range contactSignals {
		if contactID == "" {
			continue
		}

		// Check if buying intent or budget signals exist
		hasBuyingSignal := false
		for _, sig := range sigs {
			if sig.SignalType == model.CRMSignalBuyingIntent || sig.SignalType == model.CRMSignalBudgetSignal {
				hasBuyingSignal = true
				break
			}
		}
		if !hasBuyingSignal {
			continue
		}

		// Check if deal already exists for this contact
		existingDeals := s.getContactDeals(ctx, workspaceID, contactID)
		if len(existingDeals) > 0 {
			continue // Already has active deals
		}

		// Get contact info
		contact, err := s.contactRepo.GetByID(ctx, contactID)
		if err != nil || contact == nil {
			continue
		}

		// Call LLM for deal inference
		inference, err := s.inferDealCreation(ctx, contact, sigs)
		if err != nil {
			slog.Error("deal creation inference failed", "error", err, "contact_id", contactID)
			continue
		}

		if !inference.ShouldCreate {
			continue
		}

		// Get default pipeline
		pipelines, err := s.dealRepo.ListPipelines(ctx, workspaceID)
		if err != nil || len(pipelines) == 0 {
			continue
		}
		var defaultPipeline *model.CRMPipeline
		for i := range pipelines {
			if pipelines[i].IsDefault {
				defaultPipeline = &pipelines[i]
				break
			}
		}
		if defaultPipeline == nil {
			defaultPipeline = &pipelines[0]
		}

		// Get first open stage
		var firstStage *model.CRMPipelineStage
		if len(defaultPipeline.Stages) > 0 {
			for i := range defaultPipeline.Stages {
				if defaultPipeline.Stages[i].StageType == model.CRMStageTypeOpen {
					firstStage = &defaultPipeline.Stages[i]
					break
				}
			}
			if firstStage == nil {
				firstStage = &defaultPipeline.Stages[0]
			}
		}
		if firstStage == nil {
			continue
		}

		// Build suggestion context
		dealContext := map[string]interface{}{
			"deal_name":   inference.DealName,
			"pipeline_id": defaultPipeline.ID,
			"stage_id":    firstStage.ID,
			"contact_id":  contactID,
			"contact_name": contact.FirstName,
			"reasoning":   inference.Reasoning,
		}
		if inference.EstimatedAmount != nil {
			dealContext["amount"] = *inference.EstimatedAmount
		}
		if contact.LastName != nil {
			dealContext["contact_name"] = contact.FirstName + " " + *contact.LastName
		}

		// Apply autonomy thresholds
		if inference.Confidence >= autonomy.AutoExecuteThreshold {
			// Auto-create deal
			deal, err := s.autoCreateDeal(ctx, workspaceID, contactID, defaultPipeline.ID, firstStage.ID, *inference)
			if err != nil {
				slog.Error("auto-create deal failed", "error", err, "contact_id", contactID)
				continue
			}

			// Create accepted suggestion for audit trail
			objectType := "deal"
			suggestion := &model.CRMSuggestion{
				WorkspaceID:    workspaceID,
				SuggestionType: model.CRMSuggestionDealCreate,
				ObjectType:     &objectType,
				ObjectID:       &deal.ID,
				Title:          "Auto-created deal: " + inference.DealName,
				Description:    &inference.Reasoning,
				Context:        model.JSONB(dealContext),
				Status:         model.CRMSuggestionStatusAccepted,
				Confidence:     inference.Confidence,
			}
			if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
				slog.Error("failed to create suggestion audit trail", "error", err)
			}
			slog.Info("auto-created deal", "deal_id", deal.ID, "contact_id", contactID, "confidence", inference.Confidence)
		} else if inference.Confidence >= autonomy.ReviewThreshold {
			// Create pending suggestion for review
			description := inference.Reasoning
			suggestion := &model.CRMSuggestion{
				WorkspaceID:    workspaceID,
				SuggestionType: model.CRMSuggestionDealCreate,
				Title:          "Suggested deal: " + inference.DealName,
				Description:    &description,
				Context:        model.JSONB(dealContext),
				Status:         model.CRMSuggestionStatusPending,
				Confidence:     inference.Confidence,
			}
			if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
				slog.Error("failed to create deal suggestion", "error", err)
			}
			slog.Info("created deal suggestion", "contact_id", contactID, "confidence", inference.Confidence)
		}
	}

	return nil
}

// EvaluateDealProgression checks if active deals should advance stages.
func (s *DealAutomationService) EvaluateDealProgression(ctx context.Context, workspaceID string) error {
	if s.llmProvider == nil {
		return nil
	}

	autonomy := s.GetAutonomySettings(ctx, workspaceID)
	if !autonomy.Enabled || !autonomy.AutoProgressDeals {
		return nil
	}

	// Get active deals (not won/lost)
	deals, _, err := s.dealRepo.List(ctx, workspaceID, model.CRMDealListFilters{}, model.PMPagination{Page: 1, PerPage: 100})
	if err != nil {
		return fmt.Errorf("list deals: %w", err)
	}

	for _, deal := range deals {
		// Skip won/lost deals
		if deal.Stage != nil && (deal.Stage.StageType == model.CRMStageTypeWon || deal.Stage.StageType == model.CRMStageTypeLost) {
			continue
		}

		// Get recent signals for this deal (last 14 days)
		since := time.Now().AddDate(0, 0, -14)
		dealID := deal.ID
		signals, _, err := s.signalRepo.ListSignals(ctx, workspaceID, model.CRMBuyerSignalListFilters{
			DealID: &dealID,
		}, model.PMPagination{Page: 1, PerPage: 50})
		if err != nil {
			continue
		}

		// Filter to recent signals
		var recentSignals []model.CRMBuyerSignal
		for _, sig := range signals {
			if sig.DetectedAt.After(since) {
				recentSignals = append(recentSignals, sig)
			}
		}

		if len(recentSignals) == 0 {
			continue
		}

		// Infer progression
		inference, err := s.inferDealProgression(ctx, &deal, recentSignals)
		if err != nil {
			slog.Error("deal progression inference failed", "error", err, "deal_id", deal.ID)
			continue
		}

		if !inference.ShouldAdvance {
			continue
		}

		// Find the recommended stage
		var targetStage *model.CRMPipelineStage
		if deal.Pipeline != nil {
			for i := range deal.Pipeline.Stages {
				if deal.Pipeline.Stages[i].Name == inference.RecommendedStage {
					targetStage = &deal.Pipeline.Stages[i]
					break
				}
			}
		}

		progressionContext := map[string]interface{}{
			"deal_id":           deal.ID,
			"deal_name":         deal.Name,
			"current_stage":     "",
			"recommended_stage": inference.RecommendedStage,
			"reasoning":         inference.Reasoning,
		}
		if deal.Stage != nil {
			progressionContext["current_stage"] = deal.Stage.Name
			progressionContext["current_stage_id"] = deal.Stage.ID
		}
		if targetStage != nil {
			progressionContext["target_stage_id"] = targetStage.ID
		}

		objectType := "deal"
		if inference.Confidence >= autonomy.AutoExecuteThreshold && targetStage != nil {
			// Auto-advance
			deal.StageID = targetStage.ID
			deal.Pipeline = nil
			deal.Stage = nil
			if err := s.dealRepo.Update(ctx, &deal); err != nil {
				slog.Error("auto-advance deal failed", "error", err, "deal_id", deal.ID)
				continue
			}

			suggestion := &model.CRMSuggestion{
				WorkspaceID:    workspaceID,
				SuggestionType: model.CRMSuggestionDealAdvance,
				ObjectType:     &objectType,
				ObjectID:       &deal.ID,
				Title:          fmt.Sprintf("Auto-advanced '%s' to %s", deal.Name, inference.RecommendedStage),
				Description:    &inference.Reasoning,
				Context:        model.JSONB(progressionContext),
				Status:         model.CRMSuggestionStatusAccepted,
				Confidence:     inference.Confidence,
			}
			if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
				slog.Error("failed to create progression audit trail", "error", err)
			}
			slog.Info("auto-advanced deal", "deal_id", deal.ID, "new_stage", inference.RecommendedStage)
		} else if inference.Confidence >= autonomy.ReviewThreshold {
			suggestion := &model.CRMSuggestion{
				WorkspaceID:    workspaceID,
				SuggestionType: model.CRMSuggestionDealAdvance,
				ObjectType:     &objectType,
				ObjectID:       &deal.ID,
				Title:          fmt.Sprintf("Advance '%s' to %s", deal.Name, inference.RecommendedStage),
				Description:    &inference.Reasoning,
				Context:        model.JSONB(progressionContext),
				Status:         model.CRMSuggestionStatusPending,
				Confidence:     inference.Confidence,
			}
			if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
				slog.Error("failed to create progression suggestion", "error", err)
			}
		}
	}

	return nil
}

func (s *DealAutomationService) getContactDeals(ctx context.Context, workspaceID, contactID string) []model.CRMDeal {
	assocs, _ := s.assocRepo.ListByObject(ctx, workspaceID, "contact", contactID)
	var deals []model.CRMDeal
	for _, a := range assocs {
		if a.FromObjectType == "deal" {
			deal, err := s.dealRepo.GetByID(ctx, a.FromObjectID)
			if err == nil && deal != nil {
				if deal.Stage != nil && (deal.Stage.StageType == model.CRMStageTypeWon || deal.Stage.StageType == model.CRMStageTypeLost) {
					continue
				}
				deals = append(deals, *deal)
			}
		}
		if a.ToObjectType == "deal" {
			deal, err := s.dealRepo.GetByID(ctx, a.ToObjectID)
			if err == nil && deal != nil {
				if deal.Stage != nil && (deal.Stage.StageType == model.CRMStageTypeWon || deal.Stage.StageType == model.CRMStageTypeLost) {
					continue
				}
				deals = append(deals, *deal)
			}
		}
	}
	return deals
}

func (s *DealAutomationService) autoCreateDeal(ctx context.Context, workspaceID, contactID, pipelineID, stageID string, inference DealCreationInference) (*model.CRMDeal, error) {
	displayID, err := s.dealRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	deal := &model.CRMDeal{
		WorkspaceID: workspaceID,
		DisplayID:   displayID,
		Name:        inference.DealName,
		PipelineID:  pipelineID,
		StageID:     stageID,
		Amount:      inference.EstimatedAmount,
		Currency:    "USD",
	}

	if err := s.dealRepo.Create(ctx, deal); err != nil {
		return nil, err
	}

	// Create association
	assoc := &model.CRMAssociation{
		WorkspaceID:    workspaceID,
		FromObjectType: "deal",
		FromObjectID:   deal.ID,
		ToObjectType:   "contact",
		ToObjectID:     contactID,
	}
	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		slog.Error("failed to create deal-contact association", "error", err, "deal_id", deal.ID, "contact_id", contactID)
	}

	return deal, nil
}

// DealProgressionInference is the LLM output for stage progression.
type DealProgressionInference struct {
	ShouldAdvance    bool    `json:"should_advance"`
	RecommendedStage string  `json:"recommended_stage"`
	Confidence       float64 `json:"confidence"`
	Reasoning        string  `json:"reasoning"`
}

func (s *DealAutomationService) inferDealCreation(ctx context.Context, contact *model.CRMContact, signals []model.CRMBuyerSignal) (*DealCreationInference, error) {
	signalSummaries := make([]map[string]interface{}, 0, len(signals))
	for _, sig := range signals {
		signalSummaries = append(signalSummaries, map[string]interface{}{
			"type":       sig.SignalType,
			"summary":    sig.Summary,
			"confidence": sig.Confidence,
			"source":     sig.SourceType,
			"detected":   sig.DetectedAt.Format(time.RFC3339),
		})
	}

	contactName := contact.FirstName
	if contact.LastName != nil {
		contactName += " " + *contact.LastName
	}
	contactInfo := map[string]interface{}{
		"name":            contactName,
		"lifecycle_stage": contact.LifecycleStage,
	}
	if contact.Email != nil {
		contactInfo["email"] = *contact.Email
	}
	if contact.JobTitle != nil {
		contactInfo["job_title"] = *contact.JobTitle
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"contact": contactInfo,
		"signals": signalSummaries,
	})

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: dealCreationSystemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Temperature:  0.1,
		MaxTokens:    2048,
		JSONMode:     true,
	})
	if err != nil {
		return nil, err
	}

	var inference DealCreationInference
	if err := json.Unmarshal([]byte(resp.Content), &inference); err != nil {
		return nil, fmt.Errorf("parse deal creation inference: %w", err)
	}
	return &inference, nil
}

func (s *DealAutomationService) inferDealProgression(ctx context.Context, deal *model.CRMDeal, signals []model.CRMBuyerSignal) (*DealProgressionInference, error) {
	signalSummaries := make([]map[string]interface{}, 0, len(signals))
	for _, sig := range signals {
		signalSummaries = append(signalSummaries, map[string]interface{}{
			"type":       sig.SignalType,
			"summary":    sig.Summary,
			"confidence": sig.Confidence,
		})
	}

	dealInfo := map[string]interface{}{
		"name": deal.Name,
	}
	if deal.Stage != nil {
		dealInfo["current_stage"] = deal.Stage.Name
	}
	if deal.Pipeline != nil {
		stages := make([]string, 0)
		for _, stg := range deal.Pipeline.Stages {
			stages = append(stages, stg.Name+" ("+stg.StageType+")")
		}
		dealInfo["pipeline_stages"] = stages
	}
	if deal.Amount != nil {
		dealInfo["amount"] = *deal.Amount
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"deal":    dealInfo,
		"signals": signalSummaries,
	})

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: dealProgressionSystemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Temperature:  0.1,
		MaxTokens:    2048,
		JSONMode:     true,
	})
	if err != nil {
		return nil, err
	}

	var inference DealProgressionInference
	if err := json.Unmarshal([]byte(resp.Content), &inference); err != nil {
		return nil, fmt.Errorf("parse deal progression inference: %w", err)
	}
	return &inference, nil
}

const dealCreationSystemPrompt = `You are a sales intelligence analyst. Based on detected buyer signals and contact information, determine if a new sales deal should be created.

Analyze the signals and provide a JSON response:
{
  "should_create": true/false,
  "deal_name": "suggested deal name (e.g., 'Acme Corp - Enterprise Plan')",
  "estimated_amount": null or number (only if budget signals give a clear indication),
  "suggested_stage": "first pipeline stage name",
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation of why this deal should/shouldn't be created"
}

Create a deal when there are clear buying signals (pricing inquiries, demo requests, budget discussions).
Do NOT create deals for general inquiries, support questions, or low-intent interactions.
Be conservative - it's better to miss a deal than create a false one.`

const dealProgressionSystemPrompt = `You are a sales intelligence analyst. Based on a deal's current stage, pipeline stages, and recent buyer signals, determine if the deal should advance to the next stage.

Analyze and provide a JSON response:
{
  "should_advance": true/false,
  "recommended_stage": "stage name to advance to",
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation"
}

Only recommend advancement when signals clearly indicate progress:
- Budget approval → advance past qualification
- Demo completed + positive feedback → advance past demo stage
- Contract discussions → advance to negotiation
- Timeline urgency → may justify faster progression

Do NOT recommend advancement for:
- Single signal in isolation
- Risk signals or objections without resolution
- Competitor mentions without resolution`
