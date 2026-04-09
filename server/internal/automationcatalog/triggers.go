package automationcatalog

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type triggerBindingDefinition struct {
	catalogID        string
	bindingKind      string
	category         string
	triggerType      string
	title            string
	description      string
	sourceSurface    string
	configSurface    *string
	supportsAgentRun bool
	aliases          []string
	showRulesSearch  *model.WorkflowRuleSearchPreset
	createRuleSearch *model.WorkflowRuleSearchPreset
}

func TriggerCatalog() []model.AutomationTriggerCatalogEntry {
	definitions := triggerBindingDefinitions()
	items := make([]model.AutomationTriggerCatalogEntry, 0, len(definitions))
	for _, def := range definitions {
		items = append(items, model.AutomationTriggerCatalogEntry{
			ID:                def.catalogID,
			BindingKind:       def.bindingKind,
			Category:          def.category,
			TriggerType:       def.triggerType,
			Title:             def.title,
			Description:       def.description,
			SourceSurface:     def.sourceSurface,
			ConfigSurface:     cloneStringPtr(def.configSurface),
			SupportsAgentRuns: def.supportsAgentRun,
			ExecutionSearch:   executionSearchPresetForDefinition(def, nil),
			ShowRulesSearch:   cloneWorkflowRuleSearchPreset(def.showRulesSearch),
			CreateRuleSearch:  cloneWorkflowRuleSearchPreset(def.createRuleSearch),
		})
	}
	return items
}

func ResolveDefinitionForExecution(
	bindingID, bindingKind, triggerType string,
) (model.AutomationTriggerCatalogEntry, bool) {
	def, ok := resolveTriggerBindingDefinition(bindingID, bindingKind, triggerType)
	if !ok {
		return model.AutomationTriggerCatalogEntry{}, false
	}
	return model.AutomationTriggerCatalogEntry{
		ID:                def.catalogID,
		BindingKind:       def.bindingKind,
		Category:          def.category,
		TriggerType:       def.triggerType,
		Title:             def.title,
		Description:       def.description,
		SourceSurface:     def.sourceSurface,
		ConfigSurface:     cloneStringPtr(def.configSurface),
		SupportsAgentRuns: def.supportsAgentRun,
		ExecutionSearch:   executionSearchPresetForDefinition(def, nil),
		ShowRulesSearch:   cloneWorkflowRuleSearchPreset(def.showRulesSearch),
		CreateRuleSearch:  cloneWorkflowRuleSearchPreset(def.createRuleSearch),
	}, true
}

func ResolveBindingForTrigger(source, triggerType, targetType string) (bindingID, bindingKind string, ok bool) {
	source = strings.TrimSpace(source)
	triggerType = strings.TrimSpace(triggerType)
	targetType = strings.TrimSpace(targetType)

	switch source {
	case model.AgentRunTriggerSourceManual:
		switch targetType {
		case "task":
			return "manual.task_run", "manual", true
		case "epic":
			return "manual.epic_run", "manual", true
		case "support_conversation":
			return "manual.support_run", "manual", true
		}
	case model.AgentRunTriggerSourceSchedule:
		return "agent.schedule", "schedule", true
	case model.AgentRunTriggerSourceAutomationRule:
		if triggerType == model.TriggerCron {
			return "automation_rule.cron", "automation_rule", true
		}
		if def, ok := resolveTriggerBindingDefinition(triggerType, "automation_rule", triggerType); ok {
			return def.catalogID, def.bindingKind, true
		}
	case model.AgentRunTriggerSourceSystem:
		switch triggerType {
		case "support.auto":
			return "support.widget_message", "support_widget", true
		case "task.assigned_agent_state_change":
			return "task.assigned_agent_state_change", "task_assignment", true
		}
	}

	return "", "", false
}

func ExecutionSearchPresetForDefinitionID(catalogID string, referenceID *string) *model.TriggerExecutionSearchPreset {
	def, ok := definitionByCatalogID(catalogID)
	if !ok {
		return nil
	}
	return executionSearchPresetForDefinition(def, referenceID)
}

func ExecutionSearchPresetForTrigger(source, triggerType, targetType string, referenceID *string) *model.TriggerExecutionSearchPreset {
	bindingID, _, ok := ResolveBindingForTrigger(source, triggerType, targetType)
	if !ok {
		return nil
	}
	return ExecutionSearchPresetForDefinitionID(bindingID, referenceID)
}

func definitionByCatalogID(catalogID string) (triggerBindingDefinition, bool) {
	for _, def := range triggerBindingDefinitions() {
		if def.catalogID == strings.TrimSpace(catalogID) {
			return def, true
		}
	}
	return triggerBindingDefinition{}, false
}

