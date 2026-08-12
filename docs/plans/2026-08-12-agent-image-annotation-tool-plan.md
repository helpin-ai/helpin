# `annotate_image` — Agent Image Annotation Tool

Date: 2026-08-12
Status: Proposed
Depends on: `docs/plans/2026-08-11-docs-image-annotation-plan.md` (shipped)

## Goal

Let an agent annotate an image — arrows, boxes, callouts, text, highlight — and attach the result
to a doc, a support reply, or a task comment. The agent takes a screenshot, marks what it found,
and hands back something a person can read at a glance.

## Why this is cheap now

The annotation schema is ours, so it doubles as the tool input schema. A vision model emits
`AnnotationState` directly and there is no translation layer to build or keep in sync. The output
carries the same node attributes a human annotation produces, which means **a person can open an
agent's annotation and edit it** — the agent draws the first pass, the human fixes the arrow.

The input already exists: `server/internal/service/agent_runtime_host.go` stores
`browser_screenshot` artifacts from agent-browser runs as first-class run artifacts with content
URLs.

## Where rendering happens

The backend is Go; the existing renderer is konva in the browser.

| option | verdict |
|---|---|
| **Go renderer** (`github.com/fogleman/gg`, MIT) | **chosen** — self-contained, works where no browser exists |
| Node sidecar running konva | one renderer, but a new deployable with native `canvas` deps |
| Return state only, let the frontend flatten it | no new renderer, but breaks for support replies and email where no browser is ever involved |

All eight shapes are drawable with `gg` plus `golang.org/x/image/font`.

**Font gotcha:** `gg` has no font fallback and scratch/distroless images carry no system fonts.
Embed the TTF used by the frontend (`Inter`) with `go:embed` and load it explicitly, or every text
and callout renders blank.

### Keeping two renderers honest

The cost of a second renderer is drift. Contain it with golden-image tests:

- A fixture set of `AnnotationState` JSON covering every shape type, plus rotation, wrapping text,
  and off-canvas coordinates.
- Committed reference PNGs produced by the **konva** path — it is the one users see, so it is the
  reference.
- A Go test renders each fixture and compares against the reference within a per-pixel tolerance.
- **Rule:** a new shape type lands in both renderers and the fixture set in the same change. Add
  this to the checklist in the annotator README.

The Go schema mirror lives in `server/internal/model/image_annotation.go` and must track
`core/annotationTypes.ts`. The `version` field is the guard — reject anything above the known
version rather than rendering it wrong.

## The hard part: coordinate grounding

This is the risk to the whole idea, not the rendering. Vision models are unreliable at emitting
exact pixel coordinates on a 2560×1440 screenshot; naive prompting produces arrows pointing at
nothing.

1. **Normalized coordinates (v1).** The tool accepts fractional `0..1` values and multiplies by
   the source image's natural size server-side. Models handle proportions meaningfully better than
   pixels, and the stored schema stays in pixel space so nothing downstream changes.
   `coordinate_space` is an explicit enum — `normalized` (default) or `pixel` — never inferred
   from value ranges.
2. **Semantic targets (v2, the real fix).** When the image came from an agent-browser screenshot,
   the browser already knows element bounding boxes. Accept
   `target: { kind: "element", ref: "<snapshot-ref>" }` and resolve it to a box server-side, so the
   model names *what* to point at instead of guessing *where*.
3. **Render-and-check (fallback).** Feed the render back to the vision model for correction. Works,
   costs a round trip per annotation.

Ship (1). Treat accuracy as an evaluation problem with a fixture set of real screenshots and
expected target regions — not as done when the endpoint returns 200.

## Tool surface

An MCP tool, registered in `specialMCPToolDefinitions()`
(`server/internal/service/mcp_catalog.go`) and dispatched from `executeSpecialMCPTool`
(`server/internal/service/mcp_tools.go`). That path already supplies the workspace principal,
argument validation, and `auditToolSuccess` audit logging.

