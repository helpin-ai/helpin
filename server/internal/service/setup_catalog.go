package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetupEvidence is kept as a service-level alias for catalog evaluation.
type SetupEvidence = model.SetupEvidence

type setupTaskDefinition struct {
	key, title, description, stage, actionKey, actionLabel string
	completed                                              func(SetupEvidence) bool
}

type setupJourneyDefinition struct {
	key, title, description, accent string
	tasks                           []setupTaskDefinition
	established                     func(SetupEvidence) bool
	activated                       func(SetupEvidence) bool
}

var setupJourneyCatalog = map[string]setupJourneyDefinition{
	model.SetupGoalFoundation: {
		key: model.SetupGoalFoundation, title: "Workspace essentials", accent: "indigo",
		description: "Give your team enough context and structure to work well together.",
		activated:   func(e SetupEvidence) bool { return e.HasCompanyContext && e.TeamCount > 0 },
		established: func(e SetupEvidence) bool { return e.HasCompanyContext && e.TeamCount > 0 && e.ActiveMemberCount > 1 },
		tasks: []setupTaskDefinition{
			{"foundation.company_context_ready", "Add company details so Helpin understands your business.", "", "Core", "workspace_context", "Add context", func(e SetupEvidence) bool { return e.HasCompanyContext }},
			{"foundation.team_ready", "Create a team so work has a clear owner.", "", "Core", "workspace_teams", "Create team", func(e SetupEvidence) bool { return e.TeamCount > 0 }},
			{"foundation.member_joined", "Have a teammate join so progress can be shared.", "", "Core", "workspace_members", "View members", func(e SetupEvidence) bool { return e.ActiveMemberCount > 1 }},
		},
	},
	model.SetupGoalProductDelivery: {
		key: model.SetupGoalProductDelivery, title: "Plan and ship team projects", accent: "blue",
		description: "Turn a clear project plan into owned work, shipped outcomes, and automated updates.",
		activated:   func(e SetupEvidence) bool { return e.SprintCloseoutCount > 0 },
		established: func(e SetupEvidence) bool { return e.ProductAgentRunCount > 0 || e.ReleaseNotesSuccessCount > 0 },
		tasks: []setupTaskDefinition{
			{"product.project_planned", "Create an epic with an owner and timeline so the team knows what it is delivering.", "", "Core", "pm_epics", "Plan epic", func(e SetupEvidence) bool { return e.PlannedProjectCount > 0 }},
			{"product.sprint_planned", "Create a sprint with planned work so the team can execute a clear delivery cycle.", "", "Core", "pm_sprints", "Plan sprint", func(e SetupEvidence) bool { return e.PlannedSprintCount > 0 }},
			{"product.work_assigned", "Assign project work to teammates so every task has clear ownership.", "", "Core", "pm_tasks", "Assign work", func(e SetupEvidence) bool { return e.AssignedProjectTaskCount > 0 }},
			{"product.sprint_closeout_reviewable", "Close a sprint so the team can review what shipped and improve the next cycle.", "", "Core", "pm_sprints", "Review sprints", func(e SetupEvidence) bool { return e.SprintCloseoutCount > 0 }},
			{"product.repository_ready", "Connect a code repository so Helpin can link project work to what you ship.", "", "Power", "git_settings", "Connect repository", func(e SetupEvidence) bool { return e.ConnectedRepositoryCount > 0 }},
			{"product.agent_result_used", "Run an agent on project work so planning or review takes less manual effort.", "", "Power", "product_agent", "Run planning agent", func(e SetupEvidence) bool { return e.ProductAgentRunCount > 0 }},
			{"product.release_notes_flow_succeeded", "Run release-notes automation so customer updates are created from shipped work.", "", "Power", "automation_flows", "Build automation", func(e SetupEvidence) bool { return e.ReleaseNotesSuccessCount > 0 }},
		},
	},
	model.SetupGoalCustomerSupport: {
		key: model.SetupGoalCustomerSupport, title: "Scale customer support", accent: "emerald",
		description: "Connect every support channel, equip AI with trusted knowledge, and route customer issues to the right team.",
		activated:   func(e SetupEvidence) bool { return e.SupportAIAgentActive },
		established: func(e SetupEvidence) bool { return e.TeamInboxCount > 0 && e.AutomaticRoutingCount > 0 },
		tasks: []setupTaskDefinition{
			{"support.email_inbox_connected", "Connect your support email inbox so customer emails arrive in Helpin.", "Receive customer support emails in Helpin.", "Core", "support_email_inbox", "Connect email", func(e SetupEvidence) bool { return e.SupportEmailInboxCount > 0 }},
			{"support.live_chat_installed", "Add Helpin live chat to your website so customers can contact you instantly.", "Let website visitors start support conversations in Helpin.", "Core", "support_live_chat", "Add live chat", func(e SetupEvidence) bool { return e.LiveChatInstallationCount > 0 }},
			{"support.help_docs_ready", "Import or create public help docs so customers can find answers themselves.", "Publish answers customers can find without contacting support.", "Core", "support_help_docs", "Add help docs", func(e SetupEvidence) bool { return e.PublicHelpDocCount > 0 }},
			{"support.brand_knowledge_ready", "Add and sync a brand knowledge source so the AI support agent gives accurate, on-brand answers.", "Give the AI support agent trusted brand and product context.", "Core", "support_brand_knowledge", "Add knowledge", func(e SetupEvidence) bool { return e.BrandKnowledgeSourceCount > 0 }},
			{"support.ai_agent_activated", "Activate the AI support agent so common customer questions can be answered automatically.", "Let AI handle common questions using your approved knowledge.", "Core", "support_ai", "Activate AI agent", func(e SetupEvidence) bool { return e.SupportAIAgentActive }},
			{"support.team_inbox_created", "Create a team inbox so conversations have clear ownership.", "Give a support team a focused place to own conversations.", "Core", "support_team_inboxes", "Create inbox", func(e SetupEvidence) bool { return e.TeamInboxCount > 0 }},
			{"support.routing_enabled", "Turn on automatic routing so every conversation reaches the right team.", "Send incoming conversations to the right team automatically.", "Core", "support_routing", "Set up routing", func(e SetupEvidence) bool { return e.AutomaticRoutingCount > 0 }},
			{"support.pm_task_linked", "Create or link a task from a customer issue so feedback becomes product work.", "Connect customer feedback to work the product team can track.", "Power", "support_inbox", "Open inbox", func(e SetupEvidence) bool { return e.LinkedSupportTaskCount > 0 }},
			{"support.coverage_fix_applied", "Review a coverage gap and apply a fix so the AI can answer more customer questions.", "Turn a known answer gap into better support knowledge.", "Power", "support_coverage", "Review coverage", func(e SetupEvidence) bool { return e.CoverageImprovementCount > 0 }},
		},
	},
	model.SetupGoalHelpCenterDocs: {
		key: model.SetupGoalHelpCenterDocs, title: "Publish help center docs", accent: "emerald",
		description: "Give customers a searchable place to find accurate answers before they contact support.",
		activated:   func(e SetupEvidence) bool { return e.PublicHelpDocCount > 0 }, established: func(e SetupEvidence) bool { return e.HelpCenterSiteCount > 0 },
		tasks: []setupTaskDefinition{
			{"help_center.space_ready", "Create a public help-center space so customer content has a clear home.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.HelpCenterSpaceCount > 0 }},
			{"help_center.content_ready", "Create or import customer-facing articles so common questions are documented.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.HelpCenterContentCount > 0 }},
			{"help_center.article_published", "Publish an article so customers can use it to solve a real question.", "", "Core", "help_center_article_publish", "Publish article", func(e SetupEvidence) bool { return e.PublicHelpDocCount > 0 }},
			{"help_center.site_published", "Publish your help center so customers can browse and search your documentation.", "", "Core", "help_center_settings", "Publish help center", func(e SetupEvidence) bool { return e.HelpCenterSiteCount > 0 }},
			{"help_center.widget_connected", "Add help-center content to live chat so customers can find answers before starting a conversation.", "", "Power", "help_center_widget", "Add to live chat", func(e SetupEvidence) bool { return e.HelpCenterWidgetCount > 0 }},
		},
	},
	model.SetupGoalInternalDocs: {
		key: model.SetupGoalInternalDocs, title: "Build internal knowledge", accent: "indigo",
		description: "Create trusted company knowledge that stays current and gives AI reliable context.",
		activated:   func(e SetupEvidence) bool { return e.InternalDocsPublishedCount > 0 }, established: func(e SetupEvidence) bool { return e.InternalAgentKnowledgeCount > 0 },
		tasks: []setupTaskDefinition{
			{"internal_docs.space_ready", "Create an internal knowledge space so company information has a trusted home.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.InternalDocsSpaceCount > 0 }},
			{"internal_docs.content_ready", "Create or import internal documents so teammates can find essential information.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.InternalDocsContentCount > 0 }},
			{"internal_docs.published", "Publish internal knowledge so it is available across the workspace.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.InternalDocsPublishedCount > 0 }},
			{"internal_docs.ownership_ready", "Assign document owners and review dates so important knowledge stays current.", "", "Core", "docs_home", "Open Docs", func(e SetupEvidence) bool { return e.InternalDocsOwnershipCount > 0 }},
			{"internal_docs.agent_connected", "Connect internal knowledge to an AI agent so it can answer with trusted company context.", "", "Power", "internal_docs_agent_knowledge", "Connect agent", func(e SetupEvidence) bool { return e.InternalAgentKnowledgeCount > 0 }},
			{"internal_docs.agent_succeeded", "Run the documentation agent on a real document so maintaining knowledge takes less manual effort.", "", "Power", "internal_docs_agent_run", "Run documentation agent", func(e SetupEvidence) bool { return e.InternalDocAgentSuccessCount > 0 }},
		},
	},
	model.SetupGoalSalesCRM: {
		key: model.SetupGoalSalesCRM, title: "Build a sales pipeline", accent: "blue",
		description: "Centralize customer context, make opportunities actionable, and automate deal progression.",
		activated:   func(e SetupEvidence) bool { return e.CRMActionableDealCount > 0 }, established: func(e SetupEvidence) bool { return e.CRMSignalValueCount > 0 },
		tasks: []setupTaskDefinition{
			{"crm.contact_ready", "Add or import a contact so customer conversations have useful sales context.", "", "Core", "crm_contacts", "Add contacts", func(e SetupEvidence) bool { return e.CRMContactCount > 0 }},
			{"crm.company_ready", "Add or import a company so contacts and opportunities can be grouped by account.", "", "Core", "crm_companies", "Add companies", func(e SetupEvidence) bool { return e.CRMCompanyCount > 0 }},
			{"crm.pipeline_ready", "Configure pipeline stages so every opportunity follows a consistent sales process.", "", "Core", "crm_pipelines", "Configure pipeline", func(e SetupEvidence) bool { return e.CRMPipelineCount > 0 }},
			{"crm.deal_ready", "Create a deal with an owner, value, and close date so the opportunity is actionable.", "", "Core", "crm_deals", "Create deal", func(e SetupEvidence) bool { return e.CRMActionableDealCount > 0 }},
			{"crm.email_connected", "Connect your sales inbox so Helpin can capture customer conversations and buying signals.", "", "Power", "crm_email", "Connect inbox", func(e SetupEvidence) bool { return e.CRMConnectedEmailCount > 0 }},
			{"crm.autonomy_enabled", "Enable CRM automation so Helpin can create or progress deals from strong customer signals.", "", "Power", "crm_autonomy", "Enable automation", func(e SetupEvidence) bool { return e.CRMAutonomyEnabledCount > 0 }},
			{"crm.signal_value_proven", "Create or progress a deal from a detected customer signal so the pipeline updates itself.", "", "Power", "crm_review", "Review CRM activity", func(e SetupEvidence) bool { return e.CRMSignalValueCount > 0 }},
		},
	},
	model.SetupGoalAutomationMastery: {
		key: model.SetupGoalAutomationMastery, title: "Automate repeatable work", accent: "violet",
		description: "Progress from one assisted result to reliable, triggered execution.",
		activated:   func(e SetupEvidence) bool { return e.CompletedAgentRunCount > 0 },
		established: func(e SetupEvidence) bool { return e.CompletedAgentRunDayCount > 1 && e.TriggeredSuccessRunCount > 0 },
		tasks: []setupTaskDefinition{
			{"automation.first_assisted_value", "Complete an agent run on real work so you can see where Helpin saves time.", "", "Core", "automation_agents", "Run an agent", func(e SetupEvidence) bool { return e.CompletedAgentRunCount > 0 }},
			{"automation.custom_agent_succeeded", "Run a custom agent successfully so it can handle work specific to your team.", "", "Power", "automation_custom_agent", "Build custom agent", func(e SetupEvidence) bool { return e.CustomAgentSuccessCount > 0 }},
			{"automation.flow_enabled", "Turn on an automation flow so repeat work can run automatically.", "", "Core", "automation_flows", "Build automation", func(e SetupEvidence) bool { return e.EnabledAutomationCount > 0 }},
			{"automation.approval_guard_configured", "Require approval for sensitive agent actions so automation stays under human control.", "", "Power", "automation_approval_guard", "Configure approvals", func(e SetupEvidence) bool { return e.ApprovalGuardCount > 0 }},
			{"automation.triggered_value", "Complete a triggered or scheduled automation so value no longer depends on a manual start.", "", "Core", "automation_flows", "Review automation runs", func(e SetupEvidence) bool { return e.TriggeredSuccessRunCount > 0 }},
			{"automation.reliable_unattended_value", "Run the same automation successfully over time so your team can trust it unattended.", "", "Power", "automation_flows", "Review automation runs", func(e SetupEvidence) bool { return e.ReliableAutomationCount > 0 }},
		},
	},
}

var setupPlaceholderCatalog = map[string]model.SetupPlaceholderGoal{}

var setupJourneyOrder = []string{
	model.SetupGoalProductDelivery,
	model.SetupGoalCustomerSupport,
	model.SetupGoalHelpCenterDocs,
	model.SetupGoalInternalDocs,
	model.SetupGoalSalesCRM,
	model.SetupGoalAutomationMastery,
}

type SetupAccess struct {
	Unrestricted bool
	Permissions  map[string]bool
	Modules      map[string]bool
	Entitlements map[string]bool
}

func NormalizeSetupGoals(raw []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(raw))
	for _, key := range raw {
		key = strings.TrimSpace(key)
		if key == model.SetupGoalTeamProjects {
			key = model.SetupGoalProductDelivery
		}
		if key == model.SetupGoalFoundation {
			continue
		}
		if _, ok := setupJourneyCatalog[key]; !ok {
			if _, placeholder := setupPlaceholderCatalog[key]; !placeholder {
				return nil, fmt.Errorf("unknown setup goal %q", key)
			}
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	if len(result) > 3 {
		return nil, fmt.Errorf("choose up to 3 setup goals")
	}
	return result, nil
}

func BuildSetupView(evidence SetupEvidence, goals []string, preference model.MemberSetupPreference) model.SetupView {
	normalized, _ := NormalizeSetupGoals(goals)
	goalRows := make([]model.SetupGoal, 0, len(normalized))
	for position, key := range normalized {
		goalRows = append(goalRows, model.SetupGoal{Key: key, Position: position})
	}
	evidenceByGoal := map[string]SetupEvidence{model.SetupGoalFoundation: evidence}
	for _, key := range normalized {
		evidenceByGoal[key] = evidence
	}
	return BuildSetupViewWithState(evidenceByGoal, goalRows, nil, SetupAccess{Unrestricted: true}, preference)
}

func BuildSetupViewWithState(evidenceByGoal map[string]SetupEvidence, goals []model.SetupGoal, achievements map[string]time.Time, access SetupAccess, preference model.MemberSetupPreference) model.SetupView {
	rawGoals := make([]string, 0, len(goals))
	for _, goal := range goals {
		rawGoals = append(rawGoals, goal.Key)
	}
	normalized, _ := NormalizeSetupGoals(rawGoals)
	keys := []string{model.SetupGoalFoundation}
	for _, key := range normalized {
		if _, active := setupJourneyCatalog[key]; active {
			keys = append(keys, key)
		}
	}
	for _, key := range setupJourneyOrder {
		if !containsSetupGoal(keys, key) {
			keys = append(keys, key)
		}
	}
	view := model.SetupView{Goals: normalized, Preference: preference, Journeys: make([]model.SetupJourney, 0, len(keys))}
	completedTasks := make(map[string]bool)
	for _, key := range normalized {
		if placeholder, ok := setupPlaceholderCatalog[key]; ok {
			view.PlaceholderGoals = append(view.PlaceholderGoals, placeholder)
		}
	}
	for _, key := range keys {
		definition := setupJourneyCatalog[key]
		evidence, hasEvidence := evidenceByGoal[key]
		if !hasEvidence {
			evidence = evidenceByGoal[model.SetupGoalFoundation]
		}
		scope := "active"
		if key == model.SetupGoalFoundation {
			scope = "foundation"
		} else if !containsSetupGoal(normalized, key) {
			scope = "featured"
		}
		journey := model.SetupJourney{Key: key, Title: definition.title, Description: definition.description, Accent: definition.accent, Scope: scope, Tasks: make([]model.SetupTask, 0, len(definition.tasks))}
		stageRequirements := map[string]int{}
		stageRequirementCompletions := map[string]int{}
		stageExpansionCompleted := map[string]bool{}
		visibleCompleted, visiblePower, visiblePowerCompleted := 0, 0, 0
		for _, taskDefinition := range definition.tasks {
			if !setupTaskApplicable(key, taskDefinition.key, normalized) {
				continue
			}
			status := model.SetupTaskAvailable
			currentlyComplete := taskDefinition.completed(evidence)
			_, achieved := achievements[taskDefinition.key]
			if currentlyComplete || achieved {
				status = model.SetupTaskCompleted
				completedTasks[taskDefinition.key] = true
				stageExpansionCompleted[taskDefinition.stage] = true
				if achieved && !currentlyComplete && isSetupReadinessTask(taskDefinition.key) {
					status = model.SetupTaskNeedsAttention
				}
			}
			core := isSetupCoreTask(taskDefinition.key)
			if !core {
				visiblePower++
			}
			if currentlyComplete || achieved {
				visibleCompleted++
				if !core {
					visiblePowerCompleted++
				}
			}
			if core {
				stageRequirements[taskDefinition.stage]++
				if currentlyComplete || achieved {
					stageRequirementCompletions[taskDefinition.stage]++
				}
			}
			if core && (currentlyComplete || achieved) {
				journey.CompletedCount++
			}
			if core {
				journey.TotalCount++
			}
			shared := !isMemberSetupTask(taskDefinition.key)
			task := model.SetupTask{Key: taskDefinition.key, Title: taskDefinition.title, Description: taskDefinition.description, Stage: taskDefinition.stage, Status: status, Shared: shared, Core: core}
			prerequisites := setupPrerequisites(taskDefinition.key)
			prerequisitesMet := true
			for _, prerequisite := range prerequisites {
				if !completedTasks[prerequisite] {
					prerequisitesMet = false
					break
				}
			}
			if taskDefinition.key == "support.ai_agent_activated" && evidence.PublicHelpDocCount == 0 {
				prerequisitesMet = false
			}
			allowed, reason := setupActionAllowed(taskDefinition.key, taskDefinition.actionKey, access)
			if status != model.SetupTaskCompleted && status != model.SetupTaskNeedsAttention && !prerequisitesMet {
				task.Status = model.SetupTaskBlocked
				task.BlockedReason = "Complete the earlier journey step first."
			} else if status != model.SetupTaskCompleted && !allowed {
				if status != model.SetupTaskNeedsAttention {
					task.Status = model.SetupTaskBlocked
				}
				task.BlockedReason = reason
			}
			if prerequisitesMet && allowed && (task.Status == model.SetupTaskAvailable || task.Status == model.SetupTaskNeedsAttention) {
				task.Action = &model.SetupAction{Key: taskDefinition.actionKey, Label: taskDefinition.actionLabel}
				if view.Recommended == nil && scope != "featured" && (key != model.SetupGoalFoundation || core) {
					view.Recommended = &model.SetupRecommendation{JourneyKey: key, TaskKey: task.Key, Title: task.Title, Reason: recommendationReason(key), Action: *task.Action}
				}
			}
			journey.Tasks = append(journey.Tasks, task)
		}
		stageCompleted := map[string]bool{}
		for _, stage := range []string{"Prepare", "Activate", "Repeat"} {
			stageCompleted[stage] = stageRequirements[stage] > 0 && stageRequirementCompletions[stage] == stageRequirements[stage]
		}
		stageCompleted["Connect"] = stageExpansionCompleted["Connect"]
		stageCompleted["Prove value"] = stageExpansionCompleted["Prove value"]
		journey.Maturity = setupMaturityFromProgress(journey.CompletedCount, journey.TotalCount, visibleCompleted, len(journey.Tasks), visiblePowerCompleted, visiblePower)
		if scope != "featured" {
			view.CompletedCount += journey.CompletedCount
			view.TotalCount += journey.TotalCount
		}
		view.Journeys = append(view.Journeys, journey)
	}
	if view.Recommended == nil {
		for _, journey := range view.Journeys {
			if journey.Scope != "featured" {
				continue
			}
			for _, task := range journey.Tasks {
				if task.Action != nil && (task.Status == model.SetupTaskAvailable || task.Status == model.SetupTaskNeedsAttention) {
					view.Recommended = &model.SetupRecommendation{JourneyKey: journey.Key, TaskKey: task.Key, Title: task.Title, Reason: "Your core journey is ready for a useful automation step.", Action: *task.Action}
					return view
				}
			}
		}
	}
	return view
}

func setupTaskApplicable(goalKey, taskKey string, goals []string) bool {
	if goalKey == model.SetupGoalFoundation {
		return foundationTaskApplicable(taskKey, goals)
	}
	return !(goalKey == model.SetupGoalCustomerSupport && taskKey == "support.help_docs_ready" && containsSetupGoal(goals, model.SetupGoalHelpCenterDocs))
}

func foundationTaskApplicable(taskKey string, goals []string) bool {
	switch taskKey {
	case "foundation.team_ready":
		return containsSetupGoal(goals, model.SetupGoalProductDelivery) ||
			containsSetupGoal(goals, model.SetupGoalAutomationMastery)
	case "foundation.member_joined":
		return len(goals) > 0
	default:
		return true
	}
}

func setupPrerequisites(taskKey string) []string {
	switch taskKey {
	case "product.project_planned":
		return []string{"foundation.team_ready"}
	case "product.sprint_planned":
		return []string{"product.project_planned"}
	case "product.work_assigned", "product.sprint_closeout_reviewable":
		return []string{"product.sprint_planned"}
	case "product.release_notes_flow_succeeded":
		return []string{"product.repository_ready"}
	case "support.ai_agent_activated":
		return []string{"support.brand_knowledge_ready"}
	case "support.routing_enabled":
		return []string{"support.team_inbox_created"}
	case "support.coverage_fix_applied":
		return []string{"support.ai_agent_activated"}
	case "help_center.content_ready":
		return []string{"help_center.space_ready"}
	case "help_center.article_published":
		return []string{"help_center.content_ready"}
	case "help_center.site_published":
		return []string{"help_center.article_published"}
	case "help_center.widget_connected":
		return []string{"help_center.article_published", "help_center.site_published"}
	case "internal_docs.content_ready":
		return []string{"internal_docs.space_ready"}
	case "internal_docs.published":
		return []string{"internal_docs.content_ready"}
	case "internal_docs.ownership_ready":
		return []string{"internal_docs.published"}
	case "internal_docs.agent_connected":
		return []string{"internal_docs.published"}
	case "internal_docs.agent_succeeded":
		return []string{"internal_docs.agent_connected"}
	case "crm.deal_ready":
		return []string{"crm.contact_ready", "crm.company_ready", "crm.pipeline_ready"}
	case "crm.autonomy_enabled":
		return []string{"crm.email_connected"}
	case "crm.signal_value_proven":
		return []string{"crm.email_connected", "crm.autonomy_enabled"}
	case "automation.custom_agent_succeeded", "automation.approval_guard_configured":
		return []string{"automation.first_assisted_value"}
	case "automation.triggered_value":
		return []string{"automation.flow_enabled"}
	case "automation.reliable_unattended_value":
		return []string{"automation.triggered_value"}
	default:
		return nil
	}
}

func isMemberSetupTask(key string) bool {
	return false
}

func containsSetupGoal(goals []string, key string) bool {
	for _, goal := range goals {
		if goal == key {
			return true
		}
	}
	return false
}

func setupTaskAction(taskKey string) (string, string, bool) {
	for goalKey, journey := range setupJourneyCatalog {
		for _, task := range journey.tasks {
			if task.key == taskKey {
				return goalKey, task.actionKey, true
			}
		}
	}
	return "", "", false
}

func verifiedSetupTaskAchievements(goalKey string, evidence SetupEvidence, verifiedAt time.Time) map[string]time.Time {
	definition, ok := setupJourneyCatalog[goalKey]
	if !ok {
		return nil
	}
	result := make(map[string]time.Time)
	for _, task := range definition.tasks {
		if isMemberSetupTask(task.key) || !task.completed(evidence) {
			continue
		}
		achievedAt := evidence.TaskAchievementTimes[task.key]
		if achievedAt.IsZero() && isSetupReadinessTask(task.key) {
			achievedAt = verifiedAt
		}
		if !achievedAt.IsZero() {
			result[task.key] = achievedAt
		}
	}
	return result
}

func verifiedMemberSetupTaskAchievements(goalKey string, evidence SetupEvidence) map[string]time.Time {
	definition, ok := setupJourneyCatalog[goalKey]
	if !ok {
		return nil
	}
	result := make(map[string]time.Time)
	for _, task := range definition.tasks {
		if !isMemberSetupTask(task.key) || !task.completed(evidence) {
			continue
		}
		if achievedAt := evidence.TaskAchievementTimes[task.key]; !achievedAt.IsZero() {
			result[task.key] = achievedAt
		}
	}
	return result
}

func setupActionAllowed(taskKey, actionKey string, access SetupAccess) (bool, string) {
	if access.Unrestricted {
		return true, ""
	}
	requirePerm := func(permission string) (bool, string) {
		if access.Permissions[permission] {
			return true, ""
		}
		return false, "Ask a workspace admin to complete this step."
	}
	switch actionKey {
	case "workspace_context":
		return requirePerm("workspace.update")
	case "workspace_teams":
		return requirePerm("team.manage")
	case "workspace_members":
		if taskKey == "foundation.member_joined" {
			return requirePerm("workspace.members.read")
		}
		return requirePerm("workspace.invites.manage")
	case "pm_epics", "pm_sprints", "pm_tasks", "product_agent":
		return requirePerm("pm.edit")
	case "git_settings":
		return requirePerm("integrations.connect")
	case "support_email_inbox", "support_live_chat", "support_brand_knowledge", "support_ai", "support_team_inboxes", "support_routing":
		if !access.Modules["support"] {
			return false, "Support is not available to you in this workspace."
		}
		return requirePerm("support.admin")
	case "support_help_docs":
		if !access.Modules["docs"] {
			return false, "Docs is not available to you in this workspace."
		}
		return requirePerm("docs.edit")
	case "support_inbox":
		if !access.Modules["support"] {
			return false, "Support is not available to you in this workspace."
		}
		return requirePerm("support.edit")
	case "support_coverage":
		if !access.Modules["support"] {
			return false, "Support is not available to you in this workspace."
		}
		if !access.Permissions["support.edit"] {
			return false, "Ask a workspace admin to complete this step."
		}
		return requirePerm("docs.edit")
	case "docs_home", "help_center_article_publish", "help_center_settings", "help_center_widget", "internal_docs_agent_knowledge", "internal_docs_agent_run":
		if !access.Modules["docs"] {
			return false, "Docs is not available to you in this workspace."
		}
		return requirePerm("docs.edit")
	case "crm_contacts", "crm_companies", "crm_pipelines", "crm_deals", "crm_email", "crm_review":
		if !access.Modules["crm"] {
			return false, "CRM is not available to you in this workspace."
		}
		return requirePerm("crm.edit")
	case "crm_autonomy":
		if !access.Modules["crm"] {
			return false, "CRM is not available to you in this workspace."
		}
		if !access.Entitlements[string(EntitlementFeatureDealAutomation)] {
			return false, "CRM automation requires the Growth plan."
		}
		return requirePerm("crm.admin")
	case "automation_agents", "automation_custom_agent", "automation_approval_guard", "automation_flows":
		if !access.Modules["automation"] {
			return false, "Automation is not available to you in this workspace."
		}
		if (actionKey == "automation_agents" || actionKey == "automation_custom_agent") && !access.Permissions["pm.edit"] && !access.Permissions["support.edit"] {
			return false, "You need edit access to run an agent on workspace work."
		}
		if actionKey == "automation_flows" && !access.Entitlements[string(EntitlementFeatureAutomationFlows)] {
			return false, "Automation flows require the Growth plan."
		}
		if actionKey == "automation_flows" && !access.Permissions["pm.admin.automations"] {
			return false, "Ask a workspace admin to configure this automation."
		}
		if actionKey == "automation_custom_agent" && !access.Entitlements[string(EntitlementFeatureCustomAgents)] {
			return false, "Custom agents require the Growth plan."
		}
		if taskKey == "automation.triggered_value" || taskKey == "automation.reliable_unattended_value" {
			if !access.Entitlements[string(EntitlementFeatureAgentScheduling)] {
				return false, "Scheduled automation requires the Growth plan."
			}
		}
		return true, ""
	}
	return false, "This action is not available."
}

func setupMaturityFromProgress(coreDone, coreTotal, allDone, allTotal, powerDone, powerTotal int) string {
	if coreDone == 0 {
		return model.SetupMaturityPreparing
	}
	if coreDone < coreTotal {
		return model.SetupMaturityReady
	}
	if powerTotal == 0 || allDone == allTotal {
		return model.SetupMaturityAdvanced
	}
	if powerDone == 0 {
		return model.SetupMaturityActivated
	}
	return model.SetupMaturityEstablished
}

func supportSetupMaturity(e SetupEvidence) string {
	if e.SupportEmailInboxCount == 0 || e.LiveChatInstallationCount == 0 {
		return model.SetupMaturityPreparing
	}
	if e.PublicHelpDocCount == 0 || e.BrandKnowledgeSourceCount == 0 || !e.SupportAIAgentActive {
		return model.SetupMaturityReady
	}
	if e.TeamInboxCount == 0 || e.AutomaticRoutingCount == 0 {
		return model.SetupMaturityActivated
	}
	if e.LinkedSupportTaskCount == 0 || e.CoverageImprovementCount == 0 {
		return model.SetupMaturityEstablished
	}
	return model.SetupMaturityAdvanced
}

func isSetupCoreTask(key string) bool {
	for _, journey := range setupJourneyCatalog {
		for _, task := range journey.tasks {
			if task.key == key {
				return task.stage == "Core"
			}
		}
	}
	return false
}

func isSetupReadinessTask(key string) bool {
	switch key {
	case "foundation.company_context_ready", "foundation.team_ready", "foundation.member_joined",
		"product.project_planned", "product.sprint_planned", "product.work_assigned", "product.repository_ready",
		"support.email_inbox_connected", "support.live_chat_installed", "support.help_docs_ready", "support.brand_knowledge_ready",
		"support.ai_agent_activated", "support.team_inbox_created", "support.routing_enabled",
		"help_center.space_ready", "help_center.content_ready", "help_center.article_published", "help_center.site_published", "help_center.widget_connected",
		"internal_docs.space_ready", "internal_docs.content_ready", "internal_docs.published", "internal_docs.ownership_ready", "internal_docs.agent_connected",
		"crm.contact_ready", "crm.company_ready", "crm.pipeline_ready", "crm.deal_ready", "crm.email_connected", "crm.autonomy_enabled",
		"automation.flow_enabled", "automation.approval_guard_configured":
		return true
	default:
		return false
	}
}

func recommendationReason(key string) string {
	if key == model.SetupGoalFoundation {
		return "This unlocks clearer work across every journey."
	}
	return "This is the shortest path to the next verified outcome."
}
