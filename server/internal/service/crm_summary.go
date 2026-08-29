package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

const (
	crmSummaryContactEmailLimit      = 12
	crmSummaryContactSignalLimit     = 10
	crmSummaryContactOpenDealLimit   = 5
	crmSummaryDealEmailLimit         = 16
	crmSummaryDealSignalLimit        = 10
	crmSummaryCompanyContactLimit    = 25
	crmSummaryCompanyDealLimit       = 20
	crmSummaryCompanyTaskLimit       = 20
	crmSummaryCompanySupportLimit    = 15
	crmSummaryCompanyActivityLimit   = 60
	crmSummaryCompanyActivityReadMax = 250
	crmSummaryCompanyWindowDays      = 90
	crmSummaryCompanyExcerptChars    = 600
	crmSummaryContactEmailWindowDays = 45
	crmSummarySignalWindowDays       = 60
	crmSummaryTouchedContactDays     = 30
	crmSummaryGenerationVersion      = "phase1c-en"
	crmSummaryMaxBodyChars           = 1200
	crmSummaryMaxMarkdownChars       = 2200
	crmSummaryMaxHighlightChars      = 220
	crmSummaryReadinessThreshold     = 3
	crmIntelligenceEvidenceLimit     = 24
	crmSummaryDailyCronSchedule      = "0 3 * * *"
)

var crmSummaryHTMLTagRe = regexp.MustCompile(`(?s)<[^>]*>`)

type summaryWorkflowRunner interface {
	StartSummaryRefresh(ctx context.Context, input model.CRMEntitySummaryRefreshInput) error
	StartDailyReconciliation(ctx context.Context) error
}

type temporalSummaryWorkflowRunner struct {
	client tclient.Client
}

func (r *temporalSummaryWorkflowRunner) StartSummaryRefresh(ctx context.Context, input model.CRMEntitySummaryRefreshInput) error {
	if r == nil || r.client == nil || input.EntityID == "" || input.EntityType == "" {
		return nil
	}

	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:                    summaryWorkflowID(input.EntityType, input.EntityID),
		TaskQueue:             temporalapp.QueueAutomation,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, temporalapp.CRMEntitySummaryWorkflow, input)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start crm summary workflow: %w", err)
	}
	return nil
}

func (r *temporalSummaryWorkflowRunner) StartDailyReconciliation(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}

	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:           crmSummaryDailyWorkflowID,
		TaskQueue:    temporalapp.QueueAutomation,
		CronSchedule: crmSummaryDailyCronSchedule,
	}, temporalapp.CRMSummaryDailyReconciliationWorkflow)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start crm summary daily reconciliation workflow: %w", err)
	}
	return nil
}

const crmSummaryDailyWorkflowID = "crm-summary-daily-refresh"

// CRMSummaryService owns CRM summary artifacts, refresh requests, and generation.
type CRMSummaryService struct {
	summaryRepo        *repository.CRMSummaryRepository
	contactRepo        *repository.CRMContactRepository
	companyRepo        *repository.CRMCompanyRepository
	dealRepo           *repository.CRMDealRepository
	associationRepo    *repository.CRMAssociationRepository
	signalRepo         *repository.CRMSignalRepository
	emailRepo          *repository.CRMEmailRepository
	timelineRepo       *repository.CRMCompanyTimelineRepository
	taskRepo           *repository.PMTaskRepository
	supportRepo        *repository.SupportConversationRepository
	supportMessageRepo *repository.SupportMessageRepository
	activityRepo       *repository.CRMActivityRepository
	signalDetector     *SignalDetectionService
	llmProvider        llm.Provider
	runner             summaryWorkflowRunner
}

// SetIntelligenceDependencies enables explicit detect-then-summarize refreshes.
func (s *CRMSummaryService) SetIntelligenceDependencies(detector *SignalDetectionService, activityRepo *repository.CRMActivityRepository, supportMessageRepo *repository.SupportMessageRepository) *CRMSummaryService {
	s.signalDetector = detector
	s.activityRepo = activityRepo
	s.supportMessageRepo = supportMessageRepo
	return s
}

// SetCompanyEvidenceRepositories enables account-level company summaries.
func (s *CRMSummaryService) SetCompanyEvidenceRepositories(
	timelineRepo *repository.CRMCompanyTimelineRepository,
	taskRepo *repository.PMTaskRepository,
	supportRepo *repository.SupportConversationRepository,
) *CRMSummaryService {
	s.timelineRepo = timelineRepo
	s.taskRepo = taskRepo
	s.supportRepo = supportRepo
	return s
}

// NewCRMSummaryService creates a new CRMSummaryService.
func NewCRMSummaryService(
	summaryRepo *repository.CRMSummaryRepository,
	contactRepo *repository.CRMContactRepository,
	companyRepo *repository.CRMCompanyRepository,
	dealRepo *repository.CRMDealRepository,
	associationRepo *repository.CRMAssociationRepository,
	signalRepo *repository.CRMSignalRepository,
	emailRepo *repository.CRMEmailRepository,
	llmProvider llm.Provider,
	temporalClient tclient.Client,
) *CRMSummaryService {
	return &CRMSummaryService{
		summaryRepo:     summaryRepo,
		contactRepo:     contactRepo,
		companyRepo:     companyRepo,
		dealRepo:        dealRepo,
		associationRepo: associationRepo,
		signalRepo:      signalRepo,
		emailRepo:       emailRepo,
		llmProvider:     llmProvider,
		runner:          &temporalSummaryWorkflowRunner{client: temporalClient},
	}
}

// GetContactSummary returns the stored summary for a contact, or nil when none exists.
func (s *CRMSummaryService) GetContactSummary(ctx context.Context, workspaceID, contactID string) (*model.CRMEntitySummary, error) {
	return s.getSummary(ctx, workspaceID, model.CRMObjectContact, contactID)
}

// GetDealSummary returns the stored summary for a deal, or nil when none exists.
func (s *CRMSummaryService) GetDealSummary(ctx context.Context, workspaceID, dealID string) (*model.CRMEntitySummary, error) {
	return s.getSummary(ctx, workspaceID, model.CRMObjectDeal, dealID)
}

// GetCompanySummary returns the stored summary for a company, or nil when none exists.
func (s *CRMSummaryService) GetCompanySummary(ctx context.Context, workspaceID, companyID string) (*model.CRMEntitySummary, error) {
	return s.getSummary(ctx, workspaceID, model.CRMObjectCompany, companyID)
}

func (s *CRMSummaryService) getSummary(ctx context.Context, workspaceID, entityType, entityID string) (*model.CRMEntitySummary, error) {
	if workspaceID == "" || entityID == "" {
		return nil, fmt.Errorf("workspace_id and entity_id are required")
	}
	summary, err := s.summaryRepo.GetByEntity(ctx, workspaceID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	readiness, sources, err := s.loadSummaryReadiness(ctx, workspaceID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = &model.CRMEntitySummary{
			WorkspaceID: workspaceID, EntityType: entityType, EntityID: entityID,
			Status: model.CRMEntitySummaryStatusPendingRefresh, Metadata: model.JSONB{},
		}
	}
	decorateSummaryPresentation(summary, readiness, sources)
	return summary, nil
}

// RequestContactRefresh marks a contact summary stale and starts/bumps the background workflow.
func (s *CRMSummaryService) RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error {
	return s.requestRefresh(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectContact,
		EntityID:    contactID,
	})
}

// RequestDealRefresh marks a deal summary stale and starts/bumps the background workflow.
func (s *CRMSummaryService) RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error {
	return s.requestRefresh(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectDeal,
		EntityID:    dealID,
	})
}

// RequestCompanyRefresh marks a company summary stale and starts/bumps the background workflow.
func (s *CRMSummaryService) RequestCompanyRefresh(ctx context.Context, workspaceID, companyID string) error {
	return s.requestRefresh(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectCompany,
		EntityID:    companyID,
	})
}

