package commandtools

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type RuntimeToolMetadata struct {
	CommandName string
	Alias       string
	Category    string
	Description string
	InputSchema map[string]any
	RiskLevel   string
}

const (
	RiskLevelRead        = "read"
	RiskLevelRoutine     = "routine_mutation"
	RiskLevelSensitive   = "sensitive_mutation"
	RiskLevelDestructive = "destructive_mutation"
)

func ToolMetadataForAlias(alias string) (*RuntimeToolMetadata, bool) {
	alias = strings.TrimSpace(alias)
	for _, meta := range sharedRuntimeTools {
		if meta.Alias == alias {
			copied := meta
			copied.RiskLevel = runtimeToolRiskLevel(copied.Alias)
			return &copied, true
		}
	}
	return nil, false
}

func ToolMetadataForCommand(name string) (*RuntimeToolMetadata, bool) {
	name = strings.TrimSpace(name)
	for _, meta := range sharedRuntimeTools {
		if meta.CommandName == name {
			copied := meta
			copied.RiskLevel = runtimeToolRiskLevel(copied.Alias)
			return &copied, true
		}
	}
	return nil, false
}

func AllRuntimeToolMetadata() []RuntimeToolMetadata {
	all := make([]RuntimeToolMetadata, len(sharedRuntimeTools))
	for index, meta := range sharedRuntimeTools {
		meta.RiskLevel = runtimeToolRiskLevel(meta.Alias)
		all[index] = meta
	}
	return all
}

// runtimeToolRiskLevel classifies command-backed mutations independently of
// any agent preset. An empty value is a safe fallback: callers treat an
// unclassified mutation as sensitive, never as routine.
func runtimeToolRiskLevel(alias string) string {
	if level, ok := runtimeToolRiskLevels[strings.TrimSpace(alias)]; ok {
		return level
	}
	return ""
}

var runtimeToolRiskLevels = map[string]string{
	"create_space": RiskLevelRoutine, "create_collection": RiskLevelRoutine,
	"create_document": RiskLevelRoutine, "update_space": RiskLevelRoutine,
	"update_collection": RiskLevelRoutine, "move_document": RiskLevelRoutine,
	"write_document_content": RiskLevelRoutine, "update_document_block": RiskLevelRoutine,
	"insert_document_block": RiskLevelRoutine, "insert_document_artifact": RiskLevelRoutine,
	"insert_document_image":   RiskLevelRoutine,
	"link_document_to_object": RiskLevelRoutine, "ensure_epic_spec_doc": RiskLevelRoutine,
	"ensure_task_plan_doc": RiskLevelRoutine, "publish_document_change_proposal": RiskLevelRoutine,
	"publish_ai_section_candidate": RiskLevelRoutine,
	"create_task":                  RiskLevelRoutine, "create_task_batch": RiskLevelRoutine,
	"create_task_checklist_item": RiskLevelRoutine, "update_task_checklist_item": RiskLevelRoutine,
	"update_task": RiskLevelRoutine, "update_task_state": RiskLevelRoutine,
	"update_story_state": RiskLevelRoutine, "set_task_dependencies": RiskLevelRoutine,
	"ensure_task_label": RiskLevelRoutine, "add_task_comment": RiskLevelRoutine,
	"create_epic": RiskLevelRoutine, "update_epic": RiskLevelRoutine,
	"create_sprint": RiskLevelRoutine, "update_sprint": RiskLevelRoutine,
	"create_objective": RiskLevelRoutine, "update_objective": RiskLevelRoutine,
	"add_deal_note": RiskLevelRoutine, "update_deal_stage": RiskLevelRoutine,
	"ensure_crm_contact_company": RiskLevelRoutine, "enrich_crm_contact": RiskLevelRoutine,
	"enrich_crm_company": RiskLevelRoutine, "draft_support_reply": RiskLevelRoutine,
	"update_conversation_status":    RiskLevelRoutine,
	"complete_support_coverage_gap": RiskLevelRoutine,
	"send_support_reply":            RiskLevelSensitive, "escalate_to_human": RiskLevelSensitive,
	"run_epic_delivery_pipeline": RiskLevelDestructive,
}

