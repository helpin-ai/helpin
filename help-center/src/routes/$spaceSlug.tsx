import { createFileRoute, redirect } from '@tanstack/react-router'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCanonicalCollectionPath, buildCanonicalHomePath } from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const normalizedSlug = params.spaceSlug.trim().toLowerCase()
    const isKnownLocaleSlug = rootData.config.enabled_locales.some(
      (locale) => locale.toLowerCase() === normalizedSlug,
    )
    const isSpaceSlug = rootData.spaces.some(
      (space) => space.slug.toLowerCase() === normalizedSlug,
    )

    if (rootData.multilingualEnabled && !isKnownLocaleSlug) {
      throw redirect({
        statusCode: 301,
        href: `/${rootData.config.default_locale}/${params.spaceSlug}`,
      })
    }

    if (!rootData.multilingualEnabled && isKnownLocaleSlug) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalHomePath(false, rootData.config.default_locale),
      })
    }

    if (!rootData.multilingualEnabled && !isSpaceSlug) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalCollectionPath(false, rootData.config.default_locale, params.spaceSlug),
      })
    }
  },
  component: SpaceOrLegacyRedirect,
})

function SpaceOrLegacyRedirect() {
  const { spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales, spaces } = useDocsContext()
  const normalizedSlug = spaceSlug.trim().toLowerCase()
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSlug,
  )
  const isSpaceSlug = spaces.some((space) => space.slug.toLowerCase() === normalizedSlug)

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
