import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  ChevronDown,
  Moon,
  Plus,
  Sun,
  User,
  Users,
  LogOut,
} from 'lucide-react';
import { useTheme } from 'next-themes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useInboxScopes, useUnreadStats } from '@/hooks/queries/useSupport';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { getInitials } from '@/lib/utils';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { isModuleEnabled } from '@/lib/featureFlags';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Sidebar as ShellSidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar';
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';
import { NotificationCenter } from '@/components/notifications/NotificationCenter';
import { useSupportTeammatePresence, useUpdateMySupportTeammatePresence } from '@/hooks/queries/useSupport';
import { DocsSpacesNav } from './sidebar/DocsSpacesNav';
import { buildPanelNavGroups, buildRailItems, deriveActiveRail, projectCreateOptions } from './sidebar/config';
import { ProjectsTeamsNav } from './sidebar/ProjectsTeamsNav';
import { COLLAPSIBLE_SETTINGS_GROUPS, getCollapsedSettingsGroups, getExpandedTeams, saveCollapsedSettingsGroups, saveExpandedTeams } from './sidebar/state';
import { SettingsRailNav } from './sidebar/SettingsRailNav';
import { SupportRailNav } from './sidebar/SupportRailNav';

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
  const { isAdmin, canManageSettings } = usePermissions(access);
  const {
    navFilter,
    setNavFilter,
    selectedMailboxId,
    setSelectedMailboxId,
    setTeamInboxDialogOpen,
  } = useSupportInboxStore();

  const { data: inboxScopes } = useInboxScopes(workspaceId ?? '');
  const { data: unreadStats } = useUnreadStats(workspaceId ?? '', selectedMailboxId);
  const totalSupportUnread = useMemo(
    () => (inboxScopes?.shared_inbox.unread_count ?? 0) + (inboxScopes?.mailboxes ?? []).reduce((sum, mailbox) => sum + mailbox.unread_count, 0),
    [inboxScopes],
  );

  const { data: teammatePresence = [] } = useSupportTeammatePresence(workspaceId ?? '');
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

  const panelNavGroups = useMemo(() => buildPanelNavGroups(wsSlug, canManageSettings), [wsSlug, canManageSettings]);
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

  const isActive = (link: string) => {
    const [linkPath, linkQuery] = link.split('?');
    const search = location.search as Record<string, string | undefined>;

    if (linkQuery) {
      if (location.pathname !== linkPath) {
        return false;
      }

      const params = new URLSearchParams(linkQuery);
      for (const [key, value] of params.entries()) {
        if (search[key] !== value) {
          return false;
        }
      }
      return true;
    }

    if (location.pathname === linkPath) {
      if (search.collection) {
        return false;
      }
      return true;
    }

    if (link.endsWith('/docs') || link.endsWith('/pm') || link.endsWith('/support')) {
      return false;
    }

    return location.pathname.startsWith(`${linkPath}/`);
  };

  const isTeamSubActive = (teamId: string, subPath: string) =>
    isActive(`/w/${wsSlug}/pm/${subPath}`) && activeTeamParam === teamId;

  const handleNavigate = (args: { to: string; params?: Record<string, string>; search?: Record<string, string> } | string) => {
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

  const renderStandardGroups = () =>
    currentNavGroups.map((group, index) => (
      <SidebarGroup key={group.label || index} className="p-0 pb-3">
        {group.label && (
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            {group.label}
          </SidebarGroupLabel>
        )}
        <SidebarMenu>
          {group.items.map((item) => (
            <SidebarMenuItem key={item.link}>
              <SidebarMenuButton
                asChild
                tooltip={item.label}
                isActive={isActive(item.link)}
                className="h-8 rounded-md px-2 text-[13px]"
              >
                <a
                  href={item.link}
                  onClick={(event) => {
                    event.preventDefault();
                    handleNavigate(item.link);
                  }}
                >
                  <item.icon />
                  <span>{item.label}</span>
                </a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </SidebarGroup>
    ));

  return (
    <ShellSidebar collapsible="offcanvas" className="border-r border-border/70 bg-[#f0f0f2] dark:border-transparent dark:bg-sidebar">
      <SidebarHeader className="border-b border-border/70 p-2 dark:border-sidebar-border">
        <div className="flex items-center gap-1">
          <div className="min-w-0 flex-1">
            <WorkspaceSwitcher />
          </div>
          <NotificationCenter />
        </div>
      </SidebarHeader>

      <SidebarContent className="gap-0">
        <div className="flex min-h-0 flex-1">
          <div className="flex w-16 shrink-0 flex-col border-r border-border/70 py-2 dark:border-sidebar-border">
            <div className="flex flex-1 flex-col items-center gap-1.5">
              {railItems
                .filter((item) => isModuleEnabled(item.id, user?.email))
                .map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    aria-label={item.label}
                    onClick={() => handleNavigate(item.defaultLink)}
                    className={`flex w-12 flex-col items-center justify-center gap-0.5 rounded-md px-1.5 py-2 transition-colors ${
                      activeRail === item.id
                        ? 'bg-foreground/10 text-foreground'
                        : 'text-muted-foreground hover:bg-muted/80 hover:text-foreground'
                    }`}
                  >
                    <div className="relative">
                      <item.icon className="h-3.5 w-3.5" />
                      {item.indicator && (
                        <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-blue-600" />
                      )}
                      {!!item.badge && (
                        <span className="absolute -right-1.5 -top-1 flex h-3.5 min-w-[14px] items-center justify-center rounded-full bg-blue-600 px-0.5 text-[9px] font-bold leading-none text-white">
                          {item.badge > 99 ? '99+' : item.badge}
                        </span>
                      )}
                    </div>
                    <span className="text-[10px] leading-none">{item.label}</span>
                  </button>
                ))}
            </div>

            <div className="flex flex-col items-center gap-1.5 pt-2">
              <button
                type="button"
                className="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
                onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                aria-label="Toggle theme"
              >
                <Sun className="h-3.5 w-3.5 rotate-0 scale-100 transition-transform dark:rotate-90 dark:scale-0" />
                <Moon className="absolute h-3.5 w-3.5 rotate-90 scale-0 transition-transform dark:rotate-0 dark:scale-100" />
              </button>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="relative rounded-full transition-opacity hover:opacity-80"
                    aria-label="Account menu"
                  >
                    <Avatar className="size-8">
                      <AvatarImage src={user?.avatar_url ?? undefined} alt={user?.full_name || user?.email || 'Account'} />
                      <AvatarFallback className="text-[11px]">
                        {initials}
                      </AvatarFallback>
                    </Avatar>
                    {mySupportPresence && (
                      <span
                        className={`absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-background ${
                          mySupportPresence.status === 'online'
                            ? 'bg-emerald-500'
                            : mySupportPresence.status === 'away'
                              ? 'bg-amber-500'
                              : 'bg-slate-300 dark:bg-slate-600'
                        }`}
                      />
                    )}
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="end" className="w-56">
                  <DropdownMenuLabel className="truncate">
                    {user?.full_name || user?.email || 'Account'}
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {mySupportPresence && (
                    <DropdownMenuSub>
                      <DropdownMenuSubTrigger>
                        <span className="flex items-center gap-2">
                          <span
                            className={`h-2 w-2 rounded-full ${
                              mySupportPresence.status === 'online'
                                ? 'bg-emerald-500'
                                : mySupportPresence.status === 'away'
                                  ? 'bg-amber-500'
                                  : 'bg-slate-300 dark:bg-slate-600'
                            }`}
                          />
                          <span>Support status</span>
                        </span>
                      </DropdownMenuSubTrigger>
                      <DropdownMenuSubContent className="w-44">
                        <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
                          Set your status
                        </DropdownMenuLabel>
                        <DropdownMenuSeparator />
                        <DropdownMenuRadioGroup
                          value={selectedSupportPresenceMode}
                          onValueChange={(value) => updateMyPresence.mutate(value === 'auto' ? null : (value as 'online' | 'away' | 'offline'))}
                        >
                          <DropdownMenuRadioItem value="auto">Automatic</DropdownMenuRadioItem>
                          <DropdownMenuRadioItem value="online">Online</DropdownMenuRadioItem>
                          <DropdownMenuRadioItem value="away">Away</DropdownMenuRadioItem>
                          <DropdownMenuRadioItem value="offline">Offline</DropdownMenuRadioItem>
                        </DropdownMenuRadioGroup>
                      </DropdownMenuSubContent>
                    </DropdownMenuSub>
                  )}
                  {mySupportPresence && <DropdownMenuSeparator />}
                  <DropdownMenuItem onClick={() => handleNavigate({ to: '/w/$slug/settings/$section', params: { slug: wsSlug, section: 'profile' } })}>
                    <User className="h-4 w-4" />
                    <span>Profile</span>
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => handleNavigate('/workspaces')}>
                    <Users className="h-4 w-4" />
                    <span>All Workspaces</span>
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={signOut} variant="destructive">
                    <LogOut className="h-4 w-4" />
                    <span>Sign out</span>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          <div className="min-w-0 flex-1 overflow-y-auto p-2 pb-16">
            {activeRail === 'projects' && (
              <div className="mb-2 flex w-full">
                <Button
                  size="sm"
                  className="h-7 flex-1 gap-1.5 rounded-r-none text-xs"
                  onClick={() => openCreate(primaryCreate.key, activeTeamParam ? { teamId: activeTeamParam } : undefined)}
                >
                  <Plus className="h-3 w-3" />
                  {primaryCreate.label}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="sm" className="h-7 rounded-l-none border-l border-primary-foreground/20 px-1.5">
                      <ChevronDown className="h-3 w-3" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {secondaryOptions.map((option) => (
                      <DropdownMenuItem key={option.key} onClick={() => openCreate(option.key, activeTeamParam ? { teamId: activeTeamParam } : undefined)}>
                        <option.icon className="h-4 w-4" />
                        {option.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}

            {activeRail === 'docs' && (
              <div className="mb-2 flex w-full">
                <Button
                  size="sm"
                  className="h-7 flex-1 gap-1.5 rounded-r-none text-xs"
                  onClick={() => {
                    const spaceMatch = location.pathname.match(/\/docs\/spaces\/([^/]+)/);
                    openCreate('docs_document', { spaceId: spaceMatch?.[1] });
                  }}
                >
                  <Plus className="h-3 w-3" />
                  Document
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="sm" className="h-7 rounded-l-none border-l border-primary-foreground/20 px-1.5">
                      <ChevronDown className="h-3 w-3" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    <DropdownMenuItem onClick={() => openCreate('docs_space')}>
                      Space
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => openCreate('docs_collection')}>
                      Collection
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}

            {activeRail === 'settings' ? (
              <SettingsRailNav
                groups={currentNavGroups}
                isActive={isActive}
                collapsedGroups={collapsedSettingsGroups}
                toggleGroup={toggleSettingsGroup}
                onNavigate={(link) => handleNavigate(link)}
              />
            ) : (
              renderStandardGroups()
            )}

            {activeRail === 'support' && (
              <SupportRailNav
                navFilter={navFilter}
                unreadStats={unreadStats}
                inboxScopes={inboxScopes}
                selectedMailboxId={selectedMailboxId}
                canManageSettings={canManageSettings}
                onNavFilterChange={setNavFilter}
                onMailboxSelect={setSelectedMailboxId}
                onCreateMailbox={() => setTeamInboxDialogOpen(true)}
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
