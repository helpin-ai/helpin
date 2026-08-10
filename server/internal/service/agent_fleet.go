package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"golang.org/x/sync/errgroup"
)

const agentFleetRecentRunLimit = 5

// GetAgentFleet returns the bounded read model used by the agents overview.
// Repository calls are performed per data set, never per agent.
func (s *AgentService) GetAgentFleet(
	ctx context.Context,
	workspaceID string,
	actor *authorization.Actor,
) (*model.AgentFleetResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	agents, err := s.ListAgentsForActor(ctx, workspaceID, actor)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	windowStartedAt := now.Add(-7 * 24 * time.Hour)
	response := &model.AgentFleetResponse{
		GeneratedAt:     now,
		WindowStartedAt: windowStartedAt,
		Agents:          make([]model.AgentFleetItem, 0, len(agents)),
	}
	if len(agents) == 0 {
		return response, nil
	}

	agentIDs := make([]string, 0, len(agents))
	for _, agent := range agents {
		agentIDs = append(agentIDs, agent.ID)
	}

	var (
		aggregates    []repository.AgentRunFleetAggregate
		recentRuns    []model.AgentRun
		attentionRuns []model.AgentRun
		usageByAgent  map[string]model.AgentTriggerUsageSummary
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var queryErr error
		aggregates, queryErr = s.runRepo.SummarizeFleetSince(groupCtx, workspaceID, agentIDs, windowStartedAt)
		return queryErr
	})
	group.Go(func() error {
		var queryErr error
		recentRuns, queryErr = s.runRepo.ListRecentByAgentIDs(groupCtx, workspaceID, agentIDs, agentFleetRecentRunLimit)
		return queryErr
	})
	group.Go(func() error {
		var queryErr error
		attentionRuns, queryErr = s.runRepo.ListPausedByAgentIDs(groupCtx, workspaceID, agentIDs)
		return queryErr
	})
	group.Go(func() error {
		var queryErr error
		usageByAgent, queryErr = s.getAgentFleetUsage(groupCtx, workspaceID, agents)
		return queryErr
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	recentRuns = s.normalizeRunCollection(s.reconcileStuckRuns(ctx, recentRuns))
	attentionRuns = s.normalizeRunCollection(s.reconcileStuckRuns(ctx, attentionRuns))
	s.enrichRunTargets(ctx, workspaceID, recentRuns)
	s.enrichRunTargets(ctx, workspaceID, attentionRuns)
	compactAgentFleetRunInputs(recentRuns)
	compactAgentFleetRunInputs(attentionRuns)

	statsByAgent := make(map[string]*model.AgentFleetStats, len(agents))
	for _, agentID := range agentIDs {
		statsByAgent[agentID] = &model.AgentFleetStats{RecentRunItems: []model.AgentRun{}}
	}
	for _, aggregate := range aggregates {
		stats := statsByAgent[aggregate.AgentID]
		if stats == nil {
			continue
		}
		stats.RecentRuns = aggregate.RecentRuns
		stats.RecentCompleted = aggregate.RecentCompleted
		stats.RecentFailed = aggregate.RecentFailed
		stats.RecentTokens = aggregate.RecentTokens
	}
	for idx := range recentRuns {
		run := recentRuns[idx]
		stats := statsByAgent[run.AgentID]
		if stats == nil {
			continue
		}
		stats.RecentRunItems = append(stats.RecentRunItems, run)
		if stats.LastRun == nil {
			runCopy := run
			stats.LastRun = &runCopy
		}
	}
	for idx := range attentionRuns {
		run := attentionRuns[idx]
		stats := statsByAgent[run.AgentID]
		if stats == nil || !isFleetAttentionRun(run) {
			continue
		}
		stats.AttentionCount++
		if stats.AttentionRun == nil || fleetAttentionPriority(run) > fleetAttentionPriority(*stats.AttentionRun) {
			runCopy := run
			stats.AttentionRun = &runCopy
		}
	}

	for _, agent := range agents {
		stats := statsByAgent[agent.ID]
		if stats == nil {
			stats = &model.AgentFleetStats{RecentRunItems: []model.AgentRun{}}
		}
		usage := usageByAgent[agent.ID]
		usage.AgentID = agent.ID
		usage.AgentName = agent.Name
		if usage.Items == nil {
			usage.Items = []model.AgentTriggerUsage{}
		}
		response.Agents = append(response.Agents, model.AgentFleetItem{
			Agent: agent,
			Stats: *stats,
			Usage: usage,
		})
	}
	return response, nil
}

func (s *AgentService) getAgentFleetUsage(
	ctx context.Context,
	workspaceID string,
	agents []model.Agent,
) (map[string]model.AgentTriggerUsageSummary, error) {
	usageByAgent := make(map[string]model.AgentTriggerUsageSummary, len(agents))
	visibleAgentIDs := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		visibleAgentIDs[agent.ID] = struct{}{}
		usageByAgent[agent.ID] = model.AgentTriggerUsageSummary{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Items:     []model.AgentTriggerUsage{},
		}
	}

	if s.automationRuleRepo != nil {
		rules, err := s.automationRuleRepo.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list automation rules for agent fleet: %w", err)
		}
		managePath := "/w/$slug/settings/workflows"
		for _, rule := range rules {
			if rule.ActionType != model.ActionStartAgentRun {
				continue
			}
			var cfg model.ActionConfigRunAgent
			if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
				continue
			}
			agentID := strings.TrimSpace(cfg.AgentID)
			if _, ok := visibleAgentIDs[agentID]; !ok {
				continue
			}
			ruleID := rule.ID
			triggerType := rule.TriggerType
			referenceType := "automation_rule"
			summary := usageByAgent[agentID]
			summary.Items = append(summary.Items, model.AgentTriggerUsage{
				ID:            "automation_rule:" + rule.ID,
				Kind:          "automation_rule",
				Title:         rule.Name,
				Description:   describeAutomationRuleBinding(rule),
				TriggerType:   &triggerType,
				Enabled:       rule.Enabled,
				ReferenceID:   &ruleID,
				ReferenceType: &referenceType,
				ManagePath:    &managePath,
			})
			usageByAgent[agentID] = summary
		}
	}

	if s.installationRepo != nil {
		installation, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("get support installation for agent fleet: %w", err)
		}
		if installation != nil {
			settings := parseSettings(installation.Settings)
			agentID := strings.TrimSpace(derefString(settings.AIAgentID))
			if settings.AIEnabled {
				if _, ok := visibleAgentIDs[agentID]; ok {
					triggerType := supportAutoTriggerType
					managePath := "/w/$slug/settings/chat-general"
					summary := usageByAgent[agentID]
					summary.Items = append(summary.Items, model.AgentTriggerUsage{
						ID:          "support.widget_message",
						Kind:        "support_widget",
						Title:       "Support widget AI auto-replies",
						Description: "Runs this agent automatically on new visitor messages in the chat widget.",
						TriggerType: &triggerType,
						Enabled:     true,
						ManagePath:  &managePath,
					})
					usageByAgent[agentID] = summary
				}
			}
		}
	}
	return usageByAgent, nil
}

