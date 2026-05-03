# Addressable Docs Blocks Plan

## Status

Implementation started.

## Goal

Move docs toward stable, addressable blocks while preserving the current full-document aggregate until every creation, editing, publishing, search, and agent path has been migrated safely.

The core model is:

- every document has ordered block rows
- every block has a stable UUID
- blocks are typed
- block content is stored as JSON
- document content remains a materialized aggregate for compatibility
- links, citations, chunks, vectors, and agent edits can target a block

## Current Findings

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
