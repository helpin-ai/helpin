package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/automationcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	automationGroupBuiltIn    = "built_in_automations"
	automationGroupContextual = "contextual_agents"
	automationGroupCustom     = "custom_automations"
	automationGroupRules      = "automation_rules"
)

// AutomationInventoryService assembles the shared read-only automation inventory.
type AutomationInventoryService struct {
	settingsRepo         *repository.SettingsRepository
	pmAutomationRepo     *repository.PMAutomationRepository
	crmEmailRepo         *repository.CRMEmailRepository
	automationHealthRepo *repository.AutomationHealthRepository
	automationRuleRepo   *repository.AutomationRuleRepository
	triggerExecRepo      *repository.AgentTriggerExecutionRepository
	agentRepo            *repository.AgentRepository
	taskRepo             *repository.PMTaskRepository
	installationRepo     *repository.SupportInboxInstallationRepository
}

func NewAutomationInventoryService(
	settingsRepo *repository.SettingsRepository,
	pmAutomationRepo *repository.PMAutomationRepository,
	crmEmailRepo *repository.CRMEmailRepository,
	automationHealthRepo *repository.AutomationHealthRepository,
	automationRuleRepo *repository.AutomationRuleRepository,
	triggerExecRepo *repository.AgentTriggerExecutionRepository,
	agentRepo *repository.AgentRepository,
	taskRepo *repository.PMTaskRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
) *AutomationInventoryService {
	return &AutomationInventoryService{
		settingsRepo:         settingsRepo,
		pmAutomationRepo:     pmAutomationRepo,
		crmEmailRepo:         crmEmailRepo,
		automationHealthRepo: automationHealthRepo,
		automationRuleRepo:   automationRuleRepo,
		triggerExecRepo:      triggerExecRepo,
		agentRepo:            agentRepo,
		taskRepo:             taskRepo,
		installationRepo:     installationRepo,
	}
}

func (s *AutomationInventoryService) GetWorkspaceInventory(ctx context.Context, workspaceID string) (*model.AutomationInventoryResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	catalogEntries := automationcatalog.All()
	catalogByID := make(map[string]model.AutomationCatalogEntry, len(catalogEntries))
	for _, entry := range catalogEntries {
		catalogByID[entry.ID] = entry
	}

	healthByKey, err := s.loadBuiltInHealth(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	items := make([]model.AutomationInventoryItem, 0, 32)

	crmItems, err := s.crmBuiltInItems(ctx, workspaceID, catalogByID, healthByKey)
	if err != nil {
		return nil, err
	}
	items = append(items, crmItems...)

	pmBuiltIns, err := s.pmBuiltInItems(ctx, workspaceID, catalogByID, healthByKey)
	if err != nil {
		return nil, err
	}
	items = append(items, pmBuiltIns...)

	ruleItems, err := s.automationRuleItems(ctx, workspaceID, catalogByID)
	if err != nil {
		return nil, err
	}
	items = append(items, ruleItems...)

	triggerCatalog, err := s.triggerCatalogItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return kindSortRank(items[i].Kind) < kindSortRank(items[j].Kind)
		}
		if items[i].Group != items[j].Group {
			return items[i].Group < items[j].Group
		}
		if items[i].Title != items[j].Title {
			return items[i].Title < items[j].Title
		}
		return items[i].InventoryID < items[j].InventoryID
	})

	return &model.AutomationInventoryResponse{
		Groups: []model.AutomationInventoryGroup{
			{ID: automationGroupBuiltIn, Title: "Built-in Automations", Description: "System intelligence and deterministic built-ins that already operate inside the product."},
			{ID: automationGroupRules, Title: "Automation Rules", Description: "User-configured rules triggered by workflow events. Powers stage-based agent pipelines."},
		},
		Items:          items,
		TriggerCatalog: triggerCatalog,
		GeneratedAt:    time.Now().UTC(),
	}, nil
}

