# Two-way Slack integration for PM and Support

**Product direction · November 20, 2026.** This document records the intended first release of `@Helpin`. Customers can get support in a Slack Connect thread, teammates can create and follow PM work from Slack, and support owners can bring an engineering outcome back to the customer. The behavior below is planned.

## Installation and intake

A workspace admin connects the Helpin Slack app once and configures each participating channel. The admin can see the Helpin destination and whether new customer threads automatically open conversations.

| Channel purpose | When work starts | Where it goes |
| --- | --- | --- |
| Dedicated customer support, including Slack Connect | A customer starts a top-level thread. Further replies and files join that conversation. | The configured Support inbox |
| Mixed customer and account discussion | A person explicitly asks `@Helpin` for support or chooses **Create support conversation** on a message. | The configured Support inbox |
| Internal project work | A teammate chooses **Create task** or **Link existing task**, or asks the bot to do so. | A PM task in a team the teammate can access |

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
| Support agent chooses **Reply to #channel** in the inbox | The customer receives the reply and supported attachments in the original thread. **Internal note** stays within Helpin. |
| Teammate replies in the linked internal PM thread | The reply appears in a labeled Slack discussion on the task. A Helpin teammate can explicitly reply to that discussion. Ordinary task comments stay within PM. |
| Engineering changes the linked task | Refresh the internal Slack task card. Post completion in the internal thread and notify the support owner. The owner can review a customer-facing update and send it to the original support thread. |

A support-to-task link records the relationship. It gives the support owner a route to the engineering outcome while preserving the two audiences. A customer thread must never receive private task titles, links, comments, or notes through that relationship.

## Asking for summaries and updates

- **“Summarize this conversation.”** For an authorized teammate, summarize the customer’s report, troubleshooting, commitments, owner, and next action from the Support record. Return the full summary privately by default. Allow sharing to an internal channel only when the audience is authorized.
- **“Give me an update on PAY-142.”** Resolve the task key, link, or current thread. Read status, owner, and due date from Helpin. Summarize recent progress, blockers, and the next step when recorded. Include an accessible source link. Ask which record the requester means when the reference is ambiguous.
- **Customer asks “Any update?”** In their own support thread, answer immediately from the latest published customer-visible information. If there is nothing new, say so and notify the support owner once. Proposed deduplication rule: repeated asks in the same thread share one owner notification until a new customer-facing update is published.

Use a fast LLM for natural-language requests, drafting, and summarization when enabled. Resolve records, check the requester and audience, and select permissible evidence before invoking the model. Apply existing AI usage checks. Manual task creation, factual published updates, message routing, and human replies must keep working when AI is disabled or usage is exhausted. [AI usage metering](../ai-usage-metering.md)

## What to reuse from Telegram

Reuse the Support channel registry, inbound event deduplication, attachment jobs, realtime inbox updates, queued reply delivery, and delivery states including *failed* and *unconfirmed*. Telegram’s current implementation handles private chats, finds conversations by customer identity, and sends to a user ID. Slack needs a durable destination keyed by installation, channel, and root thread timestamp, with the sender recorded separately for each message. Configure routing per channel. Give Slack its own installation, request verification, interactive actions, and AI settings. Update provider-specific labels in the composer and merged conversations. [Support channel architecture](../support-channels.md) · [Inbound processing](../../server/internal/service/support_channel_inbound.go) · [Outbound delivery](../../server/internal/service/support_channel_outbox.go) · [Frontend channel registry](../../frontend/src/lib/supportChannels.ts)

## Release checks

- One customer starts two threads and a colleague joins one. Helpin shows two conversations, preserves authorship, and posts each reply to its intended thread.
- Repeated Slack events and double submissions produce one message or task. The bot’s own outbound posts do not echo back as new customer messages. Agents can distinguish failed delivery from unconfirmed delivery before retrying.
- A customer asks for an update after engineering adds a private note and completes the task. They receive only published information. The support owner can review and send a new update.
- A teammate without access to a task asks for its status in a shared channel. Neither the reply nor a preview discloses its contents. A permitted teammate can review an AI draft privately, change its fields, and create or link the work.
- With AI unavailable, a customer can still reach support, an agent can reply, and a teammate can create a task manually.

Before implementation, verify Slack Connect behavior in both host-owned and customer-owned channels, including file access and reconnecting an installation. Specify edit and deletion handling, the authorized audience check for shared summaries, and how a changed source thread affects an open draft. Keep those decisions here as they are resolved.

## Research behind the design

| Reference | Design lesson |
| --- | --- |
| [Linear for Slack](https://linear.app/docs/slack) and [Asana for Slack](https://help.asana.com/s/article/slack-and-asana) | Linear provides the PM model for a scoped discussion and completion feedback. Asana makes creating a task and attaching a message to existing work easy to discover. |
| [Pylon customer channels](https://docs.usepylon.com/pylon-docs/integrations/chat/slack) and [internal threads](https://docs.usepylon.com/pylon-docs/support-workflows/issues/internal-threads) | A strong B2B support model keeps the customer thread and internal discussion distinct while giving the support owner access to both. |
| [Intercom Slack channel](https://www.intercom.com/help/en/articles/11534476-connect-your-slack-channel) | Customers can start naturally, receive an agent’s reply in the original thread, and follow published ticket updates there. |
| [Zendesk side conversations](https://support.zendesk.com/hc/en-us/articles/4408844202778-Using-side-conversations-for-Slack) and [Jira Service Management intake](https://support.atlassian.com/jira-service-management-cloud/docs/create-issues-in-slack/) | Zendesk shows how to involve colleagues from a ticket. Jira illustrates channel-specific request intake. Zendesk’s documented shared-channel limitations also motivate testing Slack Connect explicitly. |
