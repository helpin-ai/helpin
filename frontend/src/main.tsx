import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { queryClient } from '@/lib/queryClient'
import { useAuthStore } from '@/stores/authStore'
import { routeTree } from './routeTree.gen'
import './index.css'

const router = createRouter({
  routeTree,
  context: {
    auth: {
      user: null,
      loading: true,
      serverUnreachable: false,
    },
  },
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
  }, [])

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

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
