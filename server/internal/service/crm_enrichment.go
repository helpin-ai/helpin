package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
)

const (
	crmEnrichmentMinConfidence = 0.70
	crmEnrichmentMaxFields     = 20
)

// CRMEnrichmentService contains CRM enrichment business logic.
type CRMEnrichmentService struct {
	enrichmentRepo *repository.CRMEnrichmentRepository
	contactRepo    *repository.CRMContactRepository
	companyRepo    *repository.CRMCompanyRepository
	assocRepo      *repository.CRMAssociationRepository
	activityRepo   *repository.CRMActivityRepository
	wsPublisher    websocket.EventPublisher
}

// SetActivityRepository enables immutable CRM timeline entries for enrichment changes.
func (s *CRMEnrichmentService) SetActivityRepository(repo *repository.CRMActivityRepository) *CRMEnrichmentService {
	s.activityRepo = repo
	return s
}

// SetWebsocketPublisher enables immediate CRM cache refreshes after enrichment.
func (s *CRMEnrichmentService) SetWebsocketPublisher(publisher websocket.EventPublisher) *CRMEnrichmentService {
	s.wsPublisher = publisher
	return s
}

// NewCRMEnrichmentService creates a new CRMEnrichmentService.
func NewCRMEnrichmentService(enrichmentRepo *repository.CRMEnrichmentRepository, contactRepo *repository.CRMContactRepository, companyRepo *repository.CRMCompanyRepository, assocRepo *repository.CRMAssociationRepository) *CRMEnrichmentService {
	return &CRMEnrichmentService{enrichmentRepo: enrichmentRepo, contactRepo: contactRepo, companyRepo: companyRepo, assocRepo: assocRepo}
}

// List returns enrichment results with filters and pagination.
func (s *CRMEnrichmentService) List(ctx context.Context, workspaceID string, filters model.CRMEnrichmentListFilters, pagination model.PMPagination) ([]model.CRMEnrichmentResult, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.enrichmentRepo.List(ctx, workspaceID, filters, pagination)
}

// Create creates a new enrichment result.
func (s *CRMEnrichmentService) Create(ctx context.Context, req model.CreateCRMEnrichmentRequest) (*model.CRMEnrichmentResult, error) {
	if req.WorkspaceID == "" || req.ObjectType == "" || req.ObjectID == "" {
		return nil, fmt.Errorf("workspace_id, object_type, and object_id are required")
	}

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}

	enrichment := &model.CRMEnrichmentResult{
		WorkspaceID: req.WorkspaceID,
		ObjectType:  req.ObjectType,
		ObjectID:    req.ObjectID,
		Source:      req.Source,
		Data:        model.JSONB(req.Data),
		Confidence:  confidence,
	}

	if enrichment.Source == "" {
		enrichment.Source = model.CRMEnrichmentSourceManual
	}

	if err := s.enrichmentRepo.Create(ctx, enrichment); err != nil {
		return nil, err
	}
	return enrichment, nil
}

// Delete removes an enrichment result.
func (s *CRMEnrichmentService) Delete(ctx context.Context, id string) error {
	return s.enrichmentRepo.Delete(ctx, id)
}

