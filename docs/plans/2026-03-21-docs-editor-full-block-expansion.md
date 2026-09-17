# Docs Editor Full Block Expansion Plan

## 1. Current-State Analysis

### Current editor architecture

The Docs editor today is a relatively small Tiptap surface in [frontend/src/components/docs/DocsEditor.tsx](/root/teampulse/frontend/src/components/docs/DocsEditor.tsx):

- `StarterKit`
- `Placeholder`
- `tiptap-markdown`
- one custom `ResizableImageExtension` from [frontend/src/components/ui/resizable-image-extension.ts](/root/teampulse/frontend/src/components/ui/resizable-image-extension.ts)

Current authoring UX includes:

- floating selection toolbar
- image upload via paste/drop/file picker
- markdown source mode
- markdown import/export
- `.doc` export and `.docx` import
- autosave into `docs_contents.content`

Current docs storage is canonical JSON in [server/internal/model/docs.go](/root/teampulse/server/internal/model/docs.go) as `DocsContent.Content json.RawMessage`, with plain-text extraction and word count computed in [server/internal/repository/docs_content.go](/root/teampulse/server/internal/repository/docs_content.go).

### What the current editor actually supports

Effective authoring support today is:

- paragraphs
- headings
- bold / italic / strike / inline code
- bullet / ordered lists
- blockquote
- links
- images via `resizableImage`
- markdown source editing and markdown import/export

The Go Help Center renderer in [server/internal/tiptap/html.go](/root/teampulse/server/internal/tiptap/html.go) already knows how to render more than the Docs editor exposes:

- paragraphs
- headings
- bullet / ordered / task lists
- code blocks
- blockquotes
- horizontal rules
- images
- tables
- core marks like link / underline / highlight / subscript / superscript

That means the current system already has a schema mismatch: the public renderer can output nodes the editor does not intentionally expose or re-edit cleanly.

### What Tiptap already gives us vs what is missing

Tiptap or the existing markdown stack can cover most of the standard blocks:

- `Blockquote`: built-in
- `CodeBlock`: built-in
- `HorizontalRule`: built-in
- `Image`: built-in, but we already use a custom image node for resizing
- `Table`, `TableRow`, `TableCell`, `TableHeader`: built-in extensions

The `tiptap-markdown` package already has markdown serialization helpers for:

- blockquote
- code block
- horizontal rule
- image
- link
- table
- raw HTML fallback blocks

The blocks that need custom work are:

- `Callout`
- `Emoji` picker UX for inserting emoji into the editor
- `HTML` block
- `Video`
- richer `Image` behavior if we want captions / asset ownership / public fidelity

### Current import path and why Help Scout content loses fidelity

The current Help Scout import flow is in [server/internal/service/docs_import.go](/root/teampulse/server/internal/service/docs_import.go) and [server/internal/helpscout/convert.go](/root/teampulse/server/internal/helpscout/convert.go):

1. Fetch article HTML from Help Scout.
2. Re-upload `<img>` URLs to S3 in [server/internal/helpscout/images.go](/root/teampulse/server/internal/helpscout/images.go).
3. Convert HTML to Markdown.
4. Save the result as a `_markdown_source` JSON envelope, not as canonical Tiptap JSON.

That causes four major problems:

1. **Lossy conversion by design**
   - callouts are flattened into generic blockquotes
   - iframes/videos become plain links
   - inline styles are stripped
   - raw HTML blocks are reduced or lost
   - complex tables can degrade

2. **Imported content is not immediately canonical**
   - imported docs are stored as `{"_markdown_source":"..."}` instead of `{"type":"doc", ...}`
   - canonical JSON only exists after the document is opened in the frontend editor and re-saved

3. **Public Help Center rendering can fail before a human opens the doc**
   - [server/internal/service/docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go) renders public articles by calling [server/internal/tiptap/html.go](/root/teampulse/server/internal/tiptap/html.go)
   - the `_markdown_source` envelope is not a Tiptap doc and therefore renders as empty or incomplete content

4. **Import and public rendering use different contracts**
   - import produces markdown envelope
   - editor eventually produces Tiptap JSON
   - public help center expects Tiptap JSON rendered to HTML

This is the single biggest current architecture flaw for Help Scout fidelity.

### Current public Help Center rendering path

The in-repo public path today is:

