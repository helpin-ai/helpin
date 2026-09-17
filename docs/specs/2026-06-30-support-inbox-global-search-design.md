# Support Inbox Global Search Design

**Date:** 2026-06-30
**Status:** Approved for implementation planning
**Module:** Support Inbox

## Summary

Make Support Inbox search a first-class global search workspace, modeled after Intercom's Inbox search behavior and the reference image in `waqar-images/search.png`.

The current Helpin support inbox has a compact scoped search inside the conversation list. That is useful for narrowing the current view, but it is not a global search experience: it does not search message bodies, does not expose a results table, does not show snippets or highlighted matches, and does not support relevance sorting.

This feature adds a dedicated global search surface for support conversations. It searches across accessible inboxes and returns ranked, filterable results with snippets, highlights, result counts, status, assignment, and conversation display numbers.

This is the complete first-class search feature for the current support conversation model. It is not a phased MVP. The implementation should stay deliberately small: no async indexer, no Temporal workflow, no GORM hooks, and no separate search-entry table. Search uses generated/search-vector columns and expression indexes that are maintained by PostgreSQL.

## Product Goals

- Let support users find historical conversations quickly by customer email, conversation number, title, or message content.
- Match the Intercom mental model: a global search bar, filter chips, result count, table rows, highlights, and relevance/newest/oldest sort.
- Keep the existing inbox view search behavior stable as scoped search within the current view.
- Respect Team Inbox access rules. Users must only see conversations in the shared inbox or mailboxes they can access.
- Avoid inventing a separate ticket entity. Helpin's current support inbox items are `support_conversations`, so "ticket number" maps to the conversation display number.

## Non-Goals

- A separate support ticket object.
- Search over ticket attributes that do not exist in the current data model.
- Boolean keyword syntax such as `AND` / `OR`.
- Saved search presets beyond existing support inbox view persistence.
- Semantic or vector search. This feature uses deterministic full-text and exact/partial matching.
- Customer-facing widget search. This is teammate-only Support Inbox search.

## Current Behavior

### Frontend

- `ConversationList.tsx` renders a narrow side-panel list with a collapsed search icon.
- `supportInboxStore.searchQuery` is stored with view filters.
- `buildConversationListRequestFilters()` sends `search` to the normal list endpoint.
- Client-side `filterSupportConversations()` only checks subject, customer name, email, and display ID.
- There is no dedicated `/support/search` route or full-width search result workspace.

### Backend

- `SupportInboxHandler.ListConversations` accepts a `search` query param.
- `SupportConversationRepository.applyConversationSearch()` searches:
  - `support_conversations.subject`
  - `support_conversations.customer_name`
  - `support_conversations.customer_email`
  - `support_conversations.display_id`
- Message content is not searched.
- Sorting is `updated_at DESC` or `updated_at ASC`, not relevance.
- The list response returns conversations, not search-specific snippets or highlights.

`SupportConversation` serializes from `server/internal/model/support_inbox.go` and is already used by the existing list/detail APIs and frontend type `frontend/src/lib/pm-types/support.ts`.

## Target UX

### Entry Points

- The Support sidebar `Search` item opens global support search.
- The conversation-list search icon remains a scoped search for the current inbox view.
- The global search route should be reachable at a stable route such as `/w/:slug/support/search`.

### Layout

Global search is a full workspace, not a narrow conversation-list mode.

```
+----------------------------------------------------------------+
| [search input: project                                           ] |
|                                                                  |
| [All] [Assigned to] [Team inbox] [Tag] [User] [Created]          |
| [Status] [Priority] [Title] [+ filters]                          |
|                                                                  |
| 199 results found                             Sort: Relevance v  |
|------------------------------------------------------------------|
| User                  Preview              ID    Status Assignee |
| beth@example.com      Ticket · 3y           #31   Open   Unassigned
|                       Adding teammates to project...             |
|                                                                  |
| Blue Grill            I'm Fin, an AI assistant... - project...   |
|                                                                  |
+----------------------------------------------------------------+
```

### Required Result Behavior

- Keyword search runs globally across all inboxes the actor may access.
- Search can be run with keywords, filters only, or keywords plus filters.
- Filters use AND semantics.
- Multiple selected values within one filter group use OR semantics.
- Search terms are case-insensitive.
- Punctuation should not prevent ordinary matches.
- Quoted phrases, such as `"billing portal"`, require exact phrase matching.
- Searching `beth@example.com` returns conversations for that customer email.
- Searching partial email/domain values such as `beth` or `@example.com` returns matching customer emails.
- Searching `#31` or `31` returns the conversation whose `display_id` is `31`, ranked above incidental text matches.
- Searching a UUID conversation ID returns that conversation if accessible.
- Title/subject is included in global keyword search.
- A `Title` filter narrows keyword matching to conversation title/subject.
- Message body search includes non-deleted support reply content and non-deleted internal notes. System messages and deleted messages are excluded.
- Internal-note snippets are only returned if the actor can view internal notes. Today that follows `support.read`, because conversation messages are already readable with `PermSupportRead`. Search must follow the effective message-visibility policy. When the actor cannot view note text, a note-only match may still return the conversation row, but the snippet/highlight must fall back to subject/customer fields or be blank.
- Empty `q` with no filters is invalid and returns `400`. Empty `q` with at least one filter is valid and returns newest matching conversations.

