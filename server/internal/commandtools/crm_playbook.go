package commandtools

func crmPlaybookActionSchema() map[string]any {
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	id := func() map[string]any { return map[string]any{"type": "string", "format": "uuid"} }
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	return object(map[string]any{
		"version": map[string]any{"type": "integer", "const": 1}, "kind": map[string]any{"type": "string", "enum": []string{"email", "task", "handoff", "milestone", "deal_stage"}},
		"title": text(240), "reason": text(2000),
		"email":      object(map[string]any{"account_id": map[string]any{"type": "string", "maxLength": 36, "description": "Authorized mailbox ID, or empty until the reviewer selects their sender."}, "contact_id": id(), "to": text(320), "subject": text(500), "body_html": text(50000)}, "account_id", "contact_id", "to", "subject", "body_html"),
		"task":       object(map[string]any{"team_id": id(), "owner_member_id": id(), "name": text(240), "description": text(10000), "deadline": map[string]any{"type": "string", "format": "date-time"}}, "team_id", "owner_member_id", "name", "description"),
		"handoff":    object(map[string]any{"receiving_member_id": id(), "summary": text(4000)}, "receiving_member_id", "summary"),
		"milestone":  object(map[string]any{"key": text(64), "evidence": text(2000)}, "key", "evidence"),
		"deal_stage": object(map[string]any{"deal_id": id(), "stage_id": id()}, "deal_id", "stage_id"),
	}, "version", "kind", "title", "reason")
}
