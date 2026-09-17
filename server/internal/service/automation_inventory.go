package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/automationcatalog"
	"github.com/helpin-ai/helpin/server/internal/automationcron"
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
	runRepo              *repository.AgentRunRepository
	agentRepo            *repository.AgentRepository
	workspaceRepo        *repository.WorkspaceRepository
	taskRepo             *repository.PMTaskRepository
	epicRepo             *repository.PMEpicRepository
	docsDocumentRepo     *repository.DocsDocumentRepository
	conversationRepo     *repository.SupportConversationRepository
	crmContactRepo       *repository.CRMContactRepository
	crmDealRepo          *repository.CRMDealRepository
	gitRepo              *repository.GitRepositoryRepository
	supportCoverageRepo  *repository.SupportCoverageRepository
	installationRepo     *repository.SupportInboxInstallationRepository
}

func NewAutomationInventoryService(
	settingsRepo *repository.SettingsRepository,
	pmAutomationRepo *repository.PMAutomationRepository,
	crmEmailRepo *repository.CRMEmailRepository,
	automationHealthRepo *repository.AutomationHealthRepository,
	automationRuleRepo *repository.AutomationRuleRepository,
	triggerExecRepo *repository.AgentTriggerExecutionRepository,
	runRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	workspaceRepo *repository.WorkspaceRepository,
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
		runRepo:              runRepo,
		agentRepo:            agentRepo,
		workspaceRepo:        workspaceRepo,
		taskRepo:             taskRepo,
		installationRepo:     installationRepo,
	}
}

