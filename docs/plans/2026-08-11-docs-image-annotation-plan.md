# Docs Image Annotation — Implementation Plan (react-konva)

Date: 2026-08-11
Status: Proposed
Supersedes: the marker.js variant of this plan (same file, earlier revision)

## Goal

Let people annotate images in Helpin docs — arrows, boxes, callouts, text, highlight, and
redaction — and save the result so it renders everywhere the doc renders, while staying
re-editable later.

## Library decision: build on react-konva

| candidate | license | verdict |
|---|---|---|
| `konva` 10.3.0 + `react-konva` 19.2.5 | MIT | **chosen** — 55 KB gzip, zero deps, actively maintained |
| `@markerjs/markerjs3` | Linkware | 23 KB gzip, but requires attribution or a paid license, and locks our persisted state to a third-party schema |
| `tldraw` 5.3.0 | commercial | requires a production license key; without one it refuses to run outside localhost |
| `filerobot-image-editor` 5.0.0 | MIT | built on react-konva, but is a full photo editor and pulls in `styled-components` |
| `tui-image-editor` | MIT | abandoned since 2022-04, 3.5 MB, fabric v4 |
| `@annotorious/react` | BSD-3 | W3C region model — no arrows or callouts |

Konva supplies the parts that are genuinely hard: `Transformer` (drag/resize/rotate handles),
hit detection, and `stage.toDataURL()` for flattening. We supply tool definitions, a toolbar,
and a state schema.

The decisive argument is **state ownership**, not licensing. Annotation state is persisted
inside document HTML and therefore lives in doc version history permanently. A third-party
schema there is a permanent coupling — a breaking format change or a library swap makes every
historical annotation unreadable. Our own schema is a flat array we can migrate.

Cost of this choice, stated plainly: roughly 600–900 lines we own and maintain, versus about a
day of integration for marker.js. Accepted deliberately.

**Licensing hygiene:** marker.js is linkware — architecture and UX may be studied, but do not
port its source or reproduce its state schema. MIT sources (konva examples, filerobot) may be
borrowed from with attribution retained.

## Core design decision: flatten for rendering, keep state for re-editing

The saved image is a flattened PNG; the annotation state is JSON kept only for re-editing.
Readers pay 0 KB — the annotator is lazy-loaded and never enters a read path.

The alternative (live overlay rendering) is rejected because of the read surfaces:

- `help-center/src/components/ArticleContent.tsx` renders article HTML through
  `dangerouslySetInnerHTML`, and the help center is server-prerendered
  (`serverRenderCache.mjs`, `serverSeo.mjs`). An overlay would mean hydrating an annotation
  runtime on every public article.
- `frontend/src/components/docs/htmlSanitizer.ts` allows a fixed attribute list; published HTML
  also feeds search indexing and embeddings.
- Doc export/import (`server/internal/handler/docs_import.go`) expects plain `<img>`.

This mirrors the existing `excalidrawBlockToImage` transform in
`frontend/src/lib/docsPublishTransforms.ts`, which already flattens an interactive block to an
image at publish time.

### Where annotation state lives

On the TipTap node attributes (`annotationState`, serialized to `data-annotation`), not in a new
backend table:

- Doc version history and change proposals restore the correct annotation state for free. A
  server-side table keyed by attachment ID would drift when an old version is restored.
- Document duplication copies annotations with no extra hook.
- No migration, no orphan-cleanup job.
- Typical state for 5–10 shapes is 1–3 KB of JSON.

The tradeoff is HTML weight, mitigated by stripping the attribute at publish time (Phase 6).

### Data flow

```
original attachment (never mutated)
  └─ annotate → konva stage → PNG blob at natural resolution
       └─ uploadEditorImage() → new attachment
            └─ node attrs: src = rendered, sourceAttachmentId = original,
                           annotationState = our JSON
```

Re-edit loads `sourceAttachmentId` as the base image and replays `annotationState`, so
annotations never compound onto already-flattened pixels.

