package worker

import (
	"encoding/json"
	"fmt"
)

// ToolRegistry holds all available tool implementations.
type ToolRegistry struct {
	tools     map[string]ToolFunc
	defs      []ToolDefinition
	webSearch WebSearchClient
}

// ToolFunc is a function that executes a tool and returns its result.
type ToolFunc func(ctx *ExecutionContext, input json.RawMessage) (string, error)

// NewToolRegistry creates a registry with all built-in tools.
func NewToolRegistry(webSearch WebSearchClient) *ToolRegistry {
	r := &ToolRegistry{
		tools:     make(map[string]ToolFunc),
		webSearch: webSearch,
	}

	// Filesystem tools
	r.register("read_file", "Read a bounded window of a text file at the given path (relative to the workspace root). Returns the first chunk by default; use offset_line and limit_lines to continue without dumping the whole file.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
			"offset_line": map[string]interface{}{
				"type":        "integer",
				"description": "Optional 1-based line number to start reading from. Defaults to 1.",
			},
			"limit_lines": map[string]interface{}{
				"type":        "integer",
				"description": "Optional maximum number of lines to return. Defaults to 200, max 400.",
			},
		},
		"required": []string{"path"},
	}, toolReadFile)

	r.register("write_file", "Write content to a file at the given path (relative to the workspace root). Use this for new files or full rewrites after reading the current file first. Creates directories as needed.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write",
			},
		},
		"required": []string{"path", "content"},
	}, toolWriteFile)

	r.register("edit_file", "Edit an existing text file by replacing exactly one matching string. Read the file first and include enough surrounding context in old_string so the match is unique.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "Exact text to replace. Must match exactly once in the file, including whitespace.",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "Replacement text. Use an empty string to delete the matched content.",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}, toolEditFile)

	r.register("apply_patch", "Apply coordinated multi-file edits using the structured *** Begin Patch format. Read each existing file first, and use exact context lines so each hunk matches uniquely.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"patch": map[string]interface{}{
				"type":        "string",
				"description": "Patch text in the *** Begin Patch / *** End Patch format.",
			},
		},
		"required": []string{"patch"},
	}, toolApplyPatch)

	r.register("list_directory", "List files and directories at the given path.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path relative to the workspace root (empty string for root)",
			},
		},
		"required": []string{"path"},
	}, toolListDirectory)

	r.register("search_files", "Search for files matching a glob pattern, optionally grep for content.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern (e.g. '**/*.go')",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Optional text/regex to search within matched files",
			},
		},
		"required": []string{"pattern"},
	}, toolSearchFiles)

	r.register("read_file_range", "Read specific line range from a file. Much more token-efficient than read_file for large files.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
			"start_line": map[string]interface{}{
				"type":        "integer",
				"description": "First line number to read (1-based)",
			},
			"end_line": map[string]interface{}{
				"type":        "integer",
				"description": "Last line number to read (1-based, inclusive)",
			},
		},
		"required": []string{"path", "start_line", "end_line"},
	}, toolReadFileRange)

	r.register("ripgrep", "Fast regex code search using ripgrep. Preferred over search_files for content search.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern (regex by default, literal if fixed_strings is true)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Optional subdirectory to search within (relative to workspace root)",
			},
			"file_type": map[string]interface{}{
				"type":        "string",
				"description": "Restrict to file type (e.g. 'go', 'ts', 'py', 'js', 'rust', 'java')",
			},
			"context_lines": map[string]interface{}{
				"type":        "integer",
				"description": "Lines of context around each match (0-5, default 0)",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum result lines to return (default 50, max 200)",
			},
			"case_insensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "Case-insensitive search (default false)",
			},
			"fixed_strings": map[string]interface{}{
				"type":        "boolean",
				"description": "Treat pattern as literal string instead of regex (default false)",
			},
		},
		"required": []string{"pattern"},
	}, toolRipgrep)

	r.register("grep", "Simple text/regex search (Go-native, no external dependencies). Use ripgrep for better performance if available.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern (substring or regex)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Optional subdirectory to search within (relative to workspace root)",
			},
			"include": map[string]interface{}{
				"type":        "string",
				"description": "Filename glob filter (e.g. '*.go', '*.ts')",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum results to return (default 50)",
			},
		},
		"required": []string{"pattern"},
	}, toolGrep)

	r.register("list_symbols", "Extract function, type, and class declarations from a source file. Returns only signature lines with line numbers.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
		},
		"required": []string{"path"},
	}, toolListSymbols)

	// Command tools
	r.register("run_command", "Run an allowlisted command in the workspace directory. Prefer program + args; shell syntax is not supported.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"program": map[string]interface{}{
				"type":        "string",
				"description": "The executable name (for example: go, npm, git)",
			},
			"args": map[string]interface{}{
				"type":        "array",
				"description": "Command arguments as a JSON string array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Deprecated compatibility field. Plain commands only; shell operators are rejected.",
			},
		},
		"required": []string{},
	}, toolRunCommand)

	if webSearch != nil {
		r.register("web_search_brave", "Search the public web with Brave Search. Use this for market context, standards, competitors, and external evidence. Returns normalized JSON results.", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query to run",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default 5, max 10)",
				},
				"freshness": map[string]interface{}{
					"type":        "string",
					"description": "Optional freshness hint such as pd, pw, pm, or py",
				},
				"domain_allowlist": map[string]interface{}{
					"type":        "array",
					"description": "Optional list of domains to prioritize",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"query"},
		}, func(ctx *ExecutionContext, input json.RawMessage) (string, error) {
			return r.toolWebSearchBrave(ctx, input)
		})
	}

	// Git tools
	r.register("create_branch", "Create a new git branch and switch to it.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Branch name",
			},
		},
		"required": []string{"name"},
	}, toolCreateBranch)

	r.register("commit_and_push", "Stage all changes, commit with the given message, and push to origin.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message",
			},
		},
		"required": []string{"message"},
	}, toolCommitAndPush)

	r.register("open_pr", "Open a pull request on the remote repository.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "PR title",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "PR description body",
			},
			"base_branch": map[string]interface{}{
				"type":        "string",
				"description": "Base branch to merge into (defaults to 'main')",
			},
		},
		"required": []string{"title", "body"},
	}, toolOpenPR)

	// Helpin tools
	r.register("request_human_input", "Present structured single-select questions in the interactive run drawer and wait for the human's answer.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":   map[string]interface{}{"type": "string"},
						"type": map[string]interface{}{"type": "string", "enum": []string{QuestionTypeSingleSelect}},
						"text": map[string]interface{}{"type": "string"},
						"options": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"value":    map[string]interface{}{"type": "string"},
									"label":    map[string]interface{}{"type": "string"},
									"freetext": map[string]interface{}{"type": "boolean"},
								},
								"required":             []string{"value", "label"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"id", "text", "options"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"questions"},
		"additionalProperties": false,
	}, toolRequestHumanInput)

	r.register("request_human_approval", "Request an inline human approval or review checkpoint in the interactive run drawer and wait for approval or change feedback.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"phase":   map[string]interface{}{"type": "string"},
			"title":   map[string]interface{}{"type": "string"},
			"summary": map[string]interface{}{"type": "string"},
		},
		"required":             []string{"title"},
		"additionalProperties": false,
	}, toolRequestHumanApproval)

	r.register("preview_md", "Publish a markdown preview into a named review slot in the interactive run drawer right pane.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"slot":    map[string]interface{}{"type": "string"},
			"title":   map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"slot", "content"},
		"additionalProperties": false,
	}, toolPreviewMarkdown)

	r.register("preview_json", "Publish a JSON preview into a named review slot in the interactive run drawer right pane.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"slot": map[string]interface{}{
				"type": "string",
			},
			"title": map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{
				"description": "Structured JSON content for the preview slot.",
				"oneOf": []map[string]interface{}{
					{"type": "object"},
					{"type": "array"},
					{"type": "number"},
					{"type": "boolean"},
					{"type": "null"},
					{"type": "string"},
				},
			},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"slot", "content"},
		"additionalProperties": false,
	}, toolPreviewJSON)

	r.register("publish_prd_draft", "Publish the current PRD markdown draft for epic planner review.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title":   map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}, toolPublishPRDDraft)

	r.register("publish_story_plan", "Publish the current epic story plan JSON for review.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{
				"description": "Story plan JSON content, typically including summary and proposed_stories.",
				"oneOf": []map[string]interface{}{
					{"type": "object"},
					{"type": "array"},
					{"type": "number"},
					{"type": "boolean"},
					{"type": "null"},
					{"type": "string"},
				},
			},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}, toolPublishStoryPlan)

	r.register("publish_story_plan_doc", "Publish the current story planning document markdown for review.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title":   map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}, toolPublishStoryPlanDoc)

	r.register("publish_preview", "Publish a structured preview panel in the interactive run drawer right pane. Use this for markdown drafts and JSON plans that should be reviewed separately from the main chat.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"panel_key": map[string]interface{}{"type": "string"},
			"title":     map[string]interface{}{"type": "string"},
			"format":    map[string]interface{}{"type": "string", "enum": []string{PreviewFormatMarkdown, PreviewFormatJSON}},
			"content": map[string]interface{}{
				"description": "Panel content. Use a string for markdown previews or any JSON value for json previews.",
				"oneOf": []map[string]interface{}{
					{"type": "string"},
					{"type": "object"},
					{"type": "array"},
					{"type": "number"},
					{"type": "boolean"},
					{"type": "null"},
				},
			},
			"replace": map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"panel_key", "title", "format", "content"},
		"additionalProperties": false,
	}, toolPublishPreview)

	r.register("add_story_comment", "Add a comment to the current story visible in Helpin.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The comment text (supports markdown)",
			},
		},
		"required": []string{"content"},
	}, toolAddStoryComment)

	r.register("update_story_state", "Transition the current story to a different workflow state.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"state_id": map[string]interface{}{
				"type":        "string",
				"description": "The target workflow state ID",
			},
		},
		"required": []string{"state_id"},
	}, toolUpdateStoryState)

	r.register("list_story_checklist", "List the checklist items for the current story.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolListStoryChecklist)

	r.register("list_workspace_teams", "List workspace teams that the agent can use for team selection or planning context.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolListWorkspaceTeams)

	r.register("list_conversation_messages", "List the current support conversation messages.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolListConversationMessages)

	r.register("draft_support_reply", "Draft a support reply for later human approval.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The reply content to send after approval",
			},
			"is_internal": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether this should be saved as an internal-only note",
			},
			"sender_display_name": map[string]interface{}{
				"type":        "string",
				"description": "Optional display name for the drafted response",
			},
		},
		"required": []string{"content"},
	}, toolDraftSupportReply)

	r.register("update_conversation_status", "Transition the current support conversation to a different status.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "The target conversation status",
			},
		},
		"required": []string{"status"},
	}, toolUpdateConversationStatus)

	// CRM tools
	r.register("list_deals", "List CRM deals in the workspace. Returns deal name, stage, and amount.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of deals to return (default 20, max 50)",
			},
		},
	}, toolListDeals)

	r.register("update_deal_stage", "Move a CRM deal to a different pipeline stage.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"deal_id": map[string]interface{}{
				"type":        "string",
				"description": "The deal ID to update",
			},
			"stage_id": map[string]interface{}{
				"type":        "string",
				"description": "The target pipeline stage ID",
			},
		},
		"required": []string{"deal_id", "stage_id"},
	}, toolUpdateDealStage)

	r.register("add_deal_note", "Add a note or comment to a CRM deal.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"deal_id": map[string]interface{}{
				"type":        "string",
				"description": "The deal ID to add a note to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The note content",
			},
		},
		"required": []string{"deal_id", "content"},
	}, toolAddDealNote)

	r.register("list_contacts", "List CRM contacts in the workspace. Returns name, email, and job title.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of contacts to return (default 20, max 50)",
			},
		},
	}, toolListContacts)

	r.register("list_buyer_signals", "List detected buyer signals from emails, meetings, and support conversations.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"deal_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional deal ID to filter signals for a specific deal",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of signals to return (default 20, max 50)",
			},
		},
	}, toolListBuyerSignals)

	// Docs tools
	r.register("list_documents", "List documents in the workspace, optionally filtered by space.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"space_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional space ID to filter documents",
			},
		},
	}, toolListDocuments)

	r.register("read_document", "Read the metadata of a specific document by ID.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"document_id": map[string]interface{}{
				"type":        "string",
				"description": "The document ID to read",
			},
		},
		"required": []string{"document_id"},
	}, toolReadDocument)

	r.register("search_documents", "Search documents by keyword across the workspace.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum results to return (default 10, max 20)",
			},
		},
		"required": []string{"query"},
	}, toolSearchDocuments)

	r.register("write_document_content", "Write document content to a document in Helpin Docs. Accepts either structured document JSON or a markdown string, which will be auto-converted.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"document_id": map[string]interface{}{
				"type":        "string",
				"description": "The document ID to update",
			},
			"content": map[string]interface{}{
				"description": "The document content to save. Use either a structured document JSON object or a markdown string.",
				"oneOf": []map[string]interface{}{
					{"type": "object"},
					{"type": "string"},
				},
			},
		},
		"required": []string{"document_id", "content"},
	}, toolWriteDocumentContent)

	r.register("link_document_to_object", "Create a Helpin Docs link between a document and another internal object.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"document_id": map[string]interface{}{
				"type":        "string",
				"description": "The document ID to link",
			},
			"linked_object_type": map[string]interface{}{
				"type":        "string",
				"description": "The linked object type such as epic or story",
			},
			"linked_object_id": map[string]interface{}{
				"type":        "string",
				"description": "The linked object ID",
			},
			"link_context": map[string]interface{}{
				"type":        "string",
				"description": "Optional link context, defaults to attached",
			},
		},
		"required": []string{"document_id", "linked_object_type", "linked_object_id"},
	}, toolLinkDocumentToObject)

	r.register("ensure_epic_spec_doc", "Create or load the canonical product spec document for the current epic. Returns document metadata, whether an approved spec exists, the current story count, and a planning_hint for branching.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolEnsureEpicSpecDoc)

	r.register("ensure_story_plan_doc", "Create or load the canonical planning document for the current story. Returns document metadata and whether a draft already exists.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolEnsureStoryPlanDoc)

	r.register("approve_epic_spec", "Mark the current epic spec document as approved and record the approved spec version on the epic.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"version_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional existing document version ID to approve. Omit to approve the current document content.",
			},
		},
	}, toolApproveEpicSpec)

	fileChangeSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":        map[string]interface{}{"type": "string"},
			"action":      map[string]interface{}{"type": "string", "enum": []string{"create", "modify", "delete"}},
			"description": map[string]interface{}{"type": "string"},
		},
		"required":             []string{"path", "action", "description"},
		"additionalProperties": false,
	}
	implementationBriefSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"approach": map[string]interface{}{"type": "string"},
			"files_to_modify": map[string]interface{}{
				"type":  "array",
				"items": fileChangeSchema,
			},
			"test_strategy": map[string]interface{}{
				"anyOf": []map[string]interface{}{
					{"type": "string"},
					{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
			},
			"vertical_layers": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"depends_on_files": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required":             []string{"approach", "files_to_modify", "test_strategy"},
		"additionalProperties": false,
	}

	r.register("create_story_batch", "Create implementation-ready stories for the current epic. Supports stable refs, direct assignment, and dependency refs.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"stories": map[string]interface{}{
				"description": "Preferred field. The list of stories to create.",
				"type":        "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ref":         map[string]interface{}{"type": "string"},
						"name":        map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"story_type":  map[string]interface{}{"type": "string"},
						"estimate":    map[string]interface{}{"type": "integer"},
						"priority":    map[string]interface{}{"type": "string"},
						"acceptance_criteria": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"dependency_refs": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"source_refs": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "object"},
						},
						"assign_agent_id":      map[string]interface{}{"type": "string"},
						"slice_type":           map[string]interface{}{"type": "string"},
						"implementation_brief": implementationBriefSchema,
					},
					"required": []string{"name", "description", "story_type"},
				},
			},
			"proposed_stories": map[string]interface{}{
				"description": "Compatibility alias for story-plan payloads. If present, it is treated the same as stories.",
				"type":        "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ref":         map[string]interface{}{"type": "string"},
						"name":        map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"story_type":  map[string]interface{}{"type": "string"},
						"estimate":    map[string]interface{}{"type": "integer"},
						"priority":    map[string]interface{}{"type": "string"},
						"acceptance_criteria": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"dependency_refs": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"source_refs": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "object"},
						},
						"assign_agent_id":      map[string]interface{}{"type": "string"},
						"slice_type":           map[string]interface{}{"type": "string"},
						"implementation_brief": implementationBriefSchema,
					},
					"required": []string{"name", "description", "story_type"},
				},
			},
		},
	}, toolCreateStoryBatch)

	r.register("assign_story_agent", "Assign or reassign an agent to an existing story.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"story_id": map[string]interface{}{
				"type":        "string",
				"description": "The story ID to assign",
			},
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "The target agent ID",
			},
		},
		"required": []string{"story_id", "agent_id"},
	}, toolAssignStoryAgent)

	r.register("list_epic_stories", "List all non-archived stories linked to the current epic with name, type, status, estimate, priority, and agent assignment.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolListEpicStories)

	r.register("set_story_dependencies", "Create explicit story dependency links between existing stories.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dependencies": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source_story_id": map[string]interface{}{"type": "string"},
						"target_story_id": map[string]interface{}{"type": "string"},
					},
					"required": []string{"source_story_id", "target_story_id"},
				},
			},
		},
		"required": []string{"dependencies"},
	}, toolSetStoryDependencies)

	return r
}

