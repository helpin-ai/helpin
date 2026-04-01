import { useEffect } from 'react'
import { createRouter, ErrorComponent } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { LoadingState } from '@/components/LoadingState'
import { createHelpCenterQueryClient } from '@/lib/queryClient'
import { routeTree } from './routeTree.gen'

function ChunkErrorHandler({ error }: { error: Error }) {
  const isChunkError =
    error.message?.includes('dynamically imported module') ||
    error.message?.includes('Failed to fetch') ||
    error.message?.includes('Loading chunk')

  useEffect(() => {
    if (isChunkError) {
      window.location.reload()
    }
  }, [isChunkError])

  if (isChunkError) {
    return <LoadingState message="Updating..." />
  }

  return <ErrorComponent error={error} />
}

export function getRouter() {
  const queryClient = createHelpCenterQueryClient()
  const router = createRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPendingComponent: () => <LoadingState />,
    defaultPendingMinMs: 0,
    defaultErrorComponent: ChunkErrorHandler,
  })

  setupRouterSsrQueryIntegration({
    router,
    queryClient,
  })

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