```go
{
  Name: "annotate_image", Title: "Annotate image",
  Description: "Draw arrows, boxes, callouts, and text on an existing image and return a new attachment.",
  InputSchema: withMCPIdempotencyKey(object(...)),
  Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite,
  Permission: authorization.PermDocsEdit, Module: model.ModuleDocs,
  Mutating: true, IdempotentHint: true,
}
```

**Input:** exactly one of `source_attachment_id` or `source_artifact_id`; `shapes[]`;
`coordinate_space`; optional `alt`.

**Output:** `{ attachment_id, url, width, height }`. Produced through
`PMAttachmentService.CreateImported`, which already uploads server-side bytes and confirms the
record in one call — no presign round trip needed.

Bound the input: at most ~50 shapes, text capped at a few hundred characters, source image capped
at the existing 50 MB attachment limit.

## Security

**An agent must not be able to use `cover` as a security control.** Redaction in the UI deletes the
original; a tool call has no such guarantee and an agent cannot judge what is sensitive. Reject
`cover` shapes from tool input with a clear error telling the agent to ask a human. Revisit only
with an explicit human-approval step.

Everything else follows the existing MCP posture: workspace-scoped principal, `pm.edit`-equivalent
permission, audit trail per call.

## Phases

### Phase 1 — Schema mirror and Go renderer
- `server/internal/model/image_annotation.go` — Go mirror of `AnnotationState`, with a version
  check and per-shape validation that mirrors `parseAnnotationState` (unknown types dropped, never
  fatal).
- `server/internal/service/image_annotation_render.go` — `gg`-based renderer, embedded font.
- Golden fixtures + comparison test. This phase is independently valuable and independently
  testable — no tool wiring yet.

### Phase 2 — Coordinate normalization
- `coordinate_space` handling, applied before rendering, against the source image's natural size.
- Table-driven tests at several aspect ratios, including non-square and very wide screenshots.

### Phase 3 — Tool wiring
- Definition in `specialMCPToolDefinitions()`, dispatch in `executeSpecialMCPTool`.
- Source resolution: attachment via `PMAttachmentService`, artifact via the agent-runtime artifact
  path.
- Output attachment via `CreateImported`; return the content URL.
- Reject `cover`; enforce bounds; audit.

### Phase 4 — Evaluation
- Fixture set of real screenshots with expected target regions.
- Scored runs measuring how often the emitted geometry lands inside the intended region.
- This is what decides whether v2 semantic targets are needed immediately or can wait.

### Phase 5 — Prompting and docs
- Tool description carrying the normalized-coordinate contract explicitly, with a worked example.
- A note in `docs/AGENTS_AND_AUTOMATION.md` describing the tool as a generic executor capability.

## Risks

| Risk | Mitigation |
|---|---|
| Agent annotations land in the wrong place | Normalized coordinates; semantic targets as the real fix; measured in Phase 4 rather than assumed |
| Go and konva renderers drift | Golden fixtures in CI; new shapes land in both renderers in one change |
| Blank text in production containers | Embed the font with `go:embed`; assert non-blank text in a golden fixture |
| Agent-driven redaction gives false assurance | `cover` rejected from tool input |
| Schema mirror rots | `version` guard, and the fixture set fails loudly on divergence |

## Open questions

1. Should `annotate_image` sit in the Docs toolset, or does an agent annotating a support
   screenshot need a support-scoped variant?
2. Is Phase 4 a gate for shipping, or do we ship behind a flag and evaluate with real traffic?
3. Do we expose the tool to custom agents immediately, or restrict to system agents until the
   grounding numbers are in?

## Estimate

Phase 1 is the bulk — a renderer plus fixtures. Phases 2 and 3 are small once the schema mirror
exists. Phase 4 is open-ended by nature and is where the real product risk lives. No frontend
changes and no migration.
