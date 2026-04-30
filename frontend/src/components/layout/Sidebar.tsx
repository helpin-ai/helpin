import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { useTheme } from 'next-themes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useSupportInboxStore, type NavFilter } from '@/stores/supportInboxStore';
import { useQuery } from '@tanstack/react-query';
import { useArchiveMailbox, useInboxScopes, useUnreadStats } from '@/hooks/queries/useSupport';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { getInitials } from '@/lib/utils';
import { buildSupportInboxSearch } from '@/lib/supportInboxRouting';
import { ACTIVE_RUN_STATUSES, isPausedAgentRun } from '@/components/pm/agentRunConstants';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import {
  Sidebar as ShellSidebar,
  SidebarContent,
  SidebarHeader,
} from '@/components/ui/sidebar';
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';
import { NotificationCenter } from '@/components/notifications/NotificationCenter';
import { useSupportTeammatePresence, useUpdateMySupportTeammatePresence } from '@/hooks/queries/useSupport';
import { DocsSpacesNav } from './sidebar/DocsSpacesNav';
import { buildPanelNavGroups, buildRailItems, deriveActiveRail, projectCreateOptions } from './sidebar/config';
import { isSidebarLinkActive, isTeamSubLinkActive, type SidebarNavigateTarget } from './sidebar/navigation';
import { ProjectsTeamsNav } from './sidebar/ProjectsTeamsNav';
import { COLLAPSIBLE_SETTINGS_GROUPS, getCollapsedSettingsGroups, getExpandedTeams, saveCollapsedSettingsGroups, saveExpandedTeams } from './sidebar/state';
import { SidebarAccountMenu } from './sidebar/SidebarAccountMenu';
import { SidebarCreateBar } from './sidebar/SidebarCreateBar';
import { SidebarRail } from './sidebar/SidebarRail';
import { SettingsRailNav } from './sidebar/SettingsRailNav';
import { StandardRailNav } from './sidebar/StandardRailNav';
import { CrmRailNav } from './sidebar/CrmRailNav';
import { SupportRailNav } from './sidebar/SupportRailNav';