func (r *ToolRegistry) register(name, description string, schema interface{}, fn ToolFunc) {
	r.tools[name] = fn
	r.defs = append(r.defs, ToolDefinition{
		Name:        name,
		Description: description,
		InputSchema: schema,
	})
}

// Definitions returns all tool definitions for the Claude API.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	return r.defs
}

// DefinitionsFor returns only the tool definitions allowed by the runtime profile.
func (r *ToolRegistry) DefinitionsFor(allowed map[string]bool) []ToolDefinition {
	if len(allowed) == 0 {
		return nil
	}

	filtered := make([]ToolDefinition, 0, len(allowed))
	for _, def := range r.defs {
		if allowed[def.Name] {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx *ExecutionContext, name string, input json.RawMessage) (string, error) {
	fn, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return fn(ctx, input)
}

// ExecuteAllowed runs a tool by name only if it is enabled for the execution context.
func (r *ToolRegistry) ExecuteAllowed(ctx *ExecutionContext, name string, input json.RawMessage) (string, error) {
	if len(ctx.AllowedTools) > 0 && !ctx.AllowedTools[name] {
		if ctx == nil {
			return "", fmt.Errorf("tool %q is not allowed", name)
		}
		if ctx.Agent != nil && ctx.Agent.Name != "" {
			return "", fmt.Errorf("tool %q is not allowed for agent %q", name, ctx.Agent.Name)
		}
		return "", fmt.Errorf("tool %q is not allowed for the current agent policy", name)
	}
	return r.Execute(ctx, name, input)
}