1. Public API routes in [server/internal/handler/docs.go](/root/teampulse/server/internal/handler/docs.go)
2. Public article assembly in [server/internal/service/docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go)
3. Tiptap JSON -> HTML in [server/internal/tiptap/html.go](/root/teampulse/server/internal/tiptap/html.go)
4. HTML injected in the widget view from [packages/widget-core/src/components/HelpArticleView.tsx](/root/teampulse/packages/widget-core/src/components/HelpArticleView.tsx)
5. styled by [packages/widget-core/src/styles/widget.css](/root/teampulse/packages/widget-core/src/styles/widget.css)

What that renderer path visibly supports today:

- headings
- paragraphs
- lists
- blockquotes
- code / pre
- images
- tables
- links

What it does not support today as first-class blocks:

- callouts
- video embeds
- raw HTML blocks
- image captions / figures
- custom divider styling beyond a plain `<hr>`
- any node-specific public fallback for unsupported content

### Current image model

Docs images currently piggyback the PM attachment upload path in [frontend/src/hooks/useEditorImageUpload.ts](/root/teampulse/frontend/src/hooks/useEditorImageUpload.ts) and [server/internal/service/pm_attachment.go](/root/teampulse/server/internal/service/pm_attachment.go). That works, but it is not a production-grade docs asset model:

- it is PM-owned infrastructure used from Docs
- the stored node is basically URL-first
- there is no docs-specific asset contract
- there is no docs-specific caption / attribution / reuse model

This is acceptable for current simple image support, but not for a complete editor expansion.

### Recommended target architecture

The right target is:

- **canonical storage**: Tiptap JSON only
- **single docs schema contract**: one explicit set of supported node types and attrs
- **full-surface parity**: editor, importer, public Help Center renderer, shared-doc rendering, and export all understand the same node set
- **strict validation**: server rejects or sanitizes unsupported / unsafe attrs before persistence or publication
- **no markdown envelope fallback for Help Scout import**

Recommended architecture layers:

1. **Docs schema bundle**
   - headless Tiptap extensions for all supported docs nodes
   - no React node views in this layer
   - reusable by editor and import conversion

2. **Editor UI layer**
   - slash menu
   - node views
   - dialogs for HTML / video / image editing
   - toolbar / bubble menu actions

3. **Import conversion layer**
   - preprocess Help Scout HTML
   - convert HTML directly to canonical Tiptap JSON using a Node-based Tiptap HTML parser with the same schema bundle
   - no intermediate markdown as the primary import representation

4. **Public render layer**
   - extend the Go renderer to support the same node set with strict sanitization and provider validation
   - render safe HTML only
   - keep `content_html` as the public API contract for the widget

5. **Validation layer**
   - server-side validation of node names and attrs
   - publish-time validation to guarantee Help Center renderability

---

## 2. Block-by-Block Architecture Plan

### Blockquote

**Editor insertion**

- Add toolbar and slash entry for `Blockquote`.
- Use existing blockquote toggle behavior.

**Built-in vs custom**

- Use Tiptap built-in `Blockquote`.

**Tiptap JSON storage**

- Standard ProseMirror/Tiptap:
  - `type: "blockquote"`
  - `content: block+`

**Editor rendering**

- Default Tiptap block rendering is enough.
- Add docs-specific typography styles so the editor and Help Center feel closer.

**Public Help Center rendering**

- Continue using `<blockquote>...</blockquote>`.
- Add explicit Help Center styling tokens for spacing, border, and color consistency.

**Help Scout import**

- Directly map `<blockquote>` to `blockquote`.
- Do not route callouts through blockquote anymore.

**Markdown/export**

- Clean markdown mapping via `>`.

**Risk**

- Low.

**Likely failure points**

- nested blocks inside blockquotes if editor toolbar only handles a simple paragraph case

### Callout

**Editor insertion**

- Add one `Callout` slash-menu root item with a submenu:
  - Blue
  - Green
  - Grey
  - Red
  - Yellow
- Toolbar can optionally expose a generic callout button that opens a small variant picker.

**Built-in vs custom**

- Custom block node.

**Recommended data model**

- Node name: `callout`
- Attrs:
  - `variant: "blue" | "green" | "grey" | "red" | "yellow"`
- Content:
  - `block+`

Example:

```json
{
  "type": "callout",
  "attrs": { "variant": "yellow" },
  "content": [
    {
      "type": "paragraph",
      "content": [{ "type": "text", "text": "This action is irreversible." }]
    }
  ]
}
```

