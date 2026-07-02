package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	ErrAgentRuntimeHostBadRequest = errors.New("agent runtime host bad request")
	ErrAgentRuntimeHostForbidden  = errors.New("agent runtime host forbidden")
	ErrAgentRuntimeHostNotFound   = errors.New("agent runtime host not found")
)

// AgentRuntimeHostService serves the host callbacks consumed by agent-runtime.
// It exposes runtime contracts only; Helpin lifecycle policy remains in the
// existing projection, billing, and finalizer services.
type AgentRuntimeHostService struct {
	appID          string
	runRepo        *repository.AgentRunRepository
	workspaceRepo  *repository.WorkspaceRepository
	taskRepo       *repository.PMTaskRepository
	epicRepo       *repository.PMEpicRepository
	supportRepo    *repository.SupportConversationRepository
	docsRepo       *repository.DocsDocumentRepository
	crmContactRepo *repository.CRMContactRepository
	crmCompanyRepo *repository.CRMCompanyRepository
	crmDealRepo    *repository.CRMDealRepository
	commandService *InternalCommandService
	gitService     *GitService
}

func NewAgentRuntimeHostService(
	appID string,
	runRepo *repository.AgentRunRepository,
	workspaceRepo *repository.WorkspaceRepository,
	taskRepo *repository.PMTaskRepository,
	epicRepo *repository.PMEpicRepository,
	supportRepo *repository.SupportConversationRepository,
	docsRepo *repository.DocsDocumentRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmCompanyRepo *repository.CRMCompanyRepository,
	crmDealRepo *repository.CRMDealRepository,
	commandService *InternalCommandService,
	gitService *GitService,
) *AgentRuntimeHostService {
	return &AgentRuntimeHostService{
		appID:          strings.TrimSpace(appID),
		runRepo:        runRepo,
		workspaceRepo:  workspaceRepo,
		taskRepo:       taskRepo,
		epicRepo:       epicRepo,
		supportRepo:    supportRepo,
		docsRepo:       docsRepo,
		crmContactRepo: crmContactRepo,
		crmCompanyRepo: crmCompanyRepo,
		crmDealRepo:    crmDealRepo,
		commandService: commandService,
		gitService:     gitService,
	}
}