func compactAgentFleetRunInputs(runs []model.AgentRun) {
	for idx := range runs {
		var input model.AgentRunInputPayload
		if err := json.Unmarshal(runs[idx].Input, &input); err != nil || input.Trigger == nil {
			runs[idx].Input = json.RawMessage(`{}`)
			continue
		}
		compact := struct {
			Trigger *struct {
				Source      string `json:"source,omitempty"`
				TriggerType string `json:"trigger_type,omitempty"`
			} `json:"trigger,omitempty"`
		}{
			Trigger: &struct {
				Source      string `json:"source,omitempty"`
				TriggerType string `json:"trigger_type,omitempty"`
			}{Source: input.Trigger.Source, TriggerType: input.Trigger.TriggerType},
		}
		encoded, err := json.Marshal(compact)
		if err != nil {
			runs[idx].Input = json.RawMessage(`{}`)
			continue
		}
		runs[idx].Input = encoded
	}
}

func isFleetAttentionRun(run model.AgentRun) bool {
	if run.Status != model.AgentRunStatusPaused {
		return false
	}
	return run.PauseReason != model.AgentRunPauseReasonUserMessage
}

func fleetAttentionPriority(run model.AgentRun) int {
	if run.PauseReason == model.AgentRunPauseReasonHumanApproval || run.ApprovalState == "pending" {
		return 3
	}
	if run.PauseReason == model.AgentRunPauseReasonAuthentication {
		return 2
	}
	return 1
}