// ApplySuggestion accepts one protected-value suggestion from an enrichment audit.
func (s *CRMEnrichmentService) ApplySuggestion(ctx context.Context, workspaceID, enrichmentID string, req model.ApplyCRMEnrichmentSuggestionRequest) (*model.CRMEnrichmentResult, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	enrichmentID = strings.TrimSpace(enrichmentID)
	fieldName := strings.TrimSpace(strings.ToLower(req.Field))
	if workspaceID == "" || enrichmentID == "" || fieldName == "" {
		return nil, fmt.Errorf("workspace_id, enrichment_id, and field are required")
	}
	audit, err := s.enrichmentRepo.GetByID(ctx, enrichmentID)
	if err != nil {
		return nil, err
	}
	if audit == nil || audit.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("enrichment not found")
	}

	skipped, err := enrichmentFieldResults(audit.Data["skipped"])
	if err != nil {
		return nil, err
	}
	var suggestion *model.CRMEnrichmentFieldResult
	for i := range skipped {
		if skipped[i].Field == fieldName && skipped[i].Reason == "existing_value_protected" {
			suggestion = &skipped[i]
			break
		}
	}
	if suggestion == nil {
		return nil, fmt.Errorf("reviewable suggestion not found")
	}
	requested, err := enrichmentFieldInputs(audit.Data["requested_fields"])
	if err != nil {
		return nil, err
	}
	var proposed model.CRMEnrichmentFieldInput
	for _, candidate := range requested {
		if strings.EqualFold(candidate.Field, fieldName) {
			proposed = candidate
			proposed.Value = suggestion.ProposedValue
			break
		}
	}
	if proposed.Field == "" {
		return nil, fmt.Errorf("suggestion source data is missing")
	}
	proposed, err = normalizeCRMEnrichmentField(proposed)
	if err != nil {
		return nil, err
	}

	accepted := model.CRMEnrichmentFieldResult{Field: fieldName, OldValue: suggestion.OldValue, NewValue: proposed.Value, SourceURL: proposed.SourceURL, Confidence: proposed.Confidence, Reason: "accepted_suggestion"}
	err = s.enrichmentRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch audit.ObjectType {
		case "contact":
			contact, getErr := s.contactRepo.WithTx(tx).GetByID(ctx, audit.ObjectID)
			if getErr != nil || contact == nil || contact.WorkspaceID != workspaceID {
				if getErr != nil {
					return getErr
				}
				return fmt.Errorf("contact not found")
			}
			if err := applyContactEnrichmentValue(contact, proposed); err != nil {
				return err
			}
			props := cloneJSONB(contact.CustomProperties)
			recordEnrichmentProvenance(props, proposed)
			contact.CustomProperties = props
			if err := s.contactRepo.WithTx(tx).Update(ctx, contact); err != nil {
				return err
			}
		case "company":
			company, getErr := s.companyRepo.WithTx(tx).GetByID(ctx, audit.ObjectID)
			if getErr != nil || company == nil || company.WorkspaceID != workspaceID {
				if getErr != nil {
					return getErr
				}
				return fmt.Errorf("company not found")
			}
			if err := applyCompanyEnrichmentValue(company, proposed); err != nil {
				return err
			}
			props := cloneJSONB(company.CustomProperties)
			recordEnrichmentProvenance(props, proposed)
			company.CustomProperties = props
			if err := s.companyRepo.WithTx(tx).Update(ctx, company); err != nil {
				return err
			}
		default:
			return fmt.Errorf("enrichment suggestions are unsupported for %s", audit.ObjectType)
		}
		if s.activityRepo != nil {
			if err := s.activityRepo.WithTx(tx).Create(ctx, enrichmentActivity(workspaceID, audit.ObjectType, audit.ObjectID, req.ActorUserID, []model.CRMEnrichmentFieldResult{accepted})); err != nil {
				return err
			}
		}
		for i := range skipped {
			if skipped[i].Field == fieldName && skipped[i].Reason == "existing_value_protected" {
				skipped[i].Reason = "accepted_suggestion"
				skipped[i].NewValue = proposed.Value
			}
		}
		audit.Data["skipped"] = skipped
		hasPendingSuggestion := false
		for _, item := range skipped {
			if item.Reason == "existing_value_protected" {
				hasPendingSuggestion = true
				break
			}
		}
		if !hasPendingSuggestion {
			audit.Data["status"] = "applied"
		}
		acceptedList, _ := enrichmentFieldResults(audit.Data["accepted_suggestions"])
		audit.Data["accepted_suggestions"] = append(acceptedList, accepted)
		return s.enrichmentRepo.WithTx(tx).Update(ctx, audit)
	})
	if err != nil {
		return nil, err
	}
	entity := "crm_contact"
	if audit.ObjectType == "company" {
		entity = "crm_company"
	}
	s.publishCRMEnrichment(entity, workspaceID, audit.ObjectID, req.ActorUserID)
	return audit, nil
}

