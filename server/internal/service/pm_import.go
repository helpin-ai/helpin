package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const shortcutImportStaleAfter = 30 * time.Minute

type PMImportService struct {
	db                *gorm.DB
	workspaceRepo     *repository.WorkspaceRepository
	workflowRepo      *repository.PMWorkflowRepository
	attachmentService shortcutImportedAttachmentService
	mediaDownloader   shortcutMediaDownloader
	docsDocumentSvc   *DocsDocumentService
	docsContentSvc    *DocsContentService
	publisher         *websocket.Publisher
	temporalClient    tclient.Client
	entitlementSvc    EntitlementPolicy
	encryptionKey     []byte
}

var shortcutImportLabelColors = []string{
	"#3b82f6",
	"#16a34a",
	"#ec4899",
	"#64748b",
	"#ef4444",
	"#f97316",
	"#eab308",
	"#14b8a6",
	"#8b5cf6",
	"#6366f1",
	"#06b6d4",
	"#d946ef",
	"#84cc16",
	"#f43f5e",
	"#0ea5e9",
	"#a855f7",
}

func NewPMImportService(db *gorm.DB, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, attachmentService shortcutImportedAttachmentService, encryptionKey ...[]byte) *PMImportService {
	var key []byte
	if len(encryptionKey) > 0 && len(encryptionKey[0]) == 32 {
		key = append([]byte(nil), encryptionKey[0]...)
	}
	return &PMImportService{
		db:                db,
		workspaceRepo:     workspaceRepo,
		workflowRepo:      workflowRepo,
		attachmentService: attachmentService,
		mediaDownloader:   newShortcutHTTPMediaDownloader(),
		encryptionKey:     key,
	}
}

func (s *PMImportService) SetPublisher(publisher *websocket.Publisher) {
	s.publisher = publisher
}

func (s *PMImportService) SetTemporalClient(client tclient.Client) {
	if s == nil {
		return
	}
	s.temporalClient = client
}

func (s *PMImportService) SetEntitlementService(entitlementSvc EntitlementPolicy) *PMImportService {
	s.entitlementSvc = entitlementSvc
	return s
}

func (s *PMImportService) SetDocsImportDependencies(documentSvc *DocsDocumentService, contentSvc *DocsContentService) {
	if s == nil {
		return
	}
	s.docsDocumentSvc = documentSvc
	s.docsContentSvc = contentSvc
}

func (s *PMImportService) PreviewShortcut(ctx context.Context, workspaceID, actorID string, csvData []byte, apiToken string) (*model.ShortcutImportPreviewResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	data, err := parseShortcutCSV(csvData)
	if err != nil {
		return nil, err
	}

	// If API token provided, validate and fetch member names for better matching
	var apiEnrichment *shortcutAPIEnrichment
	var apiWarnings []string
	if apiToken != "" {
		client := NewShortcutAPIClient(apiToken)
		if err := client.ValidateToken(ctx); err != nil {
			return nil, fmt.Errorf("Shortcut API token validation failed: %w", err)
		}
		apiEnrichment, apiWarnings = client.FetchEnrichment(ctx)
	}

	members, err := s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberByEmail := make(map[string]model.AssignableMember, len(members))
	for _, member := range members {
		memberByEmail[normalizeShortcutName(member.Email)] = member
	}

	taskTypeCounts := map[string]int{}
	epicIDs := map[string]struct{}{}
	objectiveIDs := map[string]struct{}{}
	sprintIDs := map[string]struct{}{}
	labelNames := map[string]struct{}{}
	teamCounts := map[string]int{}
	workflowStateCounts := map[string]*shortcutWorkflowAggregate{}
	emailCounts := map[string]int{}
	ownerCounts := map[string]int{}
	requesterCounts := map[string]int{}
	checklistCount := 0
	for _, row := range data.Rows {
		taskTypeCounts[row.Type]++
		if row.EpicID != "" {
			epicIDs[row.EpicID] = struct{}{}
		}
		if row.ObjectiveID != "" {
			objectiveIDs[row.ObjectiveID] = struct{}{}
		}
		if row.IterationID != "" {
			sprintIDs[row.IterationID] = struct{}{}
		}
		for _, label := range shortcutLabelNames(row.Labels) {
			labelNames[normalizeShortcutName(label)] = struct{}{}
		}
		for _, label := range shortcutLabelNames(row.EpicLabels) {
			labelNames[normalizeShortcutName(label)] = struct{}{}
		}
		if row.Team != "" {
			teamCounts[row.Team]++
		}
		workflowKey := shortcutWorkflowKey(row.WorkflowID, row.Workflow)
		workflow := workflowStateCounts[workflowKey]
		if workflow == nil {
			workflow = &shortcutWorkflowAggregate{
				ID:          strings.TrimSpace(row.WorkflowID),
				Name:        shortcutWorkflowDisplayName(row.WorkflowID, row.Workflow),
				StateCounts: map[string]int{},
			}
			workflowStateCounts[workflowKey] = workflow
		} else {
			if workflow.ID == "" {
				workflow.ID = strings.TrimSpace(row.WorkflowID)
			}
			if strings.TrimSpace(workflow.Name) == "" {
				workflow.Name = shortcutWorkflowDisplayName(row.WorkflowID, row.Workflow)
			}
		}
		workflow.StateCounts[row.State]++
		workflow.TaskCount++
		if row.Requester != "" {
			email := normalizeShortcutName(row.Requester)
			emailCounts[email]++
			requesterCounts[email]++
		}
		for _, owner := range shortcutOwnerEmails(row.Owners) {
			email := normalizeShortcutName(owner)
			emailCounts[email]++
			ownerCounts[email]++
		}
		checklistCount += len(parseShortcutChecklist(row.Tasks))
	}

	duplicateTasks, err := s.countTaskDuplicates(ctx, workspaceID, mapKeys(shortcutStoryExternalIDs(data.Rows)))
	if err != nil {
		return nil, err
	}

	users := make([]model.ShortcutUserMatch, 0, len(emailCounts))
	for _, email := range sortKeysByCount(emailCounts) {
		match := model.ShortcutUserMatch{
			Email:          email,
			TaskCount:      emailCounts[email],
			OwnerCount:     ownerCounts[email],
			RequesterCount: requesterCounts[email],
		}
		if member, ok := memberByEmail[email]; ok {
			match.MatchedMemberID = &member.ID
			match.MatchedMemberStatus = &member.Status
			match.MatchedName = &member.DisplayName
			if member.UserID != nil && strings.TrimSpace(*member.UserID) != "" {
				match.MatchedUserID = member.UserID
			}
		}
		if apiEnrichment != nil {
			if scMember, ok := apiEnrichment.MembersByEmail[email]; ok {
				match.ShortcutMemberID = &scMember.ID
				if scMember.Profile.Name != "" {
					match.ShortcutName = &scMember.Profile.Name
				}
			}
		}
		users = append(users, match)
	}

	teams := make([]model.ShortcutTeamPreview, 0, len(teamCounts))
	for _, name := range sortKeysByCount(teamCounts) {
		teams = append(teams, model.ShortcutTeamPreview{Name: name, TaskCount: teamCounts[name]})
	}

	workflows := make([]model.ShortcutWorkflowPreview, 0, len(workflowStateCounts))
	workflowGroups := make([]*shortcutWorkflowAggregate, 0, len(workflowStateCounts))
	for _, workflow := range workflowStateCounts {
		workflowGroups = append(workflowGroups, workflow)
	}
	sort.Slice(workflowGroups, func(i, j int) bool {
		leftName := normalizeShortcutName(workflowGroups[i].Name)
		rightName := normalizeShortcutName(workflowGroups[j].Name)
		if leftName == rightName {
			return workflowGroups[i].ID < workflowGroups[j].ID
		}
		return leftName < rightName
	})
	stateTypeOrder := map[string]int{
		model.PMStateTypeBacklog:   0,
		model.PMStateTypeUnstarted: 1,
		model.PMStateTypeStarted:   2,
		model.PMStateTypeDone:      3,
	}
	for _, workflow := range workflowGroups {
		states := make([]model.ShortcutWorkflowStatePreview, 0, len(workflow.StateCounts))
		for _, stateName := range sortKeysByCount(workflow.StateCounts) {
			states = append(states, model.ShortcutWorkflowStatePreview{
				Name:          stateName,
				SuggestedType: suggestedStateType(stateName),
				TaskCount:     workflow.StateCounts[stateName],
			})
		}
		// Sort states by logical workflow order (backlog → unstarted → started → done)
		sort.SliceStable(states, func(i, j int) bool {
			return stateTypeOrder[states[i].SuggestedType] < stateTypeOrder[states[j].SuggestedType]
		})
		workflows = append(workflows, model.ShortcutWorkflowPreview{
			ID:        workflow.ID,
			Name:      workflow.Name,
			TaskCount: workflow.TaskCount,
			States:    states,
		})
	}

	return &model.ShortcutImportPreviewResponse{
		Summary: model.ShortcutImportPreviewSummary{
			TotalTasks:          len(data.Rows),
			TasksByType:         taskTypeCounts,
			EpicsCount:          len(epicIDs),
			ObjectivesCount:     len(objectiveIDs),
			SprintsCount:        len(sprintIDs),
			LabelsCount:         len(labelNames),
			TeamsCount:          len(teamCounts),
			WorkflowsCount:      len(workflowStateCounts),
			WorkflowStatesCount: countWorkflowStates(workflowStateCounts),
			ChecklistItemsCount: checklistCount,
			DuplicateTasks:      duplicateTasks,
		},
		Users:     users,
		Teams:     teams,
		Workflows: workflows,
		Warnings:  append(append([]string(nil), data.Warnings...), apiWarnings...),
	}, nil
}