**Editor rendering**

- React NodeView with:
  - variant-specific background/border/icon treatment
  - editable nested content
  - inline variant switcher in node controls

**Public Help Center rendering**

- Render as:
  - `<aside class="docs-callout docs-callout--yellow" data-callout-variant="yellow">...</aside>`
- Add matching Help Center CSS classes in widget/public styles.
- Do not render as plain blockquote.

**Help Scout import**

- Primary mapping:
  - Help Scout callout wrapper -> `callout` node
- Mapping strategy:
  - class-based mapping first
  - color-style heuristic second
  - fallback to `grey`
- The importer should emit a warning when it had to guess a variant.

**Markdown/export**

- Markdown does not cleanly support colored callouts.
- Best-fidelity markdown export should emit raw HTML block:

```html
<div data-helpin-callout="yellow">
  <p>This action is irreversible.</p>
</div>
```

- If plain-text fallback is required, downgrade to blockquote with a label, but treat that as lossy.

**Risk**

- Medium.

**Likely failure points**

- incorrect Help Scout class/color mapping
- nested complex content inside callouts
- markdown fallback losing variant information

### Code

**Editor insertion**

- Distinguish:
  - inline code mark
  - code block
- Slash menu should insert a code block.
- Toolbar should expose both inline code and code block, or keep inline in the floating selection menu and block in slash menu.

**Built-in vs custom**

- Use built-in `Code` mark and `CodeBlock`.
- If syntax highlighting is wanted immediately, use a syntax-aware block extension, but the storage contract should still be standard `codeBlock`.

**Tiptap JSON storage**

- Inline code:
  - text node with `marks: [{ type: "code" }]`
- Code block:
  - `type: "codeBlock"`
  - attrs:
    - `language?: string`

**Editor rendering**

- Inline code via standard mark styling.
- Code block with:
  - monospace styling
  - optional language label selector
  - paste-friendly plain-text behavior

**Public Help Center rendering**

- Continue rendering `<pre><code class="language-x">...</code></pre>`.
- Add a more intentional code theme and preserve whitespace.

**Help Scout import**

- Map `<pre><code>` directly to `codeBlock`.
- Parse syntax classes when present into `language`.

**Markdown/export**

- Inline code -> backticks
- Code block -> fenced code blocks with language

**Risk**

- Low to medium.

**Likely failure points**

- malformed imported HTML with nested markup inside `<code>`
- language class normalization across Help Scout variants

### Divider

**Editor insertion**

- Slash menu item `Divider`
- optional toolbar button

**Built-in vs custom**

- Use built-in `HorizontalRule`

**Tiptap JSON storage**

- `type: "horizontalRule"`

**Editor rendering**

- Styled `<hr>` block with spacing handles

**Public Help Center rendering**

- Render `<hr>`
- add Help Center styling so it looks intentional rather than browser-default

**Help Scout import**

- Map `<hr>` directly

**Markdown/export**

- `---`

**Risk**

- Low

**Likely failure points**

- mostly visual consistency only

### Emoji

**Editor insertion**

- Inline insertion only
- Slash menu item `Emoji`
- searchable emoji picker popover / dialog
- recently used row if practical
- category tabs if practical

**Built-in vs custom**

- Recommend a **custom picker UI with plain Unicode insertion**
- Do **not** store emoji as a custom persisted node unless we later need custom sprite rendering or analytics

**Tiptap JSON storage**

- Store as normal `text` content containing Unicode emoji

Example:

```json
{
  "type": "text",
  "text": "✅"
}
```

**Editor rendering**

- Renders as normal text in the editor
- picker owns search, category, and insertion UX

**Public Help Center rendering**

- Normal text rendering

**Help Scout import**

- Unicode emoji should already survive HTML import naturally
- if Help Scout emits emoji images rather than Unicode in some cases, map known emoji-image patterns back to Unicode during import

**Markdown/export**

- No special handling
- Unicode text should round-trip cleanly in markdown and HTML export

**Risk**

- Low

**Likely failure points**

- poor picker UX
- accidental shortcode storage if implementation mixes models

### HTML

**Editor insertion**

- Slash menu item `HTML`
- opens a code-editor dialog for editing the block source
- block preview in the document with clear “sanitized HTML” framing

**Built-in vs custom**

- Custom block node

**Recommended model**

