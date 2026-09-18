# Docs AI translation design


> Historical design, source-compared on 2026-09-17. This page explains the
> structured article-translation pipeline for contributors. The main design is
> implemented; the original plain-text baseline and phase list are historical.

## Current implementation and limits

[Article generation](../../server/internal/service/docs_helpcenter_translation.go)
parses source TipTap JSON and calls
[PrepareArticleTranslationPlan](../../server/internal/docsi18n/extract.go). The
model receives segments for approved content and article metadata, including the
title, excerpt, and SEO fields. [Apply](../../server/internal/docsi18n/reinsert.go)
clones the source tree, restores protected terms, reinserts translated fields, and
validates schema and preservation before the service saves a draft. The current
article-generation path does not rebuild the document from plain text.

The [registry](../../server/internal/docsi18n/registry.go) also supports `aiSection`,
`taskList`, `taskItem`, and preserve-only `entityEmbed` and `citationBlock`, beyond
the original matrix below. Unknown nodes fail. Handler signatures and file layout
have evolved; use the registry as the implementation reference rather than copying
the proposed interface verbatim.

[Protected terms](../../server/internal/docsi18n/protected_terms.go) come from
help-center configuration and use placeholders whose occurrence counts must survive
translation. This enforces configured terms; prompt instructions about all other
proper nouns and technical tokens are not equivalent deterministic guarantees.

Response validation rejects duplicate, missing, and extra segment IDs. It does
not explicitly reject every empty `translated_text`: the service checks the final
title, while the [translation schema](../../server/internal/docsi18n/schema.go)
does not require nonempty text-node contents. The general “no empty translations”
requirement below is therefore not fully implemented. This validator is a maintained
backend schema, not proof of automatic parity with every editor extension.

The generation pipeline does not include the proposed separate HTML render check
or HTML-block review metadata in its draft result. Structural preservation is
validated, but linguistic quality still needs review. Generation sets draft status;
it does not publish the result. Validation errors before upsert leave existing
translation content untouched; that is a narrower guarantee than rollback of every
possible persistence or downstream failure.

[Structural test source](../../server/internal/docsi18n/docsi18n_test.go) covers
preservation and protected terms. These tests were inspected, not rerun during
this documentation review. The original phases and test matrix below are design
history, not an outstanding implementation checklist or current test report.

## Original design

## Goal

Replace the current plain-text help-center article translation flow with a schema-aware AI translation pipeline that preserves TipTap document structure by construction, translates only approved text-bearing fields, validates aggressively, and fails closed on any structural or schema mismatch.

## Why the original approach needed replacement

At the time of this design, the AI translation path in [`server/internal/service/docs_helpcenter_translation.go`](../../server/internal/service/docs_helpcenter_translation.go) translated `DocsContent.ContentText` and then rebuilt content with `plainTextToTipTapDoc(...)`. This destroys structure before translation begins and guarantees loss of:

- rich-text marks
- tables and table cell layout
- callouts and custom block structure
- resizable image metadata
- video and embed blocks
- HTML blocks
- any future custom node types

This architecture is not safe for production multilingual docs.

## Non-Negotiable Principles

1. Translate TipTap JSON, not `content_text`.
2. Preserve structure by construction.
3. The model never controls document structure.
4. Only approved text-bearing fields are translated.
5. Rebuild from the original tree.
6. Validate aggressively.
7. Fail closed.
8. AI-generated translations always remain draft until a human publishes.

## Scope

This design applies to AI generation and regeneration of help-center article translations only.

It does not change:

- default-locale mirror behavior
- space and collection translation editing flows
- human-authored translation editing after the initial draft is generated

## Architecture Overview

The new pipeline operates on TipTap JSON using the existing typed node model in [`server/internal/tiptap/html.go`](../../server/internal/tiptap/html.go).

High-level flow:

1. Load source article TipTap JSON.
2. Parse into a typed `tiptap.Node` tree.
3. Walk the tree with a node-handler registry.
4. Extract only approved translatable segments.
5. Protect configured terms before sending content to the model.
6. Send one structured translation request with segment IDs and context.
7. Validate the translation response.
8. Clone the original tree and reinsert translated text only into approved fields.
9. Validate structure, marks, preserved attrs, and schema.
10. Save the generated locale draft only if every check passes.

The model returns translated segments, not a rebuilt document.

## Package Layout

Add a new backend package:

- `server/internal/docsi18n/types.go`
- `server/internal/docsi18n/registry.go`
- `server/internal/docsi18n/extract.go`
- `server/internal/docsi18n/reinsert.go`
- `server/internal/docsi18n/validate.go`
- `server/internal/docsi18n/protected_terms.go`
- `server/internal/docsi18n/schema.go`
- `server/internal/docsi18n/handlers_text.go`
- `server/internal/docsi18n/handlers_blocks.go`
- `server/internal/docsi18n/handlers_media.go`
- `server/internal/docsi18n/handlers_special.go`

This keeps translation behavior isolated from the help-center service and makes node rules explicit and testable.

## Node Handler Registry

Use a registry pattern:

```go
type NodeTranslationHandler interface {
    NodeType() string
    Extract(node *tiptap.Node, path NodePath, ctx *ExtractContext) ([]Segment, error)
    Reinsert(node *tiptap.Node, path NodePath, byID map[string]TranslatedSegment, ctx *ReinsertContext) error
    Validate(source, translated *tiptap.Node, path NodePath) error
}
```

