# Help Center Canonical URL Mode Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the public help center use non-locale canonical URLs when multilingual is disabled and locale-prefixed canonical URLs when multilingual is enabled, with one-way redirects only from non-canonical to canonical paths.

**Architecture:** Centralize URL-mode decisions in shared help-center routing helpers, then reuse shared collection/article page views from both locale and non-locale route files. Route shells become responsible only for canonical-mode redirects, while rendering logic stays shared and stable.

**Tech Stack:** React 19, TanStack Router, Vitest, TypeScript 5.9

---

### Task 1: Lock Canonical URL Rules In Tests

**Files:**
- Modify: `help-center/src/lib/__tests__/locale.test.ts`

- [ ] Add failing tests for multilingual-mode detection and canonical home/collection/article/search path building.
- [ ] Add failing tests for locale-switch fallback behavior in both multilingual and non-multilingual modes.
- [ ] Run: `cd help-center && npm exec vitest run src/lib/__tests__/locale.test.ts`

### Task 2: Centralize Canonical Path Logic

**Files:**
- Modify: `help-center/src/lib/locale.ts`

- [ ] Add a single `isMultilingualEnabled` helper based on enabled locales.
- [ ] Add canonical path builders that switch between locale and non-locale URL formats.
- [ ] Keep locale fallback behavior path-based, not redirect-chain based.
- [ ] Run: `cd help-center && npm exec vitest run src/lib/__tests__/locale.test.ts`

### Task 3: Share Collection/Article Rendering Across Modes

**Files:**
- Create: `help-center/src/components/routes/CollectionRouteView.tsx`
- Create: `help-center/src/components/routes/ArticleRouteView.tsx`
- Modify: `help-center/src/routes/$locale/$spaceSlug/index.tsx`
- Modify: `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/index.tsx`
- Modify: `help-center/src/routes/$spaceSlug.tsx`
- Modify: `help-center/src/routes/$spaceSlug/$articleSlug.tsx`

- [ ] Extract shared collection page rendering that works with either locale-prefixed or non-locale mode.
- [ ] Extract shared article page rendering that works with either locale-prefixed or non-locale mode.
- [ ] Make localized routes serve only when multilingual is enabled; otherwise redirect once to non-locale canonical paths.
- [ ] Make non-locale routes serve only when multilingual is disabled; otherwise redirect once to default-locale canonical paths.
- [ ] Preserve legacy space-tab behavior by redirecting space slugs to the first collection inside the current canonical mode.

### Task 4: Update Navigation And Link Surfaces

**Files:**
- Modify: `help-center/src/components/layout/TopBar.tsx`
- Modify: `help-center/src/components/home/LocalizedHomePage.tsx`
- Modify: `help-center/src/components/navigation/NavTree.tsx`
- Modify: `help-center/src/components/navigation/Breadcrumbs.tsx`
- Modify: `help-center/src/components/navigation/ArticlePager.tsx`
- Modify: `help-center/src/components/search/SearchResultItem.tsx`
- Modify: `help-center/src/routes/index.tsx`
- Modify: `help-center/src/routes/$locale/index.tsx`
- Modify: `help-center/src/routes/search.tsx`
- Modify: `help-center/src/routes/$locale/search.tsx`

- [ ] Make home, search, collection, article, and space-tab links use canonical URLs for the active mode.
- [ ] Keep locale switcher active only when multilingual is enabled.
- [ ] Ensure no route/link layer tries to canonicalize both ways at the same time.

### Task 5: Verify End-To-End Behavior

**Files:**
- Modify: `help-center/src/components/layout/__tests__/LocaleSwitcher.test.tsx`
- Modify: `help-center/src/components/search/__tests__/SearchResultItem.test.tsx`

- [ ] Update focused tests for the new canonical path shapes.
- [ ] Run: `cd help-center && npm exec vitest run src/lib/__tests__/locale.test.ts src/components/layout/__tests__/LocaleSwitcher.test.tsx src/components/search/__tests__/SearchResultItem.test.tsx`
- [ ] Run: `cd help-center && npm run build`
