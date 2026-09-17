# Docs AI translation implementation plan

This historical plan records the move from plain-text translation to structured
TipTap translation. Contributors should start with the current implementation
notes; unchecked steps and expected failures below are not today's task backlog.

## Current implementation and limits

Source-compared on 2026-09-18. The companion
[design review](../specs/2026-03-26-docs-ai-translation-design.md) describes the
current preservation contract and its limits.

- [Article generation](../../server/internal/service/docs_helpcenter_translation.go)
  uses `PrepareArticleTranslationPlan`, sends structured segments, applies the
  response, requires a nonempty final title, and upserts draft status. Body and
  metadata use the segment pipeline; generation does not publish the translation.
- [Apply](../../server/internal/docsi18n/reinsert.go) clones source content,
  restores protected terms, reinserts approved fields, then validates schema and
  preservation. Duplicate, missing, and extra response segment IDs are rejected.
  It does not universally reject empty translated body text, and there is no
  separate `RenderHTML` check in this generation path.
- The [registry](../../server/internal/docsi18n/registry.go) rejects unknown node
  types. Registered preserve-only nodes are different from unsupported nodes;
  Task 7's proposal to preserve unsupported nodes should not be read as current
  behavior or as permission to bypass unknown-node validation.
- Protected-term configuration and placeholder handling exist. The legacy
  `server/migrations/060_helpcenter_protected_terms.sql` is not the current place
  to add deployment migrations. The
  [versioned foundation](../../server/internal/dbmigrate/sql/000000000001_core_foundation.sql)
  includes `protected_terms`; follow the current migration process for changes.
- Validation before upsert protects existing content from those validation
  failures. It does not prove atomic rollback of every persistence or downstream
  operation, nor linguistic quality. Human review remains necessary.

Current package tests are consolidated in
[docsi18n_test.go](../../server/internal/docsi18n/docsi18n_test.go), rather than
all the proposed per-handler/per-schema files below. Historical Go 1.24, npm,
help-center directory, and commit commands must be reconciled with current
contributor setup before reuse. No application tests, model calls, or browser
translation checks were performed for this documentation review.

## Original implementation sequence

**Goal:** Replace the current plain-text article translation path with a schema-aware AI translation pipeline that preserves TipTap structure, marks, and protected terms, and fails closed on any unsafe output.

**Architecture:** Build a dedicated backend `docsi18n` package around the existing typed `tiptap.Node` model. Extract typed translation segments from the source tree, send them in one structured LLM request, reinsert returned text into a cloned source tree, validate structure/schema aggressively, and only then save a draft translation. Keep the frontend workflow largely unchanged, but surface clearer errors from the new fail-closed pipeline.

**Tech Stack:** Go 1.24, existing LLM provider abstraction, TipTap JSON model in `server/internal/tiptap`, React 19, TanStack Query, Vitest, Go testing.

---

### Task 1: Freeze The Current Failure With Tests

**Files:**
- Modify: `server/internal/service/docs_helpcenter_translation_test.go`
- Create: `server/internal/service/testdata/docs_i18n_source_fixture.json`

- [ ] **Step 1: Write failing backend tests for structural preservation**

Add tests that prove generation must preserve:
- callout nodes
- table nodes
- resizable image attrs
- inline marks
- preserve-only nodes like `htmlBlock`

- [ ] **Step 2: Run only the new tests to verify they fail under the current plain-text pipeline**

Run: `cd server && go test ./internal/service -run 'TestDocsHelpcenterTranslationService_(GeneratePreservesStructure|GeneratePreservesMarks|GeneratePreservesProtectedTerms|GenerateFailsClosedOnUnsafeOutput)' -count=1 -v`

Expected: FAIL because generation currently flattens to plain text and rebuilds paragraph-only JSON.

- [ ] **Step 3: Add fixture coverage for mixed rich docs**

Use a realistic JSON fixture with:
- heading
- paragraph with bold/link/code marks
- callout
- table
- resizableImage
- codeBlock
- htmlBlock
- videoEmbed

- [ ] **Step 4: Re-run the targeted tests**

Run the same command and confirm the failures are still the intended structural-loss failures.

### Task 2: Add The Translation Pipeline Package

**Files:**
- Create: `server/internal/docsi18n/types.go`
- Create: `server/internal/docsi18n/registry.go`
- Create: `server/internal/docsi18n/extract.go`
- Create: `server/internal/docsi18n/reinsert.go`
- Create: `server/internal/docsi18n/validate.go`
- Create: `server/internal/docsi18n/schema.go`

- [ ] **Step 1: Add typed translation pipeline models**