func (s *PMImportService) ExecuteShortcut(ctx context.Context, workspaceID, actorID, fileName string, csvData []byte, req model.ShortcutImportExecuteRequest) (*model.ShortcutImportExecuteResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if _, err := parseShortcutCSV(csvData); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := &model.PMImportJob{
		WorkspaceID: workspaceID,
		Source:      model.PMImportSourceShortcut,
		Status:      model.PMImportStatusPending,
		FileName:    fileName,
		StartedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	go s.runShortcutImport(job.ID, workspaceID, actorID, csvData, req, req.APIToken)

	return &model.ShortcutImportExecuteResponse{
		ImportID: job.ID,
		Status:   model.PMImportStatusProcessing,
	}, nil
}

func (s *PMImportService) GetShortcutStatus(ctx context.Context, workspaceID, actorID, importID string) (*model.ShortcutImportStatusResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if err := s.reconcileStaleShortcutImports(ctx, workspaceID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", importID, workspaceID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("import job not found")
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	return shortcutImportStatusFromJob(job), nil
}

func (s *PMImportService) GetShortcutStatusDetail(ctx context.Context, workspaceID, actorID, importID string) (*model.ShortcutImportDetailResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if err := s.reconcileStaleShortcutImports(ctx, workspaceID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", importID, workspaceID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("import job not found")
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	status := shortcutImportStatusFromJob(job)
	detail := &model.ShortcutImportDetailResponse{
		ShortcutImportStatusResponse: *status,
		Diagnostics:                  shortcutImportDiagnostics(status.Result),
		Retryable:                    shortcutImportCanRetry(job) && len(s.encryptionKey) == 32,
		RetryBlockedReason:           shortcutImportRetryBlockedReason(job, s.encryptionKey),
		Cancelable:                   shortcutImportCanCancel(job.Status),
	}
	if opts, ok := s.shortcutImportStoredOptions(job); ok {
		detail.Options = opts
	}
	return detail, nil
}

func (s *PMImportService) ListShortcutStatuses(ctx context.Context, workspaceID, actorID string) ([]model.ShortcutImportStatusResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if err := s.reconcileStaleShortcutImports(ctx, workspaceID); err != nil {
		return nil, err
	}
	var jobs []model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND source = ?", workspaceID, model.PMImportSourceShortcut).
		Order("created_at DESC").
		Limit(50).
		Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("list import jobs: %w", err)
	}
	statuses := make([]model.ShortcutImportStatusResponse, 0, len(jobs))
	for _, job := range jobs {
		statuses = append(statuses, *shortcutImportStatusFromJob(job))
	}
	return statuses, nil
}

func (s *PMImportService) CancelShortcutImport(ctx context.Context, workspaceID, actorID, importID string) (*model.ShortcutImportStatusResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", importID, workspaceID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("import job not found")
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if !shortcutImportCanCancel(job.Status) {
		return shortcutImportStatusFromJob(job), nil
	}
	if s.temporalClient != nil && job.WorkflowID != nil && strings.TrimSpace(*job.WorkflowID) != "" {
		_ = s.temporalClient.CancelWorkflow(ctx, *job.WorkflowID, "")
	}
	msg := "Import canceled by user"
	now := time.Now().UTC()
	if err := s.updateJob(ctx, job.ID, map[string]interface{}{
		"status":       model.PMImportStatusCanceled,
		"error":        &msg,
		"completed_at": &now,
		"updated_at":   now,
	}); err != nil {
		return nil, fmt.Errorf("cancel import job: %w", err)
	}
	job.Status = model.PMImportStatusCanceled
	job.Error = &msg
	job.CompletedAt = &now
	job.UpdatedAt = now
	return shortcutImportStatusFromJob(job), nil
}

func (s *PMImportService) RetryShortcutImport(ctx context.Context, workspaceID, actorID, importID string) (*model.ShortcutImportExecuteResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", importID, workspaceID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("import job not found")
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if !shortcutImportCanRetry(job) || len(s.encryptionKey) != 32 {
		return nil, fmt.Errorf("%s", shortcutImportRetryBlockedReason(job, s.encryptionKey))
	}
	raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("Shortcut import payload could not be decrypted")
	}
	var req model.ShortcutAPIImportExecuteRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("Shortcut import payload is invalid")
	}
	if strings.TrimSpace(req.APIToken) == "" {
		return nil, fmt.Errorf("Shortcut import token is missing from stored payload")
	}
	return s.ExecuteShortcutAPI(ctx, workspaceID, actorID, req)
}

func shortcutImportStatusFromJob(job model.PMImportJob) *model.ShortcutImportStatusResponse {
	resp := &model.ShortcutImportStatusResponse{
		ImportID:  job.ID,
		Status:    job.Status,
		FileName:  job.FileName,
		TotalRows: job.TotalRows,
		Progress: model.ShortcutImportStatusProgress{
			CurrentStep:       job.CurrentStep,
			StepsCompleted:    job.StepsCompleted,
			StepsTotal:        job.StepsTotal,
			EntitiesProcessed: job.EntitiesProcessed,
			EntitiesTotal:     job.EntitiesTotal,
		},
		Error:       job.Error,
		CreatedAt:   &job.CreatedAt,
		UpdatedAt:   &job.UpdatedAt,
		CompletedAt: job.CompletedAt,
	}
	if job.Result != nil && *job.Result != "" {
		var result model.ShortcutImportResult
		if err := json.Unmarshal([]byte(*job.Result), &result); err == nil {
			resp.Result = &result
		}
	}
	return resp
}

func (s *PMImportService) reconcileStaleShortcutImports(ctx context.Context, workspaceID string) error {
	cutoff := time.Now().UTC().Add(-shortcutImportStaleAfter)
	msg := fmt.Sprintf("Import did not report progress for %s. Retry the import or cancel it from history.", shortcutImportStaleAfter)
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&model.PMImportJob{}).
		Where("workspace_id = ? AND source = ? AND status IN ? AND updated_at < ?", workspaceID, model.PMImportSourceShortcut, []string{
			model.PMImportStatusPending,
			model.PMImportStatusScanning,
			model.PMImportStatusProcessing,
		}, cutoff).
		Updates(map[string]interface{}{
			"status":       model.PMImportStatusFailed,
			"error":        &msg,
			"completed_at": &now,
			"updated_at":   now,
		}).Error; err != nil {
		return fmt.Errorf("reconcile stale Shortcut imports: %w", err)
	}
	return nil
}

func shortcutImportCanCancel(status string) bool {
	switch status {
	case model.PMImportStatusPending, model.PMImportStatusScanning, model.PMImportStatusProcessing:
		return true
	default:
		return false
	}
}

func shortcutImportCanRetry(job model.PMImportJob) bool {
	return (job.Status == model.PMImportStatusFailed || job.Status == model.PMImportStatusCanceled) &&
		job.PayloadEncrypted != nil &&
		strings.TrimSpace(*job.PayloadEncrypted) != ""
}

func shortcutImportRetryBlockedReason(job model.PMImportJob, encryptionKey []byte) string {
	if job.Status != model.PMImportStatusFailed && job.Status != model.PMImportStatusCanceled {
		return "Only failed or canceled Shortcut API imports can be retried"
	}
	if len(encryptionKey) != 32 {
		return "Shortcut import retry requires the import encryption key to be configured"
	}
	if job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
		return "This import cannot be retried because no stored API payload is available"
	}
	return ""
}

func (s *PMImportService) shortcutImportStoredOptions(job model.PMImportJob) (*model.ShortcutImportOptions, bool) {
	if len(s.encryptionKey) != 32 || job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
		return nil, false
	}
	raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
	if err != nil {
		return nil, false
	}
	var req model.ShortcutAPIImportExecuteRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, false
	}
	return &req.Options, true
}

func shortcutImportDiagnostics(result *model.ShortcutImportResult) model.ShortcutImportDiagnostics {
	diag := model.ShortcutImportDiagnostics{
		Counts:               []model.ShortcutImportCount{},
		WarningGroups:        []model.ShortcutImportWarningGroup{},
		FailedMedia:          []model.ShortcutImportDiagnosticItem{},
		UnmappedMembers:      []model.ShortcutImportDiagnosticItem{},
		UnmappedStates:       []model.ShortcutImportDiagnosticItem{},
		UnmappedTeams:        []model.ShortcutImportDiagnosticItem{},
		RetryableFailures:    []model.ShortcutImportDiagnosticItem{},
		NonRetryableFailures: []model.ShortcutImportDiagnosticItem{},
	}
	if result == nil {
		return diag
	}
	diag.Counts = []model.ShortcutImportCount{
		{Entity: "teams", Count: result.TeamsCreated},
		{Entity: "workflows", Count: result.WorkflowsCreated},
		{Entity: "workflow_states", Count: result.WorkflowStatesCreated},
		{Entity: "labels", Count: result.LabelsCreated},
		{Entity: "objectives", Count: result.ObjectivesCreated},
		{Entity: "epics", Count: result.EpicsCreated},
		{Entity: "sprints", Count: result.SprintsCreated},
		{Entity: "tasks_created", Count: result.TasksCreated},
		{Entity: "tasks_skipped", Count: result.TasksSkipped},
		{Entity: "docs_created", Count: result.DocsCreated},
		{Entity: "docs_skipped", Count: result.DocsSkipped},
		{Entity: "checklist_items", Count: result.ChecklistItemsCreated},
		{Entity: "owner_links", Count: result.OwnerLinksCreated},
		{Entity: "label_links", Count: result.LabelLinksCreated},
		{Entity: "external_links", Count: result.ExternalLinksCreated},
		{Entity: "task_links", Count: result.TaskLinksCreated},
		{Entity: "attachments", Count: result.AttachmentsCreated},
		{Entity: "comments", Count: result.CommentsCreated},
	}
	groups := map[string][]string{}
	for _, warning := range result.Warnings {
		category := shortcutImportWarningCategory(warning)
		groups[category] = append(groups[category], warning)
		item := shortcutImportDiagnosticItemFromWarning(category, warning)
		switch category {
		case "media":
			diag.FailedMedia = append(diag.FailedMedia, item)
			if item.Retryable {
				diag.RetryableFailures = append(diag.RetryableFailures, item)
			} else {
				diag.NonRetryableFailures = append(diag.NonRetryableFailures, item)
			}
		case "members":
			diag.UnmappedMembers = append(diag.UnmappedMembers, item)
		case "states":
			diag.UnmappedStates = append(diag.UnmappedStates, item)
		case "teams":
			diag.UnmappedTeams = append(diag.UnmappedTeams, item)
		}
	}
	for category, warnings := range groups {
		visible := warnings
		if len(visible) > 8 {
			visible = visible[:8]
		}
		diag.WarningGroups = append(diag.WarningGroups, model.ShortcutImportWarningGroup{
			Type:     category,
			Count:    len(warnings),
			Warnings: append([]string(nil), visible...),
		})
	}
	sort.Slice(diag.WarningGroups, func(i, j int) bool {
		if diag.WarningGroups[i].Count == diag.WarningGroups[j].Count {
			return diag.WarningGroups[i].Type < diag.WarningGroups[j].Type
		}
		return diag.WarningGroups[i].Count > diag.WarningGroups[j].Count
	})
	return diag
}