- Node name: `htmlBlock`
- Attrs:
  - `html: string`
- The stored `html` should be the **sanitized canonical fragment**, not arbitrary raw source

Example:

```json
{
  "type": "htmlBlock",
  "attrs": {
    "html": "<div class=\"docs-html-block\"><p>Safe custom snippet</p></div>"
  }
}
```

**Security recommendation**

- Treat this as a **safe HTML fragment block**, not an executable embed surface
- server-side sanitization is required before persistence and before public render
- disallow:
  - `script`
  - `style`
  - `iframe`
  - `object`
  - `embed`
  - `form`
  - `input`
  - `button`
  - `link`
  - event-handler attrs (`on*`)
  - `javascript:` / `data:` / unsafe URLs
- allow only a controlled subset of structural/content tags
- video must use the dedicated video block, not HTML block

**Recommended allowlist direction**

- tags:
  - `div`
  - `span`
  - `p`
  - `strong`
  - `em`
  - `b`
  - `i`
  - `u`
  - `s`
  - `a`
  - `ul`
  - `ol`
  - `li`
  - `br`
  - `hr`
  - `code`
  - `pre`
  - `table`
  - `thead`
  - `tbody`
  - `tr`
  - `th`
  - `td`
  - `img`
- attrs:
  - safe global attrs only where needed
  - `href`, `target`, `rel` on links after URL sanitization
  - `src`, `alt`, `width`, `height` on images after URL sanitization
  - no arbitrary classes unless we explicitly support them
  - no inline styles in v1 unless we deliberately allow a tightly restricted CSS subset

**Editor rendering**

- Render sanitized preview in a non-inline-editable NodeView
- show a warning badge when input was modified by sanitization

**Public Help Center rendering**

- Output the sanitized fragment inside a block wrapper
- never trust raw client HTML

**Help Scout import**

- Use `htmlBlock` only for content that cannot be mapped to a native supported node but remains safe after sanitization
- do not force raw HTML blocks into markdown
- unsafe fragments should be:
  - stripped and warned, or
  - preserved as escaped code if user fidelity is more important than visual parity

**Markdown/export**

- Markdown export should emit raw HTML block using the sanitized fragment
- HTML export is the correct high-fidelity path for this block

**Risk**

- High

**Likely failure points**

- unsafe tags or attrs slipping through
- layout-breaking CSS if style attrs are allowed too broadly
- authors expecting arbitrary JS/embed behavior from an HTML block

### Image

**Editor insertion**

- toolbar button
- slash menu item
- paste/drop/file picker
- image edit controls for:
  - alt text
  - caption
  - width
  - remove / replace

**Built-in vs custom**

- Keep a custom image node, but evolve it into a docs-first block rather than the current PM attachment piggyback

**Recommended data model**

- Node name: `docsImage`
- Attrs:
  - `assetId?: string`
  - `src: string`
  - `alt?: string`
  - `caption?: string`
  - `width?: string`
  - `height?: string`
  - `aspectRatio?: number | null`

Example:

```json
{
  "type": "docsImage",
  "attrs": {
    "assetId": "asset_123",
    "src": "https://cdn.helpin.ai/docs/asset_123.png",
    "alt": "Billing dashboard",
    "caption": "The billing dashboard after migration",
    "width": "720px"
  }
}
```

**Editor rendering**

- React NodeView with resize handles, alt/caption editing, preview, copy/download/open actions

**Public Help Center rendering**

- Render `<figure>` with `<img>` and optional `<figcaption>`
- responsive sizing and safe width handling

**Help Scout import**

- Map `<img>` directly
- map `<figure>` + caption into one image node with caption
- preserve re-uploaded S3 URL

**Markdown/export**

- basic images map cleanly to markdown
- captioned images need a fallback:
  - markdown export: image + following caption paragraph, or raw `<figure>` HTML for fidelity
- HTML export is better when caption/layout fidelity matters

**Risk**

- Medium

**Likely failure points**

- current PM attachment infra continuing to leak into Docs
- caption round-tripping
- broken asset ownership or cleanup

### Table

**Editor insertion**

- Slash menu item inserts a default 2x2 table
- add table controls for:
  - add/remove row
  - add/remove column
  - toggle header row

**Built-in vs custom**

- Use built-in Tiptap table extensions

**Tiptap JSON storage**

- Standard `table`, `tableRow`, `tableHeader`, `tableCell`

