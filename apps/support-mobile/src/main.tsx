import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { configureSessionStorage, createBrowserSessionStorage } from '@helpin-ai/support-core'
import { setupVisibilityRefresh, startTokenRefreshTimer, stopTokenRefreshTimer } from '@mobile/lib/api'
import { isTauri } from '@mobile/lib/host'
import { queryClient } from '@mobile/lib/queryClient'
import { createTauriSessionStorage } from '@mobile/lib/session-storage'
import { router } from '@mobile/router'
import { bootstrapAuth, useAuthStore } from '@mobile/stores/auth-store'
import './index.css'

configureSessionStorage(isTauri() ? createTauriSessionStorage() : createBrowserSessionStorage())

function InnerApp() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)
  const serverUnreachable = useAuthStore((state) => state.serverUnreachable)

  useEffect(() => {
    void bootstrapAuth()
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
    <>
      <RouterProvider
        router={router}
        context={{ auth: { user, loading, serverUnreachable } }}
      />
      <Toaster richColors position="top-center" />
    </>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <InnerApp />
    </QueryClientProvider>
  )
}

const rootEl = document.getElementById('root')
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
