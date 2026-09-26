# OG Images and Metadata Implementation Plan

> Historical design/implementation record (2026-08-03), source-compared on
> 2026-09-17. Marketing metadata, generated images, app-shell metadata and the
> Netlify share transformer are implemented. Original “current state”, checkboxes,
> test passes and image inspections below describe the earlier session.

## Current behavior and deployment limits

- [Marketing metadata](../../website/src/lib/metadata.ts) is used by root, pricing,
  privacy and terms layouts; the root supplies `metadataBase`. This four-page
  contract does not establish unique metadata for every future marketing route.
- [Image generation](../../website/scripts/generate-og-images.mjs) runs through
  the website prebuild script. The six committed PNG assets currently have
  1200×630 headers; this audit did not visually inspect or regenerate them.
- The [Vite shell](../../frontend/index.html) contains the marked generic noindex
  metadata block. [Share middleware](../../frontend/netlify/edge-functions/share-meta.ts)
  fetches the existing public document response, then uses only `document.title`.
  It does not request a title-only projection. The title is HTML-escaped and
  truncated to 80 Unicode code points including the ellipsis.
- The preview URL is origin plus pathname, without query parameters or fragment.
  The transformer recognizes `app.helpin.ai`, `app.stage.helpin.ai`, `localhost`,
  and `127.0.0.1`; other hosts keep generic metadata. The upstream deadline is two
  seconds, and failures preserve the original response. Successfully transformed
  HTML receives `private, no-store` and `X-Robots-Tag` headers.
- [Netlify configuration](../../frontend/netlify.toml) registers the edge-functions
  directory and the function declares `/share/*` with bypass-on-error. A generic
  static server or Community container does not execute this Netlify middleware
  merely because the source file exists. No deployed preview/cache behavior was
  checked during this review.


> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add complete, page-accurate social metadata and polished 1200×630 OG images to the Helpin website and app, including privacy-safe public-document share metadata.

**Architecture:** A deterministic Node generator produces committed PNG assets from a shared Helpin visual system. Next.js route layouts consume a typed metadata helper; the Vite shell contains a replaceable default metadata block; and a small Netlify Edge Function replaces that block for public document shares after fetching only the public title.

**Tech Stack:** Next.js 15 metadata API, React `ImageResponse`, Node test runner, Vite, Vitest, Netlify Edge Functions, TypeScript.

---

### Task 1: Marketing metadata contract

**Files:** `website/tests/seo-metadata.test.mjs`, `website/src/lib/metadata.ts`, route layouts, and the root layout.

- [x] Write and run a failing test for unique metadata, canonicals, Open Graph details, X cards, and no keywords.
- [x] Implement the typed helper and route layouts.
- [x] Re-run the metadata test until green.

### Task 2: Deterministic OG assets

**Files:** `website/scripts/generate-og-images.mjs`, `website/public/og/*`, `frontend/public/og/*`, and `website/package.json`.

- [x] Add and run failing image signature, dimension, and size tests.
- [x] Implement the generator and create all approved variants.
- [x] Inspect the generated variants and re-run tests until green.

### Task 3: App shell metadata

**Files:** `frontend/src/lib/__tests__/appMetadata.test.ts` and `frontend/index.html`.

- [x] Write and run a failing test for the complete default metadata block.
- [x] Add the marked metadata block and re-run until green.

### Task 4: Public shared-document edge metadata

**Files:** `frontend/netlify/edge-functions/share-meta.ts`, its focused test, and `frontend/netlify.toml`.

- [x] Write and run failing tests for environment selection, encoded tokens, escaping, exact URL, noindex, shared image, privacy, and fallbacks.
- [x] Implement the pure helpers and Netlify response middleware.
- [x] Configure `/share/*` with bypass-on-error and no edge caching; re-run until green.

### Task 5: Verification and review

- [x] Run website and frontend focused tests.
- [x] Run website and frontend production builds.
- [x] Inspect generated HTML and PNGs.
- [x] Run `git diff --check`, review the scoped diff, and commit the work.
