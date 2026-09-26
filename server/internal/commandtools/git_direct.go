package commandtools

var directGitTools = []RuntimeToolMetadata{{
	CommandName: "git.open_pr", Alias: "open_pr", Category: "Git", RiskLevel: RiskLevelSensitive,
	Description: "Open or reuse a pull request for a branch already pushed to an enabled workspace repository. No task is required. Publication is approval-gated.",
	InputSchema: map[string]any{"type": "object", "properties": map[string]any{
		"repository_id": map[string]any{"type": "string"}, "head": map[string]any{"type": "string"}, "base": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}, "body": map[string]any{"type": "string"},
	}, "required": []string{"repository_id", "head", "base", "title", "body"}, "additionalProperties": false},
}}
