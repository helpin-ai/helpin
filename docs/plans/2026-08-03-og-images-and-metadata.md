# OG Images and Metadata Implementation Plan

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
