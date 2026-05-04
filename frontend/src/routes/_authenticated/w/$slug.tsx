import { memo, useEffect, type CSSProperties } from 'react'
import { createFileRoute, Outlet, useLocation } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useWorkspaceBySlug } from '@/hooks/queries/useWorkspaces'
import { useSession, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { useWorkspaceSettings } from '@/hooks/queries/useSettings'
import { useOrganizations } from '@/hooks/queries/useOrganizations'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useOrganizationStore } from '@/stores/organizationStore'
import { useRealtimeSync } from '@/hooks/useRealtimeSync'
import { Sidebar } from '@/components/layout/Sidebar'
import { Header } from '@/components/layout/Header'
import { GlobalCreateModals } from '@/components/pm/GlobalCreateModals'
import { GlobalEpicPanel } from '@/components/pm/GlobalEpicPanel'
import { GlobalTaskPanel } from '@/components/pm/GlobalTaskPanel'
import { PageContextProvider } from '@/components/command-bar/pageContext'
import { AskAgentsDock } from '@/components/agents/AskAgentsDock'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { Skeleton } from '@/components/ui/skeleton'
import { MFARequiredGate } from '@/components/auth/MFARequiredGate'
import { queryKeys } from '@/lib/queryKeys'

export const Route = createFileRoute('/_authenticated/w/$slug')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const { slug } = Route.useParams()

  // TanStack Query for all data fetching
  const { data: workspace, isLoading: wsLoading } = useWorkspaceBySlug(slug)
  const wsId = workspace?.id ?? ''
  const { data: orgs, isLoading: orgsLoading } = useOrganizations()
  const { data: access, isLoading: accessLoading } = useWorkspaceAccess(wsId)
  const securityPolicy = access?.security_policy
  const mfaBlocked = !!securityPolicy?.mfa_required
  const canLoadWorkspaceData = !!access && !mfaBlocked
  const { isLoading: sessionLoading } = useSession(wsId, { enabled: canLoadWorkspaceData })
  const { isLoading: settingsLoading } = useWorkspaceSettings(wsId, { enabled: canLoadWorkspaceData })
  const queryClient = useQueryClient()

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

  const loading = wsLoading || orgsLoading
    || (!!wsId && (sessionLoading || accessLoading || settingsLoading))
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

  if (mfaBlocked) {
    return (
      <MFARequiredGate
        workspaceName={currentWorkspace.name}
        mfaEnabled={!!securityPolicy?.mfa_enabled}
        onComplete={() => {
          queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.access(wsId) })
          queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.session(wsId) })
          queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(wsId) })
        }}
      />
    )
  }

  return (
    <div className="min-h-svh bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.92)_45%,rgba(187,210,229,0.55)_100%)]">
      <div className="h-svh w-full overflow-hidden bg-background/92 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.45)] backdrop-blur">
        <SidebarProvider
          className="!min-h-0 h-full"
          style={{ '--sidebar-width-icon': '3rem' } as CSSProperties}
        >
          <Sidebar />
          <SidebarInset className="relative min-w-0 overflow-hidden bg-transparent before:absolute before:top-3 before:left-0 before:bottom-3 before:z-10 before:w-px before:bg-border/70 before:[mask-image:linear-gradient(to_bottom,transparent,black_24px,black_calc(100%-24px),transparent)] dark:before:bg-border/60">
            <PageContextProvider>
              <RouteAwareHeader />
              <div className="flex min-h-0 flex-1 overflow-hidden">
                <main className="relative min-h-0 flex-1 overflow-hidden">
                  <Outlet />
                  <RouteAwareAskAgentsDock />
                </main>
              </div>
              <MemoizedGlobalCreateModals workspaceId={currentWorkspace.id} />
              <MemoizedGlobalTaskPanel workspaceId={currentWorkspace.id} />
              <MemoizedGlobalEpicPanel workspaceId={currentWorkspace.id} />
            </PageContextProvider>
          </SidebarInset>
        </SidebarProvider>
      </div>
    </div>
  )
}

/** Isolates useLocation subscription so WorkspaceLayout doesn't re-render on every navigation */
function RouteAwareHeader() {
  const location = useLocation()
  if (location.pathname.includes('/support')) return null
  return <Header />
}

/** Hide the Ask Agents dock on support routes — support has its own assistant flow. */
function RouteAwareAskAgentsDock() {
  const location = useLocation()
  if (location.pathname.includes('/support')) return null
  return <AskAgentsDock />
}

const MemoizedGlobalCreateModals = memo(GlobalCreateModals)
const MemoizedGlobalTaskPanel = memo(GlobalTaskPanel)
const MemoizedGlobalEpicPanel = memo(GlobalEpicPanel)
