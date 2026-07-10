package service

import (
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildSetupViewRecommendsWorkspaceContextBeforeModuleWork(t *testing.T) {
	view := BuildSetupView(SetupEvidence{}, []string{model.SetupGoalProductDelivery}, model.MemberSetupPreference{})

	if view.Recommended == nil {
		t.Fatal("expected a recommendation")
	}
	if got, want := view.Recommended.TaskKey, "foundation.company_context_ready"; got != want {
		t.Fatalf("recommended task = %q, want %q", got, want)
	}
	if got, want := len(view.Journeys), 7; got != want {
		t.Fatalf("journey count = %d, want %d", got, want)
	}
	wantOrder := []string{
		model.SetupGoalFoundation,
		model.SetupGoalProductDelivery,
		model.SetupGoalCustomerSupport,
		model.SetupGoalHelpCenterDocs,
		model.SetupGoalInternalDocs,
		model.SetupGoalSalesCRM,
		model.SetupGoalAutomationMastery,
	}
	for index, key := range wantOrder {
		if view.Journeys[index].Key != key {
			t.Fatalf("journey %d = %q, want %q", index, view.Journeys[index].Key, key)
		}
	}
}

func TestSetupTaskLabelsExplainTheActionAndWhy(t *testing.T) {
	count := 0
	for _, journey := range setupJourneyCatalog {
		for _, task := range journey.tasks {
			count++
			if task.title == "" || !strings.Contains(task.title, " so ") {
				t.Errorf("task %q must say what to do and why in one line: %q", task.key, task.title)
			}
		}
	}
	if count != 43 {
		t.Fatalf("catalog has %d tasks, want 43", count)
	}
}

func TestSupportSetupJourneyUsesApprovedHighValueOrder(t *testing.T) {
	journey := setupJourneyCatalog[model.SetupGoalCustomerSupport]
	want := []string{
		"support.email_inbox_connected",
		"support.live_chat_installed",
		"support.help_docs_ready",
		"support.brand_knowledge_ready",
		"support.ai_agent_activated",
		"support.team_inbox_created",
		"support.routing_enabled",
		"support.pm_task_linked",
		"support.coverage_fix_applied",
	}
	if len(journey.tasks) != len(want) {
		t.Fatalf("support task count = %d, want %d", len(journey.tasks), len(want))
	}
	for i, task := range journey.tasks {
		if task.key != want[i] {
			t.Errorf("support task %d = %q, want %q", i, task.key, want[i])
		}
	}
}

func TestSetupCatalogUsesOnlyHighValueJourneyTasks(t *testing.T) {
	want := map[string][]string{
		model.SetupGoalFoundation: {
			"foundation.company_context_ready", "foundation.team_ready", "foundation.member_joined",
		},
		model.SetupGoalProductDelivery: {
			"product.project_planned", "product.sprint_planned", "product.work_assigned", "product.sprint_closeout_reviewable",
			"product.repository_ready", "product.agent_result_used", "product.release_notes_flow_succeeded",
		},
		model.SetupGoalCustomerSupport: {
			"support.email_inbox_connected", "support.live_chat_installed", "support.help_docs_ready", "support.brand_knowledge_ready",
			"support.ai_agent_activated", "support.team_inbox_created", "support.routing_enabled", "support.pm_task_linked", "support.coverage_fix_applied",
		},
		model.SetupGoalHelpCenterDocs: {
			"help_center.space_ready", "help_center.content_ready", "help_center.article_published", "help_center.site_published", "help_center.widget_connected",
		},
		model.SetupGoalInternalDocs: {
			"internal_docs.space_ready", "internal_docs.content_ready", "internal_docs.published", "internal_docs.ownership_ready", "internal_docs.agent_connected", "internal_docs.agent_succeeded",
		},
		model.SetupGoalSalesCRM: {
			"crm.contact_ready", "crm.company_ready", "crm.pipeline_ready", "crm.deal_ready", "crm.email_connected", "crm.autonomy_enabled", "crm.signal_value_proven",
		},
		model.SetupGoalAutomationMastery: {
			"automation.first_assisted_value", "automation.custom_agent_succeeded", "automation.flow_enabled", "automation.approval_guard_configured", "automation.triggered_value", "automation.reliable_unattended_value",
		},
	}
	for goal, keys := range want {
		journey, ok := setupJourneyCatalog[goal]
		if !ok {
			t.Errorf("journey %q missing", goal)
			continue
		}
		if len(journey.tasks) != len(keys) {
			t.Errorf("journey %q has %d tasks, want %d", goal, len(journey.tasks), len(keys))
			continue
		}
		for i, key := range keys {
			if journey.tasks[i].key != key {
				t.Errorf("journey %q task %d = %q, want %q", goal, i, journey.tasks[i].key, key)
			}
		}
	}
}

func TestNormalizeSetupGoalsCanonicalizesTeamProjectsBeforeLimit(t *testing.T) {
	goals, err := NormalizeSetupGoals([]string{model.SetupGoalTeamProjects, model.SetupGoalProductDelivery, model.SetupGoalInternalDocs, model.SetupGoalSalesCRM})
	if err != nil {
		t.Fatalf("normalize alias plus canonical: %v", err)
	}
	want := []string{model.SetupGoalProductDelivery, model.SetupGoalInternalDocs, model.SetupGoalSalesCRM}
	if len(goals) != len(want) {
		t.Fatalf("goals = %v, want %v", goals, want)
	}
	for i := range want {
		if goals[i] != want[i] {
			t.Fatalf("goals = %v, want %v", goals, want)
		}
	}
	if _, err := NormalizeSetupGoals([]string{model.SetupGoalTeamProjects, model.SetupGoalHelpCenterDocs, model.SetupGoalInternalDocs, model.SetupGoalSalesCRM}); err == nil {
		t.Fatal("expected four canonical goals to be rejected")
	}
}

func TestSupportSetupJourneyUsesSevenCoreTasksAndHighValueEvidence(t *testing.T) {
	evidence := completeSupportSetupEvidence()
	view := BuildSetupView(evidence, []string{model.SetupGoalCustomerSupport}, model.MemberSetupPreference{})
	journey := findSetupJourney(t, view, model.SetupGoalCustomerSupport)

	if journey.TotalCount != 7 || journey.CompletedCount != 7 {
		t.Fatalf("support core progress = %d/%d, want 7/7", journey.CompletedCount, journey.TotalCount)
	}
	for _, task := range journey.Tasks {
		if task.Status != model.SetupTaskCompleted {
			t.Errorf("task %q status = %q, want completed", task.Key, task.Status)
		}
	}
	if journey.Maturity != model.SetupMaturityAdvanced {
		t.Fatalf("support maturity = %q, want advanced", journey.Maturity)
	}
}

func TestSupportAndHelpCenterSharePublishedDocsWithoutDoubleCounting(t *testing.T) {
	evidence := completeSupportSetupEvidence()
	evidence.HelpCenterSpaceCount = 1
	evidence.HelpCenterContentCount = 1
	evidence.HelpCenterSiteCount = 1
	view := BuildSetupView(evidence, []string{model.SetupGoalCustomerSupport, model.SetupGoalHelpCenterDocs}, model.MemberSetupPreference{})
	support := findSetupJourney(t, view, model.SetupGoalCustomerSupport)
	if support.TotalCount != 6 {
		t.Fatalf("support core total with help-center journey = %d, want 6", support.TotalCount)
	}
	for _, task := range support.Tasks {
		if task.Key == "support.help_docs_ready" {
			t.Fatal("support help-doc task duplicated while help-center journey selected")
		}
	}
	assertSetupTaskStatus(t, support, "support.ai_agent_activated", model.SetupTaskCompleted)
}

func TestSupportSetupJourneyMaturityRequiresEveryEarlierThreshold(t *testing.T) {
	tests := []struct {
		name     string
		evidence SetupEvidence
		want     string
	}{
		{name: "preparing", evidence: SetupEvidence{}, want: model.SetupMaturityPreparing},
		{name: "ready", evidence: SetupEvidence{SupportEmailInboxCount: 1, LiveChatInstallationCount: 1}, want: model.SetupMaturityReady},
		{name: "activated", evidence: SetupEvidence{SupportEmailInboxCount: 1, LiveChatInstallationCount: 1, PublicHelpDocCount: 1, BrandKnowledgeSourceCount: 1, SupportAIAgentActive: true, TeamInboxCount: 1, AutomaticRoutingCount: 1}, want: model.SetupMaturityActivated},
		{name: "established", evidence: SetupEvidence{SupportEmailInboxCount: 1, LiveChatInstallationCount: 1, PublicHelpDocCount: 1, BrandKnowledgeSourceCount: 1, SupportAIAgentActive: true, TeamInboxCount: 1, AutomaticRoutingCount: 1, LinkedSupportTaskCount: 1}, want: model.SetupMaturityEstablished},
		{name: "advanced", evidence: completeSupportSetupEvidence(), want: model.SetupMaturityAdvanced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := BuildSetupView(tt.evidence, []string{model.SetupGoalCustomerSupport}, model.MemberSetupPreference{})
			journey := findSetupJourney(t, view, model.SetupGoalCustomerSupport)
			if journey.Maturity != tt.want {
				t.Fatalf("maturity = %q, want %q", journey.Maturity, tt.want)
			}
		})
	}
}

