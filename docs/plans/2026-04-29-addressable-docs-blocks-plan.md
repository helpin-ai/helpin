# Addressable document blocks plan

## Status

Historical phased plan, source-compared on 2026-09-17. Core storage, mutation,
and retrieval paths are implemented. This page explains the migration to
contributors; the phases below are the original requirements, not a current
completion checklist or proof that every document path has been audited.

## Current implementation

The [content mutation repository](../../server/internal/repository/docs_content_mutation.go)
normalizes aggregate content and synchronizes block rows inside its transaction.
The [block repository](../../server/internal/repository/docs_block.go) stamps IDs
on supported top-level nodes, detects IDs owned by other documents, and retains
aggregate compatibility for malformed/non-document content. This does not mean
every nested editor node has its own row. The
[editor extension](../../frontend/src/components/docs/BlockIdExtension.ts)
provides client-side IDs for supported node types.

[Block mutations](../../server/internal/service/docs_block.go) check document
locking and use versioned aggregate saves; patch additionally requires a positive
matching block revision and verifies the current block content. The
[router](../../server/internal/router/router.go) exposes list/create/patch/reorder/
delete operations with Docs read/edit permissions. Internal commands include
`docs.update_document_block`. These entry-point checks must remain intact; a
repository method alone is not an authorization boundary.

[Embedding synchronization](../../server/internal/service/docs_embedding.go)
prefers nonempty structured block chunks, retaining aggregate-text fallback.
A block-read error fails synchronization rather than silently falling back.
[Support search](../../server/internal/service/support_knowledge_search.go)
propagates block IDs in results. This supersedes the original document-only
vector/agent description below, without asserting that every downstream consumer
or public rendering path is fully block-native.

[Document deletion](../../server/internal/service/docs_deletion.go) includes
block rows and scans block JSON when collecting owned asset references. The
[SQL migration](../../server/internal/dbmigrate/sql/202604290003_addressable_docs_blocks.sql)
contains the top-level backfill. These source paths do not establish that the
migration has run in any particular installation. Original compilation/build
steps below were not rerun during this documentation review.

## Goal

Move docs toward stable, addressable blocks while preserving the current full-document aggregate until every creation, editing, publishing, search, and agent path has been migrated safely.

The core model is:

- every document has ordered block rows
- every block has a stable UUID
- blocks are typed
- block content is stored as JSON
- document content remains a materialized aggregate for compatibility
- links, citations, chunks, vectors, and agent edits can target a block

## Original findings

Docs are not created from one path. Content can be written by editor autosave, markdown save, imports, versions/reverts, internal commands, PM import flows, Temporal agent activity bridges, and direct repository callers. The block migration has to attach to the shared content write path first, then expose block-level APIs.

The current vector pipeline chunks `docs_contents.content_text` by document. That makes citations and AI edits coarse. The new path should prefer block chunks and fall back to aggregate document text when block rows are absent.

Agent tooling currently has document-level read/write behavior. It needs a block-aware read shape and a command-backed block update path with revision checks so agents can patch one block without rewriting the entire document.

Deletion, asset retention, publishing, help-center rendering, snapshots, and version revert paths still depend on aggregate document content. The compatibility aggregate should remain authoritative for those surfaces until they are explicitly block-native.

## Implementation Phases

### Phase 1: Storage And Dual-Write

- Add `docs_blocks` with `workspace_id`, `document_id`, optional `parent_id`, `type`, `content`, `content_text`, `sort_key`, `revision`, `authored_by`, `last_edited_by`, timestamps, and `deleted_at`.
- Add optional `block_id` to doc links.
- Add optional `block_id` and `block_range` to doc chunks.
- Stamp stable `blockId` attributes into supported top-level editor nodes.
- Normalize block IDs server-side on every content save.
- Sync block rows transactionally from the aggregate content save path.
- Preserve existing `docs_contents` as the compatibility aggregate.

### Phase 2: Block APIs

- Add list/create/patch/reorder/delete block endpoints under document routes.
- Require document edit permissions through the existing docs route middleware.
- Use block `revision` for optimistic concurrency.
- Keep block mutations writing through the same content save service used by humans and automation.
- Queue snapshots and embedding sync after block mutations.

### Phase 3: Chunking And Retrieval

- Prefer active block rows when creating doc chunks.
- Store `docs_chunks.block_id` for block-derived chunks.
- Keep document-level fallback chunking for legacy or malformed content.
- Return `block_id` through knowledge search, support citations, coverage matching, and retrieval traces.
- Dedupe coverage candidates by document and block when block IDs exist.

### Phase 4: Agent And Command Surface

- Include compact block metadata in document read tools.
- Add a command-backed `update_document_block` path with `document_id`, `block_id`, `revision`, and block JSON content.
- Keep full-document write tools available during migration.
- Prefer block updates in agent instructions where the target block is known.

### Phase 5: Compatibility Hardening

- Verify every document creation path writes content through the shared repository/service path or explicitly syncs blocks.
- Verify version revert rehydrates blocks.
- Verify permanent delete removes block rows and scans block JSON for retained assets before deleting files.
- Keep aggregate publishing and public rendering unchanged until a dedicated block-native renderer is planned.
- Backfill existing top-level TipTap blocks in the addressable-block migration, stamping aggregate `blockId` attributes and inserting matching `docs_blocks` rows for documents that do not already have active blocks.

## Attention Areas

- Block IDs must be globally unique enough to prevent copied document JSON from reusing IDs across documents.
- Existing imports and automation paths may produce content without block IDs; server normalization is required.
- Unsupported or non-document JSON should not break saves; it should continue to save aggregate content and clear stale active blocks if appropriate.
- Search quality depends on grouping tiny blocks well enough to avoid fragmented context.
- Reorder operations can change block revisions; callers should refresh block metadata after structural edits.
- Block-level updates must never bypass document locking, workspace authorization, snapshotting, or embedding sync.

## Initial Validation

- Compile the touched backend packages.
- Compile the API and Temporal worker entrypoints.
- Build the frontend.
- Run focused repository/service tests for docs block sync and block mutation behavior as follow-up coverage.
