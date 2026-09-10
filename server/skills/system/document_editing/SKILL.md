---
name: document_editing
description: Read, search, and edit existing Helpin documents efficiently. Use for wording fixes, section rewrites, adding diagrams or examples, and inspecting long documents without reading block by block.
metadata:
  title: Document Reading and Editing
  required_tools:
    - read_document
    - get_document_blocks
    - edit_document
  supported_runtimes:
    - native_sdk
    - codex
---

# Read once, edit once

Use `read_document` with its default adaptive mode. A fitting document returns full readable blocks and a `version`; a large document returns an outline with heading IDs and inclusive block ranges. The title is stored separately from the document body.

- For a relevant heading, call `get_document_blocks` with `section_id`. This includes subsections up to the next equal or higher heading.
- For a known passage, call `get_document_blocks` with `query`. It searches inside this document, matching all query terms case-insensitively, and includes neighboring blocks. Use short, distinctive terms.
- For a focused block, call it with `anchor_block_id` and `around`, or fetch several known `block_ids` together.
- Use `search_documents` to find documents, then follow the matching block reference. Do not search the workspace again to read more of a known document.
- Use `read_document` with `mode: full` when the entire long document is needed. Follow `next_cursor` using only `document_id` and `cursor`. Oversized readable blocks return `fragment_format: markdown`: read `content_fragment` directly as the next text slice, with the same block ID and a character `fragment_offset`. Slices may split Markdown fences or formatting; do not treat them as standalone replacement content. Only structured `item_json` fragments require concatenating `content_fragment` by `fragment_offset` before decoding.
- `content_text` is the legacy plain-text preview; its `content_text_truncated` flag does not describe the returned full blocks. Read `blocks`, `complete`, and `content_complete` instead.
- `complete` describes the requested selection/page, not proof you read the whole document. An outline describes structure only. Summary reads always have `content_complete: false`, even when every block is listed. Sequential block pages also carry `next_offset` when more blocks exist.
- Ask for `format: json` only when the exact rich block structure is needed. Markdown and embed descriptions are reading representations, not lossless backups.

# Targeted editing

Use `edit_document` with the read's exact `expected_version` and up to 20 operations. Resolve every operation against that original snapshot. Combine inserts at the same boundary. Keep unaffected rich blocks intact rather than rewriting the document from Markdown.

Wording fix (replace example IDs and version with read results):

```json
{"document_id":"doc-id","expected_version":"read-version","operations":[{"type":"replace_text","block_id":"paragraph-id","old_text":"every 30 seconds","new_text":"every 60 seconds"}]}
```

An exact text match must occur once within a paragraph or code block. It may cross adjacent formatting nodes, but not separate paragraphs or embeds. Use a longer match if ambiguous. Text replacements preserve the block ID and surrounding formatting; replacement text inherits formatting from the start of the match.

Section rewrite:

```json
{"document_id":"doc-id","expected_version":"read-version","operations":[{"type":"replace_range","start_block_id":"section-heading-id","end_block_id":"section-last-block-id","content":"## Retry policy\n\nRetry transient errors up to three times."}]}
```

Both endpoints are inclusive. Include the heading when replacing its whole section. Range replacement creates new IDs; later calls use the returned IDs or a fresh read. To retain an embedded block, exclude it from replacement ranges or provide its exact structured content deliberately.

Diagram plus explanation in one insertion:

```json
{"document_id":"doc-id","expected_version":"read-version","operations":[{"type":"insert","after_block_id":"section-last-block-id","content":"```mermaid\nflowchart LR\n  Request --> Queue\n  Queue --> Worker\n```\n\nThe worker consumes queued requests."}]}
```

`insert` also supports `before_block_id` or `position: start|end`. `content` accepts Markdown or an array of TipTap block objects. `delete_range` takes the same inclusive endpoints as `replace_range` and no content.

On a conflict, nothing was applied: reread the affected content, reassess the edit, and retry with its version. After success, use the returned version and changed block revisions; do not reread or repeat a mutation merely to confirm it. An unusually large receipt may be marked `receipt_complete: false`; retrieve only the missing context needed for further work.

Use `write_document_content` for a deliberate full replacement, supplying `expected_version` for existing content. Keep the existing review/proposal workflow when the task requires review; direct editing does not replace publication approval. Inspect authoritative sources before changing factual claims.
