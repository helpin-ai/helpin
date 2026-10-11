# Conversation merging and unmerging

Status: implemented and locally verified; not deployed. Researched against Intercom's public documentation and Helpin commit `e1f68a638` on 2026-10-11. This document is the single design and implementation checklist for this task. The proposed feature is shared Community and cloud behavior.

Help a teammate bring duplicate conversations about one issue into one place, continue answering the customer, and separate the conversations again without losing their history. Merging conversations must not merge CRM contacts or companies.

## Confirmed product decisions

- Same-customer merging is available by default. An administrator can enable cross-customer merging for the workspace.
- Unmerge retains the discussion from the merged period as labelled history. It does not guess which new replies should move to which original conversation.
- Use Helpin's existing support interface and Quiet components. Keep the common flow short; disclose history and secondary details on demand.

The owner approved end-to-end implementation on 2026-10-11, with an explicit requirement to reuse the real conversation thread components for previews and history. Work remains in `waqar-fixes`; no separate worktree or planning document. All new functionality, schema, tests, and documentation are shared Community/cloud product behavior. Fixtures use fictional data only.

## What the Intercom research establishes

The [merge article](https://www.intercom.com/help/en/articles/8862676-merging-conversations-and-tickets) describes menu/command entry, content/title/ID search, a preview, and a surviving primary. Sources become read-only; history remains separate with a summary/link. Cross-user merging is opt-in and adds participants, potentially exposing history. Undo requires support. Multi-participant conversations are excluded; batches require successive merges. Fin and SLAs continue on the primary; reporting retains sources with a merge filter.

Its channel restrictions include same-user/same-channel social merges, same-channel Slack, and asymmetric phone merges; open WhatsApp favors the newer conversation. Customer tickets qualify; Back-office/Tracker tickets do not. The widget offers continuation only when the primary is accessible; older mobile SDKs have a broken-button case. Ticket resolution follows configured state ordering. Merge notices cannot be deleted through the UI.

All six article screenshots were inspected and revisited after feedback: source event, destination summary, security setting, widget continuation, widget without continuation, and reporting filter. They illustrate outcomes; they do not establish the exact current picker layout. The Helpin picker and comparison layout below are original proposals, not a claim to reproduce an undocumented Intercom screen. Its destination summary screenshot informs the compact source entry in the second mockup. The [conversation events guide](https://www.intercom.com/help/en/articles/13334840-conversation-events-in-the-inbox) also informs left-aligned activity and secondary details on demand.

Additional references inform separate aspects of the design:

- [Merge API](https://developers.intercom.com/docs/references/rest-api/api.intercom.io/conversations/mergeconversation): authenticated source-to-target operation, explicit write scopes, and return of the primary record.
- [Teammate permissions](https://www.intercom.com/help/en/articles/176-teammate-permissions-how-to-control-workspace-access): merging and participant management have explicit capabilities.
- [Group conversations](https://www.intercom.com/help/en/articles/6435588-manage-group-conversations-in-the-inbox): participant membership and the recipients of a particular reply are related but distinct.
- [Email threading](https://www.intercom.com/help/en/articles/7996715-email-threading): reply headers preserve thread identity; a new sender referencing a thread does not automatically become a participant.
- [History in email notifications](https://www.intercom.com/help/en/articles/3527623-sending-email-notifications-without-conversation-content-or-history): prior history and the current reply need separate disclosure controls.

Helpin should improve reversibility, bulk operation, and access to older history. Provider restrictions and information already delivered to a customer cannot be undone by an interface improvement.

## Approach

| Approach | Benefit | Cost |
| --- | --- | --- |
| **Preserve records, merge their handling — recommended** | One active conversation, original links survive, reliable unmerge | Requires shared routing, participant, and history contracts |
| Move every message into the primary | Simple combined timeline initially | Rewrites ownership of attachments, delivery logs, references, and reporting; later separation becomes ambiguous |
| Close sources and add links | Small implementation | Fragmented context and weak recovery; does not meet this feature's goal |

The primary is the conversation the team continues working in. A source retains its ID, original messages, attachments, customer association, and metadata. A durable relationship makes the primary responsible for new handling while the source is merged. This relationship has an explicit beginning and end.

## Entry points and layout

### Open a merge

- Conversation header and row menu: **Merge into…**. Both use the same dialog.
- Existing **Other Conversations** rows: their action menu offers **Merge into current conversation…**, opening the same preview with the direction already chosen.
- Bulk toolbar: **Merge…** when at least two conversations are selected. Choose the primary from the selection, or search for another conversation.
- The support-scoped command menu can expose the same action. Do not replace the application's existing Cmd/Ctrl+K behavior or create a conflicting shortcut.
- Hide mutation actions for viewers. A visible but ineligible record gets a short reason. Never reveal inaccessible conversation titles, participants, previews, or counts through search.

The dialog has two stages: choose a conversation, then review. Single merge starts with the current conversation as the source. A bulk merge requires explicit primary selection; the newest record is suggested, not silently chosen.

```text
Merge into                                                   ×
From #1048 · Cannot sign in

[ Search by subject, message, or conversation number        ]

Same customer
  #1032  Cannot sign in                                      ›
         Sam Taylor · Email · Open · Support · 12 min ago
         “The reset link has expired…”
  #998   Password reset                                     ›
         Sam Taylor · Chat · Resolved · Oct 8

                                                      Cancel
```

Use the existing search backend, with server-side customer and access filtering. Default results show recent eligible conversations from the same customer; relevance controls ordering for an entered query. Selecting a result opens the conversation comparison directly. Do not make the teammate open another preview just to read the messages they are about to merge.

When cross-customer merging is enabled, show **Search all customers** as a quiet scope control. Keep same-customer results first. Search accepts display number, UUID, subject, message content, name, and email only where the backend actually implements those fields. Match the placeholder to the implemented fields.

### Review before merging

```text
Merge conversations                                                       ×

Merge from                    Swap  →  Merge into                    Change
Sam Taylor                             Sam Taylor
Cannot sign in              #1048      Unable to reset password       #1032
Chat · Open                            Email · Open · Support · Alex
─────────────────────────────────      ────────────────────────────────────
[Original conversation messages]       [Destination conversation messages]
[Internal notes, clearly marked]       [Customer and teammate replies]

Kept as conversation history           Stays active for new replies
───────────────────────────────────────────────────────────────────────────
1 AI follow-up will stop                         Back      Merge into #1032
```

The desktop comparison shows both actual transcripts in bounded, independently scrollable columns. The source and destination roles remain labelled, with a directional arrow and the destination number in the confirm button. **Swap** reverses a single merge; **Change** returns to the destination picker. On narrow screens a labelled source/destination switcher shows one readable transcript at a time, with confirmation controls kept visible. Bulk merging reuses this comparison, with source switching in the left column. Do not preselect a bulk primary.

The primary's inbox, assignee, and channel are shown once beside its transcript. Status changes, added participants, new team access, or stopped scheduled work appear only when applicable. Essential changes remain visible, not in tooltips. Do not add a field-by-field settings form.

For a cross-customer merge, the comparison already shows both identities and email addresses. Add a visible consent row: **Share future replies with Sam and Maya**, with **Earlier conversations stay private** underneath. Do not repeat the same two identities in another tall section. For larger groups, disclose all added participants beside this consent before enabling confirmation. BCC recipients and unrecognized email senders are not promoted to participants.

For different inboxes, show the resulting inbox and the history-sharing consequence: **History will be available to teammates with access to Support.** Treat this as an explicit transfer of access. Reuse the authority required to move conversations; do not infer transfer authority solely from the ability to read them.

The confirm button is **Merge into #1032**. Merge itself does not send an email. Success navigates to the primary and shows **Merged into #1032** with an **Unmerge** action. It opens the normal unmerge operation and its current checks; it is not a client-only rollback or a time-limited recovery mechanism.

### Thread and history

The primary thread gets one left-aligned merge entry per operation:

```text
Alex merged a conversation                                    10:42 AM
┌─────────────────────────────────────────────────────────────────────┐
│ #1048 · Cannot sign in                                           ··· │
│ “I tried again, but the new link has expired too.”                   │
│ Sam Taylor · Latest message                       View conversation │
└─────────────────────────────────────────────────────────────────────┘
```

Use the actual latest-message excerpt with its author and provenance; do not introduce an automatic AI-summary dependency. Group bulk sources under one event and progressively disclose a long source list. **View conversation** opens a readable original-thread preview over the inbox, without replacing the existing Details rail or creating a second narrow transcript column. Source history loads on demand with original timestamps, authors, notes and attachments. Search hits deep-link to the matching original message.

The source entry's menu and original-thread preview offer **Unmerge…**. Existing **Other conversations** rows mark sources as **Merged** and open the same preview; the conversation menu also provides access to merged originals. Do not add a duplicate details-rail section just for this action. The original preview offers **Continue conversation** to return to the active primary.

The source remains accessible by its original URL and shows:

```text
Merged into #1032 · Cannot sign in          Open conversation
```

Replace its composer with this continuation action. Do not silently submit a teammate's stale source draft to the primary. Keep the draft and attachments recoverable, with **Continue draft in #1032** only after rechecking access and showing the primary's recipients. Never overwrite an existing primary draft.

A source's edit/assign/resolve/AI controls become unavailable while merged. Personal read markers can still change. Internal merge/unmerge events remain auditable and cannot be edited like ordinary notes. Historic events stay compact; current follow-up status retains its existing single-current-stage behavior.

### Widget and email experience

For an authorized customer, an old widget conversation shows **This conversation continues in another chat** and **Continue conversation**. Preserve any unsent text or uploads while changing the selected conversation. The primary need not have originated in chat: an authorized source widget can remain a valid reply surface for the merged conversation.

The server returns whether continuation is actually allowed. If it is unavailable, explain the real route, such as **We'll continue this conversation by email**, only when email delivery is configured and confirmed. Otherwise retain a working route to the support team rather than a dead button. Older SDKs continue through their existing source session; the server resolves handling without allowing the old source to restart its AI.

Customers see only public messages authorized for them and their delivery channel. A verified same-customer match can unify that person's history. A typed email address, untrusted browser identity, or matching company/domain cannot grant another session access to past messages. For a cross-customer merge, each participant retains their own earlier history and can receive shared future replies; they do not automatically see each other's earlier conversations.

Keep outbound **To**, **CC**, and **Chat / Email** choices visible in the existing composer. Default the reply to the latest accepted customer message's route; do not overwrite someone's draft recipients when another message arrives. A participant belongs to a conversation, but a reply's explicit delivery audience still determines who receives that reply. Do not fan out a chat-only reply by email, reveal BCC, or include another customer's historical transcript in quoted email content.

### Unmerge

Action label: **Unmerge…**. Dialog title: **Unmerge conversation?**.

```text
Unmerge conversation?                                       ×

Cannot sign in
#1048 · Sam Taylor

Restore to                         Support · Alex · Open

Replies sent while merged stay in both histories.
AI stays paused.

                                           Cancel     Unmerge
```

Default to the original inbox, valid assignment, and pre-merge status. If the inbox no longer exists or the assignee no longer has access, show an inline correction before submitting. Unassigned is a valid fallback and must be shown. Restoring a resolved source does not invent a new resolution event. Do not re-run round robin merely because the merge was undone.

The shared history is a bounded reference to the messages from the merged period, labelled **Shared while merged with #1032**. Keep one message record and its actual delivery receipts. No copies are resent and no messages are heuristically reassigned. A cross-customer participant can retain access to messages already shared with them, not messages they were never entitled to see. Staff history access still follows the applicable inbox permissions.

Future source-addressed email and source widget replies return to the restored conversation. A reply to a shared-period email follows that email's preserved route; the preview explains which thread that route continues. Unmerge must not reinterpret an already sent reply address or manufacture recipients.

Shared-history access ends at a committed message boundary, not a timestamp-only approximation or a live link to all future primary messages. New replies after that boundary belong to their resulting conversation. Participants added solely by the merge lose access to future primary activity; any independent participant membership remains. Messages already seen or emailed cannot be recalled.

No support request or arbitrary undo window is needed while the original records exist and the teammate has access and merge permission. Previously deleted or anonymized content cannot be reconstructed. Unmerge does not restart canceled AI turns, follow-ups, pending emails, or CSAT surveys. A fresh customer reply follows the restored conversation's current, paused AI state.

## Capability and behavior contract

| Area | Proposed behavior |
| --- | --- |
| Eligible sources | Open, waiting, and resolved support conversations in the same workspace; reject self-merge, deleted/anonymized records, and spam until restored |
| Identity | Prefer established contact identity; retain verified identity provenance. Unlinked records with an exact accepted email can be suggested, but suggestion is not customer-access authorization. Anonymous matching is limited to a valid session/visitor lineage |
| Customer scope | Same customer by default; workspace option enables different customers. Contact/company records remain separate |
| Channels | Widget and email, including mixed merges; internal/API-created support records qualify only when their actual recipient and delivery capabilities are known |
| Multi-participant conversations | Supported through an explicit deduplicated participant preview and delivery-audience rules; do not equate all observed email addresses with participants |
| Unsupported channels | No promise of phone, social, Slack, or separate ticket-type merging until Helpin has the matching channel contracts. Existing channel names do not prove transport capability |
| Bulk operation | One transaction for a frozen selection of 2–50 conversations including the primary; enforce request and descendant-group bounds server-side. No partial-success merge through the current per-item bulk helper |
| Chained merges | Preserve direct parent relationships and their audit history. If A → B → C, restoring B from C keeps A merged into B; restoring A detaches only A. Preview all affected descendants |
| Primary metadata | Keep subject, customer, inbox, assignee, priority, and selected tags. Show source tags, associations, and linked tasks in source history; do not overwrite or silently union them |
| Status and attention | Preserve primary status unless there is outstanding customer work, in which case preview reopening it. Keep the earliest still-unanswered work; do not restart response clocks or count a merge as a reply |
| Sources | Separate merged relationship, not a fake human/AI resolution. Hide from operational queues by default; retain in explicit historical search and merge filters |
| Reporting | Preserve original inbound volume and attribution; count one active handling thread. No extra resolution, AI success, CSAT request, reply, or billing event from the merge itself |
| AI | One current handler. Source jobs lose publication rights; the primary rechecks current context. Human-request and takeover constraints cannot be bypassed by choosing an AI-owned primary |
| Realtime | Refresh affected list rows, counts, details, participants, messages, presence, and drafts for all authorized clients; no customer payload contains internal notes or source-private metadata |
| Deletion | Normal deletion of a still-merged group requires unmerging first. Retention/customer-erasure paths must be graph-aware and may erase data without unmerge; never restore erased content from snapshots |

The request limit is an operational bound, not a total-history cap. Load-test large descendants and bounded transaction sizes before deciding the aggregate group limit. If a proposed operation exceeds the measured safe bound, return the required smaller selection before making any changes.

## Permissions and settings

Add a distinct `support.merge` capability to the existing role/module authorization system. Default it to roles that currently have support editing; viewers remain read-only. Both merge and unmerge require this capability, support editing, and current access to every affected record. Cross-inbox transfer additionally follows the existing move authority. Expose the capability wherever Helpin already presents role permissions; do not create a separate permissions product for this feature.

Place **Allow merging conversations from different customers** under the existing **Settings → Inboxes & Routing → Routing & Assignment** page, in a small **Conversation merging** settings section. Default off. Only existing support/settings administrators can change it. Supporting text: **Teammates must review participants before merging.** The review itself carries the visible sharing consequence. Turning the option off blocks new cross-customer merges but does not destroy existing merges or prevent unmerge.

Every preview and commit rechecks workspace, capability, inbox access, customer scope, identity state, and recipients. Search and source-history APIs use the same authorization. A helper that falls back to owner access when actor context is absent must not be used as the public merge authorization boundary.

## Current Helpin code and required integration work

This is a capability audit, not a claim that these changes are implemented.

| Existing capability / evidence | Required merge integration |
| --- | --- |
| [Conversation model](../../server/internal/model/support_inbox.go): one customer/contact, email CC and observed participants, one anonymous ID, status, assignment, attention projections, AI control versions | Add merge relationship and revision, normalize authorized participants, retain original identity and delivery provenance |
| [Conversation actions](../../frontend/src/components/support/ConversationActionsMenu.tsx), [bulk toolbar](../../frontend/src/components/support/ConversationBulkToolbar.tsx), [other conversations](../../frontend/src/components/support/SidebarVisitorContext.tsx) | Shared picker/review/history/unmerge components; preserve existing menu, selection, mobile, and keyboard behavior |
| [Bulk action helper](../../frontend/src/lib/supportBulkActions.ts) executes per-item requests | Merge uses a new atomic endpoint, not this helper's partial-success loop |
| [Message thread](../../frontend/src/components/support/MessageThread.tsx), [composer](../../frontend/src/components/support/ReplyComposer.tsx) | Compact events, source read-only state, recoverable drafts, explicit effective audience, bounded shared history |
| [Conversation repository](../../server/internal/repository/support_inbox.go) handles scoped lists, search, unread projections, deletion, and original IDs | Reuse mailbox scoping; add merge-aware projections and search; guard writes and deletion; preserve original message IDs and links |
| [Visitor context](../../server/internal/service/support_inbox_visitor.go) loads owner-scoped contact/anonymous conversation history | Do not reuse this unscoped path for candidates. Scope this surface before adding merge entry points; assess count/title disclosure as well as message access |
| [Mailbox access/move service](../../server/internal/service/support_mailbox.go) | Require actor and source/destination authority; make intentional cross-inbox history transfer explicit |
| [Widget service](../../server/internal/service/support_inbox_widget.go), [widget identity](../../server/internal/service/support_widget_identity.go) | Replace single-anonymous-ID assumptions with explicit authorized continuation; preserve signed vs browser-claimed trust and staged uploads |
| [Widget HTTP](../../server/internal/handler/support_inbox_widget.go), [widget WebSocket](../../server/internal/websocket/widget_handler.go), [public projection](../../server/internal/model/support_widget_public.go) | Apply participant/message visibility on reads, sends, transcripts, read receipts, typing, reconnects, and subscriptions; keep legacy clients functional |
| [Widget conversation UI](../../packages/widget-core/src/components/ConversationView.tsx), [SDK](../../packages/sdk-js/src/core/widget.ts), [legacy widget](../../widget/src/helpin-widget.js) | Continuation state, draft preservation, merge-aware conversation list and realtime handling; verify all maintained clients, not just app preview |
| [Inbound/outbound email](../../server/internal/service/email_fallback.go), [inbound jobs](../../server/internal/service/email_inbound_jobs.go) | Preserve aliases and RFC reference lookup, validate against the original sender route, then resolve the active primary atomically before message/automation commit |
| [Delivery outbox](../../server/internal/service/support_delivery_outbox.go), [pending staff sends](../../server/internal/service/support_pending_send.go) | Fence recipients and routing revisions at final publication; do not redirect queued staff sends without review or resend completed deliveries |
| [Attachments](../../server/internal/service/support_attachment_content.go) enforce original conversation/message ownership | Keep storage ownership and IDs; authorize through a permitted history membership without weakening cross-workspace or customer checks |
| [AI control](../../server/internal/repository/support_ai_control.go), [AI reply commit](../../server/internal/repository/ai_message_processing.go), [follow-ups](../../server/internal/service/support_ai_follow_up.go) | Add merge fencing at admission and final commit; durable stop/reconciliation; fresh primary context; prevent resurrected secondary jobs |
| [Agent support tools](../../server/internal/service/internal_command_support.go) and [runtime ownership](../support-agent-runtime.md) | Reads understand authorized merged history; all writes respect source read-only state. Do not give autonomous support AI a new automatic merge capability |
| [Operational events](../../server/internal/model/support_events.go), [personal state](../../server/internal/repository/support_inbox_personal_state.go), [saved views](../../server/internal/repository/support_inbox_view.go) | Dedicated merged/unmerged events, correct unread/queue projections, merge-aware filters and consumers; no synthetic resolution |
| [Routes](../../server/internal/router/router.go), [RBAC](../../server/internal/authorization/rbac.go), [frontend types](../../frontend/src/lib/types.ts) | New permission and API contracts; legacy `/tickets` mutations must obey the same policy as `/inbox/conversations` |

The inspected support model has no conversation merge ledger or general customer participant access model. Email CC arrays are not a substitute for that model. No first-class SLA policy/clock was found in the inspected support files; do not invent SLA configuration in this feature. Preserve actual message and response timestamps so future SLA behavior can build on them.

## Data and API design

### Durable records

Use additive schema changes and an idempotent versioned migration. Existing conversations remain standalone by default.

1. Conversation fields record the active direct merge parent and a revision. The existing status remains independent of whether a record is merged.
2. A merge operation ledger records actor, workspace, explicit sources, primary, preview consequences, operation ID, and the relationship to any later unmerge. Audit operations are append-only.
3. Membership intervals record where each original conversation was handled and committed message boundaries. Chained merges preserve their direct edges; a cached active root is only a projection.
4. Participant grants identify the actual principal, original source, channels, and allowed history interval. Merge-derived grants can end without revoking independently granted access. Email addresses observed in headers remain distinct from authorized recipients.
5. New messages retain immutable origin conversation/route, active handling conversation, merge operation/epoch, and the explicit delivery audience. Keep original attachments, logs, receipts, translations, and external message IDs attached to the original message.

Shared history references message IDs and memberships, not copied content. Use committed sequence boundaries or explicit membership associations; client timestamps cannot safely define the unmerge boundary under concurrency. Customer erasure and soft deletion must also apply through every history view, search snippet, summary, and attachment route.

Each source-to-parent relationship has its own merge ID; a bulk operation groups these under one operation ID. Unmerging one relationship leaves the other members of that batch intact. History references are scoped to a membership interval, including when a source is merged, unmerged, and merged again.

During a merge, an explicitly approved transfer grants the primary inbox access to the source history. On unmerge, that active transfer ends. Reading a separate conversation's retained history then requires access to that original conversation as well as the current history container; otherwise show a restricted-history placeholder without snippets or participant details. The primary's own shared-period messages remain in the primary. Customer audience grants are checked separately from staff inbox access and never derive from an internal history link.

### One service for UI and integrations

Proposed authenticated API beneath the existing workspace support inbox prefix:

| Endpoint | Purpose |
| --- | --- |
| `GET /conversations/{id}/merge-candidates` | Paginated, scoped search with eligibility reasons for records the actor can already see |
| `POST /conversations/{id}/merge-preview` | Preview merging explicit source IDs into this primary, including descendants, participants, access changes, status and automation consequences |
| `POST /conversations/{id}/merge` | Commit the reviewed operation with an idempotency key and server-issued preview token |
| `GET /conversations/{id}/merges` | Current sources and reversible operations, with authorization-filtered provenance |
| `GET /conversations/{id}/history` | Cursor-paginated original/shared history subject to actor and audience checks |
| `POST /conversation-merges/{mergeId}/unmerge-preview` | Describe exact restored subtree, routes, assignment, bounded shared history, and corrections needed |
| `POST /conversation-merges/{mergeId}/unmerge` | Commit the reviewed separation with an idempotency key |

The preview token binds actor, workspace, selected records, meaningful revisions, and consequences. It is not an authorization credential. Revalidate all permissions on commit. A changed audience, access rule, merge graph, or required status correction produces a fresh preview; ordinary new messages can be included if they do not change the reviewed consequences. Never accept client-provided history grants.

Return the primary/restored conversation and affected IDs for cache refresh. Mutation failures have stable codes and short corrective messages. Missing and inaccessible IDs are indistinguishable. Mutating a merged source returns a conflict and the primary ID only when the caller may access it; it must not silently reply in another conversation.

### Transaction and background work

Serialize graph mutations within a workspace, then lock affected rows in a consistent order. Re-read roots and descendants, reject cycles, revalidate preview consequences, and atomically persist the relationship, participant/history grants, audit events, state revisions, and durable follow-up/outbound cancellation or reconciliation work. No external calls inside the transaction.

Source inbound email, widget sends, teammate sends, AI replies, merges, and unmerges must share the same routing/ownership check at their final database commit. A lookup before a transaction is insufficient. Source-origin inbound messages are authenticated against their original route before resolving the primary; blindly substituting the primary customer would mishandle known source participants and unknown-sender protections.

Increment AI control/routing revisions so stale source or primary runs cannot publish with old context. Cancel external runs after commit using durable retryable work. If cancellation fails, the database publication guard still prevents output. The primary may resume processing genuinely unanswered work with fresh merged context; merging alone must not generate another greeting or resend an answer. Retain human-request/takeover constraints until a teammate explicitly returns the conversation to AI.

For cross-customer conversations, scope AI context and tool reads to the intended reply audience. Joining conversations for staff review does not authorize an agent to disclose another participant's earlier private discussion. If answering requires that private context, request a teammate's review. A prompt reminder alone is not an access boundary; enforce the history scope in retrieval and tool responses.

Do not silently discard queued human replies. Preview them; hold merge when a send is already being delivered, and offer cancellation/review for cancelable sends. Stop source scheduled AI reminders and closures. Revalidate primary reminders against the resulting unanswered state without restarting their clocks merely because history was linked.

An external email provider cannot participate in the database transaction. If a send already crossed the provider boundary, record its actual outcome and audience. Do not promise to recall it or retry blindly. A persistent send claim and routing revision must prevent a second delivery.

Publish staff/widget events only after commit, with durable retry/reconciliation and audience filtering. Stale connected clients must lose access to future messages when their merge grant ends, without waiting for a browser reload.

Add merge-state fields to the shared filter definitions and backend query rules used by the relevant lists. Avoid a separate filter language. Search results identify both the original match and the current handling conversation, with authorization checked on each. Merge activity alone must not mark customer messages read or reset customer wait time.

## Concise states and copy

| Situation | Text / action |
| --- | --- |
| No eligible target | **No conversations to merge into.** |
| Search has no matches | **No matching conversations.** |
| Cross-customer option off | **Different customer. Ask an admin to enable this.** |
| Already merged source | **Already merged into #1032.** + authorized open action |
| Stale review | **This conversation changed. Review the update.** |
| Teammate send underway | **A reply is sending. Try again when it finishes.** |
| Missing access | **You no longer have access to a selected conversation.** |
| Bulk limit | **Select up to 50 conversations to merge.** |
| Unmerge complete | **Conversation restored.** + open action |
| Original inbox unavailable | **Choose an inbox for this conversation.** |
| Source was erased | **This conversation was deleted and cannot be restored.** |

Tooltips explain primary metadata retention, preserved history, and permissions. They do not hide who will receive messages or who gains access. Dates use the user's timezone and omit the year when it is the current year. Resolve accessible labels, focus return, scroll containment, long subjects, and keyboard navigation using existing dialog and result primitives.

## Delivery checklist and release gates

These are implementation steps in this same document, not separate plans. Each stage includes meaningful regression tests. The complete routing and visibility contract must ship together; a working merge button alone is not a releasable feature.

- [x] Read the merge article, all six screenshots, API reference, participant, permission, threading, and history documentation.
- [x] Inspect Helpin entry points, identity/access, routing, AI, attachments, queues, and event models.
- [x] Confirm customer scope and unmerge history policy with the product owner.
- [x] Review and approve the proposed design; reuse real conversation message components.
- [x] Add models, migration, permission, transaction service, ledger, preview and commit contracts, and graph/concurrency tests.
- [x] Implement recipient grants, history intervals, routed inbound messages, outbound fencing, source write guards, attachments, and erasure behavior.
- [x] Integrate AI admission/publication, follow-up stops, primary context refresh, unread projections, reports/events, and every maintained widget transport.
- [x] Build the shared picker, transcript comparison, bulk action, contextual merge entry and original-thread preview, source continuation, unmerge, and administrator setting using Helpin's existing components.
- [x] Verify end to end in Community and cloud builds, then document actual delivered behavior here and in the existing maintained guides.

Required behavioral verification:

| Area | Release evidence |
| --- | --- |
| Ordinary flow | Same customer, email/email, chat/chat, and both mixed directions; target selection and all original URLs work |
| Recovery | Immediate undo, later unmerge after replies, individual bulk-source unmerge, A → B → C subtree behavior, unmerge/remerge without duplicated or widened shared-history ranges |
| Isolation | Cross-workspace attempts, source/target access loss, unauthorized search/counts, signed vs untrusted widget identity, private inbox transfer, removed participants, cross-customer notes/history/attachments/transcripts, HTTP and WebSocket parity |
| Concurrency | Simultaneous opposing merges, changed preview, webhook during merge/unmerge, staff reply during merge, queued translations, AI reply race, follow-up race, repeated idempotency keys, process restart after commit |
| Email | Old aliases, RFC references, forwarded mail, CC/BCC, unknown senders, provider retries, delivery/open/bounce callbacks, blocked recipients and opt-outs, actual To/CC and reply routing after separation |
| AI | Source cannot publish, stale primary context cannot publish, one current handler, human requests survive, unmerge does not restart old automation, no duplicate metering or greeting |
| Data | Attachments and quoted content still render, deletion/anonymization removes all references, no lost or duplicated message IDs, historical source metadata and tasks remain accessible with correct permissions |
| Inbox | All saved/default/team views, unread counts, search, Other Conversations, customer profile history, assignment workload, and realtime invalidation agree |
| Reports | Merge does not count as solved, AI-resolved, CSAT-triggered, or a new inbound message; historical volume remains traceable |
| UX | Keyboard-only, screen reader labels, focus restoration, long content, large history, narrow screens, light/dark themes, empty/loading/error states, unsent draft and attachment preservation |
| Editions | Shared backend tests with and without EE; frontend behavior in Community and cloud; no hosted-only dependency in core merge behavior |

PostgreSQL integration tests are required for transaction, locking, idempotency, and message-boundary guarantees. UI component tests alone cannot establish them. Use synthetic customers and a controlled email/widget environment; do not merge real customer conversations to verify the feature without a separately authorized operation.

### Deployment and operational recovery

Deploy additive schema and merge-aware reads, writes, workers, and visibility checks before enabling creation of merge records. Do not enable the UI while an older worker can still publish on a merged source. Verify the complete deployment generation, then enable the entry points.

A rollback may disable new merges but must keep interpreting existing relationships, enforcing grants, and routing replies. Rolling back to an application version that ignores persisted merges is not a safe recovery procedure. Keep migrations forward-compatible and never drop merge history to roll back a UI issue.

Record operation IDs, affected conversation IDs, actor IDs, conflicts, idempotent retries, publication rejections, and cancellation/reconciliation failures in structured logs without message bodies or credentials. Measure merge/unmerge latency and monitor for active source AI jobs, unresolved post-commit work, invalid graph relationships, and failed message delivery. Repair work must use the same durable operation and audit contracts rather than direct message reassignment.

## Research and verification limits

Public Intercom documentation and its screenshots were reviewed; no authenticated Intercom workspace was inspected. The screenshots do not verify undocumented picker behavior. Helpin was audited from the current worktree, not by changing live conversations. SLA configuration and native phone/social/Slack support were not established in the inspected support paths and are not promised by this design.

Unmerge can restore independent handling and end future sharing. It cannot make a customer forget a message, recall delivered email, resurrect erased records, or undo third-party actions already taken during the merged period. Those limits should be stated where relevant, without burdening ordinary same-customer merges with unrelated warnings.

## Implementation sequence

Execute in this worktree with the executing-plans and test-driven-development workflows. Review each stage directly; no subagents.

1. **Durable merge contract** — add `model/support_merge.go`, `repository/support_merge.go`, and idempotent SQL migration; register models in `internal/dbschema`. Start with graph, customer identity, stale preview, atomic rollback, and unmerge/remerge tests. Store immutable original IDs and explicit per-message shared-history references, avoiding timestamp boundaries.
2. **Service and API** — add `service/support_merge.go` and `handler/support_merge.go`, `support.merge` RBAC, router endpoints, scoped candidates, signed-in actor validation, administrator setting, preview fingerprint and idempotency. Verify missing/access-denied equivalence and cross-customer opt-in.
3. **Routing and history** — integrate the existing message repository, widget and email services, pending sends, delivery outbox, AI publication and follow-ups, attachment authorization, source write/deletion guards, queues and visitor context. Exercise old routes, shared-history bounds, pause behavior, and cross-customer privacy.
4. **Native UI** — build `ConversationMergeDialog`, reusable readonly transcript from existing `TranslatedMessageBubble`, source history and unmerge dialogs; wire header/row/other-conversation/bulk entry points, continuation state and settings. Keep supplementary details in tooltips, essential audience changes visible, native responsive styles.
5. **Verification** — focused Go repository/service/handler tests, frontend interaction/type/build checks and real rendered review in light/dark/mobile; review all writes, maintained widget paths, and Community/cloud compatibility. Update this document with delivered behavior and remaining limits before committing.

Primary checks: `go test ./internal/repository ./internal/service ./internal/handler ./internal/authorization`, relevant PostgreSQL migration/integration tests where configured, `pnpm --dir frontend build`, and edition checks as appropriate. Passing SQLite tests alone does not prove PostgreSQL locking behavior.

## Implemented behavior and review evidence

The implementation stays in the existing `waqar-fixes` worktree. It has not
been deployed. The merge review uses the same `TranslatedMessageBubble`,
attachment renderer, avatars, receipts, and message grouping as the live thread.
A read-only transcript adds pagination without mounting a composer or changing
read state. The selected destination is explicit; a single-source review can
swap direction. Bulk merging supports up to 50 conversations including existing
merged descendants.

Entry points are conversation actions, selected-conversation bulk actions, and
Other Conversations. The timeline groups merge operations and provides original
history and unmerge actions. A merged original remains reachable by its URL,
with an authorized link to the active conversation. Missing or inaccessible
restoration inboxes require choosing an accessible replacement. Cross-customer
merges are off by default and have a support-administrator setting.

The persistence contract uses direct merge edges, immutable original message
IDs, explicit per-message shared-history grants, and an idempotency key. Shared
history remains labelled after unmerge. Metadata, customer ownership, original
inboxes, and human assignment are not copied into the destination. AI and
follow-ups pause; queued sends prevent a merge; stale drafts require a fresh
recipient review. The original draft text and attachments remain saved.

Email routing is tested in all four source/destination channel combinations
with a fake provider: actual To/CC/BCC, old aliases, new aliases, unmerge, and
former-participant rejection. There are no live customer sends in verification.
PostgreSQL tests use a disposable local database to exercise concurrent opposing
merges, inbound writes versus unmerge, audit events, inbox contributions, and
the existing contact-erasure flow. Browser verification uses the actual app
with synthetic API data, including native notes/messages, direction swapping,
merge/unmerge, dark mode, and a 390px viewport.

Operational details: inbox lists and materialized/legacy scope counts exclude
active sources; personal unread reminders carry into the destination while
original cursors remain intact. Merge/unmerge write their own support audit
event types and do not invoke resolution or CSAT actions. Runtime cancellation
and visitor refreshes have a persisted retry queue. Already delivered email
cannot be recalled; unmerge ends future sharing only.

The backend serializes routing commits per workspace; long provider requests
can briefly delay a routing change. Retained-history permission checks use
current inbox access, and deleted content is filtered at read time. This local
verification does not replace post-deployment smoke checks with test customers.

### Verification results (2026-10-11)

- Community and cloud frontend production builds passed; widget-core and SDK-JS builds passed.
- Support-focused Go regression tests passed in Community and `ee` editions (service, repository, model, handler, websocket, authorization packages).
- API, Temporal worker, and migrator built in both editions.
- PostgreSQL merge concurrency/counter/audit tests and existing contact-anonymization integration tests passed against a disposable local database.
- Four frontend helper/draft tests passed.
- Three compiled-app Playwright checks passed: merge/swap/unmerge using real message components, dark mode, and mobile. Earlier development-server attempts timed out during cold loading; the compiled app completed all three in 17.9 seconds.
- Documentation naming/link checks and diff whitespace checks passed.

These results cover the specified synthetic scenarios; they do not claim a live
Postmark, live widget embed, load benchmark, or production deployment test.
