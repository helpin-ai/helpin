import { createFileRoute, redirect } from '@tanstack/react-router'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchCollectionRouteData } from '@/lib/routeData'
import { loadAlternateLinks } from '@/lib/alternateLinks'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCollectionHead } from '@/lib/seo'

export const Route = createFileRoute('/$locale/c/$collectionSlug')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    if (!rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/c/${params.collectionSlug}`,
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const collection = await prefetchCollectionRouteData(
      context.queryClient,
      rootData,
      params.collectionSlug,
    )
    const alternates = collection
      ? await loadAlternateLinks(context.queryClient, rootData, {
          kind: 'collection',
          spaceId: rootData.spaces.find(
            (space) => space.slug === (collection.space_slug || params.collectionSlug),
          )?.id,
          collectionId: collection.collection.id,
        })
      : []

    return { rootData, collection, alternates }
  },
  head: ({ loaderData, params }) =>
    loaderData?.collection
      ? buildCollectionHead(
          loaderData.rootData,
          loaderData.collection,
          params.collectionSlug,
          loaderData.alternates,
        )
      : {},
  component: LocalizedCollectionPage,
})

function LocalizedCollectionPage() {
  const { collectionSlug } = Route.useParams()
  const { locale, multilingualEnabled } = useDocsContext()

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <CollectionRouteView
      locale={locale}
      collectionOrSpaceSlug={collectionSlug}
      multilingualEnabled
    />
  )
}
