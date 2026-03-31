import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCanonicalCollectionPath } from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

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
  return <LoadingState message="Redirecting..." />
}
