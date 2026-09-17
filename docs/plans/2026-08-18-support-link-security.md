# Support Link Security Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google Web Risk-backed malicious-link warnings and compact HTTP transport cues to support messages while keeping message delivery and ordinary link opening resilient to provider failures.

**Architecture:** A focused Go Web Risk client produces cached three-state verdicts, while the existing asynchronous link enricher independently scans URLs and fetches previews. Verdicts live in existing message JSON metadata, the agent inbox consumes them to decorate/intercept links, and public widget transports remove provider verdicts while the widget derives HTTP cues locally from each URL.

**Tech Stack:** Go 1.24 (`net/http`, `net/url`, `net.Resolver`), Google Web Risk Lookup REST API, React 19/TypeScript/Vitest, Preact widget core, existing shadcn alert dialog.

---

## File map

- Create `server/internal/service/support_link_security.go`: URL normalization, Web Risk Lookup client, three-state verdicts, and bounded in-memory cache.
- Create `server/internal/service/support_link_security_test.go`: normalization, API, timeout/failure, expiry, and cache tests.
- Modify `server/internal/service/support_link_preview.go`: independent scan/preview enrichment and SSRF-safe fetch transport/redirect handling.
- Modify `server/internal/service/support_link_preview_test.go`: extraction limits, independent metadata persistence, and SSRF behavior.
- Modify `server/internal/model/support_inbox.go`: persisted security metadata DTOs.
- Modify `server/internal/config/config.go`, `server/internal/config/config_test.go`, `server/.env.example`, and `server/cmd/api/main.go`: backend-only key configuration and dependency wiring.
- Modify `server/internal/handler/support_inbox_widget.go` and tests: remove `link_security` from widget HTTP responses.
- Modify `server/internal/websocket/support_events.go`, hub delivery code, and tests: remove `link_security` only for widget websocket recipients.
- Modify `frontend/src/lib/pm-types/support.ts` and `frontend/src/components/support/helpers.ts`: inbox security types, parsing, expiry, and URL matching.
- Create `frontend/src/components/support/SupportLink.tsx`: reusable decorated/intercepted inbox anchor and warning dialog.
- Modify `frontend/src/components/support/MessageBubble.tsx` and tests: use security-aware anchors and preview cards.
- Modify `packages/widget-core/src/components/MessageBubble.tsx`, `packages/widget-core/src/styles/widget.css`, and tests: HTTP icon/label only.
- Keep `.gitignore` backup protection in its own commit.

### Task 1: Configuration and Web Risk client

- [ ] Add failing config tests asserting `GOOGLE_WEB_RISK_API_KEY` is trimmed and absent keys disable scanning.
- [ ] Run `cd server && go test ./internal/config` and verify the new assertion fails.
- [ ] Add `GoogleWebRiskAPIKey string` to `Config`, load it from the environment, and document it in `.env.example` without a value.
- [ ] Run the config tests and verify they pass.
- [ ] Add table-driven failing tests for canonical URL normalization: case, punycode, ports, empty path, fragment, percent encoding, queries, user info, supported schemes, and specified punctuation vectors.
- [ ] Run `cd server && go test ./internal/service -run 'TestNormalizeSupportLinkURL'` and verify failures are due to the missing implementation.
- [ ] Implement `normalizeSupportLinkURL(raw string, trimMessagePunctuation bool)` with the design contract and rerun until green.
- [ ] Add failing `httptest.Server` tests for empty no-match response, supported threat response/expiry, malformed response, non-2xx, timeout, missing key, cache hits, and expiry.
- [ ] Implement `GoogleWebRiskClient.Scan(ctx, normalizedURL)` using `GET /v1/uris:search`, repeated `threatTypes` query parameters, and `x-goog-api-key`; allow only the three supported threat enums and never log the URL or key.
- [ ] Implement a mutex-protected bounded cache (oldest-expiry eviction at a modest fixed maximum), five-minute no-match TTL, provider malicious expiry, and 30-second unknown TTL; do not serve entries past expiry.
- [ ] Run focused service tests and commit the client/config slice.

### Task 2: Independent enrichment and SSRF-safe previews

