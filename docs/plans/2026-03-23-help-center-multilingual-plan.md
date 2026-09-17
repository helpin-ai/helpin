# Public Help Center Multilingual Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add production-grade multilingual support to the public Help Center so one Help Center can serve multiple languages with locale-aware URLs, translated content, manual language switching, browser-based locale selection, search, and SEO-safe alternate pages.

**Architecture:** Keep one Help Center per workspace/domain and add locale-aware content variants beneath the existing docs hierarchy. Use shared canonical objects for spaces, collections, and documents, then store per-locale public fields, content, SEO fields, slugs, and publication status in localization tables. Public rendering, search, redirects, and sitemap generation become locale-aware, while the authoring side gains translation management and status tracking.

**Tech Stack:** Go 1.24, GORM/PostgreSQL, existing docs/help-center services, React + TanStack Router, existing public Help Center SPA, existing Tiptap JSON renderer.

---

## 1. Current State

### Current codebase shape

- Help Center config is one row per workspace in [server/internal/model/docs.go](/root/teampulse/server/internal/model/docs.go).
- Public article publishing is one row per document in `docs_helpcenter_articles`, also in [server/internal/model/docs.go](/root/teampulse/server/internal/model/docs.go).
- Public routes are single-locale today:
  - `GET /hc/{subdomain}/config`
  - `GET /hc/{subdomain}/spaces`
  - `GET /hc/{subdomain}/spaces/{spaceSlug}/navigation`
  - `GET /hc/{subdomain}/spaces/{spaceSlug}/articles/{articleSlug}`
  - `GET /hc/{subdomain}/search`
  in [server/internal/handler/docs.go](/root/teampulse/server/internal/handler/docs.go).
- Public SPA routes also have no locale segment:
  - `/`
  - `/$spaceSlug`
  - `/$spaceSlug/$articleSlug`
  in [help-center/src/routes/index.tsx](/root/teampulse/help-center/src/routes/index.tsx), [help-center/src/routes/$spaceSlug.tsx](/root/teampulse/help-center/src/routes/$spaceSlug.tsx), and [help-center/src/routes/$spaceSlug/$articleSlug.tsx](/root/teampulse/help-center/src/routes/$spaceSlug/$articleSlug.tsx).
- Public search is hard-coded to Postgres `english` full-text search in [server/internal/repository/docs_search.go](/root/teampulse/server/internal/repository/docs_search.go).
- Public content rendering is locale-agnostic TipTap JSON -> HTML in [server/internal/service/docs_helpcenter.go](/root/teampulse/server/internal/service/docs_helpcenter.go) and [server/internal/tiptap/html.go](/root/teampulse/server/internal/tiptap/html.go).

### What this means

- The current model can only publish one public version of each article.
- Search, redirects, slugs, SEO fields, and counts are all single-locale.
- There is no place to store translation completeness, fallback rules, locale-specific nav labels, or locale-specific URLs.
- The public SPA cannot switch locale without changing the entire app contract.
- The public Help Center is a client-rendered Vite SPA today, not a server-rendered article site. That is already a weakness for article-level SEO and becomes more important once we add locale-aware canonical tags, alternate tags, and social metadata.

---

## 2. What Other Tools Do

### Strong common pattern

The stronger products converge on this model:

- One help center / knowledge base
- One information architecture shared across languages
- Per-locale translations for article content and nav labels
- Locale-aware URLs
- Manual language switcher
- Browser-language auto-selection
- Incomplete translation controls

### Tool patterns

- **Intercom**
  - Supports multiple languages in one Help Center.
  - Collections and articles have translated versions.
  - Browser language can auto-select the matching language.
  - Visitors can manually switch languages.
  - Untranslated articles are not simply auto-faked; English/default remains the fallback the user can switch to.
  - Sources:
    - https://www.intercom.com/help/en/articles/3107388-support-multiple-languages-in-your-help-center
    - https://www.intercom.com/help/en/articles/3107405-add-translations-to-public-articles

