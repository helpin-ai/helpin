# Help assistant (Ask AI)

The help assistant answers questions from help center visitors and customer portal customers in a conversation. It answers only from published help articles, cites them, and offers to contact support when the articles do not answer. It is the system agent **Help assistant** (preset `help_assistant`).

It is off by default. Turn on **Ask AI help assistant** in **Settings → Help center** (chat widget section) or **Settings → Customer portal**. Both pages show the same setting. The assistant is offered only while the help center is published and Agent Runtime can launch runs.

## Where customers use it

- **Help center:**
  - **Ask AI** in the top bar, or ⌘I / Ctrl+I.
  - **Ask AI about “…”** in the search dialog.
  - **Ask a follow-up in Ask AI** under a one-shot AI answer.
  - **Ask AI about this page** on an article, which gives the assistant the article as context.
- **Customer portal:** **Ask AI** on the home, Help articles and new-request pages.

The panel shows a progress label while the assistant works, then the answer with its **Sources**, copy, and helpful / not helpful buttons. A conversation is kept for the browser tab and holds up to 20 questions; **New chat** starts over. While the panel is open on the help center, the chat widget launcher is hidden.

**Contact support** appears when the assistant could not answer. It opens the portal's new-request form, filled in with the question and the conversation, wherever the portal is served. When the portal takes no requests it opens the chat widget, or shows the support email. The request links the conversation, so teammates find it in the conversation sidebar under **Asked the help assistant first**.

## What it can read

| Who is asking | Articles |
|---|---|
| Help center visitor, or a portal visitor who is not signed in | Public help spaces |
| Customer signed in to the portal | Public help spaces and [spaces for signed-in customers](support-portal-access.md#help-articles-for-signed-in-customers) |

A conversation keeps the audience it started with: signing in starts a new conversation. Articles are read in the visitor's language when a translation is published, and in the default language otherwise.

The assistant's tools only read published content and reply:

- `search_help`: searches the articles the conversation may read, by meaning with a keyword fallback.
- `read_help_article`: reads one of those articles.
- `send_help_answer`: sends the answer and ends the turn.
- `send_help_progress`: shows a progress label.
- `offer_contact_support`: offers support and ends the turn.

The server checks every answer:
- each citation must be a passage returned to this turn;
- answers may not contain links or URLs, since the cited articles become the links.

Help assistant runs cannot use any other tool, and help assistant tools cannot run outside a help assistant turn.

## How it runs

Each question is one Agent Runtime run in **lightweight** execution: the turn runs in the Runtime API process without Temporal. The run's target is the pending answer (`help_assistant_turn`), and the recent conversation is passed in with the question. No chat run stays open and no support conversation is created. Lightweight runs are not retried, so the turn is safe to lose: the customer sees **Try again**.

Conversations are stored in `help_assistant_sessions` and `help_assistant_messages`. A session belongs to the visitor's browser (a random ID kept in local storage), to the surface, and in the portal to the signed-in customer.

## Limits

- **Per IP:** 10 questions per minute and 100 per day (needs Redis).
- **Per conversation:** 20 questions.
- **Per workspace:** 500 questions per day, and 5 answers being written at once. Above that the help center falls back to the one-shot AI answer, and the portal offers support.
- **Timeout:** an answer not written within 75 seconds is closed as failed and its run cancelled.
- **Budget:** the agent's monthly token budget, if set.

Usage is metered like other agent runs. Admins can change the agent's model in **Agents**.

## Insights

**Reports** shows a **Help assistant** block for workspace-wide reports:
- the share of questions answered from the articles;
- questions where the assistant offered support;
- helpful ratings and failures;
- the latest unanswered questions.

Each unanswered question is also recorded as a no-result search, so it appears in **Coverage gaps**.

## Related

- Articles also offer **Copy page** (as Markdown), **Open in ChatGPT** and **Open in Claude**.
- The help center serves `/llms.txt`, a list of its public articles for AI tools. It never lists articles for signed-in customers.

## API

Help center routes, under `/api/hc/{subdomain}`:
- `POST /assistant/messages`
- `GET /assistant/sessions/{id}?visitor_id=`
- `POST /assistant/messages/{id}/feedback`

The portal has the same routes under `/api/public/portal/{slug}`, where a valid portal session adds the spaces for signed-in customers. Responses are `Cache-Control: private, no-store`.
