# Shared Support email projection design

> Historical design, source-compared on 2026-09-17. This page explains the
> shared email display contract for contributors. The main projection path exists;
> the original baseline and unlimited preservation assumptions are not current
> guarantees.

## Current implementation and qualifications

The [converter](../../server/internal/email/inboundhtml/convert.go) implements
versioned HTML/visible/quoted projections and confidence values. Plain-text
splitting requires stripped reply text to be a unique prefix of the full text,
with a nonempty remainder, rather than an arbitrary matching substring. Converted
HTML and Markdown have configured size caps, so “complete”/“never discard” below
must not be read as unlimited display-output preservation. Retaining a raw log
is separate from what a UI projection can show.

[Ingestion](../../server/internal/service/email_fallback.go) also recognizes
Helpin's own reply delimiter and has empty-visible-text fallbacks. Forwarded
attribution uses the customer body as visible content. The
[hydration path](../../server/internal/service/support_inbox.go) reprojects stale
logs in memory from available stored HTML/stripped text; it does not rewrite old
rows or recover content that was never retained. See the
[reviewed implementation plan](../plans/2026-08-05-shared-support-email-projection.md)
for detailed bounds and current source links.

The [core foundation migration](../../server/internal/dbmigrate/sql/000000000001_core_foundation.sql)
contains projection columns. The original AutoMigrate-only statement is not a
valid migration policy for all deployments: installations with AutoMigrate disabled
need versioned schema changes. Do not edit an already-applied foundation migration
to deliver future projection changes.

The [inbox iframe renderer](../../frontend/src/components/support/EmailBodyRenderer.tsx)
receives HTML and collapse options, not `email_has_quoted_content`; it derives
quote presentation from the document/DOM path. The
[widget renderer](../../packages/widget-core/src/components/MessageBubble.tsx)
uses the explicit flag and nonempty quoted Markdown for its toggle.
[Websocket payload mapping](../../server/internal/websocket/support_events.go)
passes projection fields without an HTML body. These are related but distinct
rendering contracts, not identical presentation across every transport and client.

The test matrix and “no production build required” statement below describe the
original change scope, not permanent validation policy or fresh test results.
No provider ingestion, browser rendering, or database migration was run here.

## Original design record

## Context

Inbound support email currently has three competing representations:

- `support_messages.content`, chosen during ingestion for ordinary chat rendering;
- `support_email_logs.html_body`, sanitized and annotated for the support inbox iframe;
- `support_email_logs.stripped_text`, currently populated from the selected message content rather than retained as an independent display projection.

The support inbox collapses quoted HTML client-side. The chat widget ignores the email HTML and renders only `content`. As a result, the same historical email can show a collapsed prior thread in the inbox and a fully expanded prior thread in the widget. Outlook/Word messages without standard element IDs can also evade backend quote detection and depend on a separate English-only frontend heuristic.

## Goals

- Preserve the complete canonical inbound email and never discard quoted history.
- Produce one backend-owned email display projection used by the support inbox and widget.
- Show the latest visible reply by default while keeping quoted history accessible.
- Improve structural and localized Outlook/Word quote detection.
- Preserve forwarded-message attribution and the actual forwarded customer body.
- Keep attachments independent from quote collapse.
- Fall back to full content whenever the parser cannot confidently identify a boundary.

## Non-goals

- Rendering raw email HTML in the public widget.
- Replacing the detailed-email dialog or removing the canonical HTML/plaintext email log.
- Perfect natural-language signature extraction.
- Hiding content solely because it resembles a header without sufficient structural evidence.

## Chosen Architecture

Extend `server/internal/email/inboundhtml` from a two-value converter into the canonical display-projection producer. The producer receives the original HTML, Postmark `TextBody`, and Postmark `StrippedTextReply`. Its result will retain the sanitized annotated HTML and expose:

- visible Markdown;
- quoted Markdown when a confident structural boundary exists;
- whether quoted content exists;
- confidence (`high`, `medium`, or `none`);
- a numeric parser version.

The projection is persisted on `support_email_logs` when an inbound email is created. The original raw payload, sanitized full HTML, and existing message content remain intact. Adding columns is additive and will use the project's GORM AutoMigrate path. Existing rows without a current projection are projected on read from their stored HTML/text so historical conversations improve without a destructive data migration.

`SupportMessage` receives virtual projection fields during email-log hydration. Both authenticated inbox endpoints and widget endpoints already use this hydration path, so the API contract remains additive.

## Quote Detection

Detection is DOM-first and conservative:

1. Recognize known structural wrappers already supported by the processor: Gmail, Outlook/Exchange IDs, Apple Mail cite blockquotes, Yahoo, and Proton.
2. Recognize already annotated `data-helpin-quote` elements so persisted sanitized HTML can be re-projected safely.
3. Recognize Outlook/Word header blocks only when the element has a quote-boundary visual structure (for example a top border) and contains enough header roles to establish a boundary.
4. Header roles use localized aliases for common variants of From, Sent/Date, To, and Subject. Matching is case-insensitive and Unicode-aware after whitespace normalization.
5. Mark the boundary and following siblings as quoted content. Avoid duplicate extraction when quoted nodes are nested.