## State schema (v1)

Coordinates are in **source-image pixel space**, never display space — this is what makes
re-editing correct at any dialog size or zoom level.

```ts
export type AnnotationShape =
  | { id: string; type: 'arrow';     points: [number, number, number, number]; color: string; strokeWidth: number }
  | { id: string; type: 'rect';      x: number; y: number; width: number; height: number; color: string; strokeWidth: number; rotation: number }
  | { id: string; type: 'ellipse';   x: number; y: number; radiusX: number; radiusY: number; color: string; strokeWidth: number; rotation: number }
  | { id: string; type: 'text';      x: number; y: number; text: string; color: string; fontSize: number; rotation: number }
  | { id: string; type: 'callout';   x: number; y: number; width: number; height: number; text: string; color: string; tailX: number; tailY: number }
  | { id: string; type: 'highlight'; x: number; y: number; width: number; height: number; color: string }   // semi-transparent fill
  | { id: string; type: 'cover';     x: number; y: number; width: number; height: number; color: string }   // opaque redaction
  | { id: string; type: 'freehand';  points: number[]; color: string; strokeWidth: number }

export type AnnotationState = {
  version: 1
  baseWidth: number    // natural width of the source image
  baseHeight: number
  shapes: AnnotationShape[]
}
```

`version` is present from day one so a future migration is a switch statement, not an
archaeology project. Unknown `type` values must be skipped on load, never thrown on.

## Security note: redaction and the retained original

This needs a decision — it is the one place where the design's strength becomes a hazard.

Because the original attachment is retained and `sourceAttachmentId` points at it, a `cover`
shape used to redact sensitive data **does not remove that data from the system**. The
unredacted original stays in object storage and is reachable through the attachment content
URL by anyone who can read the document.

Mitigation, included in scope: when the saved state contains any `cover` shape, the save dialog
offers **"Flatten permanently"** — upload the render, then drop `sourceAttachmentId` and
`annotationState` from the node and delete the source attachment. That annotation becomes
non-re-editable, which is the correct trade for redaction. Default the toggle **on** whenever a
cover shape is present.

## Placement: in `frontend`, not a workspace package (yet)

The annotator lives at `frontend/src/components/docs/annotator/`. It is **not** extracted to
`packages/*` for the first release.

Reasoning: `packages/*` in this repo is for code that ships to third parties (`sdk-js`,
`widget-core`, `widget-embed`, `react`, `nextjs`) or is genuinely shared across apps
(`support-core`, consumed by `apps/support-desktop`). The annotator has exactly one consumer
today. A package would add turbo build ordering, a build step before `frontend` can typecheck,
`vite-plugin-dts` config, and versioning — all paid during Phase 3, which is where the code
churns most.

**The boundary is enforced from day one so extraction stays cheap:**

```
annotator/
  core/          # imports ONLY react, konva, react-konva — no @/ imports, ever
    annotationTypes.ts
    ImageAnnotator.tsx
    shapes.tsx
    AnnotatorToolbar.tsx
    useAnnotationHistory.ts
    renderAnnotations.ts
  integration/   # Helpin-specific: dialog, upload, toasts, design tokens
    DocsImageAnnotateDialog.tsx
```

Enforce it with an ESLint `no-restricted-imports` rule banning `@/` inside `core/`. That single
rule is what turns a future extraction into a directory move plus a `package.json`.

**Extraction trigger:** a second real consumer. The likely candidate is
`apps/support-desktop` (agents annotating screenshots in replies) — at that point extract to
`packages/image-annotator` and follow the `support-core` precedent.

**One future consumer to rule out now:** the embeddable chat widget. `widget-core` is Preact and
ships on customers' sites, where bundle weight is the primary constraint. Adding 55 KB gzip of
konva there is not viable — if end-user screenshot annotation is ever wanted in the widget, it
needs a separate minimal implementation, not this one. Do not plan the package API around it.

