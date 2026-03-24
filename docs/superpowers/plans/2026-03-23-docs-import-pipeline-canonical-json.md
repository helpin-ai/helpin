# Docs Import Pipeline Canonical-JSON Plan

## Goal

Rebuild the Docs import pipeline so imported content lands as **canonical Tiptap JSON**, not as a `_markdown_source` envelope.

The import path should preserve formatting as much as the current editor + Help Center renderer can support, while using `htmlBlock` as the explicit fallback for unsupported but preservable fragments.

This plan is based on the current code in:

- [server/internal/service/docs_import.go](/root/teampulse/server/internal/service/docs_import.go)
- [server/internal/helpscout/convert.go](/root/teampulse/server/internal/helpscout/convert.go)
- [server/internal/helpscout/images.go](/root/teampulse/server/internal/helpscout/images.go)
- [server/internal/service/docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go)
- [server/internal/repository/docs_content.go](/root/teampulse/server/internal/repository/docs_content.go)
- [frontend/src/components/docs/DocsEditor.tsx](/root/teampulse/frontend/src/components/docs/DocsEditor.tsx)

## Current Findings

### 1. Import and runtime contracts do not match

Today the import pipeline does this:

1. fetch Help Scout article HTML
2. re-upload images
3. convert HTML to Markdown
4. store `{"_markdown_source":"..."}` in `docs_contents.content`

That happens in [docs_import.go](/root/teampulse/server/internal/service/docs_import.go) and [convert.go](/root/teampulse/server/internal/helpscout/convert.go).

But the rest of the system expects canonical Tiptap JSON:

- public Help Center rendering in [docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go) calls [html.go](/root/teampulse/server/internal/tiptap/html.go)
- search in [docs_search.go](/root/teampulse/server/internal/repository/docs_search.go) depends on `content_text`
- text extraction and word count in [docs_content.go](/root/teampulse/server/internal/repository/docs_content.go) walk Tiptap JSON nodes

So imports are not first-class until a human opens the doc in [DocsEditor.tsx](/root/teampulse/frontend/src/components/docs/DocsEditor.tsx), which detects `_markdown_source` and re-saves JSON.

### 2. The current converter is intentionally lossy

[convert.go](/root/teampulse/server/internal/helpscout/convert.go) currently:

- rewrites Help Scout callouts into generic blockquotes
- rewrites iframes into plain links
- strips inline styles
- runs everything through HTML-to-Markdown

That loses fidelity for the exact blocks we just added to the editor:

- `callout`
- `videoEmbed`
- `table`
- `htmlBlock`

### 3. Imported docs can be weak in search and public rendering

Because `_markdown_source` is not canonical Tiptap JSON:

- `content_text` extraction can be incomplete or empty until the doc is re-saved
- public `content_html` rendering can be incomplete because the Go renderer expects node types, not markdown envelopes
- import success is currently decoupled from Help Center renderability

### 4. Images are rehosted correctly, but only at string-rewrite level

[images.go](/root/teampulse/server/internal/helpscout/images.go) is useful and should stay, but it currently rewrites image URLs in the raw HTML string before conversion.

That is fine as a preprocessing step, but the next pipeline should operate on structured HTML/DOM after image rewriting, not on Markdown text.

## Target Architecture

### Core decision

Replace:

- `HelpScout HTML -> Markdown -> _markdown_source envelope`

with:

- `HelpScout HTML -> preprocessed HTML -> canonical Tiptap JSON`

### Import target contract

The importer should only generate nodes that the system can already support end-to-end:

- paragraph
- heading
- bulletList / orderedList / listItem
- blockquote
- codeBlock
- horizontalRule
- image / resizableImage
- table / tableRow / tableHeader / tableCell
- callout
- videoEmbed
- htmlBlock
- text marks that already render correctly

Anything unsupported but still safe/preservable should become `htmlBlock`.

Anything unsafe should be stripped or downgraded to safe text/link fallback.

### Recommended converter architecture

Keep the importer in Go.

Reason:

- the import job already runs in Go
- redirect creation, document creation, image rehosting, and help center wiring are already there
- adding a Node-side conversion service just for imports would complicate deployment and background job orchestration

Recommended approach:

- parse HTML into a DOM tree using a Go HTML parser
- walk the tree and map DOM nodes into Tiptap JSON nodes directly
- keep Help Scout-specific preprocessing small and explicit

## Recommended Plan

### Phase 1: Define the canonical import schema

Create one import schema contract that matches what the editor and Help Center already support.

- enumerate supported node types and attrs for import
- define exact mappings for:
  - headings
  - paragraphs
  - bold/italic/link/code marks
  - lists
  - blockquotes
  - callouts
  - code blocks
  - divider
  - images
  - tables
  - videos
  - htmlBlock
- define what the importer must never emit

Recommended rule:

- if a fragment maps cleanly to native nodes, do that
- if it is safe but unsupported, use `htmlBlock`
- do not flatten to Markdown first

### Phase 2: Replace string-based conversion with DOM-based conversion

Create a new Go converter package dedicated to HTML -> Tiptap JSON.

Recommended new package:

- `server/internal/docsimport/` or `server/internal/tiptapimport/`

Recommended responsibilities:

- parse HTML into DOM
- normalize/clean Help Scout quirks
- map DOM nodes to canonical TipTap JSON
- return:
  - `json.RawMessage` content
  - conversion warnings
  - optional stats for debugging

Recommended file split:

- `html_to_tiptap.go` — main DOM walker
- `node_builders.go` — helper constructors for doc/paragraph/text/marks
- `helpscout_normalize.go` — Help Scout-specific DOM cleanup
- `warnings.go` — structured warnings for dropped/rewritten content

### Phase 3: Upgrade the mapping rules

#### Callouts

Stop converting Help Scout callout HTML into markdown blockquotes.

Instead:

- map Help Scout `callout-info`, `callout-warn`, `callout-danger` into native `callout`
- define the variant mapping explicitly

Recommended first mapping:

- `info` -> `blue`
- `warn` -> `yellow`
- `danger` -> `red`

#### Video / iframe

Stop rewriting every iframe to a plain link.

Instead:

- known supported providers -> `videoEmbed`
- unsupported but safe embed fragments -> `htmlBlock`
- unsafe embeds -> plain link or dropped content with warning

#### Tables

Map actual HTML tables directly to native table nodes instead of hoping markdown conversion preserves them.

#### Images

Keep image rehosting, then emit native image nodes with rehosted URLs.

#### Unsupported fragments

If a fragment contains safe markup that does not map to native nodes, emit `htmlBlock`.

Examples:

- definition lists
- details/summary
- figure/figcaption patterns the image node cannot model yet
- preserved layout fragments after sanitization

### Phase 4: Change `DocsImportService` to store canonical JSON

Update [docs_import.go](/root/teampulse/server/internal/service/docs_import.go):

- remove the HTML-to-Markdown step
- call the new HTML-to-Tiptap converter instead
- save the returned Tiptap JSON directly via `contentSvc.Save`
- record conversion warnings per article in logs and optionally in import job failures/metadata

Recommended behavior:

- article import should only fail for hard failures:
  - fetch failure
  - document creation failure
  - invalid converter output
  - content save failure
- style simplifications should be warnings, not full import failures

### Phase 5: Remove importer dependence on `_markdown_source`

Imported docs should not rely on the frontend editor to become valid.

Required changes:

- Help Scout import stops generating markdown envelopes
- `_markdown_source` detection in [DocsEditor.tsx](/root/teampulse/frontend/src/components/docs/DocsEditor.tsx) can remain temporarily for backwards compatibility
- new imports must never produce `_markdown_source`

### Phase 6: Backfill previously imported docs

Existing imported docs created by the old pipeline need a migration path.

Recommended approach:

- write a one-time backfill job or admin command
- detect docs where `content` is a `_markdown_source` envelope
- if the original source HTML is still unavailable, do one of:
  - parse markdown to Tiptap JSON server-side if feasible
  - or mark those docs as “legacy converted” and preserve current frontend auto-conversion behavior