func shortcutImportWarningCategory(warning string) string {
	lower := strings.ToLower(warning)
	switch {
	case strings.Contains(lower, "shortcut media"):
		return "media"
	case strings.Contains(lower, "owner email") || strings.Contains(lower, "requester"):
		return "members"
	case strings.Contains(lower, "could not be mapped") || strings.Contains(lower, "default state"):
		return "states"
	case strings.Contains(lower, "team"):
		return "teams"
	case strings.Contains(lower, "doc"):
		return "docs"
	default:
		return "general"
	}
}

func shortcutImportDiagnosticItemFromWarning(category, warning string) model.ShortcutImportDiagnosticItem {
	item := model.ShortcutImportDiagnosticItem{
		Type:      category,
		Message:   warning,
		Count:     firstIntInString(warning),
		Retryable: false,
	}
	if category == "media" {
		item.Key = quotedValue(warning)
		lower := strings.ToLower(warning)
		item.Retryable = strings.Contains(lower, "download") &&
			!strings.Contains(lower, "exceeds") &&
			!strings.Contains(lower, "not allowed") &&
			!strings.Contains(lower, "unsupported")
	}
	if category == "members" {
		item.Key = quotedValue(warning)
	}
	return item
}

func quotedValue(text string) string {
	if start := strings.Index(text, `"`); start >= 0 {
		if end := strings.Index(text[start+1:], `"`); end >= 0 {
			return text[start+1 : start+1+end]
		}
	}
	if start := strings.Index(text, `'`); start >= 0 {
		if end := strings.Index(text[start+1:], `'`); end >= 0 {
			return text[start+1 : start+1+end]
		}
	}
	return ""
}

func firstIntInString(text string) int {
	for _, field := range strings.Fields(text) {
		cleaned := strings.Trim(field, ".,:;()[]")
		if n, err := strconv.Atoi(cleaned); err == nil {
			return n
		}
	}
	return 0
}

func (s *PMImportService) runShortcutImport(jobID, workspaceID, actorID string, csvData []byte, req model.ShortcutImportExecuteRequest, apiToken string) {
	ctx := context.Background()
	totalSteps := s.shortcutImportTotalSteps(apiToken, req.Options)
	if err := s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusProcessing,
		"current_step":       "parse",
		"steps_total":        totalSteps,
		"updated_at":         time.Now().UTC(),
		"entities_total":     0,
		"entities_processed": 0,
	}); err != nil {
		return
	}

	result, totalRows, err := s.executeShortcutImport(ctx, workspaceID, actorID, csvData, req, jobID, apiToken)
	if err != nil {
		errText := err.Error()
		_ = s.updateJob(ctx, jobID, map[string]interface{}{
			"status":       model.PMImportStatusFailed,
			"error":        &errText,
			"completed_at": timePtr(time.Now().UTC()),
			"updated_at":   time.Now().UTC(),
		})
		return
	}
	raw, _ := json.Marshal(result)
	rawStr := string(raw)
	completed := time.Now().UTC()
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusCompleted,
		"total_rows":         totalRows,
		"progress":           100,
		"current_step":       "completed",
		"steps_completed":    totalSteps,
		"entities_processed": totalRows,
		"result":             &rawStr,
		"completed_at":       &completed,
		"updated_at":         completed,
	})
}

func (s *PMImportService) executeShortcutImport(ctx context.Context, workspaceID, actorID string, csvData []byte, req model.ShortcutImportExecuteRequest, jobID, apiToken string) (*model.ShortcutImportResult, int, error) {
	data, err := parseShortcutCSV(csvData)
	if err != nil {
		return nil, 0, err
	}
	return s.executeShortcutRows(ctx, workspaceID, actorID, data.Rows, data.Warnings, req, jobID, apiToken, nil, nil, true)
}

func (s *PMImportService) executeShortcutRows(ctx context.Context, workspaceID, actorID string, rows []shortcutCSVRow, initialWarnings []string, req model.ShortcutImportExecuteRequest, jobID, apiToken string, apiClient *ShortcutAPIClient, enrichment *shortcutAPIEnrichment, fetchEnrichment bool) (*model.ShortcutImportResult, int, error) {
	if err := repository.EnsurePMExternalLinksTaskColumn(s.db.WithContext(ctx)); err != nil {
		return nil, 0, err
	}
	totalSteps := s.shortcutImportTotalSteps(apiToken)
	rows = filterShortcutRows(rows, req.Options)
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"total_rows":     len(rows),
		"entities_total": len(rows),
		"updated_at":     time.Now().UTC(),
	})

	// Phase 1: Fetch enrichment data from Shortcut API (if token provided)
	var enrichWarnings []string
	if apiToken != "" && fetchEnrichment {
		apiClient = NewShortcutAPIClient(apiToken)
		_ = s.markStep(ctx, jobID, "api_enrichment", 0, 0, totalSteps)
		enrichment, enrichWarnings = apiClient.FetchEnrichment(ctx)
		_ = s.markStep(ctx, jobID, "api_enrichment", 1, 0, totalSteps)
	}

	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, 0, err
	}
	userByEmail := make(map[string]string, len(members))
	for _, member := range members {
		userByEmail[normalizeShortcutName(member.Email)] = member.UserID
	}
	for email, userID := range req.UserMappings {
		if strings.TrimSpace(userID) != "" {
			userByEmail[normalizeShortcutName(email)] = userID
		}
	}

	// Build email → workspace member ID map (includes pending/invited members)
	assignable, err := s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
	if err != nil {
		return nil, 0, err
	}
	memberByEmail := make(map[string]string, len(assignable))
	memberByUserID := make(map[string]string, len(assignable))
	assignableByID := make(map[string]model.AssignableMember, len(assignable))
	for _, am := range assignable {
		memberByEmail[normalizeShortcutName(am.Email)] = am.ID
		assignableByID[am.ID] = am
		if am.UserID != nil && strings.TrimSpace(*am.UserID) != "" {
			memberByUserID[*am.UserID] = am.ID
		}
	}
	for email, memberID := range req.MemberMappings {
		email = normalizeShortcutName(email)
		memberID = strings.TrimSpace(memberID)
		if email == "" || memberID == "" {
			continue
		}
		if member, ok := assignableByID[memberID]; ok && member.ID != "" {
			memberByEmail[email] = member.ID
			if member.UserID != nil && strings.TrimSpace(*member.UserID) != "" {
				userByEmail[email] = strings.TrimSpace(*member.UserID)
			}
		}
	}
	for email, userID := range req.UserMappings {
		if memberID, ok := memberByUserID[strings.TrimSpace(userID)]; ok && memberID != "" {
			memberByEmail[normalizeShortcutName(email)] = memberID
		}
	}

	// Build Shortcut member UUID → Helpin user ID map for comment author mapping
	scMemberToUser := map[string]string{}
	if enrichment != nil {
		for _, scMember := range enrichment.MembersByEmail {
			email := normalizeShortcutName(scMember.Profile.EmailAddress)
			if uid, ok := userByEmail[email]; ok {
				scMemberToUser[scMember.ID] = uid
			}
		}
	}

	result := &model.ShortcutImportResult{
		Warnings: append([]string(nil), initialWarnings...),
	}
	result.Warnings = appendUniqueWarnings(result.Warnings, enrichWarnings)

	stepOffset := 0
	if apiToken != "" {
		stepOffset = 1 // api_enrichment was step 1
	}
	var createdTaskIDs []string

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		teamMap, teamsCreated, err := s.ensureTeams(ctx, tx, workspaceID, rows, req.TeamMappings)
		if err != nil {
			return err
		}
		result.TeamsCreated = teamsCreated
		_ = s.markStep(ctx, jobID, "teams", 1+stepOffset, 0, totalSteps)

		if err := s.ensureTeamMemberships(ctx, tx, rows, teamMap, memberByEmail); err != nil {
			return err
		}

		workflowMap, stateMap, workflowDefaultStateMap, workflowsCreated, statesCreated, err := s.resolveWorkflowMappings(ctx, tx, workspaceID, req.WorkflowStateMappings, rows, teamMap)
		if err != nil {
			return err
		}
		result.WorkflowsCreated = workflowsCreated
		result.WorkflowStatesCreated = statesCreated
		_ = s.markStep(ctx, jobID, "workflows", 2+stepOffset, 0, totalSteps)

		labelMap, labelsCreated, err := s.ensureLabels(ctx, tx, workspaceID, rows, enrichment)
		if err != nil {
			return err
		}
		result.LabelsCreated = labelsCreated
		_ = s.markStep(ctx, jobID, "labels", 3+stepOffset, 0, totalSteps)

		objectiveMap, objectivesCreated, err := s.ensureObjectives(ctx, tx, workspaceID, rows, enrichment)
		if err != nil {
			return err
		}
		result.ObjectivesCreated = objectivesCreated
		_ = s.markStep(ctx, jobID, "objectives", 4+stepOffset, 0, totalSteps)

		epicMap, epicsCreated, err := s.ensureEpics(ctx, tx, workspaceID, rows, objectiveMap, labelMap, teamMap, enrichment)
		if err != nil {
			return err
		}
		result.EpicsCreated = epicsCreated
		_ = s.markStep(ctx, jobID, "epics", 5+stepOffset, 0, totalSteps)

		sprintMap, sprintsCreated, sprintWarnings, err := s.ensureSprints(ctx, tx, workspaceID, rows, teamMap, enrichment)
		if err != nil {
			return err
		}
		result.SprintsCreated = sprintsCreated
		result.Warnings = append(result.Warnings, sprintWarnings...)
		_ = s.markStep(ctx, jobID, "sprints", 6+stepOffset, 0, totalSteps)

		_ = s.setCurrentStep(ctx, jobID, "tasks")
		createdTaskIDs, err = s.createTasks(ctx, tx, workspaceID, actorID, rows, workflowMap, stateMap, workflowDefaultStateMap, teamMap, userByEmail, memberByEmail, epicMap, sprintMap, labelMap, result, jobID, stepOffset, totalSteps)
		if err != nil {
			return err
		}
		_ = s.markStep(ctx, jobID, "tasks", 7+stepOffset, len(rows), totalSteps)
		return nil
	})
	if err != nil {
		return nil, len(rows), err
	}

	if apiToken != "" {
		_ = s.markStep(ctx, jobID, "task_media", 8+stepOffset, 0, totalSteps)
		attachmentsCreated, mediaWarnings := s.importShortcutStoryMedia(ctx, workspaceID, actorID, createdTaskIDs, apiToken, jobID, 9+stepOffset, totalSteps)
		result.AttachmentsCreated += attachmentsCreated
		result.Warnings = appendUniqueWarnings(result.Warnings, mediaWarnings)
	}

	// Phase 2: Import comments from Shortcut API (outside transaction, per-task)
	if apiClient != nil {
		_ = s.markStep(ctx, jobID, "comments", 9+stepOffset, len(rows), totalSteps)
		commentsCreated, attachmentsCreated, commentWarnings := s.importShortcutComments(ctx, apiClient, workspaceID, actorID, rows, scMemberToUser, enrichment, apiToken, jobID, 10+stepOffset, totalSteps)
		result.CommentsCreated = commentsCreated
		result.AttachmentsCreated += attachmentsCreated
		result.Warnings = appendUniqueWarnings(result.Warnings, commentWarnings)
	}

	return result, len(rows), nil
}