func (s *AutomationInventoryService) loadBuiltInHealth(ctx context.Context, workspaceID string) (map[string]model.AutomationHealthSummary, error) {
	result := make(map[string]model.AutomationHealthSummary)
	if s == nil || s.automationHealthRepo == nil {
		return result, nil
	}
	snapshots, err := s.automationHealthRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		result[healthKey(snapshot.CatalogID, snapshot.ScopeType, snapshot.ScopeID)] = summarizeSnapshot(snapshot)
	}
	return result, nil
}

func (s *AutomationInventoryService) crmBuiltInItems(ctx context.Context, workspaceID string, catalogByID map[string]model.AutomationCatalogEntry, healthByKey map[string]model.AutomationHealthSummary) ([]model.AutomationInventoryItem, error) {
	accounts, err := s.crmEmailRepo.ListAccounts(ctx, workspaceID, model.CRMEmailAccountListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list crm email accounts for automation inventory: %w", err)
	}

	activeAccountCount := 0
	for _, account := range accounts {
		if account.IsActive && account.Status == model.CRMEmailAccountStatusConnected {
			activeAccountCount++
		}
	}

	enabled := activeAccountCount > 0
	scopeType := model.AutomationScopeWorkspace
	scopeID := workspaceID
	scopeLabel := "Workspace"

	catalogIDs := []string{
		"crm.buyer_signal_ingestion",
		"crm.contact_summary_refresh",
		"crm.deal_summary_refresh",
	}
	items := make([]model.AutomationInventoryItem, 0, len(catalogIDs))
	for _, catalogID := range catalogIDs {
		entry, ok := catalogByID[catalogID]
		if !ok {
			continue
		}
		health := healthByKey[healthKey(catalogID, scopeType, scopeID)]
		if !enabled {
			health = inactiveHealth("No connected CRM email accounts")
		} else if health.Status == "" {
			health = unknownHealth()
		}
		items = append(items, inventoryItemFromCatalog(entry, fmt.Sprintf("%s:%s", catalogID, scopeType), scopeType, scopeID, scopeLabel, enabled, health))
	}
	return items, nil
}

func (s *AutomationInventoryService) pmBuiltInItems(ctx context.Context, workspaceID string, catalogByID map[string]model.AutomationCatalogEntry, healthByKey map[string]model.AutomationHealthSummary) ([]model.AutomationInventoryItem, error) {
	automations, err := s.pmAutomationRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list pm automations for automation inventory: %w", err)
	}
	teams, err := s.settingsRepo.ListTeams(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list teams for automation inventory: %w", err)
	}

	workspaceRules := map[string]*model.PMAutomation{}
	teamRules := make(map[string]*model.PMAutomation)
	for idx := range automations {
		auto := &automations[idx]
		if auto.TeamID == nil || strings.TrimSpace(*auto.TeamID) == "" {
			workspaceRules[auto.AutomationType] = auto
			continue
		}
		teamRules[auto.AutomationType+":"+*auto.TeamID] = auto
	}

	items := make([]model.AutomationInventoryItem, 0, 2+len(teams)*2)

	workspaceMappings := map[string]string{
		model.PMAutomationTypeEpicAutoStart:    "pm.epic_auto_start",
		model.PMAutomationTypeEpicAutoComplete: "pm.epic_auto_complete",
	}
	for automationType, catalogID := range workspaceMappings {
		entry, ok := catalogByID[catalogID]
		if !ok {
			continue
		}
		auto := workspaceRules[automationType]
		enabled := auto != nil && auto.Enabled
		health := healthByKey[healthKey(catalogID, model.AutomationScopeWorkspace, workspaceID)]
		if !enabled {
			health = inactiveHealth("Disabled in PM automations")
		} else if health.Status == "" {
			health = unknownHealth()
		}
		items = append(items, inventoryItemFromCatalog(entry, fmt.Sprintf("%s:%s", catalogID, model.AutomationScopeWorkspace), model.AutomationScopeWorkspace, workspaceID, "Workspace", enabled, health))
	}

	teamMappings := map[string]string{
		model.PMAutomationTypeSprintAutoCreate:     "pm.sprint_auto_create",
		model.PMAutomationTypeSprintMoveUnfinished: "pm.sprint_move_unfinished",
	}
	for _, team := range teams {
		for automationType, catalogID := range teamMappings {
			entry, ok := catalogByID[catalogID]
			if !ok {
				continue
			}
			auto := teamRules[automationType+":"+team.ID]
			enabled := auto != nil && auto.Enabled
			health := healthByKey[healthKey(catalogID, model.AutomationScopeTeam, team.ID)]
			if !enabled {
				health = inactiveHealth("Disabled for team")
			} else if health.Status == "" {
				health = unknownHealth()
			}
			items = append(items, inventoryItemFromCatalog(entry, fmt.Sprintf("%s:team:%s", catalogID, team.ID), model.AutomationScopeTeam, team.ID, team.Name, enabled, health))
		}
	}

	return items, nil
}

