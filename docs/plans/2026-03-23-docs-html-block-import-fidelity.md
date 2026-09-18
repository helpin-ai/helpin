# Docs HTML Block Import-Fidelity Plan

> Source review, 2026-09-17

Historical recommendation from 2026-03-23. HTML blocks are now registered in the
[editor](../../frontend/src/components/docs/DocsEditor.tsx), and
[Help Scout import](../../server/internal/service/docs_import.go) converts HTML to
canonical Tiptap JSON. The missing-block and `_markdown_source` claims below
refer to the earlier implementation; they are not current setup or prioritization
instructions. Import fidelity still depends on the supported conversion rules.

## Recommendation

Add the `HTML` block next.

Do **not** prioritize emoji before this.

Why:

- the Docs editor already supports the main native blocks that matter for authored content: blockquote, callout, code block, divider, image, table, and video
- the biggest remaining gap for imported help-center content is unsupported HTML fragments that do not map cleanly to native nodes
- Help Scout and Zendesk both use an explicit HTML/source-code block for this exact problem
- Document360’s Advanced WYSIWYG editor also preserves unsupported HTML rather than silently flattening it
- emoji matters for authoring polish, but it is low impact for import fidelity because normal emoji already survive as Unicode text

## Current State In Helpin

### What already exists

- Authoring supports slash-inserted blockquote, code block, divider, callout, table, image, and video in [frontend/src/components/docs/DocsEditor.tsx](../../frontend/src/components/docs/DocsEditor.tsx), [frontend/src/components/docs/slash-commands.ts](../../frontend/src/components/docs/slash-commands.ts), [frontend/src/components/docs/CalloutExtension.ts](../../frontend/src/components/docs/CalloutExtension.ts), and [frontend/src/components/docs/VideoEmbedExtension.ts](../../frontend/src/components/docs/VideoEmbedExtension.ts).
- Public Help Center rendering supports those same major blocks in [server/internal/tiptap/html.go](../../server/internal/tiptap/html.go).
- Help Scout import still converts article HTML into Markdown and stores a `_markdown_source` envelope in [server/internal/service/docs_import.go](../../server/internal/service/docs_import.go) and [server/internal/helpscout/convert.go](../../server/internal/helpscout/convert.go).

### Why this is the next bottleneck

- unsupported imported fragments still have nowhere first-class to go
- unknown or custom embeds/layout fragments are still forced through a lossy HTML-to-Markdown path
- Help Scout iframes are still rewritten to links in [server/internal/helpscout/convert.go](../../server/internal/helpscout/convert.go)
- without an `htmlBlock`, the importer has to either flatten content or silently lose styling

## What Other Help-Docs Tools Do

### Help Scout

- Help Scout explicitly exposes an `HTML` block in the slash menu and positions it as the escape hatch for advanced/custom article content.
- It allows article authors to insert unsupported/custom HTML as a dedicated block instead of forcing everything into the normal editor model.
- It also lets authors convert the HTML block back into editor-native format, with the warning that custom classes and inline CSS are removed if unsupported.

Implication for Helpin:

- the HTML block should be a block-level escape hatch, not a full-page source editor
- it should exist alongside native blocks, not replace them
- it should support later “convert to editor format” behavior, but that does not need to be in the first shipping slice

### Zendesk

- Zendesk’s source-code and HTML-block flow exists specifically for unsupported HTML that the WYSIWYG editor cannot model well.
- Zendesk treats security separately from authoring: unsupported or unsafe HTML is constrained by rendering rules, and they are explicit that unsafe content can cause risk.
- Their HTML block is editable in a focused block editor rather than forcing users through the entire article source.

Implication for Helpin:

- we should use a dedicated block editor/modal, not inline plaintext in the main canvas
- sanitization and allowed tags must be a server-owned contract, not “whatever the browser accepts”

### Document360

- Document360’s Advanced WYSIWYG editor preserves unsupported HTML and flags it for correction instead of silently deleting it during parsing.
- Their product direction is to keep unsupported content visible and recoverable.

Implication for Helpin:

- imported unsupported content should be preserved in a visible block
- parse failures should not silently flatten to paragraphs if the content can instead live in `htmlBlock`

## Recommended Product Shape

Use one custom `htmlBlock` node with a focused edit dialog.

### Data model

Store block HTML as a single block node:

```json
{
  "type": "htmlBlock",
  "attrs": {
    "html": "<div>...</div>"
  }
}
```

Recommended behavior:

- block-level only
- not nestable inside callouts, table cells, list items, or inline text
- the canonical stored value is the authored HTML fragment
- rendering always goes through sanitization/validation before preview or public output

### Why this model

- it is simple
- it preserves imported fragments without inventing many one-off nodes
- it matches Help Scout/Zendesk’s “HTML block” mental model
- it can later support “convert to native blocks” if we want to parse supported subsets back into Tiptap nodes

## Plan

### Phase 1: Define the HTML block contract

Create the contract first, before UI work.

- Add a custom `htmlBlock` Tiptap node for Docs.
- Restrict it to block usage only.
- Decide the allowed HTML subset for Helpin.
- Define one sanitizer policy for:
  - editor preview
  - docs save/update validation
  - Help Center public rendering
  - import pipeline

Recommended allowed baseline:

