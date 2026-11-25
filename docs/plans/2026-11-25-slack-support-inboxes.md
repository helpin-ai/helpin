# Support inboxes in Slack

This is the working checklist for the team improving **Handle Helpin inboxes in Slack** in Community and hosted Helpin. Use it to build a compact channel card, make the speaker clear in every thread reply, and let teammates triage conversations from Slack. Tick each item when its acceptance check passes and record its commit or verification result here. The original integration and its evidence follow the new work.

## Progress at the start of this revision

| Work | Verified branch state on 2026-11-25 |
| --- | --- |
| ✅ Actionable AI handoffs, code checks and Markdown rendering | `7c4193a71` is present on remote `waqar-fixes`, `develop` and `main`. |
| ✅ Scenario-specific first-message updates, direct conversation link, destination inbox names including team inboxes, bold routing details | Implemented locally on `waqar-fixes` in `9e9dc0a9f`, `4d22daf4c` and `3ab38d6f5`. Pending promotion. |
| ✅ Direct links to spam conversations select Spam in the Helpin sidebar | Implemented locally in `83e7cc4b8`. Pending promotion. |
| ☐ Channel card, thread identities, email presentation and action layout below | Planned. The current card allows 800 characters for the title and 500 for the excerpt. The current Slack **Manage** dialog offers assignee and status. |

## What teammates should see

One channel post represents one conversation. Its short, bold subject links directly to Helpin. Beneath it, show the customer or visitor name, inbox and current status. Surface **🔴 Urgent** and **🟠 High** on this line. Medium and Low are visible in the priority picker. Reserve emoji for status, sender or activity cues. The example uses fictional data.

**#284 · Export stuck at 99%**

Maya Patel · Support · **🙋 Needs human reply** · **🟠 High**

> My export has been running for 20 minutes.

**Reply to visitor**　 **Assign…**　 **Resolve**　 **⋯ More**

Only the excerpt is indented. Keep the internal-thread note short and visible near the actions so teammates know that typing in Slack creates an internal note. Indicate channel and current owner compactly when useful, with the assignee on the assignment control.

## Implementation checklist

Start by bringing the latest `develop` into the existing `waqar-fixes` worktree. Deliver the card, sender identity, email rendering and actions in that order, with a focused commit for each. Use the final checklist to verify them together.

### 1. Compact the channel card

- [x] Update `FeedPreview` and `refreshFeedCard` to select the opening public customer message and load the fields needed for its excerpt or attachment label. AI-handled conversations in a followed inbox get the same card. Collapse title whitespace, bound long or body-derived subjects, and use a short numbered title when the subject is missing.
- [x] Render the excerpt as a native Slack quote. Collapse whitespace, keep whole words and cap it at **100 visible characters including the ellipsis**. Add an ellipsis only for shortened text. Skip empty excerpts and those that repeat the subject. For attachment-only messages show e.g. **📎 Attachment: invoice.pdf**. Use an equally compact, readable notification fallback.
- [x] Keep the full customer message in its opening thread reply and subsequent messages in that thread. Honor Slack’s message limits for long content and retain the Helpin link for the complete message. Verify refreshes edit the same parent, attachments appear once and retries do not repeat the first reply. For a moved-in conversation, preserve existing history-sharing boundaries.
- [x] Verify a priority-only edit in **Helpin** updates the existing Slack card, including High → Medium. The current conversation trigger omits `priority`. Check whether the existing priority-change event covers every path. If direct updates can be missed, add an idempotent follow-up migration that includes `priority` in the trigger while preserving the other watched columns.

Start in [the feed renderer](../../server/internal/service/slack_inbox_feed.go), [feed queries](../../server/internal/repository/slack_feed.go) and [the current enqueue trigger](../../server/internal/dbmigrate/sql/202611250001_slack_inbox_feeds.sql).

### 2. Attribute each thread message

