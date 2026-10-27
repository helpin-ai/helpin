# Phase 1b: human-owned portal tickets and article suggestions at intake

Date: 2026-10-27. Branch: `feat/company-portal` (follows phase 1 of the
[support depth roadmap](2026-10-26-support-depth-roadmap.md)).

Status: built. Two changes from the plan, both found while testing against a
live help center:

- Semantic search had no relevance floor, so every request was shown the same
  unrelated articles. Results now need a shared keyword or a similarity of at
  least 0.40.
- The teammate-facing route is `portal-context`, which combines the company
  audience with the articles opened. It replaces the unreleased
  `portal-audience` route.

Customers reach the portal after self-service has not answered them. This
phase changes two things:

- **AI tries to help before a ticket exists.** While the customer writes, the
  portal suggests help center articles.
- **People reply once the ticket exists.** AI never replies to a customer on a
  portal ticket; it may only draft a private note for the teammate.

It also turns the help center's retrieval code into the one public knowledge
search that every customer-facing surface uses.

## How others do it

| Product | Before the ticket | After the ticket |
|---|---|---|
| Zendesk | The help center request form suggests articles as the customer types the subject ([new request page](https://developer.zendesk.com/apps/docs/help-center-templates/new_request_page)). AI agents for web forms show an answer in a dialog right after submit ([announcement](https://support.zendesk.com/hc/en-us/articles/9462934829978-Announcing-AI-agents-for-email-and-web-form)). | The ticket stays open for agents when the answer doesn't help. Automated resolution can be [turned off](https://support.zendesk.com/hc/en-us/articles/7460877856026-Turning-off-automated-resolution-features-for-AI-agents). The article autoreply is [legacy](https://support.zendesk.com/hc/en-us/articles/4408820951450-Using-autoreplies-to-recommend-articles-in-web-forms-Legacy). |
| Intercom | Fin answers in the Messenger and can require a search before a ticket is opened. | Teammates handle tickets; Fin assists them, for example by [filling in ticket fields](https://www.intercom.com/changes/en/35348-speed-up-ticket-workflows-with-fin-ai-autofill). |
| Kustomer, Document360, kapa.ai | Article suggestions inside the form ([Kustomer](https://help.kustomer.com/en_us/suggest-articles-in-a-form-B1IdM4noL), [Document360](https://docs.document360.com/docs/adding-a-new-ticket-deflector), [kapa.ai](https://docs.kapa.ai/integrations/support-form-deflector/tutorials/zendesk-support-form)). | Unchanged. |
| Mintlify (docs only) | The [assistant](https://www.mintlify.com/docs/ai/assistant) answers from the docs with citations, uses the current page as context, and gives a support email when it cannot answer. Each page has a [contextual menu](https://www.mintlify.com/docs/ai/contextual-menu): copy as Markdown, ask the assistant about the page, open in ChatGPT or Claude, and an MCP server. | Not a ticketing product. |

Across these products, AI helps the customer find an answer before a ticket
exists. Once a ticket exists, people own it.

## What Helpin has today

| Surface | Search | AI |
|---|---|---|
| Help center search dialog | Keyword type-ahead, debounced 300 ms (`help-center/src/components/search/SearchDialog.tsx`) | **Enter** asks for an AI answer |
| Help center search page | Keyword, or semantic with `mode=semantic` (`DocsHandler.PublicSearchArticles`) | One grounded answer per question (`POST /hc/{subdomain}/answer`) |
| Chat widget help tab | Keyword only (`SupportInboxService.SearchWidgetHelpArticles`) | Echo, through Agent Runtime, in a support conversation |
| Help center pages | — | The chat widget is embedded when enabled (`help-center/src/components/layout/AppShell.tsx`) |
| Portal intake | None | — |
| Portal tickets | — | `portal_ai_mode`: `off` (default), `internal_note`, `ai_first`. In `ai_first`, Echo replies to the customer. |

`HelpcenterAISearchService` (`server/internal/service/helpcenter_ai_search.go`)
already searches only published external spaces. Each search makes one metered
embedding call for the query, then a hybrid keyword-and-vector query over
article chunks, grouped by article. Grounded answers make one cheap model call
with server-validated citations. They are cached by question and content
fingerprint, limited per workspace per day, and rate limited.

## Decision: search, not an Agent Runtime run

Suggestions use the existing retrieval: one embedding call, which can be cached,
plus one SQL query, typically 100–300 ms. An Agent Runtime run is a durable,
multi-step model loop that takes seconds and creates run records; it is meant
for a conversation, not a keystroke. A customer writing a request triggers about
3–8 debounced searches, so a run per search would cost far more for no better
results. Agent Runtime stays where it belongs: Echo in the Messenger, and
internal-note drafts on tickets.

## Part A: portal tickets are human-owned

1. **Settings.** `portal_ai_mode` accepts only `off` and `internal_note`, and
   rejects `ai_first` with an explanation. The Customer portal settings page
   offers "Off" and "Draft replies for teammates". The chat widget and email
   keep their own AI settings unchanged.
2. **Request creation.** Portal requests never start in the AI-handling state.
   They go to the inbox like any new request. `internal_note` still queues the
   AI draft through the existing portal AI dispatch.
3. **Migration**, applied once:
   - Workspaces with `portal_ai_mode = 'ai_first'` change to `internal_note`.
     They keep AI assistance without sending AI replies to customers.
   - Unpublished portal AI dispatch rows queued as `ai_first` become
     `internal_note`.
   - Open portal conversations still waiting on AI go to the human queue
     through the existing handoff path, so no customer waits on an AI that will
     no longer reply.
4. **Defense in depth.** The portal AI dispatcher checks the workspace's
   current mode when it publishes. A row queued before a settings change can
   never post a customer-visible AI reply.
5. **Portal UI.** "Echo is working on your request…" can no longer appear on
   portal requests. Status depends only on the inbox state.

Tests:
- settings validation;
- creation without the AI-handling state;
- the dispatcher refuses customer-visible replies;
- the migration in PostgreSQL, including pending dispatch rows;
- portal detail never reports AI processing.

## Part B: article suggestions while writing

### Server

- **Route.** `GET /api/public/portal/{slug}/suggestions?q=` is public, because
  only published help center articles are returned. It sits in the portal route
  group, so the existing per-client rate limiter applies. Requests need at least
  two words and 12 characters, and the text is cut at 400 characters (the
  existing answer limit).
- **Service.** `CustomerPortalService.SuggestArticles` calls
  `HelpcenterAISearchService.SemanticSearch` with the workspace's default help
  center locale and a limit of 5. It returns each article's id, title, space,
  one snippet and its public URL, built with the existing help center
  public-URL helpers (`supportPublicArticleURLs`). With no published help
  center, it returns an empty list.
- **Embedding cache.** `retrievePublicChunks` caches query embeddings in Redis
  for 1 hour, keyed by a hash of the embedding model and the normalized query.
  Help center search benefits too. Metering records the surface (`portal_intake`
  or `helpcenter_search`) under the existing help center search action.
- **Fallback.** When embeddings are unavailable, the existing keyword search
  runs, as on the help center search page.

### Portal UI

- **Forms.** The signed-in form and the form for sending without signing in
  both get the same panel.
- **Layout.** From `lg`, a two-column form: fields on the left, a 300 px
  "Related articles" panel on the right. Below `lg`, the list follows the
  subject field. The panel stays hidden until it has results, and its column
  keeps its width so the layout does not jump.
- **Search behavior.** The search text is the subject plus the first 500
  characters of the message, debounced by 500 ms. Stale requests are cancelled
  and the previous results stay visible while the next search loads.
- **Rows.** Each row shows the title, space and a two-line snippet. Clicking it
  opens the article in a new tab, so the draft stays in place. The draft is also
  saved to session storage in case the customer navigates away.
- **Accessibility.** A polite live region announces "3 related articles".
  Keyboard users reach the panel after the message field.
- **Shared fetching.** One hook, `usePortalArticleSuggestions`, is used by both
  forms.

### Tests

- Go:
  - minimum and maximum text lengths;
  - no help center;
  - locale fallback;
  - embedding cache hit and miss;
  - keyword fallback when embeddings fail;
  - only published external articles are returned.
- Vitest:
  - debounce and cancellation;
  - the panel shown and hidden;
  - links open in a new tab;
  - the draft is restored.

## Part C: measuring deflection and informing agents

- **Events.** Each form mount gets a random intake id. The portal posts small
  events to `POST /api/public/portal/{slug}/suggestions/events`:
  `shown` (with article ids), `opened` (with one article id) and `submitted`.
  The route is rate limited and its payload is validated. Events are stored in
  a new `support_portal_intake_events` table (workspace, intake id, event,
  article id, conversation id, time). An intake counts as **likely deflected**
  when an article was opened and nothing was submitted within 30 minutes.
- **Reporting.** The support reports overview gets a "Portal intake" block:
  - forms that showed suggestions;
  - articles opened;
  - requests submitted after opening an article;
  - likely deflected.

  It is derived from the events table, as the other report metrics are derived
  from their source tables.
- **Agent context.** The created request records the suggested and opened
  article ids on its first message. The conversation sidebar shows "Before
  writing in, the customer opened: …", so teammates don't send the same
  article again.
- **Coverage gaps.** A request submitted after its intake found no articles
  feeds the existing coverage signal (`IsEligibleCoverageWidgetSignal` and the
  coverage pipeline). It is a strong signal: the customer looked, found
  nothing, and needed a person.

## Part D: one public knowledge search for every surface

- `HelpcenterAISearchService` is the single retrieval used by:
  - help center search;
  - help center answers;
  - the widget help tab;
  - portal intake.

  It is not renamed in this phase.
- **Widget help tab.** Its search moves from keyword-only to the same semantic
  search with keyword fallback, so customers get the same results on every
  surface. It keeps its own coverage signal.
- **Shared rules:** published external spaces only, the embedding cache, the
  metering surfaces, and coverage signals.
- **Frontend.** The help center (`help-center/`) and the portal (`frontend/`)
  are separate apps. They share the API contract, not a UI package.

## Not in this phase: a help center and portal assistant

**Recommended as the next phase.** Mintlify's assistant is the right model for
customers who are reading the docs.

- **What.** A multi-turn, grounded "Ask" assistant shared by the help center and
  the portal home page. It extends the existing answer pipeline:
  - retrieval on every turn;
  - server-validated citations;
  - the current article as context;
  - feedback votes.

  It uses no Agent Runtime and creates no inbox conversation.
- **Where.** On the help center: from the search box ("Ask AI", ⌘I) and an
  "Ask about this page" action on articles. In the portal: on the home and
  new-request pages, never inside a ticket.
- **Escalation.** When it cannot answer, "Contact support" opens the portal's
  new-request form, prefilled with the question and a short transcript.
- **Analytics.** Admins see the questions, the unanswered ones and the votes.
  Unanswered questions feed coverage gaps.
- **Inexpensive additions from Mintlify:**
  - "Copy page as Markdown";
  - `llms.txt`;
  - "Open in ChatGPT/Claude" on articles.
- **Echo** remains the Messenger agent for account-specific help and actions,
  and runs only inside a conversation.

This phase does not include it, so the portal changes can ship and be measured
first.

## Slices

1. **Part A:** human-owned portal tickets and the migration. Ships alone.
2. **Part B:** suggestions route, embedding cache and portal panel.
3. **Part C:** events, report block, agent sidebar and coverage signal.
4. **Part D:** widget help search on the shared retrieval.

## Verification

- Go unit tests, plus PostgreSQL tests for the migration and the events table.
- Vitest for the portal panel and the hook.
- Typecheck and build for both editions.
- A live check on the dev deployment:
  - suggestions while typing;
  - an opened article followed by no submission counts as deflected;
  - a submitted request shows the opened articles to the teammate;
  - an `ai_first` workspace was migrated and its next portal request gets no
    AI reply.

## Risks

- **Cost from typing.** Mitigated by the debounce, the minimum length, the
  embedding cache and the rate limiter. Each search costs one cached embedding
  and one SQL query, with no model call.
- **Workspaces relying on `ai_first`.** They lose automatic customer replies on
  portal tickets. The migration keeps AI drafts for teammates. The settings page
  explains that portal requests are answered by people, and the Messenger
  remains the place for automatic AI answers.
- **Weak suggestions.** Only results from the shared search are shown, and
  nothing appears without a match. The deflection measure shows whether the
  feature helps.
