# Two-way Slack integration for PM and Support

**Product direction · November 20, 2026.** This document records the intended first release of `@Helpin`. Customers can get support in a Slack Connect thread, teammates can create and follow PM work from Slack, and support owners can bring an engineering outcome back to the customer. The sections below describe the product contract. Implementation coverage and remaining release checks are recorded at the end.

## Installation and intake

A workspace admin connects the Helpin Slack app once and configures each participating channel. The admin can see the Helpin destination and whether new customer threads automatically open conversations.

| Channel purpose | When work starts | Where it goes |
| --- | --- | --- |
| Dedicated customer support, including Slack Connect | A customer starts a top-level thread. Further replies and files join that conversation. | The configured Support inbox |
| Mixed customer and account discussion | A person explicitly asks `@Helpin` for support or chooses **Create support request** on a message. | The configured Support inbox |
| Internal project work | A teammate chooses **Create task** or **Link existing task**, or asks the bot to do so. | A PM task in a team the teammate can access |

In a Slack Connect channel configured for customer support or support on request, a settings manager with CRM access can select an optional default company. A new inbox conversation inherits that company. Agents can change it in Company details. Editing the channel default applies to future conversations and leaves the sender’s contact record alone.

Start with channels and threads. Bot DMs as a separate support intake can follow later. A private response to the person invoking a command in a channel is part of the first release.

## Creating a task

For “@Helpin create a bug from this thread,” acknowledge the request promptly and open a private review. Use Helpin’s existing AI task flow. When matching is enabled and finds accessible related work, offer **Link existing task** and **Create new task**. Otherwise open the editable draft. Save after the teammate confirms.

| Field | How the form chooses it |
| --- | --- |
| Team | For PM, prefill the channel’s mapped team. Without a mapping, select the sole eligible team or ask the creator to choose. For a Support escalation, follow the inbox’s existing linked-team and preference rules. Enforce the creator’s permissions. |
| Epic | Prefill an optional channel epic. Otherwise leave it empty unless explicitly requested and unambiguously matched. Let the creator search, change, or clear it. Revalidate it when the team changes. |
| Title and description | Draft from the source discussion. Include reproduction steps when the discussion supplies them. The manual form can start from the selected Slack message. |
| Type, status, and priority | Start with the team’s task type and workflow defaults. The AI draft may suggest a type and priority. Show the values before save. |
| Assignees, sprint, and labels | Keep optional fields under **More options**. Existing task templates can supply defaults. A mention in the discussion carries no assignment authority. |

A support-linked Slack thread already has a Helpin conversation. Use its existing matching, draft review, and task-linking services. For a PM-only thread, adapt the shared drafting logic to use thread messages as evidence. Keep task suggestions and the review form private to the requester. [Support task form](../../frontend/src/components/support/SupportCreateTaskModal.tsx) · [Metered draft generator entry point](../../server/internal/service/support_task_preview.go) · [Support task linking](../../server/internal/service/support_conversation_tasks.go)

## Replies and engineering handoffs

| Where someone writes | Who receives it |
| --- | --- |
| Customer or teammate replies in the customer’s Slack thread | Participants see the reply in Slack. Helpin records it in the linked Support conversation with the correct author. |
| Support agent reviews **Reply on Slack to #channel** and sends from the inbox | The customer receives the reply and supported attachments in the original thread. **Internal note** stays within Helpin. |
| Teammate replies in the linked internal PM thread | The reply appears in a labeled Slack discussion on the task. A Helpin teammate can explicitly reply to that discussion. Ordinary task comments stay within PM. |
| Engineering changes the linked task | Refresh the internal Slack task card. Post completion in the internal thread and notify the support owner. The owner can review a customer-facing update and send it to the original support thread. |

A support-to-task link records the relationship. It gives the support owner a route to the engineering outcome while preserving the two audiences. A customer thread must never receive private task titles, links, comments, or notes through that relationship.

### Customer message source link

