# Help Center Multilingual Design

> Source review, 2026-09-17

Historical multilingual design. Locale routing and translation services now exist
in [locale helpers](../../help-center/src/lib/locale.ts) and
[translation service](../../server/internal/service/docs_helpcenter_translation.go).
The public application now uses [TanStack Start SSR](../../help-center/vite.config.ts),
not the earlier SPA-only delivery described by these plans. PublicID-based routes
and publication snapshots further evolved the original model. This source review
confirms those implementation boundaries, not every proposed acceptance item or
live locale configuration.

## Summary

Helpin should support one help center per workspace with multiple public language variants layered on top of the existing docs tree.

V1 multilingual support is strictly for the public help center. Internal docs remain single-language and continue to act as the source of truth.

The correct architecture is:

- one canonical internal docs tree
- one help center per workspace
- multiple locale-specific public translations for spaces, collections, and articles
- publish state per locale
- locale-aware public routing
- fallback to the default locale when a requested translation is missing

This keeps the internal authoring model simple while giving the public help center first-class multilingual behavior.

## Goals

- Support multiple languages in a single help center.
- Keep internal docs single-language in v1.
- Treat multilingual as a structured public content tree, not as loose article copies.
- Allow translated spaces, collections, and articles to publish independently per locale.
- Provide locale-aware public URLs and a language switcher.
- Track when translated content becomes stale relative to the source.

## Non-Goals

- Making the internal docs editor fully multilingual.
- Creating separate help centers per language.
- Auto-publishing machine translations.
- Building AI-assisted translation in v1.
- Mixing multiple languages in public search results by default.
- Replacing the current docs structure with duplicated locale-specific trees.

## Product Decision

### Scope boundary

V1 multilingual support belongs only to the public help center layer.

Internal docs continue to store:

- the source space tree
- the source collection tree
- the source article metadata
- the source article content

Public multilingual behavior is added as a translation overlay on top of those source records.

### Core model

The source docs tree stays canonical.

Public multilingual content is represented by locale-specific translation records for:

- external-capable spaces
- collections inside those spaces
- public articles

Default-locale translation rows should also exist, but they should be system-managed mirrors of the source content rather than separately edited copies.

This gives Helpin one uniform public read model across all locales.

## Why This Model

This model is the best fit for the current codebase because public help center behavior is already layered on top of the docs module:

- workspace-level help center config lives in [`server/internal/model/docs.go`](../../server/internal/model/docs.go)
- public article publish state currently lives in [`DocsHelpcenterArticle`](../../server/internal/model/docs.go)
- public reads already go through [`server/internal/repository/docs_helpcenter.go`](../../server/internal/repository/docs_helpcenter.go) and [`server/internal/service/docs_helpcenter.go`](../../server/internal/service/docs_helpcenter.go)

That makes multilingual a public-layer concern, not a reason to duplicate internal docs records.

## UX Design

### Help center settings

Help center settings should manage locale configuration.

Add the following settings to the help center configuration UI:

- default locale
- enabled locales
- show language switcher
- fallback to default locale

This keeps language policy centralized and matches the product expectation that multilingual is a help-center capability.

### Translation management

Do not make the main internal doc editor locale-tab based.

Instead, add a dedicated `Translations` panel to public-facing spaces, collections, and articles.

Each locale row should show:

- locale code and display name
- status
  - `missing`
  - `draft`
  - `published`
  - `needs_review`
- freshness relative to source
- whether the parent translations are ready

Primary actions:

- `Add translation`
- `Edit translation`
- `Publish`
- `Unpublish`
- `Mark reviewed`

This keeps the internal source editor clean and makes multilingual management feel intentional rather than bolted into every docs surface.

### Editing flow

The recommended v1 workflow is:

1. edit the source document in the existing docs editor
2. open the `Translations` panel for that public entity
3. create or edit a locale translation in a dedicated public translation workflow
4. publish that locale only when it is ready

Do not present source content and translations as equivalent peer tabs. The source document remains the authoritative content object.

### Parent-before-child publishing

Translated hierarchy should publish from the top down.

Rules:

- a translated space must exist before translated collections inside it can publish
- a translated collection must exist before translated articles inside it can publish
- an article translation cannot publish into a partially untranslated navigation path

This preserves public navigation consistency and avoids broken locale trees.

## Data Model

### Help center config

Extend [`DocsHelpcenterConfig`](../../server/internal/model/docs.go) with multilingual settings:

- `default_locale`
- `enabled_locales`
- `show_language_switcher`
- `fallback_to_default_locale`

The existing workspace-level help center config remains the correct owner for these settings.

### Translation tables

Add public translation overlay tables.

#### `docs_helpcenter_space_translations`

Fields:

- `id`
- `workspace_id`
- `space_id`
- `locale`
- `name`
- `slug`
- optional localized description
- `status`
- `published_at`
- `source_updated_at`
- `reviewed_at`
- `created_at`
- `updated_at`

Uniqueness:

- unique on `(space_id, locale)`
- unique public slug per `(workspace_id, locale, slug)`

#### `docs_helpcenter_collection_translations`

Fields:

- `id`
- `workspace_id`
- `space_id`
- `collection_id`
- `locale`
- `name`
- `description`
- `slug`
- `status`
- `published_at`
- `source_updated_at`
- `reviewed_at`
- `created_at`
- `updated_at`

Uniqueness:

- unique on `(collection_id, locale)`
- unique public slug per `(space_id, locale, slug)`

#### `docs_helpcenter_article_translations`

Fields:

- `id`
- `workspace_id`
- `space_id`
- `collection_id`
- `document_id`
- `locale`
- `title`
- `slug`
- `excerpt`
- `content` as TipTap JSON
- `content_text`
- `seo_title`
- `seo_description`
- `status`
- `published_at`
- `source_updated_at`
- `reviewed_at`
- `view_count`
- `helpful_count`
- `not_helpful_count`
- `created_at`
- `updated_at`

Uniqueness:

- unique on `(document_id, locale)`
- unique public slug per translated collection path and locale

### Editorial state

Use an explicit locale editorial state in v1:

- `draft`
- `published`
- `needs_review`

`missing` should be derived from the absence of a translation record, not stored as a state.

This is preferable to a separate `needs_review` boolean because it keeps the lifecycle readable and extensible.

### Default locale mirrors

Default-locale translation rows should be generated and maintained automatically from the source docs tree.

That means:

- the default-locale public translation exists for every externally published source entity
- public reads always come from translation tables, even for the default locale
- source edits refresh the default-locale translation row automatically

The default locale should not be edited as a separate manual translation.

## Routing Design

### V1 route shape

Use safe locale-prefixed public routes:

- `/:locale`
- `/:locale/:spaceSlug`
- `/:locale/:spaceSlug/:collectionSlug`
- `/:locale/:spaceSlug/:collectionSlug/:articleSlug`

This route structure is not the shortest possible final public URL shape, but it is the safest first release because it preserves deterministic resolution across locales and translated parents.

### Why keep `spaceSlug` in v1

Including `spaceSlug` in the public path:

- makes uniqueness rules simpler
- makes language switching deterministic
- matches the existing public information architecture more closely
- avoids relying on globally unique collection slugs too early

The path can be reconsidered later if Helpin wants shorter public URLs, but v1 should optimize for correctness.

### Legacy route compatibility

Existing non-locale public routes should redirect to the default locale equivalent.

For example:

- `/getting-started/install-widget` → `/en/help-center/getting-started/install-widget`

This preserves existing public links while establishing locale-first routing.

## Read Behavior

### Locale resolution

Public request behavior should be:

1. if the requested locale is not enabled, redirect to the default locale
2. if the requested locale translation exists and is `published`, serve it
3. if the requested locale translation is missing or not published and fallback is enabled, serve the default locale translation
4. if no eligible content exists, return not found

Fallback should be a read-time behavior, not a substitute for good translation management.

### Fallback visibility

If fallback is used, the public help center may optionally show a subtle note that the current page is being shown in the default language because the requested translation is unavailable.

This is optional for v1, but the internal admin UX must make missing translations visible regardless.

## Search

### V1 behavior

