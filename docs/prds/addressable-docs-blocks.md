# Addressable docs blocks

**Status:** Draft
**Date:** 2026-04-29
**Owners:** Docs, AI Platform, Support
**Primary areas:** Docs, Help Center, Support AI, Agents, Search

---

## 1. Product Thesis

Helpin Docs should become precise enough for humans, support workflows, and agents to refer to and update specific parts of a document without rewriting the entire article.

Today a document is effectively one JSON aggregate in `docs_contents`. This works for the current editor, publishing, imports, search, translations, and agent writes, but it makes granular workflows difficult:

- Agents must read or rewrite whole documents.
- Support coverage suggestions cannot safely target one section.
- Search and RAG can cite articles, but not exact source sections.
- Links from CRM, PM, support, and automation attach to documents rather than the exact relevant passage.
- Future collaborative editing or review workflows have no durable section identity.

Introduce stable, addressable document blocks as the canonical editable content layer while preserving the existing full-document API and downstream behavior during migration.

The product goal is:

> Make every meaningful section of Helpin Docs stable, citable, and individually editable without breaking existing docs workflows.

---

## 2. Goals

1. Store document content as ordered, stable block rows.
2. Preserve existing document reads, saves, rendering, versions, translations, publishing, public help center behavior, imports, and search during migration.
3. Let agents read exact block IDs and update individual blocks.
4. Let support, CRM, PM, and automation links target exact blocks while existing document-level links continue to work.
5. Let embedding and search pipelines cite exact source blocks or small block ranges.
6. Keep the current editor and full-document autosave path initially.
7. Provide a safe migration path from aggregate-only documents to block-native documents.

---

## 3. Non-Goals

1. Real-time collaborative editing is out of scope for v1.
2. Edgeless canvas, whiteboard, and spatial document behavior are out of scope.
3. Nested block editing is out of scope for v1. Lists, tables, callouts, and other complex nodes are addressable as top-level blocks.
4. Replacing `docs_contents` everywhere is out of scope for the first release.
5. Changing public help center routes is out of scope.
6. Rewriting the Docs editor UI is out of scope.
7. Per-character or ProseMirror step-level persistence is out of scope.

---

## 4. Current System

### 4.1 Content Storage

Document metadata lives in `docs_documents`.

Editable content lives in `docs_contents`:

- `document_id`
- `content jsonb`
- `content_text`
- `word_count`
- import provenance fields

The current save path writes whole TipTap/ProseMirror JSON to `docs_contents`, extracts `content_text`, updates `word_count`, and touches the parent document.

### 4.2 Current Dependencies On Aggregate Content

The following systems currently depend on full-document content:

- Docs editor initial load and autosave.
- Document duplicate.
- Version snapshots and revert.
- Internal and external publishing.
- Help center public rendering.
- Help center translation source refresh.
- Help center article publications.
- Import and reconversion flows.
- Asset deletion and asset reference scanning.
- Docs search.
- Docs embedding chunk generation.
- Support AI knowledge retrieval.
- Support coverage draft generation.
- Agent document tools and internal commands.
- PM shortcut import docs.

### 4.3 Current Agent Tool Shape

Current relevant tools:

- `list_documents`
- `list_collections`
- `read_document`
- `search_documents`
- `create_document`
- `write_document_content`
- `link_document_to_object`

`write_document_content` is whole-document only. `read_document` primarily returns metadata and `content_text`, not structured blocks.

---

## 5. Product Requirements

### 5.1 Stable Block Identity

Every top-level editable block in a document must have a stable ID.

Requirements:

- Block IDs must survive ordinary typing, formatting, title changes, image uploads, and autosaves.
- New blocks created by Enter, slash commands, paste, import, or agent insertion must receive new IDs.
- Duplicate block IDs must be corrected before persistence.
- Block IDs must be scoped to a document and workspace.
- The frontend should store the block ID in TipTap node attributes as `attrs.blockId`.
- The backend must validate and repair missing or invalid block IDs on every content write.

### 5.2 Canonical Block Storage

`docs_blocks` becomes the canonical editable content layer.

V1 interpretation:

