# Shared Support email projection implementation plan

> Historical implementation plan, source-compared on 2026-09-17. This page
> explains the shared email display projection for contributors. The main path is
> implemented; unchecked tasks and original test commands are historical.

## Current implementation and limits

The [inbound converter](../../server/internal/email/inboundhtml/convert.go)
provides `Project` with HTML, full text, and stripped reply inputs, while retaining
`Process` compatibility. Structural quote markers take precedence over a
conservative plain-text split. Projection version is currently 1. Sanitized HTML
and Markdown outputs are capped (`maxHTMLLen = 200000`, `maxMarkdownLen = 50000`),
so the original “lossless” wording is not an unlimited preservation guarantee.
Quote detection and confidence describe parser heuristics, not certain authorship.

[Inbound handling](../../server/internal/service/email_fallback.go) stores visible
and quoted projections and handles forwarded-customer attribution. The columns
are present in the [core foundation schema](../../server/internal/dbmigrate/sql/000000000001_core_foundation.sql).
[Hydration](../../server/internal/service/support_inbox.go) uses persisted fields
at the current version; older versions are reprojected in memory. That fallback
uses stored stripped text for both text inputs and cannot reconstruct missing
original content. Forwarded attribution preserves message content as visible and
clears quoted history in that legacy fallback. Hydration sets an explicit boolean
pointer even when no quote exists; it does not constitute a persisted backfill.

[Websocket mapping](../../server/internal/websocket/support_events.go) and the
[SDK](../../packages/sdk-js/src/core/widget.ts) carry the projection fields, including
explicit false. The [inbox bubble](../../frontend/src/components/support/MessageBubble.tsx)
uses projected text and collapsed rich HTML. The
[widget bubble](../../packages/widget-core/src/components/MessageBubble.tsx)
uses visible text for email and shows a quote toggle only when the quote flag is
true and quoted text is nonempty. The two renderers share data, not identical HTML
presentation or an assurance that every provider format is recognized.

No fresh ingestion, provider, browser, or full regression run was performed for
this source comparison. Schema presence does not establish deployment, and the
old branch/build limitations below are not current verification results.

## Original implementation record

> **For agentic workers:** REQUIRED: Use `superpowers:executing-plans` to implement this plan. Do not use sub-agents for this task.

**Goal:** Give the support inbox and public widget one backend-owned, lossless projection of the visible inbound-email reply and its collapsible quoted history.

**Architecture:** Extend `internal/email/inboundhtml` to produce sanitized annotated HTML plus visible/quoted Markdown and confidence/version metadata. Persist that additive projection on `support_email_logs`, hydrate it onto `SupportMessage`, and map the optional fields through websocket/shared SDK contracts. The inbox keeps rich iframe rendering while the widget renders visible Markdown with an explicit quoted-history toggle.

**Tech Stack:** Go 1.24, GORM, goquery/bluemonday, React 19, Preact, TypeScript, Vitest, pnpm.

---

### Task 1: Backend email projection

**Files:**
- Modify: `server/internal/email/inboundhtml/convert.go`
- Modify: `server/internal/email/inboundhtml/convert_test.go`

- [ ] Add failing tests for projection output from Gmail/Outlook/Apple/Yahoo/Proton wrappers, the production Word/Outlook header shape, localized Word labels, nested quote nodes, and ambiguous bordered content.
- [ ] Add failing tests for deterministic `TextBody`/`StrippedTextReply` splitting, including no match, repeated/ambiguous match, and empty remainder.
- [ ] Run `GOCACHE=/tmp/go-build-cache-waqar-fixes go test ./internal/email/inboundhtml` and confirm the new tests fail for missing projection fields/behavior.
- [ ] Extend `ProcessedContent` with `QuotedMarkdown`, `HasQuotedContent`, `ProjectionConfidence`, and `ProjectionVersion`; add a structured projection input containing HTML, full text, and stripped reply.
- [ ] Refactor DOM processing so known wrappers and existing `data-helpin-quote` markers are high-confidence boundaries, localized Word header blocks are medium-confidence boundaries, and only top-level quote nodes are extracted.
- [ ] Preserve `Process`/`Convert` compatibility wrappers while routing new ingestion through the structured projection API.
- [ ] Run the package tests until green and format Go files.

### Task 2: Persistence, hydration, and live payload parity