func (s *AgentRuntimeHostService) ResolveTargetContext(ctx context.Context, req agentruntime.TargetContextRequest) (*agentruntime.TargetContext, error) {
	if s == nil {
		return nil, fmt.Errorf("agent runtime host service is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	target := normalizeRuntimeTarget(req.Target)
	if target.Type == "" || target.ID == "" {
		return nil, fmt.Errorf("%w: target.type and target.id are required", ErrAgentRuntimeHostBadRequest)
	}

	resp := &agentruntime.TargetContext{
		Target: target,
		Data:   map[string]interface{}{},
	}
	requestedWorkspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata)
	if requestedWorkspaceID == "" {
		var err error
		requestedWorkspaceID, err = s.workspaceIDForRuntimeRun(ctx, req.RunID)
		if err != nil {
			return nil, err
		}
	}
	workspaceID := requestedWorkspaceID

	switch target.Type {
	case "workspace":
		workspace, err := s.workspaceRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if workspace == nil {
			return nil, fmt.Errorf("%w: workspace not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = workspace.ID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Workspace: %s", workspace.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: workspace.Name}
		resp.Data = runtimeWorkspaceContextData(workspace)
	case "task", "story":
		task, err := s.taskRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("%w: task not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = task.Task.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Task %d: %s", task.Task.DisplayID, task.Task.Name)
		resp.Target.Type = "task"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: task.Task.Name}
		resp.Data = runtimeTaskContextData(task)
	case "epic":
		epic, err := s.epicRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if epic == nil {
			return nil, fmt.Errorf("%w: epic not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = epic.Epic.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Epic: %s", epic.Epic.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: epic.Epic.Name}
		resp.Data = runtimeEpicContextData(epic)
	case "support_conversation", "conversation":
		if workspaceID == "" {
			return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required for support conversation targets", ErrAgentRuntimeHostBadRequest)
		}
		conversation, err := s.supportRepo.GetByID(ctx, workspaceID, target.ID, "", "")
		if err != nil {
			return nil, err
		}
		if conversation == nil {
			return nil, fmt.Errorf("%w: support conversation not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = conversation.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Support conversation: %s", conversation.Subject)
		resp.Target.Type = "support_conversation"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: conversation.Subject}
		resp.Data = runtimeSupportConversationContextData(conversation)
	case "document":
		doc, err := s.docsRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if doc == nil {
			return nil, fmt.Errorf("%w: document not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = doc.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Document: %s", doc.Title)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: doc.Title}
		resp.Data = runtimeDocumentContextData(doc)
	case "crm_contact", "contact":
		contact, err := s.crmContactRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if contact == nil {
			return nil, fmt.Errorf("%w: CRM contact not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = contact.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		title := strings.TrimSpace(contact.FirstName + " " + agentRuntimeHostString(contact.LastName))
		if title == "" {
			title = agentRuntimeHostString(contact.Email)
		}
		resp.Summary = fmt.Sprintf("CRM contact: %s", title)
		resp.Target.Type = "crm_contact"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: title}
		resp.Data = runtimeCRMContactContextData(contact, title)
	case "crm_company", "company":
		company, err := s.crmCompanyRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if company == nil {
			return nil, fmt.Errorf("%w: CRM company not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = company.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("CRM company: %s", company.Name)
		resp.Target.Type = "crm_company"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: company.Name}
		resp.Data = runtimeCRMCompanyContextData(company)
	case "crm_deal", "deal":
		deal, err := s.crmDealRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if deal == nil {
			return nil, fmt.Errorf("%w: CRM deal not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = deal.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("CRM deal: %s", deal.Name)
		resp.Target.Type = "crm_deal"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: deal.Name}
		resp.Data = runtimeCRMDealContextData(deal)
	default:
		resp.Summary = fmt.Sprintf("%s target %s", target.Type, target.ID)
		resp.Data = map[string]interface{}{
			"target_type": target.Type,
			"target_id":   target.ID,
		}
	}

	if resp.Target.Metadata == nil {
		resp.Target.Metadata = map[string]interface{}{}
	}
	if workspaceID != "" {
		resp.Target.Metadata["workspace_id"] = workspaceID
		resp.Data["workspace_id"] = workspaceID
	}
	return resp, nil
}

func (s *AgentRuntimeHostService) ResolveRepositorySpec(ctx context.Context, req agentruntime.PrepareWorkspaceRequest) (*agentruntime.RepositoryWorkspaceSpec, error) {
	if s == nil || s.gitService == nil {
		return nil, fmt.Errorf("repository workspace provider is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	var contextData map[string]interface{}
	var contextTargetMetadata map[string]interface{}
	if req.TargetContext != nil {
		contextData = req.TargetContext.Data
		contextTargetMetadata = req.TargetContext.Target.Metadata
	}
	workspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata, contextData, contextTargetMetadata)
	mappedWorkspaceID, err := s.workspaceIDForRuntimeRun(ctx, req.RunID)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		workspaceID = mappedWorkspaceID
	} else if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, workspaceID); err != nil {
		return nil, err
	}
	return s.gitService.ResolveAgentRuntimeRepositorySpec(ctx, workspaceID, normalizeRuntimeTarget(req.Target), req.RunID)
}

func (s *AgentRuntimeHostService) ExecuteCommand(ctx context.Context, req agentruntime.CommandExecutionRequest) (*agentruntime.CommandExecutionResponse, error) {
	if s == nil || s.commandService == nil {
		return nil, fmt.Errorf("command service is not configured")
	}
	if err := s.validateAppID(req.Meta.AppID); err != nil {
		return nil, err
	}
	meta := model.InternalCommandContext{
		WorkspaceID: strings.TrimSpace(req.Meta.WorkspaceID),
		ActorID:     strings.TrimSpace(req.Meta.ExternalActorID),
		AgentID:     strings.TrimSpace(req.Meta.AgentID),
		RunID:       strings.TrimSpace(req.Meta.RunID),
		TargetType:  strings.TrimSpace(req.Meta.TargetType),
		TargetID:    strings.TrimSpace(req.Meta.TargetID),
	}
	if meta.TargetType == "" {
		meta.TargetType = strings.TrimSpace(req.Meta.Target.Type)
	}
	if meta.TargetID == "" {
		meta.TargetID = strings.TrimSpace(req.Meta.Target.ID)
	}
	if meta.WorkspaceID == "" {
		meta.WorkspaceID = runtimeWorkspaceID(req.Meta.WorkspaceMetadata, req.Meta.RunInputMetadata, req.Meta.TargetMetadata, req.Meta.Target.Metadata)
	}
	mappedWorkspaceID, err := s.workspaceIDForRuntimeRun(ctx, meta.RunID)
	if err != nil {
		return nil, err
	}
	if meta.WorkspaceID == "" {
		meta.WorkspaceID = mappedWorkspaceID
	} else if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, meta.WorkspaceID); err != nil {
		return nil, err
	}
	if meta.WorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrAgentRuntimeHostBadRequest)
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}
	output, err := s.commandService.Execute(ctx, meta, strings.TrimSpace(req.CommandName), req.Input)
	if err != nil {
		return &agentruntime.CommandExecutionResponse{Error: err.Error()}, nil
	}
	return &agentruntime.CommandExecutionResponse{Output: output}, nil
}

func normalizeRuntimeTarget(target agentruntime.TargetRef) agentruntime.TargetRef {
	target.Type = strings.TrimSpace(target.Type)
	target.ID = strings.TrimSpace(target.ID)
	return target
}

func (s *AgentRuntimeHostService) validateAppID(appID string) error {
	expected := strings.TrimSpace(s.appID)
	if expected == "" {
		return nil
	}
	if strings.TrimSpace(appID) != expected {
		return fmt.Errorf("%w: app_id is not allowed", ErrAgentRuntimeHostForbidden)
	}
	return nil
}

func (s *AgentRuntimeHostService) workspaceIDForRuntimeRun(ctx context.Context, runtimeRunID string) (string, error) {
	runtimeRunID = strings.TrimSpace(runtimeRunID)
	if s == nil || s.runRepo == nil || runtimeRunID == "" {
		return "", nil
	}
	run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, runtimeRunID)
	if err != nil {
		return "", err
	}
	if run == nil {
		return "", nil
	}
	return strings.TrimSpace(run.WorkspaceID), nil
}