func (s *AutomationInventoryService) SetTargetResolvers(
	epicRepo *repository.PMEpicRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
	conversationRepo *repository.SupportConversationRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmDealRepo *repository.CRMDealRepository,
	gitRepo *repository.GitRepositoryRepository,
	supportCoverageRepo *repository.SupportCoverageRepository,
) *AutomationInventoryService {
	s.epicRepo = epicRepo
	s.docsDocumentRepo = docsDocumentRepo
	s.conversationRepo = conversationRepo
	s.crmContactRepo = crmContactRepo
	s.crmDealRepo = crmDealRepo
	s.gitRepo = gitRepo
	s.supportCoverageRepo = supportCoverageRepo
	return s
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
		"crm.company_summary_refresh",
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

	healthByRuleID := map[string]model.AutomationHealthSummary{}
	runCountsByRuleID := map[string]repository.AutomationRuleExecutionCounts{}
	if s.triggerExecRepo != nil {
		ruleIDs := collectRuleIDs(rules)
		latestExecutions, err := s.triggerExecRepo.ListLatestAutomationRuleExecutions(ctx, workspaceID, ruleIDs)
		if err != nil {
			return nil, fmt.Errorf("list automation rule executions for inventory: %w", err)
		}
		for _, execution := range latestExecutions {
			if execution.ReferenceID == nil || strings.TrimSpace(*execution.ReferenceID) == "" {
				continue
			}
			healthByRuleID[strings.TrimSpace(*execution.ReferenceID)] = summarizeRuleExecution(execution)
		}
		runCountsByRuleID, err = s.triggerExecRepo.CountAutomationRuleExecutions(ctx, workspaceID, ruleIDs)
		if err != nil {
			return nil, fmt.Errorf("count automation rule executions for inventory: %w", err)
		}
	}

	items := make([]model.AutomationInventoryItem, 0, len(rules))
	for _, rule := range rules {
		health := healthByRuleID[rule.ID]
		if health.Status == "" {
			health = model.AutomationHealthSummary{
				Status:    model.AutomationHealthHealthy,
				Freshness: "active",
				Metrics:   model.JSONB{"trigger_type": rule.TriggerType, "action_type": rule.ActionType},
			}
		}
		if !rule.Enabled {
			health = inactiveHealth("Disabled")
		}
		health.Metrics = ensureMetrics(health.Metrics)
		health.Metrics["trigger_type"] = rule.TriggerType
		health.Metrics["action_type"] = rule.ActionType
		counts := runCountsByRuleID[rule.ID]
		health.Metrics["total_runs"] = counts.Total
		health.Metrics["error_runs"] = counts.Failed
		if rule.Enabled && rule.TriggerType == model.TriggerCron {
			var cfg model.TriggerConfigCron
			if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
				if schedule, _, err := resolveCronTriggerConfig(cfg); err == nil {
					if nextRun, err := automationcron.Next(schedule, time.Now()); err == nil {
						health.Metrics["next_run_at"] = nextRun.Format(time.RFC3339)
					}
				}
			}
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
		case "manual.repository_run":
			entries[idx].BindingCount = countRunnableAgentsForTarget(agents, "repository")
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

	fetchLimit := pagination.Page * pagination.PerPage
	executionPagination := model.PMPagination{Page: 1, PerPage: fetchLimit}
	executions, executionTotal, err := s.triggerExecRepo.ListByWorkspace(ctx, workspaceID, filters, executionPagination)
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

	actorNames := map[string]string{}
	if s.workspaceRepo != nil {
		members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list members for trigger executions: %w", err)
		}
		for _, member := range members {
			actorNames[member.UserID] = member.FullName
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
		actorName := (*string)(nil)
		if execution.ActorID != nil {
			if name := strings.TrimSpace(actorNames[strings.TrimSpace(*execution.ActorID)]); name != "" {
				actorName = &name
			}
		}
		items = append(items, model.AutomationTriggerExecutionListItem{
			ExecutionID:           execution.ID,
			ConditionOutcome:      execution.ConditionOutcome,
			ConditionAssessmentID: execution.ConditionAssessmentID,
			AgentID:               execution.AgentID,
			AgentName:             agentDisplayName(execution.AgentID, agentNames),
			ActorID:               execution.ActorID,
			ActorName:             actorName,
			BindingID:             execution.BindingID,
			BindingKind:           execution.BindingKind,
			BindingTitle:          bindingTitle,
			TriggerType:           execution.TriggerType,
			TriggerTitle:          triggerTitle,
			ReferenceID:           execution.ReferenceID,
			ReferenceType:         execution.ReferenceType,
			ReferenceTitle:        referenceTitle,
			ManagePath:            managePath,
			TargetType:            execution.TargetType,
			TargetID:              execution.TargetID,
			RunID:                 execution.RunID,
			Status:                execution.Status,
			ErrorMessage:          execution.ErrorMessage,
			FiredAt:               execution.FiredAt,
			StartedAt:             execution.StartedAt,
			CompletedAt:           execution.CompletedAt,
		})
	}
	total := executionTotal
	if s.runRepo != nil {
		runs, runTotal, err := s.runRepo.ListWorkspaceRunsWithoutTriggerExecutions(ctx, workspaceID, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		total += runTotal
		for _, run := range runs {
			items = append(items, automationActivityItemForRun(run, agentNames))
		}
	}
	if err := s.enrichActivityTargets(ctx, workspaceID, items); err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].FiredAt.Equal(items[j].FiredAt) {
			return strings.TrimSpace(items[i].ExecutionID) > strings.TrimSpace(items[j].ExecutionID)
		}
		return items[i].FiredAt.After(items[j].FiredAt)
	})
	offset := (pagination.Page - 1) * pagination.PerPage
	if offset >= len(items) {
		items = []model.AutomationTriggerExecutionListItem{}
	} else {
		end := offset + pagination.PerPage
		if end > len(items) {
			end = len(items)
		}
		items = items[offset:end]
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

type automationActivityTargetInfo struct {
	title string
	key   string
}

func (s *AutomationInventoryService) enrichActivityTargets(ctx context.Context, workspaceID string, items []model.AutomationTriggerExecutionListItem) error {
	if len(items) == 0 || strings.TrimSpace(workspaceID) == "" {
		return nil
	}

	idsByType := make(map[string][]string)
	seen := make(map[string]struct{})
	for _, item := range items {
		targetType := normalizeRunTargetType(derefString(item.TargetType))
		targetID := strings.TrimSpace(derefString(item.TargetID))
		if targetType == "" || targetID == "" {
			continue
		}
		key := targetType + ":" + targetID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		idsByType[targetType] = append(idsByType[targetType], targetID)
	}
	if len(idsByType) == 0 {
		return nil
	}

	targets := make(map[string]automationActivityTargetInfo)
	addTarget := func(targetType, targetID, title, key string) {
		title = strings.TrimSpace(title)
		key = strings.TrimSpace(key)
		if title == "" && key == "" {
			return
		}
		targets[normalizeRunTargetType(targetType)+":"+strings.TrimSpace(targetID)] = automationActivityTargetInfo{
			title: title,
			key:   key,
		}
	}

	if ids := idsByType["task"]; len(ids) > 0 && s.taskRepo != nil {
		tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity task targets: %w", err)
		}
		workspaceKey := ""
		if s.workspaceRepo != nil {
			ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
			if err != nil {
				return fmt.Errorf("resolve activity workspace key: %w", err)
			}
			if ws != nil {
				workspaceKey = strings.TrimSpace(ws.WorkspaceKey)
			}
		}
		for _, task := range tasks {
			taskKey := strconv.Itoa(task.DisplayID)
			if workspaceKey != "" {
				taskKey = model.FormatTaskKey(workspaceKey, task.DisplayID)
			}
			addTarget("task", task.ID, task.Name, taskKey)
		}
	}
	if ids := idsByType["epic"]; len(ids) > 0 && s.epicRepo != nil {
		epics, err := s.epicRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity epic targets: %w", err)
		}
		for _, epic := range epics {
			addTarget("epic", epic.ID, epic.Name, "")
		}
	}
	if ids := idsByType["document"]; len(ids) > 0 && s.docsDocumentRepo != nil {
		docs, err := s.docsDocumentRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity document targets: %w", err)
		}
		for _, doc := range docs {
			addTarget("document", doc.ID, doc.Title, "")
		}
	}
	if ids := idsByType["support_conversation"]; len(ids) > 0 && s.conversationRepo != nil {
		conversations, err := s.conversationRepo.ListTitlesByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity support conversation targets: %w", err)
		}
		for _, conversation := range conversations {
			addTarget("support_conversation", conversation.ID, conversation.Subject, "")
		}
	}
	if ids := idsByType["crm_contact"]; len(ids) > 0 && s.crmContactRepo != nil {
		contacts, err := s.crmContactRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity crm contact targets: %w", err)
		}
		for _, contact := range contacts {
			addTarget("crm_contact", contact.ID, crmContactDisplayName(&contact), "")
		}
	}
	if ids := idsByType["crm_deal"]; len(ids) > 0 && s.crmDealRepo != nil {
		deals, err := s.crmDealRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity crm deal targets: %w", err)
		}
		for _, deal := range deals {
			addTarget("crm_deal", deal.ID, deal.Name, "")
		}
	}
	if ids := idsByType["repository"]; len(ids) > 0 && s.gitRepo != nil {
		repos, err := s.gitRepo.ListByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity repository targets: %w", err)
		}
		for _, repo := range repos {
			addTarget("repository", repo.ID, repo.FullName, "")
		}
	}
	if ids := idsByType["workspace"]; len(ids) > 0 && s.workspaceRepo != nil {
		for _, id := range ids {
			ws, err := s.workspaceRepo.GetByID(ctx, id)
			if err != nil {
				return fmt.Errorf("resolve activity workspace targets: %w", err)
			}
			if ws != nil && ws.ID == workspaceID {
				addTarget("workspace", ws.ID, ws.Name, "")
			}
		}
	}
	if ids := idsByType["support_coverage_gap"]; len(ids) > 0 && s.supportCoverageRepo != nil {
		gaps, err := s.supportCoverageRepo.ListGapsByIDs(ctx, workspaceID, ids)
		if err != nil {
			return fmt.Errorf("resolve activity coverage gap targets: %w", err)
		}
		for _, gap := range gaps {
			addTarget("support_coverage_gap", gap.ID, gap.Title, "")
		}
	}

	for idx := range items {
		item := &items[idx]
		info, ok := targets[normalizeRunTargetType(derefString(item.TargetType))+":"+strings.TrimSpace(derefString(item.TargetID))]
		if !ok {
			continue
		}
		if info.title != "" {
			item.TargetTitle = strPtr(info.title)
		}
		if info.key != "" {
			item.TargetKey = strPtr(info.key)
		}
	}
	return nil
}

