package commandtools

import "strings"

type RuntimeToolMetadata struct {
	CommandName string
	Alias       string
	Category    string
	Description string
	InputSchema map[string]any
}

func ToolMetadataForAlias(alias string) (*RuntimeToolMetadata, bool) {
	alias = strings.TrimSpace(alias)
	for _, meta := range sharedRuntimeTools {
		if meta.Alias == alias {
			copied := meta
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
			return &copied, true
		}
	}
	return nil, false
}

func AllRuntimeToolMetadata() []RuntimeToolMetadata {
	all := make([]RuntimeToolMetadata, len(sharedRuntimeTools))
	copy(all, sharedRuntimeTools)
	return all
}

var sharedRuntimeTools = []RuntimeToolMetadata{
	{
		CommandName: "docs.ensure_spec_doc",
		Alias:       "ensure_epic_spec_doc",
		Category:    "Docs",
		Description: "Create or load the canonical product spec document for the current epic. Returns document metadata, whether an approved spec exists, the current task count, and a planning_hint for branching.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		CommandName: "docs.ensure_task_plan_doc",
		Alias:       "ensure_task_plan_doc",
		Category:    "Docs",
		Description: "Create or load the canonical planning document for the current task. Returns document metadata and whether a draft already exists.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		CommandName: "pm.approve_epic_spec",
		Alias:       "approve_epic_spec",
		Category:    "PM / Tasks",
		Description: "Mark the current epic spec document as approved and record the approved spec version on the epic.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version_id": map[string]any{
					"type":        "string",
					"description": "Optional existing document version ID to approve. Omit to approve the current document content.",
				},
			},
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
		Description: "List tasks in the current workspace with optional label, team, open-only, description, and comment filters.",
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
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"source_task_id": map[string]any{"type": "string"},
							"target_task_id": map[string]any{"type": "string"},
						},
					},
				},
			},
			"required": []string{"dependencies"},
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
		CommandName: "docs.write_document_content",
		Alias:       "write_document_content",
		Category:    "Docs",
		Description: "Write document content to a document in Helpin Docs. Accepts either structured document JSON or a markdown string, which will be auto-converted.",
		InputSchema: map[string]any{
			"type": "object",
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
		CommandName: "docs.create_document",
		Alias:       "create_document",
		Category:    "Docs",
		Description: "Create a new document in Helpin Docs. Accepts optional markdown content that will be auto-converted to rich text.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space_id": map[string]any{
					"type":        "string",
					"description": "The space ID where the document will be created",
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
					"description": "Optional icon for the document",
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Optional tags for the document",
				},
			},
			"required":             []string{"space_id", "title"},
			"additionalProperties": false,
		},
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
				},
				"linked_object_id": map[string]any{
					"type":        "string",
					"description": "The linked object ID",
				},
				"link_context": map[string]any{
					"type":        "string",
					"description": "Optional link context, defaults to attached",
				},
			},
			"required": []string{"document_id", "linked_object_type", "linked_object_id"},
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

func createTaskSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Task title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Optional task description",
			},
			"task_type": map[string]any{
				"type":        "string",
				"description": "Optional task type such as feature, bug, or chore",
			},
			"estimate": map[string]any{
				"type":        "integer",
				"description": "Optional estimate value",
			},
			"priority": map[string]any{
				"type":        "string",
				"description": "Optional priority such as low, medium, high, or urgent",
			},
			"epic_id": map[string]any{
				"type":        "string",
				"description": "Optional epic ID to link the task to",
			},
			"team_id": map[string]any{
				"type":        "string",
				"description": "Team ID that owns the task",
			},
			"workflow_id": map[string]any{
				"type":        "string",
				"description": "Optional workflow ID override. Defaults to the resolved team workflow.",
			},
			"state_id": map[string]any{
				"type":        "string",
				"description": "Optional workflow state ID override. Defaults to the resolved workflow default state.",
			},
			"owner_member_id": map[string]any{
				"type":        "string",
				"description": "Optional workspace member ID to assign as owner",
			},
			"label_ids": map[string]any{
				"type":        "array",
				"description": "Optional label IDs to attach to the task",
				"items":       map[string]any{"type": "string"},
			},
			"deadline": map[string]any{
				"type":        "string",
				"description": "Optional deadline as YYYY-MM-DD or RFC3339",
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
			"label_id": map[string]any{
				"type":        "string",
				"description": "Optional label ID filter.",
			},
			"team_id": map[string]any{
				"type":        "string",
				"description": "Optional team ID filter.",
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
			},
		},
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