Define:
- `Segment`
- `TranslatedSegment`
- `TranslationRequest`
- `TranslationResponse`
- `NodePath`
- `ExtractContext`
- `ReinsertContext`
- `ValidationError`

- [ ] **Step 2: Add the node-handler registry**

Implement:
- `NodeTranslationHandler`
- handler registration
- lookup by node type
- fail if any encountered node type is missing a handler

- [ ] **Step 3: Add tree traversal helpers**

Implement path-tracking traversal using `tiptap.Node` so handlers can operate deterministically on typed nodes rather than `map[string]any`.

- [ ] **Step 4: Add clone helpers**

Create a safe deep-clone helper for `tiptap.Node` trees so reinsertion never mutates the source tree in place.

- [ ] **Step 5: Run package-level compile checks**

Run: `cd server && go test ./internal/docsi18n/... -count=1`

Expected: package compiles, even if handler tests are not complete yet.

### Task 3: Implement Safe Node Handlers

**Files:**
- Create: `server/internal/docsi18n/handlers_text.go`
- Create: `server/internal/docsi18n/handlers_blocks.go`
- Create: `server/internal/docsi18n/handlers_media.go`
- Create: `server/internal/docsi18n/handlers_special.go`
- Create: `server/internal/docsi18n/handlers_test.go`

- [ ] **Step 1: Write failing handler tests for text-bearing nodes**

Cover:
- paragraph
- heading
- blockquote
- list structures
- callout descendants
- table header/cell descendants

- [ ] **Step 2: Implement text/block handlers**

Translate only:
- text-node `Text`
- image `alt` / `title`

Preserve:
- attrs
- node order
- child counts

- [ ] **Step 3: Add preserve-only handlers**

Implement explicit preserve-only handlers for:
- `htmlBlock`
- `videoEmbed`
- `codeBlock`
- `horizontalRule`
- `hardBreak`

- [ ] **Step 4: Add unknown-node failure tests**

Confirm the extractor or validator fails when an unregistered node type appears.

- [ ] **Step 5: Run handler tests**

Run: `cd server && go test ./internal/docsi18n -run 'Test.*Handler' -count=1 -v`

Expected: PASS.

### Task 4: Preserve Rich Text Marks Exactly

**Files:**
- Modify: `server/internal/docsi18n/extract.go`
- Modify: `server/internal/docsi18n/reinsert.go`
- Modify: `server/internal/docsi18n/validate.go`
- Create: `server/internal/docsi18n/marks_test.go`

- [ ] **Step 1: Write failing tests for mark preservation**

Cover:
- bold + italic adjacent runs
- link text with preserved href
- inline code spans that must remain untranslated
- mixed runs inside one paragraph

- [ ] **Step 2: Implement text-node extraction at run granularity**

Each text node becomes one segment. Do not merge or split nodes.

- [ ] **Step 3: Implement mark-preserving reinsertion**

Replace only the `Text` field while preserving `Marks` exactly.

- [ ] **Step 4: Add validation that mark arrays and mark attrs are unchanged**

Fail if translated trees change mark boundaries or link attrs.

- [ ] **Step 5: Run mark tests**

Run: `cd server && go test ./internal/docsi18n -run 'Test.*Mark' -count=1 -v`

Expected: PASS.

### Task 5: Add Protected-Term Support

**Files:**
- Create: `server/internal/docsi18n/protected_terms.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Create: `server/migrations/060_helpcenter_protected_terms.sql`
- Create: `server/internal/docsi18n/protected_terms_test.go`

- [ ] **Step 1: Write failing tests for protected-term preservation**

Cover exact protected-term round-tripping for:
- product names
- acronyms
- UI labels

- [ ] **Step 2: Add config storage for protected terms**

Store protected terms on the help-center config so generation can load them at runtime.

- [ ] **Step 3: Implement placeholder protection**

Before the model call:
- replace protected terms with stable placeholders

After the response:
- validate placeholder survival
- restore original terms

- [ ] **Step 4: Run protected-term tests**

Run: `cd server && go test ./internal/docsi18n -run 'Test.*ProtectedTerm' -count=1 -v`

Expected: PASS.

### Task 6: Add Schema Validation And Fail-Closed Rules

**Files:**
- Modify: `server/internal/docsi18n/schema.go`
- Modify: `server/internal/docsi18n/validate.go`
- Create: `server/internal/docsi18n/schema_test.go`

- [ ] **Step 1: Write failing schema-validation tests**

Cover:
- disallowed node type
- missing required attrs
- wrong child shape for table/list structures
- malformed translated tree

- [ ] **Step 2: Implement server-side schema validation**

Mirror the supported editor nodes and attrs currently registered in:
- `frontend/src/components/docs/DocsEditor.tsx`
- custom extensions under `frontend/src/components/docs/`

- [ ] **Step 3: Add render validation**

Require translated output to render through `server/internal/tiptap.RenderHTML`.

- [ ] **Step 4: Add fail-closed error aggregation**

Return useful validation errors and refuse to save if any check fails.

- [ ] **Step 5: Run schema tests**

Run: `cd server && go test ./internal/docsi18n -run 'Test.*Schema|Test.*FailClosed' -count=1 -v`

Expected: PASS.

### Task 7: Replace The Existing Service Generation Path

**Files:**
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/service/docs_helpcenter_translation_test.go`

