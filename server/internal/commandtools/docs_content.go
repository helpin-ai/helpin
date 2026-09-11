package commandtools

func documentObjectSchema(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func documentString(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

var documentReadTools = []RuntimeToolMetadata{
	{CommandName: "docs.read_document", Alias: "read_document", Category: "Docs",
		Description: "Read a document in one call when it fits. mode=auto (default) returns full readable blocks or a heading index for large documents. mode=full reads all content with next_cursor continuation; mode=outline returns section IDs/ranges. Responses include a version for edit_document. Full content is in blocks; content_text is a legacy preview and its truncation flag does not mean the blocks are incomplete. Follow next_cursor with document_id and cursor only. Do not scan blocks individually.",
		InputSchema: documentObjectSchema(map[string]any{
			"document_id": documentString("Document ID; defaults to the current document target."),
			"mode":        map[string]any{"type": "string", "enum": []string{"auto", "full", "outline"}},
			"cursor":      documentString("Exact next_cursor from this tool. Supply only document_id and cursor when continuing. Oversized readable blocks return Markdown text slices in content_fragment with block id and character offset; read slices directly. Structured item_json fragments require concatenation before decoding."),
		})},
	{CommandName: "docs.get_document_blocks", Alias: "get_document_blocks", Category: "Docs",
		Description: "Read a section, selected blocks, or search inside one document with neighboring context. Select exactly one of section_id, block_ids, anchor_block_id, query, or offset. Selected content defaults to readable Markdown; request format=json only for structured edits. Includes the snapshot version and precise continuation. Search matches all case-insensitive query terms and merges neighbor windows.",
		InputSchema: documentObjectSchema(map[string]any{
			"document_id":     documentString("Document ID; defaults to the current target."),
			"section_id":      documentString("Heading block ID from the outline; includes subsections until the next heading of equal or higher level. Use preamble for the initial content."),
			"block_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 100},
			"anchor_block_id": documentString("Read this block and its siblings together."),
			"query":           map[string]any{"type": "string", "maxLength": 500, "description": "Find blocks containing all query terms, case-insensitively, within this document."},
			"around":          map[string]any{"type": "integer", "minimum": 0, "maximum": 25, "description": "Neighbor count on each side. Defaults to 1 for search and 5 for an anchor."},
			"offset":          map[string]any{"type": "integer", "minimum": 0},
			"limit":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Sequential page size, default 40. Follow next_offset for the next page, or next_cursor to finish a budget-limited page."},
			"format":          map[string]any{"type": "string", "enum": []string{"markdown", "json", "summary"}},
			"include_content": map[string]any{"type": "boolean", "description": "Compatibility option: true returns full node JSON; false returns compact previews. Prefer format."},
			"cursor":          documentString("Exact next_cursor. Continue using document_id and cursor only. Markdown fragments are readable text slices with block id and character offset. Concatenate only item_json fragments before decoding."),
		})},
	{CommandName: "docs.edit_document", Alias: "edit_document", Category: "Docs",
		Description: "Atomically apply up to 20 targeted document edits against expected_version from a read. Prefer this for wording changes, section replacements, and inserting multiple blocks. Targets refer to the same original snapshot. Preserve surrounding rich content; do not rewrite the full document for a local edit. On conflict nothing is applied: reread affected content. Returns committed version and changed block revisions; never repeat a successful edit merely to retrieve its result.",
		InputSchema: documentObjectSchema(map[string]any{
			"document_id":      documentString("Document ID to edit."),
			"expected_version": documentString("Exact document version returned by read_document or get_document_blocks."),
			"operations":       map[string]any{"type": "array", "minItems": 1, "maxItems": 20, "items": documentEditOperationSchema()},
		}, "document_id", "expected_version", "operations")},
}

// Each alternative is a complete operation contract. A flat object with only
// type required lets models omit selectors or mix fields from different edits.
func documentEditOperationSchema() map[string]any {
	nonempty := func(description string) map[string]any {
		result := documentString(description)
		result["minLength"] = 1
		return result
	}
	content := map[string]any{"description": "Markdown or an array of complete TipTap blocks. Range replacement creates new block IDs.", "oneOf": []map[string]any{
		{"type": "string", "minLength": 1},
		{"type": "array", "minItems": 1, "items": map[string]any{"type": "object"}},
	}}
	operation := func(kind string, properties map[string]any, required ...string) map[string]any {
		properties["type"] = map[string]any{"type": "string", "enum": []string{kind}}
		return documentObjectSchema(properties, append([]string{"type"}, required...)...)
	}
	variants := []map[string]any{
		operation("replace_text", map[string]any{
			"block_id": nonempty("Exact block ID from read_document or get_document_blocks in the expected_version snapshot."),
			"old_text": nonempty("Exact text matching once within a paragraph or code block. May span adjacent formatting nodes, never embedded objects or paragraph boundaries."),
			"new_text": documentString("Replacement text; an empty string removes the match."),
		}, "block_id", "old_text", "new_text"),
		operation("replace_range", map[string]any{
			"start_block_id": nonempty("First block of the inclusive range, from the same snapshot."),
			"end_block_id":   nonempty("Last block of the inclusive range."),
			"content":        content,
		}, "start_block_id", "end_block_id", "content"),
		operation("delete_range", map[string]any{
			"start_block_id": nonempty("First block of the inclusive range, from the same snapshot."),
			"end_block_id":   nonempty("Last block of the inclusive range."),
		}, "start_block_id", "end_block_id"),
	}
	for _, anchor := range []string{"before_block_id", "after_block_id", "position"} {
		selector := nonempty("Exact insertion anchor block ID from the same snapshot.")
		if anchor == "position" {
			selector = map[string]any{"type": "string", "enum": []string{"start", "end"}}
		}
		variants = append(variants, operation("insert", map[string]any{anchor: selector, "content": content}, anchor, "content"))
	}
	return map[string]any{"anyOf": variants}
}
