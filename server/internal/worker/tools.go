package worker

import (
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
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

	r.register("read_files", "Read bounded windows from multiple text files in one call. Use this when you need to inspect several files without spending extra tool round-trips.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"files": map[string]interface{}{
				"type":        "array",
				"description": "Files to read. Max 8 files per call.",
				"items": map[string]interface{}{
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
							"description": "Optional maximum number of lines to return for this file. Defaults to 120, max 250.",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		"required": []string{"files"},
	}, toolReadFiles)

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
	r.register(ToolRequestUserInput, "Present structured questions in the interactive run drawer and wait for the human's answer. Use the Codex-style question payload exactly.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type":        "array",
				"description": "Questions to present. Prefer 1-3 focused questions per request.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":       map[string]interface{}{"type": "string", "description": "Stable question identifier."},
						"header":   map[string]interface{}{"type": "string", "description": "Optional short heading shown above the question."},
						"question": map[string]interface{}{"type": "string", "description": "The prompt to answer."},
						"isOther":  map[string]interface{}{"type": "boolean", "description": "When true, allow a freeform Other answer."},
						"isSecret": map[string]interface{}{"type": "boolean", "description": "When true, render the answer field as secret input."},
						"options": map[string]interface{}{
							"type":        "array",
							"description": "Optional mutually exclusive answer choices.",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"label":       map[string]interface{}{"type": "string", "description": "User-facing option label."},
									"description": map[string]interface{}{"type": "string", "description": "Optional short description for the option."},
								},
								"required":             []string{"label"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"id", "question"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"questions"},
		"additionalProperties": false,
	}, toolRequestUserInput)

	r.register(ToolRequestHumanInput, "Legacy alias for request_user_input. Present structured single-select questions in the interactive run drawer and wait for the human's answer.", map[string]interface{}{
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

	r.register(ToolRequestReviewCheckpoint, "Request an inline product review checkpoint in the interactive run drawer and wait for approval or change feedback.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"phase":   map[string]interface{}{"type": "string", "description": "Short product workflow phase label such as prd, stories, or story_doc."},
			"title":   map[string]interface{}{"type": "string", "description": "User-facing title for the checkpoint."},
			"summary": map[string]interface{}{"type": "string", "description": "Optional short review summary."},
		},
		"required":             []string{"title"},
		"additionalProperties": false,
	}, toolRequestReviewCheckpoint)

	r.register(ToolRequestHumanApproval, "Legacy alias for request_review_checkpoint. Request an inline human approval or review checkpoint in the interactive run drawer and wait for approval or change feedback.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"phase":   map[string]interface{}{"type": "string"},
			"title":   map[string]interface{}{"type": "string"},
			"summary": map[string]interface{}{"type": "string"},
		},
		"required":             []string{"title"},
		"additionalProperties": false,
	}, toolRequestHumanApproval)

	r.register("update_plan", "Update the current execution plan for this run. Use this for short working-step checklists, not for PRDs, story plans, or canonical planning documents.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"note": map[string]interface{}{
				"type":        "string",
				"description": "Optional short note about the current state of the plan.",
			},
			"plan": map[string]interface{}{
				"type":        "array",
				"description": "Ordered execution steps. Max 12 steps, with at most one in_progress step.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"step":   map[string]interface{}{"type": "string"},
						"status": map[string]interface{}{"type": "string", "enum": []string{PlanStepPending, PlanStepInProgress, PlanStepCompleted}},
					},
					"required":             []string{"step", "status"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"plan"},
		"additionalProperties": false,
	}, toolUpdatePlan)

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
				"description": "Story plan JSON object with summary and proposed_stories. Pass structured JSON, not a stringified blob.",
				"type":        "object",
				"properties": map[string]interface{}{
					"summary":          map[string]interface{}{"type": "string"},
					"proposed_stories": map[string]interface{}{"type": "array"},
				},
				"required": []string{"summary", "proposed_stories"},
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

	r.register("list_epic_stories", "List all non-archived stories linked to the current epic with name, type, status, estimate, priority, and agent assignment.", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, toolListEpicStories)

	r.registerSharedCommandTools(map[string]ToolFunc{
		"update_story_state":      toolUpdateStoryState,
		"update_deal_stage":       toolUpdateDealStage,
		"add_deal_note":           toolAddDealNote,
		"write_document_content":  toolWriteDocumentContent,
		"link_document_to_object": toolLinkDocumentToObject,
		"ensure_epic_spec_doc":    toolEnsureEpicSpecDoc,
		"ensure_story_plan_doc":   toolEnsureStoryPlanDoc,
		"approve_epic_spec":       toolApproveEpicSpec,
		"create_story_batch":      toolCreateStoryBatch,
		"assign_story_agent":      toolAssignStoryAgent,
		"set_story_dependencies":  toolSetStoryDependencies,
	})

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

func (r *ToolRegistry) registerSharedCommandTools(fns map[string]ToolFunc) {
	registered := make(map[string]struct{}, len(fns))
	for _, meta := range commandtools.AllRuntimeToolMetadata() {
		fn, ok := fns[meta.Alias]
		if !ok {
			panic("missing shared command tool implementation for alias " + meta.Alias)
		}
		r.register(meta.Alias, meta.Description, meta.InputSchema, fn)
		registered[meta.Alias] = struct{}{}
	}

	for alias := range fns {
		if _, ok := registered[alias]; !ok {
			panic("missing shared command tool metadata for alias " + alias)
		}
	}
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
	canonicalName := CanonicalToolName(name)
	if ctx != nil && len(ctx.AllowedTools) > 0 && !ctx.AllowedTools[name] && !ctx.AllowedTools[canonicalName] {
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
