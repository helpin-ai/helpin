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
	r.register("read_file", "Read the contents of a file at the given path (relative to the workspace root).", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path relative to the workspace root",
			},
		},
		"required": []string{"path"},
	}, toolReadFile)

	r.register("write_file", "Write content to a file at the given path (relative to the workspace root). Creates directories as needed.", map[string]interface{}{
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
		r.register("web_search", "Search the public web for planning research. Use this for market context, standards, competitors, and external evidence. Returns normalized JSON results.", map[string]interface{}{
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
			return r.toolWebSearch(ctx, input)
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
		return "", fmt.Errorf("tool %q is not allowed for capability profile %q", name, ctx.RuntimeProfile.Name)
	}
	return r.Execute(ctx, name, input)
}