## Current state of the code

- `frontend/src/components/ui/resizable-image-extension.ts` — the docs image node
  (`resizableImage`), already carries `attachmentId` / `artifactId` / `caption`.
- `frontend/src/components/ui/resizable-image-component.tsx` — node view with the floating
  toolbar; the **Annotate** button goes next to the existing "Edit with AI" wand (line ~396).
- `frontend/src/components/docs/DocsImageEditDialog.tsx` — has a red-brush canvas, but it is a
  mask for AI inpainting (`docsService.editImage`), not user annotation, and is not
  re-editable. It stays as-is; the annotator is separate and separately labelled.
- `frontend/src/hooks/useEditorImageUpload.ts` — `uploadEditorImage(file, config)` does
  presign → S3 → confirm and returns `{ attachmentId, publicUrl }`. Reused unchanged.
- `frontend/src/lib/docsPublishTransforms.ts` — where the publish-time strip goes.

**Backend: no changes required** — no migration, no endpoint, no service or repository work.
The exception is the redaction path above, which needs attachment *deletion*; verify
`pmAttachmentService` already exposes it before Phase 5.

## Phases

### Phase 1 — Dependencies
- `pnpm add konva react-konva --filter frontend`
- Annotator is `React.lazy` + dynamic `import()`. Verify with `pnpm build` that konva lands in
  its own chunk and not the main bundle.

### Phase 2 — Node attributes
In `resizable-image-extension.ts`:
- Add attrs `annotationState` (default `null`) and `sourceAttachmentId` (default `null`).
- `parseHTML`: read `data-annotation` with `JSON.parse` in a try/catch — malformed state must
  yield `null`, never throw, or one bad attribute breaks document loading.
- `renderHTML`: emit both only when present, matching the existing conditional-spread style.

### Phase 3 — The annotator
New directory `frontend/src/components/docs/annotator/` (see Placement above for the
`core/` vs `integration/` split and the lint rule that enforces it):

| file | responsibility |
|---|---|
| `core/annotationTypes.ts` | schema above + `parseAnnotationState()` guard |
| `core/ImageAnnotator.tsx` | konva `Stage` / `Layer`, selection, `Transformer` wiring |
| `core/shapes.tsx` | one renderer per shape type |
| `core/useAnnotationHistory.ts` | undo/redo over the `shapes` array |
| `core/renderAnnotations.ts` | state + image → PNG blob at natural resolution |
| `integration/AnnotatorToolbar.tsx` | tool picker, color, stroke width, undo/redo, delete |
| `integration/DocsImageAnnotateDialog.tsx` | dialog shell, upload, toasts, permanent-flatten toggle |

The toolbar sits in `integration/`, not `core/`: it is Helpin chrome (shadcn buttons, `@/lib/icons`,
tooltips) and would violate the core import rule. `core/ImageAnnotator` is therefore fully
controlled — tool, color, and stroke width come in as props — which is the better boundary anyway.

Implementation notes that will otherwise cost a day each:

- **Scaling.** Stage is sized to fit the dialog; keep `scale = displayWidth / baseWidth` and
  convert pointer coordinates back to source space on every write. Never store display coords.
- **Export resolution.** `stage.toDataURL({ pixelRatio: baseWidth / displayWidth })` so the
  output matches source pixels. Skipping this is exactly the blurry-screenshot failure people
  hit with flattening annotators.
- **Cross-origin.** Load the base image with `crossOrigin="anonymous"` (as
  `DocsImageEditDialog` line 111 already does) or the canvas taints and export throws.
- **Arrows don't take a Transformer** cleanly — give them two draggable endpoint handles
  instead. Rect/ellipse/text/callout use `Transformer`.
- **Text needs an HTML overlay.** Konva has no text input; position a `<textarea>` over the
  node during editing, the standard konva pattern.