func automationActivityItemForRun(run model.AgentRun, agentNames map[string]string) model.AutomationTriggerExecutionListItem {
	input := model.AgentRunInputPayload{}
	_ = json.Unmarshal(run.Input, &input)
	source := strings.TrimSpace(model.AgentRunTriggerSourceManual)
	triggerType := strings.TrimSpace(model.AgentRunTriggerTypeManual)
	if input.Trigger != nil {
		if strings.TrimSpace(input.Trigger.Source) != "" {
			source = strings.TrimSpace(input.Trigger.Source)
		}
		if strings.TrimSpace(input.Trigger.TriggerType) != "" {
			triggerType = strings.TrimSpace(input.Trigger.TriggerType)
		}
	}
	bindingID := "agent_run.run"
	bindingKind := "agent_run"
	bindingTitle := "Agent run"
	triggerTitle := strPtr("Agent run")
	var referenceID *string
	var referenceType *string
	var referenceTitle *string
	if run.ParentRunID != nil && strings.TrimSpace(*run.ParentRunID) != "" {
		parentID := strings.TrimSpace(*run.ParentRunID)
		bindingID = "agent_run.child_run"
		bindingTitle = "Agent-started run"
		triggerTitle = strPtr("Agent-started run")
		referenceID = &parentID
		referenceType = strPtr("agent_run")
		referenceTitle = strPtr("Parent run")
	}
	if source == model.AgentRunTriggerSourceCommandBar {
		bindingID = "command_bar.run"
		bindingKind = model.AgentRunTriggerSourceCommandBar
		bindingTitle = "Command bar"
		triggerTitle = strPtr("Command bar")
	} else if source == model.AgentRunTriggerSourceManual && run.ParentRunID == nil {
		bindingKind = model.AgentRunTriggerSourceManual
		bindingID = manualActivityBindingIDForTargetType(run.TargetType)
		bindingTitle = "Manual run"
		triggerTitle = strPtr("Manual")
	}
	var triggerTypePtr *string
	if triggerType != "" {
		triggerTypePtr = &triggerType
	}
	targetType := nilIfEmpty(run.TargetType)
	targetID := nilIfEmpty(run.TargetID)
	runID := strings.TrimSpace(run.ID)
	return model.AutomationTriggerExecutionListItem{
		ExecutionID:    "run:" + runID,
		AgentID:        run.AgentID,
		AgentName:      agentDisplayName(run.AgentID, agentNames),
		BindingID:      bindingID,
		BindingKind:    bindingKind,
		BindingTitle:   bindingTitle,
		TriggerType:    triggerTypePtr,
		TriggerTitle:   triggerTitle,
		ReferenceID:    referenceID,
		ReferenceType:  referenceType,
		ReferenceTitle: referenceTitle,
		TargetType:     targetType,
		TargetID:       targetID,
		RunID:          &runID,
		Status:         run.Status,
		ErrorMessage:   run.ErrorMessage,
		FiredAt:        run.CreatedAt,
		StartedAt:      run.StartedAt,
		CompletedAt:    run.CompletedAt,
	}
}