- One active row per top-level TipTap document child.
- The block row `content` stores the full top-level node JSON.
- Complex nested content remains inside the block JSON.
- `parent_id` is included for future nested-addressable blocks, but should usually be null in v1.
- Ordering is controlled by `sort_key`.

### 5.3 Compatibility Aggregate

`docs_contents` remains as a compatibility aggregate.

Requirements:

- Existing `GET /docs/documents/{docId}/content` returns full document JSON.
- Existing whole-document saves still work.
- Existing public help center rendering still reads compatible aggregate JSON.
- After any block mutation, the backend regenerates:
  - `docs_contents.content`
  - `docs_contents.content_text`
  - `docs_contents.word_count`
  - `docs_documents.updated_at`

### 5.4 Block-Aware Links

Links from documents to product objects should optionally target blocks.

Requirements:

- Add nullable `block_id` to `docs_links`.
- Existing links with `block_id = null` remain document-level links.
- New links may point to a specific block.
- Reverse lookup should include document title and, when applicable, a block preview.
- If a block is soft-deleted, active link lists should either hide the block detail or mark it as deleted while retaining the document-level association.

### 5.5 Block-Aware Retrieval

Docs chunks should support block-level citations.

Requirements:

- Add nullable `block_id` to `docs_chunks`.
- Add nullable `block_range jsonb` for grouped chunks.
- Embedding sync should generate chunks from ordered blocks or small adjacent block groups.
- Search and support AI results should carry block citation metadata.
- Support AI answers should be able to cite the exact section that grounded an answer.

### 5.6 Agent Block Editing

Agents should be able to inspect and modify individual blocks.

Requirements:

- Agents can list blocks in a document.
- Agents can read a single block by ID.
- Agents can update a block without rewriting the whole document.
- Agents can insert, delete, and reorder blocks.
- Agent block mutations must enforce docs permissions, locks, workspace scope, and document existence.
- Existing whole-document agent tools remain available for compatibility.

---

## 6. Data Model

### 6.1 `docs_blocks`

```sql
CREATE TABLE IF NOT EXISTS docs_blocks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    document_id uuid NOT NULL,
    parent_id uuid NULL,
    type text NOT NULL,
    content jsonb NOT NULL DEFAULT '{}'::jsonb,
    sort_key text NOT NULL DEFAULT '~',
    authored_by uuid NULL,
    last_edited_by uuid NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz NULL
);
```

Required indexes:

- `(workspace_id, document_id, deleted_at, sort_key)`
- `(document_id, id)`
- `(parent_id)`
- `(workspace_id, document_id, type)`

Recommended constraints:

- `type <> ''`
- `sort_key <> ''`
- `parent_id IS NULL OR parent_id <> id`

Foreign keys should reference `docs_documents(id)` and `docs_blocks(id)` where feasible. If existing delete flows rely on manual cleanup, use `ON DELETE CASCADE` only after confirming deletion semantics.

### 6.2 `docs_links`

Add:

```sql
ALTER TABLE docs_links
    ADD COLUMN IF NOT EXISTS block_id uuid NULL;
```

Indexes:

- `(workspace_id, block_id)` where `block_id IS NOT NULL`
- `(document_id, block_id)` where `block_id IS NOT NULL`

### 6.3 `docs_chunks`

Add:

```sql
ALTER TABLE docs_chunks
    ADD COLUMN IF NOT EXISTS block_id uuid NULL,
    ADD COLUMN IF NOT EXISTS block_range jsonb NULL;
```

Indexes:

- `(document_id, block_id)` where `block_id IS NOT NULL`

`block_range` shape:

```json
{
  "start_block_id": "uuid",
  "end_block_id": "uuid",
  "block_ids": ["uuid"]
}
```

---

## 7. Block JSON Contract

### 7.1 Aggregate Document

Stored aggregate content remains:

```json
{
  "type": "doc",
  "content": [
    {
      "type": "paragraph",
      "attrs": {
        "blockId": "block-uuid"
      },
      "content": [
        { "type": "text", "text": "Example" }
      ]
    }
  ]
}
```

### 7.2 Block Row Content

Each `docs_blocks.content` stores one top-level node:

