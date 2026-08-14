import type { NavItem, Space } from '@/lib/types'
import { buildArticleKey } from '@/lib/articleKey'
import { buildCollectionKey } from '@/lib/collectionKey'

export type LocaleRouteKind = 'home' | 'space' | 'collection' | 'article' | 'api_reference' | 'search'

export interface LocaleRouteState {
  kind: LocaleRouteKind
  spaceId?: string
  collectionId?: string
  articleId?: string
  apiReferenceSlug?: string
  searchQuery?: string
}

interface ResolveActiveLocaleOptions {
  paramsLocale?: string
  paramsSpaceSlug?: string
  enabledLocales?: string[]
  defaultLocale: string
}

interface ResolveLocaleSwitchPathOptions {
  multilingualEnabled: boolean
  targetLocale: string
  defaultLocale: string
  current: LocaleRouteState
  targetSpaces: Space[]
  targetNavigation?: NavItem[]
  fallbackSpaces?: Space[]
  fallbackNavigation?: NavItem[]
}

interface ResolveExactLocalePathOptions {
  multilingualEnabled: boolean
  targetLocale: string
  current: LocaleRouteState
  targetSpaces: Space[]
  targetNavigation?: NavItem[]
}

export function isMultilingualEnabled(enabledLocales: string[] = []) {
  return new Set(
    enabledLocales
      .map((locale) => locale.trim().toLowerCase())
      .filter(Boolean),
  ).size > 1
}

export function buildLocaleHomePath(locale: string) {
  return `/${locale}`
}

export function buildLocaleSpacePath(locale: string, spaceSlug: string) {
  return `/${locale}/${spaceSlug}`
}

export function buildLocaleCollectionPath(
  locale: string,
  collectionSlug: string,
  publicId = '',
) {
  return `/${locale}/c/${buildCollectionKey(collectionSlug, publicId)}`
}

export function buildLocaleArticlePath(
  locale: string,
  articleSlug: string,
  publicId: string,
) {
  return `/${locale}/articles/${buildArticleKey(articleSlug, publicId)}`
}

export function buildLocaleSearchPath(
  locale: string,
  searchQuery?: string,
  spaceSlug?: string,
) {
  const params = new URLSearchParams()
  if (searchQuery) params.set('q', searchQuery)
  if (spaceSlug) params.set('space', spaceSlug)
  const queryString = params.toString()
  return queryString ? `/${locale}/search?${queryString}` : `/${locale}/search`
}

export function buildCanonicalHomePath(
  multilingualEnabled: boolean,
  locale: string,
) {
  return multilingualEnabled ? buildLocaleHomePath(locale) : '/'
}

export function buildCanonicalCollectionPath(
  multilingualEnabled: boolean,
  locale: string,
  collectionSlug: string,
  publicId = '',
) {
  return multilingualEnabled
    ? buildLocaleCollectionPath(locale, collectionSlug, publicId)
    : `/c/${buildCollectionKey(collectionSlug, publicId)}`
}

export function buildCanonicalSpacePath(
  multilingualEnabled: boolean,
  locale: string,
  spaceSlug: string,
) {
  return multilingualEnabled
    ? buildLocaleSpacePath(locale, spaceSlug)
    : `/${spaceSlug}`
}

export function buildCanonicalAPIReferencePath(
  multilingualEnabled: boolean,
  locale: string,
  spaceSlug: string,
  referenceSlug: string,
) {
  const path = `/${spaceSlug}/api/${referenceSlug}`
  return multilingualEnabled ? `/${locale}${path}` : path
}

export function buildCanonicalArticlePath(
  multilingualEnabled: boolean,
  locale: string,
  articleSlug: string,
  publicId: string,
) {
  return multilingualEnabled
    ? buildLocaleArticlePath(locale, articleSlug, publicId)
    : `/articles/${buildArticleKey(articleSlug, publicId)}`
}

export function buildCanonicalSearchPath(
  multilingualEnabled: boolean,
  locale: string,
  searchQuery?: string,
  spaceSlug?: string,
) {
  if (multilingualEnabled) {
    return buildLocaleSearchPath(locale, searchQuery, spaceSlug)
  }
  const params = new URLSearchParams()
  if (searchQuery) params.set('q', searchQuery)
  if (spaceSlug) params.set('space', spaceSlug)
  const queryString = params.toString()
  return queryString ? `/search?${queryString}` : '/search'
}

export function resolveActiveLocale({
  paramsLocale,
  paramsSpaceSlug,
  enabledLocales = [],
  defaultLocale,
}: ResolveActiveLocaleOptions) {
  const normalizedDefault = (defaultLocale || 'en').toLowerCase()
  const normalizedEnabledLocales = enabledLocales.map((locale) =>
    locale.trim().toLowerCase(),
  )
  const localeParam = paramsLocale?.trim().toLowerCase()
  if (
    localeParam &&
    normalizedEnabledLocales.some((locale) => locale === localeParam)
  ) {
    return localeParam
  }

  const spaceSlug = paramsSpaceSlug?.trim().toLowerCase()
  if (
    spaceSlug &&
    normalizedEnabledLocales.some((locale) => locale === spaceSlug)
  ) {
    return spaceSlug
  }

  return normalizedDefault
}

function findSpaceByID(spaces: Space[], spaceID?: string) {
  if (!spaceID) return undefined
  return spaces.find((space) => space.id === spaceID)
}

function findCollectionByID(navigation: NavItem[] = [], collectionID?: string) {
  if (!collectionID) return undefined
  return navigation.find((collection) => collection.id === collectionID)
}