func ensureRuntimeWorkspaceMatch(requestedWorkspaceID, actualWorkspaceID string) error {
	requestedWorkspaceID = strings.TrimSpace(requestedWorkspaceID)
	actualWorkspaceID = strings.TrimSpace(actualWorkspaceID)
	if requestedWorkspaceID == "" || actualWorkspaceID == "" || requestedWorkspaceID == actualWorkspaceID {
		return nil
	}
	return fmt.Errorf("%w: target does not belong to requested workspace", ErrAgentRuntimeHostForbidden)
}

func runtimeWorkspaceID(maps ...map[string]interface{}) string {
	for _, values := range maps {
		if values == nil {
			continue
		}
		for _, key := range []string{"workspace_id", "workspaceID", "workspaceId"} {
			if value, ok := values[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
	}
	return ""
}

func agentRuntimeHostString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func runtimeWorkspaceContextData(workspace *model.Workspace) map[string]interface{} {
	if workspace == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              workspace.ID,
		"workspace_id":    workspace.ID,
		"name":            workspace.Name,
		"slug":            workspace.Slug,
		"workspace_key":   workspace.WorkspaceKey,
		"organization_id": workspace.OrganizationID,
	}
}

func runtimeTaskContextData(task *model.TaskDetail) map[string]interface{} {
	if task == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{
		"id":                task.Task.ID,
		"workspace_id":      task.Task.WorkspaceID,
		"display_id":        task.Task.DisplayID,
		"task_key":          task.Task.TaskKey,
		"name":              task.Task.Name,
		"description":       agentRuntimeHostString(task.Task.Description),
		"task_type":         task.Task.TaskType,
		"priority":          task.Task.Priority,
		"severity":          task.Task.Severity,
		"workflow_id":       task.Task.WorkflowID,
		"workflow_state_id": task.Task.WorkflowStateID,
		"epic_id":           agentRuntimeHostString(task.Task.EpicID),
		"sprint_id":         agentRuntimeHostString(task.Task.SprintID),
		"team_id":           agentRuntimeHostString(task.Task.TeamID),
		"blocked":           task.Task.Blocked,
		"blocker":           agentRuntimeHostString(task.Task.Blocker),
	}
	if task.State != nil {
		data["state"] = map[string]interface{}{"id": task.State.ID, "name": task.State.Name, "type": task.State.StateType}
	}
	if len(task.Labels) > 0 {
		labels := make([]map[string]interface{}, 0, len(task.Labels))
		for _, label := range task.Labels {
			labels = append(labels, map[string]interface{}{"id": label.ID, "name": label.Name, "color": label.Color})
		}
		data["labels"] = labels
	}
	return data
}

func runtimeEpicContextData(epic *model.EpicWithStats) map[string]interface{} {
	if epic == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":             epic.Epic.ID,
		"workspace_id":   epic.Epic.WorkspaceID,
		"name":           epic.Epic.Name,
		"description":    agentRuntimeHostString(epic.Epic.Description),
		"external_id":    agentRuntimeHostString(epic.Epic.ExternalID),
		"team_id":        agentRuntimeHostString(epic.Epic.TeamID),
		"health":         epic.Epic.Health,
		"planning_state": epic.Epic.PlanningState,
		"stats":          epic.Stats,
	}
}

