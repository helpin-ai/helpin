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
	crmSummaryContactEmailWindowDays = 45
	crmSummarySignalWindowDays       = 60
	crmSummaryTouchedContactDays     = 30
	crmSummaryGenerationVersion      = "phase1b"
	crmSummaryMaxBodyChars           = 1200
	crmSummaryMaxMarkdownChars       = 2200
	crmSummaryMaxHighlightChars      = 220
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
	summaryRepo     *repository.CRMSummaryRepository
	contactRepo     *repository.CRMContactRepository
	companyRepo     *repository.CRMCompanyRepository
	dealRepo        *repository.CRMDealRepository
	associationRepo *repository.CRMAssociationRepository
	signalRepo      *repository.CRMSignalRepository
	emailRepo       *repository.CRMEmailRepository
	llmProvider     llm.Provider
	runner          summaryWorkflowRunner
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

func (s *CRMSummaryService) getSummary(ctx context.Context, workspaceID, entityType, entityID string) (*model.CRMEntitySummary, error) {
	if workspaceID == "" || entityID == "" {
		return nil, fmt.Errorf("workspace_id and entity_id are required")
	}
	return s.summaryRepo.GetByEntity(ctx, workspaceID, entityType, entityID)
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

// RunDailyReconciliation requests refreshes for open deals and recently touched contacts.
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

	requestedAt := current.LastTriggeredAt.UTC()

	var generated summaryGenerationOutput
	var sourceWindowStart *time.Time
	var sourceWindowEnd *time.Time
	var metadata model.JSONB

	switch input.EntityType {
	case model.CRMObjectContact:
		generated, sourceWindowStart, sourceWindowEnd, metadata, err = s.generateContactSummary(ctx, input.WorkspaceID, input.EntityID)
	case model.CRMObjectDeal:
		generated, sourceWindowStart, sourceWindowEnd, metadata, err = s.generateDealSummary(ctx, input.WorkspaceID, input.EntityID)
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
	Deal           *dealSummarySnapshot     `json:"deal,omitempty"`
	Companies      []companySummarySnapshot `json:"companies,omitempty"`
	OpenDeals      []dealSummarySnapshot    `json:"open_deals,omitempty"`
	LinkedContacts []contactSummarySnapshot `json:"linked_contacts,omitempty"`
	RecentEmails   []summaryEmailSnippet    `json:"recent_emails,omitempty"`
	BuyerSignals   []summarySignalSnippet   `json:"buyer_signals,omitempty"`
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
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
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
		Companies:    companies,
		OpenDeals:    openDeals,
		RecentEmails: messages,
		BuyerSignals: signals,
	}

	output, err := s.generateSummaryLLM(ctx, payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	return output, sourceWindowStart, sourceWindowEnd, model.JSONB{
		"source_email_count":  len(messages),
		"source_signal_count": len(signals),
		"generation_version":  crmSummaryGenerationVersion,
	}, nil
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
		BuyerSignals:   signals,
	}

	output, err := s.generateSummaryLLM(ctx, payload)
	if err != nil {
		return summaryGenerationOutput{}, nil, nil, nil, err
	}

	return output, sourceWindowStart, sourceWindowEnd, model.JSONB{
		"source_email_count":  len(messages),
		"source_signal_count": len(signals),
		"generation_version":  crmSummaryGenerationVersion,
	}, nil
}

func (s *CRMSummaryService) loadSummaryEvidence(ctx context.Context, workspaceID, entityType, entityID string) ([]summaryEmailSnippet, []summarySignalSnippet, *time.Time, *time.Time, error) {
	now := time.Now().UTC()
	emailSince := now.AddDate(0, 0, -crmSummaryContactEmailWindowDays)
	emailLimit := crmSummaryContactEmailLimit
	signalLimit := crmSummaryContactSignalLimit
	emailFilters := model.CRMEmailMessageListFilters{}
	signalFilters := model.CRMBuyerSignalListFilters{}
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

	for _, assoc := range assocs {
		otherType, otherID := associationPeer(assoc, model.CRMObjectContact, contactID)
		switch otherType {
		case model.CRMObjectCompany:
			if _, exists := seenCompanies[otherID]; exists {
				continue
			}
			seenCompanies[otherID] = struct{}{}
			company, err := s.companyRepo.GetByID(ctx, otherID)
			if err != nil || company == nil {
				continue
			}
			companySnaps = append(companySnaps, companySnapshot(*company))
			companyNames = append(companyNames, company.Name)
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

func (s *CRMSummaryService) generateSummaryLLM(ctx context.Context, payload summaryPromptEntity) (summaryGenerationOutput, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("marshal crm summary payload: %w", err)
	}

	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: crmSummarySystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: string(payloadJSON)},
		},
		Temperature: 0.1,
		MaxTokens:   1400,
		JSONMode:    true,
	})
	if err != nil {
		return summaryGenerationOutput{}, fmt.Errorf("LLM crm summary generation: %w", err)
	}

	var output summaryGenerationOutput
	if err := json.Unmarshal([]byte(resp.Content), &output); err != nil {
		var wrapper struct {
			Summary summaryGenerationOutput `json:"summary"`
		}
		if err2 := json.Unmarshal([]byte(resp.Content), &wrapper); err2 != nil {
			slog.Error("failed to parse crm summary response", "error", err, "content", resp.Content)
			return summaryGenerationOutput{}, fmt.Errorf("parse crm summary response: %w", err)
		}
		output = wrapper.Summary
	}

	output.SummaryMarkdown = normalizeSummaryMarkdown(output.SummaryMarkdown)
	output.Highlights = normalizeSummaryHighlights(output.Highlights)
	if output.SummaryMarkdown == "" {
		return summaryGenerationOutput{}, fmt.Errorf("crm summary response did not include summary_markdown")
	}
	return output, nil
}

const crmSummarySystemPrompt = `You are generating durable CRM intelligence summaries from stored CRM data.

You will receive a JSON object for either a contact or a deal.

Goals:
- summarize current momentum and recent changes
- highlight notable buyer signals, risks, and likely next step
- stay grounded in the provided CRM evidence only

Output rules:
- return JSON only
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
		ID:     company.ID,
		Name:   company.Name,
		Domain: stringValue(company.Domain),
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
