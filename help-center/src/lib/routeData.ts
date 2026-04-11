import type { QueryClient } from '@tanstack/react-query'
import {
  articleQueryOptions,
  collectionQueryOptions,
  previewArticleQueryOptions,
  searchArticlesQueryOptions,
  spaceNavigationQueryOptions,
} from '@/hooks/queries'
import type { RootRouteData } from '@/lib/rootLoader'

export async function prefetchHomeRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
) {
  const firstSpace = rootData.spaces[0]
  if (firstSpace?.slug) {
    await queryClient
      .prefetchQuery(
        spaceNavigationQueryOptions(
          rootData.subdomain,
          rootData.activeLocale,
          firstSpace.slug,
          rootData.multilingualEnabled,
        ),
      )
      .catch(() => undefined)
  }
}

export async function prefetchArticleRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  articleKey: string,
) {
  const article = await queryClient
    .fetchQuery(
      articleQueryOptions(
        rootData.subdomain,
        rootData.activeLocale,
        articleKey,
        rootData.multilingualEnabled,
      ),
    )
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

export async function prefetchCollectionRouteData(
  queryClient: QueryClient,
  rootData: RootRouteData,
  collectionSlug: string,
) {
  const collection = await queryClient
    .fetchQuery(
      collectionQueryOptions(
        rootData.subdomain,
        rootData.activeLocale,
        collectionSlug,
        rootData.multilingualEnabled,
      ),
    )
    .catch(() => null)

  if (collection?.space_slug) {
    await queryClient
      .prefetchQuery(
        spaceNavigationQueryOptions(
          rootData.subdomain,
          rootData.activeLocale,
          collection.space_slug,
          rootData.multilingualEnabled,
        ),
      )
      .catch(() => undefined)
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