- **Redaction is `cover`, not blur.** Blur and pixelate are reversible in principle; an opaque
  fill genuinely destroys the pixels in the flattened output. Combined with the permanent-flatten
  path above, this is the honest redaction story.

### Phase 4 — Save pipeline
- `renderAnnotations()` → PNG blob → `new File([blob], 'annotated-<name>.png')` →
  `uploadEditorImage(file, uploadConfig)`.
- `updateAttributes({ src, attachmentId, sourceAttachmentId, annotationState })`.
- Preserve `width` / `height` / `aspectRatio` / `alignment` / `caption` — the render is natural
  size, so the node must not reset to default width.
- If "Flatten permanently" is selected, omit `sourceAttachmentId` / `annotationState` and delete
  the source attachment.
- Failure path: toast, leave the node untouched (mirrors `handleImageUpload`'s catch).

### Phase 5 — Toolbar wiring
- Annotate button in `resizable-image-component.tsx`, gated on `editable` and an available
  upload config. Label "Edit annotations" when `annotationState` is present.
- **Wiring gap:** `ResizableImageOptions` carries only `workspaceId` / `documentId`.
  `DocsEditor.tsx:1370` passes both; `tiptap-editor.tsx:347` (PM comments, support replies)
  passes neither. Add `uploadConfig?: EditorUploadConfig` to the extension options and pass it
  from both call sites — that is what makes annotation work outside docs. Hide the button when
  it is absent rather than failing at save time.

### Phase 6 — Publish transform
- Strip `data-annotation` from published HTML in `docsPublishTransforms.ts`.
  `data-source-attachment-id` may stay (cheap, useful for provenance) — **except** on documents
  published to the help center, where it would expose the pre-redaction original to the public.
  Strip both on the help-center path.

### Phase 7 — Tests
- Schema round-trip: attrs → `renderHTML` → `parseHTML` preserves state; malformed
  `data-annotation` yields `null`; unknown shape types are dropped, not thrown on. Sits
  alongside `frontend/src/components/pm/__tests__/editorImageAttachments.test.ts`.
- Coordinate math: display ↔ source conversion round-trips at several scale factors.
- Save pipeline with `uploadEditorImage` mocked: correct attrs written, dimensions preserved,
  failure leaves the node unchanged, permanent-flatten drops state and deletes the source.
- Publish transform drops `data-annotation`; help-center path drops both attributes.
- `sanitizeHtml` keeps annotated images intact.

## Follow-on: an `annotate_image` tool for agents

Depends on Phases 2–4. Worth designing now because it is the payoff of owning the schema: the
annotation schema **is** the tool input schema, so a vision model emits `AnnotationState`
directly with no adapter layer.

The input already exists. `server/internal/service/agent_runtime_host.go` stores
`browser_screenshot` artifacts from agent-browser runs as first-class run artifacts with content
URLs. The natural loop is: agent takes a screenshot → annotates it to point at what it found →
attaches it to a doc, a support reply, or a task comment.

### Where rendering happens

The backend is Go; konva is browser/Node. Three options, and the choice matters:

| option | verdict |
|---|---|
| **Go renderer** (`github.com/fogleman/gg`, MIT) | **chosen** — self-contained, works where no browser exists |
| Node sidecar running konva + `node-canvas` | one renderer, but a new deployable service with native deps |
| No server render — emit state, let the frontend flatten it | zero new code, but breaks for support replies and email, where no browser is ever involved |

All eight shapes are trivially drawable with `gg` plus `golang.org/x/image/font` for text.

The real cost is **two renderers that can drift**. Contain it with golden-image tests: a fixture
set of `AnnotationState` JSON rendered by both the konva path and the Go path, compared within a
pixel tolerance in CI. Any new shape type must land in both renderers and the fixture set in the
same change. The Go schema mirror lives in `server/internal/model/image_annotation.go` and must
stay in lockstep with `core/annotationTypes.ts` — the `version` field is the guard.

### The hard part: LLM coordinate grounding

This is the main risk to the whole idea, not the rendering. Vision models are unreliable at
emitting exact pixel coordinates on a 2560×1440 screenshot; naive prompting produces arrows
pointing at nothing. Mitigations, in order of preference:

1. **Normalized coordinates.** The tool accepts fractional `0..1` coordinates and multiplies by
   `baseWidth` / `baseHeight` server-side. Models are meaningfully better at proportions than at
   pixels, and it keeps the stored schema in pixel space.
2. **Semantic targets (v2, the real fix).** When the image came from an agent-browser screenshot,
   the browser already knows element bounding boxes. Let the tool accept
   `target: { kind: 'element', ref: '<snapshot-ref>' }` and resolve it to a box server-side, so
   the model names *what* to point at instead of guessing *where*. This is the version worth
   building toward.
3. **Render-and-check.** Feed the rendered result back to the vision model for correction. Works,
   but costs a round trip per annotation.

Ship (1) first and treat coordinate accuracy as an evaluation problem with a fixture set of real
screenshots, not as something that is "done" when the endpoint returns 200.

### Surface and wiring

- MCP tool, registered alongside the existing docs tools and dispatched through
  `MCPService.ExecuteTool` (`server/internal/service/mcp_tools.go`) — it already provides the
  workspace principal, argument validation, and `auditToolSuccess` audit logging.
- Input: `{ source_attachment_id | source_artifact_id, shapes[], coordinate_space }`.
- Output: `{ attachment_id }`, produced through the existing attachment pipeline, so the result
  drops into a doc image node with the same attrs a human annotation produces — including
  `annotationState`, meaning **a person can open an agent's annotation and edit it**. That
  round-trip is the strongest argument for this whole design.
- Permission: reuse `PermDocsEdit` for the docs path; confirm the right permission for the
  support-reply path before implementing.
- Redaction interaction: an agent must not be able to use `cover` as a security control. Either
  reject `cover` shapes from tool input, or force the permanent-flatten path for them. Decide
  before shipping.

## Risks

| Risk | Mitigation |
|---|---|
| Redaction leaves the original readable | Permanent-flatten path, defaulted on when a cover shape exists; strip provenance attr on help-center publish |
| Export resolution mismatch → blurry screenshots | `pixelRatio` derived from natural size; covered by test |
| Canvas taint on cross-origin S3 images | `crossOrigin="anonymous"`, already proven in `DocsImageEditDialog` |
| Scope creep into a general image editor | Fixed v1 tool list: arrow, rect, ellipse, text, callout, highlight, cover, freehand. Crop/filters/resize are explicitly out |
| konva inflating the main bundle | Lazy chunk, verified in `pnpm build` output |
| Two "annotate" affordances confusing users | Distinct labels/icons: "Edit with AI" (wand) vs "Annotate" |
| Re-annotating compounds onto flattened pixels | Always re-open from `sourceAttachmentId`, never from `src` |
| We now own an annotation editor | Accepted; the fixed tool list is the containment strategy |
| Go and konva renderers drift (follow-on) | Golden-image fixtures compared in CI; new shapes must land in both renderers in one change |
| Agent annotations land in the wrong place | Normalized coordinates first, semantic element targets as the real fix; evaluate against real screenshots |

## Open questions

1. Ship annotation in PM comments and support replies at the same time (Phase 5 makes it nearly
   free), or docs-only for the first release?
2. Is permanent-flatten enough for redaction, or does compliance need the original deleted
   unconditionally whenever a cover shape is used?
3. Keep both the original and the render indefinitely for non-redacted images, or garbage-collect
   originals after some period? Keeping both is assumed here and is what makes re-editing
   lossless.

## Estimate

~6 new frontend files, ~4 modified, 0 backend changes. Phase 3 is the bulk of the work;
Phase 5's extension-options wiring is the only piece that reaches outside docs.