var sharedRuntimeTools = []RuntimeToolMetadata{
	{
		CommandName: "workspace.search",
		Alias:       "search_workspace",
		Category:    "Workspace",
		Description: "Search accessible workspace entities by keyword or identity. Every displayed result must use its returned markdown_link verbatim. Task searches match task keys, names, and descriptions. Use this for requests asking which entities mention, contain, discuss, or relate to a term; use list tools only for enumeration or structured filtering.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 500, "description": "Keyword, UUID, task key, name, email, domain, or support subject to find. Task keywords are matched against names and descriptions."},
				"entity_types": map[string]any{
					"type": "array", "maxItems": 10, "uniqueItems": true,
					"description": "Optional entity types to search. Omit to search every accessible type.",
					"items":       map[string]any{"type": "string", "enum": []string{"task", "epic", "sprint", "objective", "document", "workspace_member", "crm_contact", "crm_company", "crm_deal", "support_conversation"}},
				},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Maximum results to return. Defaults to 10, max 50."},
				"offset": map[string]any{"type": "integer", "minimum": 0, "maximum": 500, "description": "Zero-based result offset. Use next_offset from the previous response."},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "workspace.list_teams",
		Alias:       "list_workspace_teams",
		Category:    "Workspace",
		Description: "List workspace teams that the agent can use for team selection, task filtering, or planning context.",
		InputSchema: boundedListSchema(nil),
	},
	{
		CommandName: "docs.list_documents",
		Alias:       "list_documents",
		Category:    "Docs",
		Description: "List Helpin Docs documents in the current workspace. Every displayed document must use its returned markdown_link verbatim. Use status=draft for questions about documents that need to be published.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space_id":         map[string]any{"type": "string"},
				"collection_id":    map[string]any{"type": "string"},
				"team_id":          map[string]any{"type": "string"},
				"status":           map[string]any{"type": "string", "enum": []string{"draft", "published", "archived"}},
				"include_archived": map[string]any{"type": "boolean"},
				"limit":            map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum documents to return. Defaults to 50, max 100."},
				"offset":           map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based result offset. Use next_offset from the previous response."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.search_documents",
		Alias:       "search_documents",
		Category:    "Docs",
		Description: "Search documents by keyword across the workspace. Every displayed document must use its returned markdown_link verbatim. Use only when you need to find other documents or the current document ID is unknown; do not use it to inspect a known current document.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "minLength": 1, "description": "Search query"},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "description": "Maximum results to return. Defaults to 10, max 20."},
				"offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based result offset. Use next_offset from the previous response."},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "crm.list_deals",
		Alias:       "list_deals",
		Category:    "CRM",
		Description: "List CRM deals in the workspace, optionally filtered by a case-insensitive name query. Returns deal name, stage, and amount.",
		InputSchema: paginatedQuerySchema(50, 20, "Maximum number of deals to return. Defaults to 20, max 50."),
	},
	{
		CommandName: "crm.list_contacts",
		Alias:       "list_contacts",
		Category:    "CRM",
		Description: "List CRM contacts in the workspace, optionally filtered by a case-insensitive name, email, or job-title query. Returns name, email, and job title.",
		InputSchema: paginatedQuerySchema(50, 20, "Maximum number of contacts to return. Defaults to 20, max 50."),
	},
	{
		CommandName: "crm.list_buyer_signals",
		Alias:       "list_buyer_signals",
		Category:    "CRM",
		Description: "List detected buyer signals from emails, meetings, and support conversations.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deal_id": map[string]any{"type": "string", "description": "Optional deal ID to filter signals for a specific deal."},
				"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Maximum number of signals to return. Defaults to 20, max 50."},
				"offset":  map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based result offset. Use next_offset from the previous response."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "support.list_conversation_messages",
		Alias:       "list_conversation_messages",
		Category:    "Support",
		Description: "List support conversation messages newest-first by page, returned in chronological reading order with attachment metadata.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"conversation_id": map[string]any{"type": "string", "description": "Optional conversation ID. Defaults to the current conversation target."},
				"limit":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum messages to return. Defaults to the newest 20, max 100."},
				"offset":          map[string]any{"type": "integer", "minimum": 0, "description": "Number of newer messages to skip. Use next_offset to load the previous page."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "release.get_task_context",
		Alias:       "get_task_context",
		Category:    "Release",
		Description: "Load compact task context with optional linked docs, document content, comments, and git links for specific task IDs or human task keys.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_ids":                 map[string]any{"type": "array", "description": "Task IDs to load. Combined maximum with task_keys is 50.", "items": map[string]any{"type": "string"}, "maxItems": 50},
				"task_keys":                map[string]any{"type": "array", "description": "Human task keys to load, for example USE-488. Combined maximum with task_ids is 50.", "items": map[string]any{"type": "string"}, "maxItems": 50},
				"include_linked_docs":      map[string]any{"type": "boolean", "description": "Whether to include linked document metadata."},
				"include_document_content": map[string]any{"type": "boolean", "description": "Whether to include linked document content text. Only used when include_linked_docs is true."},
				"include_comments":         map[string]any{"type": "boolean", "description": "Whether to include task comments."},
				"include_git_links":        map[string]any{"type": "boolean", "description": "Whether to include git links for each task."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "workspace.list_members",
		Alias:       "list_workspace_members",
		Category:    "Workspace",
		Description: "List active assignable workspace members with stable member and user IDs, display names, and team IDs. Email addresses are not returned.",
		InputSchema: boundedListSchema(nil),
	},
	{
		CommandName: "docs.ensure_spec_doc",
		Alias:       "ensure_epic_spec_doc",
		Category:    "Docs",
		Description: "Create or load the canonical product spec document for the current epic and attach it to that epic. Returns the document_id and title.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.ensure_task_plan_doc",
		Alias:       "ensure_task_plan_doc",
		Category:    "Docs",
		Description: "Create or load the canonical planning document for the current task and attach it to that task. Returns the document_id and title.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "pm.approve_epic_spec",
		Alias:       "approve_epic_spec",
		Category:    "PM / Tasks",
		Description: "Mark the current epic spec document as approved and record the approved spec version on the epic.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"version_id": map[string]any{
					"type":        "string",
					"description": "Optional existing document version ID to approve. Omit to approve the current document content.",
				},
			},
			"required": []string{},
		},
	},
	{
		CommandName: "pm.create_task_batch",
		Alias:       "create_task_batch",
		Category:    "PM / Tasks",
		Description: "Create implementation-ready tasks for the current epic. Supports stable refs, direct assignment, and dependency refs.",
		InputSchema: createTaskBatchSchema(),
	},
	{
		CommandName: "pm.create_task",
		Alias:       "create_task",
		Category:    "PM / Tasks",
		Description: "Create a single task for a team, optionally targeting a specific workflow and stage. If workflow_id or state_id are omitted, they are resolved from the team workflow defaults.",
		InputSchema: createTaskSchema(),
	},
	{
		CommandName: "pm.ensure_label",
		Alias:       "ensure_task_label",
		Category:    "PM / Tasks",
		Description: "Create or return a PM task label in the current workspace. Use this before creating tasks that must carry a stable label.",
		InputSchema: ensureTaskLabelSchema(),
	},
	{
		CommandName: "pm.list_tasks",
		Alias:       "list_tasks",
		Category:    "PM / Tasks",
		Description: "List tasks in the current workspace with optional text query, label, team, open-only, description, and comment filters. Every displayed task must use its returned markdown_link verbatim.",
		InputSchema: listTasksSchema(),
	},
	{
		CommandName: "pm.add_task_comment",
		Alias:       "add_task_comment",
		Category:    "PM / Tasks",
		Description: "Add a markdown comment to a task. If task_id is omitted, defaults to the current task target when available.",
		InputSchema: addTaskCommentSchema(),
	},
	{
		CommandName: "pm.assign_task_agent",
		Alias:       "assign_task_agent",
		Category:    "PM / Tasks",
		Description: "Deprecated. Task agent assignment was removed; use workflow automation rules or start a run explicitly with an agent.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "The task ID to assign",
				},
				"agent_id": map[string]any{
					"type":        "string",
					"description": "The target agent ID",
				},
			},
			"required": []string{"agent_id"},
		},
	},
	{
		CommandName: "pm.set_task_dependencies",
		Alias:       "set_task_dependencies",
		Category:    "PM / Tasks",
		Description: "Create explicit task dependency links between existing tasks.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dependencies": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": 100,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"source_task_id": map[string]any{"type": "string"},
							"target_task_id": map[string]any{"type": "string"},
						},
						"required":             []string{"source_task_id", "target_task_id"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"dependencies"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "pm.update_task_state",
		Alias:       "update_task_state",
		Category:    "PM / Tasks",
		Description: "Transition the current task to a different workflow state.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"state_id": map[string]any{
					"type":        "string",
					"description": "The target workflow state ID",
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "Optional task ID override. Defaults to the current task target.",
				},
			},
			"required": []string{"state_id"},
		},
	},
	{
		CommandName: "pm.list_team_workflows_with_stages",
		Alias:       "list_team_workflows_with_stages",
		Category:    "PM / Tasks",
		Description: "List the resolved workflow and ordered stages for one team or all workspace teams. Use this to choose a valid workflow stage before creating a task.",
		InputSchema: boundedListSchema(map[string]any{
			"team_id": optionalIDSchema("Optional team ID. Omit to return workflow summaries for all workspace teams."),
		}),
	},
	{
		CommandName: "pm.list_labels",
		Alias:       "list_pm_labels",
		Category:    "PM / Tasks",
		Description: "List non-archived workspace and team labels that are compatible with the current actor's PM scope.",
		InputSchema: boundedListSchema(map[string]any{
			"team_id": optionalIDSchema("Optional team ID filter."),
			"name":    map[string]any{"type": "string", "description": "Optional case-insensitive label name filter."},
		}),
	},
	{
		CommandName: "pm.get_task",
		Alias:       "get_task",
		Category:    "PM / Tasks",
		Description: "Get one accessible task by UUID or human task key with its state, team, owners, labels, epic or sprint references, deadline, and blocking metadata.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id":  optionalIDSchema("Optional task UUID. Omit to use task_key or the current task target."),
				"task_key": map[string]any{"type": "string", "description": "Optional human task key, for example USE-488."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "pm.update_task",
		Alias:       "update_task",
		Category:    "PM / Tasks",
		Description: "Update bounded editable fields on an accessible task. Team, workflow, state, archive, and deletion changes are excluded.",
		InputSchema: updateTaskSchema(),
	},
	{
		CommandName: "pm.create_task_checklist_item",
		Alias:       "create_task_checklist_item",
		Category:    "PM / Tasks",
		Description: "Create one checklist item on an accessible task.",
		InputSchema: createTaskChecklistItemSchema(),
	},
	{
		CommandName: "pm.list_task_checklist",
		Alias:       "list_task_checklist",
		Category:    "PM / Tasks",
		Description: "List up to 100 checklist items belonging to one accessible task.",
		InputSchema: listTaskChecklistSchema(),
	},
	{
		CommandName: "pm.update_task_checklist_item",
		Alias:       "update_task_checklist_item",
		Category:    "PM / Tasks",
		Description: "Update bounded fields on one checklist item belonging to an accessible task.",
		InputSchema: updateTaskChecklistItemSchema(),
	},
	{
		CommandName: "pm.add_comment",
		Alias:       "add_pm_comment",
		Category:    "PM / Tasks",
		Description: "Add a markdown comment to a task, epic, sprint, or objective. The current entity target supplies the type and ID by default.",
		InputSchema: addPMCommentSchema(),
	},
	{
		CommandName: "pm.list_epics",
		Alias:       "list_epics",
		Category:    "PM / Epics",
		Description: "List accessible epics with team, state, owner, dates, health, labels, and compact progress stats.",
		InputSchema: listEpicsSchema(),
	},
	{
		CommandName: "pm.get_epic",
		Alias:       "get_epic",
		Category:    "PM / Epics",
		Description: "Get one accessible epic with description, associations, planning repository, health, and compact progress stats.",
		InputSchema: entityIDSchema("epic_id", "Optional epic ID. Omit to use the current epic target."),
	},
	{
		CommandName: "pm.create_epic",
		Alias:       "create_epic",
		Category:    "PM / Epics",
		Description: "Create an epic with bounded operational fields. Agent auto-run and archive controls are excluded.",
		InputSchema: createEpicSchema(),
	},
	{
		CommandName: "pm.update_epic",
		Alias:       "update_epic",
		Category:    "PM / Epics",
		Description: "Update bounded operational fields on an epic. Omitted association arrays are preserved and supplied arrays replace the complete set.",
		InputSchema: updateEpicSchema(),
	},
	{
		CommandName: "pm.list_sprints",
		Alias:       "list_sprints",
		Category:    "PM / Sprints",
		Description: "List accessible sprints with dates, derived status, team, labels, and compact progress stats.",
		InputSchema: listSprintsSchema(),
	},
	{
		CommandName: "pm.get_sprint",
		Alias:       "get_sprint",
		Category:    "PM / Sprints",
		Description: "Get one accessible sprint with description, dates, derived status, team, labels, and compact progress stats.",
		InputSchema: entityIDSchema("sprint_id", "Optional sprint ID. Omit to use the current sprint target."),
	},
	{
		CommandName: "pm.list_sprint_tasks",
		Alias:       "list_sprint_tasks",
		Category:    "PM / Sprints",
		Description: "List compact non-archived tasks in an accessible sprint, optionally filtered by a case-insensitive text query.",
		InputSchema: boundedListSchema(map[string]any{
			"query":     map[string]any{"type": "string", "description": "Optional case-insensitive text query matched against task name, description, or display ID."},
			"sprint_id": optionalIDSchema("Optional sprint ID. Omit to use the current sprint target."),
		}),
	},
	{
		CommandName: "pm.create_sprint",
		Alias:       "create_sprint",
		Category:    "PM / Sprints",
		Description: "Create a sprint using YYYY-MM-DD dates, an owning team, and optional labels.",
		InputSchema: createSprintSchema(),
	},
	{
		CommandName: "pm.update_sprint",
		Alias:       "update_sprint",
		Category:    "PM / Sprints",
		Description: "Update bounded sprint fields. Omitted label IDs preserve current labels and supplied IDs replace the complete set.",
		InputSchema: updateSprintSchema(),
	},
	{
		CommandName: "pm.list_objectives",
		Alias:       "list_objectives",
		Category:    "PM / Objectives",
		Description: "List workspace objectives with type, state, dates, health, teams, owners, labels, linked epics, and compact stats.",
		InputSchema: listObjectivesSchema(),
	},
	{
		CommandName: "pm.get_objective",
		Alias:       "get_objective",
		Category:    "PM / Objectives",
		Description: "Get one objective with description, associations, key results, linked epics, and compact progress stats.",
		InputSchema: entityIDSchema("objective_id", "Optional objective ID. Omit to use the current objective target."),
	},
	{
		CommandName: "pm.create_objective",
		Alias:       "create_objective",
		Category:    "PM / Objectives",
		Description: "Create an objective with bounded operational fields and complete association sets.",
		InputSchema: createObjectiveSchema(),
	},
	{
		CommandName: "pm.update_objective",
		Alias:       "update_objective",
		Category:    "PM / Objectives",
		Description: "Update bounded objective fields. Omitted association arrays are preserved and supplied arrays replace the complete set.",
		InputSchema: updateObjectiveSchema(),
	},
	{
		CommandName: "pm.create_key_result",
		Alias:       "create_key_result",
		Category:    "PM / Objectives",
		Description: "Create a key result on the current objective target.",
		InputSchema: createKeyResultSchema(),
	},
	{
		CommandName: "pm.update_key_result",
		Alias:       "update_key_result",
		Category:    "PM / Objectives",
		Description: "Update bounded fields on a key result belonging to the current objective target.",
		InputSchema: updateKeyResultSchema(),
	},
	{
		CommandName: "pm.update_task_delivery_target",
		Alias:       "update_task_delivery_target",
		Category:    "PM / Delivery",
		Description: "Set or clear the repository and branch delivery target for an accessible task.",
		InputSchema: deliveryTargetSchema("task_id", "working_branch"),
	},
	{
		CommandName: "pm.update_epic_delivery_target",
		Alias:       "update_epic_delivery_target",
		Category:    "PM / Delivery",
		Description: "Set or clear the repository and branch delivery target for an accessible epic.",
		InputSchema: deliveryTargetSchema("epic_id", "epic_branch"),
	},
	{
		CommandName: "docs.update_document_metadata",
		Alias:       "update_document_metadata",
		Category:    "Docs",
		Description: "Update bounded metadata on an accessible document without changing its content, location, template, or publication state.",
		InputSchema: updateDocumentMetadataSchema(),
	},
	{
		CommandName: "crm.get_contact",
		Alias:       "get_crm_contact",
		Category:    "CRM / Discovery",
		Description: "Get one CRM contact in the current workspace.",
		InputSchema: requiredEntityIDSchema("contact_id"),
	},
	{
		CommandName: "crm.get_company",
		Alias:       "get_crm_company",
		Category:    "CRM / Discovery",
		Description: "Get one CRM company in the current workspace.",
		InputSchema: requiredEntityIDSchema("company_id"),
	},
	{
		CommandName: "crm.get_deal",
		Alias:       "get_crm_deal",
		Category:    "CRM / Discovery",
		Description: "Get one CRM deal in the current workspace.",
		InputSchema: requiredEntityIDSchema("deal_id"),
	},
	{
		CommandName: "crm.list_companies",
		Alias:       "list_crm_companies",
		Category:    "CRM / Discovery",
		Description: "List a bounded set of CRM companies in the current workspace.",
		InputSchema: operationalListSchema(map[string]any{
			"query":           map[string]any{"type": "string", "maxLength": 500},
			"owner_member_id": optionalIDSchema("Optional owner member ID filter."),
		}),
	},
	{
		CommandName: "crm.list_pipelines",
		Alias:       "list_crm_pipelines",
		Category:    "CRM / Discovery",
		Description: "List CRM pipelines and their ordered stages in the current workspace.",
		InputSchema: operationalListSchema(nil),
	},
	{
		CommandName: "crm.list_associations",
		Alias:       "list_crm_associations",
		Category:    "CRM / Discovery",
		Description: "List a bounded set of associations for one existing workspace object.",
		InputSchema: closedObjectSchema(map[string]any{
			"object_type": crmObjectTypeSchema(),
			"object_id":   optionalIDSchema("Object ID whose associations should be listed."),
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results. Defaults to 50, max 100.",
				"minimum":     1,
				"maximum":     100,
			},
		}, []string{"object_type", "object_id"}),
	},
	{
		CommandName: "crm.create_deal",
		Alias:       "create_crm_deal",
		Category:    "CRM / Operations",
		Description: "Create a CRM deal for an existing contact. Use list_crm_pipelines first. If more than one pipeline exists and the user did not choose one, ask which pipeline to use. Always ask which stage to use when the user did not specify it.",
		InputSchema: createCRMDealSchema(),
	},
	{
		CommandName: "crm.update_contact",
		Alias:       "update_crm_contact",
		Category:    "CRM / Operations",
		Description: "Update only lifecycle stage, lead status, owner, or labels on an existing CRM contact.",
		InputSchema: updateCRMContactSchema(),
	},
	{
		CommandName: "crm.update_company",
		Alias:       "update_crm_company",
		Category:    "CRM / Operations",
		Description: "Update only the owner of an existing CRM company.",
		InputSchema: updateCRMCompanySchema(),
	},
	{
		CommandName: "crm.update_deal",
		Alias:       "update_crm_deal",
		Category:    "CRM / Operations",
		Description: "Update bounded operational fields on an existing CRM deal.",
		InputSchema: updateCRMDealSchema(),
	},
	{
		CommandName: "crm.add_activity",
		Alias:       "add_crm_activity",
		Category:    "CRM / Operations",
		Description: "Log a note, call, meeting, or email against exactly one existing CRM contact, company, or deal.",
		InputSchema: addCRMActivitySchema(),
	},
	{
		CommandName: "crm.link_objects",
		Alias:       "link_crm_objects",
		Category:    "CRM / Operations",
		Description: "Create a safe association between two existing objects in the current workspace.",
		InputSchema: linkCRMObjectsSchema(),
	},
	{
		CommandName: "crm.unlink_association",
		Alias:       "unlink_crm_association",
		Category:    "CRM / Operations",
		Description: "Remove one existing CRM association by ID without deleting either linked object.",
		InputSchema: requiredEntityIDSchema("association_id"),
	},
	{
		CommandName: "crm.set_primary_contact_company",
		Alias:       "set_primary_contact_company",
		Category:    "CRM / Operations",
		Description: "Set an existing company as the primary company for an existing contact in the current workspace.",
		InputSchema: closedObjectSchema(map[string]any{
			"contact_id": optionalIDSchema("Existing CRM contact ID."),
			"company_id": optionalIDSchema("Existing CRM company ID."),
		}, []string{"contact_id", "company_id"}),
	},
	{
		CommandName: "support.list_conversations",
		Alias:       "list_support_conversations",
		Category:    "Support / Discovery",
		Description: "List a bounded set of support conversations with optional status, priority, inbox, and search filters.",
		InputSchema: operationalListSchema(map[string]any{
			"status":   map[string]any{"type": "string"},
			"priority": map[string]any{"type": "string"},
			"inbox_id": optionalIDSchema("Optional inbox or mailbox ID filter."),
			"query":    map[string]any{"type": "string", "maxLength": 500},
		}),
	},
	{
		CommandName: "support.get_conversation",
		Alias:       "get_support_conversation",
		Category:    "Support / Discovery",
		Description: "Get one support conversation in the current workspace.",
		InputSchema: requiredEntityIDSchema("conversation_id"),
	},
	{
		CommandName: "support.list_tags",
		Alias:       "list_support_tags",
		Category:    "Support / Discovery",
		Description: "List a bounded set of support tags in the current workspace.",
		InputSchema: operationalListSchema(nil),
	},
	{
		CommandName: "support.list_inboxes",
		Alias:       "list_support_inboxes",
		Category:    "Support / Discovery",
		Description: "List a bounded set of support inboxes available to the current actor.",
		InputSchema: operationalListSchema(nil),
	},
	{
		CommandName: "support.list_assignees",
		Alias:       "list_support_assignees",
		Category:    "Support / Discovery",
		Description: "List assignable users for one support conversation.",
		InputSchema: operationalListSchema(map[string]any{
			"conversation_id": optionalIDSchema("Support conversation ID."),
		}),
	},
	{
		CommandName: "support.assign_conversation",
		Alias:       "assign_support_conversation",
		Category:    "Support / Triage",
		Description: "Assign or unassign an existing support conversation.",
		InputSchema: supportAssociationUpdateSchema("assignee_user_id", "Workspace user ID to assign."),
	},
	{
		CommandName: "support.move_conversation",
		Alias:       "move_support_conversation",
		Category:    "Support / Triage",
		Description: "Move an existing support conversation to another accessible inbox.",
		InputSchema: closedObjectSchema(map[string]any{
			"conversation_id": optionalIDSchema("Support conversation ID."),
			"inbox_id":        optionalIDSchema("Destination inbox or mailbox ID."),
		}, []string{"conversation_id", "inbox_id"}),
	},
	{
		CommandName: "support.add_conversation_tag",
		Alias:       "add_support_conversation_tag",
		Category:    "Support / Triage",
		Description: "Add an existing support tag to a conversation.",
		InputSchema: supportTagMutationSchema(),
	},
	{
		CommandName: "support.remove_conversation_tag",
		Alias:       "remove_support_conversation_tag",
		Category:    "Support / Triage",
		Description: "Remove a support tag from a conversation.",
		InputSchema: supportTagMutationSchema(),
	},
	{
		CommandName: "support.link_conversation_task",
		Alias:       "link_support_conversation_task",
		Category:    "Support / Triage",
		Description: "Link or unlink an existing PM task from a support conversation.",
		InputSchema: supportAssociationUpdateSchema("task_id", "Existing PM task ID to link."),
	},
	{
		CommandName: "support.link_conversation_contact",
		Alias:       "link_support_conversation_contact",
		Category:    "Support / Triage",
		Description: "Link or unlink an existing CRM contact from a support conversation.",
		InputSchema: supportAssociationUpdateSchema("contact_id", "Existing CRM contact ID to link."),
	},
	{
		CommandName: "support.update_conversation_subject",
		Alias:       "update_support_conversation_subject",
		Category:    "Support / Triage",
		Description: "Update the subject of an existing support conversation.",
		InputSchema: closedObjectSchema(map[string]any{
			"conversation_id": optionalIDSchema("Support conversation ID."),
			"subject":         map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"conversation_id", "subject"}),
	},
	{
		CommandName: "docs.write_document_content",
		Alias:       "write_document_content",
		Category:    "Docs",
		Description: "Write text or structured content to the document identified by document_id, including from a workspace-targeted Dock run. Markdown is auto-converted. This tool does not embed private run artifacts: after writing the document, call insert_document_artifact with the artifact_id returned by browser_screenshot or browser_record.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"description": "The document ID to update",
				},
				"content": map[string]any{
					"description": "The document content to save. Use either a structured document JSON object or a markdown string.",
					"oneOf": []map[string]any{
						{"type": "object"},
						{"type": "string"},
					},
				},
			},
			"required": []string{"document_id", "content"},
		},
	},
	{
		CommandName: "git.list_repositories",
		Alias:       "list_repositories",
		Category:    "Git",
		Description: "List the git repositories connected to this workspace (id, full name, default branch, provider). Use this to discover a repository to target or to ask the user which repo to use.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.create_document",
		Alias:       "create_document",
		Category:    "Docs",
		Description: "Create a new document in Helpin Docs. Accepts optional markdown content that will be auto-converted to rich text. If space_id is omitted it defaults to the workspace's only space; when several spaces exist, call list_spaces and ask the user which to use.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space_id": map[string]any{
					"type":        "string",
					"description": "The space ID where the document will be created. Optional — omit to use the workspace's only space; required when multiple spaces exist (use list_spaces to discover IDs).",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "The document title",
				},
				"collection_id": map[string]any{
					"type":        "string",
					"description": "Optional collection ID to place the document in",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Optional initial document content as a markdown string. Will be auto-converted to rich text.",
				},
				"icon": map[string]any{
					"type":        "string",
					"maxLength":   100,
					"description": "Canonical icon ID. Public MCP callers should use search_icons to discover valid values, or omit this field.",
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional tags for the document",
				},
			},
			"required":             []string{"title"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.update_document_block",
		Alias:       "update_document_block",
		Category:    "Docs",
		Description: "Update one addressable block in a Helpin Docs document using its current revision. The response includes the block's new revision; other blocks' revisions are unaffected, so sequential updates can reuse revisions from one get_document_blocks call.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"description": "The document ID to update",
				},
				"block_id": map[string]any{
					"type":        "string",
					"description": "The stable block ID to update",
				},
				"revision": map[string]any{
					"type":        "integer",
					"description": "The current block revision from get_document_blocks",
				},
				"content": map[string]any{
					"type":        "object",
					"description": "The replacement block node JSON",
				},
			},
			"required": []string{"document_id", "block_id", "revision", "content"},
		},
	},
	{
		CommandName: "docs.insert_document_block",
		Alias:       "insert_document_block",
		Category:    "Docs",
		Description: "Insert new content between existing blocks of a Helpin Docs document without rewriting them. Each top-level markdown block in content becomes one document block.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"description": "The document that receives the new blocks",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Markdown for the new content. May contain multiple blocks (for example a heading followed by a paragraph).",
				},
				"after_block_id": map[string]any{
					"type":        "string",
					"description": "Optional block ID from get_document_blocks after which to insert; omit to use position",
				},
				"position": map[string]any{
					"type":        "string",
					"enum":        []string{"start", "end"},
					"description": "Where to insert when after_block_id is omitted. Defaults to end.",
				},
			},
			"required":             []string{"document_id", "content"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.insert_document_artifact",
		Alias:       "insert_document_artifact",
		Category:    "Docs",
		Description: "Insert a supported private run artifact as an authenticated image or video block in a Helpin Docs document. Use the exact artifact_id returned by browser_screenshot or browser_record. A filename, artifact_ref, URL, or markdown link written with write_document_content is text only and is not an embed. Helpin derives the block from the trusted artifact record; never provide an artifact type, storage URL, or content type.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "The document that will receive the artifact",
				},
				"artifact_id": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "The opaque artifact_id returned by a tool such as browser_screenshot or browser_record",
				},
				"after_block_id": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "Optional block ID after which to insert the artifact; omit to append",
				},
				"description": map[string]any{
					"type":        "string",
					"minLength":   1,
					"maxLength":   1000,
					"description": "Accessible description of the artifact content",
				},
				"caption": map[string]any{
					"type":        "string",
					"maxLength":   2000,
					"description": "Optional visible caption",
				},
			},
			"required":             []string{"document_id", "artifact_id", "description"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.insert_document_image",
		Alias:       "insert_document_image",
		Category:    "Docs",
		Description: "Compatibility alias for inserting a private browser screenshot. Prefer insert_document_artifact for new calls.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"description": "The document that will receive the image",
				},
				"artifact_id": map[string]any{
					"type":        "string",
					"description": "The artifact_id returned by browser_screenshot",
				},
				"after_block_id": map[string]any{
					"type":        "string",
					"description": "Optional block ID after which to insert the image; omit to append",
				},
				"alt": map[string]any{
					"type":        "string",
					"description": "Accessible description of the screenshot",
				},
				"caption": map[string]any{
					"type":        "string",
					"description": "Optional visible image caption",
				},
			},
			"required":             []string{"document_id", "artifact_id", "alt"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "docs.create_space",
		Alias:       "create_space",
		Category:    "Docs",
		Description: "Create a Docs space in the current workspace without publishing content.",
		InputSchema: docsCreateSpaceSchema(),
	},
	{
		CommandName: "docs.create_collection",
		Alias:       "create_collection",
		Category:    "Docs",
		Description: "Create a top-level or nested collection in an existing Docs space.",
		InputSchema: docsCreateCollectionSchema(),
	},
	{
		CommandName: "docs.update_space",
		Alias:       "update_space",
		Category:    "Docs",
		Description: "Update bounded metadata for an existing Docs space.",
		InputSchema: docsUpdateSpaceSchema(),
	},
	{
		CommandName: "docs.update_collection",
		Alias:       "update_collection",
		Category:    "Docs",
		Description: "Update or reparent an existing Docs collection.",
		InputSchema: docsUpdateCollectionSchema(),
	},
	{
		CommandName: "docs.move_document",
		Alias:       "move_document",
		Category:    "Docs",
		Description: "Move a document to another Docs space or collection.",
		InputSchema: docsMoveDocumentSchema(),
	},
	{
		CommandName: "docs.link_document_to_object",
		Alias:       "link_document_to_object",
		Category:    "Docs",
		Description: "Create a Helpin Docs link between a document and another internal object.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id": map[string]any{
					"type":        "string",
					"description": "The document ID to link",
				},
				"linked_object_type": map[string]any{
					"type":        "string",
					"description": "The linked object type such as epic or task",
					"enum":        []string{"epic", "task", "deal", "crm_deal"},
				},
				"linked_object_id": map[string]any{
					"type":        "string",
					"description": "The linked object ID",
				},
				"link_context": map[string]any{
					"type":        "string",
					"description": "Optional link context, defaults to attached",
					"enum":        []string{"attached", "mentioned", "created_from", "linked_in_content"},
				},
			},
			"required":             []string{"document_id", "linked_object_type", "linked_object_id"},
			"additionalProperties": false,
		},
	},
	{
		CommandName: "crm.update_deal_stage",
		Alias:       "update_deal_stage",
		Category:    "CRM",
		Description: "Move a CRM deal to a different pipeline stage.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deal_id": map[string]any{
					"type":        "string",
					"description": "The deal ID to update",
				},
				"stage_id": map[string]any{
					"type":        "string",
					"description": "The target pipeline stage ID",
				},
			},
			"required": []string{"deal_id", "stage_id"},
		},
	},
	{
		CommandName: "crm.add_deal_note",
		Alias:       "add_deal_note",
		Category:    "CRM",
		Description: "Add a note or comment to a CRM deal.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deal_id": map[string]any{
					"type":        "string",
					"description": "The deal ID to add a note to",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The note content",
				},
			},
			"required": []string{"deal_id", "content"},
		},
	},
	{
		CommandName: "crm.enrich_contact",
		Alias:       "enrich_crm_contact",
		Category:    "CRM",
		Description: "Safely enrich a CRM contact with sourced public data. Names and existing email/phone are protected; core fields are fill-only and agent-owned metadata is namespaced.",
		InputSchema: crmEnrichmentSchema("contact_id", []string{"email", "phone", "job_title", "avatar_url", "linkedin_url", "location", "enrichment_note"}),
	},
	{
		CommandName: "crm.enrich_company",
		Alias:       "enrich_crm_company",
		Category:    "CRM",
		Description: "Safely enrich a CRM company with sourced public data. Company name and existing domain are protected; core fields are fill-only and agent-owned metadata is namespaced.",
		InputSchema: crmEnrichmentSchema("company_id", []string{"domain", "industry", "employee_count", "annual_revenue", "description", "logo_url", "linkedin_url", "headquarters", "enrichment_note"}),
	},
	{
		CommandName: "crm.ensure_contact_company",
		Alias:       "ensure_crm_contact_company",
		Category:    "CRM",
		Description: "Create or reuse a CRM company and associate it with a contact. Does not modify existing company identity fields.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"contact_id": map[string]any{
					"type":        "string",
					"description": "The contact ID to associate with a company. Defaults to the current CRM contact target when omitted by the runtime.",
				},
				"company_name": map[string]any{
					"type":        "string",
					"description": "The company name to create or match.",
				},
				"domain": map[string]any{
					"type":        "string",
					"description": "Optional company domain to match or set on a newly-created company.",
				},
				"source_url": map[string]any{
					"type":        "string",
					"description": "Public source URL supporting the company/contact relationship.",
				},
				"evidence": map[string]any{
					"type":        "string",
					"description": "Short explanation of the evidence for the relationship.",
				},
				"confidence": map[string]any{
					"type":        "number",
					"description": "Confidence from 0.0 to 1.0. Values below 0.70 are rejected.",
					"minimum":     0,
					"maximum":     1,
				},
				"association_label": map[string]any{
					"type":        "string",
					"description": "Optional association label. Defaults to primary.",
				},
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "When true, returns whether it would create/reuse/link without writing CRM records.",
				},
			},
			"required":             []string{"contact_id", "company_name", "source_url", "evidence", "confidence"},
			"additionalProperties": false,
		},
	},
}