```json
{
  "type": "heading",
  "attrs": {
    "level": 2,
    "blockId": "block-uuid"
  },
  "content": [
    { "type": "text", "text": "Billing" }
  ]
}
```

The row ID and `attrs.blockId` must match in canonical output.

### 7.3 Supported Block Types

V1 should support all currently persisted top-level editor nodes:

- paragraph
- heading
- bullet list
- ordered list
- task list
- blockquote
- callout
- code block
- table
- resizable image
- video embed
- HTML block
- divider / horizontal rule
- linked page
- entity embed

Unknown top-level node types should still be persisted as blocks with `type = node.type` if the node is valid JSON. Rendering compatibility should be preserved.

---

## 8. Backend Requirements

### 8.1 New Components

Add:

- `model.DocsBlock`
- `repository.DocsBlockRepository`
- `service.DocsBlockService`
- converter package or service methods:
  - `DocumentJSONToBlocks`
  - `BlocksToDocumentJSON`
  - `ExtractBlocksText`
  - `NormalizeBlockIDs`

### 8.2 Single Mutation Orchestrator

All document content mutations must flow through one service-level path.

Required method shape:

```go
SaveWholeDocument(ctx, documentID string, content json.RawMessage, actorID string) (*model.DocsContent, error)
PatchBlock(ctx, documentID, blockID string, patch DocsBlockPatchRequest, actorID string) (*model.DocsContent, error)
InsertBlock(ctx, documentID string, req InsertDocsBlockRequest, actorID string) (*model.DocsContent, error)
DeleteBlock(ctx, documentID, blockID string, actorID string) (*model.DocsContent, error)
MoveBlock(ctx, documentID, blockID string, req MoveDocsBlockRequest, actorID string) (*model.DocsContent, error)
```

Every method must:

1. Load the document.
2. Validate workspace scope.
3. Validate document is not deleted.
4. Validate document is not locked.
5. Validate caller has docs edit permission before entering the service or through command context.
6. Apply block changes inside a database transaction.
7. Regenerate the aggregate content inside the same transaction.
8. Extract plain text and word count from the regenerated aggregate.
9. Touch the parent document.
10. Publish websocket events after commit.
11. Refresh help center translation source after commit.
12. Queue embedding sync after commit.
13. Trigger autosnapshot behavior consistently with current whole-document saves.

### 8.3 Whole-Document Save Behavior

Existing whole-document saves must decompose the full JSON into block rows.

Rules:

- Existing blocks present in the incoming JSON are updated and reordered.
- New nodes without valid IDs get new IDs and block rows.
- Missing previous blocks are soft-deleted.
- Duplicate IDs in the incoming JSON are repaired.
- A node whose `attrs.blockId` belongs to another document is assigned a new ID.
- Empty documents produce no active block rows and aggregate content `{ "type": "doc", "content": [] }`.

### 8.4 Block Patch Behavior

Patch requests should replace one top-level block node in v1.

Rules:

- The replacement node must have the same `blockId`, or the backend must overwrite `attrs.blockId` to match the row ID.
- The replacement type may change.
- `last_edited_by` updates on content changes.
- The document aggregate is regenerated after the patch.

### 8.5 Reorder Behavior

Use fractional sort keys, consistent with existing docs ordering direction.

Rules:

- Insert and move operations accept `before_block_id` and/or `after_block_id`.
- If both are omitted, append to the end.
- If sort key space becomes too dense, rebalance the document's block sort keys inside the same transaction.

### 8.6 Deleted Blocks

Block deletes should soft-delete rows.

Rules:

- Soft-deleted blocks are excluded from aggregate reconstruction.
- Soft-deleted blocks may remain referenced by old versions, old chunks, or historical links.
- Active retrieval should ignore deleted blocks.
- Reintroducing a block with a previously soft-deleted ID should restore only when the incoming block ID belongs to the same document and the operation is a whole-document save or explicit restore. Otherwise generate a new ID.

---

## 9. API Requirements

### 9.1 Existing Endpoints

Keep existing endpoints:

- `GET /docs/documents/{docId}/content`
- `PUT /docs/documents/{docId}/content`
- `PUT /docs/documents/{docId}/content/markdown`

