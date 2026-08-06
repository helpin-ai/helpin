package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerCRMOperationalCommands() {
	definitions := []InternalCommandDefinition{
		{Name: "crm.get_contact", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_contact"}, Tool: mustCommandToolMetadata("crm.get_contact"), Execute: s.executeCRMGetContact},
		{Name: "crm.get_company", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_company"}, Tool: mustCommandToolMetadata("crm.get_company"), Execute: s.executeCRMGetCompany},
		{Name: "crm.get_deal", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_deal"}, Tool: mustCommandToolMetadata("crm.get_deal"), Execute: s.executeCRMGetDeal},
		{Name: "crm.list_companies", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_company", "crm_contact", "crm_deal"}, Tool: mustCommandToolMetadata("crm.list_companies"), Execute: s.executeCRMListCompanies},
		{Name: "crm.list_pipelines", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_deal"}, Tool: mustCommandToolMetadata("crm.list_pipelines"), Execute: s.executeCRMListPipelines},
		{Name: "crm.update_contact", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_contact"}, Tool: mustCommandToolMetadata("crm.update_contact"), Execute: s.executeCRMUpdateContact},
		{Name: "crm.update_company", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_company"}, Tool: mustCommandToolMetadata("crm.update_company"), Execute: s.executeCRMUpdateCompany},
		{Name: "crm.update_deal", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_deal"}, Tool: mustCommandToolMetadata("crm.update_deal"), Execute: s.executeCRMUpdateDeal},
		{Name: "crm.add_activity", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_contact", "crm_company", "crm_deal"}, Tool: mustCommandToolMetadata("crm.add_activity"), Execute: s.executeCRMAddActivity},
	}
	for _, def := range definitions {
		s.register(def)
	}
	s.registerCRMAssociationCommands()
}

func (s *InternalCommandService) executeCRMGetContact(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ContactID string `json:"contact_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse contact input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.ContactID, "crm_contact", "contact_id")
	if err != nil {
		return nil, err
	}
	contact, err := s.scopedCRMContact(ctx, meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMContact(contact)), nil
}

func (s *InternalCommandService) executeCRMGetCompany(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		CompanyID string `json:"company_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse company input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.CompanyID, "crm_company", "company_id")
	if err != nil {
		return nil, err
	}
	company, err := s.scopedCRMCompany(ctx, meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMCompany(company)), nil
}

func (s *InternalCommandService) executeCRMGetDeal(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		DealID string `json:"deal_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse deal input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.DealID, "crm_deal", "deal_id")
	if err != nil {
		return nil, err
	}
	deal, err := s.scopedCRMDeal(ctx, meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMDeal(deal)), nil
}

func (s *InternalCommandService) executeCRMListCompanies(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmCompanyService == nil {
		return nil, fmt.Errorf("CRM company service is not configured")
	}
	var req struct {
		Query         string  `json:"query"`
		OwnerMemberID *string `json:"owner_member_id"`
		Limit         int     `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse company list input: %w", err)
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	items, total, err := s.crmCompanyService.List(ctx, meta.WorkspaceID, model.CRMCompanyListFilters{Search: stringPtrOrNil(req.Query), OwnerMemberID: normalizeOptionalCommandString(req.OwnerMemberID)}, model.PMPagination{Page: 1, PerPage: limit})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, compactCRMCompany(&items[i]))
	}
	return mustJSON(map[string]any{"companies": result, "total": total}), nil
}

func (s *InternalCommandService) executeCRMListPipelines(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmDealService == nil {
		return nil, fmt.Errorf("CRM deal service is not configured")
	}
	var req struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse pipeline list input: %w", err)
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	pipelines, err := s.crmDealService.ListPipelines(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if len(pipelines) > limit {
		pipelines = pipelines[:limit]
	}
	return mustJSON(map[string]any{"pipelines": pipelines}), nil
}

type crmContactUpdateCommandInput struct {
	ContactID      string    `json:"contact_id"`
	LifecycleStage *string   `json:"lifecycle_stage"`
	LeadStatus     *string   `json:"lead_status"`
	OwnerMemberID  *string   `json:"owner_member_id"`
	ClearOwner     bool      `json:"clear_owner"`
	Labels         *[]string `json:"labels"`
}

func (s *InternalCommandService) executeCRMUpdateContact(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req crmContactUpdateCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse contact update input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.ContactID, "crm_contact", "contact_id")
	if err != nil {
		return nil, err
	}
	if _, err := s.scopedCRMContact(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if req.ClearOwner && req.OwnerMemberID != nil {
		return nil, fmt.Errorf("clear_owner cannot be combined with owner_member_id")
	}
	owner, err := s.validateCRMOwnerMember(ctx, meta.WorkspaceID, req.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	if req.LifecycleStage != nil && !validCRMLifecycleStage(*req.LifecycleStage) {
		return nil, fmt.Errorf("invalid lifecycle_stage")
	}
	if req.LeadStatus != nil && !validCRMLeadStatus(*req.LeadStatus) {
		return nil, fmt.Errorf("invalid lead_status")
	}
	var labels []string
	if req.Labels != nil {
		labels = normalizeDocsMetadataTags(*req.Labels)
	}
	updated, err := s.crmContactService.Update(ctx, id, model.UpdateCRMContactRequest{LifecycleStage: req.LifecycleStage, LeadStatus: req.LeadStatus, OwnerMemberID: owner, ClearOwner: req.ClearOwner, Labels: labels})
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMContact(updated)), nil
}

func (s *InternalCommandService) executeCRMUpdateCompany(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		CompanyID     string  `json:"company_id"`
		OwnerMemberID *string `json:"owner_member_id"`
		ClearOwner    bool    `json:"clear_owner"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse company update input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.CompanyID, "crm_company", "company_id")
	if err != nil {
		return nil, err
	}
	if _, err := s.scopedCRMCompany(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if req.ClearOwner && req.OwnerMemberID != nil {
		return nil, fmt.Errorf("clear_owner cannot be combined with owner_member_id")
	}
	owner, err := s.validateCRMOwnerMember(ctx, meta.WorkspaceID, req.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	updated, err := s.crmCompanyService.Update(ctx, id, model.UpdateCRMCompanyRequest{OwnerMemberID: owner, ClearOwner: req.ClearOwner})
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMCompany(updated)), nil
}

type crmDealUpdateCommandInput struct {
	DealID           string   `json:"deal_id"`
	Name             *string  `json:"name"`
	PipelineID       *string  `json:"pipeline_id"`
	StageID          *string  `json:"stage_id"`
	Amount           *float64 `json:"amount"`
	ClearAmount      bool     `json:"clear_amount"`
	Currency         *string  `json:"currency"`
	CloseDate        *string  `json:"close_date"`
	ClearCloseDate   bool     `json:"clear_close_date"`
	OwnerMemberID    *string  `json:"owner_member_id"`
	ClearOwner       bool     `json:"clear_owner"`
	Probability      *int     `json:"probability"`
	ClearProbability bool     `json:"clear_probability"`
}

func (s *InternalCommandService) executeCRMUpdateDeal(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req crmDealUpdateCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse deal update input: %w", err)
	}
	id, err := resolveCRMCommandID(meta, req.DealID, "crm_deal", "deal_id")
	if err != nil {
		return nil, err
	}
	deal, err := s.scopedCRMDeal(ctx, meta.WorkspaceID, id)
	if err != nil {
		return nil, err
	}
	if err := validateClearPair(req.ClearAmount, req.Amount != nil, "clear_amount", "amount"); err != nil {
		return nil, err
	}
	if err := validateClearPair(req.ClearCloseDate, req.CloseDate != nil, "clear_close_date", "close_date"); err != nil {
		return nil, err
	}
	if err := validateClearPair(req.ClearOwner, req.OwnerMemberID != nil, "clear_owner", "owner_member_id"); err != nil {
		return nil, err
	}
	if err := validateClearPair(req.ClearProbability, req.Probability != nil, "clear_probability", "probability"); err != nil {
		return nil, err
	}
	owner, err := s.validateCRMOwnerMember(ctx, meta.WorkspaceID, req.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	closeDate, err := parseCRMCommandDate(req.CloseDate)
	if err != nil {
		return nil, err
	}
	if req.Amount != nil && *req.Amount < 0 {
		return nil, fmt.Errorf("amount must not be negative")
	}
	if req.Probability != nil && (*req.Probability < 0 || *req.Probability > 100) {
		return nil, fmt.Errorf("probability must be between 0 and 100")
	}
	if req.Currency != nil {
		value := strings.ToUpper(strings.TrimSpace(*req.Currency))
		if len(value) != 3 {
			return nil, fmt.Errorf("currency must be a 3-letter code")
		}
		req.Currency = &value
	}
	pipelineID := deal.PipelineID
	if req.PipelineID != nil {
		pipelineID = strings.TrimSpace(*req.PipelineID)
	}
	pipeline, err := s.crmDealService.GetPipeline(ctx, pipelineID)
	if err != nil || pipeline == nil || pipeline.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("pipeline not found")
	}
	stageID := deal.StageID
	if req.StageID != nil {
		stageID = strings.TrimSpace(*req.StageID)
	}
	stageFound := false
	for _, stage := range pipeline.Stages {
		if stage.ID == stageID {
			stageFound = true
			break
		}
	}
	if !stageFound {
		return nil, fmt.Errorf("stage not found in pipeline")
	}
	updated, err := s.crmDealService.Update(ctx, id, model.UpdateCRMDealRequest{Name: req.Name, PipelineID: req.PipelineID, StageID: req.StageID, Amount: req.Amount, ClearAmount: req.ClearAmount, Currency: req.Currency, CloseDate: closeDate, ClearCloseDate: req.ClearCloseDate, OwnerMemberID: owner, ClearOwner: req.ClearOwner, Probability: req.Probability, ClearProbability: req.ClearProbability})
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMDeal(updated)), nil
}

func (s *InternalCommandService) executeCRMAddActivity(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmActivityService == nil {
		return nil, fmt.Errorf("CRM activity service is not configured")
	}
	var req struct {
		ActivityType  string  `json:"activity_type"`
		ContactID     *string `json:"contact_id"`
		CompanyID     *string `json:"company_id"`
		DealID        *string `json:"deal_id"`
		OwnerMemberID *string `json:"owner_member_id"`
		Subject       *string `json:"subject"`
		Body          *string `json:"body"`
		OccurredAt    *string `json:"occurred_at"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse CRM activity input: %w", err)
	}
	targets := 0
	if req.ContactID != nil {
		targets++
	}
	if req.CompanyID != nil {
		targets++
	}
	if req.DealID != nil {
		targets++
	}
	if targets != 1 {
		return nil, fmt.Errorf("exactly one of contact_id, company_id, or deal_id is required")
	}
	if req.ContactID != nil {
		if _, err := s.scopedCRMContact(ctx, meta.WorkspaceID, *req.ContactID); err != nil {
			return nil, err
		}
	}
	if req.CompanyID != nil {
		if _, err := s.scopedCRMCompany(ctx, meta.WorkspaceID, *req.CompanyID); err != nil {
			return nil, err
		}
	}
	if req.DealID != nil {
		if _, err := s.scopedCRMDeal(ctx, meta.WorkspaceID, *req.DealID); err != nil {
			return nil, err
		}
	}
	owner, err := s.validateCRMOwnerMember(ctx, meta.WorkspaceID, req.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	occurredAt, err := parseCRMCommandTimestamp(req.OccurredAt)
	if err != nil {
		return nil, err
	}
	activity, err := s.crmActivityService.Create(ctx, model.CreateCRMActivityRequest{WorkspaceID: meta.WorkspaceID, ActivityType: strings.TrimSpace(req.ActivityType), ContactID: normalizeOptionalCommandString(req.ContactID), CompanyID: normalizeOptionalCommandString(req.CompanyID), DealID: normalizeOptionalCommandString(req.DealID), OwnerMemberID: owner, Subject: req.Subject, Body: req.Body, OccurredAt: occurredAt})
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"activity_id": activity.ID, "activity_type": activity.ActivityType, "contact_id": activity.ContactID, "company_id": activity.CompanyID, "deal_id": activity.DealID, "owner_member_id": activity.OwnerMemberID, "subject": activity.Subject, "body": activity.Body, "occurred_at": activity.OccurredAt}), nil
}

func (s *InternalCommandService) scopedCRMContact(ctx context.Context, workspaceID, id string) (*model.CRMContact, error) {
	if s.crmContactService == nil {
		return nil, fmt.Errorf("CRM contact service is not configured")
	}
	item, err := s.crmContactService.GetByID(ctx, strings.TrimSpace(id))
	if err != nil || item == nil || item.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("contact not found")
	}
	return item, nil
}
func (s *InternalCommandService) scopedCRMCompany(ctx context.Context, workspaceID, id string) (*model.CRMCompany, error) {
	if s.crmCompanyService == nil {
		return nil, fmt.Errorf("CRM company service is not configured")
	}
	item, err := s.crmCompanyService.GetByID(ctx, strings.TrimSpace(id))
	if err != nil || item == nil || item.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("company not found")
	}
	return item, nil
}
func (s *InternalCommandService) scopedCRMDeal(ctx context.Context, workspaceID, id string) (*model.CRMDeal, error) {
	if s.crmDealService == nil {
		return nil, fmt.Errorf("CRM deal service is not configured")
	}
	item, err := s.crmDealService.GetByID(ctx, strings.TrimSpace(id))
	if err != nil || item == nil || item.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	return item, nil
}

func (s *InternalCommandService) validateCRMOwnerMember(ctx context.Context, workspaceID string, ownerID *string) (*string, error) {
	ownerID = normalizeOptionalCommandString(ownerID)
	if ownerID == nil {
		return nil, nil
	}
	if s.workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is not configured")
	}
	membership, err := s.workspaceRepo.GetMembershipByID(ctx, workspaceID, *ownerID)
	if err != nil {
		return nil, err
	}
	if membership == nil || membership.Status != model.WorkspaceMemberStatusActive {
		return nil, fmt.Errorf("owner_member_id is not an active workspace member")
	}
	return ownerID, nil
}

func resolveCRMCommandID(meta model.InternalCommandContext, explicit, targetType, field string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if strings.TrimSpace(meta.TargetType) == targetType {
		targetID := strings.TrimSpace(meta.TargetID)
		if explicit != "" && targetID != "" && explicit != targetID {
			return "", fmt.Errorf("%s conflicts with the current %s target", field, targetType)
		}
		if explicit == "" {
			explicit = targetID
		}
	}
	if explicit == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return explicit, nil
}

func normalizeOperationalLimit(value int) (int, error) {
	if value < 0 || value > 100 {
		return 0, fmt.Errorf("limit must be between 1 and 100")
	}
	if value == 0 {
		return 50, nil
	}
	return value, nil
}
func validateClearPair(clear, supplied bool, clearField, valueField string) error {
	if clear && supplied {
		return fmt.Errorf("%s cannot be combined with %s", clearField, valueField)
	}
	return nil
}
func parseCRMCommandDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("close_date must be YYYY-MM-DD")
	}
	return &parsed, nil
}
func parseCRMCommandTimestamp(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("occurred_at must be RFC3339")
	}
	return &parsed, nil
}
func validCRMLifecycleStage(value string) bool {
	for _, allowed := range []string{model.CRMLifecycleSubscriber, model.CRMLifecycleLead, model.CRMLifecycleMarketingQualified, model.CRMLifecycleSalesQualified, model.CRMLifecycleOpportunity, model.CRMLifecycleCustomer, model.CRMLifecycleEvangelist} {
		if value == allowed {
			return true
		}
	}
	return false
}
func validCRMLeadStatus(value string) bool {
	for _, allowed := range []string{model.CRMLeadStatusNew, model.CRMLeadStatusOpen, model.CRMLeadStatusInProgress, model.CRMLeadStatusUnqualified} {
		if value == allowed {
			return true
		}
	}
	return false
}

func compactCRMContact(item *model.CRMContact) map[string]any {
	return map[string]any{"contact_id": item.ID, "display_id": item.DisplayID, "first_name": item.FirstName, "last_name": item.LastName, "email": item.Email, "lifecycle_stage": item.LifecycleStage, "lead_status": item.LeadStatus, "owner_member_id": item.OwnerMemberID, "labels": item.Labels, "updated_at": item.UpdatedAt}
}
func compactCRMCompany(item *model.CRMCompany) map[string]any {
	return map[string]any{"company_id": item.ID, "display_id": item.DisplayID, "name": item.Name, "domain": item.Domain, "owner_member_id": item.OwnerMemberID, "updated_at": item.UpdatedAt}
}
func compactCRMDeal(item *model.CRMDeal) map[string]any {
	return map[string]any{"deal_id": item.ID, "display_id": item.DisplayID, "name": item.Name, "pipeline_id": item.PipelineID, "stage_id": item.StageID, "amount": item.Amount, "currency": item.Currency, "close_date": item.CloseDate, "owner_member_id": item.OwnerMemberID, "probability": item.Probability, "updated_at": item.UpdatedAt}
}
