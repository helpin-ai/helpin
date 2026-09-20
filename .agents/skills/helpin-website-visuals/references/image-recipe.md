# Helpin product image recipe

Use this when a raster marketing asset is appropriate. Prefer the shared HTML previews for interactive platform workflows described in [palette and scene references](palette-and-scenes.md). For raster product views, preserve the reference app’s light or dark theme; the approved Projects Kanban uses charcoal and restrained sage, not a forest-green wash. Straight-on, edge-to-edge framing is the usual default. Keep browser chrome, perspective, floating panels, and marketing headlines outside the image unless the requested asset specifically needs them (the existing angled CLI artwork is one such exception).

Start from the nearest actual or approved demo screen. For an edit, enumerate the layout invariants and change only the requested subject or details. A style reference is not an instruction to reproduce every incidental label.

## Prompt scaffold

```text
Use case: ui-mockup (or precise-object-edit for an edit).
Asset: <page, placement, target aspect ratio>.
Inputs: Image 1 is <role>; Image 2 is <role>.
Task: Show <one customer job and its visible result>.
Preserve: <application columns, controls, navigation, typography, framing>.
Identity: OrbitDesk workspace and green O mark; Maya Chen at Northstar Labs;
Sam Rivera; Helpin AI. No Helpin Studio or customer company named OrbitDesk.
Scene: <short, coherent conversation and status>.
Exact text: <only the key labels and short messages that matter>.
Appearance: <approved light or dark UI palette from the reference>, hairline
divisions, readable application typography, sparse semantic color. Preserve
the app’s identity; do not invent a new surface palette.
Constraints: no invented controls, metrics, testimonials, third-party branding,
watermarks, or claims. Keep proposed work visibly distinct from completed work.
```

Request a 16:9 master, preferably 3840×2160 when supported, but inspect and record the actual dimensions. Built-in output dimensions are not guaranteed by a prompt. Keep native resolution; do not upscale and call it a 4K generation.

Keep prompts specific enough for identity and layout consistency but short enough that essential text stays reliable. Generated typography must be inspected visually; correct misleading labels with a targeted image edit. Do not use CSS to conceal incorrect text behind a new claim.

## Output record

For each accepted image, keep the final prompt and a small provenance record:

- Intended page and scene.
- Reference paths and their roles.
- Tool used; model only when confirmed.
- Native dimensions and master filename.
- Published derivatives and their widths.
- Whether it is a generated illustration or an authentic application capture.

Optimize locally with the existing Sharp dependency; encoding and proportional resizing do not require another image-generation call. Content alterations should go back through the generation/editing tool.
