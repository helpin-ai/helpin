package service

// Read-only child-launch policy for support chat runs. A visitor cannot
// approve anything, so support runs may auto-launch child agents only when
// every step is confined to this server-owned read-only tool set; anything
// broader pauses for a teammate approval from the inbox.

import "strings"

// supportChildReadOnlyTools is the server-owned allowlist of tools a
// support-launched child run may carry without human approval. Never derived
// from model input.
var supportChildReadOnlyTools = map[string]bool{
	// Repository inspection (bug checks).
	"checkout_repository":   true,
	"checkout_repositories": true,
	"list_repositories":     true,
	"list_commits":          true,
	"read_file":             true,
	"read_files":            true,
	"read_file_range":       true,
	"list_directory":        true,
	"search_files":          true,
	"ripgrep":               true,
	"grep":                  true,
	"list_symbols":          true,
	// Docs / knowledge reads.
	"list_documents":   true,
	"read_document":    true,
	"get_document_blocks": true,
	"search_documents": true,
	"list_collections": true,
	"list_spaces":      true,
	"search_knowledge": true,
	// Product reads.
	"list_tasks":                 true,
	"get_task_context":           true,
	"list_workspace_teams":       true,
	"list_conversation_messages": true,
	// Web research.
	"fetch_url":       true,
	"crawl_url":       true,
	"web_search_brave": true,
	"web_search_exa":   true,
}

const (
	// supportChildMaxConcurrent bounds simultaneously running child plans per
	// conversation; supportChildMaxPerConversation is the lifetime cap.
	supportChildMaxConcurrent      = 2
	supportChildMaxPerConversation = 5
)

// supportStepsAreReadOnly reports whether every step is safely auto-
// approvable: an explicit non-empty allowed_tools list that is a subset of
// the read-only allowlist.
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