function findArticleByID(navigation: NavItem[] = [], articleID?: string) {
  if (!articleID) return undefined
  for (const collection of navigation) {
    const article = collection.articles.find((candidate) => candidate.id === articleID)
    if (article) {
      return { collection, article }
    }
  }
  return undefined
}

export function resolveLocaleSwitchPath({
  multilingualEnabled,
  targetLocale,
  defaultLocale,
  current,
  targetSpaces,
  targetNavigation = [],
  fallbackSpaces = [],
  fallbackNavigation = [],
}: ResolveLocaleSwitchPathOptions) {
  switch (current.kind) {
    case 'home':
      return buildCanonicalHomePath(multilingualEnabled, targetLocale)
    case 'search':
      return buildCanonicalSearchPath(
        multilingualEnabled,
        targetLocale,
        current.searchQuery,
        findSpaceByID(targetSpaces, current.spaceId)?.slug ??
          findSpaceByID(fallbackSpaces, current.spaceId)?.slug,
      )
    case 'space': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      if (targetSpace) {
        return buildCanonicalSpacePath(
          multilingualEnabled,
          targetLocale,
          targetSpace.slug,
        )
      }
      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      return fallbackSpace
        ? buildCanonicalSpacePath(
            multilingualEnabled,
            defaultLocale,
            fallbackSpace.slug,
          )
        : buildCanonicalHomePath(multilingualEnabled, defaultLocale)
    }
    case 'api_reference': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      if (targetSpace && current.apiReferenceSlug) {
        return buildCanonicalAPIReferencePath(
          multilingualEnabled,
          targetLocale,
          targetSpace.slug,
          current.apiReferenceSlug,
        )
      }
      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      return fallbackSpace && current.apiReferenceSlug
        ? buildCanonicalAPIReferencePath(
            multilingualEnabled,
            defaultLocale,
            fallbackSpace.slug,
            current.apiReferenceSlug,
          )
        : buildCanonicalHomePath(multilingualEnabled, defaultLocale)
    }
    case 'collection': {
      const targetCollection = findCollectionByID(targetNavigation, current.collectionId)
      if (targetCollection) {
        return buildCanonicalCollectionPath(
          multilingualEnabled,
          targetLocale,
          targetCollection.slug,
          targetCollection.public_id,
        )
      }

      const fallbackCollection = findCollectionByID(
        fallbackNavigation,
        current.collectionId,
      )
      if (fallbackCollection) {
        return buildCanonicalCollectionPath(
          multilingualEnabled,
          defaultLocale,
          fallbackCollection.slug,
          fallbackCollection.public_id,
        )
      }

      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      if (fallbackSpace) {
        return buildCanonicalSpacePath(
          multilingualEnabled,
          defaultLocale,
          fallbackSpace.slug,
        )
      }

      return buildCanonicalHomePath(multilingualEnabled, defaultLocale)
    }
    case 'article': {
      const targetArticle = findArticleByID(targetNavigation, current.articleId)
      if (targetArticle) {
        return buildCanonicalArticlePath(
          multilingualEnabled,
          targetLocale,
          targetArticle.article.slug,
          targetArticle.article.public_id,
        )
      }

      const fallbackArticle = findArticleByID(fallbackNavigation, current.articleId)
      if (fallbackArticle) {
        return buildCanonicalArticlePath(
          multilingualEnabled,
          defaultLocale,
          fallbackArticle.article.slug,
          fallbackArticle.article.public_id,
        )
      }

      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      if (fallbackSpace) {
        return buildCanonicalSpacePath(
          multilingualEnabled,
          defaultLocale,
          fallbackSpace.slug,
        )
      }

      return buildCanonicalHomePath(multilingualEnabled, defaultLocale)
    }
    default:
      return buildCanonicalHomePath(multilingualEnabled, defaultLocale)
  }
}

export function resolveExactLocalePath({
  multilingualEnabled,
  targetLocale,
  current,
  targetSpaces,
  targetNavigation = [],
}: ResolveExactLocalePathOptions) {
  switch (current.kind) {
    case 'home':
      return buildCanonicalHomePath(multilingualEnabled, targetLocale)
    case 'search':
      return buildCanonicalSearchPath(
        multilingualEnabled,
        targetLocale,
        current.searchQuery,
        findSpaceByID(targetSpaces, current.spaceId)?.slug,
      )
    case 'space': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      return targetSpace
        ? buildCanonicalSpacePath(
            multilingualEnabled,
            targetLocale,
            targetSpace.slug,
          )
        : null
    }
    case 'api_reference': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      return targetSpace && current.apiReferenceSlug
        ? buildCanonicalAPIReferencePath(
            multilingualEnabled,
            targetLocale,
            targetSpace.slug,
            current.apiReferenceSlug,
          )
        : null
    }
    case 'collection': {
      const targetCollection = findCollectionByID(
        targetNavigation,
        current.collectionId,
      )
      return targetCollection
        ? buildCanonicalCollectionPath(
            multilingualEnabled,
            targetLocale,
            targetCollection.slug,
            targetCollection.public_id,
          )
        : null
    }
    case 'article': {
      const targetArticle = findArticleByID(targetNavigation, current.articleId)
      return targetArticle
        ? buildCanonicalArticlePath(
            multilingualEnabled,
            targetLocale,
            targetArticle.article.slug,
            targetArticle.article.public_id,
          )
        : null
    }
    default:
      return null
  }
}