Behavior changes:

- `PUT /content` dual-writes blocks and aggregate.
- Response remains `DocsContent` for compatibility.

### 9.2 New Block Endpoints

Add:

```text
GET    /docs/documents/{docId}/blocks
GET    /docs/documents/{docId}/blocks/{blockId}
POST   /docs/documents/{docId}/blocks
PATCH  /docs/documents/{docId}/blocks/{blockId}
DELETE /docs/documents/{docId}/blocks/{blockId}
POST   /docs/documents/{docId}/blocks/{blockId}/move
```

All routes require existing docs RBAC:

- read routes: `PermDocsRead`
- mutation routes: `PermDocsEdit`

### 9.3 Request And Response DTOs

`DocsBlockResponse`:

```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "document_id": "uuid",
  "parent_id": null,
  "type": "paragraph",
  "content": {},
  "sort_key": "a0",
  "authored_by": "uuid",
  "last_edited_by": "uuid",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

`InsertDocsBlockRequest`:

```json
{
  "content": {},
  "before_block_id": "uuid",
  "after_block_id": "uuid"
}
```

`PatchDocsBlockRequest`:

```json
{
  "content": {}
}
```

`MoveDocsBlockRequest`:

```json
{
  "before_block_id": "uuid",
  "after_block_id": "uuid"
}
```

---

## 10. Frontend Requirements

### 10.1 Editor Block ID Management

The editor must ensure every top-level block-capable node has `attrs.blockId`.

Requirements:

- Add a TipTap global attribute for supported node types.
- Preserve `blockId` through normal edits.
- Generate IDs for newly inserted blocks.
- Generate IDs for pasted or imported blocks that lack IDs.
- Replace duplicate pasted IDs.
- Keep IDs stable through source view toggles when possible.
- Do not regenerate IDs on every render or save.

### 10.2 Initial Rollout Behavior

Initial frontend behavior should remain whole-document autosave.

Current editor save flow remains:

```text
DocsEditor onUpdate -> saveContent mutation -> PUT /content
```

Backend decomposes into blocks.

### 10.3 Later Block Patch Autosave

After block storage proves stable, autosave may switch to block patch calls.

Requirements for later phase:

- Detect changed top-level block by ID.
- Patch only changed blocks.
- Insert/delete/move blocks through block endpoints.
- Fall back to whole-document save when local diffing is ambiguous.

---

## 11. Agent And Tool Requirements

### 11.1 Existing Tools To Keep

Keep:

- `list_documents`
- `list_collections`
- `read_document`
- `search_documents`
- `create_document`
- `write_document_content`
- `link_document_to_object`

Existing tool behavior must remain valid.

### 11.2 Existing Tools To Extend

`read_document`:

- Add optional `include_blocks: boolean`.
- Default remains current small response.
- When true, return ordered block summaries and IDs.

`search_documents`:

- Include `block_id` and `block_preview` when search can identify a matching block.
- Keep document-level results compatible.

`link_document_to_object`:

- Add optional `block_id`.
- Existing calls without `block_id` create document-level links.

`write_document_content`:

- Keep whole-document write semantics.
- Internally route through the new content mutation orchestrator.

### 11.3 New Agent-Facing Tools

Add:

- `list_document_blocks`
- `read_document_block`
- `update_document_block`
- `insert_document_block`
- `delete_document_block`
- `move_document_block`

Tool contracts:

```json
{
  "name": "list_document_blocks",
  "input": {
    "document_id": "uuid"
  }
}
```

```json
{
  "name": "read_document_block",
  "input": {
    "document_id": "uuid",
    "block_id": "uuid"
  }
}
```

```json
{
  "name": "update_document_block",
  "input": {
    "document_id": "uuid",
    "block_id": "uuid",
    "content": {}
  }
}
```

```json
{
  "name": "insert_document_block",
  "input": {
    "document_id": "uuid",
    "content": {},
    "before_block_id": "uuid",
    "after_block_id": "uuid"
  }
}
```

```json
{
  "name": "delete_document_block",
  "input": {
    "document_id": "uuid",
    "block_id": "uuid"
  }
}
```

```json
{
  "name": "move_document_block",
  "input": {
    "document_id": "uuid",
    "block_id": "uuid",
    "before_block_id": "uuid",
    "after_block_id": "uuid"
  }
}
```

### 11.4 Internal Command Requirements

Block mutation tools should be command-backed because they are durable product mutations.

Add internal commands:

- `docs.update_document_block`
- `docs.insert_document_block`
- `docs.delete_document_block`
- `docs.move_document_block`

Update:

- command metadata
- worker tool registry
- tool catalog categories
- runtime profile allowlists
- agent template allowed tools where docs-editing agents need block tools
- prompt examples that currently recommend whole-document rewrites

---

## 12. Search And Retrieval Requirements

### 12.1 Internal Docs Search

Phase 1:

- Continue searching aggregate `docs_contents.content_text`.
- Optionally post-process matched result to identify likely block match.

Phase 2:

- Add block-aware search over block text.
- Return block IDs and previews.

### 12.2 Help Center Public Search

Public search can continue to use published article/translation aggregate content in phase 1.

Future enhancement:

- Return exact section anchors for public results when block citation metadata exists and the public renderer emits stable block anchors.

### 12.3 Embeddings

Embedding sync should move from whole-document text chunks to block-aware chunks.

Rules:

- Chunk each substantial block individually when it fits model limits.
- Group adjacent small blocks to avoid noisy tiny embeddings.
- Never group blocks across documents.
- Preserve document title in chunk metadata.
- Store `block_id` when the chunk is a single block.
- Store `block_range` when the chunk covers multiple adjacent blocks.

### 12.4 Support AI Citations

Support AI knowledge results should include:

- document ID
- block ID or block range
- chunk index
- title
- content snippet

The support answer renderer can initially continue displaying article citations, but the runtime should retain block citation metadata for future UI.

---

## 13. Publishing, Translations, And Versions

### 13.1 Public Help Center

The public help center continues rendering aggregate content.

Requirements:

- Block IDs should be preserved in aggregate JSON.
- Public HTML renderer may emit `data-block-id` or heading anchors later.
- Public URLs remain article-level.
- Public publications remain aggregate snapshots in v1.

### 13.2 Translations

Help center article translations remain aggregate JSON in v1.

Requirements:

- Source refresh still runs after content changes.
- `source_updated_at` and `source_synced` behavior stays unchanged.
- Auto-translation can continue translating aggregate article JSON.
- Future translation memory can map source blocks to localized blocks, but that is out of scope for v1.

### 13.3 Versions

Versions remain full-document snapshots.

Requirements:

- Snapshot creation stores regenerated aggregate content.
- Revert reconciles blocks from the selected version content.
- Old versions without block IDs get new block IDs during revert.
- Version preview remains aggregate read-only rendering.

---

## 14. Migration Plan

### 14.1 Phase 0: Schema And Converter

Ship:

- `docs_blocks` migration.
- `docs_links.block_id` migration.
- `docs_chunks.block_id` and `docs_chunks.block_range` migration.
- converter tests.

No product behavior change yet.

### 14.2 Phase 1: Backfill

Backfill `docs_blocks` from existing `docs_contents.content`.

Rules:

- For each document, parse aggregate JSON.
- Treat each top-level node as one block.
- Preserve existing `attrs.blockId` if valid and unique in the document.
- Generate missing IDs once.
- Generate new IDs for duplicates.
- Write repaired IDs back into `docs_contents.content`.
- Set block `type` from node type.
- Set `sort_key` from document order.
- Set `authored_by` from document `created_by` when no better value exists.
- Set `last_edited_by` null unless known.

### 14.3 Phase 2: Dual-Write Whole-Document Saves

Route whole-document saves through block reconciliation and aggregate regeneration.

Success criteria:

- Current editor continues working.
- Existing API responses are unchanged except for added block IDs in JSON attrs.
- Existing document saves update block rows transactionally.

### 14.4 Phase 3: Block Read APIs And Tools

Expose:

- block read endpoints
- `list_document_blocks`
- `read_document_block`
- optional `include_blocks` on `read_document`

Success criteria:

- Agents can inspect block IDs without mutating docs.

### 14.5 Phase 4: Block Mutation APIs And Tools

Expose:

- block mutation endpoints
- block mutation tools
- optional `block_id` on document links

Success criteria:

- Agents can safely update one block.
- Existing whole-document writes still work.

### 14.6 Phase 5: Block-Aware Embeddings

Change docs embedding sync to use block-aware chunks.

Success criteria:

- `docs_chunks` rows include block metadata.
- Support AI retrieval keeps document-level behavior but retains exact citations.

### 14.7 Phase 6: Frontend Block Patch Autosave

Optional later phase.

Switch editor autosave to block patches only after:

- block IDs prove stable in production,
- converters are reliable,
- agent block updates are successful,
- support and docs workflows continue to work.

---

## 15. Edge Cases

### 15.1 Missing Block IDs

Backend assigns IDs before persistence. Frontend should also assign IDs, but backend is the source of safety.

### 15.2 Duplicate Block IDs

For incoming aggregate JSON:

- Keep the first occurrence if it belongs to the same document.
- Generate new IDs for later duplicates.
- Return repaired aggregate content in `DocsContent`.

### 15.3 Cross-Document Block IDs

If an incoming block ID exists on another document, replace it with a new ID.

### 15.4 Pasted Content From Another Helpin Document

Pasted content may include existing `blockId` attrs. Treat them as copied content and generate new IDs unless the paste occurs as part of a same-document operation preserving an existing block.

### 15.5 Markdown Imports

Markdown does not contain block IDs. Generate IDs after conversion.

### 15.6 Source Mode Replacements

Source mode may replace the whole document. Backend reconciliation handles this as a whole-document save.

### 15.7 Empty Documents

Allow empty aggregate documents. Soft-delete all previous active blocks.

### 15.8 Locked Documents

All mutation paths must reject locked documents, including internal commands and agent tools. This cannot live only in HTTP handlers.

### 15.9 Concurrent Saves

V1 should use transactional reconciliation. If practical, add optimistic revision checks using `docs_contents.updated_at` or a new `content_revision`.

If no optimistic check ships in v1, document the risk: last writer wins still exists for whole-document saves.

### 15.10 Block Delete With Active Links

Do not hard-delete links automatically. Active link lists should show document-level fallback and indicate missing/deleted block detail when relevant.

### 15.11 Block Delete With Existing Chunks

Embedding sync should remove chunks for deleted blocks on the next sync. Immediate block mutation can also queue document/space reindex.

### 15.12 Version Revert

Revert may restore old content whose block IDs no longer exist. Reconciliation should restore same-document IDs when possible and generate new IDs when necessary.

### 15.13 Unknown Node Types

Persist unknown node types as opaque block content if valid JSON. Do not drop content because the backend does not recognize the type.

### 15.14 HTML Blocks

HTML block text extraction must continue stripping tags for `content_text` and embeddings.

### 15.15 Images And Assets

Image blocks may not contribute text. Asset reference scanning must inspect both aggregate content and block content during migration or rely on regenerated aggregate content.

### 15.16 Tables

Treat the entire table as one block in v1. Do not create per-row or per-cell block rows.

### 15.17 Lists

Treat the entire top-level list as one block in v1. Do not create per-list-item block rows.

### 15.18 Sort Key Exhaustion

When repeated inserts make fractional keys too dense, rebalance all active blocks for the document.

---

## 16. Security And Permissions

Requirements:

- Use existing docs RBAC.
- Enforce workspace scope on every block query and mutation.
- Reject block access when the parent document is inaccessible.
- Reject mutation when document is locked.
- Never trust client-supplied `workspace_id`, `document_id`, `authored_by`, or `last_edited_by` inside block content.
- Do not expose deleted block content unless a future audit/version endpoint explicitly requires it.
- Agent tools must use the same service path as user-initiated mutations.

---

## 17. Observability

Add structured logs for:

- block backfill start and completion
- block reconciliation failures
- duplicate or cross-document block ID repairs
- aggregate regeneration failures
- block mutation failures
- embedding sync failures with block metadata

Add metrics if available:

- documents backfilled
- blocks created during backfill
- block reconciliation duration
- whole-document saves reconciled
- block patch count
- aggregate regeneration duration
- embedding chunks with block metadata

---

## 18. Rollout And Feature Flags

Recommended flags:

- `DOCS_BLOCKS_DUAL_WRITE_ENABLED`
- `DOCS_BLOCK_READ_API_ENABLED`
- `DOCS_BLOCK_MUTATION_API_ENABLED`
- `DOCS_BLOCK_AGENT_TOOLS_ENABLED`
- `DOCS_BLOCK_EMBEDDINGS_ENABLED`
- `DOCS_BLOCK_PATCH_AUTOSAVE_ENABLED`

Rollout order:

1. Enable schema and backfill in staging.
2. Enable dual-write for staging users.
3. Compare aggregate output before and after reconciliation.
4. Enable dual-write in production.
5. Enable read APIs/tools.
6. Enable mutation APIs/tools for internal/system agents.
7. Enable mutation APIs/tools broadly.
8. Enable block-aware embeddings.
9. Consider block patch autosave.

---

## 19. Acceptance Criteria

### 19.1 Storage

- Existing documents are backfilled into `docs_blocks`.
- Whole-document saves create, update, reorder, and soft-delete block rows transactionally.
- Regenerated aggregate JSON renders the same meaningful content as the input.
- `content_text` and `word_count` remain correct.

### 19.2 Editor

- Top-level editor nodes include stable `blockId`.
- Ordinary edits do not regenerate block IDs.
- Paste/import creates new IDs when needed.
- Duplicate block IDs are repaired.

### 19.3 APIs

- Existing content endpoints remain compatible.
- New block read endpoints return ordered blocks.
- New block mutation endpoints update aggregate content.
- Mutations reject locked documents and unauthorized users.

### 19.4 Agents

- Existing agent whole-document writes keep working.
- Agents can list and read blocks.
- Agents can update, insert, delete, and move individual blocks.
- Agent block mutations enforce the same product invariants as user edits.

### 19.5 Links

- Existing document-level links keep working.
- New links can target blocks.
- Reverse lookup supports both document-level and block-level links.

### 19.6 Retrieval

- Docs chunks can store block IDs or block ranges.
- Embedding sync creates block-aware chunks.
- Support AI retrieval preserves exact source block metadata.

### 19.7 Compatibility

- Public help center rendering still works.
- Shared docs still work.
- Versions and revert still work.
- Translations and translation refresh still work.
- Imports and reconversion still work.
- Existing docs search still works.
- Existing support coverage draft flows still work.

---

## 20. Test Plan

### 20.1 Unit Tests

- Convert document JSON to block rows and back.
- Preserve block IDs on round trip.
- Generate missing block IDs.
- Repair duplicate block IDs.
- Reject or replace cross-document block IDs.
- Extract text from blocks including headings, paragraphs, lists, tables, callouts, code, HTML blocks.
- Rebalance sort keys.

### 20.2 Repository Tests

- Create/list/get blocks by document.
- Upsert blocks transactionally.
- Soft-delete omitted blocks.
- Query excludes deleted blocks.
- Block links query correctly handles null and non-null `block_id`.
- Docs chunks support `block_id` and `block_range`.

### 20.3 Service Tests

- Whole-document save creates blocks and aggregate.
- Whole-document save updates existing blocks.
- Whole-document save reorders blocks.
- Whole-document save soft-deletes removed blocks.
- Block patch updates one row and regenerated aggregate.
- Insert block places content correctly.
- Move block changes ordering.
- Delete block removes it from aggregate.
- Locked documents reject all block mutations.
- Translation refresh runs after block mutations.
- Embedding sync is queued after block mutations.
- Websocket document update event is published.

### 20.4 Handler Tests

- Existing `PUT /content` remains compatible.
- New block endpoints enforce RBAC.
- New block endpoints enforce document locks.
- Invalid block IDs return clear errors.
- Missing document returns 404.

### 20.5 Agent Tool Tests

- Existing `write_document_content` routes through block reconciliation.
- `list_document_blocks` returns ordered block IDs.
- `read_document_block` returns one block.
- `update_document_block` updates aggregate content.
- `insert_document_block` inserts around neighbors.
- `delete_document_block` removes from aggregate.
- `move_document_block` reorders aggregate content.
- `link_document_to_object` accepts optional `block_id`.

### 20.6 Integration Tests

- Editor save -> block rows -> aggregate -> editor reload preserves IDs.
- Version snapshot and revert with block-backed content.
- Public help center article render after block mutation.
- Translation source refresh after block mutation.
- Import job creates block-backed documents.
- Support AI embedding sync stores block citations.
- Search results include block citation metadata where available.

---

## 21. Implementation Checklist

### Backend

- Add migrations.
- Add model.
- Add repository.
- Add converter.
- Add service orchestrator.
- Refactor `DocsContentService.Save`.
- Refactor `DocsVersionService.Revert`.
- Refactor internal command `docs.write_document_content`.
- Refactor create document initial content path.
- Refactor import/reconversion writes.
- Refactor support coverage draft writes.
- Refactor PM shortcut docs writes.
- Add block endpoints.
- Add block-aware links.
- Add block-aware chunks.
- Update embedding sync.
- Update support AI result structs.
- Add tests.

### Frontend

- Add block ID TipTap global attribute.
- Add ID normalization helper.
- Normalize before save.
- Add block types.
- Add block services/hooks for later use.
- Add editor tests for ID preservation.
- Keep current whole-document autosave initially.

### Agents And Tools

- Add command metadata.
- Add internal commands.
- Add runtime tools.
- Add tool catalog entries.
- Update runtime profiles where appropriate.
- Update agent templates/allowed tools where needed.
- Add tests.

### Documentation

- Update internal tool framework docs if new command-backed docs tools are added.
- Add migration notes for operators.
- Add developer notes explaining `docs_contents` compatibility behavior.

---

## 22. Risks

### 22.1 ID Churn

If editor code regenerates IDs too often, block history and links become unstable.

Mitigation:

- Backend repairs only when required.
- Frontend tests verify ordinary edits preserve IDs.
- Compare block ID churn in staging logs.

### 22.2 Aggregate Divergence

Blocks and `docs_contents` could diverge if writes bypass the orchestrator.

Mitigation:

- Move all known write paths to one service.
- Make repository `Upsert` private or clearly documented as aggregate-only internal behavior.
- Add tests around every write path.

### 22.3 Agent Overwrites

Agents may still use whole-document writes and overwrite concurrent human edits.

Mitigation:

- Keep whole-document tool for compatibility.
- Prefer block tools in prompts and agent templates.
- Add optimistic content revisions in a follow-up if needed.

### 22.4 Search Quality Regression

Block-aware chunking could produce too many small chunks.

Mitigation:

- Group adjacent small blocks.
- Preserve title and surrounding heading context.
- Compare retrieval quality before enabling broadly.

### 22.5 Translation Complexity

Translations are aggregate-based while source content becomes block-based.

Mitigation:

- Keep translation aggregate flow in v1.
- Treat source refresh as document-level.
- Defer localized block mapping.

---

## 23. Open Questions

1. Should `docs_blocks.id` be generated by the backend only, or can the frontend generate UUIDs that the backend accepts?
2. Should v1 add a `content_revision` column for optimistic concurrency?
3. Should public help center HTML emit `data-block-id` immediately, or wait until block citations appear in UI?
4. Should deleted block links remain visible in reverse lookup with a "section deleted" state?
5. Should import backfill preserve any existing heading anchors as block metadata?
6. Should block text be stored redundantly on `docs_blocks` for faster block search, or derived from `content` when indexing?

---

## 24. Definition Of Done

The release is done when:

- All existing docs workflows continue to pass tests.
- Existing whole-document content endpoints remain backward compatible.
- Every active document has ordered block rows.
- Whole-document saves reconcile block rows and aggregate content transactionally.
- Agents can read and mutate individual blocks through controlled tools.
- Links can target documents or blocks.
- Embedding chunks can cite blocks or block ranges.
- Public help center, translations, versions, imports, and support coverage workflows continue working.

