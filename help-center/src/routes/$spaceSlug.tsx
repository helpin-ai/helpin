import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
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
  component: SpaceSlugLayout,
})

function SpaceSlugLayout() {
  return <Outlet />
}