Public help-center search should be locale-scoped.

When browsing a locale:

- search that locale’s published article translations
- do not mix in other locales by default

If fallback search is supported later, it should be explicit and ranked below same-locale results.

### Implementation direction

V1 should use straightforward locale-scoped database search over translated article content.

Do not extend multilingual semantic search in the first release. That is a later phase once the translation model is stable.

## Source Change and Freshness

### Source edits

When the source space, collection, or document changes in a way that affects public output:

- refresh the default-locale translation mirror
- mark non-default locale translations as `needs_review`

This should happen automatically when source content or public-facing source metadata changes.

### Review workflow

Editors should be able to:

- open a `needs_review` translation
- update it
- mark it reviewed
- publish or re-publish it

V1 does not need a full side-by-side diff workflow, but the system should record enough source freshness metadata to tell whether a translation is stale.

## Publishing Rules

### Article publishing

An article translation can only publish when:

- the locale is enabled
- the translated space exists
- the translated collection exists if the article belongs to a collection
- the article translation itself is in a publishable state

### Unpublishing

A locale variant can be unpublished independently without affecting other locales.

Examples:

- English article can remain published
- French article can stay draft
- German article can be unpublished after review

This is a core requirement of serious multilingual help-center management.

## Backend Design

### Public read path

Update public help-center repository and service methods to read from translation overlays rather than directly from source docs records.

This includes:

- public homepage space listing
- space navigation
- collection pages
- article pages
- locale-aware slug resolution
- locale-aware search

Current public query code in [`server/internal/repository/docs_helpcenter.go`](../../server/internal/repository/docs_helpcenter.go) should become translation-aware rather than source-slug aware.

### Publish orchestration

Publishing should create or update locale translation rows instead of relying only on the current 1:1 [`DocsHelpcenterArticle`](../../server/internal/model/docs.go) extension.

The existing help-center article table can remain temporarily for migration compatibility, but slug, SEO, publish state, and metrics should move toward locale-specific translation ownership.

### Migration and backfill

Migration steps:

1. add multilingual config fields to help-center config
2. create translation overlay tables
3. backfill default-locale translation rows from current public content
4. migrate current public slug and publish state into default-locale translation records
5. switch public reads to the new translation model

This allows rollout without breaking existing public help centers.

## Frontend Design

### Settings

Update the help-center settings flow to manage locale policy:

- default locale selector
- enabled locale list
- switcher visibility
- fallback policy

### Docs management surfaces

Add `Translations` panels to public-facing:

- spaces
- collections
- articles

These panels should live in the docs/help-center management workflow, not as universal locale tabs in the general docs editor.

### Public help center

Add a language switcher to the public help center when multiple locales are enabled.

Behavior:

- if the current page exists in the target locale, switch to it
- if not, fall back to the nearest valid localized parent or the default locale

## Risks

### Route heaviness

The V1 locale route shape is safe but somewhat long because it includes the space slug.

This is an acceptable tradeoff for the first release.

### Translation UX complexity

If translation management is not explicit, editors will struggle to understand what is source content versus what is public translation content.

That is why the `Translations` panel model is required.

### Over-hidden fallback gaps

If fallback is too invisible, teams may assume translations exist when they do not.

Fallback should be resilient for end users, but the admin experience must make missing or stale translations obvious.

## Alternatives Considered

### Separate help center per language

Rejected because it would:

- duplicate navigation and publishing management
- complicate switching and search
- create sync problems between languages

### Separate duplicated docs trees per language

Rejected because it breaks canonical identity and makes freshness tracking much harder.

### Article-only translations

Rejected because it produces inconsistent localized navigation and does not support translated space or collection structure.

## Recommendation

Helpin should implement multilingual help docs as:

- one help center per workspace
- one canonical internal docs tree
- locale-specific public translation overlays
- default-locale translation rows as system-managed mirrors
- explicit per-locale editorial state
- locale-aware public routes
- locale-scoped public search
- a dedicated `Translations` management workflow rather than locale tabs in the source editor

This is the strongest foundation for a serious multilingual help-center product and leaves room for later AI-assisted translation without compromising editorial control.