func manualActivityBindingIDForTargetType(targetType string) string {
	switch strings.TrimSpace(targetType) {
	case "task":
		return "manual.task_run"
	case "epic":
		return "manual.epic_run"
	case "support_conversation":
		return "manual.support_run"
	case "repository":
		return "manual.repository_run"
	case "workspace":
		return "manual.workspace_run"
	default:
		return fmt.Sprintf("manual.%s_run", strings.ReplaceAll(defaultString(strings.TrimSpace(targetType), "target"), "_", "."))
	}
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
	switch strings.TrimSpace(execution.BindingKind) {
	case model.AgentRunTriggerSourceCommandBar:
		return "Command bar", nil
	case "agent_run":
		return "Agent run", nil
	}
	return defaultString(strings.TrimSpace(execution.BindingKind), "Trigger"), nil
}

func triggerTitleForExecution(execution model.AgentTriggerExecution) *string {
	if def, ok := automationcatalog.ResolveDefinitionForExecution(execution.BindingID, execution.BindingKind, derefString(execution.TriggerType)); ok {
		return strPtr(def.Title)
	}
	switch strings.TrimSpace(execution.BindingKind) {
	case model.AgentRunTriggerSourceCommandBar:
		return strPtr("Command bar")
	case "agent_run":
		return strPtr("Agent run")
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

func collectRuleIDs(rules []model.AutomationRule) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) != "" {
			ids = append(ids, rule.ID)
		}
	}
	return ids
}

func summarizeRuleExecution(execution model.AgentTriggerExecution) model.AutomationHealthSummary {
	lastSeenAt := timePointer(execution.FiredAt)
	status := model.AutomationHealthHealthy
	switch strings.TrimSpace(execution.Status) {
	case model.AgentTriggerExecutionStatusFailed:
		status = model.AutomationHealthError
	case model.AgentTriggerExecutionStatusCancelled, model.AgentTriggerExecutionStatusSkipped:
		status = model.AutomationHealthWarning
	}

	summary := model.AutomationHealthSummary{
		Status:     status,
		LastSeenAt: lastSeenAt,
		Freshness:  summarizeFreshness(status, lastSeenAt),
		Metrics: model.JSONB{
			"last_execution_id": execution.ID,
			"last_run_status":   execution.Status,
		},
	}
	if execution.RunID != nil && strings.TrimSpace(*execution.RunID) != "" {
		summary.Metrics["last_run_id"] = strings.TrimSpace(*execution.RunID)
	}
	if execution.CompletedAt != nil && strings.TrimSpace(execution.Status) == model.AgentTriggerExecutionStatusCompleted {
		completedAt := execution.CompletedAt.UTC()
		summary.LastSuccessAt = &completedAt
	}
	if execution.ErrorMessage != nil && strings.TrimSpace(*execution.ErrorMessage) != "" {
		msg := strings.TrimSpace(*execution.ErrorMessage)
		summary.LastErrorMessage = &msg
	}
	if strings.TrimSpace(execution.Status) == model.AgentTriggerExecutionStatusFailed {
		if execution.CompletedAt != nil {
			failedAt := execution.CompletedAt.UTC()
			summary.LastErrorAt = &failedAt
		} else {
			summary.LastErrorAt = lastSeenAt
		}
	}
	return summary
}
