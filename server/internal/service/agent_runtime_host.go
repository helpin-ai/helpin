package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	agentRuntimeHelpinBuiltInSkillIDPrefix     = "helpin_builtin:"
	agentRuntimeHelpinBuiltInSkillObjectPrefix = "helpin-builtins/"
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
	skillRepo      *repository.WorkspaceSkillRepository
	skillStore     skillPackageStore
}

type AgentRuntimeSkillLookupRequest struct {
	AppID    string                 `json:"app_id"`
	AgentID  string                 `json:"agent_id,omitempty"`
	RunID    string                 `json:"run_id,omitempty"`
	Target   agentruntime.TargetRef `json:"target"`
	Trigger  map[string]interface{} `json:"trigger,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	SkillID  string                 `json:"skill_id,omitempty"`
	Key      string                 `json:"key,omitempty"`
}

type AgentRuntimeWorkspaceSkill struct {
	ID                string          `json:"id"`
	Key               string          `json:"key"`
	VersionKey        string          `json:"version_key"`
	Title             string          `json:"title"`
	Description       string          `json:"description,omitempty"`
	SourceKind        string          `json:"source_kind"`
	Instructions      string          `json:"instructions"`
	RequiredTools     []string        `json:"required_tools,omitempty"`
	SupportedRuntimes []string        `json:"supported_runtimes,omitempty"`
	Interface         json.RawMessage `json:"interface,omitempty"`
	Policy            json.RawMessage `json:"policy,omitempty"`
	PackageObjectKey  string          `json:"package_object_key,omitempty"`
	PackageFileName   string          `json:"package_file_name,omitempty"`
	PackageChecksum   string          `json:"package_checksum,omitempty"`
	PackageSize       int64           `json:"package_size,omitempty"`
	IsArchived        bool            `json:"is_archived,omitempty"`
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

func (s *AgentRuntimeHostService) SetWorkspaceSkillStore(repo *repository.WorkspaceSkillRepository, store skillPackageStore) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.skillRepo = repo
	s.skillStore = store
	return s
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

func (s *AgentRuntimeHostService) ResolveSkillByID(ctx context.Context, req AgentRuntimeSkillLookupRequest) (*AgentRuntimeWorkspaceSkill, error) {
	skillID := strings.TrimSpace(req.SkillID)
	if skillID == "" {
		return nil, fmt.Errorf("%w: skill_id is required", ErrAgentRuntimeHostBadRequest)
	}
	if s == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	if strings.HasPrefix(skillID, agentRuntimeHelpinBuiltInSkillIDPrefix) {
		key := strings.TrimPrefix(skillID, agentRuntimeHelpinBuiltInSkillIDPrefix)
		definition, ok := agentcontract.GetBuiltInSkill(key)
		if !ok {
			return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
		}
		return runtimeBuiltInWorkspaceSkill(definition)
	}
	return s.resolveWorkspaceSkill(ctx, req, func(workspaceID string) (*model.WorkspaceSkill, error) {
		return s.skillRepo.GetByID(ctx, workspaceID, skillID)
	})
}

func (s *AgentRuntimeHostService) ResolveActiveSkillByKey(ctx context.Context, req AgentRuntimeSkillLookupRequest) (*AgentRuntimeWorkspaceSkill, error) {
	key := agentcontract.CanonicalBuiltInSkillKey(strings.TrimSpace(req.Key))
	if key == "" {
		return nil, fmt.Errorf("%w: key is required", ErrAgentRuntimeHostBadRequest)
	}
	if s == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	// Product-owned built-in skill keys are immutable runtime contracts. Resolve
	// them from the currently deployed package before consulting persisted
	// workspace rows. Older releases materialized built-ins in workspace_skills;
	// allowing one of those rows to win here can silently retain stale required
	// tools or completion-interaction policy across process restarts. Workspace
	// skills remain addressable through their explicit skill IDs.
	if definition, ok := agentcontract.GetBuiltInSkill(key); ok {
		return runtimeBuiltInWorkspaceSkill(definition)
	}
	workspaceID, err := s.workspaceIDForSkillLookup(ctx, req)
	if err != nil {
		return nil, err
	}
	if workspaceID != "" && s.skillRepo != nil {
		skill, err := s.skillRepo.GetActiveByKey(ctx, workspaceID, key)
		if err != nil {
			return nil, err
		}
		if skill != nil && !skill.IsArchived {
			return runtimeWorkspaceSkill(skill), nil
		}
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required", ErrAgentRuntimeHostBadRequest)
	}
	return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
}

func (s *AgentRuntimeHostService) resolveWorkspaceSkill(ctx context.Context, req AgentRuntimeSkillLookupRequest, lookup func(workspaceID string) (*model.WorkspaceSkill, error)) (*AgentRuntimeWorkspaceSkill, error) {
	if s == nil || s.skillRepo == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	workspaceID, err := s.workspaceIDForSkillLookup(ctx, req)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required", ErrAgentRuntimeHostBadRequest)
	}
	skill, err := lookup(workspaceID)
	if err != nil {
		return nil, err
	}
	if skill == nil || skill.IsArchived {
		return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
	}
	return runtimeWorkspaceSkill(skill), nil
}

func (s *AgentRuntimeHostService) workspaceIDForSkillLookup(ctx context.Context, req AgentRuntimeSkillLookupRequest) (string, error) {
	workspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata)
	mappedWorkspaceID, err := s.workspaceIDForRuntimeRun(ctx, req.RunID)
	if err != nil {
		return "", err
	}
	if workspaceID == "" {
		return mappedWorkspaceID, nil
	}
	if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, workspaceID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func (s *AgentRuntimeHostService) GetSkillPackageObject(ctx context.Context, objectKey string) ([]byte, error) {
	objectKey = strings.TrimSpace(objectKey)
	if s == nil {
		return nil, fmt.Errorf("workspace skill package store is not configured")
	}
	if objectKey == "" {
		return nil, fmt.Errorf("%w: package object key is required", ErrAgentRuntimeHostBadRequest)
	}
	if strings.HasPrefix(objectKey, agentRuntimeHelpinBuiltInSkillObjectPrefix) {
		key := strings.TrimSuffix(strings.TrimPrefix(objectKey, agentRuntimeHelpinBuiltInSkillObjectPrefix), ".zip")
		definition, ok := agentcontract.GetBuiltInSkill(key)
		if !ok {
			return nil, fmt.Errorf("%w: workspace skill package not found", ErrAgentRuntimeHostNotFound)
		}
		archive, _, _, err := agentcontract.BuildSkillArchive(definition)
		if err != nil {
			return nil, err
		}
		return archive, nil
	}
	if s.skillRepo == nil || s.skillStore == nil {
		return nil, fmt.Errorf("workspace skill package store is not configured")
	}
	if !strings.HasPrefix(objectKey, "workspaces/") || strings.Contains(objectKey, "..") {
		return nil, fmt.Errorf("%w: package object key is not allowed", ErrAgentRuntimeHostForbidden)
	}
	skill, err := s.skillRepo.GetActiveByPackageObjectKey(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("%w: workspace skill package not found", ErrAgentRuntimeHostNotFound)
	}
	payload, err := s.skillStore.GetObject(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func runtimeWorkspaceSkill(skill *model.WorkspaceSkill) *AgentRuntimeWorkspaceSkill {
	if skill == nil {
		return nil
	}
	return &AgentRuntimeWorkspaceSkill{
		ID:                skill.ID,
		Key:               skill.Key,
		VersionKey:        skill.VersionKey,
		Title:             skill.Title,
		Description:       stringOrDefault(skill.Description, ""),
		SourceKind:        skill.SourceKind,
		Instructions:      skill.Instructions,
		RequiredTools:     parseJSONStringSlice(json.RawMessage(skill.RequiredTools)),
		SupportedRuntimes: parseJSONStringSlice(json.RawMessage(skill.SupportedRuntimes)),
		Interface:         json.RawMessage(skill.InterfaceConfig),
		Policy:            json.RawMessage(skill.PolicyConfig),
		PackageObjectKey:  skill.PackageObjectKey,
		PackageFileName:   skill.PackageFileName,
		PackageChecksum:   skill.PackageChecksum,
		PackageSize:       skill.PackageSize,
		IsArchived:        skill.IsArchived,
	}
}

func runtimeBuiltInWorkspaceSkill(definition agentcontract.SkillDefinition) (*AgentRuntimeWorkspaceSkill, error) {
	archive, checksum, filename, err := agentcontract.BuildSkillArchive(definition)
	if err != nil {
		return nil, err
	}
	return &AgentRuntimeWorkspaceSkill{
		ID:                agentRuntimeHelpinBuiltInSkillIDPrefix + definition.Key,
		Key:               definition.Key,
		VersionKey:        checksum,
		Title:             definition.Title,
		Description:       definition.Description,
		SourceKind:        model.WorkspaceSkillSourceBuiltIn,
		Instructions:      definition.Instructions,
		RequiredTools:     append([]string(nil), definition.RequiredTools...),
		SupportedRuntimes: append([]string(nil), definition.SupportedRuntimes...),
		Interface:         runtimeSkillInterfaceJSON(definition.Interface),
		Policy:            runtimeSkillPolicyJSON(definition.Policy),
		PackageObjectKey:  agentRuntimeHelpinBuiltInSkillObjectPrefix + definition.Key + ".zip",
		PackageFileName:   filename,
		PackageChecksum:   checksum,
		PackageSize:       int64(len(archive)),
	}, nil
}

func runtimeSkillInterfaceJSON(value agentcontract.SkillInterface) json.RawMessage {
	payload := map[string]interface{}{}
	if strings.TrimSpace(value.DisplayName) != "" {
		payload["display_name"] = strings.TrimSpace(value.DisplayName)
	}
	if strings.TrimSpace(value.ShortDescription) != "" {
		payload["short_description"] = strings.TrimSpace(value.ShortDescription)
	}
	if strings.TrimSpace(value.IconSmall) != "" {
		payload["icon_small"] = strings.TrimSpace(value.IconSmall)
	}
	if strings.TrimSpace(value.IconLarge) != "" {
		payload["icon_large"] = strings.TrimSpace(value.IconLarge)
	}
	if strings.TrimSpace(value.BrandColor) != "" {
		payload["brand_color"] = strings.TrimSpace(value.BrandColor)
	}
	if strings.TrimSpace(value.DefaultPrompt) != "" {
		payload["default_prompt"] = strings.TrimSpace(value.DefaultPrompt)
	}
	return mustMarshalRuntimeHostJSON(payload)
}

func runtimeSkillPolicyJSON(value agentcontract.SkillPolicy) json.RawMessage {
	payload := map[string]interface{}{}
	if value.AllowImplicitInvocation != nil {
		payload["allow_implicit_invocation"] = *value.AllowImplicitInvocation
	}
	if len(value.CompletionRequiresInteractionKinds) > 0 {
		payload["completion_requires_interaction_kinds"] = append([]string(nil), value.CompletionRequiresInteractionKinds...)
	}
	if len(value.InteractionContracts) > 0 {
		contracts := make([]map[string]interface{}, 0, len(value.InteractionContracts))
		for _, contract := range value.InteractionContracts {
			item := map[string]interface{}{}
			if strings.TrimSpace(contract.Kind) != "" {
				item["kind"] = strings.TrimSpace(contract.Kind)
			}
			if strings.TrimSpace(contract.Schema) != "" {
				item["schema"] = strings.TrimSpace(contract.Schema)
			}
			if len(contract.Transports) > 0 {
				transports := make(map[string]interface{}, len(contract.Transports))
				for key, transport := range contract.Transports {
					transportPayload := map[string]interface{}{}
					if strings.TrimSpace(transport.Type) != "" {
						transportPayload["type"] = strings.TrimSpace(transport.Type)
					}
					if strings.TrimSpace(transport.ToolName) != "" {
						transportPayload["tool_name"] = strings.TrimSpace(transport.ToolName)
					}
					if strings.TrimSpace(transport.BlockLabel) != "" {
						transportPayload["block_label"] = strings.TrimSpace(transport.BlockLabel)
					}
					transports[key] = transportPayload
				}
				item["transports"] = transports
			}
			contracts = append(contracts, item)
		}
		payload["interaction_contracts"] = contracts
	}
	return mustMarshalRuntimeHostJSON(payload)
}

func mustMarshalRuntimeHostJSON(value interface{}) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	return payload
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
