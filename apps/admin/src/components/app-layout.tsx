import { useState } from 'react'
import { Outlet } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Sidebar } from './sidebar'

export function AppLayout() {
  const user = useAuthStore((s) => s.user)
  const loading = useAuthStore((s) => s.loading)
  const serverUnreachable = useAuthStore((s) => s.serverUnreachable)
  const initialize = useAuthStore((s) => s.initialize)
  const [retrying, setRetrying] = useState(false)

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    )
  }

  if (serverUnreachable && !user) {
    const handleRetry = async () => {
      setRetrying(true)
      await initialize()
      setRetrying(false)
    }

    return (
      <div className="flex min-h-screen items-center justify-center px-6">
        <div className="w-full max-w-lg rounded-none border bg-card p-8">
          <h1 className="text-xl font-semibold">Unable to reach the API</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            The admin panel preserved your session, but the backend is currently unavailable.
          </p>
          <Button className="mt-6" onClick={handleRetry} disabled={retrying}>
            {retrying ? 'Retrying...' : 'Retry'}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