- **Zendesk**
  - Uses locale-aware URLs like `/hc/en-us/...`.
  - Categories/sections/articles all need localization support for translated content to appear correctly.
  - Treats article localization as part of a broader localized IA.
  - Sources:
    - https://support.zendesk.com/hc/en-us/articles/4408834328090-Localizing-help-center-content
    - https://support.zendesk.com/hc/en-us/articles/4707428787354-Understanding-web-crawler-locales

- **Freshdesk**
  - Uses one multilingual portal with a primary language and supported languages.
  - Languages can be enabled but hidden until content is ready.
  - Uses a master/default article and translated versions for other locales.
  - Browser/profile language detection is supported.
  - Sources:
    - https://support.freshdesk.com/support/solutions/articles/219745-setting-up-a-multilingual-knowledge-base
    - https://support.freshdesk.com/support/solutions/articles/50000000585-about-freshdesk-knowledge-base

- **Document360**
  - Supports multiple languages inside one knowledge base.
  - Supports hidden/incomplete locales and default-language fallback behavior.
  - Can also run language-specific KBs, but the single-project multilingual model is their main localization story.
  - Sources:
    - https://docs.document360.com/docs/setting-up-a-multi-lingual-knowledge-base
    - https://docs.document360.com/docs/getting-started-with-multi-lingual-knowledge-base

- **Help Scout**
  - More site-oriented than translation-native.
  - Supports translating labels and article contents, but also explicitly supports separate Docs sites per language and third-party dynamic translation integrations like Weglot.
  - This is useful context, but not the best core model for Helpin because it duplicates site structure and weakens shared management.
  - Sources:
    - https://docs.helpscout.com/article/302-translate-docs-words-and-phrases
    - https://docs.helpscout.com/article/229-multiple-docs-sites

### Best-fit conclusion for Helpin

The best model for Helpin is:

- **one Help Center per workspace/domain**
- **shared structure**
- **per-locale content variants**
- **locale in URL path**
- **manual switcher + browser detection**
- **hide incomplete locales from public navigation/search**

This is closer to Zendesk / Intercom / Freshdesk / Document360 than to Help Scout’s multi-site approach.

---

## 3. Recommended Product Model

### Recommendation

Build multilingual Help Center as:

- a **single public Help Center**
- with a **default locale**
- and **supported locales**
- where spaces, collections, and documents keep one canonical identity
- and each public-facing language variant is stored as a localization record

### Why this is the best shape

- Avoids cloning the same site per language
- Preserves one article identity across translations
- Makes language switching deterministic
- Supports proper `hreflang`, sitemaps, and SEO-safe indexing
- Lets search, analytics, and redirects reason about article equivalence across locales
- Keeps future import and translation tooling clean

### Explicit non-recommendation

Do **not** build this as:

- separate workspace-level Help Center configs per language
- separate duplicated spaces/collections/documents per language
- client-side machine translation of one canonical article

Those approaches create drift, weak SEO, poor authoring ergonomics, and messy search behavior.

---

## 4. Core Product Decisions

### 4.1 Locale URL strategy

Use locale-prefixed URLs:

- Home: `/{locale}`
- Space: `/{locale}/{spaceSlug}`
- Article: `/{locale}/{spaceSlug}/{articleSlug}`
- Canonical collection/article path: `/{locale}/{collectionSlug}/{articleSlug}`

Recommendation:

- locale in the first path segment
- subdomain/custom domain still identifies the Help Center
- locale path identifies the translated variant

Why:

- SEO-safe
- easy to crawl
- easy to add `hreflang`
- easy to redirect
- avoids locale ambiguity in current single-locale routes

### 4.2 Locale identity

Use BCP 47-style locale codes, normalized to a controlled list:

- examples: `en`, `en-US`, `fr`, `de`, `pt-BR`

Recommendation:

- store both `locale_code` and `base_language`
- treat `en` and `en-US` as distinct supported locales if explicitly configured
- default locale is required

