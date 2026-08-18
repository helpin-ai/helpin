# Help Center Icons MVP Plan

- **Status:** Implemented on the working branch; pending rollout
- **Date:** 2026-07-16
- **Scope:** Correct icon rendering in the public Help Center, valid icon discovery for public MCP tools, and lazy editor icon loading
- **Supersedes:** `2026-07-16-help-center-icon-delivery-plan.md`

## 1. Decision

Solve the customer-facing bug with generated static SVG files. Do not build workspace sprites, Temporal workflows, active-sprite records, S3 proxy infrastructure, or sprite lifecycle management.

The implementation is split into three focused changes:

1. **Public rendering:** generate one static SVG per Hugeicon and render only the files used by a Help Center page.
2. **MCP discovery and validation:** let agents search valid icon IDs and prevent invented icon names from entering storage.
3. **Editor loading:** remove the existing module-scope catalog load and fetch a static search manifest only on picker interaction.

The first two changes fix the customer and agent-facing correctness problem. The third keeps ordinary authoring routes from paying the full catalog cost.

## 2. Current state

### Public Help Center

- The installed Hugeicons Free package contains 5,099 individual modules and exposes 5,784 unique `*Icon` picker IDs, including aliases.
- The public Help Center uses a small handwritten Lucide map in `help-center/src/lib/icons.ts`.
- `help-center/src/components/PhIcon.tsx` displays an unresolved icon value as raw text.
- As a result, many icons selected by authors appear as names such as `activity01` in the customer-facing Help Center.

### Editor

- `frontend/src/components/ui/icon-picker.tsx` calls `loadAllIcons()` at module scope.
- Importing `StoredIcon`, `ICON_MAP`, or `IconPicker` starts loading the complete icon catalog even when the picker is never opened.
- This affects docs, teams, and support-inbox surfaces that consume the shared module.

### Public MCP

The MCP does not currently know which icon IDs are valid.

- `create_space`, `create_collection`, `update_space`, and `update_collection` define `icon` only as `{ "type": "string", "maxLength": 100 }`.
- The tool descriptions do not name the library or explain the canonical ID format.
- There is no `list_icons` or `search_icons` tool.
- The service accepts arbitrary icon strings without catalog validation.

An MCP agent must currently guess an icon name. A guessed value can be stored successfully and later appear as raw text in the Help Center.

## 3. Goals

- Every Hugeicon offered by the editor has a corresponding public static asset.
- The public Help Center ships no Hugeicons JavaScript, loader, or complete catalog manifest.
- Visitors download only icons used on the current page.
- Static icon files receive existing one-year immutable cache headers.
- Icons support `currentColor`, themes, tinting, SSR, and fixed dimensions.
- Existing emoji/Unicode values continue to render.
- Existing values that currently resolve to a specific icon retain a specific mapping.
- Unknown identifier-shaped values never appear as raw text.
- MCP agents can discover valid icon IDs before creating or updating spaces and collections.
- Invalid MCP, REST, config, and internal-service icon writes are rejected consistently.

## 4. Non-goals

- Per-workspace or per-space SVG sprites.
- Temporal workflows for icon generation.
- Active-sprite database fields or cache invalidation.
- S3-backed per-workspace icon compilation.
- A new icon upload system.
- Replacing Hugeicons.
- Reworking the independent fixed-preset `AgentIconPicker`.

## 5. Generated static catalog

Add one deterministic generator using the pinned `@hugeicons/core-free-icons` package. It produces:

- Optimized individual SVG files for the Help Center build.
- A searchable static editor manifest plus a compact in-memory MCP catalog containing canonical IDs and labels.
- A small legacy alias map.
- A compact server catalog used for public-response resolution, validation, and MCP search.
- A catalog hash/version used by consistency tests.

The generator preserves the existing storage-key conversion verbatim:

- `Rocket01Icon` → `rocket01`
- `FolderOpenIcon` → `folder-open`
- `AArrowDownIcon` → `aarrow-down`
- `ThreeDViewIcon` → `three-dview`

Changing those IDs would require a separate migration. This project must remain compatible with existing stored values.

The generator has two delivery modes:

- The Help Center build generates SVGs into its local `public/assets/helpin-icons/...` tree before Vite builds. Generated SVG files do not need to be committed.
- The same generator produces a compact catalog/alias artifact for the Go server. Commit or embed this small artifact so the Go build does not require Node. CI runs the generator in check mode and fails if the server catalog, package version, or canonical IDs drift from the Help Center assets.

Add `generate:icons`, `predev`, and `prebuild` scripts to both applications. Local development and production builds generate or verify their same-origin static trees before Vite starts. Both generated asset trees are gitignored so the 5,784 SVGs per application are not committed. The generator skips rewriting files when its catalog-version/hash marker is current.

Generated Help Center files live under the existing static path:

```text
/assets/helpin-icons/hugeicons/4.1.1/rocket01.svg
```

The Help Center build places them in `dist/client/assets/helpin-icons/...`. `serve.mjs` already serves `/assets/*` with:

```text
Cache-Control: public, max-age=31536000, immutable
```

This increases the container/build-artifact size, not the JavaScript bundle or per-visitor transfer. A browser fetches only the icon files referenced by the rendered page and then reuses them from cache.

The Hugeicons Free package is MIT-licensed. Retain its license notice with the generated distribution.

The Go server and Help Center deploy as separate images. For an icon-library upgrade, deploy and verify the Help Center image containing the new static catalog before, or atomically with, the server image that can emit new IDs. CI must also verify that the new asset set retains every canonical ID still emitted by the previously deployed server. This keeps the rolling-deploy window safe.

## 6. Public Help Center renderer

Resolve icon values in the Go public Help Center projection before they reach the React application. The server catalog converts each stored value into one of three outcomes:

- A canonical Hugeicons asset ID.
- Preserved Unicode/display text.
- No resolved icon, which instructs the client to use the entity fallback.

Apply this normalization to every icon field in public Help Center DTOs: spaces, collections/navigation, featured cards, preview responses, and document/article payloads. Document/article icons are not currently rendered on published routes, but normalizing their payload-only fields prevents a future UI from accidentally exposing a raw stored identifier. This prevents the complete catalog and legacy map from entering the Help Center client bundle. It also makes an invalid icon written between PR 1 and PR 2 harmless: the public response returns the fallback instead of the raw identifier.

Keep the existing nullable JSON string fields; do not introduce a new `{ type, value }` wire object. The normalized public wire convention is:

- Identifier-shaped non-empty string → canonical static asset ID.
- Any other non-empty string → preserved display text.
- Null or empty string → use the caller's entity fallback.

Replace `PhIcon` with a small asset-backed `PublicIcon` component that consumes those resolved values.

Resolution order:

1. The server trims the stored value and resolves canonical and legacy values case-insensitively.
2. The server preserves compatible Unicode/text or returns an unresolved outcome; the backfill inventory reports unresolved counts for rollout review.
3. `PublicIcon` renders a canonical asset through a CSS mask using `background-color: currentColor`.
4. `PublicIcon` renders preserved Unicode/text as text.
5. `PublicIcon` renders the entity's neutral folder/file fallback for an empty or unresolved outcome.

Case-insensitive catalog resolution occurs before text classification, so a stored value such as `Rocket` continues to resolve to an icon.

The renderer uses explicit width and height to prevent layout shift. Generated paths must preserve the existing base-path handling so reverse-proxied Help Centers resolve `/assets/` correctly.

Update all current Help Center call sites:

- Top-bar space icons.
- Home-page featured cards.
- Top-level collection navigation.
- Authenticated document preview.

Remove the raw-name fallback. Remove the ignored `weight` prop from the replacement component and its call sites. Once compatibility is verified, remove the unused `@phosphor-icons/react` dependency.

## 7. Compatibility inventory

Before switching the renderer, query every distinct icon value stored in:

- Spaces.
- Collections.
- Documents.
- `homepage_config.featured_cards[].icon`.

Record how the current `getIconComponent` implementation resolves each value, including direct matches, normalized aliases, and token heuristics.

Acceptance rule:

> Every observed value that currently resolves to a specific icon must map to a specific approved SVG after migration. A neutral fallback is allowed only for a currently unresolved value or an explicitly approved product change.

Existing Unicode/text values remain readable even if new writes would no longer accept the same ASCII text.

The inventory must produce an explicit, idempotent cleanup/backfill:

- Normalize canonical IDs, supported aliases, and case variants to the canonical stored ID.
- Map currently heuristic-resolved values to a specific approved canonical ID.
- Clear or explicitly remap unresolved identifier/ASCII junk values according to the entity fallback policy.
- Preserve approved Unicode display values.

Track remaining invalid persisted values. The update no-op tolerance in section 9 may be removed only after the backfill has run and a clean dry-run confirms that no invalid values remain in the affected fields.

## 8. MCP icon discovery

Add a public MCP read tool:

```text
search_icons
```

Suggested input:

```json
{
  "query": "rocket",
  "limit": 20
}
```

Suggested result:

```json
{
  "items": [
    { "id": "rocket", "label": "Rocket" },
    { "id": "rocket01", "label": "Rocket 01" }
  ],
  "total": 2,
  "catalog_version": "4.1.1"
}
```

Behavior:

- Use the Docs read toolset/scope and existing Docs read permission.
- Search the generated in-memory catalog by ID and label.
- Return a bounded result set; do not expose all 5,784 picker IDs in every response.
- With an empty query, optionally return a small curated set of common icons.
- Never return SVG path data or markup through MCP.

Update the `icon` schema descriptions for `create_space`, `create_collection`, `update_space`, and `update_collection`:

> Canonical Helpin icon ID. Call `search_icons` to find a valid value, or omit this field to use the default icon.

The expected agent flow becomes:

```text
search_icons("knowledge") → choose "knowledge01" → create_collection(icon="knowledge01")
```

## 9. Write validation

Use the generated server catalog as the shared validator for REST, internal services, Help Center config writes, and MCP tools.

Update validation must not reject an unchanged persisted value. Several existing forms echo the current icon even when the user edits only another field. For update operations, compare trimmed submitted and persisted values first; if they are equal, allow the no-op even when the historical value is invalid. Creates and changed icon values always use the strict rules below. Apply the same per-field comparison to featured cards in raw Help Center config.

For new writes:

1. Trim the value.
2. Treat null/empty as no custom icon.
3. Resolve canonical IDs and supported legacy aliases case-insensitively and store the canonical ID.
4. Otherwise allow only bounded display text that:
   - Contains at least one non-ASCII code point.
   - Contains no control characters or line breaks.
   - Contains at most 16 Unicode code points and 64 UTF-8 bytes.
5. Reject unknown identifier-shaped values and ASCII names such as `Rocket01Icon`, `rocket icon`, and `FAQ`.

Apply validation to:

- Space create/update.
- Collection create/update.
- Document create/update.
- `homepage_config.featured_cards[].icon`, including non-collection cards.
- Public MCP tools that accept `icon`.

Invalid MCP responses should explain that the caller must use `search_icons` or omit the optional icon. Do not silently store an invented value.

## 10. Editor loading

- Remove the module-scope `loadAllIcons()` call and mutable `ICON_MAP`.
- Give the editor its own generated same-origin static SVG copy.
- Fetch the 261 KB static search manifest only on picker hover, focus, or open intent and cache it for the browser session.
- Keep picker rendering bounded to 60 matched results.
- Compute selected canonical icon URLs without loading the search manifest.
- Bundle only the small generated legacy alias map needed by the eager stored-icon renderer.
- Migrate Docs, teams, settings, and support inbox consumers to the asset-backed `StoredIcon`.

## 11. Delivery sequence

### Rollout notes

- Public Help Center DTOs are cached for several minutes. PR 1 must bump the affected Help Center cache-key namespace/version (or perform an equivalent complete invalidation) so newly deployed server pods do not reuse cached raw icon values.
- During a mixed-version rolling deployment, a new `PublicIcon` may briefly receive an old raw identifier from an old server/cache entry. Its benign failure mode is an invisible fixed-size icon, not raw text or layout shift. Finish the coordinated rollout and verify representative Help Centers after old pods drain.
- Release notes must mention that legacy values previously approximated with Lucide heuristics will now display the corresponding exact Hugeicons glyph. This is an intentional visible change.

### PR 1 — Customer-facing rendering

- Add the generator and pinned development dependency.
- Add gitignored generated output plus `predev`/`prebuild` hooks so static SVGs exist in local development and production builds.
- Generate static SVG files into the Help Center `/assets/` build output.
- Generate/embed the compact Go catalog and add CI drift checking.
- Add the server-side canonical/legacy public-response resolver to every public icon DTO field and retain the nullable string wire contract.
- Bump the public Help Center cache-key namespace/version for affected DTOs.
- Replace `PhIcon` with `PublicIcon`.
- Preserve Unicode/text and add neutral identifier fallback.
- Add bundle, SSR, base-path, and browser tests.