func resolveTriggerBindingDefinition(bindingID, bindingKind, triggerType string) (triggerBindingDefinition, bool) {
	bindingID = strings.TrimSpace(bindingID)
	bindingKind = strings.TrimSpace(bindingKind)
	triggerType = strings.TrimSpace(triggerType)

	for _, def := range triggerBindingDefinitions() {
		if def.catalogID == bindingID {
			return def, true
		}
		for _, alias := range def.aliases {
			if alias == bindingID {
				return def, true
			}
		}
	}

	if bindingKind == "automation_rule" {
		if triggerType == model.TriggerCron {
			return definitionByCatalogID("automation_rule.cron")
		}
		for _, def := range triggerBindingDefinitions() {
			if def.bindingKind == "automation_rule" && def.triggerType == triggerType {
				return def, true
			}
		}
	}

	for _, def := range triggerBindingDefinitions() {
		if def.bindingKind == bindingKind && def.triggerType == triggerType && triggerType != "" {
			return def, true
		}
	}

	return triggerBindingDefinition{}, false
}

func executionSearchPresetForDefinition(def triggerBindingDefinition, referenceID *string) *model.TriggerExecutionSearchPreset {
	switch def.bindingKind {
	case "manual":
		return &model.TriggerExecutionSearchPreset{
			BindingID: strPtr(def.catalogID),
			Source:    strPtr(def.bindingKind),
		}
	case "schedule":
		return &model.TriggerExecutionSearchPreset{
			Source: strPtr(def.bindingKind),
		}
	case "automation_rule":
		preset := &model.TriggerExecutionSearchPreset{
			Source:      strPtr(def.bindingKind),
			TriggerType: strPtr(def.triggerType),
		}
		if referenceID != nil && strings.TrimSpace(*referenceID) != "" {
			preset.ReferenceID = strPtr(strings.TrimSpace(*referenceID))
		}
		return preset
	case "support_widget":
		return &model.TriggerExecutionSearchPreset{
			Source: strPtr(def.bindingKind),
		}
	case "task_assignment":
		return &model.TriggerExecutionSearchPreset{
			Source: strPtr(def.bindingKind),
		}
	default:
		return nil
	}
}

