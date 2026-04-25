package automationcatalog

import "github.com/helpin-ai/helpin/server/internal/model"

var entries = []model.AutomationCatalogEntry{
	builtIn(
		"crm.buyer_signal_ingestion",
		"crm",
		"CRM system intelligence",
		"Buyer signal ingestion",
		"Extracts buyer signals from stored CRM email and keeps CRM intelligence current in the background.",
		[]string{"crm_email_message"},
		[]string{"internal_domain_hook"},
		"system",
		"background_workflow",
		false,
		"automation-default",
		"CRM Settings > Email Accounts",
		"/w/$slug/settings/crm-email",
		"Background only",
		nil,
		"CRM contacts, deals, signals, and insights",
		"Automation > Activity",
	),
	builtIn(
		"crm.contact_summary_refresh",
		"crm",
		"CRM system intelligence",
		"Contact summary refresh",
		"Generates durable contact summaries from recent CRM email activity and buyer signals.",
		[]string{"crm_contact"},
		[]string{"internal_domain_hook", "cron"},
		"system",
		"background_workflow",
		false,
		"automation-default",
		"CRM Settings > Email Accounts",
		"/w/$slug/settings/crm-email",
		"Background only",
		nil,
		"CRM contact detail pages",
		"Automation > Activity",
	),
	builtIn(
		"crm.deal_summary_refresh",
		"crm",
		"CRM system intelligence",
		"Deal summary refresh",
		"Generates durable deal summaries from recent CRM email activity and buyer signals.",
		[]string{"crm_deal"},
		[]string{"internal_domain_hook", "cron"},
		"system",
		"background_workflow",
		false,
		"automation-default",
		"CRM Settings > Email Accounts",
		"/w/$slug/settings/crm-email",
		"Background only",
		nil,
		"CRM deal detail pages",
		"Automation > Activity",
	),
	builtIn(
		"pm.epic_auto_start",
		"pm",
		"PM built-in rules",
		"Epic auto-start",
		"Starts an epic automatically when one of its tasks enters a started workflow state.",
		[]string{"pm_epic"},
		[]string{"internal_domain_hook"},
		"workspace",
		"deterministic_rule",
		true,
		"inline",
		"Project Settings > Automations",
		"/w/$slug/settings/automations",
		"Task state change",
		nil,
		"PM epics and activity feed",
		"Automation > Activity",
	),
	builtIn(
		"pm.epic_auto_complete",
		"pm",
		"PM built-in rules",
		"Epic auto-complete",
		"Completes an epic automatically when all of its tasks are done.",
		[]string{"pm_epic"},
		[]string{"internal_domain_hook"},
		"workspace",
		"deterministic_rule",
		true,
		"inline",
		"Project Settings > Automations",
		"/w/$slug/settings/automations",
		"Task state change",
		nil,
		"PM epics and activity feed",
		"Automation > Activity",
	),
	builtIn(
		"pm.sprint_auto_create",
		"pm",
		"PM built-in rules",
		"Sprint auto-create",
		"Keeps a buffer of future sprints available for each configured team.",
		[]string{"pm_sprint"},
		[]string{"cron"},
		"team",
		"scheduled_rule",
		true,
		"inline",
		"Project Settings > Automations",
		"/w/$slug/settings/automations",
		"Hourly PM automation sweep",
		nil,
		"PM sprint lists and planning views",
		"Automation > Activity",
	),
	builtIn(
		"pm.sprint_move_unfinished",
		"pm",
		"PM built-in rules",
		"Move unfinished tasks",
		"Moves unfinished sprint work into the next sprint for configured teams.",
		[]string{"pm_sprint", "pm_story"},
		[]string{"cron"},
		"team",
		"scheduled_rule",
		true,
		"inline",
		"Project Settings > Automations",
		"/w/$slug/settings/automations",
		"Hourly PM automation sweep",
		nil,
		"PM sprints and tasks",
		"Automation > Activity",
	),
	{
		ID:                  "automation_rule",
		Kind:                model.AutomationKindRule,
		Module:              "pm",
		Group:               "Automation rules",
		Title:               "Automation Rules",
		Description:         "User-configured rules triggered by workflow events and GitHub webhooks. Powers stage-based agent pipelines.",
		TargetTypes:         []string{"pm_story"},
		TriggerModes:        []string{"task.state_entered", "agent_run.approved", "github.push", "github.pull_request_opened", "github.pull_request_merged", "github.pull_request_closed", "github.pull_request_review_requested", "github.release_published", "github.check_suite_completed", "cron"},
		ConfigScope:         "workspace",
		ExecutionStyle:      "event_driven_rule",
		UserGoverned:        true,
		UserCreatable:       true,
		Queue:               "inline",
		CurrentWriteSurface: "Automation > Flows",
		CurrentWritePath:    catalogPathPtr("/w/$slug/automation/flows"),
		CurrentRunSurface:   "Task state changes, agent run approvals, and GitHub webhooks",
		OutputSurface:       "PM tasks, agent runs, and activity feed",
		DiagnosticsSurface:  "Automation > Activity",
	},
}

func catalogPathPtr(value string) *string {
	return &value
}

func builtIn(
	id, module, group, title, description string,
	targetTypes, triggerModes []string,
	configScope, executionStyle string,
	userGoverned bool,
	queue, writeSurface, writePath, runSurface string,
	runPath *string,
	outputSurface, diagnosticsSurface string,
) model.AutomationCatalogEntry {
	entry := model.AutomationCatalogEntry{
		ID:                  id,
		Kind:                model.AutomationKindBuiltIn,
		Module:              module,
		Group:               group,
		Title:               title,
		Description:         description,
		TargetTypes:         targetTypes,
		TriggerModes:        triggerModes,
		ConfigScope:         configScope,
		ExecutionStyle:      executionStyle,
		UserGoverned:        userGoverned,
		UserCreatable:       false,
		Queue:               queue,
		CurrentWriteSurface: writeSurface,
		CurrentRunSurface:   runSurface,
		CurrentRunPath:      runPath,
		OutputSurface:       outputSurface,
		DiagnosticsSurface:  diagnosticsSurface,
	}
	if writePath != "" {
		entry.CurrentWritePath = &writePath
	}
	return entry
}

// All returns the canonical code-defined automation catalog.
func All() []model.AutomationCatalogEntry {
	cloned := make([]model.AutomationCatalogEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

// ByID returns one catalog entry by canonical ID.
func ByID(id string) (model.AutomationCatalogEntry, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return model.AutomationCatalogEntry{}, false
}