This PR fixes the reported customer issue.

### PR 2 — MCP discovery and write safety

- Reuse the generated server catalog from PR 1.
- Add `search_icons` to the public MCP catalog and dispatcher.
- Document `icon` fields in space/collection MCP schemas.
- Add shared write validation, including featured-card config.
- Allow unchanged invalid persisted values on updates, then run the inventory-driven cleanup/backfill.
- Add MCP and service tests.

This PR ensures agents know which icon IDs they can use and prevents recurrence.

### PR 3 — Editor performance

- Remove the module-scope full-catalog load.
- Lazy-load and virtualize the picker.
- Migrate every shared `ICON_MAP`, `StoredIcon`, and `IconPicker` consumer.
- Verify ordinary editor routes no longer request the catalog.

The inventory and cleanup implementation is the dry-run-first `go run ./cmd/backfill-helpcenter-icons` command. Canonical normalization is idempotent; clearing unresolved or reviewed ASCII display values requires explicit flags.

## 12. Tests and acceptance criteria

### Catalog and rendering

- Every picker icon has exactly one generated SVG.
- Generated IDs are unique and deterministic.
- All generated SVGs are sanitized and use the expected view box/stroke behavior.
- Every currently resolved stored value retains a specific approved mapping.
- Canonical, case-variant, legacy, Unicode, empty, and unknown values render correctly.
- Public wire tests assert canonical ID strings, preserved non-identifier text, and null/empty fallback without introducing a typed icon object.
- Public document/article payload-only icon fields are normalized even though published routes do not currently render them.
- Cache-version tests prove PR 1 cannot reuse pre-normalization public DTO entries.
- No unresolved identifier is displayed as text.
- Icons render in top bar, home cards, navigation, and preview.
- SSR and hydrated output match.
- Icon-related CLS is zero.

### Performance

- `@hugeicons/core-free-icons` and its loader do not enter a Help Center client chunk.
- The complete catalog manifest does not enter the initial Help Center client chunk.
- Catalog-related Help Center JavaScript increase is 0 KB target and no more than 1 KB gzipped glue code.
- Static SVGs return one-year immutable cache headers.
- Local `pnpm dev` generates/verifies the same icon assets as production `pnpm build`.
- Upgrade checks prove the new Help Center asset set covers every canonical ID emitted by the previously deployable server catalog.
- Public p75 LCP does not regress meaningfully; investigate an increase over 50 ms.

### MCP and validation

- `search_icons` returns only valid canonical IDs and respects its result limit.
- `create_collection` succeeds with an ID returned by `search_icons`.
- Invalid IDs return a structured error recommending `search_icons`.
- Case variants and legacy aliases are stored canonically.
- ASCII raw names are rejected; bounded non-ASCII display values are accepted.
- Featured-card icons are validated for every link type.
- Renaming or editing an entity with an unchanged invalid persisted icon succeeds; changing that icon to another invalid value fails.
- Raw featured-card config updates receive the same unchanged-value tolerance on a per-card field.
- The cleanup/backfill is idempotent and leaves no unapproved invalid values before the tolerance is removed.

## 13. Definition of done

- A customer can choose any Hugeicon and see it correctly in the public Help Center.
- Public pages contain no Hugeicons runtime catalog or raw unresolved icon names.
- Existing customer icon/emoji values retain their prior intended rendering.
- Historical invalid icons cannot block unrelated edits during migration, and the cleanup/backfill closes that compatibility window.
- MCP agents can search for valid icon IDs before creating a space or collection.
- All icon write paths use the same validation rules.
- Static assets are immutable and cached through the existing Help Center server.
- Local development and production builds both generate the static asset tree, which remains uncommitted.
- The customer-facing and MCP changes can ship without sprites, Temporal, new database metadata, or new runtime infrastructure.

## 14. Deferred optimization

Do not implement sprites unless production measurements show that individual immutable SVG requests materially affect performance. If that occurs, create a separate proposal based on measured request count, transfer size, cache reuse, and LCP impact.
