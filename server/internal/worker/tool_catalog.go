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
	"web_search_exa":   "Web Search",

	// Git
	"create_branch":   "Git",
	"commit_and_push": "Git",
	"open_pr":         "Git",

	// PM / Tasks
	"request_approval":                "Interaction",
	"request_user_input":              "Interaction",
	"request_review_checkpoint":       "Interaction",
	"request_human_input":             "Interaction",
	"request_human_approval":          "Interaction",
	"update_plan":                     "Interaction",
	"preview_md":                      "Interaction",
	"preview_json":                    "Interaction",
	"publish_prd_draft":               "Interaction",
	"publish_task_plan":               "Interaction",
	"publish_task_plan_doc":           "Interaction",
	"publish_preview":                 "Interaction",
	"add_task_comment":                "PM / Tasks",
	"list_task_checklist":             "PM / Tasks",
	"list_epic_tasks":                 "PM / Tasks",
	"list_team_workflows_with_stages": "PM / Tasks",
	"list_workspace_teams":            "Workspace",

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
	"list_collections": "Docs",
	"read_document":    "Docs",
	"search_documents": "Docs",
	"create_document":  "Docs",
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

// webSearchDefinition returns the catalog entry for a conditionally registered
// web search tool. The catalog always includes it so users can see the full tool set.
func webSearchDefinition(name, description string, schema map[string]interface{}) model.ToolCatalogEntry {
	return model.ToolCatalogEntry{
		Name:        name,
		Description: description,
		Category:    "Web Search",
		InputSchema: schema,
	}
}

// ListToolCatalog builds the full tool catalog with categories.
func ListToolCatalog() model.ToolCatalogResponse {
	// Get all tool definitions from a temporary registry (nil webSearch).
	registry := NewToolRegistry(nil)
	defs := registry.Definitions()

	seen := make(map[string]bool, len(defs)+2)
	entries := make([]model.ToolCatalogEntry, 0, len(defs)+2)

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

	// Always include web search tools even if the backing clients were nil.
	if !seen["web_search_brave"] {
		ws := webSearchDefinition("web_search_brave", webSearchBraveToolDescription(), webSearchBraveToolSchema())
		ws.Presets = []string{}
		entries = append(entries, ws)
	}
	if !seen["web_search_exa"] {
		ws := webSearchDefinition("web_search_exa", webSearchExaToolDescription(), webSearchExaToolSchema())
		ws.Presets = []string{}
		entries = append(entries, ws)
	}

	return model.ToolCatalogResponse{
		Tools:      entries,
		Categories: categoryOrder,
	}
}