func (s *AutomationInventoryService) automationRuleItems(ctx context.Context, workspaceID string, catalogByID map[string]model.AutomationCatalogEntry) ([]model.AutomationInventoryItem, error) {
	if s.automationRuleRepo == nil {
		return nil, nil
	}
	rules, err := s.automationRuleRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list automation rules for inventory: %w", err)
	}

	entry, ok := catalogByID["automation_rule"]
	if !ok {
		return nil, nil
	}

	items := make([]model.AutomationInventoryItem, 0, len(rules))
	for _, rule := range rules {
		health := model.AutomationHealthSummary{
			Status:    model.AutomationHealthHealthy,
			Freshness: "active",
			Metrics:   model.JSONB{"trigger_type": rule.TriggerType, "action_type": rule.ActionType},
		}
		if !rule.Enabled {
			health = inactiveHealth("Disabled")
		}
		items = append(items, inventoryItemFromCatalog(entry,
			fmt.Sprintf("automation_rule:rule:%s", rule.ID),
			model.AutomationScopeWorkspace, workspaceID,
			rule.Name, rule.Enabled, health))
	}
	return items, nil
}

func (s *AutomationInventoryService) triggerCatalogItems(ctx context.Context, workspaceID string) ([]model.AutomationTriggerCatalogEntry, error) {
	rules := make([]model.AutomationRule, 0)
	if s.automationRuleRepo != nil {
		list, err := s.automationRuleRepo.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list automation rules for trigger catalog: %w", err)
		}
		rules = list
	}

	agents := make([]model.Agent, 0)
	if s.agentRepo != nil {
		list, err := s.agentRepo.List(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list agents for trigger catalog: %w", err)
		}
		agents = list
	}

	bindingCounts := make(map[string]int)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if bindingID, _, ok := automationcatalog.ResolveBindingForTrigger(model.AgentRunTriggerSourceAutomationRule, rule.TriggerType, ""); ok {
			bindingCounts[bindingID]++
		}
	}

	for _, agent := range agents {
		if strings.TrimSpace(derefString(agent.Schedule)) != "" {
			bindingCounts["agent.schedule"]++
		}
	}

	if s.installationRepo != nil {
		inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("get support installation for trigger catalog: %w", err)
		}
		if inst != nil {
			settings := parseSettings(inst.Settings)
			if settings.AIEnabled && strings.TrimSpace(derefString(settings.AIAgentID)) != "" {
				bindingCounts["support.widget_message"]++
			}
		}
	}

	if s.taskRepo != nil {
		count, err := s.taskRepo.CountAssignedTasks(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("count assigned tasks for trigger catalog: %w", err)
		}
		bindingCounts["task.assigned_agent_state_change"] = int(count)
	}

	entries := automationcatalog.TriggerCatalog()
	for idx := range entries {
		entries[idx].BindingCount = bindingCounts[entries[idx].ID]
		switch entries[idx].ID {
		case "manual.task_run":
			entries[idx].BindingCount = countRunnableAgentsForTarget(agents, "task")
		case "manual.epic_run":
			entries[idx].BindingCount = countRunnableAgentsForTarget(agents, "epic")
		case "manual.support_run":
			entries[idx].BindingCount = countRunnableAgentsForTarget(agents, "support_conversation")
		}
	}
	return entries, nil
}

