# Support inboxes in Slack

Implement the two-way team workflow in the shared Helpin Slack app. This plan tracks the implementation on `waqar-fixes` for Community and hosted workspaces.

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

Slack’s unauthenticated Block Kit validator accepted the generated conversation card, reply form and management form. The provider integration tests use local PostgreSQL and a simulated Slack API. Release still requires applying the database migration and Slack manifest, then verifying an installation in a Slack development workspace. Existing installs can invite Helpin manually or reauthorize for automatic public-channel joining.
