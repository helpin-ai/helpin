# Sender address settings UI implementation plan

This plan guides implementation of the approved sender-address settings design using Helpin's existing APIs. It records the capability audit and implementation results. The UI has been implemented locally; it has not been deployed.

**Status:** Implemented locally, 2026-10-16. UI/UX only; release pending.

**Goal:** Make support sender addresses easy to add, verify, and assign to inboxes without changing how Helpin sends or receives email.

**Architecture:** Replace the domain-first presentation with an address table and focused dialogs. Reuse the existing sender/domain queries, mutations, permissions, and DNS verification. No new database fields, endpoints, migration, background verification service, or email-routing changes.

**Stack:** React, TypeScript, TanStack Query, existing Helpin settings and Quiet UI components.

**Classification:** Shared product UI and documentation. Preserve the existing provider-configuration boundary; include no customer data or credentials.

> Execute in the existing `waqar-fixes` worktree using `superpowers:executing-plans`. Keep this as the single plan and do not use subagents.

## What stays and what changes

The current mounted screen is [SupportEmailSendersTab](../../frontend/src/components/settings/SupportEmailSendersTab.tsx), inside [InboxesRoutingSettingsPage](../../frontend/src/pages/settings/InboxesRoutingSettingsPage.tsx). Compare against this screen, not the older, unmounted custom-domain component.

| Capability | Implementation decision |
| --- | --- |
| Existing addresses and verified domains | Reuse their records, provider IDs, and verification state. Do not recreate them or require DNS changes because the UI changed. |
| Workspace default and inbox-specific senders | Preserve existing assignments. Workspace default applies to Shared Inbox and inboxes without their own sender; it does not override inbox-specific choices. |
| One address used for several inboxes | Keep the existing inbox multiselect and `mailbox_ids` support. |
| Add an address on a verified domain | Reuse the domain and skip DNS setup when verification is already complete. |
| Add an address on a new domain | Derive the domain from the full email address; use existing create and verify APIs. |
| Suggested address/name | Preserve the existing workspace-derived defaults, editable before creation. |
| Editing | Keep the optional name and inbox usage editable. Keep the email address immutable after creation, as it is today. |
| Stop using an address | Keep **Do not use** when editing. Retain the address in the table. **Set up only** is the corresponding creation choice. |
| DNS maintenance | **Manage DNS** on the address opens its records and allows rechecking, including already verified domains. Preserve per-record status and pending DKIM record handling. |
| Finish later | Save the address/domain using existing APIs. Keep a visible **Finish setup** action for unverified addresses. See the inbox-choice limitation below. |
| Forwarding and reply routing | Leave the separate Email forwarding tab, forwarding associations, Reply-To, and outbound fallback behavior unchanged. DNS verification does not imply inbound forwarding is configured. |
| Permissions | Use existing `support.admin` permission checks for actions and preserve backend enforcement. Do not broaden access. |
| Existing domain without an address row | Reuse it when an address is added. Do not build a separate legacy-domain management screen or migrate existing configurations. |

Intentional simplifications: remove domain-first navigation, nested address tables, the repeated forwarding link, and the final **Use address** confirmation. Put the usage choice in the initial form and apply it with existing APIs after successful verification.

## Important details the mockup alone does not cover

**An unfinished inbox choice is not currently stored by the API.** The current create endpoint saves an inactive address; active assignments require verified DNS. Keep the chosen inboxes in the current UI flow. If the user leaves and later returns without that draft, reopen the setup form to choose inbox usage before checking DNS. Preserve all already-saved assignments. Do not add server-side draft storage or imply that an unsaved assignment survived a refresh.

**Verification and assignment are separate requests today.** After successful verification, use the existing sender-update/default mutation for the choice made in the form. Show the simple confirmation only when required requests succeed. If verification succeeds but saving inbox usage fails, say so briefly and allow retrying the save without redoing DNS setup. A retry must reuse the saved address.

**A successful HTTP response is not proof that DNS is verified.** The existing verification result includes individual DKIM/Return-Path flags and `last_error`. Use those fields rather than the current generic success toast. Preserve separate pending, failed, and verified outcomes.

**The optional name has an existing behavior mismatch.** The settings API saves `display_name`, but the normal outbound path formats the From name as teammate + workspace. Preserve the stored value and current outbound behavior in this UI change. Recommend labelling the editable field **Sender label**, with a short tooltip explaining that it identifies the address in settings; do not promise a custom outgoing From name or change email branding as part of this work.