// RequestCompanyRefreshForObject resolves the companies affected by a CRM or
// linked workspace object and marks each account summary stale. This keeps
// mutation services independent from the association traversal rules used by
// company summaries.
func (s *CRMSummaryService) RequestCompanyRefreshForObject(ctx context.Context, workspaceID, objectType, objectID string) error {
	if s == nil || s.associationRepo == nil || workspaceID == "" || objectType == "" || objectID == "" {
		return nil
	}

	companyIDs := map[string]struct{}{}
	contactIDs := map[string]struct{}{}
	if objectType == model.CRMObjectCompany {
		companyIDs[objectID] = struct{}{}
	}
	if objectType == model.CRMObjectContact {
		contactIDs[objectID] = struct{}{}
	}

	associations, err := s.associationRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return fmt.Errorf("resolve company summary associations: %w", err)
	}
	for _, association := range associations {
		peerType, peerID := associationPeer(association, objectType, objectID)
		switch peerType {
		case model.CRMObjectCompany:
			companyIDs[peerID] = struct{}{}
		case model.CRMObjectContact:
			contactIDs[peerID] = struct{}{}
		}
	}

	if objectType == model.CRMObjectSupportConversation && s.supportRepo != nil {
		var conversation struct {
			CRMCompanyID *string
			CRMContactID *string
		}
		err := s.supportRepo.DB().WithContext(ctx).
			Table("support_conversations").
			Select("crm_company_id, crm_contact_id").
			Where("workspace_id = ? AND id = ?", workspaceID, objectID).
			Take(&conversation).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("resolve support conversation company: %w", err)
		}
		if conversation.CRMCompanyID != nil && *conversation.CRMCompanyID != "" {
			companyIDs[*conversation.CRMCompanyID] = struct{}{}
		}
		if conversation.CRMContactID != nil && *conversation.CRMContactID != "" {
			contactIDs[*conversation.CRMContactID] = struct{}{}
		}
	}

	for contactID := range contactIDs {
		contactAssociations, err := s.associationRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
		if err != nil {
			return fmt.Errorf("resolve contact company summary associations: %w", err)
		}
		for _, association := range contactAssociations {
			peerType, peerID := associationPeer(association, model.CRMObjectContact, contactID)
			if peerType == model.CRMObjectCompany {
				companyIDs[peerID] = struct{}{}
			}
		}
	}

	for companyID := range companyIDs {
		if err := s.RequestCompanyRefresh(ctx, workspaceID, companyID); err != nil {
			return err
		}
	}
	return nil
}

// RefreshContactSummaryNow immediately recomputes a contact summary for an
// explicit user request, bypassing the debounce used for activity-driven
// refreshes.
func (s *CRMSummaryService) RefreshContactSummaryNow(ctx context.Context, workspaceID, contactID string) (*model.CRMEntitySummary, error) {
	return s.refreshSummaryNow(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectContact,
		EntityID:    contactID,
		Force:       true,
	})
}

// RefreshDealSummaryNow immediately recomputes a deal summary for an explicit
// user request, bypassing the debounce used for activity-driven refreshes.
func (s *CRMSummaryService) RefreshDealSummaryNow(ctx context.Context, workspaceID, dealID string) (*model.CRMEntitySummary, error) {
	return s.refreshSummaryNow(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectDeal,
		EntityID:    dealID,
	})
}

// RefreshCompanySummaryNow immediately recomputes a company summary.
func (s *CRMSummaryService) RefreshCompanySummaryNow(ctx context.Context, workspaceID, companyID string) (*model.CRMEntitySummary, error) {
	return s.refreshSummaryNow(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID,
		EntityType:  model.CRMObjectCompany,
		EntityID:    companyID,
		Force:       true,
	})
}

// RefreshContactIntelligenceNow detects missing grounded signals before
// regenerating the contact summary.
func (s *CRMSummaryService) RefreshContactIntelligenceNow(ctx context.Context, workspaceID, contactID string) (*model.CRMIntelligenceRefreshResult, error) {
	return s.refreshIntelligenceNow(ctx, workspaceID, model.CRMObjectContact, contactID)
}

// RefreshDealIntelligenceNow detects missing grounded signals before
// regenerating the deal summary.
func (s *CRMSummaryService) RefreshDealIntelligenceNow(ctx context.Context, workspaceID, dealID string) (*model.CRMIntelligenceRefreshResult, error) {
	return s.refreshIntelligenceNow(ctx, workspaceID, model.CRMObjectDeal, dealID)
}

// RefreshCompanyIntelligenceNow detects account and related-contact signals
// before regenerating the company summary.
func (s *CRMSummaryService) RefreshCompanyIntelligenceNow(ctx context.Context, workspaceID, companyID string) (*model.CRMIntelligenceRefreshResult, error) {
	return s.refreshIntelligenceNow(ctx, workspaceID, model.CRMObjectCompany, companyID)
}

func (s *CRMSummaryService) refreshIntelligenceNow(ctx context.Context, workspaceID, entityType, entityID string) (*model.CRMIntelligenceRefreshResult, error) {
	result := &model.CRMIntelligenceRefreshResult{Warnings: []string{}}
	payloads, err := s.collectIntelligenceEvidence(ctx, workspaceID, entityType, entityID)
	if err != nil {
		slog.WarnContext(ctx, "CRM intelligence evidence collection failed", "error", err, "workspace_id", workspaceID, "entity_type", entityType, "entity_id", entityID)
		result.Warnings = append(result.Warnings, "CRM signals could not be refreshed. The summary was generated from available evidence.")
	} else {
		result.SourcesAnalyzed = len(payloads)
		if len(payloads) > 0 && s.signalDetector != nil {
			signals, detectErr := s.signalDetector.DetectSignals(ctx, payloads)
			if detectErr != nil {
				slog.WarnContext(ctx, "CRM intelligence signal detection failed", "error", detectErr, "workspace_id", workspaceID, "entity_type", entityType, "entity_id", entityID)
				result.Warnings = append(result.Warnings, "CRM signals could not be refreshed. The summary was generated from available evidence.")
			} else {
				result.SignalsDetected = len(signals)
			}
		}
	}

	summary, err := s.refreshSummaryNow(ctx, model.CRMEntitySummaryRefreshInput{
		WorkspaceID: workspaceID, EntityType: entityType, EntityID: entityID, Force: true,
	})
	if err != nil {
		return nil, err
	}
	result.Summary = summary
	return result, nil
}

