import { StrictMode, Suspense, lazy, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { configureSessionStorage, createBrowserSessionStorage, useUnreadStats } from '@helpin-ai/support-core'
import { setupVisibilityRefresh, startTokenRefreshTimer, stopTokenRefreshTimer } from '@/lib/api'
import { queryClient } from '@/lib/queryClient'
import { useAuthStore } from '@/stores/authStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { isTauriDesktop } from '@desktop/lib/desktopHost'
import { updateBadgeCount } from '@desktop/lib/desktopBadge'
import { registerNotificationClickHandler, setupNotificationClickListener } from '@desktop/lib/desktopNotifications'
import { router } from '@desktop/router'
import { createTauriSessionStorage } from '@desktop/lib/sessionStorage'
import './index.css'

const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() => import('@tanstack/react-query-devtools').then((module) => ({ default: module.ReactQueryDevtools })))
  : null

configureSessionStorage(
  isTauriDesktop() ? createTauriSessionStorage() : createBrowserSessionStorage(),
)

// Disable right-click context menu globally in the desktop app.
document.addEventListener('contextmenu', (e) => { e.preventDefault() })

// Register notification click-through routing.
registerNotificationClickHandler((to) => {
  router.navigate({ to: to as string })
})
setupNotificationClickListener()

function DesktopShellSync() {
  const user = useAuthStore((state) => state.user)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id ?? '')
  const { data: unreadStats } = useUnreadStats(workspaceId, undefined, !!user && !!workspaceId)

  useEffect(() => {
    if (!user || !workspaceId) {
      void updateBadgeCount(0)
      return
    }

    void updateBadgeCount(unreadStats?.total ?? 0)
  }, [user, workspaceId, unreadStats?.total])

  useEffect(() => {
    return () => {
      void updateBadgeCount(0)
    }
  }, [])

  return null
}

function InnerApp() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)
  const serverUnreachable = useAuthStore((state) => state.serverUnreachable)

  useEffect(() => {
    useAuthStore.getState().initialize()
  }, [])

  useEffect(() => {
    if (!user) {
      stopTokenRefreshTimer()
      return
    }

    startTokenRefreshTimer()
    const cleanupVisibilityRefresh = setupVisibilityRefresh()
    return () => {
      cleanupVisibilityRefresh()
      stopTokenRefreshTimer()
    }
  }, [user])

  useEffect(() => {
    router.invalidate()
  }, [user, loading, serverUnreachable])

  return (
    <RouterProvider
      router={router}
      context={{ auth: { user, loading, serverUnreachable } }}
    />
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <DesktopShellSync />
      <InnerApp />
      <Toaster richColors />
      {ReactQueryDevtools ? (
        <Suspense fallback={null}>
          <ReactQueryDevtools initialIsOpen={false} />
        </Suspense>
      ) : null}
    </QueryClientProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