Registry behavior:

- every encountered node type must have a registered handler
- unsupported unknown nodes are errors, not best-effort fallbacks
- preserve-only nodes still need handlers so their invariants are explicit

## Segment Schema

The model should receive a structured batch of segments, not whole-document prose.

Segment shape:

```json
{
  "id": "doc/0/2/content/1",
  "node_path": "0.2.1",
  "node_type": "text",
  "field_name": "text",
  "text": "Open Settings",
  "context_hint": "heading level 2 in help-center article",
  "group_id": "block/0/2"
}
```

Top-level request:

```json
{
  "source_locale": "en",
  "target_locale": "fr",
  "protected_terms": ["Helpin", "SLA", "AI Copilot"],
  "segments": [...]
}
```

Response:

```json
{
  "segments": [
    { "id": "doc/0/2/content/1", "translated_text": "Ouvrez Paramètres" }
  ]
}
```

Rules:

- IDs must round-trip exactly.
- No extra segments are allowed.
- No missing segments are allowed.
- The model does not return structure, attrs, marks, or HTML.

## Rich Text Mark Preservation

This is critical.

Text should be translated at the text-node level, not at the whole-paragraph level.

That means:

- each TipTap `text` node is extracted as a separate segment
- the node’s `marks` array remains attached to that exact node
- reinsertion replaces only the `text` field
- marks, mark attrs, and node order are preserved unchanged

Implications:

- bold, italic, underline, strike, subscript, superscript are preserved automatically
- link text is translated, but the link mark and `href` are preserved
- inline code text stays unchanged by default
- text nodes must never be merged or split during translation

This prioritizes structural fidelity over perfect linguistic flexibility, which is the correct tradeoff for production docs.

## Node Support Matrix

### Fully translated

- `doc`
- `paragraph`
- `heading`
- `bulletList`
- `orderedList`
- `listItem`
- `blockquote`
- `callout`
- `table`
- `tableRow`
- `tableHeader`
- `tableCell`
- `text` nodes without a `code` mark
- `resizableImage` and `image` text attrs: `alt`, `title`

### Preserved unchanged

- `htmlBlock`
- `videoEmbed`
- `horizontalRule`
- `hardBreak`
- URLs and technical attrs
- attachment IDs
- image sizing and layout attrs
- embed URLs and provider attrs
- `codeBlock` code content

### Special handling

- `callout`: translate descendant rich text, preserve `variant`
- `resizableImage`: translate human-facing attrs only, preserve `src`, `width`, `height`, `alignment`, `linkUrl`, `attachmentId`
- `codeBlock`: preserve code content and language untouched
- `htmlBlock`: preserve raw HTML unchanged; surface a review note in generation metadata
- unknown node types: fail closed

## Protected Terms

Add protected-term support as part of generation, not as a prompt suggestion only.

Recommended model:

- help-center-level `protected_terms text[]`
- exact-match placeholder protection before the model call
- response validation that ensures placeholders survive intact
- restoration of the original terms after translation

Protected terms cover:

- product names
- feature names
- brand names
- UI labels
- technical acronyms
- code terms

## Validation

### Response validation

- all extracted segment IDs must be present exactly once
- no unknown IDs
- no duplicate IDs
- no empty translations for required segments
- malformed JSON or schema mismatch rejects the job

### Structural validation

- same node types at every path
- same child counts
- same node ordering
- same preserved attrs
- same marks and mark attrs for text nodes
- only approved fields may differ

### Schema validation

Add a server-side TipTap schema validator that mirrors the editor’s supported nodes and attrs from [`frontend/src/components/docs/DocsEditor.tsx`](../../frontend/src/components/docs/DocsEditor.tsx).

The final translated tree must validate before save.

### Render validation

The translated document must also render through [`server/internal/tiptap/html.go`](../../server/internal/tiptap/html.go) without error.

## Failure Behavior

If any stage fails:

- reject the generation request
- do not partially save or overwrite translation content
- keep the existing translation row untouched
- return a useful error to the UI
- log structured validation details for debugging

This system must fail closed, never degrade silently.

## Draft And Publish Behavior

AI-generated translations:

- always save as `draft`
- always require human review before publish

If source content changes later:

- non-default locales move to `needs_review`

## Default Locale

The default locale remains:

- a system-managed mirror of the source document
- not directly editable as a separate translation

This keeps the public read model uniform while preserving internal docs as the source of truth.

## Testing Strategy

Tests should use real TipTap JSON fixtures and cover:

- headings, paragraphs, lists, and blockquotes
- mixed inline marks
- links
- callouts
- tables
- resizable images
- code blocks
- `videoEmbed`
- `htmlBlock`
- protected terms
- validation failures
- partial/malformed model responses
- regeneration preserving existing translation on failure

## Implementation Phases

### Phase 1

- node-handler registry
- segment extraction
- structured LLM contract
- reinsertion into cloned tree
- validation and fail-closed behavior

### Phase 2

- mark-aware extraction and reinsertion hardening
- exact preservation of inline formatting spans

### Phase 3

- protected terms

### Phase 4

- preserve-only special-node hardening and review metadata

## Recommendation

Ship only after the plain-text translation path is fully removed from article generation and the new pipeline is covered by golden structural tests. This feature must be deterministic in structure preservation, not merely improved in prompting.