func (s *PMImportService) ensureTeams(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, mappings map[string]string) (map[string]string, int, error) {
	needed := map[string]string{}
	for _, row := range rows {
		if row.Team == "" {
			continue
		}
		needed[normalizeShortcutName(row.Team)] = row.Team
	}

	var existing []model.WorkspaceTeam
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existing).Error; err != nil {
		return nil, 0, fmt.Errorf("list teams: %w", err)
	}
	teamMap := map[string]string{}
	teamByID := map[string]model.WorkspaceTeam{}
	for _, team := range existing {
		teamMap[normalizeShortcutName(team.Name)] = team.ID
		teamByID[team.ID] = team
	}
	normalizedMappings := map[string]string{}
	for shortcutTeam, target := range mappings {
		if key := normalizeShortcutName(shortcutTeam); key != "" {
			normalizedMappings[key] = strings.TrimSpace(target)
		}
	}
	created := 0
	for key, name := range needed {
		if target := normalizedMappings[key]; target != "" {
			mode, value := parseShortcutTeamMappingTarget(target)
			switch mode {
			case "existing":
				team, ok := teamByID[value]
				if !ok {
					return nil, 0, fmt.Errorf("mapped team for %q was not found in this workspace", name)
				}
				teamMap[key] = team.ID
				continue
			case "create":
				if value != "" {
					name = value
					key = normalizeShortcutName(value)
				}
			}
		}
		if _, ok := teamMap[key]; ok {
			continue
		}
		if s.entitlementSvc != nil {
			if err := s.entitlementSvc.RequireLimitUsage(ctx, workspaceID, EntitlementLimitTeams, int64(len(existing)+created), 1); err != nil {
				return nil, 0, err
			}
		}
		team := model.WorkspaceTeam{WorkspaceID: workspaceID, Name: name}
		if err := tx.WithContext(ctx).Create(&team).Error; err != nil {
			return nil, 0, fmt.Errorf("create team: %w", err)
		}
		teamMap[key] = team.ID
		created++
	}
	return teamMap, created, nil
}

func parseShortcutTeamMappingTarget(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return "create", ""
	case strings.HasPrefix(raw, "existing:"):
		return "existing", strings.TrimSpace(strings.TrimPrefix(raw, "existing:"))
	case strings.HasPrefix(raw, "create:"):
		return "create", strings.TrimSpace(strings.TrimPrefix(raw, "create:"))
	case raw == "__create_new__":
		return "create", ""
	default:
		return "existing", raw
	}
}

func (s *PMImportService) ensureTeamMemberships(ctx context.Context, tx *gorm.DB, rows []shortcutCSVRow, teamMap, memberByEmail map[string]string) error {
	memberships := make(map[string]model.TeamWorkspaceMembership)

	for _, row := range rows {
		teamID := firstMappedValue(teamMap, []string{normalizeShortcutName(row.Team)})
		if teamID == "" {
			continue
		}
		for _, email := range shortcutOwnerEmails(row.Owners) {
			memberID := firstMappedValue(memberByEmail, []string{normalizeShortcutName(email)})
			if memberID == "" {
				continue
			}
			key := teamID + "::" + memberID
			if _, exists := memberships[key]; exists {
				continue
			}
			memberships[key] = model.TeamWorkspaceMembership{
				TeamID:            teamID,
				WorkspaceMemberID: memberID,
				Role:              "member",
			}
		}
	}

	if len(memberships) == 0 {
		return nil
	}

	records := make([]model.TeamWorkspaceMembership, 0, len(memberships))
	for _, membership := range memberships {
		records = append(records, membership)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].TeamID == records[j].TeamID {
			return records[i].WorkspaceMemberID < records[j].WorkspaceMemberID
		}
		return records[i].TeamID < records[j].TeamID
	})

	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error; err != nil {
		return fmt.Errorf("create team memberships: %w", err)
	}
	return nil
}

func (s *PMImportService) resolveWorkflowMappings(ctx context.Context, tx *gorm.DB, workspaceID string, mappings []model.ShortcutWorkflowStateMappingPayload, rows []shortcutCSVRow, teamMap map[string]string) (map[string]string, map[string]string, map[string]string, int, int, error) {
	workflowMap := map[string]string{}
	shortcutStateMap := map[string]string{}
	workflowDefaultStateMap := map[string]string{}
	var existingWorkflows []model.PMWorkflow
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existingWorkflows).Error; err != nil {
		return nil, nil, nil, 0, 0, fmt.Errorf("list workflows: %w", err)
	}
	workflowByName := map[string]model.PMWorkflow{}
	for _, workflow := range existingWorkflows {
		workflowByName[workflowIdentityKey(workflow.Name, workflow.TeamID)] = workflow
	}
	workflowTeamIDs := shortcutWorkflowTeamIDs(rows, teamMap)
	workflowsCreated := 0
	statesCreated := 0

	for _, mapping := range mappings {
		keys := shortcutWorkflowMappingKeys(mapping.ShortcutWorkflowID, mapping.ShortcutWorkflowName)
		switch mapping.Mode {
		case "create_new":
			name := strings.TrimSpace(mapping.NewWorkflowName)
			if name == "" {
				name = shortcutWorkflowDisplayName(mapping.ShortcutWorkflowID, mapping.ShortcutWorkflowName)
			}
			targetTeamIDs := shortcutWorkflowTargetTeamIDs(keys, workflowTeamIDs)
			for _, targetTeamID := range targetTeamIDs {
				var teamIDPtr *string
				if targetTeamID != "" {
					teamID := targetTeamID
					teamIDPtr = &teamID
				}
				existing, ok := workflowByName[workflowIdentityKey(name, teamIDPtr)]
				if !ok {
					existing = model.PMWorkflow{WorkspaceID: workspaceID, Name: name, TeamID: teamIDPtr}
					if err := tx.WithContext(ctx).Create(&existing).Error; err != nil {
						return nil, nil, nil, 0, 0, fmt.Errorf("create workflow: %w", err)
					}
					workflowByName[workflowIdentityKey(name, teamIDPtr)] = existing
					workflowsCreated++
				}
				for _, key := range keys {
					workflowMap[shortcutWorkflowTeamKey(key, targetTeamID)] = existing.ID
					if targetTeamID == "" {
						workflowMap[key] = existing.ID
					}
				}

				var states []model.PMWorkflowState
				if err := tx.WithContext(ctx).Where("workflow_id = ?", existing.ID).Find(&states).Error; err != nil {
					return nil, nil, nil, 0, 0, fmt.Errorf("list workflow states: %w", err)
				}
				stateByName := map[string]model.PMWorkflowState{}
				for _, state := range states {
					stateByName[normalizeShortcutName(state.Name)] = state
				}
				var defaultStateID *string
				for _, stateMapping := range mapping.States {
					stateName := strings.TrimSpace(stateMapping.NewStateName)
					if stateName == "" {
						stateName = stateMapping.ShortcutState
					}
					pos := stateMapping.Position
					state, ok := stateByName[normalizeShortcutName(stateName)]
					if ok {
						if err := tx.WithContext(ctx).Model(&model.PMWorkflowState{}).Where("id = ?", state.ID).Updates(map[string]interface{}{
							"state_type": stateMapping.StateType,
							"position":   pos,
						}).Error; err != nil {
							return nil, nil, nil, 0, 0, fmt.Errorf("update workflow state: %w", err)
						}
					} else {
						state = model.PMWorkflowState{
							WorkflowID: existing.ID,
							Name:       stateName,
							StateType:  stateMapping.StateType,
							Position:   pos,
						}
						if err := tx.WithContext(ctx).Create(&state).Error; err != nil {
							return nil, nil, nil, 0, 0, fmt.Errorf("create workflow state: %w", err)
						}
						stateByName[normalizeShortcutName(stateName)] = state
						statesCreated++
					}
					if defaultStateID == nil && stateMapping.StateType != model.PMStateTypeDone {
						defaultStateID = &state.ID
					}
					for _, key := range keys {
						shortcutStateMap[shortcutWorkflowStateKey(shortcutWorkflowTeamKey(key, targetTeamID), stateMapping.ShortcutState)] = state.ID
						if targetTeamID == "" {
							shortcutStateMap[shortcutWorkflowStateKey(key, stateMapping.ShortcutState)] = state.ID
						}
					}
				}
				if defaultStateID != nil {
					if err := tx.WithContext(ctx).Model(&model.PMWorkflow{}).Where("id = ?", existing.ID).Update("default_state_id", *defaultStateID).Error; err != nil {
						return nil, nil, nil, 0, 0, fmt.Errorf("set workflow default state: %w", err)
					}
					workflowDefaultStateMap[existing.ID] = *defaultStateID
				} else if existing.DefaultStateID != nil && *existing.DefaultStateID != "" {
					workflowDefaultStateMap[existing.ID] = *existing.DefaultStateID
				}
			}
		case "use_existing":
			var workflow model.PMWorkflow
			if err := tx.WithContext(ctx).Where("id = ? AND workspace_id = ?", mapping.ExistingWorkflowID, workspaceID).First(&workflow).Error; err != nil {
				return nil, nil, nil, 0, 0, fmt.Errorf("existing workflow not found for %s", mapping.ShortcutWorkflowName)
			}
			for _, key := range keys {
				workflowMap[key] = workflow.ID
				for _, targetTeamID := range shortcutWorkflowTargetTeamIDs(keys, workflowTeamIDs) {
					workflowMap[shortcutWorkflowTeamKey(key, targetTeamID)] = workflow.ID
				}
			}
			var states []model.PMWorkflowState
			if err := tx.WithContext(ctx).Where("workflow_id = ?", workflow.ID).Find(&states).Error; err != nil {
				return nil, nil, nil, 0, 0, fmt.Errorf("list existing workflow states: %w", err)
			}
			validStates := map[string]struct{}{}
			if workflow.DefaultStateID != nil && *workflow.DefaultStateID != "" {
				workflowDefaultStateMap[workflow.ID] = *workflow.DefaultStateID
			}
			for _, state := range states {
				validStates[state.ID] = struct{}{}
				if _, ok := workflowDefaultStateMap[workflow.ID]; !ok && state.StateType != model.PMStateTypeDone {
					workflowDefaultStateMap[workflow.ID] = state.ID
				}
			}
			for _, stateMapping := range mapping.States {
				if _, ok := validStates[stateMapping.ExistingStateID]; !ok {
					return nil, nil, nil, 0, 0, fmt.Errorf("state %s does not belong to workflow %s", stateMapping.ExistingStateID, workflow.ID)
				}
				for _, key := range keys {
					shortcutStateMap[shortcutWorkflowStateKey(key, stateMapping.ShortcutState)] = stateMapping.ExistingStateID
					for _, targetTeamID := range shortcutWorkflowTargetTeamIDs(keys, workflowTeamIDs) {
						shortcutStateMap[shortcutWorkflowStateKey(shortcutWorkflowTeamKey(key, targetTeamID), stateMapping.ShortcutState)] = stateMapping.ExistingStateID
					}
				}
			}
		default:
			return nil, nil, nil, 0, 0, fmt.Errorf("unsupported workflow mapping mode: %s", mapping.Mode)
		}
	}
	return workflowMap, shortcutStateMap, workflowDefaultStateMap, workflowsCreated, statesCreated, nil
}