func (s *CRMSummaryService) collectIntelligenceEvidence(ctx context.Context, workspaceID, entityType, entityID string) ([]model.SignalSourcePayload, error) {
	if workspaceID == "" || entityType == "" || entityID == "" {
		return nil, fmt.Errorf("workspace_id, entity_type, and entity_id are required")
	}
	inbound := "inbound"
	emailFilters := model.CRMEmailMessageListFilters{Direction: &inbound}
	activityFilters := model.CRMActivityListFilters{}
	switch entityType {
	case model.CRMObjectContact:
		emailFilters.ContactID = &entityID
		activityFilters.ContactID = &entityID
	case model.CRMObjectDeal:
		emailFilters.DealID = &entityID
		activityFilters.DealID = &entityID
	case model.CRMObjectCompany:
		emailFilters.CompanyID = &entityID
		activityFilters.CompanyID = &entityID
	default:
		return nil, fmt.Errorf("unsupported CRM intelligence entity_type %q", entityType)
	}

	payloads := make([]model.SignalSourcePayload, 0, crmIntelligenceEvidenceLimit)
	seen := make(map[string]struct{}, crmIntelligenceEvidenceLimit)
	appendPayload := func(payload model.SignalSourcePayload) {
		if len(payloads) >= crmIntelligenceEvidenceLimit || strings.TrimSpace(payload.SourceID) == "" || strings.TrimSpace(payload.Body) == "" {
			return
		}
		key := signalSourceKey(payload.SourceType, payload.SourceID)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		payloads = append(payloads, payload)
	}

	messages, _, err := s.emailRepo.ListMessages(ctx, workspaceID, emailFilters, model.PMPagination{Page: 1, PerPage: crmIntelligenceEvidenceLimit})
	if err != nil {
		return nil, err
	}
	for index := range messages {
		message := &messages[index]
		payload := model.PayloadFromEmail(message, message.Subject)
		if payload.ContactID == nil && len(message.ContactIDs) > 0 {
			payload.ContactID = &message.ContactIDs[0]
		}
		if entityType == model.CRMObjectCompany {
			payload.CompanyID = &entityID
		}
		appendPayload(payload)
	}

	if s.activityRepo == nil || len(payloads) >= crmIntelligenceEvidenceLimit {
		return payloads, nil
	}
	activities, _, err := s.activityRepo.List(ctx, workspaceID, activityFilters, model.PMPagination{Page: 1, PerPage: crmIntelligenceEvidenceLimit})
	if err != nil {
		return nil, err
	}
	for _, activity := range activities {
		sourceType := crmActivitySignalSourceType(activity.ActivityType)
		if sourceType == "" {
			continue
		}
		appendPayload(model.SignalSourcePayload{
			SourceType: sourceType, SourceID: activity.ID, WorkspaceID: activity.WorkspaceID,
			ContactID: activity.ContactID, DealID: activity.DealID, CompanyID: activity.CompanyID,
			Subject: strings.TrimSpace(derefString(activity.Subject)), Body: strings.TrimSpace(derefString(activity.Body)),
			Direction: "bilateral", OccurredAt: activity.OccurredAt,
		})
	}
	if entityType == model.CRMObjectCompany && s.timelineRepo != nil && len(payloads) < crmIntelligenceEvidenceLimit {
		rows, timelineErr := s.timelineRepo.List(ctx, workspaceID, entityID, model.CRMCompanyTimelineQuery{
			Filter: model.CRMCompanyTimelineFilterAll, Limit: crmSummaryCompanyActivityReadMax,
		})
		if timelineErr != nil {
			return nil, timelineErr
		}
		for _, row := range rows {
			if row.SourceType != "crm_activity" || row.Description == nil {
				continue
			}
			sourceType := crmActivitySignalSourceType(row.Kind)
			if sourceType == "" {
				continue
			}
			var contactID *string
			if row.Contact != nil && row.Contact.ID != "" {
				value := row.Contact.ID
				contactID = &value
			}
			appendPayload(model.SignalSourcePayload{
				SourceType: sourceType, SourceID: row.SourceID, WorkspaceID: workspaceID,
				ContactID: contactID, CompanyID: &entityID, Subject: row.Title,
				Body: strings.TrimSpace(*row.Description), Direction: "bilateral", OccurredAt: row.OccurredAt,
			})
		}
	}
	if s.supportRepo != nil && s.supportMessageRepo != nil && len(payloads) < crmIntelligenceEvidenceLimit && entityType != model.CRMObjectDeal {
		var conversations []model.SupportConversation
		var supportErr error
		if entityType == model.CRMObjectContact {
			conversations, _, supportErr = s.supportRepo.ListByContact(ctx, workspaceID, entityID, "all", "", model.PMPagination{Page: 1, PerPage: crmSummaryCompanySupportLimit})
		} else {
			conversations, _, supportErr = s.supportRepo.ListByCompany(ctx, workspaceID, entityID, "all", "", model.PMPagination{Page: 1, PerPage: crmSummaryCompanySupportLimit})
		}
		if supportErr != nil {
			return nil, supportErr
		}
		for index := range conversations {
			conversation := &conversations[index]
			messages, messageErr := s.supportMessageRepo.ListByConversation(ctx, workspaceID, conversation.ID, false)
			if messageErr != nil {
				return nil, messageErr
			}
			for messageIndex := len(messages) - 1; messageIndex >= 0; messageIndex-- {
				message := &messages[messageIndex]
				if message.SenderType != "customer" || message.IsInternal || message.MessageType != "reply" {
					continue
				}
				payload := model.PayloadFromSupportMessage(message, conversation)
				if entityType == model.CRMObjectCompany {
					payload.CompanyID = &entityID
				}
				appendPayload(payload)
				if len(payloads) >= crmIntelligenceEvidenceLimit {
					break
				}
			}
			if len(payloads) >= crmIntelligenceEvidenceLimit {
				break
			}
		}
	}
	return payloads, nil
}

func crmActivitySignalSourceType(activityType string) string {
	switch activityType {
	case model.CRMActivityNote:
		return model.CRMSignalSourceNote
	case model.CRMActivityCall:
		return model.CRMSignalSourceCall
	case model.CRMActivityMeeting:
		return model.CRMSignalSourceMeeting
	default:
		return ""
	}
}

func (s *CRMSummaryService) refreshSummaryNow(ctx context.Context, input model.CRMEntitySummaryRefreshInput) (*model.CRMEntitySummary, error) {
	if s == nil || s.summaryRepo == nil {
		return nil, fmt.Errorf("summary service not configured")
	}
	if input.WorkspaceID == "" || input.EntityType == "" || input.EntityID == "" {
		return nil, fmt.Errorf("workspace_id, entity_type, and entity_id are required")
	}

	if err := s.summaryRepo.UpsertRefreshRequest(ctx, input, time.Now().UTC()); err != nil {
		return nil, err
	}
	if _, err := s.RefreshSummary(ctx, input); err != nil {
		return nil, err
	}
	return s.getSummary(ctx, input.WorkspaceID, input.EntityType, input.EntityID)
}

