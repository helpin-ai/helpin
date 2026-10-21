package service

// Read-only child-launch policy for support chat runs. A visitor cannot
// approve anything, so support runs may auto-launch child agents only when
// every step is confined to this server-owned read-only tool set; anything
// broader must be corrected or deferred to background investigation.

import "strings"

// supportChildReadOnlyTools records server-approved reads across support
// research and background investigations. Customer launches also require the
// narrower supportCustomerResearchTools boundary. Never derived from model input.
var supportChildReadOnlyTools = map[string]bool{
	// Focused third-party context research; Echo's prompt governs its purpose.
	"web_search":            true,
	"fetch_url":             true,
	"crawl_url":             true,
	"find_skills":           true,
	"read_skill":            true,
	"get_release_context":   true,
	"get_pull_request_diff": true,
	"list_task_checklist":   true,
	// Repository inspection (bug checks).
	"checkout_repositories": true,
	"list_repositories":     true,
	"list_commits":          true,
	"read_files":            true,
	"list_directory":        true,
	"repository_search":     true,
	"list_symbols":          true,
	"read_symbol":           true,
	"trace_symbol":          true,
	// Docs / knowledge reads.
	"list_documents":      true,
	"read_document":       true,
	"get_document_blocks": true,
	"search_documents":    true,
	"list_collections":    true,
	"list_spaces":         true,
	"search_knowledge":    true,
	// Product reads.
	"list_tasks":                 true,
	"get_task_context":           true,
	"list_workspace_teams":       true,
	"list_conversation_messages": true,
}

const (
	// supportChildMaxConcurrent bounds simultaneously running child plans per
	// conversation; supportChildMaxPerConversation is the lifetime cap.
	supportChildMaxConcurrent      = 2
	supportChildMaxPerConversation = 5
)

// supportStepsAreReadOnly requires explicit non-empty tool lists drawn from
// the server-owned read-only catalog. Customer research adds a separate scope check.
func supportStepsAreReadOnly(steps []dockLaunchStep) bool {
	if len(steps) == 0 {
		return false
	}
	for _, step := range steps {
		if len(step.AllowedTools) == 0 {
			return false
		}
		for _, tool := range step.AllowedTools {
			if !supportChildReadOnlyTools[strings.TrimSpace(tool)] {
				return false
			}
		}
	}
	return true
}

func isSupportWebResearchTool(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "web_search", "fetch_url", "crawl_url":
		return true
	default:
		return false
	}
}

// Customer research excludes implementation investigation, which has its own
// independent post-handover run and private-note delivery contract.
var supportCustomerResearchTools = map[string]bool{
	"web_search": true, "fetch_url": true, "crawl_url": true,
	"find_skills": true, "read_skill": true,
	"search_knowledge": true, "list_documents": true, "read_document": true,
	"get_document_blocks": true, "search_documents": true,
	"list_collections": true, "list_spaces": true,
	"list_conversation_messages": true,
}

func supportStepsAreCustomerResearch(steps []dockLaunchStep) bool {
	if !supportStepsAreReadOnly(steps) {
		return false
	}
	for _, step := range steps {
		for _, tool := range step.AllowedTools {
			if !supportCustomerResearchTools[strings.TrimSpace(tool)] {
				return false
			}
		}
	}
	return true
}