**Editor rendering**

- editable table UI
- horizontal overflow container
- selection styling

**Public Help Center rendering**

- keep table tags, but wrap large tables in a responsive overflow container
- improve CSS for readability

**Help Scout import**

- direct HTML table -> table nodes when structure is simple and supported
- merged-cell or span-heavy tables should degrade predictably:
  - either preserve as sanitized `htmlBlock`
  - or flatten to simple table only when safe
- do not silently drop columns/cells

**Markdown/export**

- simple tables map to pipe-table markdown
- complex tables should fallback to raw HTML block

**Risk**

- Medium

**Likely failure points**

- colspan / rowspan
- nested block content inside cells
- mobile Help Center readability

### Video

**Editor insertion**

- Slash menu item `Video`
- opens URL dialog
- validates and normalizes provider URL before insertion

**Built-in vs custom**

- Custom node

**Recommended safest model**

- URL-based embed only
- whitelist supported providers
- do not support arbitrary iframe HTML through the video block
- do not support self-hosted uploaded video in this phase unless product explicitly wants the much larger storage/transcoding surface

**Recommended providers**

- YouTube
- Vimeo
- Loom
- Wistia

The exact allowlist should be confirmed before build, but it must be finite.

**Tiptap JSON storage**

- Node name: `videoEmbed`
- Attrs:
  - `provider`
  - `sourceUrl`
  - `embedUrl`
  - `title?: string`

Example:

```json
{
  "type": "videoEmbed",
  "attrs": {
    "provider": "youtube",
    "sourceUrl": "https://www.youtube.com/watch?v=abc123",
    "embedUrl": "https://www.youtube.com/embed/abc123"
  }
}
```

**Editor rendering**

- provider card with preview iframe or poster
- edit URL / remove controls
- failed validation should block insertion

**Public Help Center rendering**

- responsive iframe only from validated `embedUrl`
- strict provider validation on render
- add `sandbox`, `allowfullscreen`, `referrerpolicy`, and conservative allow attrs

**Help Scout import**

- map supported provider iframes into `videoEmbed`
- unsupported iframes:
  - downgrade to plain link, or
  - preserve as sanitized HTML only if explicitly allowed and safe
- do not treat arbitrary iframe sources as valid video embeds

**Markdown/export**

- markdown fallback should be the canonical source URL on its own line
- HTML export can preserve iframe embed

**Risk**

- High

**Likely failure points**

- unsupported provider URLs
- provider URL normalization bugs
- unsafe iframe attrs or host spoofing

---

## 3. Import Compatibility Impact

### Which new blocks materially improve Help Scout import fidelity

These blocks directly improve fidelity:

- `Callout`
  - stops collapsing Help Scout callouts into generic blockquotes
- `HTML`
  - preserves safe HTML fragments that do not map to native blocks
- `Video`
  - stops converting iframe videos into plain links
- `Table`
  - preserves real tables instead of flattening or degrading them
- `Code`
  - preserves code blocks with language metadata where present
- `Image`
  - preserves figures/captions and stable owned image URLs
- `Divider`
  - preserves `<hr>` directly

This one is mostly already okay:

- `Emoji`
  - if Help Scout stores emoji as Unicode, it already survives content conversion conceptually
  - the improvement here is authoring parity, not import fidelity

### What is likely failing today

Current likely failures or degradations:

- Help Scout callouts become plain blockquotes
- Help Scout HTML blocks lose styling or collapse completely
- video iframes become plain links
- imported docs can render blank in the public Help Center until opened internally because of `_markdown_source`
- complex tables are lossy
- figure/image caption structure is lossy
- internal article links are preserved as old Help Scout URLs instead of being rewritten to canonical Helpin paths

### Required importer changes

The importer should change from:

- `HTML -> Markdown -> _markdown_source envelope`

to:

- `HTML preprocess -> canonical Tiptap JSON -> direct save`

Recommended import pipeline:

1. Fetch raw Help Scout HTML
2. Re-upload external images to owned storage
3. Normalize internal Help Scout article links using the import mapping table
4. Parse HTML into canonical Tiptap JSON using the docs schema bundle
5. Map unmatched but safe fragments into `htmlBlock`
6. Record structured import warnings for ambiguous mappings
7. Save canonical Tiptap JSON directly

### Node mapping for Help Scout import

