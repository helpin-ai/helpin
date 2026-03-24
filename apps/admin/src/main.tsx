import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { router } from '@/router'
import { queryClient } from '@/lib/queryClient'
import { startTokenRefreshTimer, setupVisibilityRefresh } from '@/lib/api'
import { useAuthStore } from '@/stores/authStore'
import './index.css'

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
