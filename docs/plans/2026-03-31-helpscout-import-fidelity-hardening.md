# Help Scout Import Fidelity Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Help Scout imports render as close to the original as possible in one rollout by fixing structural conversion gaps, source-noise cleanup, category mapping, image rewriting coverage, import diagnostics, and public article styling without regressing already-working docs.

**Architecture:** Keep the existing generic HTML-to-TipTap converter intact as the core engine, but add a Help Scout-specific preprocessing and mapping layer before conversion. Reuse the existing import job summary, stored `import_source_html`, and reconvert flow so the same hardened pipeline repairs already-imported content and all future imports. Keep renderer changes conservative and limited to public prose styling and article enhancement code so native docs keep the same visual language.

**Tech Stack:** Go 1.24, Chi/GORM services, TipTap JSON model, React 19, Vite 7, Vitest, existing Help Center public app CSS.

---

## Planned File Map

**Backend conversion and import pipeline**
- Create: `server/internal/docsimport/helpscout_preprocess.go`
- Create: `server/internal/docsimport/helpscout_preprocess_test.go`
- Create: `server/internal/docsimport/testdata/replug_alt_text.html`
- Create: `server/internal/docsimport/testdata/replug_first_comment.html`
- Create: `server/internal/docsimport/testdata/replug_bio_links.html`
- Modify: `server/internal/docsimport/helpscout_normalize.go`
- Modify: `server/internal/docsimport/html_to_tiptap.go`
- Modify: `server/internal/docsimport/html_to_tiptap_test.go`
- Modify: `server/internal/docsimport/warnings.go`
- Modify: `server/internal/helpscout/images.go`
- Modify: `server/internal/service/docs_import.go`
- Create: `server/internal/service/docs_import_test.go`
- Modify: `server/internal/model/docs_import.go`

**Frontend and public help-center presentation**
- Modify: `frontend/src/lib/services/docsImportService.ts`
- Modify: `frontend/src/components/settings/HelpCenterImportSection.tsx`
- Modify: `help-center/src/app.css`
- Modify: `help-center/src/components/ArticleContent.tsx`
- Create: `help-center/src/components/__tests__/ArticleContent.test.tsx`

**Optional if implementation needs a shared parsing helper instead of duplicating logic**
- Create: `server/internal/docsimport/helpscout_category_map.go`

**Out of scope**
- No generic docs editor schema redesign
- No broad HTML decoding for arbitrary imports
- No fuzzy category assignment as a first-choice mapping strategy
- No new import system parallel to the current job/reconvert flow

---

### Task 1: Freeze Current Failures With Real Help Scout Fixtures

**Files:**
- Create: `server/internal/docsimport/testdata/replug_alt_text.html`
- Create: `server/internal/docsimport/testdata/replug_first_comment.html`
- Create: `server/internal/docsimport/testdata/replug_bio_links.html`
- Modify: `server/internal/docsimport/html_to_tiptap_test.go`
- Create: `server/internal/service/docs_import_test.go`

- [ ] **Step 1: Add real-world fixture HTML from the problematic Replug import**

Create sanitized fixture files from stored `import_source_html` for:
- `How to add ALT Text to your Posts?`
- `How to Add First Comment`
- `Bio-Links, A wonderful feature to Discover!`

The fixtures must preserve the exact patterns that currently fail:
- escaped pseudo-HTML note blocks
- empty headings
- leading/trailing whitespace artifacts
- Help Scout callout wrappers
- dense image-driven instructional sections

- [ ] **Step 2: Write failing converter tests for the visible regressions**

Add tests that prove the current converter must:
- turn known escaped Help Scout note blocks into actual callout content
- drop empty headings instead of preserving them
- keep `h4` and `h5` headings as real headings
- avoid leading-space text pollution in paragraphs
- preserve already-working callout/image/list behavior from the existing tests

- [ ] **Step 3: Write failing import-service tests for category mapping and summary reporting**

Add service-level tests around `DocsImportService.importArticle(...)` or a factored helper that prove:
- Help Scout article categories map correctly by stable source identifiers
- articles that cannot be mapped end up in `Uncategorized` with a warning entry
- import summary carries quality counters needed for the UI

- [ ] **Step 4: Run only the new backend tests to confirm they fail for the right reasons**