// EnrichContact applies guarded, fill-only agent enrichment to a CRM contact.
func (s *CRMEnrichmentService) EnrichContact(ctx context.Context, workspaceID string, req model.EnrichCRMContactRequest) (*model.CRMEnrichmentApplyResult, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	req.ContactID = strings.TrimSpace(req.ContactID)
	if workspaceID == "" || req.ContactID == "" {
		return nil, fmt.Errorf("workspace_id and contact_id are required")
	}
	if s.contactRepo == nil || s.enrichmentRepo == nil {
		return nil, fmt.Errorf("CRM enrichment dependencies are not configured")
	}
	if err := validateCRMEnrichmentFields(req.Fields, allowedContactEnrichmentFields()); err != nil {
		return nil, err
	}

	contact, err := s.contactRepo.GetByID(ctx, req.ContactID)
	if err != nil {
		return nil, err
	}
	if contact == nil || contact.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("contact not found")
	}

	applied := make([]model.CRMEnrichmentFieldResult, 0, len(req.Fields))
	skipped := make([]model.CRMEnrichmentFieldResult, 0, len(req.Fields))
	props := cloneJSONB(contact.CustomProperties)

	for _, field := range req.Fields {
		normalized, err := normalizeCRMEnrichmentField(field)
		if err != nil {
			return nil, err
		}
		switch normalized.Field {
		case "email":
			result, ok := fillStringField(normalized, contact.Email)
			if ok && !req.DryRun {
				contact.Email = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "phone":
			result, ok := fillStringField(normalized, contact.Phone)
			if ok && !req.DryRun {
				contact.Phone = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "job_title":
			result, ok := fillStringField(normalized, contact.JobTitle)
			if ok && !req.DryRun {
				contact.JobTitle = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "avatar_url":
			result, ok := fillStringField(normalized, contact.AvatarURL)
			if ok && !req.DryRun {
				contact.AvatarURL = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "linkedin_url":
			result, ok := fillStringField(normalized, contact.LinkedInURL)
			if ok && !req.DryRun {
				contact.LinkedInURL = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "location":
			result, ok := fillStringField(normalized, contact.PrimaryLocation)
			if ok && !req.DryRun {
				contact.PrimaryLocation = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "enrichment_note":
			result := appendAgentEnrichmentNote(props, normalized)
			applied = append(applied, result)
		default:
			return nil, fmt.Errorf("field %q is not allowed for contact enrichment", normalized.Field)
		}
	}
	recordAppliedEnrichmentProvenance(props, req.Fields, applied)

	result := crmEnrichmentApplyResult("contact", contact.ID, applied, skipped, req.DryRun)
	if req.DryRun {
		return result, nil
	}
	contact.CustomProperties = props
	enrichment := guardedEnrichmentAudit(workspaceID, "contact", contact.ID, req.EvidenceSummary, req.Fields, result)
	err = s.enrichmentRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(applied) > 0 {
			if err := s.contactRepo.WithTx(tx).Update(ctx, contact); err != nil {
				return err
			}
			if s.activityRepo != nil {
				if err := s.activityRepo.WithTx(tx).Create(ctx, enrichmentActivity(workspaceID, "contact", contact.ID, req.ActorUserID, applied)); err != nil {
					return err
				}
			}
		}
		return s.enrichmentRepo.WithTx(tx).Create(ctx, enrichment)
	})
	if err != nil {
		return nil, err
	}
	result.EnrichmentResultID = enrichment.ID
	s.publishCRMEnrichment("crm_contact", workspaceID, contact.ID, req.ActorUserID)
	return result, nil
}

// EnrichCompany applies guarded, fill-only agent enrichment to a CRM company.
func (s *CRMEnrichmentService) EnrichCompany(ctx context.Context, workspaceID string, req model.EnrichCRMCompanyRequest) (*model.CRMEnrichmentApplyResult, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	if workspaceID == "" || req.CompanyID == "" {
		return nil, fmt.Errorf("workspace_id and company_id are required")
	}
	if s.companyRepo == nil || s.enrichmentRepo == nil {
		return nil, fmt.Errorf("CRM enrichment dependencies are not configured")
	}
	if err := validateCRMEnrichmentFields(req.Fields, allowedCompanyEnrichmentFields()); err != nil {
		return nil, err
	}

	company, err := s.companyRepo.GetByID(ctx, req.CompanyID)
	if err != nil {
		return nil, err
	}
	if company == nil || company.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("company not found")
	}

	applied := make([]model.CRMEnrichmentFieldResult, 0, len(req.Fields))
	skipped := make([]model.CRMEnrichmentFieldResult, 0, len(req.Fields))
	props := cloneJSONB(company.CustomProperties)

	for _, field := range req.Fields {
		normalized, err := normalizeCRMEnrichmentField(field)
		if err != nil {
			return nil, err
		}
		switch normalized.Field {
		case "domain":
			result, ok := fillStringField(normalized, company.Domain)
			if ok && !req.DryRun {
				company.Domain = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "industry":
			result, ok := fillStringField(normalized, company.Industry)
			if ok && !req.DryRun {
				company.Industry = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "employee_count":
			value, err := crmIntValue(normalized.Value)
			if err != nil {
				return nil, fmt.Errorf("employee_count %w", err)
			}
			normalized.Value = value
			result, ok := fillIntField(normalized, company.EmployeeCount)
			if ok && !req.DryRun {
				company.EmployeeCount = &value
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "annual_revenue":
			value, err := crmFloatValue(normalized.Value)
			if err != nil {
				return nil, fmt.Errorf("annual_revenue %w", err)
			}
			normalized.Value = value
			result, ok := fillFloatField(normalized, company.AnnualRevenue)
			if ok && !req.DryRun {
				company.AnnualRevenue = &value
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "description":
			result, ok := fillStringField(normalized, company.Description)
			if ok && !req.DryRun {
				company.Description = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "logo_url":
			result, ok := fillStringField(normalized, company.LogoURL)
			if ok && !req.DryRun {
				company.LogoURL = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "linkedin_url":
			result, ok := fillStringField(normalized, company.LinkedInURL)
			if ok && !req.DryRun {
				company.LinkedInURL = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "headquarters":
			result, ok := fillStringField(normalized, company.Headquarters)
			if ok && !req.DryRun {
				company.Headquarters = stringPtr(result.NewValue.(string))
			}
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "enrichment_note":
			result := appendAgentEnrichmentNote(props, normalized)
			applied = append(applied, result)
		default:
			return nil, fmt.Errorf("field %q is not allowed for company enrichment", normalized.Field)
		}
	}
	recordAppliedEnrichmentProvenance(props, req.Fields, applied)

	result := crmEnrichmentApplyResult("company", company.ID, applied, skipped, req.DryRun)
	if req.DryRun {
		return result, nil
	}
	company.CustomProperties = props
	enrichment := guardedEnrichmentAudit(workspaceID, "company", company.ID, req.EvidenceSummary, req.Fields, result)
	err = s.enrichmentRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(applied) > 0 {
			if err := s.companyRepo.WithTx(tx).Update(ctx, company); err != nil {
				return err
			}
			if s.activityRepo != nil {
				if err := s.activityRepo.WithTx(tx).Create(ctx, enrichmentActivity(workspaceID, "company", company.ID, req.ActorUserID, applied)); err != nil {
					return err
				}
			}
		}
		return s.enrichmentRepo.WithTx(tx).Create(ctx, enrichment)
	})
	if err != nil {
		return nil, err
	}
	result.EnrichmentResultID = enrichment.ID
	s.publishCRMEnrichment("crm_company", workspaceID, company.ID, req.ActorUserID)
	return result, nil
}

// EnsureContactCompany creates or reuses a company and links it to a contact.
func (s *CRMEnrichmentService) EnsureContactCompany(ctx context.Context, workspaceID string, req model.EnsureCRMContactCompanyRequest) (*model.EnsureCRMContactCompanyResult, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	req.ContactID = strings.TrimSpace(req.ContactID)
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.SourceURL = strings.TrimSpace(req.SourceURL)
	req.Evidence = strings.TrimSpace(req.Evidence)
	domain := normalizeCRMDomain(req.Domain)
	if workspaceID == "" || req.ContactID == "" || req.CompanyName == "" {
		return nil, fmt.Errorf("workspace_id, contact_id, and company_name are required")
	}
	if s.contactRepo == nil || s.companyRepo == nil || s.assocRepo == nil || s.enrichmentRepo == nil {
		return nil, fmt.Errorf("CRM company association dependencies are not configured")
	}
	if req.Confidence < crmEnrichmentMinConfidence {
		return nil, fmt.Errorf("confidence must be at least %.2f", crmEnrichmentMinConfidence)
	}
	if _, err := validateHTTPURL(req.SourceURL); err != nil {
		return nil, fmt.Errorf("source_url %w", err)
	}
	if req.Evidence == "" {
		return nil, fmt.Errorf("evidence is required")
	}

	contact, err := s.contactRepo.GetByID(ctx, req.ContactID)
	if err != nil {
		return nil, err
	}
	if contact == nil || contact.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("contact not found")
	}

	company, err := s.findReusableCompany(ctx, workspaceID, req.CompanyName, domain)
	if err != nil {
		return nil, err
	}

	label := primaryCompanyAssociationLabel
	if req.AssociationLabel != nil && strings.TrimSpace(*req.AssociationLabel) != "" {
		label = strings.TrimSpace(*req.AssociationLabel)
	}
	result := &model.EnsureCRMContactCompanyResult{
		ContactID:        contact.ID,
		CompanyName:      req.CompanyName,
		Domain:           domain,
		AssociationLabel: label,
		DryRun:           req.DryRun,
	}

	if req.DryRun {
		result.Status = "dry_run"
		if company != nil {
			result.CompanyID = company.ID
			result.CompanyName = company.Name
			result.CreatedCompany = false
			existing, err := s.contactCompanyAssociationExists(ctx, workspaceID, contact.ID, company.ID)
			if err != nil {
				return nil, err
			}
			result.CreatedLink = !existing
		} else {
			result.CreatedCompany = true
			result.CreatedLink = true
		}
		return result, nil
	}

	if company == nil {
		displayID, err := s.companyRepo.GetNextDisplayID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		company = &model.CRMCompany{
			WorkspaceID:      workspaceID,
			DisplayID:        displayID,
			Name:             req.CompanyName,
			CustomProperties: model.JSONB{},
		}
		if domain != "" {
			company.Domain = &domain
		}
		if err := s.companyRepo.Create(ctx, company); err != nil {
			return nil, err
		}
		result.CreatedCompany = true
	}
	result.CompanyID = company.ID
	result.CompanyName = company.Name
	if company.Domain != nil && strings.TrimSpace(*company.Domain) != "" {
		result.Domain = strings.TrimSpace(*company.Domain)
	}

	alreadyLinked, err := s.contactCompanyAssociationExists(ctx, workspaceID, contact.ID, company.ID)
	if err != nil {
		return nil, err
	}
	assocLabel := label
	assoc, err := (&CRMAssociationService{assocRepo: s.assocRepo}).Create(ctx, model.CreateCRMAssociationRequest{
		WorkspaceID:      workspaceID,
		FromObjectType:   model.CRMObjectContact,
		FromObjectID:     contact.ID,
		ToObjectType:     model.CRMObjectCompany,
		ToObjectID:       company.ID,
		AssociationLabel: &assocLabel,
	})
	if err != nil {
		return nil, err
	}
	result.AssociationID = assoc.ID
	result.CreatedLink = !alreadyLinked
	result.Status = "linked"
	if result.CreatedCompany {
		result.Status = "created"
	} else if alreadyLinked {
		result.Status = "already_linked"
	}
	if _, err := s.createEnsureCompanyAudit(ctx, workspaceID, req, result); err != nil {
		return nil, err
	}
	return result, nil
}

func allowedContactEnrichmentFields() map[string]bool {
	return map[string]bool{
		"email":           true,
		"phone":           true,
		"job_title":       true,
		"avatar_url":      true,
		"linkedin_url":    true,
		"location":        true,
		"enrichment_note": true,
	}
}

func (s *CRMEnrichmentService) findReusableCompany(ctx context.Context, workspaceID, name, domain string) (*model.CRMCompany, error) {
	if domain != "" {
		company, err := s.companyRepo.GetByDomain(ctx, workspaceID, domain)
		if err != nil {
			return nil, err
		}
		if company != nil {
			return company, nil
		}
	}
	return s.companyRepo.GetByName(ctx, workspaceID, name)
}

func normalizeCRMDomain(domain *string) string {
	if domain == nil {
		return ""
	}
	value := strings.TrimSpace(strings.ToLower(*domain))
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "www.")
	if slash := strings.Index(value, "/"); slash >= 0 {
		value = value[:slash]
	}
	return strings.TrimSpace(value)
}

func (s *CRMEnrichmentService) contactCompanyAssociationExists(ctx context.Context, workspaceID, contactID, companyID string) (bool, error) {
	assocs, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return false, err
	}
	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, model.CRMObjectContact, contactID)
		if otherType == model.CRMObjectCompany && otherID == companyID {
			return true, nil
		}
	}
	return false, nil
}

func allowedCompanyEnrichmentFields() map[string]bool {
	return map[string]bool{
		"domain":          true,
		"industry":        true,
		"employee_count":  true,
		"annual_revenue":  true,
		"description":     true,
		"logo_url":        true,
		"linkedin_url":    true,
		"headquarters":    true,
		"enrichment_note": true,
	}
}

func validateCRMEnrichmentFields(fields []model.CRMEnrichmentFieldInput, allowed map[string]bool) error {
	if len(fields) == 0 {
		return fmt.Errorf("fields is required")
	}
	if len(fields) > crmEnrichmentMaxFields {
		return fmt.Errorf("fields cannot exceed %d", crmEnrichmentMaxFields)
	}
	for _, field := range fields {
		name := strings.TrimSpace(strings.ToLower(field.Field))
		if name == "" {
			return fmt.Errorf("field is required")
		}
		if !allowed[name] {
			return fmt.Errorf("field %q is not allowed for guarded CRM enrichment", name)
		}
		if field.Confidence < crmEnrichmentMinConfidence {
			return fmt.Errorf("field %q confidence must be at least %.2f", name, crmEnrichmentMinConfidence)
		}
		if strings.TrimSpace(field.SourceURL) == "" {
			return fmt.Errorf("field %q source_url is required", name)
		}
		if _, err := validateHTTPURL(field.SourceURL); err != nil {
			return fmt.Errorf("field %q source_url %w", name, err)
		}
		if strings.TrimSpace(field.Evidence) == "" {
			return fmt.Errorf("field %q evidence is required", name)
		}
	}
	return nil
}

func normalizeCRMEnrichmentField(field model.CRMEnrichmentFieldInput) (model.CRMEnrichmentFieldInput, error) {
	field.Field = strings.TrimSpace(strings.ToLower(field.Field))
	field.SourceURL = strings.TrimSpace(field.SourceURL)
	field.Evidence = strings.TrimSpace(field.Evidence)
	if len(field.Evidence) > 1000 {
		field.Evidence = field.Evidence[:1000]
	}
	switch field.Field {
	case "email":
		value, err := crmStringValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("email %w", err)
		}
		value = strings.ToLower(value)
		addr, err := mail.ParseAddress(value)
		if err != nil || addr.Address != value {
			return field, fmt.Errorf("email must be a valid email address")
		}
		field.Value = value
	case "phone", "job_title", "industry", "domain", "location", "headquarters", "enrichment_note":
		value, err := crmStringValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("%s %w", field.Field, err)
		}
		field.Value = value
	case "description":
		value, err := crmStringValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("description %w", err)
		}
		if len(value) > 2000 {
			value = value[:2000]
		}
		field.Value = value
	case "avatar_url", "logo_url", "linkedin_url":
		value, err := crmStringValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("%s %w", field.Field, err)
		}
		normalized, err := validateHTTPURL(value)
		if err != nil {
			return field, fmt.Errorf("%s %w", field.Field, err)
		}
		field.Value = normalized
	case "employee_count":
		value, err := crmIntValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("employee_count %w", err)
		}
		field.Value = value
	case "annual_revenue":
		value, err := crmFloatValue(field.Value)
		if err != nil {
			return field, fmt.Errorf("annual_revenue %w", err)
		}
		field.Value = value
	}
	return field, nil
}