func triggerBindingDefinitions() []triggerBindingDefinition {
	workflowsPath := strPtr("/w/$slug/automation/flows")
	chatPath := strPtr("/w/$slug/settings/chat-general")
	agentsPath := strPtr("/w/$slug/automation/agents")

	return []triggerBindingDefinition{
		{
			catalogID:        "manual.task_run",
			bindingKind:      "manual",
			category:         "manual",
			triggerType:      model.AgentRunTriggerTypeManual,
			title:            "Manual Task Run",
			description:      "A human starts an agent from a task.",
			sourceSurface:    "Task detail and task list run actions",
			configSurface:    agentsPath,
			supportsAgentRun: true,
		},
		{
			catalogID:        "manual.epic_run",
			bindingKind:      "manual",
			category:         "manual",
			triggerType:      model.AgentRunTriggerTypeManual,
			title:            "Manual Epic Run",
			description:      "A human starts an agent from an epic.",
			sourceSurface:    "Epic detail planning and run actions",
			configSurface:    agentsPath,
			supportsAgentRun: true,
		},
		{
			catalogID:        "manual.support_run",
			bindingKind:      "manual",
			category:         "manual",
			triggerType:      model.AgentRunTriggerTypeManual,
			title:            "Manual Support Run",
			description:      "A human starts the assigned support agent from a conversation.",
			sourceSurface:    "Support inbox run-agent actions",
			configSurface:    agentsPath,
			supportsAgentRun: true,
		},
		{
			catalogID:        "task.state_entered",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerTaskStateEntered,
			title:            "Task State Entered",
			description:      "Fires when a task is created in or moved into a workflow state.",
			sourceSurface:    "PM workflow state changes",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerTaskStateEntered),
				ShowTriggerTitle: strPtr("Task State Entered"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				Template:            strPtr(model.TriggerTaskStateEntered),
				TemplateTitle:       strPtr("Task State Entered template"),
				TemplateDescription: strPtr("Choose a workflow state and agent in Flows to create a task.state_entered automation for the selected state."),
			},
		},
		{
			catalogID:        "agent_run.approved",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerAgentRunApproved,
			title:            "Agent Run Approved",
			description:      "Fires when an interactive task run is explicitly approved by a human.",
			sourceSurface:    "Agent run approvals",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerAgentRunApproved),
				ShowTriggerTitle: strPtr("Agent Run Approved"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				Template:            strPtr(model.TriggerAgentRunApproved),
				TemplateTitle:       strPtr("Agent Run Approved template"),
				TemplateDescription: strPtr("Choose a workflow state and follow-up action in Flows to create an agent_run.approved automation for the selected state."),
			},
		},
		{
			catalogID:        "github.push",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubPush,
			title:            "GitHub Push",
			description:      "Fires when the GitHub integration receives a push webhook. Filters can scope by repo and branch.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubPush),
				ShowTriggerTitle: strPtr("GitHub Push"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubPush),
				Branch:          strPtr("main"),
			},
		},
		{
			catalogID:        "github.pull_request_opened",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubPROpened,
			title:            "GitHub Pull Request Opened",
			description:      "Fires when the GitHub integration receives a pull request opened webhook. Filters can scope by repository and PR base branch.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubPROpened),
				ShowTriggerTitle: strPtr("GitHub Pull Request Opened"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubPROpened),
			},
		},
		{
			catalogID:        "github.pull_request_merged",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubPRMerged,
			title:            "GitHub Pull Request Merged",
			description:      "Fires when the GitHub integration receives a merged pull request webhook. Filters can scope by repository and PR base branch.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubPRMerged),
				ShowTriggerTitle: strPtr("GitHub Pull Request Merged"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubPRMerged),
				BaseBranch:      strPtr("main"),
			},
		},
		{
			catalogID:        "github.pull_request_review_requested",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubPRReviewReq,
			title:            "GitHub PR Review Requested",
			description:      "Fires when the GitHub integration receives a pull request review requested webhook. Filters can scope by repository and PR base branch.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubPRReviewReq),
				ShowTriggerTitle: strPtr("GitHub Review Requested"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubPRReviewReq),
			},
		},
		{
			catalogID:        "github.release_published",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubReleasePub,
			title:            "GitHub Release Published",
			description:      "Fires when the GitHub integration receives a release published webhook. Filters can scope by repo and tag.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubReleasePub),
				ShowTriggerTitle: strPtr("GitHub Release Published"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubReleasePub),
			},
		},
		{
			catalogID:        "github.check_suite_completed",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerGitHubCheckSuite,
			title:            "GitHub Check Suite Completed",
			description:      "Fires when the GitHub integration receives a completed check suite webhook. Filters can scope by repo, branch, and conclusion.",
			sourceSurface:    "GitHub App webhook delivery",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerGitHubCheckSuite),
				ShowTriggerTitle: strPtr("GitHub Check Suite Completed"),
			},
			createRuleSearch: &model.WorkflowRuleSearchPreset{
				CreateEventRule: true,
				TriggerType:     strPtr(model.TriggerGitHubCheckSuite),
			},
		},
		{
			catalogID:        "agent.schedule",
			bindingKind:      "schedule",
			category:         "agent",
			triggerType:      model.TriggerCron,
			title:            "Agent Schedule",
			description:      "Runs an agent directly from its own cron schedule.",
			sourceSurface:    "Agent configuration",
			configSurface:    agentsPath,
			supportsAgentRun: true,
			aliases:          []string{"agent_schedule"},
		},
		{
			catalogID:        "support.widget_message",
			bindingKind:      "support_widget",
			category:         "support",
			triggerType:      "support.auto",
			title:            "Support Widget Message",
			description:      "Runs the configured support agent automatically on new visitor messages.",
			sourceSurface:    "Support widget AI auto-replies",
			configSurface:    chatPath,
			supportsAgentRun: true,
			aliases:          []string{"support_widget_ai"},
		},
		{
			catalogID:        "task.assigned_agent_state_change",
			bindingKind:      "task_assignment",
			category:         "pm",
			triggerType:      "task.assigned_agent_state_change",
			title:            "Assigned Agent On Task State Change",
			description:      "When a task has an assigned agent, state changes auto-start that agent for the task.",
			sourceSurface:    "Task workflow transitions",
			configSurface:    agentsPath,
			supportsAgentRun: true,
			aliases:          []string{"task_assignment"},
		},
		{
			catalogID:        "automation_rule.cron",
			bindingKind:      "automation_rule",
			category:         "automation_rule",
			triggerType:      model.TriggerCron,
			title:            "Automation Rule Cron",
			description:      "Runs a start-agent-run automation rule on a backend cron category. The engine supports it, but Flows does not yet surface cron authoring.",
			sourceSurface:    "Rule engine backend",
			configSurface:    workflowsPath,
			supportsAgentRun: true,
			showRulesSearch: &model.WorkflowRuleSearchPreset{
				ShowTrigger:      strPtr(model.TriggerCron),
				ShowTriggerTitle: strPtr("Automation Rule Cron"),
			},
		},
	}
}

func strPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return strPtr(*value)
}

func cloneWorkflowRuleSearchPreset(value *model.WorkflowRuleSearchPreset) *model.WorkflowRuleSearchPreset {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.ShowTrigger = cloneStringPtr(value.ShowTrigger)
	cloned.ShowTriggerTitle = cloneStringPtr(value.ShowTriggerTitle)
	cloned.Template = cloneStringPtr(value.Template)
	cloned.TemplateTitle = cloneStringPtr(value.TemplateTitle)
	cloned.TemplateDescription = cloneStringPtr(value.TemplateDescription)
	cloned.TriggerType = cloneStringPtr(value.TriggerType)
	cloned.AgentID = cloneStringPtr(value.AgentID)
	cloned.RepoFullName = cloneStringPtr(value.RepoFullName)
	cloned.Branch = cloneStringPtr(value.Branch)
	cloned.BaseBranch = cloneStringPtr(value.BaseBranch)
	cloned.TagName = cloneStringPtr(value.TagName)
	cloned.Conclusion = cloneStringPtr(value.Conclusion)
	cloned.TargetMode = cloneStringPtr(value.TargetMode)
	cloned.TargetID = cloneStringPtr(value.TargetID)
	return &cloned
}