func crmEnrichmentSchema(idField string, fieldEnum []string) map[string]any {
	fieldItem := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":        "string",
				"enum":        fieldEnum,
				"description": "The guarded CRM field to enrich. Identity fields such as names are intentionally unavailable.",
			},
			"value": map[string]any{
				"description": "The proposed field value. Use a string for text/url fields, an integer for employee_count, and a number for annual_revenue.",
			},
			"source_url": map[string]any{
				"type":        "string",
				"description": "Public source URL that supports the value.",
			},
			"evidence": map[string]any{
				"type":        "string",
				"description": "Short explanation of the evidence from the source.",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Confidence from 0.0 to 1.0. Values below 0.70 are rejected.",
				"minimum":     0,
				"maximum":     1,
			},
		},
		"required":             []string{"field", "value", "source_url", "evidence", "confidence"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			idField: map[string]any{
				"type":        "string",
				"description": "The CRM object ID to enrich.",
			},
			"fields": map[string]any{
				"type":        "array",
				"description": "Guarded field updates to apply. Existing protected values are skipped, not overwritten.",
				"items":       fieldItem,
				"minItems":    1,
				"maxItems":    20,
			},
			"evidence_summary": map[string]any{
				"type":        "string",
				"description": "Brief summary of the researched evidence.",
			},
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "When true, returns what would be applied/skipped without writing CRM fields.",
			},
		},
		"required":             []string{idField, "fields"},
		"additionalProperties": false,
	}
}

