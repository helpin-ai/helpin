import { useEffect, useState } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useQuarterStore } from '@/stores/quarterStore'
import { useSessionStore } from '@/stores/sessionStore'
import { Sidebar } from '@/components/layout/Sidebar'
import { Header } from '@/components/layout/Header'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'

export const Route = createFileRoute('/_authenticated/w/$slug')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const { slug } = Route.useParams()
  const currentWorkspace = useWorkspaceStore((s) => s.currentWorkspace)
  const [loading, setLoading] = useState(true)
  const [mobileOpen, setMobileOpen] = useState(false)

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
      <div className="flex h-screen">
        <div className="w-64 border-r p-4 space-y-4">
          <Skeleton className="h-6 w-32" />
          <div className="space-y-2">
            {Array.from({ length: 7 }).map((_, i) => (
              <Skeleton key={i} className="h-8 w-full" />
            ))}
          </div>
        </div>
        <div className="flex-1 p-6 space-y-4">
          <Skeleton className="h-8 w-48" />
          <div className="grid grid-cols-3 gap-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-32 w-full" />
            ))}
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
    <div className="flex h-screen overflow-hidden">
      {/* Desktop sidebar */}
      <aside className="hidden lg:flex lg:w-64 lg:flex-col border-r">
        <Sidebar />
      </aside>

      {/* Mobile sidebar */}
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="p-0 w-64">
          <Sidebar onClose={() => setMobileOpen(false)} />
        </SheetContent>
      </Sheet>

      {/* Main content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        <Header onMenuClick={() => setMobileOpen(true)} />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