func enrichmentFieldResults(value interface{}) ([]model.CRMEnrichmentFieldResult, error) {
	if value == nil {
		return []model.CRMEnrichmentFieldResult{}, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode enrichment fields: %w", err)
	}
	var results []model.CRMEnrichmentFieldResult
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, fmt.Errorf("decode enrichment fields: %w", err)
	}
	return results, nil
}

func enrichmentFieldInputs(value interface{}) ([]model.CRMEnrichmentFieldInput, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode requested enrichment fields: %w", err)
	}
	var results []model.CRMEnrichmentFieldInput
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, fmt.Errorf("decode requested enrichment fields: %w", err)
	}
	return results, nil
}

func applyContactEnrichmentValue(contact *model.CRMContact, field model.CRMEnrichmentFieldInput) error {
	switch field.Field {
	case "email":
		contact.Email = stringPtr(field.Value.(string))
	case "phone":
		contact.Phone = stringPtr(field.Value.(string))
	case "job_title":
		contact.JobTitle = stringPtr(field.Value.(string))
	case "avatar_url":
		contact.AvatarURL = stringPtr(field.Value.(string))
	case "linkedin_url":
		contact.LinkedInURL = stringPtr(field.Value.(string))
	case "location":
		contact.PrimaryLocation = stringPtr(field.Value.(string))
	default:
		return fmt.Errorf("field %q cannot be applied to a contact", field.Field)
	}
	return nil
}