func createTaskBatchSchema() map[string]any {
	fileChangeSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string"},
			"action":      map[string]any{"type": "string", "enum": []string{"create", "modify", "delete"}},
			"description": map[string]any{"type": "string"},
		},
		"required":             []string{"path", "action", "description"},
		"additionalProperties": false,
	}
	implementationBriefSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"approach": map[string]any{"type": "string"},
			"files_to_modify": map[string]any{
				"type":  "array",
				"items": fileChangeSchema,
			},
			"test_strategy": map[string]any{
				"anyOf": []map[string]any{
					{"type": "string"},
					{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
			},
			"vertical_layers": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"depends_on_files": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required":             []string{"approach", "files_to_modify", "test_strategy"},
		"additionalProperties": false,
	}

	taskSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref":         map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"task_type":   map[string]any{"type": "string"},
			"estimate":    map[string]any{"type": "integer"},
			"priority":    map[string]any{"type": "string"},
			"acceptance_criteria": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"dependency_refs": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"source_refs": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "object"},
			},
			"assign_agent_id":      map[string]any{"type": "string"},
			"slice_type":           map[string]any{"type": "string"},
			"implementation_brief": implementationBriefSchema,
		},
		"required": []string{"name", "description"},
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tasks": map[string]any{
				"description": "The list of tasks to create.",
				"type":        "array",
				"items":       taskSchema,
			},
			"proposed_tasks": map[string]any{
				"description": "Preferred alias for task-plan payloads. If present, it is treated the same as tasks.",
				"type":        "array",
				"items":       taskSchema,
			},
		},
	}
}

func docsCreateSpaceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":                map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"slug":                map[string]any{"type": "string", "maxLength": 200},
			"icon":                map[string]any{"type": "string", "maxLength": 100},
			"visibility":          map[string]any{"type": "string", "enum": []string{"workspace_wide", "team_only"}},
			"type":                map[string]any{"type": "string", "enum": []string{"internal", "external_capable"}},
			"default_review_days": map[string]any{"type": "integer", "minimum": 1, "maximum": 3650},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

func docsCreateCollectionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"space_id":             map[string]any{"type": "string"},
			"name":                 map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"slug":                 map[string]any{"type": "string", "maxLength": 200},
			"description":          map[string]any{"type": "string", "maxLength": 5000},
			"icon":                 map[string]any{"type": "string", "maxLength": 100},
			"parent_collection_id": map[string]any{"type": "string"},
		},
		"required":             []string{"space_id", "name"},
		"additionalProperties": false,
	}
}

func docsUpdateSpaceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"space_id":            map[string]any{"type": "string"},
			"name":                map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"icon":                map[string]any{"type": "string", "maxLength": 100},
			"visibility":          map[string]any{"type": "string", "enum": []string{"workspace_wide", "team_only"}},
			"default_review_days": map[string]any{"type": "integer", "minimum": 1, "maximum": 3650},
		},
		"required":             []string{"space_id"},
		"additionalProperties": false,
	}
}

func docsUpdateCollectionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"collection_id":        map[string]any{"type": "string"},
			"name":                 map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"description":          map[string]any{"type": "string", "maxLength": 5000},
			"icon":                 map[string]any{"type": "string", "maxLength": 100},
			"position":             map[string]any{"type": "integer", "minimum": 0},
			"parent_collection_id": map[string]any{"type": "string"},
		},
		"required":             []string{"collection_id"},
		"additionalProperties": false,
	}
}

func docsMoveDocumentSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"document_id":   map[string]any{"type": "string"},
			"space_id":      map[string]any{"type": "string"},
			"collection_id": map[string]any{"type": "string"},
		},
		"required":             []string{"document_id", "space_id"},
		"additionalProperties": false,
	}
}

func createTaskSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Task title",
				"minLength":   1,
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Optional task description",
			},
			"task_type": map[string]any{
				"type":        "string",
				"description": "Optional task type.",
				"enum":        taskTypeValues(),
			},
			"estimate": map[string]any{
				"type":        "integer",
				"description": "Optional estimate value",
			},
			"priority": map[string]any{
				"type":        "string",
				"description": "Optional task priority.",
				"enum":        taskPriorityValues(),
			},
			"severity": map[string]any{
				"type":        "string",
				"description": "Optional task severity.",
				"enum":        taskSeverityValues(),
			},
			"epic_id": map[string]any{
				"type":        "string",
				"description": "Optional epic ID to link the task to. Omit this field when no epic is configured; do not send an empty string.",
				"minLength":   1,
			},
			"sprint_id": map[string]any{
				"type":        "string",
				"description": "Optional sprint ID to link the task to. Omit this field when no sprint is configured; do not send an empty string.",
				"minLength":   1,
			},
			"team_id": map[string]any{
				"type":        "string",
				"description": "Team ID that owns the task",
				"minLength":   1,
			},
			"workflow_id": map[string]any{
				"type":        "string",
				"description": "Optional workflow ID override. Omit this field to use the resolved team workflow; do not send an empty string.",
				"minLength":   1,
			},
			"state_id": map[string]any{
				"type":        "string",
				"description": "Optional workflow state ID override. Omit this field to use the workflow default state; do not send an empty string.",
				"minLength":   1,
			},
			"owner_member_ids": map[string]any{
				"type":        "array",
				"description": "Optional workspace member IDs to assign as owners",
				"items":       map[string]any{"type": "string", "minLength": 1},
				"maxItems":    100,
			},
			"label_ids": map[string]any{
				"type":        "array",
				"description": "Optional label IDs to attach to the task",
				"items":       map[string]any{"type": "string", "minLength": 1},
				"maxItems":    100,
			},
			"deadline": map[string]any{
				"type":        "string",
				"description": "Optional deadline in YYYY-MM-DD format.",
			},
			"blocked": map[string]any{
				"type":        "boolean",
				"description": "Optional manual blocked flag.",
			},
			"blocker": map[string]any{
				"type":        "string",
				"description": "Optional description of the blocker.",
			},
			"checklist_items": map[string]any{
				"type":        "array",
				"description": "Optional checklist items to create with the task.",
				"maxItems":    100,
				"items":       checklistItemCreateSchema(),
			},
		},
		"required":             []string{"name", "team_id"},
		"additionalProperties": false,
	}
}

func ensureTaskLabelSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Label name to create or return, for example security.",
			},
			"team_id": map[string]any{
				"type":        "string",
				"description": "Optional team scope. Omit for a shared workspace label.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Optional description used when the label is first created.",
			},
			"color": map[string]any{
				"type":        "string",
				"description": "Optional hex color used when the label is first created.",
			},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

func listTasksSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Optional case-insensitive text query matched against task name, description, or display ID.",
			},
			"label_id": map[string]any{
				"type":        "string",
				"description": "Optional label ID filter.",
			},
			"team_id": map[string]any{
				"type":        "string",
				"description": "Optional team ID filter.",
			},
			"epic_id":     optionalIDSchema("Optional epic ID filter."),
			"sprint_id":   optionalIDSchema("Optional sprint ID filter."),
			"workflow_id": optionalIDSchema("Optional workflow ID filter."),
			"state_id":    optionalIDSchema("Optional workflow state ID filter."),
			"task_type": map[string]any{
				"type": "string",
				"enum": taskTypeValues(),
			},
			"priority": map[string]any{
				"type": "string",
				"enum": taskPriorityValues(),
			},
			"severity": map[string]any{
				"type": "string",
				"enum": taskSeverityValues(),
			},
			"completed": map[string]any{
				"type":        "boolean",
				"description": "Optional completion-state filter.",
			},
			"archived": map[string]any{
				"type":        "boolean",
				"description": "Optional archive-state filter.",
			},
			"updated_after": map[string]any{
				"type":        "string",
				"description": "Optional lower bound for task updates in YYYY-MM-DD format.",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Optional task ID. Omit to use the current task target when available.",
			},
			"owner_member_ids": map[string]any{
				"type":        "array",
				"description": "Optional workspace member IDs. When present, only tasks owned by at least one of these members are returned.",
				"items": map[string]any{
					"type": "string",
				},
				"maxItems": 100,
			},
			"owned_by_actor": map[string]any{
				"type":        "boolean",
				"description": "When true, filter to tasks owned by the current workspace actor.",
			},
			"open_only": map[string]any{
				"type":        "boolean",
				"description": "When true, only return non-completed, non-archived tasks.",
			},
			"include_descriptions": map[string]any{
				"type":        "boolean",
				"description": "When true, include task descriptions in the response.",
			},
			"include_comments": map[string]any{
				"type":        "boolean",
				"description": "When true, include recent task comments in the response.",
			},
			"detail_level": map[string]any{
				"type":        "string",
				"description": "Optional response shape. Use compact for bounded task rows with short description/comment excerpts.",
				"enum":        []string{"summary", "compact", "full"},
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum tasks to return. Defaults to 50, max 100.",
				"minimum":     1,
				"maximum":     100,
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "Deprecated compatibility input. 1-based result page. Do not combine with limit or offset.",
				"minimum":     1,
			},
			"per_page": map[string]any{
				"type":        "integer",
				"description": "Deprecated compatibility input. Results per page, max 100. Do not combine with limit or offset.",
				"minimum":     1,
				"maximum":     100,
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Zero-based result offset. Use next_offset from the previous response.",
				"minimum":     0,
			},
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func addTaskCommentSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "Optional task ID. Omit to use the current task target when available.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The markdown comment body.",
			},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}
}

func boundedListSchema(properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	properties["page"] = map[string]any{
		"type":        "integer",
		"description": "Deprecated compatibility input. 1-based result page. Do not combine with limit or offset.",
		"minimum":     1,
	}
	properties["per_page"] = map[string]any{
		"type":        "integer",
		"description": "Deprecated compatibility input. Results per page, max 100. Do not combine with limit or offset.",
		"minimum":     1,
		"maximum":     100,
	}
	properties["limit"] = map[string]any{
		"type":        "integer",
		"description": "Maximum results to return. Defaults to 50, max 100.",
		"minimum":     1,
		"maximum":     100,
	}
	properties["offset"] = map[string]any{
		"type":        "integer",
		"description": "Zero-based result offset. Use next_offset from the previous response.",
		"minimum":     0,
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{},
		"additionalProperties": false,
	}
}

func paginatedQuerySchema(maxLimit, defaultLimit int, limitDescription string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":  map[string]any{"type": "string", "description": "Optional case-insensitive search query."},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": maxLimit, "default": defaultLimit, "description": limitDescription},
			"offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based result offset. Use next_offset from the previous response."},
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func closedObjectSchema(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func operationalListSchema(properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	properties["limit"] = map[string]any{
		"type":        "integer",
		"description": "Maximum number of results. Defaults to 50, max 100.",
		"minimum":     1,
		"maximum":     100,
	}
	return closedObjectSchema(properties, nil)
}

func requiredEntityIDSchema(field string) map[string]any {
	return closedObjectSchema(map[string]any{
		field: optionalIDSchema("Existing object ID."),
	}, []string{field})
}

func deliveryTargetSchema(idField, branchField string) map[string]any {
	return closedObjectSchema(map[string]any{
		idField:         optionalIDSchema("Optional target ID. Omit to use the current run target."),
		"repository_id": optionalIDSchema("Enabled workspace repository ID."),
		"base_branch":   map[string]any{"type": "string", "minLength": 1, "maxLength": 255},
		branchField:     map[string]any{"type": "string", "minLength": 1, "maxLength": 255},
		"clear_target":  map[string]any{"type": "boolean", "description": "Set true to clear the configured repository and branches."},
	}, nil)
}

func updateDocumentMetadataSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"document_id":   optionalIDSchema("Existing document ID."),
		"title":         map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"owner_id":      optionalIDSchema("Workspace user ID that should own the document."),
		"clear_owner":   map[string]any{"type": "boolean"},
		"excerpt":       map[string]any{"type": "string", "maxLength": 5000},
		"clear_excerpt": map[string]any{"type": "boolean"},
		"icon":          map[string]any{"type": "string", "maxLength": 100},
		"clear_icon":    map[string]any{"type": "boolean"},
		"tags": map[string]any{
			"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 100}, "maxItems": 100,
		},
		"is_pinned": map[string]any{"type": "boolean"},
	}, []string{"document_id"})
}

func updateCRMContactSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"contact_id": optionalIDSchema("Existing CRM contact ID."),
		"lifecycle_stage": map[string]any{"type": "string", "enum": []string{
			model.CRMLifecycleSubscriber, model.CRMLifecycleLead, model.CRMLifecycleMarketingQualified,
			model.CRMLifecycleSalesQualified, model.CRMLifecycleOpportunity, model.CRMLifecycleCustomer, model.CRMLifecycleEvangelist,
		}},
		"lead_status": map[string]any{"type": "string", "enum": []string{
			model.CRMLeadStatusNew, model.CRMLeadStatusOpen, model.CRMLeadStatusInProgress, model.CRMLeadStatusUnqualified,
		}},
		"owner_member_id": optionalIDSchema("Workspace member ID that should own the contact."),
		"clear_owner":     map[string]any{"type": "boolean"},
		"labels": map[string]any{
			"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 100}, "maxItems": 100,
		},
	}, []string{"contact_id"})
}

func updateCRMCompanySchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"company_id":      optionalIDSchema("Existing CRM company ID."),
		"owner_member_id": optionalIDSchema("Workspace member ID that should own the company."),
		"clear_owner":     map[string]any{"type": "boolean"},
	}, []string{"company_id"})
}

func updateCRMDealSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"deal_id":           optionalIDSchema("Existing CRM deal ID."),
		"name":              map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"pipeline_id":       optionalIDSchema("Existing pipeline ID."),
		"stage_id":          optionalIDSchema("Existing stage ID in the selected pipeline."),
		"amount":            map[string]any{"type": "number", "minimum": 0},
		"clear_amount":      map[string]any{"type": "boolean"},
		"currency":          map[string]any{"type": "string", "minLength": 3, "maxLength": 3},
		"close_date":        map[string]any{"type": "string", "format": "date"},
		"clear_close_date":  map[string]any{"type": "boolean"},
		"owner_member_id":   optionalIDSchema("Workspace member ID that should own the deal."),
		"clear_owner":       map[string]any{"type": "boolean"},
		"probability":       map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		"clear_probability": map[string]any{"type": "boolean"},
	}, []string{"deal_id"})
}

func createCRMDealSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"name":            map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"contact_id":      optionalIDSchema("Existing CRM contact to associate with the deal."),
		"pipeline_id":     optionalIDSchema("Existing pipeline ID. May be omitted only when the workspace has exactly one pipeline."),
		"stage_id":        optionalIDSchema("Stage ID explicitly selected by the user from the chosen pipeline."),
		"amount":          map[string]any{"type": "number", "minimum": 0},
		"currency":        map[string]any{"type": "string", "minLength": 3, "maxLength": 3},
		"close_date":      map[string]any{"type": "string", "format": "date"},
		"owner_member_id": optionalIDSchema("Optional active workspace member who should own the deal."),
		"probability":     map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
	}, []string{"name", "contact_id", "stage_id"})
}

func addCRMActivitySchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"activity_type":   map[string]any{"type": "string", "enum": []string{model.CRMActivityNote, model.CRMActivityCall, model.CRMActivityMeeting, model.CRMActivityEmail}},
		"contact_id":      optionalIDSchema("Existing CRM contact ID."),
		"company_id":      optionalIDSchema("Existing CRM company ID."),
		"deal_id":         optionalIDSchema("Existing CRM deal ID."),
		"owner_member_id": optionalIDSchema("Optional workspace member ID credited for the activity."),
		"subject":         map[string]any{"type": "string", "maxLength": 500},
		"body":            map[string]any{"type": "string", "maxLength": 50000},
		"occurred_at":     map[string]any{"type": "string", "format": "date-time"},
	}, []string{"activity_type"})
}

func crmObjectTypeSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{
		model.CRMObjectContact, model.CRMObjectCompany, model.CRMObjectDeal,
		model.CRMObjectEpic, model.CRMObjectTask, model.CRMObjectSupportConversation,
	}}
}

func linkCRMObjectsSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"from_object_type":  crmObjectTypeSchema(),
		"from_object_id":    optionalIDSchema("Existing source object ID."),
		"to_object_type":    crmObjectTypeSchema(),
		"to_object_id":      optionalIDSchema("Existing destination object ID."),
		"association_label": map[string]any{"type": "string", "maxLength": 100},
	}, []string{"from_object_type", "from_object_id", "to_object_type", "to_object_id"})
}

func supportAssociationUpdateSchema(field, description string) map[string]any {
	return closedObjectSchema(map[string]any{
		"conversation_id": optionalIDSchema("Support conversation ID."),
		field:             optionalIDSchema(description),
		"clear":           map[string]any{"type": "boolean", "description": "Set true to remove the current association."},
	}, []string{"conversation_id"})
}

func supportTagMutationSchema() map[string]any {
	return closedObjectSchema(map[string]any{
		"conversation_id": optionalIDSchema("Support conversation ID."),
		"tag_id":          optionalIDSchema("Existing support tag ID."),
	}, []string{"conversation_id", "tag_id"})
}

func entityIDSchema(field, description string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			field: optionalIDSchema(description),
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func optionalIDSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"minLength":   1,
	}
}

func optionalCreateIDSchema(description string) map[string]any {
	return optionalIDSchema("Optional " + description + ". Omit this field when it is not configured; do not send an empty string.")
}

func clearableIDSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description + ", or an empty string to clear it.",
	}
}

func replaceIDArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description + " Omit this field to preserve current associations; when supplied, it replaces the complete set; an empty array clears it.",
		"items":       map[string]any{"type": "string", "minLength": 1},
		"maxItems":    100,
	}
}

func createIDArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string", "minLength": 1},
		"maxItems":    100,
	}
}

func dateSchema(description string, clearable bool) map[string]any {
	if clearable {
		description += " Use YYYY-MM-DD, or an empty string to clear it."
	} else {
		description += " Use YYYY-MM-DD."
	}
	return map[string]any{"type": "string", "description": description}
}

func optionalCreateDateSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "Optional " + description + ". Omit this field when it is not configured. When supplied, use YYYY-MM-DD.",
	}
}

func taskTypeValues() []string {
	return []string{model.PMTaskTypeFeature, model.PMTaskTypeBug, model.PMTaskTypeChore}
}

func taskPriorityValues() []string {
	return []string{model.PMTaskPriorityNone, model.PMTaskPriorityLow, model.PMTaskPriorityMedium, model.PMTaskPriorityHigh, model.PMTaskPriorityUrgent}
}

func taskSeverityValues() []string {
	return []string{model.PMTaskSeverityNone, model.PMTaskSeverityMinor, model.PMTaskSeverityMajor, model.PMTaskSeverityCritical}
}

func checklistItemCreateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":        map[string]any{"type": "string", "minLength": 1},
			"position":    map[string]any{"type": "integer", "minimum": 0},
			"assignee_id": optionalIDSchema("Optional workspace user ID assigned to the checklist item."),
			"due_date":    dateSchema("Optional checklist item due date.", false),
		},
		"required":             []string{"text"},
		"additionalProperties": false,
	}
}

func updateTaskSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id":          optionalIDSchema("Optional task ID. Omit to use the current task target."),
			"name":             map[string]any{"type": "string", "minLength": 1},
			"description":      map[string]any{"type": "string"},
			"task_type":        map[string]any{"type": "string", "enum": taskTypeValues()},
			"epic_id":          map[string]any{"type": "string", "description": "Epic ID to link, or an empty string to clear the epic."},
			"sprint_id":        map[string]any{"type": "string", "description": "Sprint ID to link, or an empty string to clear the sprint."},
			"owner_member_ids": replaceIDArraySchema("Workspace member owner IDs."),
			"estimate":         map[string]any{"type": "integer", "minimum": 0},
			"priority":         map[string]any{"type": "string", "enum": taskPriorityValues()},
			"severity":         map[string]any{"type": "string", "enum": taskSeverityValues()},
			"deadline":         dateSchema("Optional task deadline.", true),
			"blocked":          map[string]any{"type": "boolean"},
			"blocker":          map[string]any{"type": "string", "description": "Blocker description, or an empty string to clear it."},
			"label_ids":        replaceIDArraySchema("Task label IDs."),
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func createTaskChecklistItemSchema() map[string]any {
	properties := checklistItemCreateSchema()["properties"].(map[string]any)
	properties["task_id"] = optionalIDSchema("Optional task ID. Omit to use the current task target.")
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{"text"},
		"additionalProperties": false,
	}
}

func listTaskChecklistSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": optionalIDSchema("Optional task ID. Omit to use the current task target; parent epic and sprint targets require an explicit task ID."),
			"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 100},
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func updateTaskChecklistItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"checklist_item_id": optionalIDSchema("Checklist item ID to update."),
			"task_id":           optionalIDSchema("Optional parent task ID. Omit to use the current task target; parent epic and sprint targets require an explicit task ID."),
			"text":              map[string]any{"type": "string", "minLength": 1},
			"completed":         map[string]any{"type": "boolean"},
			"position":          map[string]any{"type": "integer", "minimum": 0},
			"assignee_id":       map[string]any{"type": "string", "description": "Optional workspace user ID, or an empty string to clear the assignee."},
			"due_date":          dateSchema("Optional checklist item due date.", true),
		},
		"required":             []string{"checklist_item_id"},
		"additionalProperties": false,
	}
}

func addPMCommentSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entity_type": map[string]any{
				"type":        "string",
				"enum":        []string{"task", "epic", "sprint", "objective"},
				"description": "Entity type. Omit to use the current entity target.",
			},
			"entity_id": optionalIDSchema("Entity ID. Omit to use the current entity target."),
			"content":   map[string]any{"type": "string", "minLength": 1, "description": "Markdown comment body."},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}
}

func listEpicsSchema() map[string]any {
	return boundedListSchema(map[string]any{
		"query":    map[string]any{"type": "string", "description": "Optional case-insensitive text query matched against epic name or description."},
		"team_id":  optionalIDSchema("Optional team ID filter."),
		"state_id": optionalIDSchema("Optional epic workflow state ID filter."),
		"label_id": optionalIDSchema("Optional label ID filter."),
		"archived": map[string]any{"type": "boolean", "description": "Optional archive-state filter."},
	})
}

func epicEditableProperties(forUpdate bool) map[string]any {
	properties := map[string]any{
		"name":           map[string]any{"type": "string", "minLength": 1},
		"description":    map[string]any{"type": "string"},
		"color":          map[string]any{"type": "string"},
		"health":         map[string]any{"type": "string", "enum": []string{model.PMEpicHealthNone, model.PMEpicHealthOnTrack, model.PMEpicHealthAtRisk, model.PMEpicHealthOffTrack}},
		"health_comment": map[string]any{"type": "string"},
	}
	if forUpdate {
		properties["epic_state_id"] = clearableIDSchema("Epic workflow state ID")
		properties["owner_id"] = clearableIDSchema("Workspace user ID")
		properties["owner_member_id"] = clearableIDSchema("Workspace member ID")
		properties["team_id"] = clearableIDSchema("Owning team ID")
		properties["planned_start_date"] = dateSchema("Optional planned start date.", true)
		properties["deadline"] = dateSchema("Optional epic deadline.", true)
		properties["planning_repository_id"] = clearableIDSchema("Planning repository ID")
		properties["label_ids"] = replaceIDArraySchema("Epic label IDs.")
	} else {
		properties["epic_state_id"] = optionalCreateIDSchema("epic workflow state ID")
		properties["owner_id"] = optionalCreateIDSchema("workspace user ID")
		properties["owner_member_id"] = optionalCreateIDSchema("workspace member ID")
		properties["team_id"] = map[string]any{"type": "string", "minLength": 1, "description": "Owning team ID."}
		properties["planned_start_date"] = optionalCreateDateSchema("planned start date")
		properties["deadline"] = optionalCreateDateSchema("epic deadline")
		properties["planning_repository_id"] = optionalCreateIDSchema("planning repository ID")
		properties["label_ids"] = createIDArraySchema("Epic label IDs to attach.")
	}
	return properties
}

func createEpicSchema() map[string]any {
	properties := epicEditableProperties(false)
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{"name", "team_id"},
		"additionalProperties": false,
	}
}

func updateEpicSchema() map[string]any {
	properties := epicEditableProperties(true)
	properties["epic_id"] = optionalIDSchema("Optional epic ID. Omit to use the current epic target.")
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{},
		"additionalProperties": false,
	}
}

func listSprintsSchema() map[string]any {
	return boundedListSchema(map[string]any{
		"query":   map[string]any{"type": "string", "description": "Optional case-insensitive text query matched against sprint name or description."},
		"team_id": optionalIDSchema("Optional team ID filter."),
		"status": map[string]any{
			"type": "string",
			"enum": []string{model.PMSprintStatusUnstarted, model.PMSprintStatusStarted, model.PMSprintStatusDone},
		},
		"archived": map[string]any{"type": "boolean", "description": "Optional archive-state filter."},
	})
}

func createSprintSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string", "minLength": 1},
			"description": map[string]any{"type": "string"},
			"start_date":  dateSchema("Sprint start date.", false),
			"end_date":    dateSchema("Sprint end date.", false),
			"team_id":     optionalIDSchema("Owning team ID."),
			"label_ids":   createIDArraySchema("Sprint label IDs to attach."),
		},
		"required":             []string{"name", "start_date", "end_date", "team_id"},
		"additionalProperties": false,
	}
}

func updateSprintSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sprint_id":   optionalIDSchema("Optional sprint ID. Omit to use the current sprint target."),
			"name":        map[string]any{"type": "string", "minLength": 1},
			"description": map[string]any{"type": "string"},
			"start_date":  dateSchema("Sprint start date.", false),
			"end_date":    dateSchema("Sprint end date.", false),
			"team_id":     optionalIDSchema("Owning team ID."),
			"label_ids":   replaceIDArraySchema("Sprint label IDs."),
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func listObjectivesSchema() map[string]any {
	return boundedListSchema(map[string]any{
		"query":          map[string]any{"type": "string", "description": "Optional case-insensitive text query matched against objective name or description."},
		"team_id":        optionalIDSchema("Optional linked team ID filter."),
		"label_id":       optionalIDSchema("Optional label ID filter."),
		"objective_type": map[string]any{"type": "string", "enum": []string{model.PMObjectiveTypeTactical, model.PMObjectiveTypeStrategic}},
		"state":          map[string]any{"type": "string", "enum": []string{model.PMObjectiveStateNotStarted, model.PMObjectiveStateActive, model.PMObjectiveStateClosed}},
		"archived":       map[string]any{"type": "boolean", "description": "Optional archive-state filter."},
	})
}

func objectiveEditableProperties(forUpdate bool) map[string]any {
	properties := map[string]any{
		"name":           map[string]any{"type": "string", "minLength": 1},
		"description":    map[string]any{"type": "string"},
		"objective_type": map[string]any{"type": "string", "enum": []string{model.PMObjectiveTypeTactical, model.PMObjectiveTypeStrategic}},
		"state":          map[string]any{"type": "string", "enum": []string{model.PMObjectiveStateNotStarted, model.PMObjectiveStateActive, model.PMObjectiveStateClosed}},
		"health":         map[string]any{"type": "string", "enum": []string{model.PMObjectiveHealthOnTrack, model.PMObjectiveHealthAtRisk, model.PMObjectiveHealthOffTrack}},
		"health_comment": map[string]any{"type": "string"},
	}
	if forUpdate {
		properties["planned_start_date"] = dateSchema("Optional planned start date.", true)
		properties["deadline"] = dateSchema("Optional objective deadline.", true)
		properties["team_ids"] = replaceIDArraySchema("Linked team IDs.")
		properties["owner_ids"] = replaceIDArraySchema("Workspace user owner IDs.")
		properties["owner_member_ids"] = replaceIDArraySchema("Workspace member owner IDs.")
		properties["label_ids"] = replaceIDArraySchema("Objective label IDs.")
		properties["epic_ids"] = replaceIDArraySchema("Linked epic IDs.")
	} else {
		properties["state"] = map[string]any{"type": "string", "enum": []string{model.PMObjectiveStateNotStarted, model.PMObjectiveStateActive, model.PMObjectiveStateClosed}, "description": "Optional initial objective state. Omit this field to use not_started."}
		properties["planned_start_date"] = optionalCreateDateSchema("planned start date")
		properties["deadline"] = optionalCreateDateSchema("objective deadline")
		properties["team_ids"] = createIDArraySchema("Team IDs to link.")
		properties["owner_ids"] = createIDArraySchema("Workspace user owner IDs to link.")
		properties["owner_member_ids"] = createIDArraySchema("Workspace member owner IDs to link.")
		properties["label_ids"] = createIDArraySchema("Objective label IDs to attach.")
		properties["epic_ids"] = createIDArraySchema("Epic IDs to link.")
	}
	return properties
}

func createObjectiveSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           objectiveEditableProperties(false),
		"required":             []string{"name", "objective_type"},
		"additionalProperties": false,
	}
}

func updateObjectiveSchema() map[string]any {
	properties := objectiveEditableProperties(true)
	properties["objective_id"] = optionalIDSchema("Optional objective ID. Omit to use the current objective target.")
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             []string{},
		"additionalProperties": false,
	}
}

func createKeyResultSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":          map[string]any{"type": "string", "minLength": 1},
			"result_type":   map[string]any{"type": "string", "enum": []string{model.PMKeyResultTypeBoolean, model.PMKeyResultTypePercent, model.PMKeyResultTypeNumeric}},
			"initial_value": map[string]any{"type": "number"},
			"current_value": map[string]any{"type": "number"},
			"target_value":  map[string]any{"type": "number"},
			"note":          map[string]any{"type": "string"},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

func updateKeyResultSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key_result_id": optionalIDSchema("Key result ID to update."),
			"name":          map[string]any{"type": "string", "minLength": 1},
			"result_type":   map[string]any{"type": "string", "enum": []string{model.PMKeyResultTypeBoolean, model.PMKeyResultTypePercent, model.PMKeyResultTypeNumeric}},
			"initial_value": map[string]any{"type": "number"},
			"current_value": map[string]any{"type": "number"},
			"target_value":  map[string]any{"type": "number"},
			"note":          map[string]any{"type": "string"},
		},
		"required":             []string{"key_result_id"},
		"additionalProperties": false,
	}
}
