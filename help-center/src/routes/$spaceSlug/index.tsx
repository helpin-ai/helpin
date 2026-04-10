import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { useDocsContext } from '@/contexts/DocsContext'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCanonicalCollectionPath, isMultilingualEnabled } from '@/lib/locale'
import { stripBasepath } from '@/lib/pathUtils'

export const Route = createFileRoute('/$spaceSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const internalPath = stripBasepath(location.pathname, rootData.basepath)

    if (!rootData.multilingualEnabled && internalPath === '/') {
      return
    }

    const target = buildCanonicalCollectionPath(
      rootData.multilingualEnabled,
      rootData.config.default_locale,
      params.spaceSlug,
    )

    // Avoid redirect loop when target resolves to the same path
    const currentPath = location.pathname.replace(/\/+$/, '') || '/'
    const targetPath = target.replace(/\/+$/, '') || '/'
    if (currentPath === targetPath) {
      return
    }

    throw redirect({
      statusCode: 301,
      to: target,
    })
  },
  component: LegacySpaceIndexRedirect,
})

function LegacySpaceIndexRedirect() {
  const { spaceSlug } = Route.useParams()
  const { basepath, enabledLocales } = useDocsContext()

  if (!isMultilingualEnabled(enabledLocales) && basepath === `/${spaceSlug}`) {
    return <LocalizedHomePage />
  }

  return <LoadingState message="Redirecting..." />
}
