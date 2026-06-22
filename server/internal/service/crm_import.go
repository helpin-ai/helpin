package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMImportService contains CRM import business logic.
type CRMImportService struct {
	importRepo     *repository.CRMImportRepository
	contactRepo    *repository.CRMContactRepository
	companyRepo    *repository.CRMCompanyRepository
	dealRepo       *repository.CRMDealRepository
	entitlementSvc *EntitlementService
}

// NewCRMImportService creates a new CRMImportService.
func NewCRMImportService(
	importRepo *repository.CRMImportRepository,
	contactRepo *repository.CRMContactRepository,
	companyRepo *repository.CRMCompanyRepository,
	dealRepo *repository.CRMDealRepository,
) *CRMImportService {
	return &CRMImportService{
		importRepo:  importRepo,
		contactRepo: contactRepo,
		companyRepo: companyRepo,
		dealRepo:    dealRepo,
	}
}

func (s *CRMImportService) SetEntitlementService(entitlementSvc *EntitlementService) *CRMImportService {
	s.entitlementSvc = entitlementSvc
	return s
}

// Create creates an import job.
func (s *CRMImportService) Create(ctx context.Context, req model.CreateCRMImportRequest) (*model.CRMImportJob, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if req.ObjectType != "contact" && req.ObjectType != "company" && req.ObjectType != "deal" {
		return nil, fmt.Errorf("object_type must be 'contact', 'company', or 'deal'")
	}
	if req.Source == "" {
		req.Source = model.CRMImportSourceCSV
	}

	job := &model.CRMImportJob{
		WorkspaceID:   req.WorkspaceID,
		Source:        req.Source,
		Status:        model.CRMImportStatusPending,
		ObjectType:    req.ObjectType,
		FileURL:       req.FileURL,
		ColumnMapping: model.JSONB(req.ColumnMapping),
		TotalRows:     req.TotalRows,
	}

	if err := s.importRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// GetByID returns an import job by ID.
func (s *CRMImportService) GetByID(ctx context.Context, id string) (*model.CRMImportJob, error) {
	job, err := s.importRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("import job not found")
	}
	return job, nil
}

// List returns import jobs for a workspace.
func (s *CRMImportService) List(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMImportJob, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.importRepo.List(ctx, workspaceID, pagination)
}

// Process processes an import job with the given CSV data and column mapping.
func (s *CRMImportService) Process(ctx context.Context, id string, req model.ProcessCRMImportRequest) (*model.CRMImportJob, error) {
	job, err := s.importRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("import job not found")
	}
	if job.Status != model.CRMImportStatusPending {
		return nil, fmt.Errorf("import job is not in pending status")
	}

	job.Status = model.CRMImportStatusProcessing
	job.TotalRows = len(req.CSVData)
	if err := s.importRepo.UpdateProgress(ctx, job); err != nil {
		return nil, err
	}

	// Build a field mapping: csv_column_index -> crm_field
	fieldMap := make(map[int]string)
	for i, mapping := range req.ColumnMapping {
		if mapping.CRMField != "" {
			fieldMap[i] = mapping.CRMField
		}
	}

	var errorLog []map[string]interface{}

	for rowIdx, row := range req.CSVData {
		rowErr := s.processRow(ctx, job, row, fieldMap)
		if rowErr != nil {
			job.ErrorCount++
			errorLog = append(errorLog, map[string]interface{}{
				"row":   rowIdx + 1,
				"error": rowErr.Error(),
			})
		} else {
			job.CreatedRows++
		}
		job.ProcessedRows++
	}

	job.Status = model.CRMImportStatusCompleted
	if job.ErrorCount > 0 && job.CreatedRows == 0 {
		job.Status = model.CRMImportStatusFailed
	}

	// Convert error log to JSONB
	errorLogJSONB := make(model.JSONB)
	for i, e := range errorLog {
		errorLogJSONB[fmt.Sprintf("%d", i)] = e
	}
	job.ErrorLog = errorLogJSONB

	if err := s.importRepo.UpdateProgress(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (s *CRMImportService) processRow(ctx context.Context, job *model.CRMImportJob, row []string, fieldMap map[int]string) error {
	fields := make(map[string]string)
	for idx, crmField := range fieldMap {
		if idx < len(row) {
			fields[crmField] = strings.TrimSpace(row[idx])
		}
	}

	switch job.ObjectType {
	case "contact":
		return s.importContact(ctx, job.WorkspaceID, fields)
	case "company":
		return s.importCompany(ctx, job.WorkspaceID, fields)
	case "deal":
		return s.importDeal(ctx, job.WorkspaceID, fields)
	default:
		return fmt.Errorf("unsupported object type: %s", job.ObjectType)
	}
}

func (s *CRMImportService) importContact(ctx context.Context, workspaceID string, fields map[string]string) error {
	firstName := fields["first_name"]
	if firstName == "" {
		return fmt.Errorf("first_name is required")
	}
	if s.entitlementSvc != nil {
		count, err := s.contactRepo.CountByWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		if err := s.entitlementSvc.RequireLimitUsage(ctx, workspaceID, EntitlementLimitContacts, count, 1); err != nil {
			return err
		}
	}

	displayID, err := s.contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return err
	}

	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		DisplayID:      displayID,
		FirstName:      firstName,
		LifecycleStage: model.CRMLifecycleSubscriber,
		LeadStatus:     model.CRMLeadStatusNew,
	}

	if v := fields["last_name"]; v != "" {
		contact.LastName = &v
	}
	if v := fields["email"]; v != "" {
		contact.Email = &v
	}
	if v := fields["phone"]; v != "" {
		contact.Phone = &v
	}
	if v := fields["job_title"]; v != "" {
		contact.JobTitle = &v
	}
	if v := fields["lifecycle_stage"]; v != "" {
		contact.LifecycleStage = v
	}
	if v := fields["lead_status"]; v != "" {
		contact.LeadStatus = v
	}
	if v := fields["source"]; v != "" {
		contact.Source = &v
	}

	return s.contactRepo.Create(ctx, contact)
}

func (s *CRMImportService) importCompany(ctx context.Context, workspaceID string, fields map[string]string) error {
	name := fields["name"]
	if name == "" {
		return fmt.Errorf("name is required")
	}

	displayID, err := s.companyRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return err
	}

	company := &model.CRMCompany{
		WorkspaceID: workspaceID,
		DisplayID:   displayID,
		Name:        name,
	}

	if v := fields["domain"]; v != "" {
		company.Domain = &v
	}
	if v := fields["industry"]; v != "" {
		company.Industry = &v
	}
	if v := fields["description"]; v != "" {
		company.Description = &v
	}

	return s.companyRepo.Create(ctx, company)
}

func (s *CRMImportService) importDeal(ctx context.Context, workspaceID string, fields map[string]string) error {
	name := fields["name"]
	if name == "" {
		return fmt.Errorf("name is required")
	}

	pipelineID := fields["pipeline_id"]
	stageID := fields["stage_id"]
	if pipelineID == "" || stageID == "" {
		return fmt.Errorf("pipeline_id and stage_id are required for deals")
	}

	displayID, err := s.dealRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return err
	}

	deal := &model.CRMDeal{
		WorkspaceID: workspaceID,
		DisplayID:   displayID,
		Name:        name,
		PipelineID:  pipelineID,
		StageID:     stageID,
		Currency:    "USD",
	}

	return s.dealRepo.Create(ctx, deal)
}
