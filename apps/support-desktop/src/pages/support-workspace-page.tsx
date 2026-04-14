import { useEffect } from 'react'
import { useParams } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useSupportRealtime } from '@helpin-ai/support-core'
import { SupportInboxLayout } from '@/components/support/SupportInboxLayout'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { API_BASE } from '@/lib/api'
import { workspacesService } from '@/lib/services/workspacesService'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { DesktopSupportSidebar } from '@desktop/components/DesktopSupportSidebar'

export function SupportWorkspacePage() {
  const params = useParams({ strict: false }) as { slug: string; conversationId?: string }
  const currentWorkspace = useWorkspaceStore((state) => state.currentWorkspace)
  const setCurrentWorkspace = useWorkspaceStore((state) => state.setCurrentWorkspace)

  const workspaceQuery = useQuery({
    queryKey: ['desktop-workspace', params.slug],
    queryFn: async () => {
      const response = await workspacesService.getBySlug(params.slug)
      if (response.error || !response.data) {
        throw new Error(response.error || 'Failed to load workspace')
      }
      return response.data
    },
  })

  const workspaceId = workspaceQuery.data?.id ?? ''

  useSupportRealtime({
    apiBase: API_BASE,
    workspaceId,
    selectedConversationId: params.conversationId ?? null,
  })

  useEffect(() => {
    if (workspaceQuery.data) {
      setCurrentWorkspace(workspaceQuery.data)
    }
  }, [setCurrentWorkspace, workspaceQuery.data])

  if (workspaceQuery.isLoading || (workspaceQuery.data && currentWorkspace?.id !== workspaceQuery.data.id)) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    )
  }

  if (workspaceQuery.error || !workspaceQuery.data) {
    return (
      <div className="flex h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground">
          {workspaceQuery.error instanceof Error ? workspaceQuery.error.message : 'Failed to load workspace.'}
        </p>
      </div>
    )
  }

  return (
    <div className="h-screen bg-[radial-gradient(circle_at_20%_20%,rgba(188,214,231,0.75),rgba(245,248,251,0.92)_45%,rgba(187,210,229,0.55)_100%)]">
      <div className="h-full w-full overflow-hidden border border-border/70 bg-background/92 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.45)] backdrop-blur">
      <SidebarProvider className="!min-h-0 h-full">
        <DesktopSupportSidebar
          workspaceId={workspaceId}
          workspaceName={workspaceQuery.data.name}
          wsSlug={params.slug}
        />
        <SidebarInset className="relative min-w-0 overflow-hidden">
          <SupportInboxLayout />
        </SidebarInset>
      </SidebarProvider>
      </div>
    </div>
  )
}