func (s *PMImportService) ensureLabels(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, enrichment *shortcutAPIEnrichment) (map[string]string, int, error) {
	needed := map[string]string{}
	for _, row := range rows {
		for _, label := range append(shortcutLabelNames(row.Labels), shortcutLabelNames(row.EpicLabels)...) {
			needed[normalizeShortcutName(label)] = label
		}
	}

	// Build name→color map from Shortcut API labels
	apiLabelColors := map[string]string{}
	if enrichment != nil {
		for _, l := range enrichment.Labels {
			if l.Color != "" {
				apiLabelColors[normalizeShortcutName(l.Name)] = l.Color
			}
		}
	}

	var existing []model.PMLabel
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existing).Error; err != nil {
		return nil, 0, fmt.Errorf("list labels: %w", err)
	}
	labelMap := map[string]string{}
	for _, label := range existing {
		if label.Color == nil || strings.TrimSpace(*label.Color) == "" {
			color := resolveShortcutLabelColor(label.Name, apiLabelColors)
			if err := tx.WithContext(ctx).Model(&model.PMLabel{}).Where("id = ?", label.ID).Update("color", *color).Error; err != nil {
				return nil, 0, fmt.Errorf("backfill label color: %w", err)
			}
		}
		labelMap[normalizeShortcutName(label.Name)] = label.ID
	}
	created := 0
	for key, name := range needed {
		if _, ok := labelMap[key]; ok {
			continue
		}
		label := model.PMLabel{WorkspaceID: workspaceID, Name: name, Color: resolveShortcutLabelColor(name, apiLabelColors)}
		if err := tx.WithContext(ctx).Create(&label).Error; err != nil {
			return nil, 0, fmt.Errorf("create label: %w", err)
		}
		labelMap[key] = label.ID
		created++
	}
	return labelMap, created, nil
}

func shortcutImportLabelColor(name string) *string {
	if len(shortcutImportLabelColors) == 0 {
		return nil
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(normalizeShortcutName(name)))
	color := shortcutImportLabelColors[hasher.Sum32()%uint32(len(shortcutImportLabelColors))]
	return &color
}

func resolveShortcutLabelColor(name string, apiColors map[string]string) *string {
	if c, ok := apiColors[normalizeShortcutName(name)]; ok && c != "" {
		return &c
	}
	return shortcutImportLabelColor(name)
}

func (s *PMImportService) ensureObjectives(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, enrichment *shortcutAPIEnrichment) (map[string]string, int, error) {
	grouped := map[string]shortcutCSVRow{}
	for _, row := range rows {
		if row.ObjectiveID != "" {
			if _, ok := grouped[row.ObjectiveID]; !ok {
				grouped[row.ObjectiveID] = row
			}
		}
	}
	existingMap, err := s.lookupObjectivesByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, err
	}
	created := 0
	for externalID, row := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		state := model.PMObjectiveStateNotStarted
		switch normalizeShortcutName(row.ObjectiveState) {
		case "in progress":
			state = model.PMObjectiveStateActive
		case "done":
			state = model.PMObjectiveStateClosed
		}
		ext := externalID
		obj := model.PMObjective{
			WorkspaceID:      workspaceID,
			Name:             fallbackName(row.Objective, fmt.Sprintf("Objective %s", externalID)),
			ExternalID:       &ext,
			State:            state,
			PlannedStartDate: parseShortcutTimestamp(row.ObjectiveStartedAt, row.UTCOffset),
			Deadline:         parseShortcutTimestamp(row.ObjectiveDueDate, row.UTCOffset),
			CreatedAt:        valueOrNow(parseShortcutTimestamp(row.ObjectiveCreatedAt, row.UTCOffset)),
			UpdatedAt:        valueOrNow(parseShortcutTimestamp(row.ObjectiveCreatedAt, row.UTCOffset)),
		}
		// Enrich with description from API
		if enrichment != nil {
			if apiObj, ok := enrichment.Objectives[externalID]; ok && apiObj.Description != "" {
				desc := normalizeShortcutDescription(apiObj.Description)
				obj.Description = &desc
			}
		}
		if err := tx.WithContext(ctx).Create(&obj).Error; err != nil {
			return nil, 0, fmt.Errorf("create objective: %w", err)
		}
		existingMap[externalID] = obj.ID
		created++
	}
	return existingMap, created, nil
}

func (s *PMImportService) ensureEpics(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, objectiveMap, labelMap, teamMap map[string]string, enrichment *shortcutAPIEnrichment) (map[string]string, int, error) {
	grouped := map[string][]shortcutCSVRow{}
	for _, row := range rows {
		if row.EpicID != "" {
			grouped[row.EpicID] = append(grouped[row.EpicID], row)
		}
	}
	existingMap, err := s.lookupEpicsByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, err
	}
	epicStates, err := s.ensureEpicWorkflowStates(ctx, tx, workspaceID)
	if err != nil {
		return nil, 0, err
	}
	var startedStateID, doneStateID *string
	for _, state := range epicStates {
		switch state.StateType {
		case model.PMStateTypeStarted:
			if startedStateID == nil {
				startedStateID = &state.ID
			}
		case model.PMStateTypeDone:
			if doneStateID == nil {
				doneStateID = &state.ID
			}
		}
	}
	created := 0
	for externalID, rowsForEpic := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		row := rowsForEpic[0]
		ext := externalID
		teamID := consistentTeamID(rowsForEpic, teamMap)
		epic := model.PMEpic{
			WorkspaceID:      workspaceID,
			Name:             fallbackName(row.Epic, fmt.Sprintf("Epic %s", externalID)),
			ExternalID:       &ext,
			TeamID:           teamID,
			PlannedStartDate: parseShortcutTimestamp(row.EpicPlannedStartDate, row.UTCOffset),
			Deadline:         parseShortcutTimestamp(row.EpicDueDate, row.UTCOffset),
			StartedAt:        parseShortcutTimestamp(row.EpicStartedAt, row.UTCOffset),
			Archived:         parseShortcutBool(row.EpicIsArchived),
			CreatedAt:        valueOrNow(parseShortcutTimestamp(row.EpicCreatedAt, row.UTCOffset)),
			UpdatedAt:        valueOrNow(parseShortcutTimestamp(row.EpicCreatedAt, row.UTCOffset)),
		}
		switch normalizeShortcutName(row.EpicState) {
		case "in progress":
			epic.Started = true
			epic.EpicStateID = startedStateID
		case "done":
			epic.Started = true
			epic.Completed = true
			epic.EpicStateID = doneStateID
		default:
			epic.EpicStateID = startedStateID
		}
		// Enrich with description from API
		if enrichment != nil {
			if apiEpic, ok := enrichment.Epics[externalID]; ok && apiEpic.Description != "" {
				desc := normalizeShortcutDescription(apiEpic.Description)
				epic.Description = &desc
			}
		}
		if err := tx.WithContext(ctx).Create(&epic).Error; err != nil {
			return nil, 0, fmt.Errorf("create epic: %w", err)
		}
		existingMap[externalID] = epic.ID
		created++
	}
	for _, rowsForEpic := range grouped {
		row := rowsForEpic[0]
		epicID := existingMap[row.EpicID]
		if row.ObjectiveID != "" {
			if objectiveID, ok := objectiveMap[row.ObjectiveID]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMEpicObjective{
					EpicID:      epicID,
					ObjectiveID: objectiveID,
				}).Error; err != nil {
					return nil, 0, fmt.Errorf("link epic objective: %w", err)
				}
			}
		}
		for _, label := range shortcutLabelNames(row.EpicLabels) {
			if labelID, ok := labelMap[normalizeShortcutName(label)]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMEpicLabel{
					EpicID:  epicID,
					LabelID: labelID,
				}).Error; err != nil {
					return nil, 0, fmt.Errorf("link epic label: %w", err)
				}
			}
		}
	}
	return existingMap, created, nil
}