Pragmatic recommendation:

- do not block the new pipeline on perfect historical migration
- ship the new importer first
- then add a targeted backfill for legacy imported docs

### Phase 7: Improve image preprocessing

Keep [images.go](/root/teampulse/server/internal/helpscout/images.go), but treat it as preprocessing, not conversion.

Recommended improvements:

- keep URL rewrite before DOM conversion
- preserve `alt` text and any useful figure structure
- ensure failed image downloads do not fail the article import
- emit a warning when an original image URL had to be preserved

### Phase 8: Add import warnings as first-class output

Right now import failures are binary.

Add structured warnings for things like:

- unsupported iframe converted to link
- unsupported fragment wrapped in `htmlBlock`
- inline styles removed
- image download skipped
- unknown Help Scout class ignored

This will make the import preview and retry flow much more useful.

## File Map

### New or heavily changed backend files

- Modify: [server/internal/service/docs_import.go](/root/teampulse/server/internal/service/docs_import.go)
- Replace or deprecate: [server/internal/helpscout/convert.go](/root/teampulse/server/internal/helpscout/convert.go)
- Modify: [server/internal/helpscout/convert_test.go](/root/teampulse/server/internal/helpscout/convert_test.go)
- Keep and potentially extend: [server/internal/helpscout/images.go](/root/teampulse/server/internal/helpscout/images.go)
- Create: `server/internal/docsimport/html_to_tiptap.go`
- Create: `server/internal/docsimport/helpscout_normalize.go`
- Create: `server/internal/docsimport/node_builders.go`
- Create: `server/internal/docsimport/warnings.go`

### Existing files that should remain compatible

- [server/internal/service/docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go)
- [server/internal/repository/docs_content.go](/root/teampulse/server/internal/repository/docs_content.go)
- [server/internal/repository/docs_search.go](/root/teampulse/server/internal/repository/docs_search.go)
- [frontend/src/components/docs/DocsEditor.tsx](/root/teampulse/frontend/src/components/docs/DocsEditor.tsx)

## Testing Plan

### Unit tests for the converter

Add conversion fixtures for:

- plain paragraphs/headings/lists
- Help Scout callouts
- YouTube/Vimeo/Loom/Wistia iframes
- unsupported iframe
- simple and complex tables
- images with alt text
- mixed rich content with nested formatting
- unsupported safe HTML that should become `htmlBlock`
- unsafe HTML that should be stripped or downgraded

### Import service tests

Add service-level tests that verify:

- imported documents save as canonical Tiptap JSON
- `content_text` is populated correctly
- `word_count` is non-zero for real content
- published imports render in the public Help Center without needing a frontend re-save
- redirects are still created correctly

### Regression tests

Explicitly cover:

- imported video survives as `videoEmbed`
- imported callout survives as `callout`
- imported unsupported safe fragment survives as `htmlBlock`
- search can find imported article text immediately after import

## Rollout Plan

### Step 1

Land the converter and switch new imports to canonical JSON.

### Step 2

Keep `_markdown_source` frontend fallback temporarily for backwards compatibility.

### Step 3

Run targeted verification imports against real Help Scout samples.

### Step 4

Backfill old imported docs if needed.

### Step 5

Once legacy content is migrated or accepted, deprecate markdown-envelope import usage.

## Risks

### High risk

- converter emits node shapes the editor or Go renderer cannot handle
- imported HTML gets wrapped in `htmlBlock` too aggressively, making content harder to edit
- old imports remain broken if migration is skipped

### Medium risk

- DOM-based conversion is more work than markdown conversion
- Help Scout HTML may contain patterns not covered by initial fixtures
- table/import edge cases can still produce structurally valid but ugly output

### Low risk

- keeping `_markdown_source` fallback for a transition period adds temporary complexity

## Recommended Execution Order

1. define canonical import node contract
2. build Go HTML -> Tiptap converter
3. map callout/video/table/image/htmlBlock correctly
4. switch `DocsImportService` to save canonical JSON
5. add unit + service regression tests
6. run real-sample verification
7. plan legacy backfill