func applyCompanyEnrichmentValue(company *model.CRMCompany, field model.CRMEnrichmentFieldInput) error {
	switch field.Field {
	case "domain":
		company.Domain = stringPtr(field.Value.(string))
	case "industry":
		company.Industry = stringPtr(field.Value.(string))
	case "employee_count":
		value := field.Value.(int)
		company.EmployeeCount = &value
	case "annual_revenue":
		value := field.Value.(float64)
		company.AnnualRevenue = &value
	case "description":
		company.Description = stringPtr(field.Value.(string))
	case "logo_url":
		company.LogoURL = stringPtr(field.Value.(string))
	case "linkedin_url":
		company.LinkedInURL = stringPtr(field.Value.(string))
	case "headquarters":
		company.Headquarters = stringPtr(field.Value.(string))
	default:
		return fmt.Errorf("field %q cannot be applied to a company", field.Field)
	}
	return nil
}

func crmStringValue(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return "", fmt.Errorf("value is required")
		}
		if len(value) > 500 {
			value = value[:500]
		}
		return value, nil
	default:
		return "", fmt.Errorf("value must be a string")
	}
}

func crmIntValue(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return v, nil
	case float64:
		if v < 0 || v != float64(int(v)) {
			return 0, fmt.Errorf("value must be a non-negative integer")
		}
		return int(v), nil
	case json.Number:
		n, err := strconv.Atoi(v.String())
		if err != nil || n < 0 {
			return 0, fmt.Errorf("value must be a non-negative integer")
		}
		return n, nil
	default:
		return 0, fmt.Errorf("value must be a non-negative integer")
	}
}

