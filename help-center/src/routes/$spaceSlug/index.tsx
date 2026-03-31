import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCanonicalCollectionPath } from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

    throw redirect({
      statusCode: 301,
      to: buildCanonicalCollectionPath(
        rootData.multilingualEnabled,
        rootData.config.default_locale,
        params.spaceSlug,
      ),
    })
  },
  component: LegacySpaceIndexRedirect,
})

function LegacySpaceIndexRedirect() {
  return <LoadingState message="Redirecting..." />
}
