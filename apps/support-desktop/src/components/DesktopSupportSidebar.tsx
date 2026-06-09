import { useNavigate, useRouterState } from '@tanstack/react-router'
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from '@/components/ui/sidebar'
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher'
import { SupportRailNav } from '@/components/layout/sidebar/SupportRailNav'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { useInboxScopes, useUnreadStats, useArchiveMailbox } from '@/hooks/queries/useSupport'

interface DesktopSupportSidebarProps {
  workspaceId: string
  wsSlug: string
}

export function DesktopSupportSidebar({ workspaceId, wsSlug }: DesktopSupportSidebarProps) {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const {
    navFilter,
    setNavFilter,
    selectedMailboxId,
    setSelectedMailboxId,
    setTeamInboxDialogOpen,
    setEditMailboxId,
  } = useSupportInboxStore()

  const unreadMailboxScope = selectedMailboxId === 'all' ? undefined : selectedMailboxId
  const { data: inboxScopes } = useInboxScopes(workspaceId)
  const { data: unreadStats } = useUnreadStats(workspaceId, unreadMailboxScope, !!workspaceId)
  const archiveMailbox = useArchiveMailbox(workspaceId)

  return (
    <Sidebar collapsible="none" className="border-r border-border/70 bg-[#f0f0f2] dark:bg-sidebar [&_.support-bottom-bar]:ml-0 [&_.support-bottom-bar]:w-(--sidebar-width)">
      <SidebarHeader className="relative p-2 after:absolute after:right-2 after:bottom-0 after:left-2 after:h-px after:bg-border/70 after:[mask-image:linear-gradient(to_right,transparent,black_24px,black_calc(100%-24px),transparent)] dark:after:bg-sidebar-border">
        <WorkspaceSwitcher />
      </SidebarHeader>

      <SidebarContent className="gap-0 p-2 pb-16">
        <SupportRailNav
          navFilter={navFilter}
          unreadStats={unreadStats}
          inboxScopes={inboxScopes}
          selectedMailboxId={selectedMailboxId}
          canManageSettings
          wsSlug={wsSlug}
          pathname={pathname}
          onNavFilterChange={setNavFilter}
          onMailboxSelect={setSelectedMailboxId}
          onCreateMailbox={() => setTeamInboxDialogOpen(true)}
          onEditMailbox={(id) => { setEditMailboxId(id); setTeamInboxDialogOpen(true) }}
          onArchiveMailbox={(id) => archiveMailbox.mutate(id)}
          onNavigate={(to) => navigate({ to })}
          // Custom inbox views are a web-only feature; desktop does not render
          // them (no `customViews` passed), so these handlers are never invoked.
          onCustomViewSelect={() => {}}
          onEditCustomView={() => {}}
          onDeleteCustomView={() => {}}
        />
      </SidebarContent>
    </Sidebar>
  )
}
