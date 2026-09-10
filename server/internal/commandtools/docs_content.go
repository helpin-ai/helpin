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
			"operations": map[string]any{"type": "array", "minItems": 1, "maxItems": 20, "items": documentObjectSchema(map[string]any{
				"type":            map[string]any{"type": "string", "enum": []string{"replace_text", "replace_range", "insert", "delete_range"}},
				"block_id":        documentString("Required for replace_text."),
				"old_text":        documentString("Exact text matching once within a paragraph or code block. May span adjacent formatting nodes, never embedded objects or paragraph boundaries."),
				"new_text":        documentString("Replacement text, including an empty string to remove the match."),
				"start_block_id":  documentString("First block of the inclusive range, from the same snapshot."),
				"end_block_id":    documentString("Last block of the inclusive range."),
				"before_block_id": documentString("Insert immediately before this block; choose exactly one insertion anchor."),
				"after_block_id":  documentString("Insert immediately after this block."),
				"position":        map[string]any{"type": "string", "enum": []string{"start", "end"}},
				"content":         map[string]any{"description": "Required for insert and replace_range: Markdown or an array of complete TipTap blocks. Range replacement creates new block IDs.", "oneOf": []map[string]any{{"type": "string"}, {"type": "array", "items": map[string]any{"type": "object"}}}},
			}, "type")},
		}, "document_id", "expected_version", "operations")},
}
