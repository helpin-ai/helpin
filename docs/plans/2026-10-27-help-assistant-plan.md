# Help assistant: a conversational agent for the help center and portal

Status: built on `feat/company-portal`. As built, a turn's run targets its pending answer (`help_assistant_turn`) rather than the session, and the conversation logic is shared by both apps in `packages/shared/src/help-assistant.ts`. See [Help assistant](../help-assistant.md).

Date: 2026-10-27. Follows [the portal intake plan](2026-10-27-portal-intake-deflection-plan.md), whose "help center and portal assistant" section this replaces, and [private help spaces](2026-10-27-portal-private-help-spaces-plan.md).

Customers who are reading the help center or the portal can ask questions in a conversation. Answers come from published articles and cite them. When an answer does not help, the customer can contact support with the conversation already attached. It is modelled on [Mintlify's assistant](https://www.mintlify.com/docs/ai/assistant).

## Decision: a new system agent on Agent Runtime

The intake plan kept Agent Runtime away from the help center because a run per keystroke costs too much. That argument applies to search as you type, which stays as it is. A conversation is different. The customer chooses to start it, it has several turns, and each turn may need several searches. That is the work Agent Runtime does for Echo.

The assistant is therefore a new **system agent**, "Help assistant" (preset `help_assistant`), and runs as ordinary `agent_run`s, as Echo does.

What Agent Runtime gives us:
- **Infrastructure we already run:**
  - model routing, BYOK connections and usage metering (`PreflightAgentRunAIUsage`, Runtime telemetry);
  - run records and the agent admin screens;
  - preset versions for workspace prompt changes;
  - the evaluation work planned in roadmap phase 5.
- **A tool loop.** The agent can search again with better words, read a whole article, and decide to hand off, instead of answering from one retrieval.
- **One way to build agents.** The docs say custom and system agents share one executor (`docs/agents-and-automation.md`). A one-off chat pipeline would be a second one.

What it costs, and how this plan handles each:

| Cost | Handling |
|---|---|
| Slower turns (several seconds, a few steps) | Progress labels while it works. The search box keeps the instant cached one-shot answer. |
| Needs the Runtime service | Production already runs it for Echo. The assistant uses lightweight execution, so it does not need Temporal; see below. When Runtime is unavailable, the assistant falls back to the one-shot answer. |
| Anonymous traffic can start many runs | The limits below, plus a cap on concurrent assistant runs per workspace. `MaxConcurrentRuns` is stored on agents today but not enforced, so the assistant enforces its own cap. |
| Anonymous text reaches an agent (prompt injection) | The agent can only read published content and reply. It has no tool that changes data, sees conversations or reaches private content outside the visitor's audience. |

The existing one-shot answer (`HelpcenterAISearchService.Answer`) stays for **Enter** in the search box. It is cached and fast, and the assistant reuses its validation rules.

## Execution mode: lightweight, not durable

Agent Runtime has two execution modes, chosen **per run** by the caller (`execution_mode` in StartRun, `agent-runtime/internal/engine/engine.go:271-283`). One deployment runs both at once:

- **`durable`** runs a Temporal workflow per run, with retries, heartbeats, pause and an idle-chat reconciler. Helpin sends `durable` for every run today (`server/internal/service/agent.go:622`).
- **`lightweight`** runs the turn in the Runtime API process. It keeps the run state in Postgres, so chat and approval resumes still work. It loses retries, manual pause and idle-chat expiry, and a pod restart drops a turn in progress.

The help assistant uses **`lightweight`**:
- **Durability is not needed.** A turn is a few reads and one reply. If a turn is lost, the customer sees "Try again" and the one-shot answer, and nothing is half done.
- **It keeps public traffic off the shared worker slots.** Durable native runs share the interactive queue, 8 concurrent per worker and 2 workers in production, with Echo and the Ask Agent. Anonymous visitors must never take those slots from real support conversations.
- **Faster starts.** A turn needs no workflow start or activity dispatch.
- **Lower Temporal usage.** A durable 30-second turn costs about 8 Temporal actions, most of them heartbeats every 5 seconds, and a chat workflow stays open until it goes idle. Whether that costs money depends on whether production uses Temporal Cloud, which the repositories do not show; check `TEMPORAL_ADDRESS` in Doppler. Either way it is small next to model tokens, so this is not the main reason.

Lightweight mode requires these changes:
- **Helpin selects the mode per agent.** The help assistant preset sets `lightweight`; all other agents stay `durable`.
- **One run per turn.** Each turn starts a new run with `complete_on_finish` and gets the last turns from `help_assistant_messages`, so no chat run stays open. Lightweight runs never expire on their own, because the idle-chat reconciler only checks durable runs.
- **Helpin bounds every turn.** A turn that has not answered within 60 seconds is cancelled and shown as failed, because lightweight runs are not recovered after a restart in a multi-pod deployment.
- **Helpin enforces the concurrency cap.** Lightweight runs are unbounded goroutines on the Runtime API pods, so the per-workspace cap (5 runs) must hold before StartRun. A global cap per Helpin replica protects the Runtime API pods.

## Design

### 1. Sessions, not inbox conversations

Echo requires a `support_conversation` target, so each chat would appear in the team inbox. The assistant must not fill the inbox. It gets its own target type:
- **`help_assistant_session`** (new table `help_assistant_sessions`) holds:
  - the workspace;
  - the surface: `helpcenter` or `portal`;
  - the audience: `public`, or `customers` when a portal session is present;
  - the portal identity, when signed in;
  - an anonymous visitor ID;
  - locale, the current page, and the turn count.
- **`help_assistant_messages`** holds each question and answer, with citations, status (`answered`, `no_answer`, `handed_off`) and feedback. Its rows are the analytics and the transcript used for a handoff.
- **One run per turn**, as explained under execution mode. The session's recent messages are the conversation memory.

### 2. The agent and its tools

The preset is `help_assistant` in `agent_presets.go`, with a runtime profile in `agentcontract/runtime_profiles.go`:
- **Model.** The flash tier, the same as Echo. Admins can change provider and model as for other system agents.
- **Prompt.** Answer only from retrieved articles and cite them. Say so when the articles do not answer. Never invent links, prices or policy. Offer support when stuck. The prompt is managed, with workspace preset versions for edits.
- **Allowed target types.** Only `help_assistant_session`.
- **Tools.** All are new and scoped by the session's audience. The server reads the audience from the session; the model never chooses it.
  - `search_help(query)` uses the shared retrieval: public spaces, plus spaces for signed-in customers when the audience is `customers`. It returns chunk IDs, titles and passages.
  - `read_help_article(article_id)` returns one live article in that audience as plain text, trimmed to a limit.
  - `send_help_answer(text, citations[])` gives the final reply. It is checked like the one-shot answer:
    - each citation must be a chunk returned to this run;
    - the text may contain no links;
    - the reply is stored as a message, together with the citation links.
  - `send_help_progress(label)` sets the "Searching the docs…" label.
  - `offer_contact_support(reason)` ends the turn with a handoff and records it as `no_answer` / `handed_off`.

### 3. Turn delivery

`POST …/assistant/messages` stores the question and starts or resumes the run, then returns the message ID at once. The client polls `GET …/assistant/sessions/{id}?after={message_id}` about once a second for the progress label and the reply.

We use polling, not streaming:
- Echo's "stream" only chunks a reply that is already saved.
- The help center's API proxy (`help-center/serve.mjs`) buffers compressible responses, which would break server-sent events.
- Polling also works through the portal mount.

Replies are short, so the delay is acceptable. Streaming can come later.

### 4. Where customers see it

- **Help center.**
  - An **Ask AI** action in the search dialog (⌘I), and on the search page, opens a side panel with the conversation. A visitor with an answer card can choose **Ask a follow-up**, which continues from that question.
  - Articles get **Ask about this page**, which passes the article as context.
  - When the chat widget is also on the help center, the assistant is the default for questions and the widget stays for contacting a person.
- **Portal.**
  - The same panel on the Help articles page, the home page and the new-request page, never inside a ticket.
  - Signed-in customers get the `customers` audience. The anonymous intake form gets `public`.
- **Handoff.**
  - **Contact support** opens the portal's new-request form, filled in with the question and a short transcript.
  - The created request links the session, so the agent sidebar shows the conversation and the articles cited, extending the existing portal context.
  - On a help center without a portal, it opens the chat widget instead, when enabled.

### 5. Settings and limits

- **Settings.** **Help assistant** goes on the Help center settings and the Customer portal settings. It is off by default, and needs the help center published and the agent enabled. The setting follows the shared-setting pattern of the help spaces picker: one value shown on both pages.
- **Rate limits**, a new limiter for the assistant routes:
  - 10 messages per minute and 100 per day per IP;
  - 20 turns per session;
  - 500 turns per workspace per day, matching Echo's daily cap;
  - at most 5 concurrent assistant runs per workspace (configurable). Above that, the client gets "busy" and falls back to the one-shot answer.
- **Budget.** The agent's `MonthlyTokenBudget` and the existing usage preflight apply. When the budget is spent, the panel says the assistant is unavailable and offers search and contact support.

### 6. Insights for admins

- A **Help assistant** block in Reports shows:
  - questions;
  - answered, not answered and handed off;
  - helpful votes;
  - the most frequent unanswered questions.
- Unanswered questions feed **Coverage gaps** as no-result signals, like portal intake.
- Each session links to its agent run for debugging, in the existing run views.

### 7. Inexpensive Mintlify extras (separate slice)

- **Copy page as Markdown** and **Open in ChatGPT / Claude** on public articles.
- **`/llms.txt`**, built from public spaces only and never from private ones.

## Found while researching (fix in this work)

- `POST /api/widget/messages` (the SDK route group, `router.go:551-562`) has no write rate limit, unlike `/widget` and `/api/widget/support`. Every message there can start an Echo turn. Apply the widget limiter.
- `HELPCENTER_ANSWER_MODEL` is read but never used: the one-shot answer always uses the route registry's model. Make the variable select the model, or remove it and its docs.

## Out of scope

- Streaming tokens.
- Actions on the customer's account, such as billing or settings. Those stay with Echo in a support conversation.
- Answers about a customer's own requests or tickets.
- Voice and attachments.

## Slices

Each slice gets review, tests and a live check on dev before the next.

1. **Sessions and the agent.** Migration, target type, preset and profile, the five tools with audience scoping, limits, concurrency cap, admin setting. Go tests for tool scoping (private content is never reachable from a public session), citation validation, limits and run resume.
2. **Help center panel.** The **Ask AI** panel, follow-ups from the answer card, **Ask about this page**, polling, feedback, fallback to the one-shot answer. Vitest, plus a live check with Runtime on dev.
3. **Portal and handoff.** The panel in the portal with the signed-in audience, a prefilled request, the session in the agent sidebar.
4. **Insights.** The reports block and coverage signals.
5. **Extras and fixes.** Markdown copy, `llms.txt`, Open in ChatGPT/Claude, the widget rate limit, the answer model variable.

## Verification

- Go tests for each slice.
- A Postgres test for the session tables.
- Vitest for the panel and the handoff.
- A live check on dev with Agent Runtime and the Temporal worker running:
  - a multi-turn conversation with citations;
  - a follow-up that needs a second search;
  - a question with no answer that ends in a prefilled request;
  - a signed-in portal customer getting a private article that an anonymous visitor never sees;
  - limits returning "busy" without errors.
