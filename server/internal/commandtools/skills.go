package commandtools

func skillUpdateInputSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": map[string]any{
			"skill_id":         map[string]any{"type": "string", "minLength": 1, "description": "Exact skill_id from list_workspace_skills."},
			"expected_version": map[string]any{"type": "string", "minLength": 1, "description": "Exact version_key from the current skill read. Never use placeholders or guess."},
			"title":            map[string]any{"type": "string", "minLength": 1}, "description": map[string]any{"type": "string", "minLength": 1}, "instructions": map[string]any{"type": "string", "minLength": 1}, "restore_default": map[string]any{"type": "boolean"},
			"replacements": map[string]any{"type": "array", "minItems": 1, "maxItems": 20, "description": "Preferred for focused changes: exact uniquely matching passages. All matches refer to the original instructions; overlaps are rejected. Do not combine with instructions.", "items": map[string]any{"type": "object", "properties": map[string]any{"old_text": map[string]any{"type": "string", "minLength": 1}, "new_text": map[string]any{"type": "string"}}, "required": []string{"old_text", "new_text"}, "additionalProperties": false}},
		}, "required": []string{"skill_id", "expected_version"}, "additionalProperties": false,
	}
}

var skillManagementTools = []RuntimeToolMetadata{
	{CommandName: "skills.list_workspace", Alias: "list_workspace_skills", Category: "Skills", Description: "Read editable workspace skills, including built-ins and imports. Without a selector, returns paginated metadata. Supply key OR skill_id to read current instructions in pages. Follow next_read exactly until complete=true. Use section_heading from sections to read only the passage needed for a focused edit. Offsets are relative to the selected section when supplied. offset/limit count Unicode characters for instructions, entries for catalog lists. To edit another agent's skill use this tool, not read_skill (which only loads your assigned skills). Prefer replacements when editing one passage: the server preserves all other instructions. For full-instruction replacement, read every page first. If responses lack contract_version or alternate formats, stop and report inconsistent backend versions instead of guessing offsets. This management catalog does not grant skill/tool execution permissions.", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{
			"key": map[string]any{"type": "string"}, "skill_id": map[string]any{"type": "string"},
			"offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 4000, "description": "Default/max 4000 characters for a selected skill, 10 entries for catalog pages."},
			"expected_version": map[string]any{"type": "string", "description": "Required for instruction continuation; use version_key from the first page."},
			"section_heading":  map[string]any{"type": "string", "description": "Exact Markdown heading from sections, including leading # characters. Reads that section and its subsections."},
		}, "additionalProperties": false,
	}},
	{CommandName: "skills.validate_update", Alias: "validate_skill_update", Category: "Skills", Description: "Prepare and validate an immutable skill edit without changing the skill or granting mutation approval. Read the relevant section with list_workspace_skills, then prefer exact replacements to preserve untouched instructions. Full instructions require reading every page. Supply the exact current version_key. Returns a review and save_input bound to this workspace, user, and run. Review the changes, then pass save_input unchanged to update_skill. Preparation is required; raw edits cannot be saved. restore_default is only for a user-requested restoration.", InputSchema: skillUpdateInputSchema()},
	{CommandName: "skills.update", Alias: "update_skill", Category: "Skills", Description: "Save only an edit already prepared by validate_skill_update. Pass its save_input exactly, including the review summary. Do not supply raw instructions, versions, or patches here. The server checks the preparation, permissions, current version and approval before saving. An expired or stale proposal must be prepared again. Edits affect subsequent skill loads; host policies remain enforced.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"proposal_id": map[string]any{"type": "string", "format": "uuid"}, "summary": map[string]any{"type": "string", "minLength": 1, "description": "Exact review summary returned in save_input."}}, "required": []string{"proposal_id", "summary"}, "additionalProperties": false}},
}
