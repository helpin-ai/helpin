import { useEffect, useState, type CSSProperties } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useQuarterStore } from '@/stores/quarterStore'
import { useSessionStore } from '@/stores/sessionStore'
import { useRealtimeSync } from '@/hooks/useRealtimeSync'
import { Sidebar } from '@/components/layout/Sidebar'
import { Header } from '@/components/layout/Header'
import { GlobalCreateModals } from '@/components/pm/GlobalCreateModals'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { Skeleton } from '@/components/ui/skeleton'

export const Route = createFileRoute('/_authenticated/w/$slug')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const { slug } = Route.useParams()
  const currentWorkspace = useWorkspaceStore((s) => s.currentWorkspace)
  const [loading, setLoading] = useState(true)

  useRealtimeSync(currentWorkspace?.id ?? '')

  useEffect(() => {
    if (!slug) return
    const init = async () => {
      setLoading(true)
      const cachedWs = useWorkspaceStore.getState().currentWorkspace
      const ws = cachedWs?.slug === slug
        ? cachedWs
        : await useWorkspaceStore.getState().loadWorkspaceBySlug(slug)
      if (!ws) { setLoading(false); return }
      await Promise.all([
        useQuarterStore.getState().loadQuarters(ws.id),
        useSessionStore.getState().loadMembership(ws.id),
      ])
      setLoading(false)
    }
    init()
  }, [slug])

  if (loading) {
    return (
      <div className="min-h-svh bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.9)_45%,rgba(187,210,229,0.55)_100%)]">
        <div className="h-svh w-full overflow-hidden border border-border/70 bg-background/90 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.45)] backdrop-blur">
          <div className="flex h-full">
            <div className="w-72 border-r p-4 space-y-4">
              <Skeleton className="h-7 w-48" />
              <div className="space-y-2">
                {Array.from({ length: 8 }).map((_, i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
            </div>
            <div className="flex-1 p-6 space-y-4">
              <Skeleton className="h-10 w-full max-w-xl" />
              <Skeleton className="h-8 w-52" />
              <div className="grid grid-cols-3 gap-4">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-32 w-full" />
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!currentWorkspace) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p className="text-muted-foreground">Workspace not found</p>
      </div>
    )
  }

  return (
    <div className="min-h-svh bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.92)_45%,rgba(187,210,229,0.55)_100%)]">
      <div className="h-svh w-full overflow-hidden border border-border/70 bg-background/92 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.45)] backdrop-blur">
        <SidebarProvider
          className="!min-h-0 h-full"
          style={{ '--sidebar-width': '16rem', '--sidebar-width-icon': '3rem' } as CSSProperties}
        >
          <Sidebar />
          <SidebarInset className="min-w-0 overflow-hidden bg-transparent">
            <Header />
            <main className="relative min-h-0 flex-1 overflow-hidden">
              <Outlet />
            </main>
            <GlobalCreateModals workspaceId={currentWorkspace.id} />
          </SidebarInset>
        </SidebarProvider>
      </div>
    </div>
  )
}