function defaultStatusForSupportFilter(filter: NavFilter) {
  return filter === 'my_inbox' || filter === 'unassigned' || filter === 'mentions' ? 'open' : 'all';
}

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const { user, signOut } = useAuthStore();
  const { theme, setTheme } = useTheme();
  const openCreate = useGlobalCreateStore((state) => state.openCreate);

  const wsSlug = currentWorkspace?.slug ?? '';
  const workspaceId = currentWorkspace?.id;
  const activeRail = deriveActiveRail(location.pathname);
  const initials = getInitials(user?.full_name || user?.email);

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { isAdmin, canManageSettings, canManageTeams, permissionSet, canAccessModule, modules } = usePermissions(access);
  const hasSupportModule = canAccessModule('support');
  const {
    navFilter,
    setNavFilter,
    selectedMailboxId,
    setSelectedMailboxId,
    statusFilter,
    searchQuery,
    setTeamInboxDialogOpen,
    setEditMailboxId,
  } = useSupportInboxStore();

  const { data: inboxScopes } = useInboxScopes(workspaceId ?? '', hasSupportModule);
  const unreadMailboxScope = selectedMailboxId === 'all' ? undefined : selectedMailboxId;
  const { data: unreadStats } = useUnreadStats(workspaceId ?? '', unreadMailboxScope, hasSupportModule);
  const archiveMailbox = useArchiveMailbox(workspaceId ?? '');
  const totalSupportUnread = useMemo(
    () => (inboxScopes?.shared_inbox.unread_count ?? 0) + (inboxScopes?.mailboxes ?? []).reduce((sum, mailbox) => sum + mailbox.unread_count, 0),
    [inboxScopes],
  );

  const { data: agentRunsData } = useQuery({
    queryKey: queryKeys.automation.runs(workspaceId ?? '', 1, 100),
    queryFn: async () => {
      const res = await automationService.listWorkspaceRuns(workspaceId!, 1, 100);
      return {
        data: Array.isArray(res.data?.data) ? res.data.data : [],
        total: res.data?.total ?? 0,
        page: res.data?.page ?? 1,
        per_page: res.data?.per_page ?? 100,
        total_pages: res.data?.total_pages ?? 0,
      };
    },
    enabled: !!workspaceId,
    staleTime: 30_000,
  });
  const agentRuns = Array.isArray(agentRunsData?.data) ? agentRunsData.data : [];
  const agentAttentionCount = useMemo(
    () => agentRuns.filter((run) => ACTIVE_RUN_STATUSES.has(run.status) && isPausedAgentRun(run)).length,
    [agentRuns],
  );

  const { data: teammatePresence = [] } = useSupportTeammatePresence(workspaceId ?? '', hasSupportModule);
  const updateMyPresence = useUpdateMySupportTeammatePresence(workspaceId ?? '');
  const mySupportPresence = useMemo(
    () => (user?.id ? teammatePresence.find((entry) => entry.user_id === user.id) ?? null : null),
    [teammatePresence, user?.id],
  );
  const selectedSupportPresenceMode = mySupportPresence?.manual_status ?? 'auto';

  const { teams: allTeams } = useWorkspaceTeams(workspaceId);
  const myTeamMemberships = access?.team_memberships ?? [];
  const teams = useMemo(() => {
    if (isAdmin) {
      return allTeams;
    }

    const myTeamIds = new Set(myTeamMemberships.map((membership) => membership.team_id));
    return allTeams.filter((team) => myTeamIds.has(team.id));
  }, [allTeams, myTeamMemberships, isAdmin]);

  const [expandedTeams, setExpandedTeams] = useState<Set<string>>(() =>
    workspaceId ? getExpandedTeams(workspaceId) : new Set(),
  );
  const [collapsedSettingsGroups, setCollapsedSettingsGroups] = useState<Set<string>>(getCollapsedSettingsGroups);

  useEffect(() => {
    if (!workspaceId) {
      setExpandedTeams(new Set());
      return;
    }

    setExpandedTeams(getExpandedTeams(workspaceId));
  }, [workspaceId]);

  useEffect(() => {
    if (workspaceId) {
      saveExpandedTeams(workspaceId, expandedTeams);
    }
  }, [expandedTeams, workspaceId]);

  const toggleTeam = (teamId: string) => {
    setExpandedTeams((previous) => {
      const next = new Set(previous);
      if (next.has(teamId)) {
        next.delete(teamId);
      } else {
        next.add(teamId);
      }
      return next;
    });
  };

  const toggleSettingsGroup = (groupLabel: string) => {
    setCollapsedSettingsGroups((previous) => {
      const next = new Set(previous);
      if (next.has(groupLabel)) {
        next.delete(groupLabel);
      } else {
        next.add(groupLabel);
      }
      saveCollapsedSettingsGroups(next);
      return next;
    });
  };

  const panelNavGroups = useMemo(
    () => buildPanelNavGroups(wsSlug, canManageSettings, permissionSet, agentAttentionCount),
    [wsSlug, canManageSettings, permissionSet, agentAttentionCount],
  );
  const currentNavGroups = panelNavGroups[activeRail];
  const railItems = useMemo(() => buildRailItems(wsSlug, totalSupportUnread), [wsSlug, totalSupportUnread]);

  useEffect(() => {
    if (activeRail !== 'settings') {
      return;
    }

    for (const group of currentNavGroups) {
      if (!COLLAPSIBLE_SETTINGS_GROUPS.has(group.label)) {
        continue;
      }

      const hasActiveItem = group.items.some((item) => isActive(item.link));
      if (hasActiveItem && collapsedSettingsGroups.has(group.label)) {
        setCollapsedSettingsGroups((previous) => {
          const next = new Set(previous);
          next.delete(group.label);
          saveCollapsedSettingsGroups(next);
          return next;
        });
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, activeRail, currentNavGroups, collapsedSettingsGroups]);

  const activeTeamParam = useMemo(() => {
    const search = location.search as Record<string, string | undefined>;
    return search.team ?? null;
  }, [location.search]);

  useEffect(() => {
    if (!activeTeamParam) {
      return;
    }

    setExpandedTeams((previous) => {
      if (previous.has(activeTeamParam)) {
        return previous;
      }

      const next = new Set(previous);
      next.add(activeTeamParam);
      return next;
    });
  }, [activeTeamParam]);

  const primaryCreate = useMemo(() => {
    const segments = location.pathname.split('/').filter(Boolean);
    if (segments[0] === 'w' && segments[2] === 'pm') {
      const sub = segments[3];
      const match = projectCreateOptions.find((option) => option.pages.includes(sub));
      if (match) {
        return match;
      }
    }

    return projectCreateOptions[0];
  }, [location.pathname]);

  const secondaryOptions = projectCreateOptions.filter((option) => option.key !== primaryCreate.key);

  const isActive = (link: string) => isSidebarLinkActive(location.pathname, location.search as Record<string, string | undefined>, link);

  const isTeamSubActive = (teamId: string, subPath: string) =>
    isTeamSubLinkActive(isActive, wsSlug, activeTeamParam, teamId, subPath);

  const handleNavigate = (args: SidebarNavigateTarget) => {
    if (typeof args === 'string') {
      navigate({ to: args as string });
      return;
    }

    navigate({
      to: args.to as string,
      params: args.params,
      search: args.search as never,
    });
  };

  return (
    <ShellSidebar collapsible="offcanvas" className="border-r border-border/70 bg-[#f0f0f2] dark:border-transparent dark:bg-sidebar">
      <SidebarHeader className="relative p-2 after:absolute after:right-2 after:bottom-0 after:left-2 after:h-px after:bg-border/70 after:[mask-image:linear-gradient(to_right,transparent,black_24px,black_calc(100%-24px),transparent)] dark:after:bg-sidebar-border">
        <div className="flex items-center gap-1">
          <div className="min-w-0 flex-1">
            <WorkspaceSwitcher />
          </div>
          <NotificationCenter />
        </div>
      </SidebarHeader>

      <SidebarContent className="gap-0">
        <div className="flex min-h-0 flex-1">
          <SidebarRail
            railItems={railItems}
            activeRail={activeRail}
            userEmail={user?.email ?? undefined}
            accessibleModules={modules}
            theme={theme}
            onRailSelect={(link) => handleNavigate(link)}
            onToggleTheme={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
            accountMenu={(
              <SidebarAccountMenu
                user={user}
                initials={initials}
                presence={mySupportPresence}
                selectedPresenceMode={selectedSupportPresenceMode}
                onPresenceChange={(value) => updateMyPresence.mutate(value === 'auto' ? null : value)}
                onProfile={() => handleNavigate({ to: '/w/$slug/settings/$section', params: { slug: wsSlug, section: 'profile' } })}
                onSettings={() => handleNavigate({ to: '/w/$slug/settings/$section', params: { slug: wsSlug, section: 'general' } })}
                onWorkspaces={() => handleNavigate('/workspaces')}
                onSignOut={signOut}
              />
            )}
          />

          <div className="min-w-0 flex-1 overflow-y-auto p-2 pb-16">
            {activeRail === 'projects' && (
              <SidebarCreateBar
                primaryLabel={primaryCreate.label}
                onPrimaryClick={() => openCreate(primaryCreate.key, activeTeamParam ? { teamId: activeTeamParam } : undefined)}
                options={secondaryOptions.map((option) => ({
                  key: option.key,
                  label: option.label,
                  icon: option.icon,
                  onSelect: () => openCreate(option.key, activeTeamParam ? { teamId: activeTeamParam } : undefined),
                }))}
              />
            )}

            {activeRail === 'docs' && (
              <SidebarCreateBar
                primaryLabel="Document"
                onPrimaryClick={() => {
                  const spaceMatch = location.pathname.match(/\/docs\/spaces\/([^/]+)/);
                  openCreate('docs_document', { spaceId: spaceMatch?.[1] });
                }}
                options={[
                  { key: 'docs_space', label: 'Space', onSelect: () => openCreate('docs_space') },
                  { key: 'docs_collection', label: 'Collection', onSelect: () => openCreate('docs_collection') },
                ]}
              />
            )}

            {activeRail === 'settings' ? (
              <SettingsRailNav
                groups={currentNavGroups}
                isActive={isActive}
                collapsedGroups={collapsedSettingsGroups}
                toggleGroup={toggleSettingsGroup}
                onNavigate={(link) => handleNavigate(link)}
              />
            ) : activeRail === 'crm' ? (
              <CrmRailNav
                groups={currentNavGroups}
                isActive={isActive}
                wsSlug={wsSlug}
                onNavigate={(link) => handleNavigate(link)}
                onNavigateTo={(to) => navigate({ to })}
              />
            ) : (
              <StandardRailNav
                groups={currentNavGroups}
                isActive={isActive}
                onNavigate={(link) => handleNavigate(link)}
              />
            )}

            {activeRail === 'support' && (
              <SupportRailNav
                navFilter={navFilter}
                unreadStats={unreadStats}
                inboxScopes={inboxScopes}
                selectedMailboxId={selectedMailboxId}
                canManageSettings={canManageSettings}
                wsSlug={wsSlug}
                pathname={location.pathname}
                onNavFilterChange={(filter) => {
                  const nextSearch = buildSupportInboxSearch({
                    navFilter: filter,
                    selectedMailboxId: 'all',
                    statusFilter: defaultStatusForSupportFilter(filter),
                    searchQuery,
                  });
                  setNavFilter(filter);
                  navigate({ to: `/w/${wsSlug}/support`, search: nextSearch });
                }}
                onMailboxSelect={(id) => {
                  const nextSearch = buildSupportInboxSearch({
                    navFilter: 'all',
                    selectedMailboxId: id,
                    statusFilter,
                    searchQuery,
                  });
                  setSelectedMailboxId(id);
                  navigate({ to: `/w/${wsSlug}/support`, search: nextSearch });
                }}
                onCreateMailbox={() => { setEditMailboxId(null); setTeamInboxDialogOpen(true); }}
                onEditMailbox={(id) => { setEditMailboxId(id); setTeamInboxDialogOpen(true); }}
                onArchiveMailbox={(id) => archiveMailbox.mutate(id)}
                onNavigate={(to) => navigate({ to })}
              />
            )}

            {activeRail === 'projects' && (
              <ProjectsTeamsNav
                wsSlug={wsSlug}
                teams={teams}
                expandedTeams={expandedTeams}
                isTeamSubActive={isTeamSubActive}
                toggleTeam={toggleTeam}
                onNavigate={handleNavigate}
                canManageTeams={canManageTeams}
              />
            )}

            {activeRail === 'docs' && (
              <DocsSpacesNav
                wsId={workspaceId ?? ''}
                wsSlug={wsSlug}
                expandedTeams={expandedTeams}
                setExpandedTeams={setExpandedTeams}
                isActive={isActive}
                openCreate={openCreate}
                onNavigate={handleNavigate}
              />
            )}
          </div>
        </div>
      </SidebarContent>
    </ShellSidebar>
  );
}
