import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { configureSessionStorage, createBrowserSessionStorage } from '@helpin-ai/support-core'
import { setupVisibilityRefresh, startTokenRefreshTimer } from '@/lib/api'
import { queryClient } from '@/lib/queryClient'
import { useAuthStore } from '@/stores/authStore'
import { isTauriDesktop } from '@desktop/lib/desktopHost'
import { router } from '@desktop/router'
import { createTauriSessionStorage } from '@desktop/lib/sessionStorage'
import './index.css'

configureSessionStorage(
  isTauriDesktop() ? createTauriSessionStorage() : createBrowserSessionStorage(),
)

function InnerApp() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)
  const serverUnreachable = useAuthStore((state) => state.serverUnreachable)

  useEffect(() => {
    useAuthStore.getState().initialize()
    setupVisibilityRefresh()
  }, [])

  useEffect(() => {
    if (user) {
      startTokenRefreshTimer()
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
      <InnerApp />
      <Toaster richColors />
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
