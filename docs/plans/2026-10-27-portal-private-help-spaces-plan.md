# Private help spaces for signed-in portal customers

Date: 2026-10-27. Follows [the portal intake plan](2026-10-27-portal-intake-deflection-plan.md).

A support admin can mark a help center space as **for signed-in customers**.
Its articles are then readable only by customers signed in to the customer
portal, and never by anonymous visitors, search engines, the chat widget or
Echo.

## How competitors do it

- **Zendesk** restricts sections and articles to user segments such as
  "signed-in users". Restricted articles stay inside the help center, which then
  needs a sign-in to show them.
- **Intercom** targets help center collections at audiences. Restricted content
  requires a signed-in or identity-verified user.

Both put restricted content inside the help center itself.

## Decision: private articles are read in the portal, not on public help center pages

The help center is built to be public:
- The help center HTML cache keys pages by host and path only, with no user in
  the key.
- The API marks responses `Cache-Control: public` and caches them in Redis with
  no user component.
- About 25 public queries decide visibility with a copied SQL fragment, and
  several of them never check the space type.

Showing private content on those pages would need every one of those paths to
know about the viewer, and one miss leaks an article to everyone through a
shared cache. A branch inventory (2026-10-27) found no central predicate to
change instead.

So private articles are read **in the portal**. The portal is already mounted
on the help center at `/requests`, signed in, and served with
`Cache-Control: private`. Customers get a **Help** tab there. Showing private
articles on the public help center pages stays out of scope.

## Design

1. **Space audience.** A new column `docs_spaces.helpcenter_audience`:
   - `public` (the default): today's behavior;
   - `customers`: only signed-in portal customers can read the space.

   It applies only to external-capable spaces. The space dialog shows it next to
   the type.
2. **Publishing is unchanged for authors.** Publish, slugs, collections and
   snapshots work as in any help center space. Publishing in a `customers`
   space sets the new `docs_helpcenter_articles.customer_published_at`, never
   `public_published_at`. Every public article query already requires
   `public_published_at`, so private articles are excluded from the help center,
   search, AI answers, sitemaps, the widget and Echo without touching those
   queries. The article's public search entries are not built.
3. **Public space listings exclude `customers` spaces.** Places that list
   external spaces publicly also require `helpcenter_audience = 'public'`:
   - the help center bootstrap, spaces and navigation;
   - space translations;
   - public article link targets;
   - the widget space picker and the widget itself.

   Admin screens that list help center spaces mark private ones.
4. **Changing audience moves the live articles.** Switching a space between
   `public` and `customers` moves each live article to the matching timestamp
   column, removes or rebuilds its public search entries, clears the help
   center caches and re-syncs the search index.
5. **Changing a space to internal unpublishes it.** Today, changing an
   external space to internal leaves its articles public through the paths
   that never check the space type. This is fixed too.
6. **Portal reading.** Routes need a portal session:
   - `GET /public/portal/{slug}/help`: private spaces, collections and article
     titles;
   - `GET /public/portal/{slug}/help/articles/{public_id}`: one article,
     rendered to HTML on the server like public articles;
   - `GET /public/portal/{slug}/help/search?q=`: semantic search with a
     keyword fallback over private spaces, through the same chunk retrieval as
     public search.
7. **Suggestions while writing a request.** Signed-in requests also suggest
   private articles. Anonymous intake never does.
8. **Search index.** The embedding indexer also indexes live articles in
   `customers` spaces, from the published snapshot. Chunk retrieval accepts a
   private article only when the caller passes its space explicitly, which only
   portal routes do.

## Out of scope

- Private articles on the public help center pages.
- Restricting content by company or by role.
- Translations of private articles. They use the default locale.
- Echo drafting from private articles.

## Verification

- Go tests for each layer:
  - publish and unpublish in both audiences;
  - an audience change in both directions;
  - an internal type change unpublishing articles;
  - public endpoints returning nothing from a private space: bootstrap,
    navigation, article by every path, lexical and semantic search, answer
    retrieval, widget, suggestions without a session;
  - portal routes requiring a session.
- Vitest for the space dialog, the portal Help tab and the article page.
- A live dev check: a private article is readable after portal sign-in and is
  absent from the help center, its search, the widget and anonymous suggestions.