- `<blockquote>` -> `blockquote`
- Help Scout callout wrappers -> `callout`
- `<pre><code>` -> `codeBlock`
- `<hr>` -> `horizontalRule`
- Unicode emoji -> text nodes
- raw safe custom HTML -> `htmlBlock`
- `<img>` / `<figure>` -> `docsImage`
- `<table>` -> table nodes when supported, else `htmlBlock`
- whitelisted `<iframe>` video -> `videoEmbed`
- unsafe / unsupported iframe -> plain link plus warning, or sanitized `htmlBlock` only if policy allows it

---

## 4. Help Center Rendering Strategy

### Rendering contract

Help Center rendering must remain a strict server-owned contract:

- editor authors should not be able to create nodes that the Help Center cannot render
- public clients should continue consuming safe HTML, not arbitrary raw JSON or raw user HTML

### Recommended renderer model

Keep the current public API shape:

- server returns `content_html`

But upgrade the render contract so that:

- every supported docs node has an explicit Go render path
- unknown nodes fail validation before publish
- HTML and video nodes are sanitized/validated before render

### Shared renderer/component system needed

We need a shared **capability matrix**, not necessarily one literal runtime renderer shared across Go and React.

Recommended contract:

- one canonical node list and attr contract
- one importer schema bundle
- one editor extension bundle
- one Go public renderer with matching node support
- one test fixture set that proves the same JSON content:
  - edits in the editor
  - survives import
  - renders in Help Center
  - exports acceptably

### Which blocks are safe to render directly

Safe with straightforward direct rendering:

- blockquote
- code
- divider
- emoji
- image
- table

Require extra sanitization or validation:

- callout
  - variant validation
- HTML
  - strict sanitization
- video
  - provider allowlist + embed URL validation

### How to avoid editor-only blocks

Add a server-side validator that runs on save and on publish:

- validates node names
- validates attr shapes
- validates provider URLs
- sanitizes HTML blocks
- rejects or normalizes unsupported content

For external/public docs:

- publish should fail if the content contains a node that cannot be rendered safely in the Help Center

That is the guardrail that prevents “editor saved it, Help Center can’t render it”.

---

## 5. Markdown/Export Strategy

### Overall recommendation

Keep markdown export as **best-effort**, not as the canonical fidelity target.

For full fidelity, add or preserve HTML-based export paths because these blocks do not all map cleanly to markdown:

- callout
- HTML
- video
- complex tables
- captioned images

### Per-block markdown/export behavior

- `Blockquote`
  - clean markdown

- `Callout`
  - raw HTML block in markdown export for fidelity
  - optional plain blockquote fallback only as a lossy downgrade

- `Code`
  - clean markdown

- `Divider`
  - clean markdown

- `Emoji`
  - preserve Unicode text

- `HTML`
  - raw sanitized HTML block in markdown export
  - HTML export preferred

- `Image`
  - plain image maps cleanly
  - captioned image needs HTML fallback or image + caption paragraph downgrade

- `Table`
  - simple tables to pipe markdown
  - complex tables to raw HTML block

- `Video`
  - markdown export should fallback to source URL
  - HTML export preserves the actual embed

### Markdown source mode implications

Because the editor already has markdown source mode, the source-mode contract must stay predictable:

- standard nodes should round-trip to clean markdown
- callout / HTML / video must round-trip through explicit HTML fallback blocks
- source mode must not silently destroy non-markdown-native nodes

That means the markdown serializer/parser work must be part of the feature, not an afterthought.

---

## 6. Risk Analysis

### Highest-risk issues

1. **Malformed imported HTML**
   - Help Scout content can contain nested divs, inline styles, figures, iframes, and odd wrappers
   - Mitigation:
     - preprocess imported HTML
     - use schema-based HTML -> JSON conversion
     - preserve unknown safe fragments in `htmlBlock`
     - generate import warnings

2. **Unsafe HTML**
   - HTML blocks are the most dangerous feature in this scope
   - Mitigation:
     - sanitized canonical storage
     - no scripts, no inline event handlers, no arbitrary iframe embeds
     - server-side sanitizer as source of truth

3. **Unsupported or spoofed video URLs**
   - Users can paste non-provider URLs or deceptive provider-like URLs
   - Mitigation:
     - parse URL by hostname/path rules
     - store normalized provider + embed URL
     - reject unsupported providers before persistence

