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
		CommandName: "pm.assign_task_agent",
		Alias:       "assign_task_agent",
		Category:    "PM / Tasks",
		Description: "Assign or reassign an agent to an existing task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"story_id": map[string]any{
					"type":        "string",
					"description": "Legacy alias for the task ID to assign",
				},
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
							"source_story_id": map[string]any{"type": "string"},
							"target_story_id": map[string]any{"type": "string"},
							"source_task_id":  map[string]any{"type": "string"},
							"target_task_id":  map[string]any{"type": "string"},
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
			"stories": map[string]any{
				"description": "Legacy compatibility field. The list of tasks to create.",
				"type":        "array",
				"items":       taskSchema,
			},
			"proposed_stories": map[string]any{
				"description": "Legacy compatibility alias for older task-plan payloads. If present, it is treated the same as tasks.",
				"type":        "array",
				"items":       taskSchema,
			},
			"tasks": map[string]any{
				"description": "Preferred field. The list of tasks to create.",
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
