# Helpin

Monorepo for the Helpin app, embedded widget SDK, shared widget components, and the standalone widget bundle.

## Main Packages

- `frontend/`: main React app
- `packages/shared/`: shared types and shared emoji catalog/search helpers
- `packages/widget-core/`: reusable chat widget UI components
- `packages/sdk-js/`: embeddable `lib.js` SDK loader and widget runtime
- `widget/`: standalone non-SDK widget bundle
- `server/`: Go API and workers

## Widget Paths

There are three different UI paths in this repo. They share some data, but they do not build the same way.

### 1. Embedded Widget via `lib.js`

This is the path used when a site loads:

```html
<script src="https://.../lib.js"></script>
```

Flow:

1. `packages/sdk-js/src/loader.ts` builds to stable `dist/lib.js`.
2. `packages/sdk-js/vite.config.ts` injects the hashed SDK filename into `lib.js`.
3. `lib.js` loads `helpin.[hash].js`.
4. `packages/sdk-js/src/core/widget.ts` mounts the widget from `@helpin-ai/widget-core`.
5. `sdk-js` aliases `@helpin-ai/widget-core` to `packages/widget-core/src/*`, so the SDK build bundles widget-core source directly.

Important consequence:

- If `widget-core` changes, rebuild `@helpin-ai/sdk-js` for the embed widget to pick it up.
- Building `widget-core` alone does not update the live `lib.js` widget.

### 2. Frontend App

The frontend uses emoji-related code in two different places:

- Support composer: `frontend/src/components/support/EmojiPicker.tsx`
  - Uses `loadEmojiCatalog()` from `@helpin-ai/widget-core`
- Widget preview in settings: `frontend/src/components/settings/WidgetPreview.tsx`
  - Uses `@helpin-ai/widget-core`
  - Frontend aliases `@helpin-ai/widget-core` to `packages/widget-core/dist/index.js`

Important consequence:

- The app support composer and widget-core now share the same async emoji catalog loader.
- The app support composer is not literally the same component as the widget-core picker.

### 3. Standalone Widget Bundle

This is the plain JS bundle under `widget/`:

- Source: `widget/src/helpin-widget.js`
- Emoji payload: `widget/src/emoji-data.js`
- Build script: `widget/build.sh`
- Outputs:
  - `widget/dist/helpin-widget.min.js`
  - `widget/dist/emoji-data.js`

This path is independent from `packages/sdk-js` and `packages/widget-core`.

Use the term "standalone widget bundle" for this path to avoid confusion with the `lib.js` SDK widget.

## Emoji Flow

### Shared Catalog

Shared emoji data lives in:

- `packages/shared/src/emoji-data.ts`
- re-exported from `packages/shared/src/index.ts`

This is the source of truth for:

- frontend support composer, via widget-core's loader
- widget-core lazy emoji catalog

The frontend app reaches this catalog through `@helpin-ai/widget-core`'s exported `loadEmojiCatalog()` helper, rather than importing the dataset eagerly.

### Embedded Widget Emoji Load

Flow:

1. User clicks the emoji button in widget-core.
2. `packages/widget-core/src/components/EmojiPicker.tsx` calls `emoji-loader.ts`.
3. `packages/widget-core/src/components/emoji-loader.ts` dynamically imports `emoji-catalog.ts`.
4. `packages/widget-core/src/components/emoji-catalog.ts` reads from `@helpin-ai/shared`.
5. The SDK build emits a lazy chunk like `dist/chunks/emoji-catalog.[hash].js`.
6. Browser fetches that chunk only when the picker is opened.

Serving requirements for the embedded widget:

- `lib.js`
- `helpin.[hash].js`
- `chunks/emoji-catalog.[hash].js`
- `sounds/ping.mp3`

If `/chunks/` is not served as JavaScript, the picker will fail at runtime.

### Standalone Widget Emoji Load

Flow:

1. User clicks the emoji button.
2. `widget/src/helpin-widget.js` resolves the widget asset base URL.
3. It lazy-loads `emoji-data.js` from the same base path.

This is separate from the `sdk-js` lazy chunk approach.

## Build and Deploy

Useful commands:

```bash
pnpm --filter @helpin-ai/shared build
pnpm --filter @helpin-ai/widget-core build
pnpm --filter @helpin-ai/sdk-js build
pnpm --filter frontend build
bash widget/build.sh
```

Deploy notes:

- `packages/sdk-js` CDN deploy must include `dist/chunks/`, not just `lib.js` and `helpin.*.js`
- nginx/static hosting for the embed widget must serve `/chunks/`
- frontend preview depends on `packages/widget-core/dist/index.js`

## Frontend Bundle Size Notes

Current frontend behavior:

- The emoji catalog is emitted as one async `emoji-catalog-*.js` chunk.
- The support composer and widget preview both load that same async catalog path through widget-core.
- The support route no longer carries a second copy of the catalog.

Important clarification:

- The large `index-*.js` chunk warning in the frontend build is not caused by the emoji catalog.
- The emoji-related payload is now isolated to the separate `emoji-catalog-*.js` chunk plus the picker UI code.

The shared dataset was also cleaned up:

- per-category duplicate entries were removed
- malformed entries were removed
- total catalog entries dropped from 1534 to 1409 without changing category coverage across the file

## Quick Rules of Thumb

- Change `packages/widget-core` and want the embed widget updated: rebuild `@helpin-ai/sdk-js`
- Change the standalone widget under `widget/`: run `bash widget/build.sh`
- Change only shared emoji data: rebuild anything that consumes it
- Debug a broken embed-widget emoji picker first by checking `/chunks/emoji-catalog.*.js` delivery and MIME type

## Agents and Automation

The canonical doc for the current backend model and near-term proposal is [docs/AGENTS_AND_AUTOMATION.md](docs/AGENTS_AND_AUTOMATION.md).

Workspace-owned outbound MCP servers for agents are documented in [docs/EXTERNAL_MCP_SERVERS.md](docs/EXTERNAL_MCP_SERVERS.md). Public inbound MCP for outside AI clients remains documented separately in [docs/HELPIN_PUBLIC_MCP.md](docs/HELPIN_PUBLIC_MCP.md).

Current truth:

- `agent_run` is the durable execution primitive
- automation rules are the user-authored trigger-to-action layer
- built-in automations remain product-owned backend behavior
- run input now carries explicit `trigger` / `target` / `event` metadata while preserving legacy fields
- generic target launching now exists for direct runs and automation-rule `start_agent_run`
- the backend already behaves as two agent categories:
  - system agents as product-owned preset/default wrappers
  - custom agents as user-defined wrappers
- for `native_sdk`, both categories share the same core run machinery

Current trigger surfaces:

- manual run actions
- agent `trigger_mode`
- agent `schedule`
- automation-rule triggers: `story.state_entered`, `agent_run.approved`, `cron`

Proposed custom-agent direction:

- keep system agents product-owned defaults
- make custom agents generic `native_sdk` executors
- trigger custom agents via `manual`, automation-rule `event`, and automation-rule `cron`
- pass a minimal trigger payload into `agent_run.input`
- let custom agents gather additional context with tools