Run:
```bash
cd server && go test ./internal/docsimport -run 'TestConvertHelpScout|TestHelpScoutPreprocess' -count=1 -v
cd server && go test ./internal/service -run 'TestDocsImportService_(ImportArticle|Reconvert)' -count=1 -v
```

Expected:
- conversion tests fail on literal escaped note blocks / empty headings / whitespace cleanup
- service tests fail on missing summary counters or weak category fallback behavior

---

### Task 2: Add A Help Scout-Specific Preprocessing Layer

**Files:**
- Create: `server/internal/docsimport/helpscout_preprocess.go`
- Create: `server/internal/docsimport/helpscout_preprocess_test.go`
- Modify: `server/internal/docsimport/helpscout_normalize.go`
- Modify: `server/internal/docsimport/warnings.go`
- Modify: `server/internal/docsimport/html_to_tiptap.go`

- [ ] **Step 1: Add an explicit `PreprocessHelpScoutHTML` entry point**

Implement a Help Scout-specific preprocessing function that returns:
- normalized HTML
- preprocessing warnings

It should be called only from the Help Scout import/reconvert path, not from every HTML conversion in the system.

- [ ] **Step 2: Normalize known Help Scout structural patterns safely**

Implement narrow, whitelist-based transforms for:
- escaped pseudo-HTML note/callout blocks such as `&lt;aside&gt;...&lt;/aside&gt;`
- known Help Scout callout wrappers/classes that should map to canonical callout HTML
- repeated spacer paragraphs and empty heading tags

Guardrails:
- do not decode arbitrary escaped HTML globally
- do not transform content inside `<pre>` or `<code>`
- do not rewrite literal instructional code snippets that only happen to contain `<aside>` text

- [ ] **Step 3: Add whitespace and junk cleanup rules**

Normalize:
- leading/trailing paragraph whitespace caused by import artifacts
- blank paragraphs that only contain `&nbsp;` or whitespace
- headings whose inline content is empty after trimming

Keep:
- meaningful inline spacing inside actual prose
- hard breaks and intentional list/table text content

- [ ] **Step 4: Merge preprocessing warnings into conversion warnings**

Add warning types such as:
- `helpscout_note_block_normalized`
- `empty_heading_removed`
- `blank_paragraph_removed`
- `uncategorized_article`

These warnings will feed import summaries later.

- [ ] **Step 5: Run the preprocessing and converter tests**

Run:
```bash
cd server && go test ./internal/docsimport -run 'TestHelpScoutPreprocess|TestConvertHelpScout' -count=1 -v
```

Expected: PASS, while all pre-existing `html_to_tiptap` tests still pass unchanged.

---

### Task 3: Harden The Generic HTML-To-TipTap Conversion Without Breaking Working Cases

**Files:**
- Modify: `server/internal/docsimport/html_to_tiptap.go`
- Modify: `server/internal/docsimport/html_to_tiptap_test.go`
- Modify: `server/internal/docsimport/warnings.go`

- [ ] **Step 1: Keep the generic converter source-agnostic, but tighten block extraction where safe**

Implement only generic improvements that are safe for all inputs:
- drop empty headings/paragraphs after inline conversion
- avoid emitting paragraph nodes that contain no meaningful content
- preserve working image/list/callout/table behavior

- [ ] **Step 2: Ensure container flattening does not erase valid native content**

Add tests and minimal code for:
- nested Help Scout sections/divs that contain headings, paragraphs, images, and callouts
- figures with captions
- mixed text/image paragraphs

Goal:
- native content should stay native
- fallback `htmlBlock` should only happen when there is no better structural mapping

- [ ] **Step 3: Keep fallback behavior explicit and measurable**

Retain `htmlBlock` fallback for unsupported content, but ensure warnings are emitted consistently so import diagnostics can count them.

- [ ] **Step 4: Run the full converter test suite**

Run:
```bash
cd server && go test ./internal/docsimport -count=1 -v
```

Expected: PASS with new Help Scout fixtures and no regressions in existing converter tests.

---

### Task 4: Fix Category Mapping Deterministically

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Create: `server/internal/service/docs_import_test.go`
- Modify: `server/internal/model/docs_import.go`

- [ ] **Step 1: Write failing tests around category identifier edge cases**

Cover:
- article category values arriving as Help Scout category IDs
- article category values arriving as slugs
- article category values arriving as normalized names
- unmapped category values landing in `Uncategorized`

- [ ] **Step 2: Replace single-key category mapping with deterministic layered matching**