Structural wrappers and explicit annotations are high confidence. Localized Word-header heuristics are medium confidence. A plain-text marker without supporting structure does not hide content by itself.

If no confident DOM boundary exists, Postmark's `StrippedTextReply` may establish a plaintext boundary only when it can be matched deterministically inside `TextBody` and the remaining text can be retained as `quoted_markdown`. If that split cannot be proven, `visible_markdown` remains the complete safe message and `has_quoted_content` is false. This prevents legitimate content from disappearing or becoming inaccessible.

## Ingestion and Persistence

At inbound-email ingestion:

1. Process the HTML and Postmark `StrippedTextReply` together.
2. Prefer the processor's visible Markdown and quoted Markdown when it found a structural boundary.
3. When no structural boundary is found, accept Postmark's non-empty stripped reply only when the full plaintext can be split deterministically into that visible reply plus a non-empty quoted remainder. Store both pieces.
4. When neither boundary is proven, use the complete safe message as the visible projection and expose no quoted section.
5. Store projection fields on `support_email_logs` with the parser version.
6. Continue storing the canonical `support_messages.content` for compatibility and search. Do not rewrite historical message rows.

Forwarded attribution remains backend-owned. When a forwarded sender is confidently attributed, the forwarded customer body remains visible; the forwarded header metadata may be represented as quoted/auxiliary content, but the body itself must not be hidden.

## API Contract

Add optional fields to support messages and the shared widget message type:

- `email_visible_text`
- `email_quoted_text`
- `email_has_quoted_content`
- `email_projection_confidence`
- `email_projection_version`

Optional fields preserve compatibility with older servers, SDK bundles, cached websocket payloads, and non-email messages. `email_has_quoted_content` is nullable/optional on the API projection: absent means a legacy or unprojected message, while an explicit `false` means the current parser found no safe boundary. `email_projection_version` provides the same distinction for consumers that need to validate the whole projection.

The SDK maps these fields for session joins, conversation selection, HTTP refreshes, and websocket-delivered messages. Websocket payload creation must carry the same fields or the client must refresh the canonical message before rendering; the preferred implementation is to include them directly.

## Support Inbox Rendering

The support inbox continues to use sanitized HTML for visual fidelity. Backend annotations become the primary collapse boundary. The existing iframe toggle remains the renderer for rich quoted history.

Changes:

- Collapse annotated quote history by default for every ordinary inbound email.
- Do not expand the entire HTML merely because forwarded attribution exists.
- Use `email_visible_text` for copy, quote-reply, shortcut creation, and Markdown fallback.
- Use `email_has_quoted_content` to decide whether a quote toggle is expected, while retaining iframe DOM detection as compatibility fallback for older rows.
- Keep the detailed-email dialog expanded because it is explicitly opened to inspect the complete email.
- Keep all attachments outside the collapsed HTML region.

## Widget Rendering

The widget remains plaintext/Markdown-based and never receives or renders raw email HTML.

- Render `email_visible_text` when present, otherwise `content`.
- Show a compact `Show previous messages` control only when `email_has_quoted_content` and `email_quoted_text` are present.
- Render quoted text in a secondary, visually subdued block that can be expanded and collapsed.
- Keep the `Via email` badge and all attachments outside that block.
- Apply the existing long-URL/code width protections to both visible and quoted content.

## Failure Handling and Compatibility

- Parser failure returns complete safe content and no collapse boundary.
- Missing projection fields use existing behavior.
- Existing logs are projected in memory during hydration when their version is missing or stale.
- Projection errors must not prevent listing a conversation; log structured diagnostics without email body content.
- The parser version allows future detection improvements without silently treating stale persisted projections as current.

## Testing

Backend unit coverage will include:

- Gmail, Outlook IDs, Apple Mail, Yahoo, Proton, and nested quote wrappers;
- the production Outlook/Word sample shape;
- localized Word headers;
- ambiguous bordered content that must remain visible;
- deterministic Postmark plaintext splitting, including ambiguous matches and empty quoted remainders;
- forwarded content where the original customer's body remains visible;
- projection persistence and legacy on-read fallback;
- explicit `email_has_quoted_content: false` serialization for a current projection;
- widget message payload hydration.

Frontend and SDK coverage will include:

- inbox rendering uses visible projection and defaults rich history to collapsed;
- detailed-email dialog remains fully expanded;
- widget renders only the latest reply by default;
- widget quote toggle reveals and hides previous messages;
- attachments remain visible in both states;
- SDK maps projection fields across initial and refreshed message payloads;
- long URLs and code in quoted content cannot widen the widget or inbox thread.

No production build is required for this change. Verification will use targeted Go, frontend support, widget-core, shared-package, and SDK tests, followed by relevant regression suites and `git diff --check`.
