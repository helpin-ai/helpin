package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSignalService contains CRM signal and deal health business logic.
type CRMSignalService struct {
	signalRepo     *repository.CRMSignalRepository
	summaryRefresh interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	}
}

// NewCRMSignalService creates a new CRMSignalService.
func NewCRMSignalService(signalRepo *repository.CRMSignalRepository, summaryRefresh interface {
	RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
	RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
}) *CRMSignalService {
	return &CRMSignalService{signalRepo: signalRepo, summaryRefresh: summaryRefresh}
}

// ── Buyer Signals ──

// ListSignals returns buyer signals with filters and pagination.
func (s *CRMSignalService) ListSignals(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination) ([]model.CRMBuyerSignal, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.ListSignals(ctx, workspaceID, filters, pagination)
}

// CreateSignal creates a new buyer signal.
func (s *CRMSignalService) CreateSignal(ctx context.Context, req model.CreateCRMBuyerSignalRequest) (*model.CRMBuyerSignal, error) {
	if req.WorkspaceID == "" || req.SignalType == "" || req.Summary == "" {
		return nil, fmt.Errorf("workspace_id, signal_type, and summary are required")
	}

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}

	sourceType := model.CRMSignalSourceManual
	if req.SourceType != "" {
		sourceType = req.SourceType
	}

	signal := &model.CRMBuyerSignal{
		WorkspaceID:     req.WorkspaceID,
		ContactID:       req.ContactID,
		DealID:          req.DealID,
		SignalType:      req.SignalType,
		SourceType:      sourceType,
		SourceID:        req.SourceID,
		SourceThreadID:  req.SourceThreadID,
		Summary:         req.Summary,
		EvidenceExcerpt: req.EvidenceExcerpt,
		Metadata:        model.JSONB(req.Metadata),
		Confidence:      confidence,
		DetectedAt:      time.Now(),
	}

	if err := s.signalRepo.CreateSignal(ctx, signal); err != nil {
		return nil, err
	}
	s.requestSummaryRefresh(ctx, signal)
	return signal, nil
}

// DeleteSignal removes a buyer signal.
func (s *CRMSignalService) DeleteSignal(ctx context.Context, id string) error {
	return s.signalRepo.DeleteSignal(ctx, id)
}

// ── Deal Health Scores ──

// GetLatestHealthScore returns the latest health score for a deal.
func (s *CRMSignalService) GetLatestHealthScore(ctx context.Context, dealID string) (*model.CRMDealHealthScore, error) {
	if dealID == "" {
		return nil, fmt.Errorf("deal_id is required")
	}
	score, err := s.signalRepo.GetLatestHealthScore(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if score == nil {
		return nil, fmt.Errorf("no health score found for deal")
	}
	return score, nil
}

// ListHealthScores returns health scores for a workspace.
func (s *CRMSignalService) ListHealthScores(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMDealHealthScore, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.signalRepo.ListHealthScores(ctx, workspaceID, pagination)
}

// CreateHealthScore creates a new deal health score.
func (s *CRMSignalService) CreateHealthScore(ctx context.Context, req model.CreateCRMDealHealthScoreRequest) (*model.CRMDealHealthScore, error) {
	if req.WorkspaceID == "" || req.DealID == "" {
		return nil, fmt.Errorf("workspace_id and deal_id are required")
	}
	if req.Score < 0 || req.Score > 100 {
		return nil, fmt.Errorf("score must be between 0 and 100")
	}

	score := &model.CRMDealHealthScore{
		WorkspaceID:  req.WorkspaceID,
		DealID:       req.DealID,
		Score:        req.Score,
		Factors:      model.JSONB(req.Factors),
		CalculatedAt: time.Now(),
	}

	if err := s.signalRepo.CreateHealthScore(ctx, score); err != nil {
		return nil, err
	}
	return score, nil
}

func (s *CRMSignalService) requestSummaryRefresh(ctx context.Context, signal *model.CRMBuyerSignal) {
	if s == nil || s.summaryRefresh == nil || signal == nil {
		return
	}
	if signal.ContactID != nil && *signal.ContactID != "" {
		if err := s.summaryRefresh.RequestContactRefresh(ctx, signal.WorkspaceID, *signal.ContactID); err != nil {
			slog.ErrorContext(ctx, "failed to request contact summary refresh from manual crm signal", "error", err, "workspace_id", signal.WorkspaceID, "contact_id", *signal.ContactID, "signal_id", signal.ID)
		}
	}
	if signal.DealID != nil && *signal.DealID != "" {
		if err := s.summaryRefresh.RequestDealRefresh(ctx, signal.WorkspaceID, *signal.DealID); err != nil {
			slog.ErrorContext(ctx, "failed to request deal summary refresh from manual crm signal", "error", err, "workspace_id", signal.WorkspaceID, "deal_id", *signal.DealID, "signal_id", signal.ID)
		}
	}
}
