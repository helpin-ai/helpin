import { createFileRoute } from '@tanstack/react-router'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'

// The canonical non-multilingual space URL is /{spaceSlug}, which is exactly
// this index route. It must render the space view itself — redirecting from
// here targets the same URL and loops the router (see 48046d978 for the
// previous occurrence). Non-space slugs are redirected by the parent layout's
// beforeLoad before this component renders.
export const Route = createFileRoute('/$spaceSlug/')({
  component: SpaceIndex,
})

function SpaceIndex() {
  const { spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales, spaces } = useDocsContext()
  const normalizedSlug = spaceSlug.trim().toLowerCase()
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSlug,
  )
  const isSpaceSlug = spaces.some(
    (space) => space.slug.toLowerCase() === normalizedSlug,
  )

  if (!isKnownLocaleSlug && isSpaceSlug) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={spaceSlug}
        multilingualEnabled={false}
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