- [x] Derive the speaker from the **individual message**. Use sender type, user/agent identity and existing AI metadata for older records. Distinguish customers, teammates and AI even when the same display name appears twice. Reassigning a conversation must preserve attribution on earlier messages.
- [x] Display the sender inside the Slack message using its existing app identity. For people, use the saved avatar when Slack can display it, then one or two initials from their name, then a role icon for unnamed senders. Reuse Helpin’s initials rules. Where an initials image is unavailable, render the initials as text next to the name. Keep the role readable when images are missing. Omit avatars and initials from the channel card.
- [x] Use **Maya Patel · Visitor** for a named widget visitor, **👤 Visitor** for an anonymous visitor, and **Maya Patel · Customer** when a linked CRM contact has the Customer lifecycle stage. A name, email address or verified identity alone does not establish customer status. For other inbound email show **Maya Patel · ✉️ Email**. Use the actual message sender on multi-participant conversations.
- [x] Give public AI replies **🤖 Helpin AI** or the configured agent name with an explicit AI label. Give human replies **Sam Rivera · Team reply**, with 🎧 as the unnamed teammate fallback. Give internal notes the author’s same avatar or initials and a prominent **📝 Internal note** label. Native Slack replies retain their native identity and create one internal Helpin note. Use **🙋 Needs human reply** for a handoff and **🔎 Code check** for an actionable result. Keep system events visually distinct from replies.
- [x] Check the same sender and audience appear in Slack notifications and screen-reader text. Names and inbox labels containing Markdown punctuation or mention-like strings must render literally.

Start in [thread delivery](../../server/internal/service/slack_inbox_feed.go), [AI summaries](../../server/internal/service/slack_feed_ai.go), [message sender fields](../../server/internal/model/support_inbox.go) and [existing initials handling](../../server/internal/service/support_inbox_transcript.go).

### 3. Render HTML email for reading in Slack

- [x] Use Helpin’s email body hydration and visible-reply extraction for both the card and the thread, including older email records. Generate the 100-character card excerpt from the new reply. Exclude recognized quoted history and skip hidden email content in the preview.
- [x] In the thread, render the authored reply as Slack-formatted paragraphs, lists, bold and clickable links. Use the plain-text email part when rich content is unavailable. Retain attachments and a direct **Open in Helpin** link for original tables, images and email layout. Preserve normal plain-text messages and literal characters.
- [x] Verify with fictional HTML and plain-text emails, a quoted chain, an attachment-only message and a legacy stored email. Check the excerpt, thread, notification and direct link. Check that neither raw tags nor formatting delimiters appear as accidental text.

Reuse [email hydration](../../server/internal/service/support_inbox.go), [inbound email extraction](../../server/internal/email/inboundhtml/convert.go) and [email hydration fixtures](../../server/internal/service/support_inbox_email_hydration_test.go).

### 4. Replace Manage with direct actions and More

| Position | Control | Behavior |
| --- | --- | --- |
| First | **Reply to visitor**, **Reply to customer** or **Reply by email** | Match the label to identity and available delivery. Use **Reply…** when multiple destinations need a choice. Keep transport, To/CC, files and stale-draft handling in the reply modal. |
| Second | **Assign…** or **Assigned to Maya ▾** | Open a searchable picker with **Assign to me**, eligible Helpin teammates and **Unassigned**. Show the current owner. Save through Helpin’s existing assignment checks. |
| Third | **Resolve** or **Reopen** | Apply the existing status transition and refresh the parent. |
| Last | **⋯ More** | Offer **Set priority…**, **Move to inbox…**, **Snooze… / Wake from snooze**, **Change status…**, and **Pause AI / Resume AI** as applicable. |

- [x] Wire the buttons, overflow selection and modal submissions through the existing Slack callback and job flow. Keep old `feed_manage` buttons and in-flight forms usable as older cards refresh. Keep direct conversation links on active cards, spam cards and stopped-sharing notices.
- [x] Call the existing Helpin services on behalf of the linked teammate. Recheck inbox access, writable state and current values at submit time. Keep **Assign to me** from overwriting someone who has just claimed the conversation. Assignment and public reply retain Helpin’s human-takeover behavior.
- [x] In **Set priority…**, preselect the current value and offer **Urgent, High, Medium, Low**. For **Move to inbox…**, show eligible destinations and retain the existing channel-stop behavior. For **Snooze…**, use Helpin’s time choices and expose **Wake from snooze** when applicable. **Change status…** includes **Waiting on customer** and **Spam**. Once marked as spam, the direct link opens Helpin’s Spam view for restoration.
- [x] Offer AI controls only when the conversation supports them. Reuse the version check and the explicit confirmation when a person requested a human. Make clear that resuming releases the human assignment and allows AI to answer the next incoming message.
- [x] Exercise old/new cards, stale forms, revoked inbox access and duplicate callbacks. Preserve confirmed delivery retry on the failed thread reply and the existing guidance for unconfirmed delivery.