Use the inbound email footer’s position and type size for a single **Received from Slack ↗** link beneath each incoming customer message. Open the exact message in Slack in a new tab so the agent keeps their Helpin draft. For threaded replies and merged conversations, resolve the permalink using the inbound message’s own channel ID and timestamp. Slack checks the agent’s access to the source conversation. When Helpin has no source link, show the same attribution as plain text. [Slack message permalinks](https://docs.slack.dev/reference/methods/chat.getPermalink/)

## Asking for summaries and updates

- **“Summarize this conversation.”** For an authorized teammate, summarize the customer’s report, troubleshooting, commitments, owner, and next action from the Support record. Return the full summary privately by default. Allow sharing to an internal channel only when the audience is authorized.
- **“Give me an update on PAY-142.”** Resolve the task key, link, or current thread. Read status, owner, and due date from Helpin. Summarize recent progress, blockers, and the next step when recorded. Include an accessible source link. Ask which record the requester means when the reference is ambiguous.
- **Customer asks “Any update?”** In their own support thread, answer immediately from the latest published customer-visible information. If there is nothing new, say so and notify the support owner once. Repeated asks in the same thread share one owner notification until a new customer-facing update is published.

Use a fast LLM for natural-language requests, drafting, and summarization when enabled. Resolve records, check the requester and audience, and select permissible evidence before invoking the model. Apply existing AI usage checks. Manual task creation, factual published updates, message routing, and human replies must keep working when AI is disabled or usage is exhausted. [AI usage metering](../ai-usage-metering.md)

In a merged inbox conversation, the Send picker offers Chat for a widget session and Slack or Telegram for connected destinations in that conversation. When several channels can receive a reply, the teammate chooses one. Slack replies stay in the selected thread and Telegram replies in the selected DM. Only a “+ email” or “Email only” choice sends email. The closed picker shows the selected icons beside Send.

## Ask Agent in Slack

Opening Helpin’s Messages tab sends a one-time welcome. A teammate can DM Helpin directly or mention it in a channel configured for PM. Each person pairs once per installation. “What can you do?” shows examples for their current module access and the installation’s AI setting.

With Slack AI enabled, workspace questions such as “Which tasks are blocked on my team?” use the in-app Ask Agent and its model selection and usage checks. Follow up in the same Slack thread. Slack shows a working indicator during processing. Fast answers go straight to the thread. In a DM, an observed PM or Support read can create one activity message that becomes the final answer. Failed requests show a reason when available and a next step. Each member has a private conversation with PM and Support read tools. Task creation uses the review form. Answers in PM channels are visible to the requester. Customer threads continue through the support inbox.

For rollout, apply the updated manifest, choose **Reauthorize** in the connected Slack settings to grant DM history and agent permissions, and enable AI replies in Slack settings in Helpin. Verify the welcome, account pairing, a follow-up question, and private channel delivery in a test workspace.

## What to reuse from Telegram

Reuse the Support channel registry, inbound event deduplication, attachment jobs, realtime inbox updates, queued reply delivery, and delivery states including *failed* and *unconfirmed*. Telegram’s current implementation handles private chats, finds conversations by customer identity, and sends to a user ID. Slack needs a durable destination keyed by installation, channel, and root thread timestamp, with the sender recorded separately for each message. Configure routing per channel. Give Slack its own installation, request verification, interactive actions, and AI settings. Update provider-specific labels in the composer and merged conversations. [Support channel architecture](../support-channels.md) · [Inbound processing](../../server/internal/service/support_channel_inbound.go) · [Outbound delivery](../../server/internal/service/support_channel_outbox.go) · [Frontend channel registry](../../frontend/src/lib/supportChannels.ts)

## Release checks

- One customer starts two threads and a colleague joins one. Helpin shows two conversations, preserves authorship, and posts each reply to its intended thread.
- Repeated Slack events and double submissions produce one message or task. The bot’s own outbound posts do not echo back as new customer messages. Agents can distinguish failed delivery from unconfirmed delivery before retrying.
- A customer asks for an update after engineering adds a private note and completes the task. They receive only published information. The support owner can review and send a new update.
- A teammate without access to a task asks for its status in a shared channel. Neither the reply nor a preview discloses its contents. A permitted teammate can review an AI draft privately, change its fields, and create or link the work.
- With AI unavailable, a customer can still reach support, an agent can reply, and a teammate can create a task manually.

## Implementation and operator setup

Settings → Integration → External tools → Apps is the installation point for both modules. Support → Channels → Slack → Manage opens that same tab. The MCP servers tab is available with Agents access and permission to read settings. [External tools gallery](../../.mockups/external-tools/index.html) The Support inbox uses its existing conversation list, details, reply composer, note action, and delivery receipt. The PM task uses its existing threaded comments. While drafting, the recipient line shows the Slack channel in the same place as email recipients. The delivery picker shows Slack for connected Support threads and offers Slack + email when email is available. The adjacent button reads **Send**. A linked PM comment shows where its replies sync beneath the comment body. The browser screenshots follow one fictional checkout issue through the actual Helpin app, including the narrow-screen inbox list. [Helpin screenshot gallery](../../.mockups/slack/index.html)

### Create and activate the production Slack app

Release the Helpin implementation through `main`. The production app is named **Helpin** and its Slack handle is `@Helpin`.

1. In the Slack workspace that will own Helpin, register the app using the bootstrap manifest from `python3 integrations/slack/render_manifest.py --bootstrap`. The bootstrap manifest creates the bot before enabling callback verification. [Slack app settings](https://api.slack.com/apps)
2. Configure `SLACK_CLIENT_ID`, `SLACK_CLIENT_SECRET`, and `SLACK_SIGNING_SECRET` in the production secret source for the API (`helpin/helpin-secrets`). Set `APP_BASE_URL=https://app.helpin.ai`. Confirm the existing 32-byte hex `CHANNEL_ENCRYPTION_KEY` or its `CRM_ENCRYPTION_KEY` fallback is available. Apply the release migrations and reload the API after secret synchronization.
3. Apply the [production manifest](../../integrations/slack/manifest.yaml) to the same app. It sends OAuth redirects to `https://app.helpin.ai/oauth/slack/callback` and events and interactions to `https://api.helpin.ai/api/slack/events` and `https://api.helpin.ai/api/slack/actions`. Verify Slack accepts the Events URL and lists all three message shortcuts. [Slack manifest setup](https://docs.slack.dev/app-manifests/configuring-apps-with-app-manifests/)
4. In Helpin **Settings → Integration → External tools → Apps → Slack**, choose **Connect** and authorize the owning workspace. Invite `@Helpin` to the participating channels and assign each its purpose and destination. Each teammate links their Helpin account when first using a private or PM action.
5. Check a private PM task review and a Support conversation in Slack Connect. Verify files in both directions, an original-message link, and delivery of an approved reply to its thread. Enable Slack app distribution to install into additional workspaces. Reinstall after adding scopes.

The PostgreSQL and Slack API fixtures cover OAuth state replay, signed event intake, thread isolation, delivery to the original thread, reconnect, note privacy, merged destinations, team-specific task review, completion handoffs, and the audience check for sharing. [Integration checks](../../server/internal/service/slack_postgres_test.go)

### Coverage and remaining release checks

| Area | Current behavior or release check |
| --- | --- |
| Task review | Slack exposes related work, title, description, team, epic, type, and priority. The usual team workflow supplies initial status. After choosing a team, **More options** exposes status, assignees, sprint, labels, and template. Team-specific controls refresh when the creator changes teams. Each Slack selector currently offers up to 100 options. |
| AI and history | Support escalations reuse the inbox task-draft service. Scoped questions use the fast completion route and normal usage metering. Task answers fall back to record fields when AI is unavailable. PM-only drafting follows up to four pages of thread replies and retains the selected message if history is incomplete. The private review says when it has a partial thread, only the selected message, or no message text. A shortened manual description is also identified. |
| Sharing | Full Support summaries and task answers go privately to the linked requester. A teammate can explicitly ask `@Helpin share summary` with a Helpin conversation link or task key in a project channel. Detailed shared answers, PM cards, and linked replies require a private internal channel where every member can read the record. Public channels allow private answers and task creation. |
| Edits and retention | New messages and file shares sync. Slack Connect uploads resolve their filename and file type when downloaded. A signed channel ID change keeps Support and PM thread keys stable while delivery and source links use the current Slack ID. Slack message edits and deletions are outside this implementation. A Support draft checks the source conversation again before submission. PM-only review uses the captured thread excerpt. |
| Database setup | The Slack migration and its channel dependencies applied to a disposable PostgreSQL database. A separate fresh-instance bootstrap stopped in the earlier CRM migration `202604170001` on a missing `pm_tasks.owner_member_id` column. |
| Slack Connect pilot | Verify installation, file access in both directions, reconnect, and channel membership changes with one host-owned and one customer-owned Slack Connect channel. Automated checks use a local provider fixture. |

## Research behind the design

| Reference | Design lesson |
| --- | --- |
| [Linear for Slack](https://linear.app/docs/slack) and [Asana for Slack](https://help.asana.com/s/article/slack-and-asana) | Linear provides the PM model for a scoped discussion and completion feedback. Asana makes creating a task and attaching a message to existing work easy to discover. |
| [Pylon customer channels](https://docs.usepylon.com/pylon-docs/integrations/chat/slack) and [internal threads](https://docs.usepylon.com/pylon-docs/support-workflows/issues/internal-threads) | A strong B2B support model keeps the customer thread and internal discussion distinct while giving the support owner access to both. |
| [Intercom Slack channel](https://www.intercom.com/help/en/articles/11534476-connect-your-slack-channel) | Customers can start naturally, receive an agent’s reply in the original thread, and follow published ticket updates there. |
| [Zendesk side conversations](https://support.zendesk.com/hc/en-us/articles/4408844202778-Using-side-conversations-for-Slack) and [Jira Service Management intake](https://support.atlassian.com/jira-service-management-cloud/docs/create-issues-in-slack/) | Zendesk shows how to involve colleagues from a ticket. Jira illustrates channel-specific request intake. Zendesk’s documented shared-channel limitations also motivate testing Slack Connect explicitly. |
