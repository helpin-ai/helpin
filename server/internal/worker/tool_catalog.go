package worker

import "github.com/helpin-ai/helpin/server/internal/model"

// toolCategory maps each tool name to its display category.
var toolCategory = map[string]string{
	// Filesystem
	"read_file":       "Filesystem",
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

	// PM / Stories
	"request_human_input":    "Interaction",
	"request_human_approval": "Interaction",
	"publish_preview":        "Interaction",
	"add_story_comment":      "PM / Stories",
	"update_story_state":     "PM / Stories",
	"list_story_checklist":   "PM / Stories",
	"create_story_batch":     "PM / Stories",
	"assign_story_agent":     "PM / Stories",
	"set_story_dependencies": "PM / Stories",
	"list_epic_stories":      "PM / Stories",
	"approve_epic_spec":      "PM / Stories",
	"list_workspace_teams":   "Workspace",

	// Support
	"list_conversation_messages": "Support",
	"draft_support_reply":        "Support",
	"update_conversation_status": "Support",

	// CRM
	"list_deals":         "CRM",
	"update_deal_stage":  "CRM",
	"add_deal_note":      "CRM",
	"list_contacts":      "CRM",
	"list_buyer_signals": "CRM",

	// Docs
	"list_documents":          "Docs",
	"read_document":           "Docs",
	"search_documents":        "Docs",
	"write_document_content":  "Docs",
	"link_document_to_object": "Docs",
	"ensure_epic_spec_doc":    "Docs",
}

var categoryOrder = []string{
	"Filesystem",
	"Code Analysis",
	"Commands",
	"Web Search",
	"Git",
	"Interaction",
	"PM / Stories",
	"Workspace",
	"Support",
	"CRM",
	"Docs",
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
		cat := toolCategory[def.Name]
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
