import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { HelpinProvider } from '@helpin-ai/react'
import { configureSessionStorage, createCookieSessionStorage } from '@helpin-ai/support-core'
import { queryClient } from '@/lib/queryClient'
import { helpinClient } from '@/lib/helpin'
import { clearClientSession, useAuthStore } from '@/stores/authStore'
import { startTokenRefreshTimer, stopTokenRefreshTimer, setupVisibilityRefresh } from '@/lib/api'
import { HelpinIdentitySync } from '@/components/HelpinIdentitySync'
import { RoutePendingState } from '@/components/layout/RoutePendingState'
import { identifyAnalyticsUser, initializeAppAnalytics } from '@/lib/analytics'
import { routeTree } from './routeTree.gen'
import 'streamdown/styles.css'
import './index.css'

configureSessionStorage(createCookieSessionStorage())
initializeAppAnalytics()

function normalizePathname(pathname: string) {
  if (!pathname) return '/'
  const normalized = pathname.replace(/\/+$/, '')
  return normalized === '' ? '/' : normalized
}

const isLogoutPath =
  typeof window !== 'undefined' && normalizePathname(window.location.pathname) === '/logout'

// The customer portal is public and has its own session. Staff session checks
// and developer tools never run there.
const isCustomerPortalPath =
  typeof window !== 'undefined' && /^\/portal(\/|$)/.test(normalizePathname(window.location.pathname))

if (isLogoutPath) {
  void clearClientSession().finally(() => {
    window.location.replace('/login')
  })
}

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

function App() {
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

if (!isLogoutPath) {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