Build a mapping helper that resolves category-to-collection using this order:
1. exact Help Scout category ID
2. exact Help Scout category slug
3. normalized exact Help Scout category name
4. fallback to `Uncategorized`

Do not add fuzzy matching beyond normalized exact comparisons.

- [ ] **Step 3: Record mapping failures in job summary/warnings**

Track:
- count of articles sent to `Uncategorized`
- the unmatched source identifiers for diagnostics

This data should be stored in existing import job `summary`, not in a new subsystem.

- [ ] **Step 4: Run targeted import-service tests**

Run:
```bash
cd server && go test ./internal/service -run 'TestDocsImportService_(CategoryMapping|ImportArticleSummary)' -count=1 -v
```

Expected: PASS.

---

### Task 5: Harden Image Rewriting Coverage Without Regressing Working Images

**Files:**
- Modify: `server/internal/helpscout/images.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/docsimport/warnings.go`
- Modify: `server/internal/service/docs_import_external_image_test.go`
- Modify: `server/internal/service/docs_import_test.go`

- [ ] **Step 1: Write failing tests for currently missed image patterns**

Cover:
- `img[data-src]`
- single-quoted `src`
- duplicate URLs
- failed downloads keeping original URL with warning
- already rewritten storage URLs remaining untouched

- [ ] **Step 2: Replace regex-only image extraction with a real HTML-node walk if needed**

If the current regex approach in `helpscout/images.go` cannot safely cover the missing patterns, switch to HTML parsing so the rewrite logic can inspect:
- `src`
- `data-src`
- other common Help Scout image attributes

Keep the replacement behavior idempotent.

- [ ] **Step 3: Surface image rewrite failures into import warnings**

When an image cannot be downloaded or stored:
- keep the original URL
- emit a warning that contributes to import quality summary counts
- never fail the whole article import for one bad image

- [ ] **Step 4: Run image tests**

Run:
```bash
cd server && go test ./internal/service -run 'TestDocsImportService_ImportExternalImage|TestDocsImportService_ImageRewrite' -count=1 -v
```

Expected: PASS.

---

### Task 6: Improve Public Help Center Styling For Imported Docs

**Files:**
- Modify: `help-center/src/app.css`
- Modify: `help-center/src/components/ArticleContent.tsx`
- Create: `help-center/src/components/__tests__/ArticleContent.test.tsx`

- [ ] **Step 1: Write lightweight rendering tests for imported article structures**

Add tests that render article HTML containing:
- `h4` and `h5`
- callouts
- figures/images
- code blocks
- tables

The tests should verify structure remains present in the DOM and article enhancement logic still works.

- [ ] **Step 2: Extend prose styling conservatively**

Update [app.css](../../help-center/src/app.css) to give imported docs a better visual hierarchy without redesigning the site:
- add `h4`, `h5`, `h6` styles
- improve vertical rhythm for step-heavy docs
- improve spacing between headings, paragraphs, callouts, figures, and images
- ensure callouts and figures feel intentional instead of raw blocks

Constraints:
- keep the same design tokens
- do not change the editor
- do not introduce a separate imported-doc theme

- [ ] **Step 3: Adjust article enhancement logic only where safe**

In [ArticleContent.tsx](../../help-center/src/components/ArticleContent.tsx):
- keep copy-button behavior intact
- keep heading-ID enhancement intact
- only expand heading enhancement depth if it is needed for imported-doc navigation and does not clutter existing TOC behavior

- [ ] **Step 4: Run help-center tests and build**

Run:
```bash
cd help-center && npm exec vitest run src/components/__tests__/ArticleContent.test.tsx
cd help-center && npm run build
```

Expected: PASS.

---

### Task 7: Surface Import Quality Diagnostics In Existing Import Jobs

