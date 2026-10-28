import { useEffect } from 'react'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { HelpinProvider } from '@helpin-ai/react'
import { queryClient } from '@/lib/queryClient'
import { helpinClient } from '@/lib/helpin'
import { useAuthStore } from '@/stores/authStore'
import { startTokenRefreshTimer, stopTokenRefreshTimer, setupVisibilityRefresh } from '@/lib/api'
import { HelpinIdentitySync } from '@/components/HelpinIdentitySync'
import { RoutePendingState } from '@/components/layout/RoutePendingState'
import { AppLoadError } from '@/components/layout/AppLoadError'
import { identifyAnalyticsUser } from '@/lib/analytics'
import { isCustomerPortalPath } from './appPaths'
import { routeTree } from './routeTree.gen'

const router = createRouter({
  routeTree,
  context: {
    auth: {
      user: null,
      loading: true,
      serverUnreachable: false,
    },
  },
  defaultPendingComponent: RoutePendingState,
  defaultErrorComponent: AppLoadError,
  defaultPendingMs: 120,
  defaultPendingMinMs: 300,
  defaultNotFoundComponent: () => {
    router.navigate({ to: '/workspaces' })
    return null
  },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function InnerApp() {
  const user = useAuthStore((s) => s.user)
  const loading = useAuthStore((s) => s.loading)
  const serverUnreachable = useAuthStore((s) => s.serverUnreachable)

  useEffect(() => {
    if (isCustomerPortalPath) {
      useAuthStore.setState({ loading: false })
      return
    }
    useAuthStore.getState().initialize()
  }, [])

  // Start proactive token refresh when user is authenticated
  useEffect(() => {
    if (!user) {
      stopTokenRefreshTimer()
      return
    }

    identifyAnalyticsUser(user)
    startTokenRefreshTimer()
    const cleanupVisibilityRefresh = setupVisibilityRefresh()
    return () => {
      cleanupVisibilityRefresh()
      stopTokenRefreshTimer()
    }
  }, [user])

  // Force router to re-evaluate routes when auth state changes
  useEffect(() => {
    router.invalidate()
  }, [user, loading, serverUnreachable])

  return <RouterProvider router={router} context={{ auth: { user, loading, serverUnreachable } }} />
}

/** App is the authenticated app and public pages, with their providers. */
export function App() {
  return (
    <HelpinProvider client={helpinClient}>
      <QueryClientProvider client={queryClient}>
        <HelpinIdentitySync />
        <InnerApp />
        {isCustomerPortalPath ? null : (
          <div className="helpin-query-devtools">
            <ReactQueryDevtools initialIsOpen={false} buttonPosition="top-right" />
          </div>
        )}
      </QueryClientProvider>
    </HelpinProvider>
  )
}
