# Support Link Security Design

**Date:** 2026-08-18

## Goal

Protect support agents from known malicious links without interrupting ordinary conversations or treating every unfamiliar link as dangerous. HTTP links remain usable, while the support inbox clearly distinguishes an insecure transport from a confirmed malicious verdict.

## Product behavior

- Allow `http://` and `https://` links in the support inbox and customer widget.
- Show a subtle `Not secure` label for HTTP links. HTTP alone does not trigger a confirmation dialog and is not classified as malicious.
- Open external links in a new tab with `noopener noreferrer`.
- In the support inbox, open links normally when Google Web Risk reports no threat match or when no verdict is available.
- For a confirmed Web Risk match, prevent the first click from navigating and show a strong warning dialog containing the full URL and threat category. The agent can go back or explicitly acknowledge the risk and continue.
- Keep the customer widget frictionless in this first release: it receives compact previews and the HTTP indicator, but no malicious-link dialog. The security warning is an agent-protection feature initially.
- Do not add trusted-domain administration, allowlists, blocklists, or a warning for every unknown domain.

## Verdict semantics and outages

The backend uses three states:

- `no_match`: Web Risk returned successfully with no matching threat.
- `malicious`: Web Risk returned one or more supported threat types.
- `unknown`: scanning was disabled, timed out, rate-limited, failed, or returned an invalid response.

Only `malicious` triggers the inbox warning. `no_match` means no threat was found by this lookup; the UI must not describe the link as guaranteed safe. `unknown` is fail-open for message delivery and navigation but fail-closed for trust claims: the conversation remains usable, no safe badge is shown, and the system never converts an error into `no_match`.

Web Risk runs within the existing asynchronous message-enrichment path. It never delays or rejects message creation. Calls use a short timeout. A small process-local cache prevents repeated lookups: successful no-match responses receive a five-minute TTL, malicious responses honor the provider expiry with a bounded fallback, and failures receive a 30-second TTL to prevent an outage from creating a retry storm. No persistent global reputation database is introduced.

Every persisted entry has a valid `checked_at` and `expires_at`. Cached results retain the timestamps from the lookup that populated the cache. `expires_at` describes provider/cache freshness; it does not remove a warning from a historical message. Because enrichment currently runs only once, the inbox continues warning for a persisted malicious verdict after that timestamp. A future rescan can replace the verdict, but silently aging a known malicious result into an unprotected link is not allowed. Invalid statuses or timestamps parse as `unknown`.

If `GOOGLE_WEB_RISK_API_KEY` is absent, scanning is disabled and verdicts are `unknown`. The key is read only by the backend and sent in the `x-goog-api-key` header. Neither the key nor scanned URLs are written to logs.

## Metadata contract

Security results are stored independently from preview results so a malicious link remains protected even if its HTML preview cannot be fetched. Both stay inside the existing support-message JSON metadata, so no database migration is needed.

```json
{
  "link_previews": [
    {
      "url": "http://example.com/path",
      "host": "example.com",
      "title": "Example"
    }
  ],
  "link_security": [
    {
      "url": "http://example.com/path",
      "status": "malicious",
      "threat_types": ["SOCIAL_ENGINEERING"],
      "checked_at": "2026-08-18T12:00:00Z",
      "expires_at": "2026-08-18T12:30:00Z"
    }
  ]
}
```

URLs are normalized for matching by trimming only message punctuation that trails the extracted URL, then parsing an absolute URL, accepting only HTTP(S), removing the fragment, lowercasing the scheme and ASCII/punycode hostname, removing the default port (`80` for HTTP and `443` for HTTPS), and using `/` for an empty path. User information is rejected. Escaped path bytes and the query are otherwise preserved; query parameters are not reordered and trailing slashes are not removed. Duplicate normalized URLs are scanned once.

Trailing punctuation uses the existing deterministic extraction rule: repeatedly remove `.`, `,`, `!`, `?`, `:`, or `;`; remove `)`, `]`, or `}` only when the candidate contains more closing than opening characters of that pair; otherwise stop. No leading characters are trimmed. Thus `https://example.com/a).` becomes `https://example.com/a`, `https://example.com/a_(b)` retains its balanced parentheses, and `https://example.com/?q=a%29` is unchanged. The frontend applies URL normalization to the already parsed anchor destination and therefore does not perform message-punctuation trimming a second time.

Go and TypeScript implementations share table-driven expected input/output vectors for the examples above plus scheme/host case, default and non-default ports, empty paths, fragments, percent encoding, Unicode/punycode hosts, queries, and duplicates. Web Risk receives the complete normalized URL because its verdict can depend on the path and query. The UI normalizes rendered anchor and preview destinations with the same contract before matching them to security entries.

