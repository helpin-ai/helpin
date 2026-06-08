import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { configureSessionStorage, createCookieSessionStorage } from '@helpin-ai/support-core'
import { queryClient } from '@/lib/queryClient'
import { clearClientSession, useAuthStore } from '@/stores/authStore'
import { startTokenRefreshTimer, setupVisibilityRefresh } from '@/lib/api'
import { RoutePendingState } from '@/components/layout/RoutePendingState'
import { routeTree } from './routeTree.gen'
import './index.css'

configureSessionStorage(createCookieSessionStorage())

function normalizePathname(pathname: string) {
  if (!pathname) return '/'
  const normalized = pathname.replace(/\/+$/, '')
  return normalized === '' ? '/' : normalized
}

const isLogoutPath =
  typeof window !== 'undefined' && normalizePathname(window.location.pathname) === '/logout'

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
    useAuthStore.getState().initialize()
    setupVisibilityRefresh()
  }, [])

  // Start proactive token refresh when user is authenticated
  useEffect(() => {
    if (user) {
      startTokenRefreshTimer()
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
    <QueryClientProvider client={queryClient}>
      <InnerApp />
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  )
}

if (!isLogoutPath) {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