- [ ] **Step 1: Write a failing service-level regression test for end-to-end generation**

Test that `GenerateArticleTranslationDraft`:
- uses TipTap JSON source
- saves draft only
- preserves structure
- preserves unsupported nodes unchanged
- rejects unsafe LLM output without overwriting existing translations

- [ ] **Step 2: Remove `contentText` / `plainTextToTipTapDoc` generation usage**

Stop using flattened body translation for article generation.

- [ ] **Step 3: Build the new batched request contract**

Send:
- source locale
- target locale
- protected terms
- typed segment batch

Receive:
- translated segments only

- [ ] **Step 4: Rebuild title/excerpt/SEO using the same segment system**

Do not maintain a separate prose-generation path for metadata.
Slug should be derived server-side from the translated title.

- [ ] **Step 5: Ensure AI-generated drafts remain `draft`**

Keep publish manual and preserve `needs_review` rules.

- [ ] **Step 6: Run service tests**

Run: `cd server && go test ./internal/service -run 'TestDocsHelpcenterTranslationService' -count=1 -v`

Expected: PASS.

### Task 8: Surface Better Frontend Errors And Keep The Draft Workflow Clean

**Files:**
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/components/docs/helpcenter/MissingArticleTranslationDialog.tsx`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Create: `frontend/src/components/docs/helpcenter/__tests__/MissingArticleTranslationDialog.test.tsx`

- [ ] **Step 1: Write failing frontend tests for generation errors**

Cover:
- fail-closed backend error shown cleanly
- existing translation not replaced on error
- success still opens the locale draft editor

- [ ] **Step 2: Improve frontend error presentation**

Surface clear generation failures without implying partial success.

- [ ] **Step 3: Keep draft-only generation behavior intact**

Ensure the locale draft editor opens only after a successful safe generation.

- [ ] **Step 4: Run frontend translation-flow tests**

Run: `cd frontend && npm exec vitest run src/components/docs/helpcenter/__tests__/MissingArticleTranslationDialog.test.tsx`

Expected: PASS.

### Task 9: End-To-End Verification

**Files:**
- Verify touched files from Tasks 1-8

- [ ] **Step 1: Run backend docsi18n tests**

Run: `cd server && go test ./internal/docsi18n/... -count=1 -v`

- [ ] **Step 2: Run backend translation service tests**

Run: `cd server && go test ./internal/service -run 'TestDocsHelpcenterTranslationService' -count=1 -v`

- [ ] **Step 3: Run backend handler tests for translation endpoints**

Run: `cd server && go test ./internal/handler -run 'TestDocsHelpcenterTranslationHandler' -count=1 -v`

- [ ] **Step 4: Run frontend tests**

Run: `cd frontend && npm exec vitest run src/components/docs/helpcenter/__tests__/MissingArticleTranslationDialog.test.tsx src/components/docs/helpcenter/__tests__/ArticleLocalePillRail.test.tsx src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx`

- [ ] **Step 5: Run typecheck**

Run: `cd frontend && npm exec tsc --noEmit`

- [ ] **Step 6: Run backend build**

Run: `cd server && go build ./cmd/api`

- [ ] **Step 7: Run help-center build only if public translation contracts changed**

Run: `cd help-center && npm run build`

- [ ] **Step 8: Commit**

```bash
git add docs/specs/2026-03-26-docs-ai-translation-design.md \
        docs/plans/2026-03-26-docs-ai-translation-implementation.md \
        server/internal/docsi18n \
        server/internal/service/docs_helpcenter_translation.go \
        server/internal/service/docs_helpcenter_translation_test.go \
        server/internal/model/docs.go \
        server/internal/repository/docs_helpcenter.go \
        server/migrations/060_helpcenter_protected_terms.sql \
        frontend/src/pages/docs/DocsDocumentDetail.tsx \
        frontend/src/components/docs/helpcenter/MissingArticleTranslationDialog.tsx \
        frontend/src/components/docs/helpcenter/__tests__/MissingArticleTranslationDialog.test.tsx \
        frontend/src/hooks/queries/useDocs.ts
git commit -m "feat(docs): add safe ai translation pipeline"
```