## UI to build

Keep the current **Inboxes & routing → Sender addresses** location and `?tab=senders` link.

- Header: **Sender addresses** and a visible **Add address** button.
- Description: **Send support replies from your own email addresses.**
- Table: **Email address**, **Used for inboxes**, **Status**, and the action menu. Show the optional label beneath the address. Keep inactive addresses visible with **Not in use** in the usage column.
- Form: full email, optional label, **Use for inboxes** with **Workspace default**, **Selected inboxes**, or **Set up only**. Editing uses **Do not use** for the last choice. Explain default fallback in a tooltip instead of repeating it across the page.
- Row actions: **Edit sender**, **Make workspace default** where applicable, and **Manage DNS**. Do not add deletion or new domain-management features.
- DNS dialog: DKIM and Return-Path, each with its own status. Put **Type**, **Name**, and **Value** on separate rows. Wrap long values and copy the complete raw value. Keep the approved short record descriptions.
- Instructions: separate **Copy for AI** and **Copy for developer** actions using the same records shown in the dialog.
- Checking: use the actual mutation's pending state for **Checking DNS records…** and disable duplicate checks. Do not use the mockup's artificial delay or simulated outcomes. A slow request must remain visibly in progress; pending DNS and request errors return to the records with a retry action.
- Confirmation: a subtle check, the address, **Verified**, and **Done**. No extra activation step. **Set up only** remains unused after verification.

Use existing typography, spacing, colors, buttons, dialogs, and tooltips. Keep the dialog body scrollable and actions visible on small screens. Preserve keyboard focus, accessible copy labels, and light/dark mode.

## Implementation tasks

### 1. Replace the list and form presentation

**Files:** `frontend/src/components/settings/SupportEmailSendersTab.tsx`, `frontend/src/components/settings/supportEmailSenderUtils.ts`; add focused `SupportEmailSenderDialog.tsx` and `SupportEmailSenderDNSDialog.tsx` beside them if needed to keep the existing large component readable.

- [x] Bind the flat table to the existing sender query; retain domain data for reuse and DNS display.
- [x] Build the add/edit form with existing defaults, validation, active inbox choices, and permission checks.
- [x] Preserve saved workspace/inbox mappings, inactive rows, and immutable email editing.
- [x] Reuse existing domain records on create. If domain creation succeeds but address creation fails, keep the form and allow retry without duplicating the domain.
- [x] Keep navigation and Email forwarding unchanged. Do not add a separate domain screen.

### 2. Connect the approved verification flow

**Files:** `frontend/src/components/settings/SupportEmailSendersTab.tsx`, proposed `SupportEmailSenderDNSDialog.tsx`, `frontend/src/hooks/queries/useSupport.ts` only where mutation result/cache handling needs adjustment. Existing API adapters are in `frontend/src/lib/services/supportService.ts`.

- [x] Reuse create, verify, and update/default mutations. Do not add an API contract.
- [x] Skip DNS for an already verified domain and save the selected usage through the existing mutation.
- [x] For unverified domains, retain the form choice during the open flow; apply it only after the returned DNS result is verified and has no verification error.
- [x] Preserve individual record states and key-rotation data. A verified current key must not falsely mark a different pending key as verified.
- [x] Replace the unconditional verification-success toast with accurate pending/error/success feedback.
- [x] Handle assignment failure after DNS success with a retryable save state. Do not announce completion before both operations finish.
- [x] On Finish later, retain the saved address. On resume without the form draft, show the usage choice again; never guess a default from an old related-inbox association.
- [x] Refetch/invalidate existing sender/domain queries after changes so the table and inbox settings agree. On reopening after a slow request, read current server state before retrying. Do not imply closing a dialog undoes a server request.

### 3. Generate the two instruction formats

**Files:** create `frontend/src/components/settings/supportEmailSenderInstructions.ts`; call it from the DNS dialog.

- [x] Generate both formats from actual domain records, address, and the current Helpin settings URL; do not hardcode hosted URLs or sample records.
- [x] Include exact type/name/value, what each record does, checking instructions, and the distinction between DNS pending and a failed check.
- [x] AI instructions should request adding only the required missing records, handling conflicting records explicitly, and returning to Helpin to check verification. Do not assume the assistant has DNS or Helpin access.
- [x] Developer instructions should provide concise manual DNS steps. Mention DNS-only for the Return-Path CNAME where the provider supports proxying.
- [x] Preserve unrelated DNS records. Do not request changing MX, SPF, or DMARC as part of these two records. Do not copy credentials or hidden verification tokens.
- [x] Use the existing clipboard helper and provide copy feedback; retain complete long values.