**Files:**
- Modify: `server/internal/model/docs_import.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `frontend/src/lib/services/docsImportService.ts`
- Modify: `frontend/src/components/settings/HelpCenterImportSection.tsx`

- [ ] **Step 1: Extend import summary shape with quality counters**

Add fields such as:
- `articles_uncategorized`
- `articles_with_conversion_warnings`
- `html_block_fallbacks`
- `image_rewrite_failures`
- `normalized_note_blocks`

Use the existing `summary` JSON field on `docs_import_jobs`.

- [ ] **Step 2: Aggregate warnings during import and reconvert**

Update import/reconvert logic so warning counts are tracked per article and rolled into the final job summary.

- [ ] **Step 3: Display summary signals in the existing import history UI**

Extend [HelpCenterImportSection.tsx](../../frontend/src/components/settings/HelpCenterImportSection.tsx) so completed jobs can show:
- article/collection totals
- quality warning counts
- reconvert result context

Keep the UI compact and aligned with existing table/badge styles.

- [ ] **Step 4: Run frontend typecheck and focused tests**

Run:
```bash
cd frontend && npm exec tsc --noEmit
cd frontend && npm exec eslint src/components/settings/HelpCenterImportSection.tsx src/lib/services/docsImportService.ts
```

Expected: PASS.

---

### Task 8: Repair Existing Imported Content Through The Reconvert Flow

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Modify: `frontend/src/components/settings/HelpCenterImportSection.tsx`
- Modify: `server/internal/service/docs_import_test.go`

- [ ] **Step 1: Write failing reconvert tests for Help Scout-specific normalization**

Add tests proving reconvert:
- uses the new Help Scout preprocessing path
- rewrites stored imported content from `import_source_html`
- counts conversion failures and warnings correctly
- does not require a fresh import to benefit from the new pipeline

- [ ] **Step 2: Ensure reconvert reuses the exact same hardened import pipeline**

Avoid duplicate logic. The reconvert path should invoke the same Help Scout normalization + conversion + warning aggregation used by fresh imports.

- [ ] **Step 3: Keep reconvert safety explicit**

Preserve the current destructive warning in the UI and make sure reconvert only targets documents with stored Help Scout import provenance.

- [ ] **Step 4: Run reconvert tests**

Run:
```bash
cd server && go test ./internal/service -run 'TestDocsImportService_Reconvert' -count=1 -v
```

Expected: PASS.

---

### Task 9: End-To-End Verification And Rollout Checklist

**Files:**
- No new code files; verification only

- [ ] **Step 1: Run the full targeted verification suite**

Run:
```bash
cd server && go test ./internal/docsimport ./internal/service -count=1
cd server && go build ./cmd/api
cd frontend && npm exec tsc --noEmit
cd frontend && npm exec eslint src/components/settings/HelpCenterImportSection.tsx src/lib/services/docsImportService.ts
cd help-center && npm exec vitest run src/components/__tests__/ArticleContent.test.tsx
cd help-center && npm run build
```

Expected: PASS across all commands.

- [ ] **Step 2: Manual QA on the exact Replug docs space**

In `test-docs` workspace, `Replug docs` space:
- verify `How to add ALT Text to your Posts?` renders its note block as a real callout
- verify `How to Add First Comment` no longer shows empty-heading/spacing artifacts
- verify `Bio-Links, A wonderful feature to Discover!` keeps callouts plus clearer `h4`/`h5` hierarchy
- verify a few already-good imported articles did not regress
- verify at least one non-imported native Helpin doc still looks correct

- [ ] **Step 3: Reconvert the existing Replug import job after deployment**

Use the existing `Re-convert` action in Settings → Help Center import history for the relevant completed Help Scout job. Confirm:
- imported content updates in place
- warning counts appear in the job summary
- no manual editor changes were unintentionally targeted outside imported docs

- [ ] **Step 4: Confirm import quality metrics after reconvert**

Check that:
- `Uncategorized` count drops or at least becomes explained via warnings
- literal escaped note blocks are gone from the sampled articles
- images remain present
- public spacing/heading hierarchy is visibly improved

- [ ] **Step 5: Commit in focused checkpoints**

Recommended commit sequence:
1. `test: add Help Scout import regression fixtures`
2. `feat: normalize Help Scout HTML before conversion`
3. `feat: harden import mapping and image rewrite diagnostics`
4. `feat: polish imported docs rendering and import summaries`
5. `feat: repair imported docs through reconvert pipeline`

---

## Execution Notes

- Use `@superpowers/test-driven-development` before each implementation task.
- Keep Help Scout logic isolated to the import/reconvert path; do not bleed source-specific heuristics into generic docs authoring behavior.
- Prefer deterministic mapping and conservative normalization over “smart” fuzzy behavior.
- If a source pattern cannot be safely transformed, preserve it and emit a warning rather than guessing.
- Do not skip the reconvert/remediation work; this rollout is incomplete if it only improves future imports.