### Filters

Supported filters:

- `All`: no entity type restriction. Since Helpin only has conversations in this module today, this means all accessible support conversations.
- `Assigned to`: current user, unassigned, specific teammate.
- `Team inbox`: shared inbox or one or more accessible team inboxes. In API params, `shared` is the public sentinel for `support_conversations.mailbox_id IS NULL`; it is never treated as a literal mailbox UUID.
- `Tag`: conversation tags.
- `User`: exact customer email or CRM contact where available.
- `Created`: date presets and custom date range over `support_conversations.created_at`.
- `Status`: open, waiting on customer, resolved, spam.
- `Priority`: low, medium, high, urgent.
- `Title`: title-specific text matching.
- `AI state`: supported under `+ filters` using the existing repository predicates:
  - `handling` maps to active AI handling: `human_takeover=false` and `flow_state='ai_handling'` or pending AI fallback.
  - `handoff` maps to AI escalation / requested human / queued-for-human conditions.
  - `resolved` maps to AI-resolved conditions: `human_takeover=false` and `flow_state='resolved_by_ai'` or `ai_state='resolved'`.

Unavailable filters:

- `Company`, `Topic`, `AI Topic`, `AI Subtopic`, `Brand`, and ticket attributes are not support conversation fields in the current Helpin data model.
- They are not omitted as deferred search work; they are outside this feature's source data unless those product entities become part of `support_conversations`.

### Sort

- `Relevance` is the default when a keyword is present.
- `Newest` sorts by `support_conversations.updated_at DESC`.
- `Oldest` sorts by `support_conversations.updated_at ASC`.
- Filters-only searches default to `Newest`, because there is no keyword relevance signal.

## Backend Design

### API

Add a new search endpoint:

```text
GET /api/support/inbox/search
```

Required query params:

- `workspace_id`

Optional query params:

- `q`
- `sort`: `relevance`, `newest`, `oldest`
- `page`
- `per_page`
- `assigned_to`: `me`, `unassigned`, teammate UUID, or comma-separated values
- `mailbox_ids`: comma-separated mailbox IDs, with `shared` for `NULL mailbox_id`
- `tag_ids`: comma-separated support tag IDs
- `customer_email`
- `created_from`: `YYYY-MM-DD`
- `created_to`: `YYYY-MM-DD`
- `statuses`: comma-separated support statuses
- `priorities`: comma-separated priorities
- `title`
- `ai`: existing AI filter values such as `handling`, `handoff`, `resolved`

Limits and validation:

- `q`, `title`, and `customer_email` are trimmed and capped at 256 characters.
- `per_page` defaults to 50 and is capped at 50.
- `page` must be positive.
- A quoted phrase query supports one exact phrase. Multiple quoted phrases are rejected with `400` rather than converted into a pathological search query.
- Either `q` or at least one filter is required. Requests with neither `q` nor filters return `400`.

Response shape:

```json
{
  "data": [
    {
      "conversation": {},
      "display_id": 31,
      "matched_fields": ["title", "message", "customer_email"],
      "snippet": "Adding teammates to project...",
      "highlights": [
        {
          "field": "message",
          "text": "Adding teammates to project...",
          "ranges": [{ "start": 20, "end": 27 }]
        }
      ],
      "score": 0.82
    }
  ],
  "total": 199,
  "page": 1,
  "per_page": 50,
  "total_pages": 4,
  "meta": {
    "sort": "relevance",
    "query": "project",
    "total_capped": false,
    "total_cap": 1000
  }
}
```

The response intentionally wraps a normal `SupportConversation` so existing row rendering and navigation can reuse known types while search-specific fields remain separate.

`total` is exact up to 1000. For larger result sets, the repository counts to 1001, returns `total: 1000`, `meta.total_capped: true`, and the UI renders `1000+ results found`. `total_pages` is computed from the capped total.

### Query Semantics

Use PostgreSQL full-text search when running against Postgres:

- Conversation vector:
  - high weight: subject, display ID as text, customer email
  - medium weight: customer name
  - normal weight: message content
- Exact phrase search:
  - detect balanced double-quoted query
  - use phrase matching or exact `ILIKE` fallback over subject, customer fields, and message content
- Numeric query:
  - normalize leading `#`
  - if numeric, add exact `display_id` match and rank it above text matches
- UUID query:
  - if UUID-looking, add exact `support_conversations.id` match
- Email query:
  - if email-looking, prioritize exact `LOWER(customer_email) = LOWER(q)`
  - still allow partial email/domain matches for unquoted non-exact queries

SQLite test fallback may use `LOWER(...) LIKE` and `EXISTS` over messages. The production behavior should be optimized for Postgres.

### Repository Boundaries

Add a dedicated search path rather than overloading `ListConversations`:

- Model/DTO: `SupportConversationSearchResult`, `SupportConversationSearchResponse`, `SupportConversationSearchParams`
- Handler: parse and validate search query params in `support_inbox.go`
- Service: enforce workspace ID, normalize params, resolve actor mailbox scope
- Repository: build the search query and apply existing mailbox access restrictions