### 4.3 Translation visibility

Each supported locale should have:

- `enabled` in admin
- `public_visible`
- optional `hidden_until_ready`

Recommendation:

- locales can be configured before launch
- only `public_visible` locales appear in the switcher and public crawlable routes

### 4.4 Missing translation behavior

Recommended public behavior:

- search and navigation should only show published content for the requested locale
- if a direct locale URL is requested for an article with no translation:
  - return a **302 redirect to the default-locale equivalent URL**
  - do not render default-language content at the untranslated locale URL

Why:

- avoids indexing wrong-language content at locale-specific URLs
- keeps search results and `hreflang` clean
- simpler than serving fallback content plus banners on localized paths

### 4.5 Language detection

Recommended behavior:

- root path without locale should act as `x-default`
- initial visit chooses best locale by:
  1. explicit user choice cookie
  2. stored locale preference
  3. `Accept-Language`
  4. default locale
- redirect should be **302**, not 301

### 4.6 Language switching

Add a language switcher in the public header.

Behavior:

- on article page, switch to the matching translated article if it exists
- otherwise switch to the default-locale equivalent
- preserve current article identity when possible

---

## 5. Data Model Plan

## 5.1 Keep canonical objects

Keep:

- `docs_spaces`
- `docs_collections`
- `docs_documents`

These remain the canonical content graph and permission/container model.

### 5.2 Add locale registry to Help Center config

Add a locale settings table, for example:

- `docs_helpcenter_locales`

Suggested fields:

- `id`
- `workspace_id`
- `locale_code`
- `is_default`
- `is_enabled`
- `is_public_visible`
- `position`
- `autodetect_enabled`
- `created_at`
- `updated_at`

This should define which locales exist for the Help Center.

### 5.3 Localize Help Center config content

Add:

- `docs_helpcenter_config_localizations`

Suggested fields:

- `helpcenter_config_id`
- `locale_code`
- `seo_title`
- `seo_description`
- `search_placeholder`
- `homepage_config`
- `header_links`
- `footer_config`
- `created_at`
- `updated_at`

Keep global:

- domain/subdomain
- logos
- colors
- favicon
- theme mode
- support email

Rationale:

- brand identity is global
- textual chrome and SEO copy are locale-specific

### 5.4 Localize spaces

Add:

- `docs_space_localizations`

Suggested fields:

- `space_id`
- `locale_code`
- `name`
- `slug`
- `description` if introduced later
- `created_at`
- `updated_at`

Recommendation:

- localize `name`
- keep `slug` optionally localizable
- for first rollout, space slugs can remain stable if you want lower risk

Best recommendation:

- keep `space.slug` stable in v1
- localize only `name`

Reason:

- reduces cascading route complexity
- spaces are broad containers, not the main SEO target

### 5.5 Localize collections

Add:

- `docs_collection_localizations`

Suggested fields:

- `collection_id`
- `locale_code`
- `name`
- `description`
- `slug`
- `created_at`
- `updated_at`

Recommendation:

- localize `name`
- localize `slug`

Reason:

- collection names are prominent in navigation and SEO
- collection-level localized slugs are useful and manageable

### 5.6 Localize documents and public article metadata

Add one main table, for example:

- `docs_document_localizations`

Suggested fields:

- `document_id`
- `locale_code`
- `title`
- `excerpt`
- `icon`
- `content` JSONB
- `content_text`
- `word_count`
- `status` (`draft`, `published`, `archived`)
- `translation_status` (`missing`, `draft`, `published`, `outdated`)
- `slug`
- `seo_title`
- `seo_description`
- `published_at`
- `source_locale_code`
- `source_version_id`
- `last_synced_from_source_at`
- `created_at`
- `updated_at`

Recommendation:

- move all locale-sensitive public article fields here
- treat this as the source of truth for public article variants

### 5.7 Versions

Add locale awareness to versions.

Either:

- add `locale_code` to `docs_versions`

or:

- add `docs_document_localization_versions`

Recommendation:

- add `locale_code` to `docs_versions`
- keep version snapshots per localized content variant

### 5.8 Redirects

Extend redirects to support locale.

Current `docs_redirects` is locale-agnostic. Add:

- `source_locale_code`
- `target_locale_code`

or store locale-prefixed paths directly.

Recommendation:

- store fully qualified locale-prefixed source path
- store target locale explicitly

This keeps imported legacy paths and future slug changes unambiguous.

---

## 6. Public Routing and API Plan

### 6.1 Public SPA routes

Add locale-aware route tree in `help-center`:

- `/$locale`
- `/$locale/$spaceSlug`
- `/$locale/$spaceSlug/$articleSlug`

Keep `/` as locale resolver only.

### 6.2 Public API

Update public API to accept locale explicitly:

- `GET /hc/{subdomain}/config?locale=fr`
- `GET /hc/{subdomain}/spaces?locale=fr`
- `GET /hc/{subdomain}/spaces/{spaceSlug}/navigation?locale=fr`
- `GET /hc/{subdomain}/spaces/{spaceSlug}/articles/{articleSlug}?locale=fr`
- `GET /hc/{subdomain}/collections/{collectionSlug}/articles/{articleSlug}?locale=fr`
- `GET /hc/{subdomain}/search?q=...&locale=fr`

Alternative:

- put locale in API path

Recommendation:

- keep locale in query or header on the API
- keep locale in path on the public SPA

This is a cleaner separation and avoids multiplying handler paths.

### 6.3 Locale resolution API

Add a lightweight endpoint or embed resolution in config response:

- supported locales
- default locale
- locale visibility
- locale labels

The SPA needs this before it can resolve `/`.

---

## 7. Search Plan

### Current weakness

Current search uses Postgres `english` tsvector in [server/internal/repository/docs_search.go](/root/teampulse/server/internal/repository/docs_search.go). That is not viable for a multilingual Help Center.

### Recommendation

Search should operate on localized rows, not canonical rows.

For each document localization:

- store `content_text`
- store `search_locale_code`
- optionally store a precomputed `search_vector`

### Search strategy

Recommended:

- locale-specific search over `docs_document_localizations`
- use locale-specific Postgres text search config where available
- fall back to `simple` for unsupported locales

Example mapping:

- `en` -> `english`
- `fr` -> `french`
- `de` -> `german`
- `es` -> `spanish`
- `pt` / `pt-BR` -> `portuguese`
- unsupported -> `simple`

### Public behavior

- search only published translations in the requested locale
- do not leak default-locale results into another locale’s search by default
- optional admin toggle later for “include default-language fallback results” if desired

### Widget/search parity

If the support widget or in-product help search surfaces public articles, locale must be part of that contract too.

---

## 8. SEO and Indexing Plan

### Recommendation

Use localized URLs plus `hreflang`.

Required:

- per-locale absolute canonical URL
- alternate `hreflang` links for every localized variant
- `x-default` for the root selector/fallback experience
- locale-aware sitemap output

### Important implementation note

The current public Help Center is a client-rendered SPA. If multilingual SEO is important, the strongest implementation is to move public article routes to one of these:

- server-rendered HTML
- static pre-rendered HTML
- hybrid rendering for article and collection pages only

Why:

- crawlers get locale-specific `<title>`, meta description, canonical, and `hreflang` in the initial HTML
- social/link previews are more reliable
- locale-specific alternate links are easier to guarantee

Recommendation:

- keep the editor/admin SPA as-is
- keep the public Help Center app if you want
- but add a backend-rendered or pre-rendered article shell for public article/collection/home routes before calling multilingual SEO “done”

If that is too much for v1, multilingual content can still ship on the SPA first, but the plan should treat server-rendered article HTML as the target end state, not an optional afterthought

Google guidance:

- localized pages should explicitly reference alternate language versions
- every localized page should include itself and all alternates
- `x-default` should exist for unmatched users