- structural/content tags like `div`, `section`, `figure`, `figcaption`, `dl`, `dt`, `dd`, `details`, `summary`
- safe text tags like `span`, `strong`, `em`, `code`, `pre`
- safe links and images with strict attribute validation
- `iframe` only through explicit allowlist, ideally by converting known providers into native `videoEmbed`

Recommended disallowed baseline:

- `script`
- `style`
- form elements
- inline event handlers like `onclick`
- `javascript:` URLs
- arbitrary `srcdoc`
- arbitrary external embeds without allowlisting

### Phase 2: Add authoring UX

Add a dedicated HTML block authoring flow in the editor.

- Add `HTML` to the slash menu in [frontend/src/components/docs/slash-commands.ts](../../frontend/src/components/docs/slash-commands.ts).
- Insert an empty `htmlBlock` node from slash menu.
- Open a focused modal or side-sheet editor for editing the block’s HTML.
- Render a block placeholder/preview in the editor canvas.
- Support:
  - insert
  - edit
  - replace
  - delete

Recommended UX:

- show a label like `HTML Block`
- show a sanitized preview when renderable
- if sanitization strips content, show a warning badge and a “Review HTML” action

### Phase 3: Add server-owned sanitization and validation

The editor must not be the trust boundary.

- Add a backend sanitizer/validator for `htmlBlock`.
- Validate HTML fragments on save/update and on publish.
- Reject or normalize unsafe markup before it reaches public Help Center HTML.
- Make `docs_helpcenter` rendering depend on sanitized output, not raw block HTML.

Recommended behavior:

- store authored HTML in the node
- sanitize every time before public rendering
- optionally return save-time warnings for stripped markup so authors know when content changed

### Phase 4: Add public Help Center rendering

Extend [server/internal/tiptap/html.go](../../server/internal/tiptap/html.go) to render `htmlBlock`.

Recommended render contract:

- if the sanitized fragment is safe and non-empty, render it
- if sanitization removes everything, render nothing or a benign placeholder in preview-only contexts
- do not render raw unsanitized HTML into `content_html`

This avoids a mismatch where the editor can create something the Help Center cannot display.

### Phase 5: Wire importer fallback to `htmlBlock`

Once `htmlBlock` exists, the importer should use it deliberately.

Recommended mapping order for imported Help Scout content:

1. map supported HTML to native blocks first
   - blockquote -> `blockquote`
   - callout HTML -> `callout`
   - code/pre -> `codeBlock`
   - hr -> `horizontalRule`
   - table -> `table`
   - safe supported video URLs -> `videoEmbed`
   - img/figure -> native image node
2. wrap remaining unsupported but preservable fragments in `htmlBlock`
3. only fall back to plain paragraphs/links when the HTML is truly unsafe or empty

This is the main reason to add `htmlBlock` next.

### Phase 6: Add export behavior

Define export behavior up front so the block is not editor-only.

Recommended behavior:

- Markdown export:
  - emit raw HTML fragment if markdown export is in permissive HTML mode
  - otherwise emit a fenced fallback marker like:
    - `HTML block omitted in plain markdown export`
- `.doc` / HTML export:
  - include sanitized HTML
- Copy-as-Markdown:
  - preserve block HTML only if safe export mode is enabled

### Phase 7: Add warnings and failure visibility

Do not silently mutate imported or authored HTML.

- Show when markup was stripped or normalized.
- Surface warnings for:
  - unsupported tags removed
  - unsafe attributes removed
  - unsupported embeds converted to plain links
- Keep the visible block even when parts were removed, so authors can fix it

## Risks

### High risk

- unsafe HTML reaching public Help Center rendering
- allowing arbitrary embeds that bypass the provider validation model built for `videoEmbed`
- storing content that the editor shows but the Help Center later strips

### Medium risk

- imported HTML fragments becoming “opaque blocks” that authors cannot easily edit without code comfort
- markdown export degrading if raw HTML is emitted into systems that do not accept it
- style drift if imported classes depend on site CSS that Helpin does not have

### Low risk

- authors expecting inline HTML rather than block-only HTML
- confusion between `videoEmbed` and “paste iframe into HTML block”

## What Not To Do

- do not build raw free-form HTML insertion with no sanitization policy
- do not keep `_markdown_source` as the long-term import target
- do not support editor-only HTML that the Help Center cannot render
- do not let HTML blocks appear inline inside normal paragraphs
- do not use HTML block as the first parser target for things that already have native nodes

## Why Not Emoji Next

Emoji is still worth adding later, but not next.

Why it is lower priority:

- plain Unicode emoji already survives import/export reasonably well
- emoji does not address the current style-preservation problem
- emoji has much lower impact on public Help Center rendering correctness
- HTML block directly improves imported-content fidelity and gives the importer a safe fallback target

## Recommended Build Order

1. Add `htmlBlock` schema + sanitizer contract
2. Add editor insert/edit/delete UX
3. Add public Help Center rendering
4. Add importer fallback mapping into `htmlBlock`
5. Add export behavior and warning surfaces
6. After that, add emoji picker if you want Help Scout-level authoring polish

## Success Criteria

- authors can insert and re-edit a dedicated HTML block in Docs
- saved docs render safely in the public Help Center
- imported Help Scout fragments that do not map to native nodes are preserved in `htmlBlock`
- unsupported or unsafe markup never silently becomes dangerous public HTML
- markdown/html export has defined fallback behavior
