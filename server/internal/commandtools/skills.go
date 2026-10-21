package commandtools

func skillUpdateInputSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": map[string]any{
			"skill_id":         map[string]any{"type": "string", "minLength": 1, "description": "Exact skill_id from list_workspace_skills."},
			"expected_version": map[string]any{"type": "string", "minLength": 1, "description": "Exact version_key from the current skill read. Never use placeholders or guess."},
			"title":            map[string]any{"type": "string", "minLength": 1}, "description": map[string]any{"type": "string", "minLength": 1}, "instructions": map[string]any{"type": "string", "minLength": 1}, "restore_default": map[string]any{"type": "boolean"},
		}, "required": []string{"skill_id", "expected_version"}, "additionalProperties": false,
	}
}

var skillManagementTools = []RuntimeToolMetadata{
	{CommandName: "skills.list_workspace", Alias: "list_workspace_skills", Category: "Skills", Description: "Read editable workspace skills, including built-ins and imports. Without a selector, returns paginated metadata. Supply key OR skill_id to read current instructions in pages. Follow next_offset until complete=true; for instruction continuation pass expected_version from the first page. offset/limit count Unicode characters for instructions, entries for catalog lists. To edit another agent's skill use this tool, not read_skill (which only loads your assigned skills). Preserve all existing guidance: never replace instructions from a partial read. This management catalog does not grant skill/tool execution permissions.", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{
			"key": map[string]any{"type": "string"}, "skill_id": map[string]any{"type": "string"},
			"offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 4000, "description": "Default/max 4000 characters for a selected skill, 10 entries for catalog pages."},
			"expected_version": map[string]any{"type": "string", "description": "Required for instruction continuation; use version_key from the first page."},
		}, "additionalProperties": false,
	}},
	{CommandName: "skills.validate_update", Alias: "validate_skill_update", Category: "Skills", Description: "Validate the complete proposed skill edit without saving or requesting mutation approval. First read all current instruction pages using list_workspace_skills; preserve unrelated guidance and use its exact skill_id and version_key. Submit the actual edited fields here, not a probe. After valid=true, call update_skill with these exact arguments. On version mismatch reread the skill and reapply the intended edit. This read-only check does not grant approval or reserve a version.", InputSchema: skillUpdateInputSchema()},
	{CommandName: "skills.update", Alias: "update_skill", Category: "Skills", Description: "Save an existing workspace skill edit. First read all current pages with list_workspace_skills, then validate the actual proposed edit with validate_skill_update. Submit the same validated arguments here for approval and saving. Never probe this mutation with guessed versions, placeholder instructions, or unchanged fields. Omitted fields remain unchanged. restore_default resets to the shipped built-in or original package; use only when requested and do not combine with edited fields. Edits affect subsequent skill loads; host permissions and policies remain enforced.", InputSchema: skillUpdateInputSchema()},
}