4. **Partially supported nodes**
   - adding an editor extension without importer/export/public renderer parity would recreate the current mismatch
   - Mitigation:
     - do not ship a node until all four surfaces exist:
       - editor
       - storage validation
       - Help Center renderer
       - export behavior

5. **Imported content saved but not renderable publicly**
   - this is already happening with `_markdown_source`
   - Mitigation:
     - import directly to canonical JSON
     - validate renderability before marking import complete

6. **Migration concerns for old docs**
   - existing docs contain only the current subset, and imported docs may contain `_markdown_source`
   - Mitigation:
     - add a one-time backfill path for `_markdown_source` docs
     - keep old nodes rendering during migration
     - do not change existing node names lightly

7. **Older documents or versions breaking after schema expansion**
   - custom nodes can destabilize historical versions if names or attrs change
   - Mitigation:
     - version node attrs carefully
     - keep defaults stable
     - add render fixtures for historical content

### Specific failure-prevention rules

- No new node ships without:
  - save/load test
  - Help Center render test
  - markdown/export test
  - Help Scout import fixture if relevant

- No HTML or video node ships without:
  - hostile input tests
  - sanitizer/provider validation tests

- No import work should continue using `_markdown_source` for Help Scout

---

## 7. Recommended Implementation Order

### Order based on dependency chain and risk

1. **Create the canonical docs schema bundle**
   - define the supported node contract first
   - this is the dependency for editor, import, and export

2. **Replace the Help Scout import contract**
   - move from markdown envelope to direct canonical JSON import
   - this has the biggest impact on fidelity and fixes the current public-render gap

3. **Upgrade the public Help Center renderer contract**
   - extend the Go renderer and validation layer to cover the full node set
   - without this, editor work can outpace public rendering

4. **Ship low-risk native blocks**
   - blockquote
   - code
   - divider
   - table

5. **Ship the image model upgrade**
   - docs-first image node with caption/asset ownership
   - image fidelity matters heavily for imported Help Scout content

6. **Ship callout**
   - strong Help Scout parity impact
   - medium complexity

7. **Ship emoji picker**
   - low-risk UX completion

8. **Ship video**
   - high-risk because of provider validation and public embeds

9. **Ship HTML block last**
   - highest security risk
   - depends on sanitization policy, validation, and public render guardrails being fully in place

10. **Finish export/source-mode hardening**
   - markdown fallback coverage
   - HTML export coverage
   - golden round-trip tests

### Why this order is best

- It fixes the current import architecture problem early.
- It establishes the schema/render contract before adding risky nodes.
- It moves from low-risk/high-confidence blocks to high-risk/high-security blocks.
- It minimizes the chance of creating new editor-only or import-only behavior.

---

## 8. Critical Decisions/Questions To Confirm Before Implementation

1. **Do we keep the current Go public renderer or materialize/stash public HTML from a Node-based renderer?**
   - Recommendation: keep Go runtime rendering, but use a shared schema contract and extend the Go renderer deliberately.

2. **Do Docs images continue using PM attachment infrastructure, or do we introduce a docs-first asset model?**
   - Recommendation: introduce a docs-first or generic content-asset model. Continuing to piggyback PM attachments is a hack.

3. **What exact video provider allowlist is in scope for v1?**
   - Recommendation: confirm the finite list before implementation.

4. **How much HTML styling do we allow in `htmlBlock`?**
   - Recommendation: start with sanitized structural HTML and no arbitrary inline styles unless product explicitly accepts the theme/layout risk.

5. **What is the exact Help Scout callout-to-variant mapping in the real imported HTML?**
   - Recommendation: capture real sample HTML from Help Scout before implementation and lock the mapping table before coding.

6. **Do we want HTML export as a first-class export option in addition to markdown?**
   - Recommendation: yes, because markdown alone is not a full-fidelity export target for this node set.

7. **Do we block public publish when validation/sanitization changes content materially?**
   - Recommendation: yes for unsafe HTML/video, and at minimum surface a visible warning for lossy normalization.

8. **Should unsupported imported fragments become sanitized `htmlBlock` nodes or escaped code blocks?**
   - Recommendation: use sanitized `htmlBlock` for safe visual fidelity, escaped code only for unsafe or non-renderable fragments.

9. **Do we support self-hosted uploaded video now?**
   - Recommendation: no, unless product explicitly wants the much larger storage/transcoding/captioning surface. Help Scout parity is better served by provider embeds.
