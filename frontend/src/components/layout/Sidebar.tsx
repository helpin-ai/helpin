import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { useTheme } from 'next-themes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useSetup, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useQuery } from '@tanstack/react-query';
import { useArchiveMailbox, useInboxScopes, useSupportBuiltinInboxViews, useSupportInboxViewCounts, useUnreadStats } from '@/hooks/queries/useSupport';
import { useDeleteSupportInboxView, useSupportInboxViews, useUpdateSupportInboxView } from '@/hooks/queries/useSupport';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { getInitials } from '@/lib/utils';
import { buildSupportInboxSearch } from '@/lib/supportInboxRouting';
import { supportInboxCountMailboxScope } from '@/lib/supportInboxFilters';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import {
  Sidebar as ShellSidebar,
  SidebarContent,
  SidebarHeader,
} from '@/components/ui/sidebar';
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';
import { TrialBanner } from '@/components/layout/TrialBanner';
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
import { SetupRailNav } from './sidebar/SetupRailNav';
import { SettingsRailNav } from './sidebar/SettingsRailNav';
import { StandardRailNav } from './sidebar/StandardRailNav';
import { CrmRailNav } from './sidebar/CrmRailNav';
import { SupportRailNav } from './sidebar/SupportRailNav';
import { isSetupSuccessEnabled } from '@/lib/featureFlags';
import type { SupportInboxView } from '@/lib/pmTypes';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';

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
  const { data: setup } = useSetup(isSetupSuccessEnabled() ? workspaceId : undefined);
  const { isAdmin, canManageSettings, canManageTeams, permissionSet, canAccessModule, modules } = usePermissions(access);
  const hasSupportModule = canAccessModule('support');
  const {
    navFilter,
    setNavFilter,
    selectedMailboxId,
    setSelectedMailboxId,
    activeCustomViewId,
    applyCustomView,
    searchQuery,
    setBuiltinViewFilters,
    setTeamInboxDialogOpen,
    setEditMailboxId,
  } = useSupportInboxStore();

  const { data: inboxScopes } = useInboxScopes(workspaceId ?? '', hasSupportModule);
  const { data: customViews = [] } = useSupportInboxViews(workspaceId ?? '', hasSupportModule);
  const { data: customViewCounts = [] } = useSupportInboxViewCounts(workspaceId ?? '', hasSupportModule);
  const { data: builtinViews } = useSupportBuiltinInboxViews(workspaceId ?? '', hasSupportModule);
  const inboxUnreadMailboxScope = supportInboxCountMailboxScope('all');
  const { data: unreadStats } = useUnreadStats(workspaceId ?? '', undefined, hasSupportModule);
  const { data: inboxUnreadStats } = useUnreadStats(workspaceId ?? '', inboxUnreadMailboxScope, hasSupportModule);
  const globalUnreadStats = unreadStats;
  const archiveMailbox = useArchiveMailbox(workspaceId ?? '');
  const updateSupportInboxView = useUpdateSupportInboxView(workspaceId ?? '');
  const deleteSupportInboxView = useDeleteSupportInboxView(workspaceId ?? '');
  const [editingSupportView, setEditingSupportView] = useState<SupportInboxView | null>(null);
  const [editingSupportViewName, setEditingSupportViewName] = useState('');
  const [editingSupportViewShared, setEditingSupportViewShared] = useState(false);
  const totalSupportUnread = useMemo(
    () => (inboxScopes?.shared_inbox.unread_count ?? 0) + (inboxScopes?.mailboxes ?? []).reduce((sum, mailbox) => sum + mailbox.unread_count, 0),
    [inboxScopes],
  );
  const sortedCustomViews = useMemo(
    () => [...customViews].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
    [customViews],
  );
  const customViewCountMap = useMemo(
    () => Object.fromEntries(customViewCounts.map((count) => [count.view_id, count])),
    [customViewCounts],
  );

  const builtinViewFilterMap = useMemo(
    () => Object.fromEntries(
      (builtinViews ?? [])
        .filter((view) => view.view_key)
        .map((view) => [view.view_key as string, view.filters]),
    ),
    [builtinViews],
  );

  useEffect(() => {
    setBuiltinViewFilters(builtinViewFilterMap);
  }, [builtinViewFilterMap, setBuiltinViewFilters]);

  const { data: agentAttentionCount = 0 } = useQuery({
    queryKey: queryKeys.automation.runAttentionCount(workspaceId ?? ''),
    queryFn: async () => {
      const res = await automationService.getRunAttentionCount(workspaceId!);
      return res.data?.count ?? 0;
    },
    enabled: !!workspaceId,
    staleTime: 30_000,
  });

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
  const [activeSetupJourney, setActiveSetupJourney] = useState<string>();

  useEffect(() => {
    setActiveSetupJourney(setup?.recommended?.journey_key ?? setup?.journeys[0]?.key);
  }, [setup?.recommended?.journey_key, setup?.journeys[0]?.key, workspaceId]);

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
    () => buildPanelNavGroups(wsSlug, canManageSettings, permissionSet, agentAttentionCount, teams),
    [wsSlug, canManageSettings, permissionSet, agentAttentionCount, teams],
  );
  const currentNavGroups = panelNavGroups[activeRail];
  const setupProgress = setup?.total_count ? Math.round((setup.completed_count / setup.total_count) * 100) : 0;
  const railItems = useMemo(
    () => buildRailItems(wsSlug, totalSupportUnread, isSetupSuccessEnabled() ? setupProgress : undefined),
    [wsSlug, totalSupportUnread, setupProgress],
  );

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

  const submitSupportViewEdit = () => {
    if (!editingSupportView || !editingSupportViewName.trim()) return;
    updateSupportInboxView.mutate({
      id: editingSupportView.id,
      name: editingSupportViewName.trim(),
      ...(canManageSettings ? { is_shared: editingSupportViewShared } : {}),
    }, {
      onSuccess: () => setEditingSupportView(null),
    });
  };

  return (
    <>
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
            ) : activeRail === 'support' ? null : (
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
                inboxUnreadStats={inboxUnreadStats}
                globalUnreadStats={globalUnreadStats}
                inboxScopes={inboxScopes}
                selectedMailboxId={selectedMailboxId}
                activeCustomViewId={activeCustomViewId}
                currentUserId={user?.id}
                customViews={sortedCustomViews}
                customViewCounts={customViewCountMap}
                canManageSettings={canManageSettings}
                wsSlug={wsSlug}
                pathname={location.pathname}
                onNavFilterChange={(filter) => {
                  const nextSearch = buildSupportInboxSearch({
                    navFilter: filter,
                    selectedMailboxId: 'all',
                    statusFilter: 'all',
                    searchQuery,
                  });
                  setNavFilter(filter);
                  navigate({ to: `/w/${wsSlug}/support`, search: nextSearch });
                }}
                onMailboxSelect={(id) => {
                  const nextSearch = buildSupportInboxSearch({
                    navFilter: 'inbox',
                    selectedMailboxId: id,
                    statusFilter: 'all',
                    searchQuery,
                  });
                  setSelectedMailboxId(id);
                  navigate({ to: `/w/${wsSlug}/support`, search: nextSearch });
                }}
                onCustomViewSelect={(view) => {
                  applyCustomView(view);
                  const next = useSupportInboxStore.getState();
                  navigate({
                    to: `/w/${wsSlug}/support`,
                    search: buildSupportInboxSearch({
                      navFilter: next.navFilter,
                      selectedMailboxId: next.selectedMailboxId,
                      statusFilter: next.statusFilter,
                      searchQuery: next.searchQuery,
                      activeCustomViewId: view.id,
                      listFilters: next.conversationListFilters,
                      includeFilterParams: false,
                    }),
                  });
                }}
                onEditCustomView={(view) => {
                  setEditingSupportView(view);
                  setEditingSupportViewName(view.name);
                  setEditingSupportViewShared(view.is_shared);
                }}
                onDeleteCustomView={(view) => {
                  if (!window.confirm(`Delete "${view.name}"?`)) return;
                  deleteSupportInboxView.mutate(view.id);
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

            {activeRail === 'setup' && setup && (
              <SetupRailNav
                journeys={setup.journeys}
                activeJourneyKey={activeSetupJourney}
                onSelect={(journeyKey) => {
                  setActiveSetupJourney(journeyKey);
                  window.dispatchEvent(new CustomEvent('setup-journey-navigate', { detail: { journeyKey } }));
                }}
              />
            )}
          </div>
        </div>
        <TrialBanner />
      </SidebarContent>
    </ShellSidebar>
    <Dialog open={!!editingSupportView} onOpenChange={(open) => !open && setEditingSupportView(null)}>
      <DialogContent aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>Edit view</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="support-custom-view-name">Name</Label>
            <Input
              id="support-custom-view-name"
              value={editingSupportViewName}
              onChange={(event) => setEditingSupportViewName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  submitSupportViewEdit();
                }
              }}
            />
          </div>
          {canManageSettings && (
            <div className="flex items-center justify-between rounded-md border px-3 py-2">
              <Label htmlFor="support-custom-view-shared" className="text-sm font-medium">
                Shared with workspace
              </Label>
              <Switch
                id="support-custom-view-shared"
                checked={editingSupportViewShared}
                onCheckedChange={setEditingSupportViewShared}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setEditingSupportView(null)}>
            Cancel
          </Button>
          <Button
            onClick={submitSupportViewEdit}
            disabled={!editingSupportViewName.trim() || updateSupportInboxView.isPending}
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    </>
  );
}