func runtimeSupportConversationContextData(conversation *model.SupportConversation) map[string]interface{} {
	if conversation == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":             conversation.ID,
		"workspace_id":   conversation.WorkspaceID,
		"display_id":     conversation.DisplayID,
		"subject":        conversation.Subject,
		"status":         conversation.Status,
		"flow_state":     agentRuntimeHostString(conversation.FlowState),
		"priority":       conversation.Priority,
		"channel":        conversation.Channel,
		"customer_name":  agentRuntimeHostString(conversation.CustomerName),
		"customer_email": agentRuntimeHostString(conversation.CustomerEmail),
		"ai_state":       agentRuntimeHostString(conversation.AIState),
	}
}

func runtimeDocumentContextData(doc *model.DocsDocument) map[string]interface{} {
	if doc == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            doc.ID,
		"workspace_id":  doc.WorkspaceID,
		"space_id":      doc.SpaceID,
		"collection_id": agentRuntimeHostString(doc.CollectionID),
		"title":         doc.Title,
		"status":        doc.Status,
		"visibility":    doc.Visibility,
		"excerpt":       agentRuntimeHostString(doc.Excerpt),
		"tags":          doc.Tags,
		"is_locked":     doc.IsLocked,
	}
}

func runtimeCRMContactContextData(contact *model.CRMContact, title string) map[string]interface{} {
	if contact == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              contact.ID,
		"workspace_id":    contact.WorkspaceID,
		"display_id":      contact.DisplayID,
		"name":            strings.TrimSpace(title),
		"first_name":      contact.FirstName,
		"last_name":       agentRuntimeHostString(contact.LastName),
		"email":           agentRuntimeHostString(contact.Email),
		"job_title":       agentRuntimeHostString(contact.JobTitle),
		"lifecycle_stage": contact.LifecycleStage,
		"lead_status":     contact.LeadStatus,
		"owner_member_id": agentRuntimeHostString(contact.OwnerMemberID),
	}
}

func runtimeCRMCompanyContextData(company *model.CRMCompany) map[string]interface{} {
	if company == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              company.ID,
		"workspace_id":    company.WorkspaceID,
		"display_id":      company.DisplayID,
		"name":            company.Name,
		"domain":          agentRuntimeHostString(company.Domain),
		"industry":        agentRuntimeHostString(company.Industry),
		"employee_count":  company.EmployeeCount,
		"description":     agentRuntimeHostString(company.Description),
		"owner_member_id": agentRuntimeHostString(company.OwnerMemberID),
	}
}

func runtimeCRMDealContextData(deal *model.CRMDeal) map[string]interface{} {
	if deal == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{
		"id":              deal.ID,
		"workspace_id":    deal.WorkspaceID,
		"display_id":      deal.DisplayID,
		"name":            deal.Name,
		"pipeline_id":     deal.PipelineID,
		"stage_id":        deal.StageID,
		"amount":          deal.Amount,
		"currency":        deal.Currency,
		"close_date":      deal.CloseDate,
		"owner_member_id": agentRuntimeHostString(deal.OwnerMemberID),
		"probability":     deal.Probability,
	}
	if deal.Pipeline != nil {
		data["pipeline"] = map[string]interface{}{"id": deal.Pipeline.ID, "name": deal.Pipeline.Name}
	}
	if deal.Stage != nil {
		data["stage"] = map[string]interface{}{"id": deal.Stage.ID, "name": deal.Stage.Name, "stage_type": deal.Stage.StageType}
	}
	return data
}