**Files:**
- Modify: `server/internal/model/support_email_log.go`
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/service/email_fallback.go`
- Modify: `server/internal/service/email_fallback_test.go`
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/service/support_inbox_email_hydration_test.go`
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/websocket/support_events.go`
- Modify: `server/internal/websocket/support_events_test.go`
- Modify if required by SQLite fixtures: `server/internal/service/testdb_test.go`

- [ ] Add failing ingestion tests asserting projection fields are stored and forwarded-attribution messages retain the forwarded customer body as visible content.
- [ ] Add failing hydration tests for current persisted projections, explicit `false`, and legacy on-read projection from old HTML.
- [ ] Add a failing websocket test asserting the projection fields are included in `WidgetMessageReceivedPayload`.
- [ ] Run the targeted Go tests and confirm expected failures.
- [ ] Add additive `SupportEmailLog` columns for visible text, quoted text, quote presence, confidence, and version. Add optional virtual `SupportMessage` fields and widget payload fields.
- [ ] Replace ingestion's tuple-only body selection with the structured projection, rerun it after CID rewriting, and populate both the email log and live message.
- [ ] For confidently forwarded messages, set visible projection text to the cleaned forwarded customer body and do not classify that body as hidden history.
- [ ] During hydration, use current persisted fields or project stale/missing historical rows in memory; return an explicit false pointer for a current no-quote projection.
- [ ] Include projection fields in support-message websocket events.
- [ ] Run targeted service/model/websocket tests until green and format Go files.

### Task 3: Shared message and SDK mapping

**Files:**
- Modify: `packages/shared/src/types/message.ts`
- Modify: `packages/sdk-js/src/core/widget.ts`
- Modify: `packages/sdk-js/test/unit/core/widget.test.ts`

- [ ] Add a failing SDK test proving session-join and conversation-message payloads map all optional projection fields, including explicit `false`.
- [ ] Run `pnpm exec vitest run test/unit/core/widget.test.ts` in `packages/sdk-js` and confirm the new assertion fails.
- [ ] Add optional camelCase projection fields to the shared `Message` type and map their snake_case API counterparts in `mapSupportMessage`.
- [ ] Run the targeted SDK test until green.

### Task 4: Support inbox integration

**Files:**
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/components/support/MessageBubble.tsx`
- Modify: `frontend/src/components/support/__tests__/MessageBubble.test.tsx`
- Modify only if projection metadata is needed by the iframe: `frontend/src/components/support/EmailBodyRenderer.tsx`

- [ ] Add failing tests that email copy/reply display uses `email_visible_text`, forwarded messages retain their visible customer body, and ordinary rich email always starts collapsed even with attribution metadata.
- [ ] Run `pnpm exec vitest run src/components/support/__tests__/MessageBubble.test.tsx` in `frontend` and confirm expected failures.
- [ ] Add projection fields to `SupportMessage`; choose projected visible text for inbox actions and Markdown fallback.
- [ ] Keep rich HTML in the message bubble and detailed-email dialog, but always default ordinary quoted HTML to collapsed; retain the existing DOM detector as a legacy fallback.
- [ ] Keep attachments outside the email body/collapse region.
- [ ] Run the targeted inbox tests until green.

### Task 5: Widget quoted-history UI

**Files:**
- Modify: `packages/widget-core/src/components/MessageBubble.tsx`
- Modify: `packages/widget-core/src/styles/widget.css`
- Modify: `packages/widget-core/src/__tests__/MessageBubble.test.tsx`
- Modify: `packages/widget-core/src/__tests__/widgetStyles.test.ts`

- [ ] Add failing tests asserting only `emailVisibleText` is initially rendered, `Show previous messages` reveals `emailQuotedText`, explicit false/no quoted text shows no toggle, and attachments stay visible in both states.
- [ ] Add a failing style test for width-safe quoted content and long URLs/code.
- [ ] Run `pnpm exec vitest run src/__tests__/MessageBubble.test.tsx src/__tests__/widgetStyles.test.ts` and confirm expected failures.
- [ ] Add a small email-quoted-content component inside the message bubble, using the existing Markdown sanitizer and local expanded state.
- [ ] Render the email badge and attachments outside the collapsible section; preserve customer/agent alignment.
- [ ] Add subdued quote/toggle styling with `min-width: 0`, `max-width: 100%`, and anywhere wrapping/scroll-safe code behavior.
- [ ] Run targeted widget tests until green.

### Task 6: Verification and commit

**Files:** all files above.

- [ ] Run focused Go tests for `internal/email/inboundhtml`, projection hydration/ingestion, and websocket events.
- [ ] Run focused frontend support, widget-core, and SDK tests directly with `pnpm exec vitest run`.
- [ ] Run relevant broader regression suites that do not require production builds. Record the known baseline `reply-time-parity` limitation if shared `dist` remains absent.
- [ ] Run `git diff --check`, inspect `git diff --stat`, and review every changed hunk for unrelated edits or canonical-content loss.
- [ ] Commit the completed implementation on `waqar-fixes` with a focused message.
