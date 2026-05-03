import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { storeRedirectAfterLogin } from '@/lib/authRedirect'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ context }) => {
    // Don't redirect to login if server is just unreachable — user may still have valid tokens
    if (!context.auth.loading && !context.auth.user && !context.auth.serverUnreachable) {
      storeRedirectAfterLogin()
      throw redirect({ to: '/login' })
    }
  },
  component: AuthenticatedLayout,
})

function AuthenticatedLayout() {
  const { auth } = Route.useRouteContext()
  const [retrying, setRetrying] = useState(false)

  const handleRetry = async () => {
    setRetrying(true)
    await useAuthStore.getState().initialize()
    setRetrying(false)
  }

  if (auth.loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    )
  }

  if (auth.serverUnreachable && !auth.user) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4">
        <div className="text-center space-y-2">
          <h2 className="text-xl font-semibold">Unable to reach the server</h2>
          <p className="text-muted-foreground">
            The server appears to be unavailable. Your session is preserved — please try again.
          </p>
        </div>
        <Button onClick={handleRetry} disabled={retrying}>
          {retrying ? 'Retrying...' : 'Retry'}
        </Button>
      </div>
    )
  }

  return <Outlet />
}
