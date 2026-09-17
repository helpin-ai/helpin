# Widget architecture and builds

Build paths, shared emoji loading, and deployment requirements for the embedded
SDK, app preview, and standalone widget. Paths below are relative to the repository root.

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

## Check bundle output

The source shares `loadEmojiCatalog()` between the support composer and widget
preview, and that helper dynamically imports the catalog. Actual chunk filenames,
chunk sizes, and deduplication depend on the build; inspect emitted assets after
changing dependencies or bundler settings. Earlier entry counts and bundle-warning
observations were measurements from a prior build, not maintained limits.

The npm SDK entry is built separately by
[build-esm.mjs](../packages/sdk-js/scripts/build-esm.mjs). Browser widget runtime
loading and `widgetRuntimeUrl` are documented in the [SDK README](../packages/sdk-js/README.md).
Do not assume rebuilding a consumer application republishes hosted SDK assets.

## Quick Rules of Thumb

- Change `packages/widget-core` and want the embed widget updated: rebuild `@helpin-ai/sdk-js`
- Change the standalone widget under `widget/`: run `bash widget/build.sh`
- Change only shared emoji data: rebuild anything that consumes it
- Debug a broken embed-widget emoji picker first by checking `/chunks/emoji-catalog.*.js` delivery and MIME type