func (s *AutomationInventoryService) ListTriggerExecutions(
	ctx context.Context,
	workspaceID string,
	filters model.TriggerExecutionListFilters,
	pagination model.PMPagination,
) (*model.AutomationTriggerExecutionListResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if s == nil || s.triggerExecRepo == nil {
		return &model.AutomationTriggerExecutionListResponse{
			Data:       []model.AutomationTriggerExecutionListItem{},
			Total:      0,
			Page:       pagination.Page,
			PerPage:    pagination.PerPage,
			TotalPages: 0,
		}, nil
	}

	if pagination.Page <= 0 {
		pagination.Page = 1
	}
	if pagination.PerPage <= 0 {
		pagination.PerPage = 25
	}
	if pagination.PerPage > 100 {
		pagination.PerPage = 100
	}

	executions, total, err := s.triggerExecRepo.ListByWorkspace(ctx, workspaceID, filters, pagination)
	if err != nil {
		return nil, err
	}

	agentNames := map[string]string{}
	if s.agentRepo != nil {
		agents, err := s.agentRepo.List(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list agents for trigger executions: %w", err)
		}
		for _, agent := range agents {
			agentNames[agent.ID] = agent.Name
		}
	}

	ruleNames := map[string]string{}
	if s.automationRuleRepo != nil {
		rules, err := s.automationRuleRepo.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list automation rules for trigger executions: %w", err)
		}
		for _, rule := range rules {
			ruleNames[rule.ID] = rule.Name
		}
	}

	items := make([]model.AutomationTriggerExecutionListItem, 0, len(executions))
	for _, execution := range executions {
		bindingTitle, managePath := describeTriggerBinding(execution, ruleNames)
		triggerTitle := triggerTitleForExecution(execution)
		referenceTitle := referenceTitleForExecution(execution, ruleNames)
		items = append(items, model.AutomationTriggerExecutionListItem{
			ExecutionID:    execution.ID,
			AgentID:        execution.AgentID,
			AgentName:      agentDisplayName(execution.AgentID, agentNames),
			BindingID:      execution.BindingID,
			BindingKind:    execution.BindingKind,
			BindingTitle:   bindingTitle,
			TriggerType:    execution.TriggerType,
			TriggerTitle:   triggerTitle,
			ReferenceID:    execution.ReferenceID,
			ReferenceType:  execution.ReferenceType,
			ReferenceTitle: referenceTitle,
			ManagePath:     managePath,
			TargetType:     execution.TargetType,
			TargetID:       execution.TargetID,
			RunID:          execution.RunID,
			Status:         execution.Status,
			ErrorMessage:   execution.ErrorMessage,
			FiredAt:        execution.FiredAt,
			StartedAt:      execution.StartedAt,
			CompletedAt:    execution.CompletedAt,
		})
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}

	return &model.AutomationTriggerExecutionListResponse{
		Data:       items,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	}, nil
}

func countRunnableAgentsForTarget(agents []model.Agent, targetType string) int {
	count := 0
	for idx := range agents {
		agent := agents[idx]
		if strings.TrimSpace(agent.Status) == "disabled" {
			continue
		}
		if validateAgentTarget(&agent, targetType) == nil {
			count++
		}
	}
	return count
}

func inventoryItemFromCatalog(entry model.AutomationCatalogEntry, inventoryID, scopeType, scopeID, scopeLabel string, enabled bool, health model.AutomationHealthSummary) model.AutomationInventoryItem {
	return model.AutomationInventoryItem{
		InventoryID:         inventoryID,
		CatalogID:           entry.ID,
		Kind:                entry.Kind,
		Module:              entry.Module,
		Group:               entry.Group,
		Title:               entry.Title,
		Description:         entry.Description,
		ScopeType:           scopeType,
		ScopeID:             scopeID,
		ScopeLabel:          scopeLabel,
		TargetTypes:         entry.TargetTypes,
		TriggerModes:        entry.TriggerModes,
		ConfigScope:         entry.ConfigScope,
		ExecutionStyle:      entry.ExecutionStyle,
		UserGoverned:        entry.UserGoverned,
		Enabled:             enabled,
		CurrentWriteSurface: entry.CurrentWriteSurface,
		CurrentWritePath:    entry.CurrentWritePath,
		CurrentRunSurface:   entry.CurrentRunSurface,
		CurrentRunPath:      entry.CurrentRunPath,
		OutputSurface:       entry.OutputSurface,
		DiagnosticsSurface:  entry.DiagnosticsSurface,
		Health:              health,
	}
}