Source:

- https://developers.google.com/search/docs/advanced/crawling/localized-versions

### Canonical strategy

Each localized page should have:

- canonical = itself
- alternates = all published locales of the same article
- `x-default` = default-locale or locale selector root

### Important rule

Do not serve English content at a French URL and then mark it as `hreflang="fr"`.

If a translation does not exist:

- redirect to default-locale URL
- do not pretend the untranslated locale page exists

### Sitemap plan

Add locale-aware sitemap generation:

- `sitemap.xml`
- optionally split by locale or section if large

Each entry should include alternate language references.

---

## 9. Authoring and Admin UX Plan

### 9.1 Help Center settings

Add a multilingual settings area:

- default locale
- supported locales
- per-locale visibility
- browser auto-detect on/off
- language switcher on/off

### 9.2 Space and collection editing

Add translation management for public-facing names:

- translated names
- translated descriptions
- translated slugs where supported

### 9.3 Document editor

Recommended editor model:

- canonical document stays the same
- user chooses locale in a translation switcher/tab
- each locale has its own title, excerpt, SEO, content, publish state

### 9.4 Translation status

Track and display:

- missing
- draft
- published
- outdated

### 9.5 Outdated translation tracking

When the default-locale content changes:

- other localized versions become `outdated`

Track this via:

- `source_version_id`
- or `source_updated_at` snapshot

This is important. Mature systems surface staleness, not just existence.

### 9.6 Publish model

Recommended:

- canonical doc can remain internally published/active
- each locale has its own public publish state

That avoids forcing all locales to publish at once.

---

## 10. Rendering and Frontend Plan

### 10.1 Help Center app

The public SPA needs:

- locale route param
- locale in DocsContext
- locale-aware queries
- header language switcher
- root locale resolver page

### 10.2 Localized chrome

Localize:

- search placeholder
- homepage hero copy
- header link labels
- footer link labels
- empty states
- common article UI strings if needed

Recommendation:

- public app shell UI strings should come from app i18n resources
- workspace-authored textual chrome should come from localized Help Center config rows

These are different concerns and should stay separate.

### 10.3 Article/collection switching

When the visitor changes language:

- if equivalent translation exists, navigate to it
- if not, navigate to the default-locale equivalent

Do not dump them at the homepage unless there is no valid counterpart.

---

## 11. Analytics, Feedback, and Metrics

### Recommendation

Make metrics locale-aware.

For articles:

- store view/helpful/not-helpful counts per localization, not only per canonical article

Reason:

- translation quality varies
- search and usefulness vary by locale
- otherwise French and English feedback become mixed

If you want aggregated reporting later, compute rollups from localized records.

---

## 12. Import and Migration Implications

### Existing documents

Current docs are effectively single-locale.

Migration rule:

- all existing public docs become the default locale translation

That means:

- existing `docs_documents` rows become canonical parents
- existing public article fields move into default-locale localization rows
- existing `docs_contents.content` becomes default-locale localized content
- existing slugs become default-locale slugs

### Imported help centers

For future imports:

- every imported article should land in a known locale
- if source system supports multiple locales, import them as sibling localizations under the same canonical article identity when possible

### Redirect migration

Old public URLs should redirect to the new default-locale prefixed URLs.

Examples:

- `/billing/refunds` -> `/en/billing/refunds`
- `/collection/article` -> `/en/collection/article`

These redirects are mandatory to avoid breaking existing published links.

---

## 13. Risks and Failure Modes

### High-risk areas

- trying to bolt locale onto the current single-row article model
- mixing fallback content with locale-specific indexed pages
- keeping English-only search while exposing multiple languages
- duplicating whole documents per language instead of using localization rows
- forgetting locale-aware redirects and `hreflang`
- not tracking outdated translations

### Specific failure cases to avoid

- French URL serves English article and gets indexed as French
- manual switcher lands on wrong article or homepage too often
- search returns English content in French without telling the user
- locale slugs collide within the same language
- untranslated navigation labels break space/collection browsing
- default-locale content updates silently desynchronize translations

