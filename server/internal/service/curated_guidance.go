package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	ErrCuratedGuidanceNotFound = errors.New("curated guidance not found")
	ErrCuratedGuidanceInvalid  = errors.New("invalid curated guidance")
)

// CuratedGuidanceService manages first-class pinned support answers.
type CuratedGuidanceService struct {
	repo           *repository.CuratedGuidanceRepository
	agentRepo      *repository.AgentRepository
	embedder       llm.EmbeddingProvider
	embeddingModel string
}

func NewCuratedGuidanceService(
	repo *repository.CuratedGuidanceRepository,
	agentRepo *repository.AgentRepository,
	embedder llm.EmbeddingProvider,
	embeddingModel string,
) *CuratedGuidanceService {
	if strings.TrimSpace(embeddingModel) == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	return &CuratedGuidanceService{
		repo:           repo,
		agentRepo:      agentRepo,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
	}
}

func (s *CuratedGuidanceService) List(ctx context.Context, workspaceID, agentID string) ([]model.CuratedGuidance, error) {
	if err := s.validateAgent(ctx, workspaceID, agentID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, workspaceID, agentID)
}

func (s *CuratedGuidanceService) Create(
	ctx context.Context,
	workspaceID string,
	agentID string,
	createdByID string,
	req model.CreateCuratedGuidanceRequest,
) (*model.CuratedGuidance, error) {
	if err := s.validateAgent(ctx, workspaceID, agentID); err != nil {
		return nil, err
	}
	language := normalizeSupportLanguage(req.Language)
	if strings.TrimSpace(req.Language) != "" && language == "" {
		return nil, fmt.Errorf("%w: language must be a BCP-47 tag such as en or en-us", ErrCuratedGuidanceInvalid)
	}
	item := &model.CuratedGuidance{
		WorkspaceID:      workspaceID,
		AgentID:          agentID,
		Title:            strings.TrimSpace(req.Title),
		QuestionPatterns: cleanGuidanceStrings(req.QuestionPatterns),
		Answer:           strings.TrimSpace(req.Answer),
		Intent:           supportIntentDefinition(req.Intent).ID,
		Topics:           cleanGuidanceStrings(req.Topics),
		Language:         language,
		AudiencePolicyID: req.AudiencePolicyID,
		BrandID:          req.BrandID,
		Status:           model.CuratedGuidanceStatusActive,
		ValidFrom:        req.ValidFrom,
		ValidUntil:       req.ValidUntil,
		CreatedByID:      strings.TrimSpace(createdByID),
	}
	if item.CreatedByID == "" {
		return nil, fmt.Errorf("%w: created_by_id is required", ErrCuratedGuidanceInvalid)
	}
	if err := validateCuratedGuidance(item); err != nil {
		return nil, err
	}
	if err := s.embed(ctx, item); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CuratedGuidanceService) Update(
	ctx context.Context,
	workspaceID string,
	agentID string,
	id string,
	req model.UpdateCuratedGuidanceRequest,
) (*model.CuratedGuidance, error) {
	if err := s.validateAgent(ctx, workspaceID, agentID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetByID(ctx, workspaceID, agentID, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCuratedGuidanceNotFound
	}

	reembed := false
	if req.Title != nil {
		item.Title = strings.TrimSpace(*req.Title)
		reembed = true
	}
	if req.QuestionPatterns != nil {
		item.QuestionPatterns = cleanGuidanceStrings(req.QuestionPatterns)
		reembed = true
	}
	if req.Answer != nil {
		item.Answer = strings.TrimSpace(*req.Answer)
		reembed = true
	}
	if req.Intent != nil {
		item.Intent = supportIntentDefinition(*req.Intent).ID
	}
	if req.Topics != nil {
		item.Topics = cleanGuidanceStrings(req.Topics)
		reembed = true
	}
	if req.Language != nil {
		language := normalizeSupportLanguage(*req.Language)
		if strings.TrimSpace(*req.Language) != "" && language == "" {
			return nil, fmt.Errorf("%w: language must be a BCP-47 tag such as en or en-us", ErrCuratedGuidanceInvalid)
		}
		item.Language = language
	}
	if req.AudiencePolicyID != nil {
		item.AudiencePolicyID = normalizeOptionalString(req.AudiencePolicyID)
	}
	if req.BrandID != nil {
		item.BrandID = normalizeOptionalString(req.BrandID)
	}
	if req.Status != nil {
		item.Status = strings.ToLower(strings.TrimSpace(*req.Status))
	}
	if req.ValidFrom != nil {
		item.ValidFrom = req.ValidFrom
	}
	if req.ValidUntil != nil {
		item.ValidUntil = req.ValidUntil
	}
	if err := validateCuratedGuidance(item); err != nil {
		return nil, err
	}
	if reembed {
		if err := s.embed(ctx, item); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CuratedGuidanceService) Delete(ctx context.Context, workspaceID, agentID, id string) error {
	if err := s.validateAgent(ctx, workspaceID, agentID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, workspaceID, agentID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCuratedGuidanceNotFound
		}
		return err
	}
	return nil
}

func (s *CuratedGuidanceService) validateAgent(ctx context.Context, workspaceID, agentID string) error {
	if s == nil || s.repo == nil || s.agentRepo == nil {
		return fmt.Errorf("curated guidance service is not configured")
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return fmt.Errorf("get curated guidance agent: %w", err)
	}
	if agent == nil {
		return fmt.Errorf("%w: agent not found", ErrCuratedGuidanceInvalid)
	}
	return nil
}

func (s *CuratedGuidanceService) embed(ctx context.Context, item *model.CuratedGuidance) error {
	if !embeddingsAvailable(ctx, s.embedder, item.WorkspaceID) {
		item.Embedding = nil
		item.EmbeddingProvider = ""
		item.EmbeddingModel = ""
		item.EmbeddingVersion = ""
		item.EmbeddingDimensions = 0
		return nil
	}
	input := strings.Join(append([]string{item.Title}, append([]string(item.QuestionPatterns), item.Answer)...), "\n")
	embedCtx := withAIActionMetering(ctx, item.WorkspaceID, aipolicy.ActionCuratedGuidanceEmbed, "curated_guidance_embed", input, map[string]interface{}{
		"surface": "curated_guidance", "guidance_id": item.ID,
	})
	response, err := s.embedder.CreateEmbeddings(embedCtx, llm.EmbeddingRequest{
		Provider: "openai",
		Model:    s.embeddingModel,
		Inputs:   []string{input},
	})
	if err != nil {
		return fmt.Errorf("embed curated guidance: %w", err)
	}
	if len(response.Vectors) != 1 || len(response.Vectors[0]) != docsEmbeddingDimensions {
		return fmt.Errorf("embed curated guidance: expected one %d-dimensional vector", docsEmbeddingDimensions)
	}
	formatted := formatVector(response.Vectors[0])
	item.Embedding = &formatted
	item.EmbeddingProvider = "openai"
	item.EmbeddingModel = s.embeddingModel
	item.EmbeddingVersion = contentChunkEmbeddingVersion
	item.EmbeddingDimensions = docsEmbeddingDimensions
	return nil
}

func validateCuratedGuidance(item *model.CuratedGuidance) error {
	if strings.TrimSpace(item.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrCuratedGuidanceInvalid)
	}
	if strings.TrimSpace(item.Answer) == "" {
		return fmt.Errorf("%w: answer is required", ErrCuratedGuidanceInvalid)
	}
	if item.Status != model.CuratedGuidanceStatusActive && item.Status != model.CuratedGuidanceStatusDisabled {
		return fmt.Errorf("%w: status must be active or disabled", ErrCuratedGuidanceInvalid)
	}
	if item.ValidFrom != nil && item.ValidUntil != nil && !item.ValidUntil.After(*item.ValidFrom) {
		return fmt.Errorf("%w: valid_until must be after valid_from", ErrCuratedGuidanceInvalid)
	}
	return nil
}

func cleanGuidanceStrings(values []string) model.DocsStringArray {
	seen := map[string]struct{}{}
	cleaned := make(model.DocsStringArray, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