func (s *CRMSummaryService) requestRefresh(ctx context.Context, input model.CRMEntitySummaryRefreshInput) error {
	if s == nil || s.summaryRepo == nil {
		return nil
	}
	if input.WorkspaceID == "" || input.EntityType == "" || input.EntityID == "" {
		return fmt.Errorf("workspace_id, entity_type, and entity_id are required")
	}

	triggeredAt := time.Now().UTC()
	if err := s.summaryRepo.UpsertRefreshRequest(ctx, input, triggeredAt); err != nil {
		return err
	}
	if s.runner != nil {
		if err := s.runner.StartSummaryRefresh(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDailyReconciliation starts the singleton Temporal cron workflow if it is not already running.
func (s *CRMSummaryService) EnsureDailyReconciliation(ctx context.Context) error {
	if s == nil || s.runner == nil {
		return nil
	}
	return s.runner.StartDailyReconciliation(ctx)
}

// RunDailyReconciliation requests refreshes for active CRM entities.
func (s *CRMSummaryService) RunDailyReconciliation(ctx context.Context) (*model.CRMSummaryReconciliationResult, error) {
	if s == nil || s.summaryRepo == nil {
		return &model.CRMSummaryReconciliationResult{}, nil
	}

	result := &model.CRMSummaryReconciliationResult{}
	deals, err := s.summaryRepo.ListOpenDealRefreshInputs(ctx)
	if err != nil {
		return nil, err
	}
	for _, input := range deals {
		if err := s.requestRefresh(ctx, input); err != nil {
			return nil, err
		}
		result.DealsQueued++
	}

	contacts, err := s.summaryRepo.ListRecentlyTouchedContactRefreshInputs(ctx, time.Now().UTC().AddDate(0, 0, -crmSummaryTouchedContactDays))
	if err != nil {
		return nil, err
	}
	for _, input := range contacts {
		if err := s.requestRefresh(ctx, input); err != nil {
			return nil, err
		}
		result.ContactsQueued++
	}

	companies, err := s.summaryRepo.ListStaleCompanyRefreshInputs(ctx, time.Now().UTC().AddDate(0, 0, -crmSummaryCompanyWindowDays))
	if err != nil {
		return nil, err
	}
	for _, input := range companies {
		if err := s.requestRefresh(ctx, input); err != nil {
			return nil, err
		}
		result.CompaniesQueued++
	}

	return result, nil
}

// RefreshSummary recomputes one entity summary and reports whether another pass
// should immediately continue after this run.
func (s *CRMSummaryService) RefreshSummary(ctx context.Context, input model.CRMEntitySummaryRefreshInput) (*model.CRMEntitySummaryRefreshResult, error) {
	if s == nil || s.summaryRepo == nil {
		return nil, fmt.Errorf("summary service not configured")
	}
	if s.llmProvider == nil {
		status, updateErr := s.summaryRepo.UpdateFailure(ctx, input, "LLM provider not configured")
		if updateErr != nil {
			return nil, updateErr
		}
		return &model.CRMEntitySummaryRefreshResult{EntityType: input.EntityType, EntityID: input.EntityID, Status: status}, fmt.Errorf("LLM provider not configured")
	}

	current, err := s.summaryRepo.GetByEntity(ctx, input.WorkspaceID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	if current == nil || current.LastTriggeredAt == nil {
		return &model.CRMEntitySummaryRefreshResult{
			EntityType: input.EntityType,
			EntityID:   input.EntityID,
			Status:     model.CRMEntitySummaryStatusReady,
		}, nil
	}
	// A user-triggered refresh can finish while an older debounced workflow is
	// still sleeping. Treat that workflow as satisfied instead of paying for a
	// second generation pass over the same evidence.
	if current.Status == model.CRMEntitySummaryStatusReady && current.ComputedAt != nil && !current.ComputedAt.Before(*current.LastTriggeredAt) {
		return &model.CRMEntitySummaryRefreshResult{
			EntityType:        input.EntityType,
			EntityID:          input.EntityID,
			Status:            current.Status,
			Highlights:        len(current.Highlights),
			SourceEmailCount:  intValue(current.Metadata["source_email_count"]),
			SourceSignalCount: intValue(current.Metadata["source_signal_count"]),
		}, nil
	}

	requestedAt := current.LastTriggeredAt.UTC()
	if !input.Force && (input.EntityType == model.CRMObjectContact || input.EntityType == model.CRMObjectCompany) {
		readiness, _, readinessErr := s.loadSummaryReadiness(ctx, input.WorkspaceID, input.EntityType, input.EntityID)
		if readinessErr != nil {
			return nil, readinessErr
		}
		if readiness != nil && !readiness.Ready {
			return &model.CRMEntitySummaryRefreshResult{
				EntityType: input.EntityType, EntityID: input.EntityID, Status: current.Status,
			}, nil
		}
	}

	var generated summaryGenerationOutput
	var sourceWindowStart *time.Time
	var sourceWindowEnd *time.Time
	var metadata model.JSONB

	switch input.EntityType {
	case model.CRMObjectContact:
		generated, sourceWindowStart, sourceWindowEnd, metadata, err = s.generateContactSummary(ctx, input.WorkspaceID, input.EntityID)
	case model.CRMObjectDeal:
		generated, sourceWindowStart, sourceWindowEnd, metadata, err = s.generateDealSummary(ctx, input.WorkspaceID, input.EntityID)
	case model.CRMObjectCompany:
		generated, sourceWindowStart, sourceWindowEnd, metadata, err = s.generateCompanySummary(ctx, input.WorkspaceID, input.EntityID)
	default:
		err = fmt.Errorf("unsupported crm summary entity_type %q", input.EntityType)
	}
	if err != nil {
		status, updateErr := s.summaryRepo.UpdateFailure(ctx, input, err.Error())
		if updateErr != nil {
			return nil, updateErr
		}
		return &model.CRMEntitySummaryRefreshResult{
			EntityType: input.EntityType,
			EntityID:   input.EntityID,
			Status:     status,
		}, err
	}

	computedAt := time.Now().UTC()
	status, needsContinue, err := s.summaryRepo.UpdateGenerated(
		ctx,
		input,
		requestedAt,
		computedAt,
		generated.SummaryMarkdown,
		generated.Highlights,
		sourceWindowStart,
		sourceWindowEnd,
		metadata,
	)
	if err != nil {
		return nil, err
	}

	return &model.CRMEntitySummaryRefreshResult{
		EntityType:        input.EntityType,
		EntityID:          input.EntityID,
		Status:            status,
		NeedsContinue:     needsContinue,
		Highlights:        len(generated.Highlights),
		SourceEmailCount:  intValue(metadata["source_email_count"]),
		SourceSignalCount: intValue(metadata["source_signal_count"]),
	}, nil
}

type summaryGenerationOutput struct {
	SummaryMarkdown string                     `json:"summary_markdown"`
	Highlights      model.CRMSummaryHighlights `json:"highlights"`
}

type summaryPromptEntity struct {
	EntityType     string                   `json:"entity_type"`
	Contact        *contactSummarySnapshot  `json:"contact,omitempty"`
	Company        *companySummarySnapshot  `json:"company,omitempty"`
	Deal           *dealSummarySnapshot     `json:"deal,omitempty"`
	Companies      []companySummarySnapshot `json:"companies,omitempty"`
	OpenDeals      []dealSummarySnapshot    `json:"open_deals,omitempty"`
	LinkedContacts []contactSummarySnapshot `json:"linked_contacts,omitempty"`
	RecentEmails   []summaryEmailSnippet    `json:"recent_emails,omitempty"`
	Signals   []summarySignalSnippet   `json:"signals,omitempty"`
	RelatedDeals   []dealSummarySnapshot    `json:"related_deals,omitempty"`
	RelatedTasks   []summaryTaskSnapshot    `json:"related_tasks,omitempty"`
	RelatedSupport []summarySupportSnapshot `json:"related_support,omitempty"`
	RecentActivity []summaryActivitySnippet `json:"recent_activity,omitempty"`
}

type contactSummarySnapshot struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email,omitempty"`
	JobTitle       string   `json:"job_title,omitempty"`
	LifecycleStage string   `json:"lifecycle_stage,omitempty"`
	LeadStatus     string   `json:"lead_status,omitempty"`
	Companies      []string `json:"companies,omitempty"`
}

type companySummarySnapshot struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Domain        string   `json:"domain,omitempty"`
	Industry      string   `json:"industry,omitempty"`
	Description   string   `json:"description,omitempty"`
	Headquarters  string   `json:"headquarters,omitempty"`
	EmployeeCount *int     `json:"employee_count,omitempty"`
	AnnualRevenue *float64 `json:"annual_revenue,omitempty"`
}

type dealSummarySnapshot struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Stage       string   `json:"stage,omitempty"`
	StageType   string   `json:"stage_type,omitempty"`
	Amount      *float64 `json:"amount,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	Probability *int     `json:"probability,omitempty"`
	CloseDate   *string  `json:"close_date,omitempty"`
}

type summaryEmailSnippet struct {
	SentAt     time.Time `json:"sent_at"`
	Direction  string    `json:"direction"`
	Subject    string    `json:"subject,omitempty"`
	From       string    `json:"from"`
	To         []string  `json:"to,omitempty"`
	CC         []string  `json:"cc,omitempty"`
	Body       string    `json:"body"`
	ContactIDs []string  `json:"contact_ids,omitempty"`
}

type summarySignalSnippet struct {
	SignalType string    `json:"signal_type"`
	Summary    string    `json:"summary"`
	Confidence float64   `json:"confidence"`
	DetectedAt time.Time `json:"detected_at"`
}

type summaryTaskSnapshot struct {
	ID        string     `json:"id"`
	Key       string     `json:"key,omitempty"`
	Name      string     `json:"name"`
	State     string     `json:"state,omitempty"`
	StateType string     `json:"state_type,omitempty"`
	Priority  string     `json:"priority,omitempty"`
	Blocked   bool       `json:"blocked,omitempty"`
	Completed bool       `json:"completed,omitempty"`
	Deadline  *time.Time `json:"deadline,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type summarySupportSnapshot struct {
	ID        string    `json:"id"`
	DisplayID int       `json:"display_id"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"`
	Priority  string    `json:"priority,omitempty"`
	Channel   string    `json:"channel,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type summaryActivitySnippet struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	EventType   string    `json:"event_type"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
	Contact     string    `json:"contact,omitempty"`
	EntityType  string    `json:"entity_type,omitempty"`
	EntityName  string    `json:"entity_name,omitempty"`
}

func (s *CRMSummaryService) generateContactSummary(ctx context.Context, workspaceID, contactID string) (summaryGenerationOutput, *time.Time, *time.Time, model.JSONB, error) {
	contact, err := s.contactRepo.GetByID(ctx, contactID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	if contact == nil || contact.WorkspaceID != workspaceID {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("contact not found")
	}

	companies, companyNames, openDeals, err := s.loadContactAssociations(ctx, workspaceID, contactID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	messages, signals, sourceWindowStart, sourceWindowEnd, err := s.loadSummaryEvidence(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	readiness, sources, recentActivity, _, activityTimes, err := s.loadSummaryContext(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	sourceWindowStart, sourceWindowEnd = mergeSummaryBounds(sourceWindowStart, sourceWindowEnd, activityTimes)

	payload := summaryPromptEntity{
		EntityType: model.CRMObjectContact,
		Contact: &contactSummarySnapshot{
			ID:             contact.ID,
			Name:           contactDisplayName(*contact),
			Email:          stringValue(contact.Email),
			JobTitle:       stringValue(contact.JobTitle),
			LifecycleStage: contact.LifecycleStage,
			LeadStatus:     contact.LeadStatus,
			Companies:      companyNames,
		},
		Companies:      companies,
		OpenDeals:      openDeals,
		RecentEmails:   messages,
		Signals:   signals,
		RecentActivity: recentActivity,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("marshal contact summary usage payload: %w", err)
	}

	output, err := s.generateSummaryLLM(ctx, AIUsageMeteringContext{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureCRMSummary,
		IdempotencyKey: aiUsagePayloadIdempotencyKey(payloadJSON, workspaceID, BillingFeatureCRMSummary, crmSummaryGenerationVersion, model.CRMObjectContact, contactID),
		Metadata: map[string]interface{}{
			"entity_type": model.CRMObjectContact,
			"entity_id":   contactID,
		},
	}, payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	metadata := model.JSONB{
		"source_email_count":  len(messages),
		"source_signal_count": len(signals),
		"generation_version":  crmSummaryGenerationVersion,
		"source_refs":         sources,
	}
	if readiness != nil {
		metadata["readiness_count"] = readiness.Count
	}
	return output, sourceWindowStart, sourceWindowEnd, metadata, nil
}

func (s *CRMSummaryService) generateDealSummary(ctx context.Context, workspaceID, dealID string) (summaryGenerationOutput, *time.Time, *time.Time, model.JSONB, error) {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	if deal == nil || deal.WorkspaceID != workspaceID {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("deal not found")
	}

	contacts, companies, err := s.loadDealAssociations(ctx, workspaceID, dealID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	messages, signals, sourceWindowStart, sourceWindowEnd, err := s.loadSummaryEvidence(ctx, workspaceID, model.CRMObjectDeal, dealID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	payload := summaryPromptEntity{
		EntityType:     model.CRMObjectDeal,
		Deal:           dealSnapshot(*deal),
		LinkedContacts: contacts,
		Companies:      companies,
		RecentEmails:   messages,
		Signals:   signals,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("marshal deal summary usage payload: %w", err)
	}

	output, err := s.generateSummaryLLM(ctx, AIUsageMeteringContext{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureCRMSummary,
		IdempotencyKey: aiUsagePayloadIdempotencyKey(payloadJSON, workspaceID, BillingFeatureCRMSummary, crmSummaryGenerationVersion, model.CRMObjectDeal, dealID),
		Metadata: map[string]interface{}{
			"entity_type": model.CRMObjectDeal,
			"entity_id":   dealID,
		},
	}, payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	return output, sourceWindowStart, sourceWindowEnd, model.JSONB{
		"source_email_count":  len(messages),
		"source_signal_count": len(signals),
		"generation_version":  crmSummaryGenerationVersion,
	}, nil
}

func (s *CRMSummaryService) generateCompanySummary(ctx context.Context, workspaceID, companyID string) (summaryGenerationOutput, *time.Time, *time.Time, model.JSONB, error) {
	if s.timelineRepo == nil || s.taskRepo == nil || s.supportRepo == nil {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("company summary evidence is not configured")
	}
	company, err := s.companyRepo.GetByID(ctx, companyID)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	if company == nil || company.WorkspaceID != workspaceID {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("company not found")
	}

	contacts, contactTotal, err := s.companyRepo.ListContacts(ctx, workspaceID, companyID, "", model.PMPagination{Page: 1, PerPage: crmSummaryCompanyContactLimit})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	contactSnapshots := make([]contactSummarySnapshot, 0, len(contacts))
	for _, contact := range contacts {
		contactSnapshots = append(contactSnapshots, contactSummarySnapshot{
			ID: contact.ID, Name: contactDisplayName(contact), Email: stringValue(contact.Email),
			JobTitle: stringValue(contact.JobTitle), LifecycleStage: contact.LifecycleStage, LeadStatus: contact.LeadStatus,
		})
	}

	dealRows, dealTotal, err := s.companyRepo.ListDeals(ctx, workspaceID, companyID, "", model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	dealRows = prioritizeCompanyDeals(dealRows, crmSummaryCompanyDealLimit)
	dealSnapshots := make([]dealSummarySnapshot, 0, len(dealRows))
	for _, deal := range dealRows {
		dealSnapshots = append(dealSnapshots, *dealSnapshot(deal))
	}

	taskRows, taskTotal, err := s.taskRepo.List(ctx, workspaceID, model.PMTaskFilters{CompanyRollupID: &companyID}, model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	taskRows = prioritizeCompanyTasks(taskRows, crmSummaryCompanyTaskLimit)
	taskSnapshots := make([]summaryTaskSnapshot, 0, len(taskRows))
	for _, task := range taskRows {
		taskSnapshots = append(taskSnapshots, summaryTaskSnapshot{
			ID: task.ID, Name: task.Name, State: stringValue(task.StateName), StateType: stringValue(task.StateType),
			Priority: task.Priority, Blocked: task.Blocked, Completed: task.Completed, Deadline: task.Deadline, UpdatedAt: task.UpdatedAt,
		})
	}

	supportRows, supportTotal, err := s.supportRepo.ListByCompany(ctx, workspaceID, companyID, "all", "", model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	supportRows = prioritizeCompanySupport(supportRows, crmSummaryCompanySupportLimit)
	supportSnapshots := make([]summarySupportSnapshot, 0, len(supportRows))
	for _, conversation := range supportRows {
		supportSnapshots = append(supportSnapshots, summarySupportSnapshot{
			ID: conversation.ID, DisplayID: conversation.DisplayID, Subject: conversation.Subject,
			Status: conversation.Status, Priority: conversation.Priority, Channel: conversation.Channel, UpdatedAt: conversation.UpdatedAt,
		})
	}

	signalRows, signalTotal, err := s.signalRepo.ListSignalsByCompany(ctx, workspaceID, companyID, model.PMPagination{Page: 1, PerPage: crmSummaryDealSignalLimit})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	signalSnapshots := make([]summarySignalSnippet, 0, len(signalRows))
	for _, signal := range signalRows {
		signalSnapshots = append(signalSnapshots, summarySignalSnippet{
			SignalType: signal.SignalType, Summary: strings.TrimSpace(signal.Summary),
			Confidence: signal.Confidence, DetectedAt: signal.DetectedAt,
		})
	}

	timelineRows, err := s.timelineRepo.List(ctx, workspaceID, companyID, model.CRMCompanyTimelineQuery{
		Filter: model.CRMCompanyTimelineFilterAll,
		Limit:  crmSummaryCompanyActivityReadMax,
	})
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	activities, activityCounts, activityTimes := sampleCompanySummaryActivity(timelineRows, time.Now().UTC().AddDate(0, 0, -crmSummaryCompanyWindowDays))
	readiness, sources := buildSummaryReadiness(timelineRows, time.Now().UTC().AddDate(0, 0, -crmSummaryCompanyWindowDays))
	sourceWindowStart, sourceWindowEnd := boundsFromTimes(activityTimes)

	payload := summaryPromptEntity{
		EntityType:     model.CRMObjectCompany,
		Company:        ptrCompanySummarySnapshot(companySnapshot(*company)),
		LinkedContacts: contactSnapshots,
		RelatedDeals:   dealSnapshots,
		RelatedTasks:   taskSnapshots,
		RelatedSupport: supportSnapshots,
		Signals:   signalSnapshots,
		RecentActivity: activities,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, fmt.Errorf("marshal company summary usage payload: %w", err)
	}
	output, err := s.generateSummaryLLM(ctx, AIUsageMeteringContext{
		WorkspaceID: workspaceID, FeatureKey: BillingFeatureCRMSummary,
		IdempotencyKey: aiUsagePayloadIdempotencyKey(payloadJSON, workspaceID, BillingFeatureCRMSummary, crmSummaryGenerationVersion, model.CRMObjectCompany, companyID),
		Metadata:       map[string]interface{}{"entity_type": model.CRMObjectCompany, "entity_id": companyID},
	}, payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}
	metadata := model.JSONB{
		"source_contact_count": contactTotal, "source_deal_count": dealTotal,
		"source_task_count": taskTotal, "source_support_count": supportTotal,
		"source_signal_count":   signalTotal,
		"source_activity_count": len(activities), "source_activity_counts": activityCounts,
		"source_window_days": crmSummaryCompanyWindowDays, "generation_version": crmSummaryGenerationVersion,
		"source_refs": sources, "readiness_count": readiness.Count,
	}
	return output, sourceWindowStart, sourceWindowEnd, metadata, nil
}

func ptrCompanySummarySnapshot(value companySummarySnapshot) *companySummarySnapshot { return &value }

func prioritizeCompanyDeals(rows []model.CRMDeal, limit int) []model.CRMDeal {
	result := make([]model.CRMDeal, 0, min(limit, len(rows)))
	for _, wantOpen := range []bool{true, false} {
		for _, row := range rows {
			isOpen := row.Stage != nil && row.Stage.StageType == model.CRMStageTypeOpen
			if isOpen == wantOpen {
				result = append(result, row)
				if len(result) == limit {
					return result
				}
			}
		}
	}
	return result
}

func prioritizeCompanyTasks(rows []model.BoardTask, limit int) []model.BoardTask {
	result := make([]model.BoardTask, 0, min(limit, len(rows)))
	for _, wantActive := range []bool{true, false} {
		for _, row := range rows {
			if (!row.Completed) == wantActive {
				result = append(result, row)
				if len(result) == limit {
					return result
				}
			}
		}
	}
	return result
}

func prioritizeCompanySupport(rows []model.SupportConversation, limit int) []model.SupportConversation {
	result := make([]model.SupportConversation, 0, min(limit, len(rows)))
	for _, wantActive := range []bool{true, false} {
		for _, row := range rows {
			isActive := row.Status == model.SupportConversationStatusOpen || row.Status == model.SupportConversationStatusWaitingOnCustomer
			if isActive == wantActive {
				result = append(result, row)
				if len(result) == limit {
					return result
				}
			}
		}
	}
	return result
}

func sampleCompanySummaryActivity(rows []model.CRMCompanyTimelineItem, since time.Time) ([]summaryActivitySnippet, map[string]int, []time.Time) {
	perKindCaps := map[string]int{"email": 12, "meeting": 10, "note": 10, "call": 8, "task": 10, "deal": 10, "support": 10, "enrichment": 6}
	counts := map[string]int{}
	seen := map[string]struct{}{}
	result := make([]summaryActivitySnippet, 0, min(crmSummaryCompanyActivityLimit, len(rows)))
	times := make([]time.Time, 0, cap(result))
	for _, row := range rows {
		if row.OccurredAt.Before(since) || len(result) >= crmSummaryCompanyActivityLimit {
			continue
		}
		key := row.SourceType + ":" + row.SourceID
		if _, ok := seen[key]; ok {
			continue
		}
		capForKind := perKindCaps[row.Kind]
		if capForKind == 0 {
			capForKind = 8
		}
		if counts[row.Kind] >= capForKind {
			continue
		}
		description := strings.TrimSpace(stringValue(row.Description))
		if len(description) > crmSummaryCompanyExcerptChars {
			description = description[:crmSummaryCompanyExcerptChars]
		}
		snippet := summaryActivitySnippet{ID: row.ID, Kind: row.Kind, EventType: row.EventType, Title: row.Title, Description: description, OccurredAt: row.OccurredAt}
		if row.Contact != nil {
			snippet.Contact = row.Contact.Name
		}
		if row.Entity != nil {
			snippet.EntityType, snippet.EntityName = row.Entity.Type, row.Entity.Name
		}
		result = append(result, snippet)
		times = append(times, row.OccurredAt)
		counts[row.Kind]++
		seen[key] = struct{}{}
	}
	return result, counts, times
}

func (s *CRMSummaryService) loadSummaryReadiness(
	ctx context.Context,
	workspaceID, entityType, entityID string,
) (*model.CRMSummaryReadiness, []model.CRMSummarySource, error) {
	readiness, sources, _, _, _, err := s.loadSummaryContext(ctx, workspaceID, entityType, entityID)
	return readiness, sources, err
}

func (s *CRMSummaryService) loadSummaryContext(
	ctx context.Context,
	workspaceID, entityType, entityID string,
) (*model.CRMSummaryReadiness, []model.CRMSummarySource, []summaryActivitySnippet, map[string]int, []time.Time, error) {
	if entityType != model.CRMObjectContact && entityType != model.CRMObjectCompany {
		return nil, nil, nil, nil, nil, nil
	}
	if s.timelineRepo == nil {
		return nil, nil, nil, nil, nil, nil
	}

	query := model.CRMTimelineQuery{Filter: model.CRMTimelineFilterAll, Limit: crmSummaryCompanyActivityReadMax}
	var rows []model.CRMTimelineItem
	var err error
	if entityType == model.CRMObjectContact {
		rows, err = s.timelineRepo.ListContact(ctx, workspaceID, entityID, query)
	} else {
		rows, err = s.timelineRepo.List(ctx, workspaceID, entityID, query)
	}
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	since := time.Now().UTC().AddDate(0, 0, -crmSummaryCompanyWindowDays)
	readiness, sources := buildSummaryReadiness(rows, since)
	if entityType == model.CRMObjectContact {
		if err := s.addSignalReadiness(ctx, workspaceID, entityID, since, readiness, &sources); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}
	activities, counts, times := sampleCompanySummaryActivity(rows, since)
	return readiness, sources, activities, counts, times, nil
}

func buildSummaryReadiness(rows []model.CRMTimelineItem, since time.Time) (*model.CRMSummaryReadiness, []model.CRMSummarySource) {
	allowed := map[string]bool{
		model.CRMTimelineFilterEmail: true, model.CRMTimelineFilterMeeting: true,
		model.CRMTimelineFilterCall: true, model.CRMTimelineFilterNote: true,
		model.CRMTimelineFilterDeal: true, model.CRMTimelineFilterTask: true,
		model.CRMTimelineFilterSupport: true,
	}
	seen := map[string]struct{}{}
	artifacts := make([]model.CRMSummarySource, 0, crmSummaryReadinessThreshold)
	for _, item := range rows {
		if item.OccurredAt.Before(since) || !allowed[item.Kind] {
			continue
		}
		source := summarySourceFromTimeline(item)
		if source.Key == "" {
			continue
		}
		if _, exists := seen[source.Key]; exists {
			continue
		}
		seen[source.Key] = struct{}{}
		artifacts = append(artifacts, source)
	}
	return readinessFromSources(artifacts), firstSummarySources(artifacts, 3)
}

func (s *CRMSummaryService) addSignalReadiness(
	ctx context.Context,
	workspaceID, contactID string,
	since time.Time,
	readiness *model.CRMSummaryReadiness,
	sources *[]model.CRMSummarySource,
) error {
	if s.signalRepo == nil || readiness == nil || readiness.Ready {
		return nil
	}
	rows, _, err := s.signalRepo.ListSignals(ctx, workspaceID, model.CRMSignalListFilters{ContactID: &contactID}, model.PMPagination{Page: 1, PerPage: crmSummaryContactSignalLimit})
	if err != nil {
		return err
	}
	artifacts := append([]model.CRMSummarySource(nil), (*sources)...)
	seen := map[string]struct{}{}
	for _, source := range artifacts {
		seen[source.Key] = struct{}{}
	}
	for _, signal := range rows {
		if signal.DetectedAt.Before(since) {
			continue
		}
		key := "signal:" + signal.ID
		if _, exists := seen[key]; exists {
			continue
		}
		artifacts = append(artifacts, model.CRMSummarySource{
			Key: key, Type: "signal", SourceID: signal.ID,
			Label: strings.TrimSpace(signal.Summary), OccurredAt: signal.DetectedAt,
		})
		seen[key] = struct{}{}
	}
	updated := readinessFromSources(artifacts)
	*readiness = *updated
	*sources = firstSummarySources(artifacts, 3)
	return nil
}

func summarySourceFromTimeline(item model.CRMTimelineItem) model.CRMSummarySource {
	source := model.CRMSummarySource{
		Type: item.Kind, SourceID: item.SourceID, Label: strings.TrimSpace(item.Title), OccurredAt: item.OccurredAt,
	}
	if item.Entity != nil {
		source.EntityType, source.EntityID = item.Entity.Type, item.Entity.ID
		if item.Entity.Name != "" {
			source.Label = item.Entity.Name
		}
		if item.Entity.Type == "email_thread" {
			source.ThreadID = item.Entity.ID
		}
	}
	if source.Label == "" {
		source.Label = summaryReadinessLabel(item.Kind)
	}
	keyID := source.SourceID
	if source.EntityID != "" {
		keyID = source.EntityID
	}
	if keyID != "" {
		source.Key = item.Kind + ":" + keyID
	}
	return source
}

func readinessFromSources(sources []model.CRMSummarySource) *model.CRMSummaryReadiness {
	steps := make([]model.CRMSummaryReadinessStep, 0, crmSummaryReadinessThreshold)
	for index := 0; index < crmSummaryReadinessThreshold; index++ {
		step := model.CRMSummaryReadinessStep{Key: fmt.Sprintf("slot-%d", index+1), Label: "One more activity"}
		if index < len(sources) {
			step.Key = sources[index].Key
			step.Label = summaryReadinessLabel(sources[index].Type)
			step.Complete = true
		}
		steps = append(steps, step)
	}
	count := min(len(sources), crmSummaryReadinessThreshold)
	return &model.CRMSummaryReadiness{Count: count, Threshold: crmSummaryReadinessThreshold, Ready: count >= crmSummaryReadinessThreshold, Steps: steps}
}

func summaryReadinessLabel(kind string) string {
	switch kind {
	case model.CRMTimelineFilterEmail:
		return "Email logged"
	case model.CRMTimelineFilterMeeting:
		return "Meeting recorded"
	case model.CRMTimelineFilterCall:
		return "Call logged"
	case model.CRMTimelineFilterNote:
		return "Note added"
	case model.CRMTimelineFilterDeal:
		return "Deal activity"
	case model.CRMTimelineFilterTask:
		return "Task activity"
	case model.CRMTimelineFilterSupport:
		return "Support activity"
	case "signal":
		return "CRM signal"
	default:
		return "CRM activity"
	}
}

func firstSummarySources(sources []model.CRMSummarySource, limit int) []model.CRMSummarySource {
	if len(sources) <= limit {
		return sources
	}
	return sources[:limit]
}

func mergeSummaryBounds(start, end *time.Time, times []time.Time) (*time.Time, *time.Time) {
	all := append([]time.Time(nil), times...)
	if start != nil {
		all = append(all, *start)
	}
	if end != nil {
		all = append(all, *end)
	}
	return boundsFromTimes(all)
}

func decorateSummaryPresentation(
	summary *model.CRMEntitySummary,
	readiness *model.CRMSummaryReadiness,
	fallbackSources []model.CRMSummarySource,
) {
	if summary == nil {
		return
	}
	summary.Readiness = readiness
	summary.Sources = fallbackSources
	if value, ok := summary.Metadata["source_refs"]; ok {
		encoded, err := json.Marshal(value)
		if err == nil {
			var stored []model.CRMSummarySource
			if json.Unmarshal(encoded, &stored) == nil && len(stored) > 0 {
				summary.Sources = stored
			}
		}
	}
	for _, highlight := range summary.Highlights {
		if highlight.Kind == model.CRMSummaryHighlightNextStep {
			summary.NextStep = highlight.Text
			break
		}
	}
}

func (s *CRMSummaryService) loadSummaryEvidence(ctx context.Context, workspaceID, entityType, entityID string) ([]summaryEmailSnippet, []summarySignalSnippet, *time.Time, *time.Time, error) {
	now := time.Now().UTC()
	emailSince := now.AddDate(0, 0, -crmSummaryContactEmailWindowDays)
	emailLimit := crmSummaryContactEmailLimit
	signalLimit := crmSummaryContactSignalLimit
	emailFilters := model.CRMEmailMessageListFilters{}
	signalFilters := model.CRMSignalListFilters{}
	if entityType == model.CRMObjectDeal {
		emailSince = now.AddDate(0, 0, -crmSummarySignalWindowDays)
		emailLimit = crmSummaryDealEmailLimit
		signalLimit = crmSummaryDealSignalLimit
		emailFilters.DealID = &entityID
		signalFilters.DealID = &entityID
	} else {
		emailFilters.ContactID = &entityID
		signalFilters.ContactID = &entityID
	}

	messageRows, _, err := s.emailRepo.ListMessages(ctx, workspaceID, emailFilters, model.PMPagination{Page: 1, PerPage: emailLimit})
	if err != nil {
		return nil, nil, nil, nil, err
	}

	messages := make([]summaryEmailSnippet, 0, len(messageRows))
	timestamps := make([]time.Time, 0, len(messageRows)+signalLimit)
	for _, row := range messageRows {
		if row.SentAt.Before(emailSince) {
			continue
		}
		body := summaryBody(row.BodyText, row.BodyHTML)
		if body == "" {
			continue
		}
		messages = append(messages, summaryEmailSnippet{
			SentAt:     row.SentAt,
			Direction:  row.Direction,
			Subject:    strings.TrimSpace(row.Subject),
			From:       row.FromAddress,
			To:         parseAddressJSONArray(row.ToAddresses),
			CC:         parseAddressJSONArray(row.CCAddresses),
			Body:       body,
			ContactIDs: append([]string(nil), row.ContactIDs...),
		})
		timestamps = append(timestamps, row.SentAt)
	}

	signalRows, _, err := s.signalRepo.ListSignals(ctx, workspaceID, signalFilters, model.PMPagination{Page: 1, PerPage: signalLimit})
	if err != nil {
		return nil, nil, nil, nil, err
	}

	signalSince := now.AddDate(0, 0, -crmSummarySignalWindowDays)
	signals := make([]summarySignalSnippet, 0, len(signalRows))
	for _, row := range signalRows {
		if row.DetectedAt.Before(signalSince) {
			continue
		}
		signals = append(signals, summarySignalSnippet{
			SignalType: row.SignalType,
			Summary:    strings.TrimSpace(row.Summary),
			Confidence: row.Confidence,
			DetectedAt: row.DetectedAt,
		})
		timestamps = append(timestamps, row.DetectedAt)
	}

	start, end := boundsFromTimes(timestamps)
	return messages, signals, start, end, nil
}

func (s *CRMSummaryService) loadContactAssociations(ctx context.Context, workspaceID, contactID string) ([]companySummarySnapshot, []string, []dealSummarySnapshot, error) {
	assocs, err := s.associationRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return nil, nil, nil, err
	}

	companySnaps := make([]companySummarySnapshot, 0, 4)
	companyNames := make([]string, 0, 4)
	openDeals := make([]dealSummarySnapshot, 0, crmSummaryContactOpenDealLimit)
	seenCompanies := map[string]struct{}{}
	seenDeals := map[string]struct{}{}
	type companyAssociationRef struct {
		id        string
		isPrimary bool
	}
	companyRefs := make([]companyAssociationRef, 0, 4)

	for _, assoc := range assocs {
		otherType, otherID := associationPeer(assoc, model.CRMObjectContact, contactID)
		switch otherType {
		case model.CRMObjectCompany:
			if _, exists := seenCompanies[otherID]; exists {
				continue
			}
			seenCompanies[otherID] = struct{}{}
			companyRefs = append(companyRefs, companyAssociationRef{
				id:        otherID,
				isPrimary: isPrimaryCompanyAssociationLabel(assoc.AssociationLabel),
			})
		case model.CRMObjectDeal:
			if len(openDeals) >= crmSummaryContactOpenDealLimit {
				continue
			}
			if _, exists := seenDeals[otherID]; exists {
				continue
			}
			seenDeals[otherID] = struct{}{}
			deal, err := s.dealRepo.GetByID(ctx, otherID)
			if err != nil || deal == nil || deal.Stage == nil || deal.Stage.StageType != model.CRMStageTypeOpen {
				continue
			}
			openDeals = append(openDeals, *dealSnapshot(*deal))
		}
	}

	slices.SortStableFunc(companyRefs, func(a, b companyAssociationRef) int {
		switch {
		case a.isPrimary == b.isPrimary:
			return 0
		case a.isPrimary:
			return -1
		default:
			return 1
		}
	})

	for _, ref := range companyRefs {
		company, err := s.companyRepo.GetByID(ctx, ref.id)
		if err != nil || company == nil {
			continue
		}
		companySnaps = append(companySnaps, companySnapshot(*company))
		companyNames = append(companyNames, company.Name)
	}

	return companySnaps, companyNames, openDeals, nil
}

func (s *CRMSummaryService) loadDealAssociations(ctx context.Context, workspaceID, dealID string) ([]contactSummarySnapshot, []companySummarySnapshot, error) {
	assocs, err := s.associationRepo.ListByObject(ctx, workspaceID, model.CRMObjectDeal, dealID)
	if err != nil {
		return nil, nil, err
	}

	contacts := make([]contactSummarySnapshot, 0, 6)
	companies := make([]companySummarySnapshot, 0, 4)
	seenContacts := map[string]struct{}{}
	seenCompanies := map[string]struct{}{}

	for _, assoc := range assocs {
		otherType, otherID := associationPeer(assoc, model.CRMObjectDeal, dealID)
		switch otherType {
		case model.CRMObjectContact:
			if _, exists := seenContacts[otherID]; exists {
				continue
			}
			seenContacts[otherID] = struct{}{}
			contact, err := s.contactRepo.GetByID(ctx, otherID)
			if err != nil || contact == nil {
				continue
			}
			contacts = append(contacts, contactSummarySnapshot{
				ID:             contact.ID,
				Name:           contactDisplayName(*contact),
				Email:          stringValue(contact.Email),
				JobTitle:       stringValue(contact.JobTitle),
				LifecycleStage: contact.LifecycleStage,
				LeadStatus:     contact.LeadStatus,
			})
		case model.CRMObjectCompany:
			if _, exists := seenCompanies[otherID]; exists {
				continue
			}
			seenCompanies[otherID] = struct{}{}
			company, err := s.companyRepo.GetByID(ctx, otherID)
			if err != nil || company == nil {
				continue
			}
			companies = append(companies, companySnapshot(*company))
		}
	}
	return contacts, companies, nil
}

func (s *CRMSummaryService) generateSummaryLLM(
	ctx context.Context,
	metering AIUsageMeteringContext,
	payload summaryPromptEntity,
) (summaryGenerationOutput, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("marshal crm summary payload: %w", err)
	}

	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID: metering.WorkspaceID, FeatureKey: metering.FeatureKey,
		IdempotencyKey: metering.IdempotencyKey, Metadata: metering.Metadata,
		RequireComplete: true, RetryInvalidOutput: true,
		ValidateResponse: func(response *llm.ChatResponse) error {
			if response == nil || strings.TrimSpace(response.Content) == "" {
				return fmt.Errorf("CRM summary model returned an empty response")
			}
			output, parseErr := parseSummaryGenerationOutput(response.Content)
			if parseErr != nil {
				return fmt.Errorf("CRM summary model returned invalid JSON: %w", parseErr)
			}
			values := []string{output.SummaryMarkdown}
			for _, highlight := range output.Highlights {
				values = append(values, highlight.Text)
			}
			if languageErr := validateEnglishCRMNarrative(values...); languageErr != nil {
				return languageErr
			}
			return nil
		},
		Chat: llm.ChatRequest{
			SystemPrompt: crmSummarySystemPrompt,
			Messages: []llm.Message{
				{Role: "user", Content: string(payloadJSON)},
			},
			Temperature: 0.1,
			MaxTokens:   1400,
			JSONMode:    true,
		},
	})
	if err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("LLM crm summary generation: %w", err)
	}

	output, err := parseSummaryGenerationOutput(resp.Content)
	if err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("CRM summary could not be read after retry: %w", err)
	}

	output.SummaryMarkdown = normalizeSummaryMarkdown(output.SummaryMarkdown)
	output.Highlights = normalizeSummaryHighlights(output.Highlights)
	if output.SummaryMarkdown == "" {
		return summaryGenerationOutput{}, fmt.Errorf("crm summary response did not include summary_markdown")
	}
	return output, nil
}

func parseSummaryGenerationOutput(content string) (summaryGenerationOutput, error) {
	if strings.TrimSpace(content) == "" {
		return summaryGenerationOutput{}, fmt.Errorf("empty model response")
	}
	var output summaryGenerationOutput
	if err := llm.UnmarshalResponse(content, &output); err == nil && strings.TrimSpace(output.SummaryMarkdown) != "" {
		return output, nil
	}
	var wrapper struct {
		Summary summaryGenerationOutput `json:"summary"`
	}
	if err := llm.UnmarshalResponse(content, &wrapper); err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("invalid JSON response")
	}
	if strings.TrimSpace(wrapper.Summary.SummaryMarkdown) == "" {
		return summaryGenerationOutput{}, fmt.Errorf("response did not include summary_markdown")
	}
	return wrapper.Summary, nil
}

const crmSummarySystemPrompt = `You are generating durable CRM intelligence summaries from stored CRM data.

You will receive a JSON object for a contact, company, or deal.

Goals:
- summarize current momentum and recent changes
- highlight notable CRM signals, risks, and likely next step
- stay grounded in the provided CRM evidence only
- for companies, synthesize the account state across stakeholders, opportunities, product work, support, meetings, and communication

Output rules:
- return JSON only
- write summary_markdown and every highlight in English, even when source material uses another language
- fields:
  - summary_markdown: short markdown summary, usually 1-3 short paragraphs with optional bullet list
  - highlights: array of { kind, text }
- allowed highlight kinds:
  - momentum
  - risk
  - next_step
  - stakeholder
  - signal
- do not invent CRM state changes
- do not say something happened unless it is supported by the provided inputs
- do not copy long verbatim quotes from emails
- do not include more than 5 highlights
- keep each highlight text concise

If there is little recent activity, say so plainly and summarize the current known state from the record and linked context.
`

func companySnapshot(company model.CRMCompany) companySummarySnapshot {
	return companySummarySnapshot{
		ID: company.ID, Name: company.Name, Domain: stringValue(company.Domain), Industry: stringValue(company.Industry),
		Description: stringValue(company.Description), Headquarters: stringValue(company.Headquarters),
		EmployeeCount: company.EmployeeCount, AnnualRevenue: company.AnnualRevenue,
	}
}

func dealSnapshot(deal model.CRMDeal) *dealSummarySnapshot {
	var closeDate *string
	if deal.CloseDate != nil {
		value := deal.CloseDate.UTC().Format(time.RFC3339)
		closeDate = &value
	}
	stage := ""
	stageType := ""
	if deal.Stage != nil {
		stage = deal.Stage.Name
		stageType = deal.Stage.StageType
	}
	return &dealSummarySnapshot{
		ID:          deal.ID,
		Name:        deal.Name,
		Stage:       stage,
		StageType:   stageType,
		Amount:      deal.Amount,
		Currency:    deal.Currency,
		Probability: deal.Probability,
		CloseDate:   closeDate,
	}
}

func associationPeer(assoc model.CRMAssociation, currentType, currentID string) (string, string) {
	if assoc.FromObjectType == currentType && assoc.FromObjectID == currentID {
		return assoc.ToObjectType, assoc.ToObjectID
	}
	return assoc.FromObjectType, assoc.FromObjectID
}

func contactDisplayName(contact model.CRMContact) string {
	name := strings.TrimSpace(contact.FirstName + " " + stringValue(contact.LastName))
	if name != "" {
		return name
	}
	if contact.Email != nil && *contact.Email != "" {
		return *contact.Email
	}
	return "Unknown contact"
}

func summaryBody(bodyText, bodyHTML *string) string {
	value := strings.TrimSpace(stringValue(bodyText))
	if value == "" {
		value = strings.TrimSpace(stringValue(bodyHTML))
		value = crmSummaryHTMLTagRe.ReplaceAllString(value, " ")
		value = html.UnescapeString(value)
	}
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > crmSummaryMaxBodyChars {
		value = value[:crmSummaryMaxBodyChars]
	}
	return strings.TrimSpace(value)
}

func parseAddressJSONArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var addresses []string
	if err := json.Unmarshal(raw, &addresses); err != nil {
		return nil
	}
	return addresses
}

func boundsFromTimes(values []time.Time) (*time.Time, *time.Time) {
	if len(values) == 0 {
		return nil, nil
	}
	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value.Before(minValue) {
			minValue = value
		}
		if value.After(maxValue) {
			maxValue = value
		}
	}
	minUTC := minValue.UTC()
	maxUTC := maxValue.UTC()
	return &minUTC, &maxUTC
}

func normalizeSummaryMarkdown(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > crmSummaryMaxMarkdownChars {
		value = value[:crmSummaryMaxMarkdownChars]
	}
	return strings.TrimSpace(value)
}

func normalizeSummaryHighlights(highlights model.CRMSummaryHighlights) model.CRMSummaryHighlights {
	allowedKinds := []string{
		model.CRMSummaryHighlightMomentum,
		model.CRMSummaryHighlightRisk,
		model.CRMSummaryHighlightNextStep,
		model.CRMSummaryHighlightStakeholder,
		model.CRMSummaryHighlightSignal,
	}
	normalized := make(model.CRMSummaryHighlights, 0, len(highlights))
	for _, highlight := range highlights {
		kind := strings.TrimSpace(highlight.Kind)
		text := strings.TrimSpace(highlight.Text)
		if !slices.Contains(allowedKinds, kind) || text == "" {
			continue
		}
		if len(text) > crmSummaryMaxHighlightChars {
			text = text[:crmSummaryMaxHighlightChars]
		}
		normalized = append(normalized, model.CRMSummaryHighlight{Kind: kind, Text: text})
		if len(normalized) == 5 {
			break
		}
	}
	return normalized
}

func summaryWorkflowID(entityType, entityID string) string {
	switch entityType {
	case model.CRMObjectContact:
		return "crm-contact-summary-" + entityID
	case model.CRMObjectDeal:
		return "crm-deal-summary-" + entityID
	default:
		return "crm-summary-" + entityType + "-" + entityID
	}
}

func intValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
