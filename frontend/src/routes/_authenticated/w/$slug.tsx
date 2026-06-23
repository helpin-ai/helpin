import { memo, useEffect, type CSSProperties } from 'react'
import { createFileRoute, Link, Navigate, Outlet, useLocation } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { CalendarClock, CircleAlert } from 'lucide-react'
import { useWorkspaceBySlug } from '@/hooks/queries/useWorkspaces'
import { useSession, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { useWorkspaceSettings } from '@/hooks/queries/useSettings'
import { useWorkspaceBilling } from '@/hooks/queries/useBilling'
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
import { Button } from '@/components/ui/button'
import { MFARequiredGate } from '@/components/auth/MFARequiredGate'
import { queryKeys } from '@/lib/queryKeys'
import type { WorkspaceBillingSummary } from '@/lib/types'

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
  const { data: billing, isLoading: billingLoading } = useWorkspaceBilling(wsId)
  const queryClient = useQueryClient()
  const location = useLocation()
  const isOwner = access?.membership?.role === 'owner'

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
    || (!!wsId && billingLoading)
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

  if (billing?.locked && !location.pathname.endsWith('/settings/billing')) {
    return <Navigate to="/w/$slug/settings/billing" params={{ slug }} replace />
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
              <WorkspaceBillingNotice billing={billing} slug={slug} isOwner={isOwner} />
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

function WorkspaceBillingNotice({
  billing,
  slug,
  isOwner,
}: {
  billing?: WorkspaceBillingSummary | null
  slug: string
  isOwner: boolean
}) {
  const location = useLocation()
  if (!billing || location.pathname.endsWith('/settings/billing')) return null

  const isPaymentIssue = billing.status === 'past_due' || billing.status === 'unpaid' || billing.billing_notice_type === 'payment_failed'
  const isTrialEnding = billing.billing_notice_type === 'trial_will_end'
  if (!isPaymentIssue && !isTrialEnding) return null

  const Icon = isPaymentIssue ? CircleAlert : CalendarClock
  const title = isPaymentIssue ? 'Payment needs attention' : 'Trial ending soon'
  const message = isPaymentIssue
    ? billing.billing_notice_message || 'Update your payment method to keep this workspace active.'
    : billing.billing_notice_message || 'Choose a plan to keep this workspace active after the trial.'
  const ownerCTA = isPaymentIssue ? 'Update' : 'Upgrade'
  const bannerClassName = isPaymentIssue
    ? 'border-red-200 bg-red-50 text-red-900 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-200'
    : 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200'

  return (
    <div className={`border-b px-4 py-2 text-sm ${bannerClassName}`}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-2">
          <Icon className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="min-w-0">
            <p className="font-medium">{title}</p>
            <p className="mt-0.5 text-xs sm:text-sm">{message}</p>
          </div>
        </div>
        {isOwner ? (
          <Button asChild size="sm" variant="destructive" className="h-7 shrink-0 px-3 text-xs">
            <Link to="/w/$slug/settings/billing" params={{ slug }}>
              {ownerCTA}
            </Link>
          </Button>
        ) : (
          <Button size="sm" variant="outline" className="h-7 shrink-0 bg-background px-3 text-xs" disabled>
            Ask owner
          </Button>
        )}
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
