import type { NavItem, Space } from '@/lib/types'

export type LocaleRouteKind = 'home' | 'space' | 'collection' | 'article' | 'search'

export interface LocaleRouteState {
  kind: LocaleRouteKind
  spaceId?: string
  collectionId?: string
  articleId?: string
  searchQuery?: string
}

interface ResolveLocaleSwitchPathOptions {
  targetLocale: string
  defaultLocale: string
  current: LocaleRouteState
  targetSpaces: Space[]
  targetNavigation?: NavItem[]
  fallbackSpaces?: Space[]
  fallbackNavigation?: NavItem[]
}

export function buildLocaleHomePath(locale: string) {
  return `/${locale}`
}

export function buildLocaleSpacePath(locale: string, spaceSlug: string) {
  return `/${locale}/${spaceSlug}`
}

export function buildLocaleCollectionPath(
  locale: string,
  spaceSlug: string,
  collectionSlug: string,
) {
  return `/${locale}/${spaceSlug}/${collectionSlug}`
}

export function buildLocaleArticlePath(
  locale: string,
  spaceSlug: string,
  collectionSlug: string,
  articleSlug: string,
) {
  return `/${locale}/${spaceSlug}/${collectionSlug}/${articleSlug}`
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
      return buildLocaleHomePath(targetLocale)
    case 'search':
      return buildLocaleSearchPath(
        targetLocale,
        current.searchQuery,
        findSpaceByID(targetSpaces, current.spaceId)?.slug ??
          findSpaceByID(fallbackSpaces, current.spaceId)?.slug,
      )
    case 'space': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      if (targetSpace) {
        return buildLocaleSpacePath(targetLocale, targetSpace.slug)
      }
      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      return fallbackSpace
        ? buildLocaleSpacePath(defaultLocale, fallbackSpace.slug)
        : buildLocaleHomePath(defaultLocale)
    }
    case 'collection': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      const targetCollection = findCollectionByID(targetNavigation, current.collectionId)
      if (targetSpace && targetCollection) {
        return buildLocaleCollectionPath(targetLocale, targetSpace.slug, targetCollection.slug)
      }

      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      const fallbackCollection = findCollectionByID(
        fallbackNavigation,
        current.collectionId,
      )
      if (fallbackSpace && fallbackCollection) {
        return buildLocaleCollectionPath(
          defaultLocale,
          fallbackSpace.slug,
          fallbackCollection.slug,
        )
      }

      if (fallbackSpace) {
        return buildLocaleSpacePath(defaultLocale, fallbackSpace.slug)
      }

      return buildLocaleHomePath(defaultLocale)
    }
    case 'article': {
      const targetSpace = findSpaceByID(targetSpaces, current.spaceId)
      const targetArticle = findArticleByID(targetNavigation, current.articleId)
      if (targetSpace && targetArticle) {
        return buildLocaleArticlePath(
          targetLocale,
          targetSpace.slug,
          targetArticle.collection.slug,
          targetArticle.article.slug,
        )
      }

      const fallbackSpace = findSpaceByID(fallbackSpaces, current.spaceId)
      const fallbackArticle = findArticleByID(fallbackNavigation, current.articleId)
      if (fallbackSpace && fallbackArticle) {
        return buildLocaleArticlePath(
          defaultLocale,
          fallbackSpace.slug,
          fallbackArticle.collection.slug,
          fallbackArticle.article.slug,
        )
      }

      if (fallbackSpace) {
        return buildLocaleSpacePath(defaultLocale, fallbackSpace.slug)
      }

      return buildLocaleHomePath(defaultLocale)
    }
    default:
      return buildLocaleHomePath(defaultLocale)
  }
}