### 4. Verify behavior and review the implementation

**Files:** existing `frontend/src/components/settings/__tests__/supportEmailSenderPreview.test.ts`, `senderGroupsExpanded.test.ts`, `inboxSenderFallback.test.ts`; add `SupportEmailSendersTab.test.tsx` and `supportEmailSenderInstructions.test.ts` in that directory for changed behavior.

- [x] Replace the obsolete nested-table source assertion with meaningful address-list behavior coverage.
- [x] Cover verified-domain reuse, unverified setup, Finish later/resume, single/multiple inbox choices, do-not-use, partial creation failure, verification pending/error, and assignment failure after verification.
- [x] Check permissions, copied record fidelity, and that existing sender settings render without being reset or resaved automatically.
- [x] Run focused frontend tests, then Community and EE frontend builds. Review light/dark and narrow layouts in one browser pass using representative fixtures. Do not write tests for copy, spacing, or colors.
- Deferred outside this UI-only change: backend reassignment between two senders that share inbox mappings. Source review identified a possible existing edge case; this task does not change that repository behavior. Frontend checks cover single/multiple inbox payloads and prove that editing only a label does not resave assignments.
- [x] Review the diff for unintended API, routing, schema, or forwarding changes. Update this plan with actual results and any remaining limitations.

Suggested focused test command, after adding the named tests:

```sh
pnpm --dir frontend test src/components/settings/__tests__/SupportEmailSendersTab.test.tsx src/components/settings/__tests__/supportEmailSenderInstructions.test.ts src/components/settings/__tests__/supportEmailSenderPreview.test.ts src/components/settings/__tests__/inboxSenderFallback.test.ts src/components/settings/__tests__/EmailForwardingSetup.test.tsx src/pages/settings/__tests__/InboxesRoutingSettingsPage.test.ts
pnpm --dir frontend build
pnpm --dir frontend build:ee
```

Publishing or deployment is a separate action. See the actual results below.

## Existing contracts consulted

- [Sender/domain data and request fields](../../server/internal/model/support_inbox.go)
- [Sender validation and activation](../../server/internal/service/support_email_sender.go)
- [Domain verification and status propagation](../../server/internal/service/support_email_sender_domain.go)
- [Workspace and inbox sender mappings](../../server/internal/repository/support_email_sender.go)
- [Outbound address fallback](../../server/internal/service/support_email_route.go)
- [Actual outgoing From name and reply handling](../../server/internal/service/email_fallback.go)
- [Frontend sender types](../../frontend/src/lib/pm-types/support.ts)

These contracts remain unchanged by this plan. Existing configured addresses and domains continue working; the redesign changes how users manage them.

## Implementation results

- Replaced the nested domain screen with the address table, add/edit dialog, and DNS dialog using existing API hooks. No backend files, schemas, or API contracts changed.
- Retained domain reuse, inbox defaults, multiple-inbox selection, inactive addresses, and DNS maintenance. Sender-only configurations can use the existing sender verification endpoint when no separate domain row exists.
- Added complete AI/developer instruction generation and accurate partial-verification/error feedback. Inbox-save retries reuse the address and do not repeat successful DNS checks.
- All 34 focused frontend tests pass, covering the settings flow, existing forwarding UI, navigation, and instruction generation. Both Community and Enterprise frontend builds pass (existing large-chunk warnings only).
- Browser checks used the real components and HTTP adapters with fictional API fixtures. Light/dark layouts, keyboard focus, long DNS values, a 390px viewport, verification, confirmation, and Manage DNS were checked. No live DNS records or email settings were modified.
- The changed sender components pass ESLint. The settings page has an existing `react-refresh/only-export-components` violation on its exported tab-normalization helper; the same violation is present in HEAD.
- Draft inbox choices remain local to the open dialog, as planned. Returning later requires choosing usage again. DNS calls use the existing API behavior; this UI change does not add a new network timeout or background polling.
- Removed the sender-address mockup source, generated assets, screenshots, and temporary browser-check entry point; stopped its preview server. Other mockups were left alone.
