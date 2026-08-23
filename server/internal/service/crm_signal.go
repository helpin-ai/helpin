package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSignalService contains CRM signal and deal health business logic.
type CRMSignalService struct {
	signalRepo     *repository.CRMSignalRepository
	dealRepo       *repository.CRMDealRepository
	summaryRefresh interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	}
}

const dealHealthSignalWindow = 180 * 24 * time.Hour

// SetHealthScoreDependencies enables deterministic deal-health production.
func (s *CRMSignalService) SetHealthScoreDependencies(dealRepo *repository.CRMDealRepository) *CRMSignalService {
	s.dealRepo = dealRepo
	return s
}

// NewCRMSignalService creates a new CRMSignalService.
func NewCRMSignalService(signalRepo *repository.CRMSignalRepository, summaryRefresh interface {
	RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
	RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
}) *CRMSignalService {
	return &CRMSignalService{signalRepo: signalRepo, summaryRefresh: summaryRefresh}
}

// ListCompanySignals returns signals rolled up through company relationships.
func (s *CRMSignalService) ListCompanySignals(ctx context.Context, workspaceID, companyID string, pagination model.PMPagination) ([]model.CRMBuyerSignal, int64, error) {
	if workspaceID == "" || companyID == "" {
		return nil, 0, fmt.Errorf("workspace_id and company_id are required")
	}
	return s.signalRepo.ListSignalsByCompany(ctx, workspaceID, companyID, pagination)
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
	if !validDetectedSignalType(req.SignalType) {
		return nil, fmt.Errorf("invalid signal_type")
	}

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return nil, fmt.Errorf("confidence must be between 0 and 1")
	}

	sourceType := model.CRMSignalSourceManual
	if req.SourceType != "" {
		sourceType = req.SourceType
	}
	if !validCRMSignalSourceType(sourceType) {
		return nil, fmt.Errorf("invalid source_type")
	}

	signal := &model.CRMBuyerSignal{
		WorkspaceID:     req.WorkspaceID,
		ContactID:       req.ContactID,
		DealID:          req.DealID,
		CompanyID:       req.CompanyID,
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
	if signal.DealID != nil && *signal.DealID != "" {
		if _, err := s.RefreshDealHealthScore(ctx, signal.WorkspaceID, *signal.DealID); err != nil {
			slog.WarnContext(ctx, "failed to refresh deal health after manual signal", "error", err, "deal_id", *signal.DealID)
		}
	}
	return signal, nil
}

func validCRMSignalSourceType(sourceType string) bool {
	switch sourceType {
	case model.CRMSignalSourceEmail, model.CRMSignalSourceMeeting, model.CRMSignalSourceNote,
		model.CRMSignalSourceCall, model.CRMSignalSourceManual, model.CRMSignalSourceSupport:
		return true
	default:
		return false
	}
}

// DeleteSignal removes a buyer signal.
func (s *CRMSignalService) DeleteSignal(ctx context.Context, id string) error {
	return s.signalRepo.DeleteSignal(ctx, id)
}

// DismissSignal hides a signal until the detector observes materially changed evidence.
func (s *CRMSignalService) DismissSignal(ctx context.Context, workspaceID, id, memberID string) error {
	if workspaceID == "" || id == "" || memberID == "" {
		return fmt.Errorf("workspace_id, signal_id, and member_id are required")
	}
	return s.signalRepo.DismissSignal(ctx, workspaceID, id, memberID, time.Now().UTC())
}

// ── Deal Health Scores ──

// GetLatestHealthScore returns the latest health score for a deal.
func (s *CRMSignalService) GetLatestHealthScore(ctx context.Context, workspaceID, dealID string) (*model.CRMDealHealthScore, error) {
	return s.RefreshDealHealthScore(ctx, workspaceID, dealID)
}

// ListHealthScores returns health scores for a workspace.
func (s *CRMSignalService) ListHealthScores(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMDealHealthScore, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	if err := s.RefreshWorkspaceHealthScores(ctx, workspaceID); err != nil {
		slog.WarnContext(ctx, "deal health refresh before list failed", "error", err, "workspace_id", workspaceID)
	}
	return s.signalRepo.ListHealthScores(ctx, workspaceID, pagination)
}

// RefreshWorkspaceHealthScores calculates current scores for every deal in a
// workspace. It is used by the periodic producer and as a read-time backstop.
func (s *CRMSignalService) RefreshWorkspaceHealthScores(ctx context.Context, workspaceID string) error {
	if s == nil || s.dealRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	for page := 1; ; page++ {
		deals, total, err := s.dealRepo.List(ctx, workspaceID, model.CRMDealListFilters{}, model.PMPagination{Page: page, PerPage: 100})
		if err != nil {
			return err
		}
		for index := range deals {
			if _, err := s.calculateAndStoreDealHealthScore(ctx, &deals[index], time.Now().UTC()); err != nil {
				slog.WarnContext(ctx, "deal health calculation failed", "error", err, "workspace_id", workspaceID, "deal_id", deals[index].ID)
			}
		}
		if page*100 >= int(total) || len(deals) == 0 {
			return nil
		}
	}
}

// RefreshDealHealthScore calculates and persists a current score for one deal.
func (s *CRMSignalService) RefreshDealHealthScore(ctx context.Context, workspaceID, dealID string) (*model.CRMDealHealthScore, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(dealID) == "" {
		return nil, fmt.Errorf("workspace_id and deal_id are required")
	}
	if s.dealRepo == nil {
		return s.signalRepo.GetLatestHealthScore(ctx, workspaceID, dealID)
	}
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if deal == nil || deal.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	return s.calculateAndStoreDealHealthScore(ctx, deal, time.Now().UTC())
}

