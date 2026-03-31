import { createRouter } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { LoadingState } from '@/components/LoadingState'
import { createHelpCenterQueryClient } from '@/lib/queryClient'
import { routeTree } from './routeTree.gen'

export function getRouter() {
  const queryClient = createHelpCenterQueryClient()
  const router = createRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPendingComponent: () => <LoadingState fullScreen />,
    defaultPendingMinMs: 300,
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