---

## 14. Recommended Rollout Order

### Phase 1: Data model and backend contract

- add locale registry
- add localization tables
- migrate existing public docs into default-locale localizations
- update repositories/services to read/write localized content

### Phase 2: Public API and route contract

- add locale-aware public endpoints
- add locale resolver behavior
- add redirect support from old non-locale URLs

### Phase 3: Help Center frontend

- add locale-prefixed routes
- add locale context and switcher
- add localized config loading
- add translated nav/article fetching

### Phase 4: Search and SEO

- locale-aware search index/query
- `hreflang`
- locale-aware sitemap
- canonical and alternate tags
- decide and execute public article rendering strategy for SEO:
  - server-rendered or pre-rendered preferred
  - client-only acceptable only as an interim step

### Phase 5: Authoring/admin UX

- supported locales settings
- translation tabs/status
- per-locale publish flows
- outdated translation indicators

### Phase 6: Migration hardening

- backfill redirects
- QA on crawling/indexing
- analytics/feedback locale split

---

## 15. Concrete Recommendation

If we want the best long-term shape for Helpin, we should build multilingual Help Center as:

- one Help Center per workspace/domain
- one shared canonical content tree
- per-locale localized rows for spaces, collections, documents, and Help Center config copy
- locale-prefixed public URLs
- browser detection plus manual switching
- locale-specific search
- strict SEO with `hreflang`, `x-default`, and locale-aware sitemap
- per-locale publication status and outdated translation tracking

This is the strongest blend of:

- Zendesk’s locale-aware routing
- Intercom’s shared IA and language switching
- Freshdesk’s primary-language plus hidden-locale workflow
- Document360’s multilingual project model

and avoids Help Scout’s weaker “clone more sites” shape.

---

## 16. Critical Decisions To Confirm Before Implementation

1. Whether article slugs should be localized per language.
   Recommendation: yes.

2. Whether space slugs should be localized in v1.
   Recommendation: no, keep space slug stable in v1.

3. Whether untranslated direct locale URLs should redirect to default or render default with banner.
   Recommendation: redirect to default-locale URL.

4. Whether public analytics/feedback counts should be per localization.
   Recommendation: yes.

5. Whether machine translation is in scope for v1.
   Recommendation: no, support human-authored localizations first and add translation assist later.

6. Whether widget/help-search surfaces should participate in locale-aware retrieval in the same rollout.
   Recommendation: yes for read-path parity if public articles appear there.

---

## Research Sources

- Intercom: multilingual Help Center and article translations
  - https://www.intercom.com/help/en/articles/3107388-support-multiple-languages-in-your-help-center
  - https://www.intercom.com/help/en/articles/3107405-add-translations-to-public-articles
- Zendesk: localized Help Center content and locale URLs
  - https://support.zendesk.com/hc/en-us/articles/4408834328090-Localizing-help-center-content
  - https://support.zendesk.com/hc/en-us/articles/4707428787354-Understanding-web-crawler-locales
- Freshdesk: multilingual KB with primary language and hidden locales
  - https://support.freshdesk.com/support/solutions/articles/219745-setting-up-a-multilingual-knowledge-base
  - https://support.freshdesk.com/support/solutions/articles/50000000585-about-freshdesk-knowledge-base
- Document360: multilingual single-project KB and fallback/visibility behavior
  - https://docs.document360.com/docs/setting-up-a-multi-lingual-knowledge-base
  - https://docs.document360.com/docs/getting-started-with-multi-lingual-knowledge-base
- Help Scout: separate Docs sites and translation tooling
  - https://docs.helpscout.com/article/302-translate-docs-words-and-phrases
  - https://docs.helpscout.com/article/229-multiple-docs-sites
- Google Search Central: localized pages and `hreflang`
  - https://developers.google.com/search/docs/advanced/crawling/localized-versions