func kindSortRank(kind string) int {
	switch kind {
	case model.AutomationKindBuiltIn:
		return 0
	case model.AutomationKindRule:
		return 1
	default:
		return 9
	}
}

func healthKey(catalogID, scopeType, scopeID string) string {
	return catalogID + "|" + scopeType + "|" + scopeID
}

func summarizeSnapshot(snapshot model.AutomationHealthSnapshot) model.AutomationHealthSummary {
	return model.AutomationHealthSummary{
		Status:           defaultString(snapshot.Status, model.AutomationHealthUnknown),
		LastSeenAt:       snapshot.LastSeenAt,
		LastSuccessAt:    snapshot.LastSuccessAt,
		LastErrorAt:      snapshot.LastErrorAt,
		LastErrorMessage: snapshot.LastErrorMessage,
		Freshness:        summarizeFreshness(snapshot.Status, snapshot.LastSeenAt),
		Metrics:          ensureMetrics(snapshot.Metrics),
	}
}

func describeTriggerBinding(execution model.AgentTriggerExecution, ruleNames map[string]string) (string, *string) {
	if def, ok := automationcatalog.ResolveDefinitionForExecution(execution.BindingID, execution.BindingKind, derefString(execution.TriggerType)); ok {
		if execution.ReferenceID != nil && strings.TrimSpace(execution.BindingKind) == "automation_rule" {
			if name := strings.TrimSpace(ruleNames[strings.TrimSpace(*execution.ReferenceID)]); name != "" {
				return name, def.ConfigSurface
			}
		}
		return def.Title, def.ConfigSurface
	}
	return defaultString(strings.TrimSpace(execution.BindingKind), "Trigger"), nil
}

func triggerTitleForExecution(execution model.AgentTriggerExecution) *string {
	if def, ok := automationcatalog.ResolveDefinitionForExecution(execution.BindingID, execution.BindingKind, derefString(execution.TriggerType)); ok {
		return strPtr(def.Title)
	}
	return nil
}

func referenceTitleForExecution(execution model.AgentTriggerExecution, ruleNames map[string]string) *string {
	if execution.ReferenceType == nil || strings.TrimSpace(*execution.ReferenceType) == "" {
		return nil
	}
	if strings.TrimSpace(*execution.ReferenceType) == "automation_rule" && execution.ReferenceID != nil {
		if name := strings.TrimSpace(ruleNames[strings.TrimSpace(*execution.ReferenceID)]); name != "" {
			return &name
		}
	}
	return nil
}

func agentDisplayName(agentID string, agentNames map[string]string) string {
	if name := strings.TrimSpace(agentNames[strings.TrimSpace(agentID)]); name != "" {
		return name
	}
	return "Unknown agent"
}

func summarizeFreshness(status string, ts *time.Time) string {
	if status == model.AutomationHealthInactive {
		return "inactive"
	}
	if ts == nil {
		return "unknown"
	}
	age := time.Since(*ts)
	if age <= 6*time.Hour {
		return "fresh"
	}
	if age <= 48*time.Hour {
		return "aging"
	}
	return "stale"
}

func inactiveHealth(reason string) model.AutomationHealthSummary {
	metrics := model.JSONB{}
	if strings.TrimSpace(reason) != "" {
		metrics["reason"] = reason
	}
	return model.AutomationHealthSummary{
		Status:    model.AutomationHealthInactive,
		Freshness: "inactive",
		Metrics:   metrics,
	}
}

func unknownHealth() model.AutomationHealthSummary {
	return model.AutomationHealthSummary{
		Status:    model.AutomationHealthUnknown,
		Freshness: "unknown",
		Metrics:   model.JSONB{},
	}
}

func ensureMetrics(metrics model.JSONB) model.JSONB {
	if metrics == nil {
		return model.JSONB{}
	}
	return metrics
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}