Use [Slack action dispatch](../../server/internal/service/slack_pm.go), [action handling](../../server/internal/service/slack_feed_actions.go), [forms](../../server/internal/service/slack_feed_forms.go), [inbox move rules](../../server/internal/service/support_mailbox.go), [priority and snooze](../../server/internal/service/support_conversation_controls.go), and [AI control](../../server/internal/service/support_ai_control.go). Match the [Helpin inbox menu](../../frontend/src/components/support/ConversationActionsMenu.tsx). The order above is a product choice. Usage frequency has not been measured.

### 5. Verify and promote

- [x] Use the existing [feed tests](../../server/internal/service/slack_inbox_feed_test.go) and [PostgreSQL integration suite](../../server/internal/service/slack_postgres_test.go). Cover AI-handled conversations, public replies, team notes, deduplication, attachments and merged/anonymized conversations. Handoffs and completed or failed actionable code checks reach the thread once. Routine AI notes, standalone pre-reply checks, in-progress checks and unsent drafts stay internal to Helpin.
- [x] Check stop notices for AI, routing-rule and teammate moves, unknown actor, merge and personal-data removal. Verify the destination name, including a team inbox name with formatting characters, bold key words, direct link and channel audience checks. Test moving out and back, and opening a spam link with Spam selected.
- [x] From `server/`, run `go test ./internal/email/inboundhtml ./internal/service -run 'Test(SlackInboxFeed|HydrateEmailBodies|SupportAIControl|Project)'`. Run `go test -tags integration ./internal/service -run '^TestSlackInboxFeed' -count=1 -v` and repeat with `-tags 'ee integration'`. Set `SUPPORT_OPS_TEST_DATABASE_URL` to a migrated disposable PostgreSQL database and confirm the database tests actually run. From the repository root run `pnpm --dir frontend exec vitest run src/components/support/__tests__/SupportInboxLayoutRouting.test.tsx`.
- [x] Slack’s Block Kit validator accepted 16 representative card, thread and modal payloads captured from the signed-callback integration tests with fictional conversations.
- [ ] Review the fictional examples in a designated Slack development workspace on desktop and mobile. Slack’s web preview prompted for workspace sign-in at both widths. Inspect quote wrapping, avatar size and the overflow menu in the native clients. The team confirmed that a dedicated Slack development channel is unavailable.
- [x] Published the verified implementation to `waqar-fixes` and `develop` at `0d4648ec9`. The staging release passed and its API returned healthy. [Staging release](https://github.com/helpin-ai/helpin/actions/runs/45682444705)
- [x] Promoted `develop` to `main` at `b27282d8e` after CI passed. Production published `server-v0.95.547`, deployed the frontend, passed its link-preview checks and returned a healthy API response. Active cards pick up the changes on their next eligible update. [Promotion PR](https://github.com/helpin-ai/helpin/pull/848) · [Production release](https://github.com/helpin-ai/helpin/actions/runs/45683909252)

## Revision verification

| Change | Evidence |
| --- | --- |
| Compact linked conversation card and priority refresh | `684845655`. The migration adds priority to the existing trigger. The PostgreSQL test verifies High appears on the same parent and disappears on a change to Medium. |
| Per-message role, avatar and initials | `03bef8dc0`. Service tests exercise team, AI, visitor, customer and email senders. Slack posts under the app identity and shows the speaker inside the reply. |
| Readable email | `b6f1374df`. The email and service tests cover latest replies, quoted history, hidden HTML, formatted content, plain-text fallbacks and attachments. |
| Direct assignment, status and More | `8298a1a13`. Signed callbacks exercise reassignment, stale priority, move, snooze and wake, status and AI pause and return. A human request is shown when confirming return to AI. |
| Spam navigation and move notices | `83e7cc4b8`, `9e9dc0a9f`, `4d22daf4c` and `3ab38d6f5`. The routing suite passed all 19 cases. |

The service suite ran against migrated disposable PostgreSQL for Community and hosted builds. Public and internal threads, callbacks and delivery retries used a simulated Slack API. Slack validated 16 real payload shapes from those fictional conversations through its nonposting validator.

## Original implementation and evidence

The sections below record the integration that already existed when this revision began.

## Existing behavior to preserve

Customer support and on-request rules route inbound Slack threads and files to one inbox and optionally a company. PM rules have team and epic defaults. Invited channels without a rule support explicit requests. Existing routes, Slack Connect intake, signed callbacks, identity pairing and durable provider delivery must keep working.

## Implementation and acceptance

- [x] Extend `slack_channel_routes` with an inbox-feed purpose and explicit inbox selection. Store stable conversation-to-thread bindings separately from customer transport. Save and validate selections atomically. Preserve legacy intake modes.
- [x] Extend `SlackService.AvailableChannels` and configuration to discover public channels and join only the selected channel on save. Keep private-channel invite and refresh, membership checks, reauthorization and clear setup errors.
- [x] In Slack rule settings group both intake behaviors, add a multi-inbox destination rule, show Slack Connect and public audience information, and link to the same editor from inbox settings. Prevent unsafe sharing before save and while delivering.
- [x] Queue committed Helpin changes transactionally. Create one parent per conversation per eligible destination. Update status, assignee and snooze in place. Send customer messages, team replies and meaningful lifecycle changes in the same thread. Stop stale delivery following moves, merges, spam, anonymization, loss of access or removal of the app.
- [x] Treat authorized ordinary thread replies as internal notes. Open a customer reply modal with explicit transport, recipient/CC controls, files and stale-draft handling. Route changes through existing Helpin services for authorization, recipient validation, translation, AI takeover, assignment and status. Handle duplicate callbacks, rate limits and uncertain delivery.
- [x] Persist Slack origin on affected Helpin messages and activity entries. Show a subtle linked “via Slack” label, plus an accessible link to the team thread in the conversation.
- [x] Verify positive and negative paths with focused service tests, PostgreSQL migration/worker integration checks and the actual frontend build. Capture the real rule editor, inbox shortcut and conversation timeline with fictional data. Record test and screenshot evidence here.

Shared implementation and synthetic review evidence belong with the existing support and Slack integration. Production credentials, customer messages and operations stay outside the change.

## Product decisions

One inbox has one handling channel per Slack installation. A channel can handle several inboxes. This gives each conversation a single staff discussion and a single destination for the Helpin thread link. New rules start with new conversations. Moving an existing conversation into a selected inbox also starts sharing it.

The main inbox can use an internal public channel. Its rule shows who can read messages, notes and files. Team inboxes use private channels with linked Helpin members who have inbox access. Slack Connect channels can receive customer requests. Helpin checks the current Slack audience before sharing each update.

Threads distinguish a teammate’s internal note from **Reply to customer**. Delivery failures with a confirmed channel rejection have a retry action. Unconfirmed delivery asks the teammate to check the destination. The staff timeline records the author and Slack origin. Previously shared messages remain in Slack history after disconnecting a rule.

## Review evidence

Both Community and hosted frontend production builds passed. All 124 tests in the message bubble and inbox dialog regression suites passed. A clean PostgreSQL installation passed the Community foundation and schema parity test, including the new migration.

The screenshots use the production Helpin frontend and fictional records. The browser exercised the controls, verified the inbox settings shortcut, and found the Slack origin links on a reply, note and assignment.

- [Channel rules](../../.mockups/slack-inbox-feed/channel-rules.png) and [Slack Connect intake](../../.mockups/slack-inbox-feed/customer-intake.png)
- [Selected inboxes](../../.mockups/slack-inbox-feed/inbox-rule.png), [inbox picker](../../.mockups/slack-inbox-feed/inbox-selection.png) and [public channel audience](../../.mockups/slack-inbox-feed/public-channel.png)
- [Shared inbox shortcut](../../.mockups/slack-inbox-feed/inboxes.png) and [team inbox settings](../../.mockups/slack-inbox-feed/inbox-settings.png)
- [Conversation timeline](../../.mockups/slack-inbox-feed/conversation.png) and [open Slack thread](../../.mockups/slack-inbox-feed/conversation-actions.png)

The final Slack service regression suites passed with both Community and hosted build tags on an isolated source snapshot, using the migrated PostgreSQL database. They exercised public-channel joining, Slack Connect exclusion, confirmed and unconfirmed delivery retries, note deduplication, customer-reply attribution, draft preservation, parent status updates, moving out of an inbox and back, and rule removal. The Slack file client tests also verified reconciliation of an interrupted upload.

Slack’s unauthenticated Block Kit validator accepted the original conversation card, reply form and management form. The provider integration tests used local PostgreSQL and a simulated Slack API. At that review, the remaining release steps were applying the database migration and Slack manifest, then verifying an installation in a Slack development workspace. Existing installs can invite Helpin manually or reauthorize for automatic public-channel joining.
