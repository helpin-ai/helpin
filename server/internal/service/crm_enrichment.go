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
}

// NewCRMEnrichmentService creates a new CRMEnrichmentService.
func NewCRMEnrichmentService(enrichmentRepo *repository.CRMEnrichmentRepository, contactRepo *repository.CRMContactRepository, companyRepo *repository.CRMCompanyRepository) *CRMEnrichmentService {
	return &CRMEnrichmentService{enrichmentRepo: enrichmentRepo, contactRepo: contactRepo, companyRepo: companyRepo}
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
		case "linkedin_url", "location":
			result, ok := upsertAgentEnrichmentProperty(props, normalized)
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "enrichment_note":
			result := appendAgentEnrichmentNote(props, normalized)
			applied = append(applied, result)
		default:
			return nil, fmt.Errorf("field %q is not allowed for contact enrichment", normalized.Field)
		}
	}

	if !req.DryRun && len(applied) > 0 {
		contact.CustomProperties = props
		if err := s.contactRepo.Update(ctx, contact); err != nil {
			return nil, err
		}
	}

	result := crmEnrichmentApplyResult("contact", contact.ID, applied, skipped, req.DryRun)
	enrichment, err := s.createGuardedEnrichmentAudit(ctx, workspaceID, "contact", contact.ID, req.EvidenceSummary, req.Fields, result)
	if err != nil {
		return nil, err
	}
	result.EnrichmentResultID = enrichment.ID
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
		case "linkedin_url", "headquarters":
			result, ok := upsertAgentEnrichmentProperty(props, normalized)
			appendCRMFieldResult(result, ok, &applied, &skipped)
		case "enrichment_note":
			result := appendAgentEnrichmentNote(props, normalized)
			applied = append(applied, result)
		default:
			return nil, fmt.Errorf("field %q is not allowed for company enrichment", normalized.Field)
		}
	}

	if !req.DryRun && len(applied) > 0 {
		company.CustomProperties = props
		if err := s.companyRepo.Update(ctx, company); err != nil {
			return nil, err
		}
	}

	result := crmEnrichmentApplyResult("company", company.ID, applied, skipped, req.DryRun)
	enrichment, err := s.createGuardedEnrichmentAudit(ctx, workspaceID, "company", company.ID, req.EvidenceSummary, req.Fields, result)
	if err != nil {
		return nil, err
	}
	result.EnrichmentResultID = enrichment.ID
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

func upsertAgentEnrichmentProperty(props model.JSONB, field model.CRMEnrichmentFieldInput) (model.CRMEnrichmentFieldResult, bool) {
	bucket := agentEnrichmentBucket(props)
	next := map[string]interface{}{
		"value":      field.Value,
		"source_url": field.SourceURL,
		"confidence": field.Confidence,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	result := model.CRMEnrichmentFieldResult{Field: field.Field, NewValue: field.Value, SourceURL: field.SourceURL, Confidence: field.Confidence}
	if existing, ok := bucket[field.Field].(map[string]interface{}); ok {
		result.OldValue = existing["value"]
		if fmt.Sprint(existing["value"]) == fmt.Sprint(field.Value) {
			result.Reason = "same_value"
			result.CurrentValuePresent = true
			result.ProposedValue = field.Value
			return result, false
		}
	}
	bucket[field.Field] = next
	props["agent_enrichment"] = bucket
	return result, true
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

func (s *CRMEnrichmentService) createGuardedEnrichmentAudit(ctx context.Context, workspaceID, objectType, objectID, evidenceSummary string, fields []model.CRMEnrichmentFieldInput, result *model.CRMEnrichmentApplyResult) (*model.CRMEnrichmentResult, error) {
	data := map[string]interface{}{
		"evidence_summary": strings.TrimSpace(evidenceSummary),
		"requested_fields": fields,
		"applied":          result.Applied,
		"skipped":          result.Skipped,
		"status":           result.Status,
		"dry_run":          result.DryRun,
	}
	confidence := averageCRMFieldConfidence(fields)
	return s.Create(ctx, model.CreateCRMEnrichmentRequest{
		WorkspaceID: workspaceID,
		ObjectType:  objectType,
		ObjectID:    objectID,
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