func crmFloatValue(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return v, nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return float64(v), nil
	case json.Number:
		n, err := strconv.ParseFloat(v.String(), 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return n, nil
	default:
		return 0, fmt.Errorf("value must be non-negative")
	}
}

func validateHTTPURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("must be an http or https URL")
	}
	return parsed.String(), nil
}

func fillStringField(field model.CRMEnrichmentFieldInput, current *string) (model.CRMEnrichmentFieldResult, bool) {
	proposed := field.Value.(string)
	result := model.CRMEnrichmentFieldResult{
		Field:      field.Field,
		NewValue:   proposed,
		SourceURL:  field.SourceURL,
		Confidence: field.Confidence,
	}
	if current == nil || strings.TrimSpace(*current) == "" {
		return result, true
	}
	result.CurrentValuePresent = true
	result.ProposedValue = proposed
	result.OldValue = *current
	if strings.EqualFold(strings.TrimSpace(*current), proposed) {
		result.Reason = "same_value"
		return result, false
	}
	result.Reason = "existing_value_protected"
	return result, false
}

func fillIntField(field model.CRMEnrichmentFieldInput, current *int) (model.CRMEnrichmentFieldResult, bool) {
	proposed := field.Value.(int)
	result := model.CRMEnrichmentFieldResult{Field: field.Field, NewValue: proposed, SourceURL: field.SourceURL, Confidence: field.Confidence}
	if current == nil {
		return result, true
	}
	result.CurrentValuePresent = true
	result.ProposedValue = proposed
	result.OldValue = *current
	if *current == proposed {
		result.Reason = "same_value"
		return result, false
	}
	result.Reason = "existing_value_protected"
	return result, false
}

