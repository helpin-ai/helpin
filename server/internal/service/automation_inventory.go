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
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

const (
	automationGroupBuiltIn    = "built_in_automations"
	automationGroupContextual = "contextual_agents"
	automationGroupCustom     = "custom_automations"
)

type agentHealthProvider interface {
	GetRunnerHealth(ctx context.Context, workspaceID string) temporalapp.RunnerHealth
}

// AutomationInventoryService assembles the shared read-only automation inventory.
type AutomationInventoryService struct {
	settingsRepo         *repository.SettingsRepository
	pmAutomationRepo     *repository.PMAutomationRepository
	crmEmailRepo         *repository.CRMEmailRepository
	agentRepo            *repository.AgentRepository
	agentRunRepo         *repository.AgentRunRepository
	automationHealthRepo *repository.AutomationHealthRepository
	agentHealthProvider  agentHealthProvider
}

func NewAutomationInventoryService(
	settingsRepo *repository.SettingsRepository,
	pmAutomationRepo *repository.PMAutomationRepository,
	crmEmailRepo *repository.CRMEmailRepository,
	agentRepo *repository.AgentRepository,
	agentRunRepo *repository.AgentRunRepository,
	automationHealthRepo *repository.AutomationHealthRepository,
	agentHealthProvider agentHealthProvider,
) *AutomationInventoryService {
	return &AutomationInventoryService{
		settingsRepo:         settingsRepo,
		pmAutomationRepo:     pmAutomationRepo,
		crmEmailRepo:         crmEmailRepo,
		agentRepo:            agentRepo,
		agentRunRepo:         agentRunRepo,
		automationHealthRepo: automationHealthRepo,
		agentHealthProvider:  agentHealthProvider,
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

	contextualItems, err := s.contextualAgentItems(ctx, workspaceID, catalogByID)
	if err != nil {
		return nil, err
	}
	items = append(items, contextualItems...)

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
			{ID: automationGroupContextual, Title: "Contextual Agents", Description: "Explicit agents that run from PM or Support context and keep their current write surfaces."},
			{ID: automationGroupCustom, Title: "Custom Automations", Description: "Future user-created automations will appear here once the shared control plane supports them."},
		},
		Items:       items,
		GeneratedAt: time.Now().UTC(),
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

func (s *AutomationInventoryService) contextualAgentItems(ctx context.Context, workspaceID string, catalogByID map[string]model.AutomationCatalogEntry) ([]model.AutomationInventoryItem, error) {
	agents, err := s.agentRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list agents for automation inventory: %w", err)
	}

	runnerHealth := temporalapp.RunnerHealth{}
	if s.agentHealthProvider != nil {
		runnerHealth = s.agentHealthProvider.GetRunnerHealth(ctx, workspaceID)
	}
	activeRunByAgent := make(map[string]temporalapp.RunnerActiveRun)
	for _, run := range runnerHealth.ActiveRuns {
		if existing, ok := activeRunByAgent[run.AgentID]; ok && existing.CreatedAt.After(run.CreatedAt) {
			continue
		}
		activeRunByAgent[run.AgentID] = run
	}

	items := make([]model.AutomationInventoryItem, 0, len(agents))
	for _, agent := range agents {
		catalogID, ok := catalogIDForAgent(agent.AgentClass)
		if !ok {
			continue
		}
		entry, ok := catalogByID[catalogID]
		if !ok {
			continue
		}
		latestRun, latestRunErr := s.latestRunForAgent(ctx, workspaceID, agent.ID)
		if latestRunErr != nil {
			return nil, latestRunErr
		}
		health := summarizeAgentHealth(agent, latestRun, activeRunByAgent[agent.ID])
		enabled := strings.ToLower(strings.TrimSpace(agent.Status)) != "paused"
		items = append(items, inventoryItemFromCatalog(entry, fmt.Sprintf("%s:agent:%s", catalogID, agent.ID), model.AutomationScopeAgent, agent.ID, agent.Name, enabled, health))
	}

	return items, nil
}

func (s *AutomationInventoryService) latestRunForAgent(ctx context.Context, workspaceID, agentID string) (*model.AgentRun, error) {
	runs, _, err := s.agentRunRepo.ListByAgent(ctx, workspaceID, agentID, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil {
		return nil, fmt.Errorf("list latest agent run: %w", err)
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
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

func catalogIDForAgent(agentClass string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(agentClass)) {
	case model.AgentClassProductPlanner:
		return "pm.product_planner", true
	case model.AgentClassEngineer:
		return "pm.engineer", true
	case model.AgentClassReviewer:
		return "pm.reviewer", true
	case model.AgentClassSupport:
		return "support.support_agent", true
	default:
		return "", false
	}
}

func kindSortRank(kind string) int {
	switch kind {
	case model.AutomationKindBuiltIn:
		return 0
	case model.AutomationKindContextual:
		return 1
	case model.AutomationKindCustom:
		return 2
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

func summarizeAgentHealth(agent model.Agent, latestRun *model.AgentRun, activeRun temporalapp.RunnerActiveRun) model.AutomationHealthSummary {
	status := strings.ToLower(strings.TrimSpace(agent.Status))
	now := time.Now().UTC()
	if status == "paused" {
		return inactiveHealth("Paused")
	}
	if status == "error" {
		return model.AutomationHealthSummary{
			Status:    model.AutomationHealthError,
			Freshness: "attention_needed",
			Metrics:   model.JSONB{"agent_status": status},
		}
	}

	if activeRun.ID != "" {
		metrics := model.JSONB{
			"run_status": activeRun.Status,
			"task_queue": activeRun.TaskQueue,
		}
		freshness := "active"
		status := model.AutomationHealthHealthy
		if activeRun.Stale {
			status = model.AutomationHealthWarning
			freshness = "stale"
		}
		return model.AutomationHealthSummary{
			Status:     status,
			LastSeenAt: &now,
			Freshness:  freshness,
			Metrics:    metrics,
		}
	}

	if latestRun != nil {
		var healthStatus string
		switch latestRun.Status {
		case "completed":
			healthStatus = model.AutomationHealthHealthy
		case "failed":
			healthStatus = model.AutomationHealthError
		case "cancelled":
			healthStatus = model.AutomationHealthWarning
		default:
			healthStatus = model.AutomationHealthUnknown
		}
		return model.AutomationHealthSummary{
			Status:           healthStatus,
			LastSeenAt:       timePointer(latestRun.UpdatedAt),
			LastSuccessAt:    latestRun.CompletedAt,
			LastErrorAt:      errorTimeForRun(latestRun),
			LastErrorMessage: latestRun.ErrorMessage,
			Freshness:        summarizeFreshness(healthStatus, timePointer(latestRun.UpdatedAt)),
			Metrics: model.JSONB{
				"latest_run_status": latestRun.Status,
				"trigger_mode":      agent.TriggerMode,
			},
		}
	}

	return unknownHealth()
}

func errorTimeForRun(run *model.AgentRun) *time.Time {
	if run == nil || run.ErrorMessage == nil {
		return nil
	}
	if run.CompletedAt != nil {
		return run.CompletedAt
	}
	return timePointer(run.UpdatedAt)
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