func (s *PMImportService) ensureEpicWorkflowStates(ctx context.Context, tx *gorm.DB, workspaceID string) ([]model.PMEpicWorkflowState, error) {
	var states []model.PMEpicWorkflowState
	if err := tx.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list epic workflow states: %w", err)
	}
	if len(states) > 0 {
		return states, nil
	}

	color := func(c string) *string { return &c }
	states = []model.PMEpicWorkflowState{
		{WorkspaceID: workspaceID, Name: "To Do", StateType: model.PMStateTypeUnstarted, Position: 0, Color: color("#9ca3af"), IsDefault: true},
		{WorkspaceID: workspaceID, Name: "In Progress", StateType: model.PMStateTypeStarted, Position: 1, Color: color("#3b82f6"), IsDefault: false},
		{WorkspaceID: workspaceID, Name: "Done", StateType: model.PMStateTypeDone, Position: 2, Color: color("#22c55e"), IsDefault: false},
	}
	if err := tx.WithContext(ctx).Create(&states).Error; err != nil {
		return nil, fmt.Errorf("seed default epic states: %w", err)
	}
	return states, nil
}

func (s *PMImportService) ensureSprints(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, teamMap map[string]string, enrichment *shortcutAPIEnrichment) (map[string]string, int, []string, error) {
	grouped := map[string][]shortcutCSVRow{}
	for _, row := range rows {
		if row.IterationID != "" {
			grouped[row.IterationID] = append(grouped[row.IterationID], row)
		}
	}
	existingMap, err := s.lookupSprintsByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, nil, err
	}
	created := 0
	nullDateCount := 0
	for externalID, rowsForSprint := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		row := rowsForSprint[0]

		// Prefer real dates from Shortcut API, fall back to name inference
		var startDate, endDate *time.Time
		if enrichment != nil {
			if apiIter, ok := enrichment.Iterations[externalID]; ok {
				startDate = parseShortcutDateOnly(apiIter.StartDate)
				endDate = parseShortcutDateOnly(apiIter.EndDate)
			}
		}
		if startDate == nil || endDate == nil {
			startDate, endDate = inferShortcutSprintDates(row.Iteration, rowsForSprint)
		}
		if startDate == nil || endDate == nil {
			nullDateCount++
		}

		teamID := consistentTeamID(rowsForSprint, teamMap)
		ext := externalID
		sprint := model.PMSprint{
			WorkspaceID: workspaceID,
			Name:        fallbackName(row.Iteration, fmt.Sprintf("Sprint %s", externalID)),
			ExternalID:  &ext,
			StartDate:   startDate,
			EndDate:     endDate,
			TeamID:      teamID,
			Archived:    false,
			CreatedAt:   valueOrNow(parseShortcutTimestamp(row.CreatedAt, row.UTCOffset)),
			UpdatedAt:   valueOrNow(parseShortcutTimestamp(row.UpdatedAt, row.UTCOffset)),
		}
		if err := tx.WithContext(ctx).Create(&sprint).Error; err != nil {
			return nil, 0, nil, fmt.Errorf("create sprint: %w", err)
		}
		existingMap[externalID] = sprint.ID
		created++
	}
	warnings := []string{}
	if nullDateCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imported sprints had null dates because iteration names could not be parsed", nullDateCount))
	}
	return existingMap, created, warnings, nil
}

func (s *PMImportService) createTasks(ctx context.Context, tx *gorm.DB, workspaceID, actorID string, rows []shortcutCSVRow, workflowMap, stateMap, workflowDefaultStateMap, teamMap, userByEmail, memberByEmail, epicMap, sprintMap, labelMap map[string]string, result *model.ShortcutImportResult, jobID string, stepOffset, totalSteps int) ([]string, error) {
	existingTasks, err := s.lookupTasksByExternalID(ctx, tx, workspaceID, mapKeys(shortcutStoryExternalIDs(rows)))
	if err != nil {
		return nil, err
	}
	maxDisplayID, err := s.maxTaskDisplayID(ctx, tx, workspaceID)
	if err != nil {
		return nil, err
	}
	unmappedRequesterCount := 0
	unmappedOwnerCounts := map[string]int{}
	stateFallbackCounts := map[string]int{}
	processed := 0
	createdTaskIDs := make([]string, 0)

	for _, row := range rows {
		teamID, workflowID, stateID, err := resolveShortcutStoryPlacement(row, workflowMap, stateMap, workflowDefaultStateMap, teamMap, stateFallbackCounts)
		if err != nil {
			return nil, err
		}
		epicID := mappedEntityID(row.EpicID, epicMap)
		sprintID := mappedEntityID(row.IterationID, sprintMap)
		if existingID, ok := existingTasks[row.ID]; ok {
			if err := s.updateExistingShortcutStoryPlacement(ctx, tx, workspaceID, existingID, teamID, epicID, sprintID, workflowID, stateID); err != nil {
				return nil, err
			}
			if len(row.APIExternalLinks) > 0 {
				now := time.Now().UTC()
				if err := s.createAPITaskExternalLinks(ctx, tx, actorID, model.PMTask{
					ID:        existingID,
					CreatedAt: now,
					UpdatedAt: now,
				}, row.APIExternalLinks, result); err != nil {
					return nil, err
				}
			}
			result.TasksSkipped++
			processed++
			if processed%100 == 0 {
				_ = s.markStep(ctx, jobID, "tasks", 7+stepOffset, processed, totalSteps)
			}
			continue
		}

		maxDisplayID++
		description := normalizeShortcutDescription(row.Description)
		var descriptionPtr *string
		if strings.TrimSpace(description) != "" {
			descriptionPtr = &description
		}

		var requesterID *string
		var requesterMemberID *string
		if row.Requester != "" {
			norm := normalizeShortcutName(row.Requester)
			if id, ok := userByEmail[norm]; ok && id != "" {
				requesterID = &id
			}
			if mid, ok := memberByEmail[norm]; ok && mid != "" {
				requesterMemberID = &mid
			}
			if requesterID == nil && requesterMemberID == nil {
				unmappedRequesterCount++
			}
		}

		ownerIDs := make([]string, 0)
		for _, email := range shortcutOwnerEmails(row.Owners) {
			norm := normalizeShortcutName(email)
			if id, ok := userByEmail[norm]; ok && id != "" {
				if !containsString(ownerIDs, id) {
					ownerIDs = append(ownerIDs, id)
				}
			}
			if _, hasUser := userByEmail[norm]; !hasUser {
				if _, hasMember := memberByEmail[norm]; !hasMember {
					unmappedOwnerCounts[email]++
				}
			}
		}
		priority := mapShortcutPriority(row.Priority)
		severity := mapShortcutSeverity(row.Severity)
		startedAt := parseShortcutTimestamp(row.StartedAt, row.UTCOffset)
		completedAt := parseShortcutTimestamp(row.CompletedAt, row.UTCOffset)
		movedAt := parseShortcutTimestamp(row.MovedAt, row.UTCOffset)
		completed := parseShortcutBool(row.IsCompleted)
		started := startedAt != nil || completedAt != nil || completed
		externalID := row.ID
		task := model.PMTask{
			WorkspaceID:       workspaceID,
			DisplayID:         maxDisplayID,
			Name:              fallbackName(row.Name, fmt.Sprintf("Untitled Task (SC-%s)", row.ID)),
			Description:       descriptionPtr,
			TaskType:          mapShortcutStoryType(row.Type),
			WorkflowID:        workflowID,
			WorkflowStateID:   stateID,
			EpicID:            epicID,
			SprintID:          sprintID,
			TeamID:            teamID,
			RequesterID:       requesterID,
			RequesterMemberID: requesterMemberID,
			Estimate:          parseShortcutInt(row.Estimate),
			Priority:          priority,
			Severity:          severity,
			Deadline:          parseShortcutTimestamp(row.DueDate, row.UTCOffset),
			Started:           started,
			StartedAt:         startedAt,
			Completed:         completed,
			CompletedAt:       completedAt,
			MovedAt:           movedAt,
			Blocked:           parseShortcutBool(row.IsBlocked),
			Archived:          parseShortcutBool(row.IsArchived),
			ExternalID:        &externalID,
			CreatedAt:         valueOrNow(parseShortcutTimestamp(row.CreatedAt, row.UTCOffset)),
			UpdatedAt:         valueOrNow(parseShortcutTimestamp(row.UpdatedAt, row.UTCOffset)),
		}
		if err := tx.WithContext(ctx).Create(&task).Error; err != nil {
			return nil, fmt.Errorf("create task %s: %w", row.ID, err)
		}
		existingTasks[row.ID] = task.ID
		createdTaskIDs = append(createdTaskIDs, task.ID)
		result.TasksCreated++

		if len(row.APIExternalLinks) > 0 {
			if err := s.createAPITaskExternalLinks(ctx, tx, actorID, task, row.APIExternalLinks, result); err != nil {
				return nil, err
			}
		}

		for _, owner := range ownerIDs {
			if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMTaskOwner{
				TaskID: task.ID,
				UserID: owner,
			}).Error; err != nil {
				return nil, fmt.Errorf("link task owner: %w", err)
			}
			result.OwnerLinksCreated++
		}

		for _, label := range shortcutLabelNames(row.Labels) {
			if labelID, ok := labelMap[normalizeShortcutName(label)]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMTaskLabel{
					TaskID:  task.ID,
					LabelID: labelID,
				}).Error; err != nil {
					return nil, fmt.Errorf("link task label: %w", err)
				}
				result.LabelLinksCreated++
			}
		}

		for idx, item := range parseShortcutChecklist(row.Tasks) {
			checklist := model.PMChecklistItem{
				TaskID:    task.ID,
				Text:      item.Text,
				Completed: item.Completed,
				Position:  idx,
				CreatedAt: task.CreatedAt,
				UpdatedAt: task.UpdatedAt,
			}
			if err := tx.WithContext(ctx).Create(&checklist).Error; err != nil {
				return nil, fmt.Errorf("create checklist item: %w", err)
			}
			result.ChecklistItemsCreated++
		}

		processed++
		if processed%100 == 0 {
			_ = s.markStep(ctx, jobID, "tasks", 7+stepOffset, processed, totalSteps)
		}
	}

	if unmappedRequesterCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d tasks had unmapped requester emails — requester_id set to null", unmappedRequesterCount))
	}
	if len(unmappedOwnerCounts) > 0 {
		type pair struct {
			Email string
			Count int
		}
		pairs := make([]pair, 0, len(unmappedOwnerCounts))
		for email, count := range unmappedOwnerCounts {
			pairs = append(pairs, pair{Email: email, Count: count})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Count > pairs[j].Count })
		for _, p := range pairs[:min(3, len(pairs))] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Owner email '%s' not mapped — %d owner links skipped", p.Email, p.Count))
		}
	}
	if len(stateFallbackCounts) > 0 {
		type pair struct {
			Key   string
			Count int
		}
		pairs := make([]pair, 0, len(stateFallbackCounts))
		for key, count := range stateFallbackCounts {
			pairs = append(pairs, pair{Key: key, Count: count})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Count > pairs[j].Count })
		for _, p := range pairs[:min(5, len(pairs))] {
			parts := strings.SplitN(p.Key, "::", 2)
			workflowLabel := parts[0]
			stateLabel := ""
			if len(parts) == 2 {
				stateLabel = parts[1]
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("%d tasks in workflow %q used the default state because Shortcut state %q could not be mapped", p.Count, workflowLabel, stateLabel))
		}
	}
	if err := s.createAPIStoryLinks(ctx, tx, workspaceID, actorID, rows, existingTasks, result); err != nil {
		return nil, err
	}
	return createdTaskIDs, nil
}