func fillFloatField(field model.CRMEnrichmentFieldInput, current *float64) (model.CRMEnrichmentFieldResult, bool) {
	proposed := field.Value.(float64)
	result := model.CRMEnrichmentFieldResult{Field: field.Field, NewValue: proposed, SourceURL: field.SourceURL, Confidence: field.Confidence}
	if current == nil {
		return result, true
	}
	result.CurrentValuePresent = true
	result.ProposedValue = proposed
	result.OldValue = *current
	if *current == proposed {
		result.Reason = "same_value"
		return result, false
	}
	result.Reason = "existing_value_protected"
	return result, false
}

func appendCRMFieldResult(result model.CRMEnrichmentFieldResult, applied bool, appliedOut, skippedOut *[]model.CRMEnrichmentFieldResult) {
	if applied {
		*appliedOut = append(*appliedOut, result)
		return
	}
	*skippedOut = append(*skippedOut, result)
}

func cloneJSONB(input model.JSONB) model.JSONB {
	out := model.JSONB{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func recordEnrichmentProvenance(props model.JSONB, field model.CRMEnrichmentFieldInput) {
	bucket := agentEnrichmentBucket(props)
	bucket[field.Field] = map[string]interface{}{
		"value":      field.Value,
		"source_url": field.SourceURL,
		"evidence":   field.Evidence,
		"confidence": field.Confidence,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	props["agent_enrichment"] = bucket
}

func recordAppliedEnrichmentProvenance(props model.JSONB, requested []model.CRMEnrichmentFieldInput, applied []model.CRMEnrichmentFieldResult) {
	appliedFields := make(map[string]model.CRMEnrichmentFieldResult, len(applied))
	for _, result := range applied {
		if result.Field != "enrichment_note" {
			appliedFields[result.Field] = result
		}
	}
	for _, field := range requested {
		field.Field = strings.TrimSpace(strings.ToLower(field.Field))
		if result, ok := appliedFields[field.Field]; ok {
			field.Value = result.NewValue
			recordEnrichmentProvenance(props, field)
		}
	}
}

func appendAgentEnrichmentNote(props model.JSONB, field model.CRMEnrichmentFieldInput) model.CRMEnrichmentFieldResult {
	bucket := agentEnrichmentBucket(props)
	note := map[string]interface{}{
		"value":      field.Value,
		"source_url": field.SourceURL,
		"confidence": field.Confidence,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	notes := make([]interface{}, 0, 1)
	if existing, ok := bucket["notes"].([]interface{}); ok {
		notes = append(notes, existing...)
	}
	notes = append(notes, note)
	bucket["notes"] = notes
	props["agent_enrichment"] = bucket
	return model.CRMEnrichmentFieldResult{Field: field.Field, NewValue: field.Value, SourceURL: field.SourceURL, Confidence: field.Confidence}
}

func agentEnrichmentBucket(props model.JSONB) map[string]interface{} {
	if existing, ok := props["agent_enrichment"].(map[string]interface{}); ok {
		return existing
	}
	bucket := map[string]interface{}{}
	props["agent_enrichment"] = bucket
	return bucket
}

func crmEnrichmentApplyResult(objectType, objectID string, applied, skipped []model.CRMEnrichmentFieldResult, dryRun bool) *model.CRMEnrichmentApplyResult {
	status := "skipped"
	if dryRun {
		status = "dry_run"
	} else if len(applied) > 0 && len(skipped) > 0 {
		status = "partial"
	} else if len(applied) > 0 {
		status = "applied"
	}
	return &model.CRMEnrichmentApplyResult{
		Status:     status,
		ObjectType: objectType,
		ObjectID:   objectID,
		Applied:    applied,
		Skipped:    skipped,
		DryRun:     dryRun,
	}
}

func guardedEnrichmentAudit(workspaceID, objectType, objectID, evidenceSummary string, fields []model.CRMEnrichmentFieldInput, result *model.CRMEnrichmentApplyResult) *model.CRMEnrichmentResult {
	data := map[string]interface{}{
		"evidence_summary": strings.TrimSpace(evidenceSummary),
		"requested_fields": fields,
		"applied":          result.Applied,
		"skipped":          result.Skipped,
		"status":           result.Status,
		"dry_run":          result.DryRun,
	}
	confidence := averageCRMFieldConfidence(fields)
	return &model.CRMEnrichmentResult{
		WorkspaceID: workspaceID,
		ObjectType:  objectType,
		ObjectID:    objectID,
		Source:      model.CRMEnrichmentSourceAI,
		Data:        model.JSONB(data),
		Confidence:  confidence,
	}
}

func enrichmentActivity(workspaceID, objectType, objectID, actorUserID string, applied []model.CRMEnrichmentFieldResult) *model.CRMActivity {
	subject := "AI enrichment updated CRM fields"
	changes := make([]string, 0, len(applied))
	for _, field := range applied {
		if field.Field != "enrichment_note" {
			name := strings.ReplaceAll(field.Field, "_", " ")
			if field.OldValue != nil {
				changes = append(changes, fmt.Sprintf("%s: %v → %v", name, field.OldValue, field.NewValue))
			} else {
				changes = append(changes, fmt.Sprintf("%s: added %v", name, field.NewValue))
			}
		}
	}
	body := strings.Join(changes, "\n")
	if body == "" {
		body = "Added enrichment research notes"
	}
	activity := &model.CRMActivity{
		WorkspaceID:  workspaceID,
		ActivityType: model.CRMActivityNote,
		Subject:      &subject,
		Body:         &body,
		OccurredAt:   time.Now().UTC(),
		Metadata: model.JSONB{
			"event_type":    "crm_enrichment_applied",
			"actor_user_id": actorUserID,
			"changes":       applied,
			"immutable":     true,
		},
	}
	if objectType == "contact" {
		activity.ContactID = &objectID
	} else if objectType == "company" {
		activity.CompanyID = &objectID
	}
	return activity
}

func (s *CRMEnrichmentService) publishCRMEnrichment(entity, workspaceID, objectID, actorUserID string) {
	if s.wsPublisher == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: entity, EntityID: objectID, WorkspaceID: workspaceID, ActorID: actorUserID})
}

func (s *CRMEnrichmentService) createEnsureCompanyAudit(ctx context.Context, workspaceID string, req model.EnsureCRMContactCompanyRequest, result *model.EnsureCRMContactCompanyResult) (*model.CRMEnrichmentResult, error) {
	data := map[string]interface{}{
		"status":            result.Status,
		"contact_id":        result.ContactID,
		"company_id":        result.CompanyID,
		"company_name":      result.CompanyName,
		"domain":            result.Domain,
		"association_id":    result.AssociationID,
		"association_label": result.AssociationLabel,
		"created_company":   result.CreatedCompany,
		"created_link":      result.CreatedLink,
		"source_url":        req.SourceURL,
		"evidence":          req.Evidence,
		"dry_run":           result.DryRun,
	}
	confidence := req.Confidence
	return s.Create(ctx, model.CreateCRMEnrichmentRequest{
		WorkspaceID: workspaceID,
		ObjectType:  model.CRMObjectContact,
		ObjectID:    result.ContactID,
		Source:      model.CRMEnrichmentSourceAI,
		Data:        data,
		Confidence:  &confidence,
	})
}

func averageCRMFieldConfidence(fields []model.CRMEnrichmentFieldInput) float64 {
	if len(fields) == 0 {
		return 0
	}
	var total float64
	for _, field := range fields {
		total += field.Confidence
	}
	return total / float64(len(fields))
}
