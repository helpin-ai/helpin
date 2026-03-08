import { useEffect, type CSSProperties } from 'react'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useWorkspaceBySlug } from '@/hooks/queries/useWorkspaces'
import { useSession, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { useWorkspaceSettings } from '@/hooks/queries/useSettings'
import { useOrganizations } from '@/hooks/queries/useOrganizations'
import { useQuarters } from '@/hooks/queries/useQuarters'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useOrganizationStore } from '@/stores/organizationStore'
import { useRewardQuarterStore } from '@/stores/quarterStore'
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

  // TanStack Query for all data fetching
  const { data: workspace, isLoading: wsLoading } = useWorkspaceBySlug(slug)
  const wsId = workspace?.id ?? ''
  const { data: orgs, isLoading: orgsLoading } = useOrganizations()
  const { data: quarters, isLoading: quartersLoading } = useQuarters(wsId)
  const { isLoading: sessionLoading } = useSession(wsId)
  const { isLoading: accessLoading } = useWorkspaceAccess(wsId)
  const { isLoading: settingsLoading } = useWorkspaceSettings(wsId)

  // Selection stores (Zustand) — sync from query data
  const currentWorkspace = useWorkspaceStore((s) => s.currentWorkspace)

  useRealtimeSync(wsId)

  // Sync workspace selection
  useEffect(() => {
    if (workspace) useWorkspaceStore.getState().setCurrentWorkspace(workspace)
  }, [workspace])

  // Sync organization selection
  useEffect(() => {
    if (!orgs?.length || !workspace?.organization_id) return
    const org = orgs.find(o => o.id === workspace.organization_id)
    if (org) useOrganizationStore.getState().setCurrentOrganization(org)
  }, [orgs, workspace?.organization_id])

  // Sync quarter selection (initial only — don't override user choice)
  useEffect(() => {
    if (!quarters?.length) return
    const current = useRewardQuarterStore.getState().currentQuarter
    if (!current || !quarters.find(q => q.id === current.id)) {
      const active = quarters.find(q => q.status === 'active')
      useRewardQuarterStore.getState().setCurrentQuarter(active ?? quarters[0])
    }
  }, [quarters])

  const loading = wsLoading || orgsLoading
    || (!!wsId && (sessionLoading || accessLoading || settingsLoading || quartersLoading))
    || (!!workspace && currentWorkspace?.id !== workspace.id)

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
          <SidebarInset className="min-w-0 overflow-hidden bg-transparent shadow-[inset_2px_0_12px_0_rgba(0,0,0,0.06)] dark:shadow-[inset_2px_0_12px_0_rgba(0,0,0,0.2)]">
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