func (s *CRMSignalService) calculateAndStoreDealHealthScore(ctx context.Context, deal *model.CRMDeal, now time.Time) (*model.CRMDealHealthScore, error) {
	signals, err := s.signalRepo.ListSignalsForDealSince(ctx, deal.WorkspaceID, deal.ID, now.Add(-dealHealthSignalWindow))
	if err != nil {
		return nil, err
	}
	score, factors := calculateDealHealthScore(deal, signals, now)
	health := &model.CRMDealHealthScore{WorkspaceID: deal.WorkspaceID, DealID: deal.ID, Score: score, Factors: factors, CalculatedAt: now}
	if err := s.signalRepo.SaveCalculatedHealthScore(ctx, health); err != nil {
		return nil, err
	}
	return health, nil
}

type dealSignalWeight struct {
	weight   float64
	halfLife float64
}

var dealHealthSignalWeights = map[string]dealSignalWeight{
	model.CRMSignalBuyingIntent:      {weight: 15, halfLife: 7},
	model.CRMSignalBudgetSignal:      {weight: 12, halfLife: 21},
	model.CRMSignalTimelineSignal:    {weight: 10, halfLife: 10},
	model.CRMSignalChampionSignal:    {weight: 12, halfLife: 180},
	model.CRMSignalObjection:         {weight: -12, halfLife: 30},
	model.CRMSignalCompetitorMention: {weight: -8, halfLife: 45},
	model.CRMSignalRiskSignal:        {weight: -20, halfLife: 30},
}

func calculateDealHealthScore(deal *model.CRMDeal, signals []model.CRMBuyerSignal, now time.Time) (int, model.JSONB) {
	if deal.Stage != nil {
		switch deal.Stage.StageType {
		case model.CRMStageTypeWon:
			return 100, model.JSONB{"outcome": "closed_won", "model_version": "deterministic-v1"}
		case model.CRMStageTypeLost:
			return 0, model.JSONB{"outcome": "closed_lost", "model_version": "deterministic-v1"}
		}
	}
	stageProbability := 50
	if deal.Stage != nil {
		stageProbability = deal.Stage.Probability
	}
	if deal.Probability != nil {
		stageProbability = *deal.Probability
	}
	stageProbability = max(0, min(100, stageProbability))
	score := 55.0 + float64(stageProbability-50)*0.2
	activityPenalty := 0.0
	ageDays := 0.0
	if !deal.UpdatedAt.IsZero() {
		ageDays = math.Max(0, now.Sub(deal.UpdatedAt).Hours()/24)
	}
	switch {
	case ageDays > 30:
		activityPenalty = -15
	case ageDays > 14:
		activityPenalty = -8
	case ageDays > 7:
		activityPenalty = -4
	}
	score += activityPenalty
	closeDateImpact := 0.0
	if deal.CloseDate != nil && deal.CloseDate.Before(now) {
		closeDateImpact = -15
		score += closeDateImpact
	}
	signalImpact := 0.0
	recentPositiveTypes := map[string]bool{}
	for _, signal := range signals {
		weight, ok := dealHealthSignalWeights[signal.SignalType]
		if !ok {
			continue
		}
		age := math.Max(0, now.Sub(signal.DetectedAt).Hours()/24)
		confidence := signal.Confidence
		if signal.SourceType == model.CRMSignalSourceManual && confidence <= 0 {
			confidence = 1
		}
		confidence = math.Max(0, math.Min(1, confidence))
		impact := weight.weight * confidence * math.Pow(0.5, age/weight.halfLife)
		signalImpact += impact
		if weight.weight > 0 && age <= 14 {
			recentPositiveTypes[signal.SignalType] = true
		}
	}
	compoundBoost := 0.0
	if len(recentPositiveTypes) >= 2 {
		compoundBoost = 5
	}
	score += signalImpact + compoundBoost
	finalScore := int(math.Round(math.Max(0, math.Min(100, score))))
	factors := model.JSONB{
		"stage_probability":     stageProbability,
		"activity_recency_days": int(math.Max(0, math.Floor(ageDays))),
		"activity_impact":       int(math.Round(activityPenalty)),
		"signal_count":          len(signals),
		"signal_impact":         int(math.Round(signalImpact)),
		"compound_signal_boost": int(compoundBoost),
		"close_date_impact":     int(closeDateImpact),
		"model_version":         "deterministic-v1",
	}
	return finalScore, factors
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
	companyRefresh, canRefreshCompany := s.summaryRefresh.(CompanySummaryRefreshRequester)
	if !canRefreshCompany {
		return
	}
	if signal.DealID != nil && *signal.DealID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectDeal, *signal.DealID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from crm signal deal", "error", err, "deal_id", *signal.DealID)
		}
	}
	if signal.ContactID != nil && *signal.ContactID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectContact, *signal.ContactID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from crm signal contact", "error", err, "contact_id", *signal.ContactID)
		}
	}
	if signal.CompanyID != nil && *signal.CompanyID != "" {
		if err := companyRefresh.RequestCompanyRefreshForObject(ctx, signal.WorkspaceID, model.CRMObjectCompany, *signal.CompanyID); err != nil {
			slog.ErrorContext(ctx, "request company summary refresh from crm signal", "error", err, "company_id", *signal.CompanyID)
		}
	}
}