func TestSupportSetupJourneyBlocksOnlyRealDependencies(t *testing.T) {
	view := BuildSetupView(SetupEvidence{}, []string{model.SetupGoalCustomerSupport}, model.MemberSetupPreference{})
	journey := findSetupJourney(t, view, model.SetupGoalCustomerSupport)

	assertSetupTaskStatus(t, journey, "support.email_inbox_connected", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.live_chat_installed", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.help_docs_ready", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.brand_knowledge_ready", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.ai_agent_activated", model.SetupTaskBlocked)
	assertSetupTaskStatus(t, journey, "support.team_inbox_created", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.routing_enabled", model.SetupTaskBlocked)
	assertSetupTaskStatus(t, journey, "support.pm_task_linked", model.SetupTaskAvailable)
	assertSetupTaskStatus(t, journey, "support.coverage_fix_applied", model.SetupTaskBlocked)
}

func TestSupportSetupActionsUseSupportAndDocsPermissions(t *testing.T) {
	access := SetupAccess{
		Modules:     map[string]bool{"support": true, "docs": true},
		Permissions: map[string]bool{"support.admin": true, "support.edit": true, "docs.edit": true},
	}
	tests := []struct {
		taskKey   string
		actionKey string
	}{
		{"support.email_inbox_connected", "support_email_inbox"},
		{"support.live_chat_installed", "support_live_chat"},
		{"support.help_docs_ready", "support_help_docs"},
		{"support.brand_knowledge_ready", "support_brand_knowledge"},
		{"support.ai_agent_activated", "support_ai"},
		{"support.team_inbox_created", "support_team_inboxes"},
		{"support.routing_enabled", "support_routing"},
		{"support.pm_task_linked", "support_inbox"},
		{"support.coverage_fix_applied", "support_coverage"},
	}
	for _, tt := range tests {
		allowed, reason := setupActionAllowed(tt.taskKey, tt.actionKey, access)
		if !allowed {
			t.Errorf("action %q blocked: %s", tt.actionKey, reason)
		}
	}

	access.Permissions["docs.edit"] = false
	if allowed, _ := setupActionAllowed("support.help_docs_ready", "support_help_docs", access); allowed {
		t.Fatal("public help-doc action allowed without docs.edit")
	}
}

func completeSupportSetupEvidence() SetupEvidence {
	return SetupEvidence{
		SupportEmailInboxCount:    1,
		LiveChatInstallationCount: 1,
		PublicHelpDocCount:        1,
		BrandKnowledgeSourceCount: 1,
		SupportAIAgentActive:      true,
		TeamInboxCount:            1,
		AutomaticRoutingCount:     1,
		LinkedSupportTaskCount:    1,
		CoverageImprovementCount:  1,
	}
}

func TestBuildSetupViewPreservesAchievementAndFlagsReadinessLoss(t *testing.T) {
	goals := []model.SetupGoal{{Key: model.SetupGoalProductDelivery, ActivatedAt: time.Now()}}
	view := BuildSetupViewWithState(
		map[string]SetupEvidence{model.SetupGoalFoundation: {}, model.SetupGoalProductDelivery: {}},
		goals,
		map[string]time.Time{"foundation.company_context_ready": time.Now()},
		SetupAccess{Unrestricted: true},
		model.MemberSetupPreference{},
	)
	foundation := findSetupJourney(t, view, model.SetupGoalFoundation)
	assertSetupTaskStatus(t, foundation, "foundation.company_context_ready", model.SetupTaskNeedsAttention)
	if foundation.CompletedCount != 1 {
		t.Fatalf("durable completed count = %d, want 1", foundation.CompletedCount)
	}
}

func TestBuildSetupViewBlocksInaccessibleRecommendations(t *testing.T) {
	goals := []model.SetupGoal{{Key: model.SetupGoalProductDelivery}}
	view := BuildSetupViewWithState(
		map[string]SetupEvidence{model.SetupGoalFoundation: {}, model.SetupGoalProductDelivery: {}},
		goals,
		nil,
		SetupAccess{Permissions: map[string]bool{}, Modules: map[string]bool{}, Entitlements: map[string]bool{}},
		model.MemberSetupPreference{},
	)
	if view.Recommended != nil {
		t.Fatalf("recommendation = %+v, want nil for inaccessible actions", view.Recommended)
	}
	foundation := findSetupJourney(t, view, model.SetupGoalFoundation)
	assertSetupTaskStatus(t, foundation, "foundation.company_context_ready", model.SetupTaskBlocked)
}

func TestBuildSetupViewBlocksTasksUntilPrerequisitesComplete(t *testing.T) {
	view := BuildSetupView(SetupEvidence{}, []string{model.SetupGoalProductDelivery}, model.MemberSetupPreference{})
	product := findSetupJourney(t, view, model.SetupGoalProductDelivery)

	assertSetupTaskStatus(t, product, "product.sprint_planned", model.SetupTaskBlocked)
	assertSetupTaskStatus(t, product, "product.work_assigned", model.SetupTaskBlocked)
	if task := findSetupTask(t, product, "product.sprint_planned"); task.Action != nil {
		t.Fatalf("blocked prerequisite task has action: %+v", task.Action)
	}
}

func TestBuildSetupViewMakesFoundationApplicableToSelectedGoals(t *testing.T) {
	view := BuildSetupView(SetupEvidence{HasCompanyContext: true}, []string{model.SetupGoalCustomerSupport}, model.MemberSetupPreference{})
	foundation := findSetupJourney(t, view, model.SetupGoalFoundation)

	for _, task := range foundation.Tasks {
		if task.Key == "foundation.team_ready" {
			t.Fatal("support-only setup should not include PM team setup")
		}
	}
	if foundation.TotalCount != 2 {
		t.Fatalf("support-only foundation total = %d, want company context and teammate", foundation.TotalCount)
	}
	if view.Recommended == nil || view.Recommended.TaskKey != "foundation.member_joined" {
		t.Fatalf("support-only recommendation = %+v, want teammate after company context", view.Recommended)
	}
}

func TestBuildSetupViewRequiresEveryCoreSupportThresholdForMaturity(t *testing.T) {
	evidence := SetupEvidence{SupportEmailInboxCount: 1, LiveChatInstallationCount: 1, PublicHelpDocCount: 1}
	view := BuildSetupView(evidence, []string{model.SetupGoalCustomerSupport}, model.MemberSetupPreference{})
	support := findSetupJourney(t, view, model.SetupGoalCustomerSupport)

	if support.Maturity != model.SetupMaturityReady {
		t.Fatalf("maturity = %q, want ready while brand knowledge and AI activation are incomplete", support.Maturity)
	}
}

func TestNormalizeSetupGoalsPreservesPlaceholderIntent(t *testing.T) {
	goals, err := NormalizeSetupGoals([]string{model.SetupGoalInternalDocs, model.SetupGoalSalesCRM})
	if err != nil {
		t.Fatalf("normalize placeholders: %v", err)
	}
	if len(goals) != 2 || goals[0] != model.SetupGoalInternalDocs || goals[1] != model.SetupGoalSalesCRM {
		t.Fatalf("placeholder goals = %v", goals)
	}
}

func TestInferSetupGoalsUsesActualConfigurationAndKeepsEmptyWorkspaceNeutral(t *testing.T) {
	if goals := inferSetupGoals(SetupEvidence{}); len(goals) != 0 {
		t.Fatalf("empty workspace goals = %v, want none", goals)
	}
	if goals := inferSetupGoals(SetupEvidence{PlannedProjectCount: 1}); len(goals) != 1 || goals[0] != model.SetupGoalProductDelivery {
		t.Fatalf("configured PM workspace goals = %v, want product delivery", goals)
	}
	goals := inferSetupGoals(SetupEvidence{PlannedProjectCount: 1, SupportEmailInboxCount: 1, EnabledAutomationCount: 1})
	if len(goals) != 3 || goals[0] != model.SetupGoalProductDelivery || goals[1] != model.SetupGoalCustomerSupport || goals[2] != model.SetupGoalAutomationMastery {
		t.Fatalf("strong evidence goals = %v", goals)
	}
}

func TestBuildSetupViewUsesCoreDenominatorAndAdvancedMilestones(t *testing.T) {
	evidence := SetupEvidence{
		HasCompanyContext: true, TeamCount: 1,
		PlannedProjectCount: 1, PlannedSprintCount: 1, AssignedProjectTaskCount: 1,
		ConnectedRepositoryCount: 1, SprintCloseoutCount: 1, ProductAgentRunCount: 1, ReleaseNotesSuccessCount: 1,
	}
	view := BuildSetupView(evidence, []string{model.SetupGoalProductDelivery}, model.MemberSetupPreference{})
	journey := findSetupJourney(t, view, model.SetupGoalProductDelivery)
	if journey.Maturity != model.SetupMaturityAdvanced {
		t.Fatalf("maturity = %q, want advanced", journey.Maturity)
	}
	if journey.TotalCount != 4 || journey.CompletedCount != 4 {
		t.Fatalf("product core progress = %d/%d, want 4/4", journey.CompletedCount, journey.TotalCount)
	}
	if view.TotalCount != 7 {
		t.Fatalf("global core total = %d, want foundation 3 + product 4", view.TotalCount)
	}
}

func TestBuildSetupViewUsesVerifiedAutomationOutcomes(t *testing.T) {
	evidence := SetupEvidence{
		HasCompanyContext:         true,
		TeamCount:                 1,
		ActiveMemberCount:         2,
		InitialWorkCount:          3,
		CompletedTaskCount:        2,
		CompletedTaskDayCount:     2,
		CompletedAgentRunCount:    2,
		CompletedAgentRunDayCount: 2,
		EnabledAutomationCount:    1,
		TriggeredSuccessRunCount:  1,
		CustomAgentSuccessCount:   1,
		ApprovalGuardCount:        1,
		ReliableAutomationCount:   1,
	}
	view := BuildSetupView(evidence, []string{model.SetupGoalAutomationMastery}, model.MemberSetupPreference{})

	journey := findSetupJourney(t, view, model.SetupGoalAutomationMastery)
	assertSetupTaskStatus(t, journey, "automation.first_assisted_value", model.SetupTaskCompleted)
	assertSetupTaskStatus(t, journey, "automation.flow_enabled", model.SetupTaskCompleted)
	assertSetupTaskStatus(t, journey, "automation.triggered_value", model.SetupTaskCompleted)
	if journey.Maturity != model.SetupMaturityAdvanced {
		t.Fatalf("maturity = %q, want %q", journey.Maturity, model.SetupMaturityAdvanced)
	}
}

func TestNormalizeSetupGoalsRejectsUnknownAndCapsSelection(t *testing.T) {
	if _, err := NormalizeSetupGoals([]string{
		model.SetupGoalProductDelivery,
		model.SetupGoalCustomerSupport,
		model.SetupGoalAutomationMastery,
		"sales_pipeline",
	}); err == nil {
		t.Fatal("expected more than three goals to be rejected")
	}

	if _, err := NormalizeSetupGoals([]string{"not-real"}); err == nil {
		t.Fatal("expected unknown goal to be rejected")
	}

	goals, err := NormalizeSetupGoals([]string{model.SetupGoalProductDelivery, model.SetupGoalProductDelivery})
	if err != nil {
		t.Fatalf("normalize goals: %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("deduplicated goals = %v", goals)
	}
}

func findSetupJourney(t *testing.T, view model.SetupView, key string) model.SetupJourney {
	t.Helper()
	for _, journey := range view.Journeys {
		if journey.Key == key {
			return journey
		}
	}
	t.Fatalf("journey %q not found", key)
	return model.SetupJourney{}
}

func assertSetupTaskStatus(t *testing.T, journey model.SetupJourney, key, want string) {
	t.Helper()
	for _, task := range journey.Tasks {
		if task.Key == key {
			if task.Status != want {
				t.Fatalf("task %q status = %q, want %q", key, task.Status, want)
			}
			return
		}
	}
	t.Fatalf("task %q not found", key)
}

func findSetupTask(t *testing.T, journey model.SetupJourney, key string) model.SetupTask {
	t.Helper()
	for _, task := range journey.Tasks {
		if task.Key == key {
			return task
		}
	}
	t.Fatalf("task %q not found", key)
	return model.SetupTask{}
}
