package worker

import (
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// toolCategory maps each tool name to its display category.
var toolCategory = map[string]string{
	// Filesystem
	"read_file":       "Filesystem",
	"read_files":      "Filesystem",
	"write_file":      "Filesystem",
	"list_directory":  "Filesystem",
	"search_files":    "Filesystem",
	"read_file_range": "Filesystem",

	// Code Analysis
	"ripgrep":      "Code Analysis",
	"grep":         "Code Analysis",
	"list_symbols": "Code Analysis",

	// Commands
	"run_command": "Commands",

	// Web Search
	"web_search_brave": "Web Search",

	// Git
	"create_branch":   "Git",
	"commit_and_push": "Git",
	"open_pr":         "Git",

	// PM / Tasks
	"request_user_input":        "Interaction",
	"request_review_checkpoint": "Interaction",
	"request_human_input":       "Interaction",
	"request_human_approval":    "Interaction",
	"update_plan":               "Interaction",
	"preview_md":                "Interaction",
	"preview_json":              "Interaction",
	"publish_prd_draft":         "Interaction",
	"publish_task_plan":         "Interaction",
	"publish_task_plan_doc":     "Interaction",
	"publish_preview":           "Interaction",
	"add_task_comment":          "PM / Tasks",
	"list_task_checklist":       "PM / Tasks",
	"list_epic_tasks":           "PM / Tasks",
	"list_workspace_teams":      "Workspace",

	// Support
	"list_conversation_messages": "Support",
	"draft_support_reply":        "Support",
	"update_conversation_status": "Support",

	// CRM
	"list_deals":         "CRM",
	"list_contacts":      "CRM",
	"list_buyer_signals": "CRM",

	// Docs
	"list_documents":   "Docs",
	"read_document":    "Docs",
	"search_documents": "Docs",
}

var categoryOrder = []string{
	"Filesystem",
	"Code Analysis",
	"Commands",
	"Web Search",
	"Git",
	"Interaction",
	"PM / Tasks",
	"Workspace",
	"Support",
	"CRM",
	"Docs",
}

var hiddenToolCatalogAliases = map[string]bool{
	ToolRequestHumanInput:    true,
	ToolRequestHumanApproval: true,
}

// webSearchDefinition returns the catalog entry for web_search_brave, which is
// conditionally registered in the ToolRegistry only when a WebSearchClient is
// provided. The catalog always includes it so users can see the full tool set.
func webSearchDefinition() model.ToolCatalogEntry {
	return model.ToolCatalogEntry{
		Name:        "web_search_brave",
		Description: "Search the public web with Brave Search. Use this for market context, standards, competitors, and external evidence. Returns normalized JSON results.",
		Category:    "Web Search",
		InputSchema: map[string]interface{}{
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
		},
	}
}

// ListToolCatalog builds the full tool catalog with categories.
func ListToolCatalog() model.ToolCatalogResponse {
	// Get all tool definitions from a temporary registry (nil webSearch).
	registry := NewToolRegistry(nil)
	defs := registry.Definitions()

	seen := make(map[string]bool, len(defs)+1)
	entries := make([]model.ToolCatalogEntry, 0, len(defs)+1)

	for _, def := range defs {
		if hiddenToolCatalogAliases[def.Name] {
			continue
		}
		cat := ""
		if meta, ok := commandtools.ToolMetadataForAlias(def.Name); ok {
			cat = meta.Category
		}
		if cat == "" {
			cat = toolCategory[def.Name]
		}
		if cat == "" {
			cat = "Other"
		}
		entries = append(entries, model.ToolCatalogEntry{
			Name:        def.Name,
			Description: def.Description,
			Category:    cat,
			InputSchema: def.InputSchema,
			Presets:     []string{},
		})
		seen[def.Name] = true
	}

	// Always include web_search_brave even if the WebSearchClient was nil.
	if !seen["web_search_brave"] {
		ws := webSearchDefinition()
		ws.Presets = []string{}
		entries = append(entries, ws)
	}

	return model.ToolCatalogResponse{
		Tools:      entries,
		Categories: categoryOrder,
	}
}