func resolveShortcutStoryPlacement(row shortcutCSVRow, workflowMap, stateMap, workflowDefaultStateMap, teamMap map[string]string, stateFallbackCounts map[string]int) (*string, string, string, error) {
	teamID := mappedTeamID(row.Team, teamMap)
	teamIDValue := ""
	if teamID != nil {
		teamIDValue = *teamID
	}
	workflowID := firstMappedValue(workflowMap, shortcutWorkflowTeamLookupKeys(row.WorkflowID, row.Workflow, teamIDValue))
	if workflowID == "" {
		return nil, "", "", fmt.Errorf("no workflow/state mapping found for %s / %s", row.Workflow, row.State)
	}
	stateID := firstMappedValue(stateMap, shortcutWorkflowStateTeamLookupKeys(row.WorkflowID, row.Workflow, row.State, teamIDValue))
	if stateID == "" {
		if defaultStateID := workflowDefaultStateMap[workflowID]; defaultStateID != "" {
			stateID = defaultStateID
			stateFallbackCounts[fallbackStateWarningKey(row.WorkflowID, row.Workflow, row.State)]++
		} else {
			return nil, "", "", fmt.Errorf("no workflow/state mapping found for %s / %s", row.Workflow, row.State)
		}
	}
	return teamID, workflowID, stateID, nil
}

func (s *PMImportService) updateExistingShortcutStoryPlacement(ctx context.Context, tx *gorm.DB, workspaceID, taskID string, teamID, epicID, sprintID *string, workflowID, stateID string) error {
	if err := tx.WithContext(ctx).Model(&model.PMTask{}).
		Where("id = ? AND workspace_id = ?", taskID, workspaceID).
		Updates(map[string]interface{}{
			"team_id":           teamID,
			"epic_id":           epicID,
			"sprint_id":         sprintID,
			"workflow_id":       workflowID,
			"workflow_state_id": stateID,
		}).Error; err != nil {
		return fmt.Errorf("update existing task placement: %w", err)
	}
	return nil
}

func (s *PMImportService) importShortcutComments(ctx context.Context, client *ShortcutAPIClient, workspaceID, actorID string, rows []shortcutCSVRow, scMemberToUser map[string]string, enrichment *shortcutAPIEnrichment, apiToken, jobID string, stepNum, totalSteps int) (int, int, []string) {
	// Lookup existing tasks by external ID so we can attach comments to the right task
	var taskRows []struct {
		ID         string
		ExternalID string
	}
	externalIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			externalIDs = append(externalIDs, row.ID)
		}
	}
	if len(externalIDs) == 0 {
		return 0, 0, nil
	}
	if err := s.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&taskRows).Error; err != nil {
		return 0, 0, []string{fmt.Sprintf("Failed to look up tasks for comments: %s", err.Error())}
	}
	taskMap := make(map[string]string, len(taskRows)) // external_id → helpin task ID
	for _, sr := range taskRows {
		taskMap[sr.ExternalID] = sr.ID
	}

	commentsCreated := 0
	attachmentsCreated := 0
	var warnings []string
	fetchErrors := 0
	processed := 0

	for _, row := range rows {
		taskID, ok := taskMap[row.ID]
		if !ok || row.ID == "" {
			processed++
			continue
		}

		comments := row.APIComments
		if !row.APICommentsFetched {
			var err error
			comments, err = client.ListStoryComments(ctx, row.ID)
			if err != nil {
				fetchErrors++
				if fetchErrors <= 3 {
					warnings = append(warnings, fmt.Sprintf("Failed to fetch comments for task %s: %s", row.ID, err.Error()))
				}
				processed++
				continue
			}
		}

		// Two-pass: create top-level comments first, then replies (for parent_id mapping)
		scCommentIDMap := map[int]string{} // Shortcut comment ID → Helpin comment ID

		// Pass 1: top-level comments
		for _, c := range comments {
			if c.Deleted || c.ParentID != nil {
				continue
			}
			body := normalizeShortcutDescription(c.Text)
			if strings.TrimSpace(body) == "" {
				continue
			}
			authorID := scMemberToUser[c.AuthorID]
			if authorID == "" {
				authorID = actorID
				if authorName := shortcutMemberName(enrichment, c.AuthorID); authorName != "" {
					body = fmt.Sprintf(`<p><em>Comment by %s (imported from Shortcut)</em></p>`, authorName) + body
				}
			}
			if existingID := s.findImportedShortcutComment(ctx, taskID, body, parseShortcutAPITimestamp(c.CreatedAt)); existingID != "" {
				scCommentIDMap[c.ID] = existingID
				continue
			}
			comment := model.PMComment{
				EntityType: "task",
				EntityID:   taskID,
				AuthorID:   authorID,
				Body:       body,
				CreatedAt:  valueOrNow(parseShortcutAPITimestamp(c.CreatedAt)),
				UpdatedAt:  valueOrNow(parseShortcutAPITimestamp(c.UpdatedAt)),
			}
			if err := s.db.WithContext(ctx).Create(&comment).Error; err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to create comment for task %s: %s", row.ID, err.Error()))
				continue
			}
			rewrittenBody, created, mediaWarnings := s.rewriteShortcutMediaBody(ctx, workspaceID, actorID, "comment", comment.ID, comment.Body, apiToken)
			attachmentsCreated += created
			warnings = appendUniqueWarnings(warnings, mediaWarnings)
			if rewrittenBody != comment.Body {
				if err := s.db.WithContext(ctx).Model(&model.PMComment{}).Where("id = ?", comment.ID).UpdateColumn("body", rewrittenBody).Error; err != nil {
					warnings = append(warnings, fmt.Sprintf("Failed to update imported comment media for task %s: %s", row.ID, err.Error()))
				}
			}
			scCommentIDMap[c.ID] = comment.ID
			commentsCreated++
		}

		// Pass 2: replies (threaded comments)
		for _, c := range comments {
			if c.Deleted || c.ParentID == nil {
				continue
			}
			body := normalizeShortcutDescription(c.Text)
			if strings.TrimSpace(body) == "" {
				continue
			}
			authorID := scMemberToUser[c.AuthorID]
			if authorID == "" {
				authorID = actorID
				if authorName := shortcutMemberName(enrichment, c.AuthorID); authorName != "" {
					body = fmt.Sprintf(`<p><em>Comment by %s (imported from Shortcut)</em></p>`, authorName) + body
				}
			}
			if existingID := s.findImportedShortcutComment(ctx, taskID, body, parseShortcutAPITimestamp(c.CreatedAt)); existingID != "" {
				scCommentIDMap[c.ID] = existingID
				continue
			}
			comment := model.PMComment{
				EntityType: "task",
				EntityID:   taskID,
				AuthorID:   authorID,
				Body:       body,
				CreatedAt:  valueOrNow(parseShortcutAPITimestamp(c.CreatedAt)),
				UpdatedAt:  valueOrNow(parseShortcutAPITimestamp(c.UpdatedAt)),
			}
			if parentHelpin, ok := scCommentIDMap[*c.ParentID]; ok {
				comment.ParentID = &parentHelpin
			}
			if err := s.db.WithContext(ctx).Create(&comment).Error; err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to create reply for task %s: %s", row.ID, err.Error()))
				continue
			}
			rewrittenBody, created, mediaWarnings := s.rewriteShortcutMediaBody(ctx, workspaceID, actorID, "comment", comment.ID, comment.Body, apiToken)
			attachmentsCreated += created
			warnings = appendUniqueWarnings(warnings, mediaWarnings)
			if rewrittenBody != comment.Body {
				if err := s.db.WithContext(ctx).Model(&model.PMComment{}).Where("id = ?", comment.ID).UpdateColumn("body", rewrittenBody).Error; err != nil {
					warnings = append(warnings, fmt.Sprintf("Failed to update imported comment media for task %s: %s", row.ID, err.Error()))
				}
			}
			scCommentIDMap[c.ID] = comment.ID
			commentsCreated++
		}

		processed++
		if processed%50 == 0 {
			_ = s.markStep(ctx, jobID, "comments", stepNum, processed, totalSteps)
		}
	}

	if fetchErrors > 3 {
		warnings = append(warnings, fmt.Sprintf("... and %d more comment fetch errors", fetchErrors-3))
	}
	return commentsCreated, attachmentsCreated, warnings
}

