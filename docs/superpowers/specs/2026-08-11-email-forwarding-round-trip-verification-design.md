# Email Forwarding Round-Trip Verification Design

## Goal

Only show email forwarding as verified after Helpin has observed mail travel through the customer's original inbox and back into the configured Helpin route.

## Behavior

- A newly enabled Shared or Team Inbox route is orange and marked `Setup incomplete`.
- The setup panel shows the generated Helpin address and asks for the original address being forwarded, such as `support@company.com`.
- Provider confirmation messages may arrive as inbox conversations so the user can open their link or retrieve their code. Receiving that message does not verify forwarding.
- `Send test` sends a uniquely tagged Helpin message to the original address.
- When that exact message returns through the route, Helpin consumes it as verification evidence rather than creating a customer conversation.
- A genuine non-confirmation message that references the configured original address also proves the route and may verify it while continuing through normal conversation processing.
- Green means `Verified <date>` and records that forwarding worked at that time. It does not claim continuous provider access.

## State

Extend `support_email_routes` with:

- `confirmation_received_at`
- `verification_sent_at`
- `forwarding_verified_at`
- an internal verification token
- `forwarding_last_error`

Keep `last_inbound_at` as operational activity only; it no longer determines verification status.

## API and Processing

- Add an admin-only `POST /support/inbox/email-routes/{routeId}/send-test` endpoint accepting `source_address`.
- Validate and store the source address, generate the marker, persist the pending test, and send through the existing Postmark client.
- Before normal route conversation processing, identify provider confirmation mail and exact Helpin test markers.
- Confirmation mail updates the confirmation timestamp and remains visible.
- Matching test mail atomically marks the route verified and is consumed.

## UI

- The Email Forwarding tab shows an inline checklist beneath incomplete routes:
  - Helpin forwarding address ready
  - Provider confirmation received, when detectable
  - Confirm and enable forwarding in the provider
  - Run forwarding test
- The Inboxes table stays orange until `forwarding_verified_at` exists.
- The route query polls briefly while a sent test is awaiting its return.

## Boundaries

- No Gmail, Microsoft, or other provider OAuth.
- No background health probes.
- No automatic clicking of links received by email.
- Existing routes remain active but show incomplete until they pass a test or receive qualifying real mail.

## Verification

- Backend tests cover pending state, confirmation mail, successful token round trip, ordinary qualifying mail, and no conversation for the test message.
- Frontend tests cover orange versus green status and checklist rendering/state.
