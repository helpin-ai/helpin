import type { QueryClient } from '@tanstack/react-query'
import {
  articleQueryOptions,
  collectionQueryOptions,
  previewArticleQueryOptions,
  searchArticlesQueryOptions,
  spaceNavigationQueryOptions,
} from '@/hooks/queries'
import type { RootRouteData } from '@/lib/rootLoader'

/**
 * Loads the sidebar tree for a space. On the server it is awaited so the
 * sidebar ships in the rendered HTML instead of popping in after hydration.
 * In the browser it only warms the cache: client navigations must not wait
 * on it, and the tree is usually cached from the first page already.
 */
async function prefetchSpaceNavigation(
  queryClient: QueryClient,
  rootData: RootRouteData,
  spaceSlug: string | undefined,
) {
  if (!spaceSlug) return
  const prefetch = queryClient.prefetchQuery(
    spaceNavigationQueryOptions(
      rootData.subdomain,
      rootData.activeLocale,
      spaceSlug,
      rootData.multilingualEnabled,
    ),
  )
  if (typeof window === 'undefined') await prefetch
}

// Most help centers have one space, so its tree can load alongside the
// article or collection instead of after it.
function onlySpaceSlug(rootData: RootRouteData) {
  return rootData.spaces.length === 1 ? rootData.spaces[0]?.slug : undefined
}

export async function prefetchHomeRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
) {
  await prefetchSpaceNavigation(queryClient, rootData, rootData.spaces[0]?.slug)
}

export async function prefetchSpaceRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  spaceSlug: string,
) {
  const space = rootData.spaces.find(
    (candidate) => candidate.slug.toLowerCase() === spaceSlug.toLowerCase(),
  )
  await prefetchSpaceNavigation(queryClient, rootData, space?.slug)
}

export async function prefetchArticleRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  articleKey: string,
) {
  const guessedSpaceSlug = onlySpaceSlug(rootData)
  const [article] = await Promise.all([
    queryClient
      .fetchQuery(
        articleQueryOptions(
          rootData.subdomain,
          rootData.activeLocale,
          articleKey,
          rootData.multilingualEnabled,
        ),
      )
      .catch(() => null),
    prefetchSpaceNavigation(queryClient, rootData, guessedSpaceSlug),
  ])

  if (article?.space_slug && article.space_slug !== guessedSpaceSlug) {
    await prefetchSpaceNavigation(queryClient, rootData, article.space_slug)
  }

  return article
}

export async function prefetchCollectionRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  collectionSlug: string,
) {
  const guessedSpaceSlug = onlySpaceSlug(rootData)
  const [collection] = await Promise.all([
    queryClient
      .fetchQuery(
        collectionQueryOptions(
          rootData.subdomain,
          rootData.activeLocale,
          collectionSlug,
          rootData.multilingualEnabled,
        ),
      )
      .catch(() => null),
    prefetchSpaceNavigation(queryClient, rootData, guessedSpaceSlug),
  ])

  if (collection?.space_slug && collection.space_slug !== guessedSpaceSlug) {
    await prefetchSpaceNavigation(queryClient, rootData, collection.space_slug)
  }

  return collection
}

export async function prefetchPreviewRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  docId: string,
  token: string,
) {
  const article = await queryClient
    .fetchQuery(previewArticleQueryOptions(rootData.subdomain, docId, token))
    .catch(() => null)

  if (article?.space_slug) {
    await queryClient
      .prefetchQuery(
        spaceNavigationQueryOptions(
          rootData.subdomain,
          rootData.activeLocale,
          article.space_slug,
          rootData.multilingualEnabled,
        ),
      )
      .catch(() => undefined)
  }

  return article
}

export async function prefetchSearchRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  query: string,
  spaceSlug?: string,
) {
  if (query.length < 2) {
    return []
  }

  return queryClient
    .fetchQuery(
      searchArticlesQueryOptions(
        rootData.subdomain,
        rootData.activeLocale,
        query,
        rootData.multilingualEnabled,
        spaceSlug,
      ),
    )
    .catch(() => [])
}