func (s *PMImportService) findImportedShortcutComment(ctx context.Context, taskID, body string, createdAt *time.Time) string {
	var existing model.PMComment
	q := s.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND body = ?", "task", taskID, body)
	if createdAt != nil {
		q = q.Where("created_at = ?", *createdAt)
	}
	if err := q.First(&existing).Error; err == nil {
		return existing.ID
	}
	return ""
}

func parseShortcutAPITimestamp(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z"} {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			utc := ts.UTC()
			return &utc
		}
	}
	return nil
}

func (s *PMImportService) countTaskDuplicates(ctx context.Context, workspaceID string, externalIDs []string) (int, error) {
	if len(externalIDs) == 0 {
		return 0, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count duplicate tasks: %w", err)
	}
	return int(count), nil
}

func (s *PMImportService) lookupTasksByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMTask{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup tasks by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupEpicsByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMEpic{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup epics by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupObjectivesByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMObjective{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup objectives by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupSprintsByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMSprint{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup sprints by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) maxTaskDisplayID(ctx context.Context, tx *gorm.DB, workspaceID string) (int, error) {
	var maxID int
	if err := tx.WithContext(ctx).Model(&model.PMTask{}).
		Where("workspace_id = ?", workspaceID).
		Select("COALESCE(MAX(display_id), 0)").
		Scan(&maxID).Error; err != nil {
		return 0, fmt.Errorf("load max display id: %w", err)
	}
	return maxID, nil
}

func (s *PMImportService) requireWorkspaceAdmin(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return fmt.Errorf("workspace_id and user_id are required")
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return fmt.Errorf("workspace admin access required")
	}
	return nil
}

func (s *PMImportService) updateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	if strings.TrimSpace(jobID) == "" {
		return nil
	}
	query := s.db.WithContext(ctx).Model(&model.PMImportJob{}).Where("id = ?", jobID)
	if status, ok := updates["status"].(string); !ok || status != model.PMImportStatusCanceled {
		query = query.Where("status <> ?", model.PMImportStatusCanceled)
	}
	return query.Updates(updates).Error
}

func (s *PMImportService) markStep(ctx context.Context, jobID, step string, completed, entitiesProcessed, totalSteps int) error {
	if totalSteps <= 0 {
		totalSteps = 1
	}
	progress := completed * 100 / totalSteps
	return s.updateJob(ctx, jobID, map[string]interface{}{
		"current_step":       step,
		"steps_completed":    completed,
		"progress":           progress,
		"entities_processed": entitiesProcessed,
		"updated_at":         time.Now().UTC(),
	})
}

func (s *PMImportService) setCurrentStep(ctx context.Context, jobID, step string) error {
	return s.updateJob(ctx, jobID, map[string]interface{}{
		"current_step": step,
		"updated_at":   time.Now().UTC(),
	})
}

func (s *PMImportService) shortcutImportTotalSteps(apiToken string, options ...model.ShortcutImportOptions) int {
	extraSteps := 0
	if len(options) > 0 && options[0].ImportDocs {
		extraSteps++
	}
	if apiToken != "" {
		return 11 + extraSteps
	}
	return 8 + extraSteps
}

func filterShortcutRows(rows []shortcutCSVRow, options model.ShortcutImportOptions) []shortcutCSVRow {
	filtered := make([]shortcutCSVRow, 0, len(rows))
	for _, row := range rows {
		if !options.ImportArchived && parseShortcutBool(row.IsArchived) {
			continue
		}
		if !options.ImportCompleted && parseShortcutBool(row.IsCompleted) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func shortcutStoryExternalIDs(rows []shortcutCSVRow) map[string]struct{} {
	out := map[string]struct{}{}
	for _, row := range rows {
		if row.ID != "" {
			out[row.ID] = struct{}{}
		}
	}
	return out
}

func mapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func countWorkflowStates(workflows map[string]*shortcutWorkflowAggregate) int {
	total := 0
	for _, workflow := range workflows {
		total += len(workflow.StateCounts)
	}
	return total
}

func valueOrNow(ts *time.Time) time.Time {
	if ts != nil {
		return *ts
	}
	return time.Now().UTC()
}

func timePtr(ts time.Time) *time.Time {
	return &ts
}

func shortcutMemberName(enrichment *shortcutAPIEnrichment, memberID string) string {
	if enrichment == nil {
		return ""
	}
	if m, ok := enrichment.Members[memberID]; ok && m.Profile.Name != "" {
		return m.Profile.Name
	}
	return ""
}

func fallbackName(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return strings.TrimSpace(name)
}

func mappedTeamID(teamName string, teamMap map[string]string) *string {
	if teamName == "" {
		return nil
	}
	if id, ok := teamMap[normalizeShortcutName(teamName)]; ok {
		return &id
	}
	return nil
}

func consistentTeamID(rows []shortcutCSVRow, teamMap map[string]string) *string {
	var current string
	for _, row := range rows {
		if row.Team == "" {
			continue
		}
		id, ok := teamMap[normalizeShortcutName(row.Team)]
		if !ok {
			continue
		}
		if current == "" {
			current = id
			continue
		}
		if current != id {
			return nil
		}
	}
	if current == "" {
		return nil
	}
	return &current
}

func mappedEntityID(externalID string, entityMap map[string]string) *string {
	if externalID == "" {
		return nil
	}
	if id, ok := entityMap[externalID]; ok {
		return &id
	}
	return nil
}

type shortcutWorkflowAggregate struct {
	ID          string
	Name        string
	TaskCount   int
	StateCounts map[string]int
}

func shortcutWorkflowDisplayName(workflowID, workflowName string) string {
	name := strings.TrimSpace(workflowName)
	if name != "" {
		return name
	}
	id := strings.TrimSpace(workflowID)
	if id != "" {
		return fmt.Sprintf("Workflow %s", id)
	}
	return "Untitled Workflow"
}

func shortcutWorkflowKey(workflowID, workflowName string) string {
	return shortcutWorkflowLookupKeys(workflowID, workflowName)[0]
}

func shortcutWorkflowLookupKeys(workflowID, workflowName string) []string {
	keys := make([]string, 0, 2)
	if id := normalizeShortcutName(workflowID); id != "" {
		keys = append(keys, "id:"+id)
	}
	if name := normalizeShortcutName(workflowName); name != "" {
		keys = append(keys, "name:"+name)
	}
	if len(keys) == 0 {
		return []string{"name:"}
	}
	return keys
}

func shortcutWorkflowMappingKeys(workflowID, workflowName string) []string {
	return shortcutWorkflowLookupKeys(workflowID, workflowName)
}

func shortcutWorkflowTeamKey(workflowKey, teamID string) string {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return workflowKey
	}
	return workflowKey + "::team:" + teamID
}

func shortcutWorkflowTeamLookupKeys(workflowID, workflowName, teamID string) []string {
	keys := shortcutWorkflowLookupKeys(workflowID, workflowName)
	out := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		out = append(out, shortcutWorkflowTeamKey(key, teamID))
	}
	if strings.TrimSpace(teamID) != "" {
		out = append(out, keys...)
	}
	return out
}

func shortcutWorkflowStateKey(workflowKey, stateName string) string {
	return workflowKey + "::" + normalizeShortcutName(stateName)
}

func shortcutWorkflowStateTeamLookupKeys(workflowID, workflowName, stateName, teamID string) []string {
	keys := shortcutWorkflowTeamLookupKeys(workflowID, workflowName, teamID)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, shortcutWorkflowStateKey(key, stateName))
	}
	return out
}

func workflowIdentityKey(name string, teamID *string) string {
	id := ""
	if teamID != nil {
		id = strings.TrimSpace(*teamID)
	}
	return normalizeShortcutName(name) + "::team:" + id
}

func shortcutWorkflowTeamIDs(rows []shortcutCSVRow, teamMap map[string]string) map[string][]string {
	sets := map[string]map[string]struct{}{}
	for _, row := range rows {
		teamID := ""
		if mapped := mappedTeamID(row.Team, teamMap); mapped != nil {
			teamID = *mapped
		}
		for _, key := range shortcutWorkflowLookupKeys(row.WorkflowID, row.Workflow) {
			if sets[key] == nil {
				sets[key] = map[string]struct{}{}
			}
			sets[key][teamID] = struct{}{}
		}
	}
	out := map[string][]string{}
	for key, set := range sets {
		values := make([]string, 0, len(set))
		for teamID := range set {
			values = append(values, teamID)
		}
		sort.Strings(values)
		out[key] = values
	}
	return out
}

func shortcutWorkflowTargetTeamIDs(keys []string, workflowTeamIDs map[string][]string) []string {
	seen := map[string]struct{}{}
	for _, key := range keys {
		for _, teamID := range workflowTeamIDs[key] {
			seen[teamID] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(seen))
	for teamID := range seen {
		out = append(out, teamID)
	}
	sort.Strings(out)
	return out
}

func firstMappedValue(values map[string]string, keys []string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != "" {
			return value
		}
	}
	return ""
}

func fallbackStateWarningKey(workflowID, workflowName, stateName string) string {
	return shortcutWorkflowDisplayName(workflowID, workflowName) + "::" + fallbackName(strings.TrimSpace(stateName), "(blank)")
}

func mapShortcutPriority(raw string) string {
	switch normalizeShortcutName(raw) {
	case "highest":
		return model.PMTaskPriorityUrgent
	case "high":
		return model.PMTaskPriorityHigh
	case "medium":
		return model.PMTaskPriorityMedium
	case "low", "lowest":
		return model.PMTaskPriorityLow
	default:
		return model.PMTaskPriorityNone
	}
}

func mapShortcutSeverity(raw string) string {
	switch normalizeShortcutName(raw) {
	case "severity 0":
		return model.PMTaskSeverityCritical
	case "severity 1":
		return model.PMTaskSeverityMajor
	case "severity 2":
		return model.PMTaskSeverityMinor
	default:
		return model.PMTaskSeverityNone
	}
}

func mapShortcutStoryType(raw string) string {
	switch normalizeShortcutName(raw) {
	case model.PMTaskTypeBug:
		return model.PMTaskTypeBug
	case model.PMTaskTypeChore:
		return model.PMTaskTypeChore
	default:
		return model.PMTaskTypeFeature
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