- [ ] Add failing tests proving ten URLs are scanned while only three previews are fetched, scan results persist when preview fetching fails, preview results persist when scanning fails, and unrelated metadata survives.
- [ ] Extend `SupportLinkPreviewService` with a consumed `SupportLinkScanner` interface and inject it from `main.go`; merge `link_security` independently of `link_previews`.
- [ ] Ensure enrichment remains asynchronous and never turns scan/fetch failure into message creation failure; log only message/conversation IDs and error classes.
- [ ] Add failing SSRF tests for URL credentials, private/reserved IPv4/IPv6, `.local`, mixed public/private DNS answers, redirects to private hosts, redirect loops/limits, and valid HTTP-to-HTTPS redirect.
- [ ] Introduce an injectable resolver/dialer boundary. Resolve every direct target, reject if any result is non-public, pin the validated public IP for the connection, retain the original Host/TLS server name, and revalidate every redirect. For configured proxies, prevalidate each original/redirect host and retain explicit proxy resolution trust.
- [ ] Keep existing timeouts and one-megabyte body limit; do not request preview images.
- [ ] Run `cd server && go test ./internal/service -run 'Test(Support|GoogleWebRisk)'` and commit the enrichment/SSRF slice.

### Task 3: Prevent widget redistribution of verdicts

- [ ] Add failing handler tests showing widget message-list and send-message HTTP responses preserve `link_previews` but omit the `link_security` metadata key.
- [ ] Add a focused metadata sanitizer that copies message values and removes only `link_security`; use it at public widget HTTP serialization boundaries.
- [ ] Add failing websocket tests showing inbox recipients receive full metadata while widget recipients receive metadata without `link_security`.
- [ ] Sanitize support-message event payloads in the hub's widget-recipient delivery path, preserving all other fields and preview metadata.
- [ ] Run handler/websocket focused tests and commit the transport privacy slice.

### Task 4: Inbox parsing and link-warning component

- [ ] Add failing helper tests for valid/invalid `link_security`, supported threat filtering, expired verdicts becoming unknown, and Go/TypeScript canonical URL test-vector parity.
- [ ] Add `SupportLinkSecurity` types and parsing/matching helpers. Do not label no-match or unknown links safe.
- [ ] Add component tests for normal HTTPS, HTTP amber icon/tooltip, active malicious red icon/style, malicious dialog interception, full URL and threat labels, disabled continue before acknowledgment, exact new-tab open after acknowledgment, and expired/unknown fail-open navigation.
- [ ] Implement `SupportLink` with the existing alert-dialog and checkbox primitives. It owns pending destination and acknowledgment state, defaults focus to cancel, and calls `window.open(url, '_blank', 'noopener,noreferrer')` only after explicit acknowledgment.
- [ ] Run focused frontend tests and commit the reusable inbox security UI.

### Task 5: Apply security UI to message anchors and compact previews

- [ ] Add failing `MessageBubble` tests for both bare/labeled markdown anchors and compact preview cards: inline icon immediately after link text, `Not secure` beside HTTP preview domains, `Potentially harmful` beside malicious preview domains, and malicious interception from either surface.
- [ ] Replace the static markdown anchor renderer with a message-scoped renderer that passes each `href` and matching verdict to `SupportLink`.
- [ ] Route compact preview cards through the same warning state and render the approved compact status text.
- [ ] Preserve full destination hover text and `noopener noreferrer` for all ordinary external links.
- [ ] Run `pnpm --dir frontend test -- src/components/support/__tests__/MessageBubble.test.tsx` and commit the inbox integration slice.

### Task 6: Widget HTTP cues

- [ ] Add failing widget tests for amber icons after HTTP message anchors and `Not secure` beside HTTP preview domains, while HTTPS has no cue and malicious verdict metadata has no UI effect.
- [ ] Extend the widget markdown renderer/component hook to decorate HTTP anchors without adding modal behavior.
- [ ] Add minimal widget CSS for the inline amber icon and preview status text, preserving wrapping and accessible tooltip/title text.
- [ ] Run `pnpm --dir packages/widget-core test -- src/__tests__/MessageBubble.test.tsx src/__tests__/markdownRenderer.test.ts` and `pnpm --dir packages/widget-core typecheck`; commit the widget slice.

### Task 7: Verification, secret-backup ignore, and push

- [ ] Run `cd server && go test ./internal/config ./internal/service ./internal/handler ./internal/websocket`.
- [ ] Run the focused frontend and widget tests, widget typecheck/build, and frontend build.
- [ ] Run `git diff --check` and inspect `git status --short` plus the complete branch diff for secrets/raw keys and accidental unrelated changes.
- [ ] Commit the existing `.gitignore` `.env.bak*` protection separately.
- [ ] Use the verification-before-completion and requesting-code-review checklists directly, without sub-agents.
- [ ] Push `waqar-fixes` to `origin/waqar-fixes` and confirm the remote ref matches local HEAD.
