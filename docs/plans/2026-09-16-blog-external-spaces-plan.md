# Blog external spaces

Research 2026-09-16; scope revised 2026-09-17. Planning only; no application changes.

A blog earns its place only as a small customer-communications surface: teams can publish product updates and educational posts from the same workspace as their support content. That is useful reuse, but reuse alone does not establish customer demand. Given the reported two-deal stage and substantial existing product surface, this should remain subordinate to closing Intercom-alternative support gaps. The timestamp correction and type audit are useful first work independently; the blog scope below assumes the product decision to proceed.

## Recommendation

Add **Blog** as a third Docs space type alongside `internal` and `external_capable` (help center). One blog space is one publication, with its own address and branding, managed inside Helpin. Reuse the Docs editor, collaboration, permissions, version history, publication snapshots, assets, and public rendering infrastructure.

Take Ghost's automatic publishing/SEO defaults and Substack's clear publication identity, readable posts, and simple publishing flow. Ship one chronological list layout, post pages, archive/tag pages, hosted/custom domains, and RSS. Newsletter delivery, memberships, and a social network are separate product scopes.

## Research informing the scope

| Reference | Useful pattern | Helpin decision |
| --- | --- | --- |
| [Ghost SEO](https://ghost.org/help/seo/) | Metadata fallbacks, canonical URLs, social cards, and automatic sitemaps | Generate these automatically; expose optional overrides in publishing settings |
| [Ghost post settings](https://ghost.org/help/post-settings/) | Slug, excerpt, tags, authors, and canonical override | Small post settings panel using existing fields; defer external canonical overrides |
| [Ghost RSS](https://ghost.org/help/where-can-i-find-my-rss-feed/) | Feed available automatically | Include a publication RSS feed at launch |
| [Substack website layouts](https://support.substack.com/hc/en-us/articles/360039015892-How-do-I-switch-my-publication-s-homepage-to-a-different-layout) | Recent posts, navigation, publication branding | One list layout and existing brand settings; defer layout customization |
| [Substack publishing](https://support.substack.com/hc/en-us/articles/360037831771-How-do-I-publish-a-new-post-on-Substack) | Writing and publishing are straightforward; web publication can be separate from email distribution | Explicit Preview, Publish, Update, and Unpublish; publishing does not imply sending email |
| [Substack sections](https://support.substack.com/hc/en-us/articles/360060687771-How-can-I-create-multiple-newsletters-or-podcasts-under-one-publication) | Sections can represent separately subscribed newsletters | Use ordinary tags initially; do not introduce sections and subscription segmentation |

## What already exists in Helpin

These are findings from source inspection, not a production deployment audit.

- `frontend/src/components/docs/SpaceDialog.tsx`: Internal/External choices; External currently means publishable to the help center.
- `server/internal/model/docs.go`: external-capable spaces, documents with excerpts/tags, versions and object links; help-center settings include branding, hosted/custom/reverse-proxy URLs, locale settings, SEO and social metadata.
- `server/internal/model/docs_helpcenter_publication.go`: public snapshots separated from editable drafts, keyed by document and locale. Title, slug, excerpt, content, SEO, and OG fields are already snapshotted. Only tags, cover, and byline need additional snapshot fields.
- `help-center/`: TanStack Start public app with server rendering, article rendering, preview routes, metadata, canonical URLs, JSON-LD, sitemap/robots handling, redirects, and caching.
- `server/internal/service/docs_helpcenter_cache.go` and `help-center/serverRenderCache.mjs`: existing cache infrastructure and invalidation mechanisms.

The remaining work:

1. Help-center configuration is unique per workspace. A workspace needs to keep its existing help center while adding independently configured blog spaces.
2. Type handling extends well beyond public repository queries: the current tree has 57 non-test server references to `external_capable`/its constant and 19 frontend source files mentioning the type. Audit background translation, embedding/knowledge ingestion and retrieval, widget retrieval, imports, validators, and tool schemas as well as UI and public reads. A distinct `blog` type keeps positive help-center checks exclusive; audit fallback branches too.
3. The date issue is a small correction, not a schema gap. `DocsHelpcenterArticle.PublicPublishedAt` already holds the publication marker/date; `PublishExternally` currently overwrites it. Set it only when nil, and use the snapshot's `PublishedAt` as the public modification date. Expose the two dates correctly in public responses and SEO; no new timestamp columns or ordering fields.
4. Sitemap generation currently walks help-center navigation/collections. Blog discovery must work for posts without collections.
5. Add public bylines, cover images, archive presentation, and RSS. Publication branding/settings mostly reuse existing config fields; the main configuration work is scoping, not a new settings system.

## Launch experience

Inside Helpin:

- Create space → **Internal / Help center / Blog**, mapped to `internal / external_capable / blog`. Existing spaces keep their types.
- A Blog space opens a post list with Drafts and Published filters, title, public author, publication date, and an unpublished-changes indicator.
- Write using the existing Docs editor, comments, history, and supported media blocks. Keep links to tasks, projects, and other internal objects available to the team; public output must exclude private object details.
- Post settings: excerpt, cover and alt text, public byline, tags, slug, and optional SEO/social overrides. Use a deliberately selected public name/avatar/bio; do not expose workspace emails or internal profile records.
- Preview the real public layout. Publish creates the live snapshot; subsequent edits remain drafts until Update. Unpublish removes the post from all public discovery surfaces.
- Publication settings live on the space and reuse existing config fields: brand name, description, logos/favicon, color/theme, navigation/footer links, SEO/OG fields, and hosted/custom address. No new typography selector or page builder.

For readers:

- Branded homepage with recent posts in reverse chronological order, using one list layout.
- Comfortable mobile reading: title, excerpt, byline, date, optional cover, article body, and copy/share link.
- Paginated archive and tag pages provide discovery. Collections may still organize work internally but are not required to publish a blog post. Blog-scoped search is deferred; adapting knowledge/search scope is unnecessary first-release work.
- RSS link and configurable CTA such as “Try the product” or a link to an existing newsletter signup page. No inactive subscribe form.
- Help-center translation, embeddings/agent knowledge, and widget article retrieval exclude blog spaces. Do not embed the support widget on the blog or offer blog knowledge-source ingestion in this release.

## Serving and SEO requirements

| Area | Launch behavior |
| --- | --- |
| HTML | Deliver full post content, headings, links, metadata, and JSON-LD in server-rendered HTML. Use the same content for readers and crawlers. Reuse the existing SSR app. |
| URLs | One canonical public address per publication. Hosted URL and custom domain only at launch. |
| Post routes | Use `/p/<slug>-<public-id>`, retaining Helpin's stable public ID convention. Reuse `DocsSlugAlias` and existing alias service/repository methods for permanent redirects. Title edits do not change URLs automatically. |
| Metadata | Default title/description/social image from post and publication fields; reuse existing SEO/OG overrides. Correct absolute self-canonical, Open Graph and Twitter image URLs. |
| Structured data | `BlogPosting` with headline, image, selected public author, publisher, `datePublished`, `dateModified`, and canonical URL. Emit only fields that describe the visible page. |
| Discovery | Automatic sitemap from live publication records, including posts without collections; accurate public modification dates. RSS at `/feed.xml` with autodiscovery in the HTML head, stable item IDs, and canonical post links. |
| Indexing | Public posts indexable by default; previews excluded from indexing. Draft access requires authorization or a valid preview token; `noindex` is not access control. |
| Pagination | Real anchor links and addressable archive/tag pages. Each paginated page has its own canonical. Avoid infinite-scroll-only discovery and duplicate sort/filter URLs. |
| Status codes | Real 404 for missing/unpublished posts, permanent redirects for moved URLs, and 5xx on temporary upstream failures. No empty `200 OK` pages for errors. |
| Performance | Responsive cover images with explicit dimensions; lazy-load below-fold media; preserve current bundle budgets and caching. Do not load the editor into the public app. |
| Cache correctness | Include publication identity, host, locale, and archive page/tag in relevant keys. Publish/update/unpublish invalidates affected API/HTML/archive/tag/feed/sitemap entries; previews use `private, no-store`. Verify browser cache behavior too. |
| Operational setup | Reuse domain ownership/TLS onboarding, reject conflicting host claims, document Search Console verification and sitemap submission. |

Google supports `BlogPosting` and recommends applicable author, image, and date properties: [Article structured data](https://developers.google.com/search/docs/appearance/structured-data/article). Server-rendered content and meaningful HTTP responses align with [JavaScript SEO guidance](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics). `robots.txt` controls crawling and does not provide privacy or reliably prevent indexing: [robots.txt guidance](https://developers.google.com/search/docs/crawling-indexing/robots/intro).

## Smallest practical architecture

Keep the current Go API and `help-center/` public deployment. Select a help-center shell or blog shell after resolving the publication. Do not introduce another CMS, rendering service, or content editor.

1. **Space type:** add `SpaceTypeBlog = "blog"`. Keep `external_capable` meaning help center, with no second format discriminator or existing-space backfill. Explicitly opt blogs into shared editing, permissions, document creation, and the blog publishing path. Validate the third type consistently in API/frontend and Docs creation tools, including the `create_space` enum in `server/internal/service/mcp_catalog.go`. Type changes for already published spaces are outside the initial release.
2. **Publication configuration:** extend the current configuration model with a publication kind and optional `space_id`. Existing help-center rows stay workspace-scoped; blog rows are space-scoped. Replace workspace-wide uniqueness with partial unique indexes for one help center per workspace and one blog configuration per space, and validate kind/space consistency. Host lookup already uses this table; propagate the resolved kind/space rather than adding another host registry. Reuse branding/settings fields and preserve the shared domain namespace.
3. **Compatibility audit:** update `GetConfig(workspaceID)` and workspace-only config joins to explicitly select the help-center row. Preserve existing help-center URLs, defaults, previews, translations, and permissions. Audit positive type checks and `else`/non-external fallbacks, including `docs_helpcenter_translation_auto.go`, `docs_embedding.go`, `agent_knowledge_source.go`, `repository/docs_chunk.go`, `support_inbox_widget.go`, creation validators/tool schemas, imports, and frontend knowledge/help-center settings. Keep help-center-only paths exclusive; add explicit type guards where fallbacks could treat blogs as internal knowledge.
4. **Post metadata:** store draft cover/byline fields beside existing document metadata; reuse document tags and existing article SEO fields. Extend publication snapshots only for tags, cover (including alt text), and public byline. Existing title/slug/excerpt/content/SEO/OG snapshots already protect draft edits.
5. **Public reads:** resolve publication identity first, then enforce workspace + permitted space + live-publication status on every public query. Add bounded, paginated post-list queries; avoid loading all posts to build the homepage. Blog posts remain excluded from help-center search/navigation unless a future explicit cross-publication feature is added.
6. **Shared rendering:** reuse rich-text-to-public-HTML, asset URL, canonical URL, preview-token, domain, and cache utilities. Extract only the helpers that both formats actually need. Avoid a generic template/plugin framework.

For redirects, reuse the workspace-scoped slug alias table. Store the ID-qualified old blog post key so identical human-readable slugs in two blog spaces do not collide. Resolve the alias to a document, then verify that it belongs to the requested publication and is still public before redirecting. No new redirect storage or engine.

Start with one configured language per blog. Preserve the existing snapshot locale key so later translation support fits; do not build a second translation system or emit alternate-language links to unpublished pages.

## Delivery sequence

| Slice | Deliverable | Exit condition |
| --- | --- | --- |
| 0. Standalone help-center correction and audit | Preserve non-nil `PublicPublishedAt`; project snapshot `PublishedAt` as public modification time; map all type checks and close unsafe fallbacks | Help-center updates retain publication date and expose correct modification time; type inventory covers background work and tools; no blog UI/config changes |
| 1. Publication boundary | Third `blog` type, scoped config, partial indexes, existing host resolution, and explicit shared-path opt-ins | Existing help center and two blog spaces coexist without content/config/domain leakage; background exclusions hold |
| 2. Authoring and publication | Blog post list, metadata, public byline, preview, publish/update/unpublish | Draft changes never alter the live page; existing Docs collaboration still works |
| 3. Public blog and discovery | One list homepage, post/archive/tag views, hosted/custom domains, SEO, structured data, RSS and sitemap | A reader can discover and read posts without JavaScript; crawler and social-preview HTML is complete |
| 4. Release verification | Cache invalidation, URL redirects, mobile/accessibility checks, domain setup, help-center regressions | All acceptance scenarios below pass on the public serving path |

Land slice 0 as a standalone change before the configuration work in slice 1. Slices 1–4 together are the first blog release. SEO, feed correctness, and private-draft isolation remain part of shipping the feature.

Date semantics for slice 0: archive order and `datePublished` use the article's existing `PublicPublishedAt`; `dateModified` and sitemap `lastmod` use the live snapshot's `PublishedAt`. Unpublish currently clears `PublicPublishedAt` as an eligibility marker: preserve that behavior, so a later republish starts a new publication period. Do not add historical-date storage to this cut or claim to recover dates already overwritten.

## Acceptance scenarios

- One workspace serves its existing help center and two blogs; another workspace cannot read or claim their content or domains. Exercise direct lookup and preview endpoints, not just navigation.
- Publish a post without a collection: homepage, archive, tag page, sitemap, and RSS all discover it on the correct publication only. Help-center search must not return it.
- Edit content and public metadata: the live page stays unchanged until Update. Updating preserves first-publication date, archive order, and RSS item identity while advancing the public modification date.
- Rename the slug or activate the custom domain: old public URLs redirect, and HTML/feed/sitemap point at the selected canonical destination.
- Reuse the same human-readable slug in two blog spaces, rename both, and resolve their saved aliases: each redirects only inside its own publication; unpublished targets remain unavailable.
- Unpublish after warming every cache: public reads stop serving the post and discovery entries disappear within an explicitly tested cache propagation bound. Preview remains authorized and uncached.
- Fetch HTML without JavaScript: post body, crawlable links, canonical, social metadata, and valid structured data are present. Check missing posts and API outages for correct statuses.
- Existing help-center routes, custom domains, translations, previews, search, and publishing continue to work after the configuration migration.
- Run help-center auto-translation, embedding ingestion/reindexing, agent knowledge retrieval, and widget article discovery with published blog spaces present: none processes or returns blog content. Cover queued/direct entry points and non-external fallback branches, not just UI selection.
- For the standalone date fix, updating a live help-center article retains `PublicPublishedAt` while its snapshot advances; public date fields/SEO reflect both. Unpublish still removes public eligibility, and republish follows the documented new-period behavior.

## Deliberately deferred

- Native email subscriptions/delivery, paid memberships, paywalls, reader accounts, comments, likes, recommendations network, podcasts, and mobile apps.
- Reverse-proxy `/blog` hosting, blog-scoped search, featured posts, grid/layout toggles, typography choices, related posts, and external canonical overrides. Add only when customer demand warrants them.
- Blog translation, embeddings/agent knowledge ingestion, and support-widget integration.
- Scheduling, author archive pages, multi-author posts, theme marketplace, drag-and-drop website builder, arbitrary code injection, and multi-language blog management.
- A custom analytics platform or SEO scoring assistant. Reuse existing analytics only where there is already a suitable event/report path.

The next expansion, if needed, is email subscriptions integrated with Helpin contacts: publication-scoped consent/status, unsubscribe and suppression, verified sending identity, retry-safe delivery, and explicit web-only versus web-plus-email publishing. Existing CRM contacts must not become subscribers automatically. Until then, RSS and an optional configured newsletter link provide an honest reader-following path.