## Backend components and data flow

1. Extract normalized HTTP(S) URLs from a non-internal, non-system support message. Preview fetching retains its three-link limit; security scanning uses an independent ten-link limit. URLs beyond ten are unscanned/unknown and remain usable. This bound limits per-message cost and abuse and is covered explicitly by tests.
2. Scan each URL with a focused Web Risk client for `MALWARE`, `SOCIAL_ENGINEERING`, and `UNWANTED_SOFTWARE`.
3. Independently fetch compact preview metadata through the hardened preview fetcher.
4. Merge whatever results are available into message metadata. A scan failure does not discard a successful preview, and a preview failure does not discard a malicious verdict.
5. Persist the enriched metadata through the existing asynchronous support inbox/widget enrichment flow.

The Web Risk client has one responsibility: convert the Lookup API response into the three-state internal verdict. The preview fetcher has a separate responsibility: retrieve public HTML metadata safely. This keeps provider outages out of preview-fetch logic and makes both paths independently testable.

## Preview-fetch SSRF controls

- Accept only HTTP and HTTPS URLs with no embedded username or password.
- Reject localhost, `.local`, literal private/reserved addresses, and hostnames whose DNS results include non-public addresses.
- Revalidate every redirect destination and cap redirect depth.
- For direct connections, dial only a previously validated public IP to close the DNS-rebinding gap while retaining the original hostname for HTTP Host and TLS verification.
- Retain strict connection, response-header, total-operation, and one-megabyte response limits.
- Do not download preview images. Image and description fields may remain parse-compatible, but the inbox and widget do not render or fetch them.
- When an explicitly configured outbound proxy is used, validate the original and redirect hosts before forwarding. The proxy performs the final network resolution, so operational trust in that configured proxy remains explicit.

## Inbox UI

The inbox parses `link_security` alongside `link_previews`.

- HTTP anchors and preview cards receive a compact `Not secure` cue and expose the complete destination in the browser title/hover affordance.
- A click on a URL with a `malicious` verdict opens one shared warning dialog rather than navigating immediately.
- The dialog shows the destination and human-readable threat categories, defaults to the safe back action, and keeps the continue action disabled until the agent checks an acknowledgment box.
- Continuing opens the exact original normalized destination in a new tab with opener isolation.
- `no_match`, `unknown`, or absent verdicts do not show the malicious dialog. A persisted `malicious` verdict continues to warn after `expires_at`; expiry controls cache freshness, not historical UI enforcement.

The warning applies to both markdown anchors and compact preview cards. Internal app links and non-HTTP schemes are outside this feature.

## Widget UI

The widget adds only the subtle `Not secure` cue to HTTP compact previews and anchors. It does not parse, render, or react to Web Risk verdicts and adds no warning dialog in this release. The existing message endpoint may still carry `link_security` inside its raw metadata; this is acceptable because it describes a URL supplied in that same customer conversation and is not a secret. Existing `noopener noreferrer` behavior remains mandatory.

## Privacy and observability

- Google receives the normalized full URL, excluding its fragment. This can include path and query values and must be documented as third-party security processing.
- Logs contain only aggregate outcome, latency, and error class; never API keys or raw URLs.
- Metrics should count lookups, cache hits, no matches, malicious matches by category, timeouts, rate limits, and other provider failures.
- Roll out to staging first using Google's official test URLs, confirm metadata/UI behavior and log hygiene, then deploy to production.

## Testing

- Web Risk client tests cover no match, each supported threat, multiple threats, malformed responses, timeout, rate limit, provider failure, missing key, cache expiry, and failure-cache behavior.
- Preview security tests cover credentials, localhost, private/reserved IPv4 and IPv6, mixed DNS answers, DNS rebinding protection, redirect revalidation, redirect limits, response limits, and valid HTTP-to-HTTPS redirects.
- Metadata tests prove preview and security results persist independently and preserve unrelated message metadata.
- Inbox tests cover normal HTTPS, insecure HTTP cues, unknown/no-match navigation, malicious interception, threat labels, acknowledgment gating, and exact destination opening.
- Widget tests prove HTTP cues render while malicious dialogs remain absent.
- Focused backend and frontend suites, type checks, builds, and diff checks run before commit and push.

## Non-goals

- Declaring any URL absolutely safe.
- Warning on every unknown or untrusted domain.
- Workspace-managed trusted or blocked domain policies.
- A database-backed global reputation cache.
- Google Web Risk Update API or Extended Coverage.
- Customer-facing malicious-link warnings in the widget.
