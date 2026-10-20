package commandtools

var skillManagementTools = []RuntimeToolMetadata{
	{CommandName: "skills.list_workspace", Alias: "list_workspace_skills", Category: "Skills", Description: "List all editable skills in the current workspace, including built-in and imported skills. Supply key or skill_id to read the full current instructions before editing. Without a selector returns compact metadata. This catalog is for management and does not grant permission to invoke skills or their tools.", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"key": map[string]any{"type": "string"}, "skill_id": map[string]any{"type": "string"}}, "additionalProperties": false,
	}},
	{CommandName: "skills.update", Alias: "update_skill", Category: "Skills", Description: "Edit an existing skill in this workspace. Read its current text with list_workspace_skills first and preserve unrelated guidance. Use the built-in key or workspace skill ID returned there. Edits affect subsequent skill loads, not the current run's instructions. Host permissions and policies remain enforced. restore_default resets to the shipped built-in or original package; use only when the user requests restoration. Omitted fields remain unchanged.", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{
			"skill_id":         map[string]any{"type": "string", "description": "Workspace skill ID, or built-in key."},
			"expected_version": map[string]any{"type": "string", "description": "version_key from the full current skill read; prevents overwriting newer edits."},
			"title":            map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "instructions": map[string]any{"type": "string"}, "restore_default": map[string]any{"type": "boolean"},
		}, "required": []string{"skill_id", "expected_version"}, "additionalProperties": false,
	}},
}