The existing conversation list endpoint remains unchanged except where shared helpers are extracted.

### Access Control

Search must apply the same mailbox access rules as conversation listing:

- owners/admins can search all workspace conversations
- other support users can search shared inbox plus team inboxes where they are members
- direct display ID/UUID search must not leak inaccessible conversations

### Search Indexing and Migration

Add a dbmigrate SQL migration for production search performance. The feature does not add a materialized search table.

Conversation-level search uses a stored generated `tsvector` column on `support_conversations`:

```sql
search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector('simple', coalesce(subject, '')), 'A') ||
  setweight(to_tsvector('simple', coalesce(customer_email, '')), 'A') ||
  setweight(to_tsvector('simple', display_id::text), 'A') ||
  setweight(to_tsvector('simple', coalesce(customer_name, '')), 'B')
) STORED
```

Message-level search uses a stored generated `tsvector` column on `support_messages`:

```sql
search_vector tsvector GENERATED ALWAYS AS (
  to_tsvector('simple', coalesce(content, ''))
) STORED
```

Indexes:

- GIN index on `support_conversations.search_vector`
- GIN index on `support_messages.search_vector`
- trigram indexes on `support_conversations.customer_email`, `support_conversations.subject`, and `support_messages.content`
- B-tree indexes for common filters if missing: `workspace_id`, `display_id`, `created_at`, `updated_at`, `status`, `priority`, `assigned_user_id`, `mailbox_id`, plus `support_messages(workspace_id, conversation_id, created_at)`

PostgreSQL maintains generated vectors synchronously with row writes, so there is no refresh hook, trigger, async job, or drift-prone application-maintained column. Existing rows are computed by PostgreSQL when the generated columns are added. If generated `tsvector` columns are not supported by the target Postgres version, the implementation should use equivalent expression GIN indexes instead, not a separate denormalized table.

Snippets are produced at query time from the highest-ranked matching field/message using `ts_headline` or a safe application-side range builder. The API still returns plain text plus highlight ranges, never HTML.

Backfill rollout is limited to adding generated columns and indexes. It should run as a normal dbmigrate migration using `IF NOT EXISTS`; no separate background job is required.

## Frontend Design

### Route and State

Add a route such as:

- `frontend/src/routes/_authenticated/w/$slug/support/search.tsx`

Add search state to URL params so search results are shareable:

- `q`
- `sort`
- `assigned_to`
- `mailbox_ids`
- `tag_ids`
- `customer_email`
- `created_from`
- `created_to`
- `statuses`
- `priorities`
- `title`
- `ai`

The global search page should not reuse `supportInboxStore.searchQuery`, because that field belongs to scoped inbox views.

### Components

Create focused components:

- `SupportGlobalSearchPage`
  - route-level orchestration
  - reads/writes URL search state
  - calls search query hook
- `SupportSearchToolbar`
  - keyword input
  - sort selector
  - clear/reset actions
- `SupportSearchFilterBar`
  - chip row and filter popovers
- `SupportSearchResultsTable`
  - result count
  - desktop table rows
  - mobile stacked rows
- `SupportSearchHighlight`
  - renders plain text with highlight ranges from the backend
  - escapes all text and inserts local `<mark>` elements by offsets; it never renders backend HTML

### UI Details

- Use restrained app UI styling consistent with existing Support Inbox.
- Use lucide or the project's existing icon wrapper for search/filter/status icons.
- Keep the table dense and scannable.
- Do not put the table inside a decorative card.
- Highlight matches with a low-contrast mark background that works in light and dark mode.
- Loading state uses table skeleton rows.
- Empty state says no conversations match and suggests changing keywords or filters.

## Testing

Backend:

- exact customer email match
- partial email/domain match
- display number search with and without `#`
- UUID search
- title search
- message body search
- quoted exact phrase search
- filters-only search
- keyword plus filters using AND semantics
- relevance sort puts exact display ID/email/title matches above incidental message matches
- mailbox access prevents inaccessible results
- pagination and total counts

Frontend:

- URL state parsing/building
- search service query serialization
- filter chip interactions
- result table rendering with highlights
- empty/loading/error states
- clicking a result navigates to the conversation detail route
- internal-note matches do not expose note snippets when the actor lacks note visibility

Performance:

- backend caps counts at 1000 and exposes `total_capped`
- backend rejects overlong queries and multiple quoted phrases
- search endpoint returns within the same order of magnitude as the existing conversation list for filters-only searches, and should have indexed query plans for keyword searches
- migration tests or SQL validation should confirm generated columns/indexes are idempotent

## Rollout Notes

- No feature flag is required.
- Keep existing scoped list search as-is for users working inside a view.
- The visible sidebar `Search` item becomes route navigation to global search. The conversation-list search icon remains scoped to the active view.
- Global search should be additive and should not change unread counts, inbox list selection, or current custom view behavior.
- No long-running backfill job is expected. The migration adds generated columns and indexes; index build time depends on message volume, so stage should run the migration before production rollout.
