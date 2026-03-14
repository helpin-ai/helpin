import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Collapsible } from 'radix-ui';
import { ICON_MAP } from '@/components/ui/icon-picker';
import {
  BarChart3,
  Bell,
  Bot,
  Briefcase,
  Building2,
  ChevronDown,
  ClipboardCheck,
  Clock,
  ChevronRight,
  DollarSign,
  FileText,
  FolderKanban,
  FolderOpen,
  GanttChart,
  Globe,
  EllipsisVertical,
  Import,
  Layers,
  Lightbulb,
  LogOut,
  Mail,
  MessageSquare,
  Inbox,
  LayoutList,
  Moon,
  Play,
  Plus,
  RefreshCw,
  Settings,
  Settings2,
  Sliders,
  Sparkles,
  SquareKanban,
  Sun,
  Tag,
  Target,
  User,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useTheme } from 'next-themes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceAccess, usePermissions, useDocsSpaces, useDocsCollections, useDocsDocuments } from '@/hooks/queries';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { getInitials } from '@/lib/utils';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
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
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';
import { NotificationCenter } from '@/components/notifications/NotificationCenter';

type NavItem = {
  link: string;
  label: string;
  icon: LucideIcon;
};

type NavGroup = {
  label: string;
  items: NavItem[];
};

type RailId = 'projects' | 'support' | 'crm' | /* 'rewards' | */ 'docs' | 'settings';

type RailItem = {
  id: RailId;
  label: string;
  icon: LucideIcon;
  defaultLink: string;
};

function deriveActiveRail(pathname: string): RailId {
  if (pathname.includes('/settings')) return 'settings';
  if (pathname.includes('/support')) return 'support';
  if (pathname.includes('/crm')) return 'crm';
  if (pathname.includes('/pm/') || pathname.endsWith('/pm')) return 'projects';
  if (pathname.includes('/docs')) return 'docs';
  return 'projects';
}

// ── localStorage helpers for expanded teams ──

function getExpandedTeams(wsId: string): Set<string> {
  try {
    const raw = localStorage.getItem(`pm_sidebar_expanded_teams_${wsId}`);
    if (raw) return new Set(JSON.parse(raw));
  } catch {}
  return new Set();
}

function saveExpandedTeams(wsId: string, teams: Set<string>) {
  try {
    localStorage.setItem(`pm_sidebar_expanded_teams_${wsId}`, JSON.stringify([...teams]));
  } catch {}
}

// ── Settings group collapse persistence ──
const COLLAPSIBLE_SETTINGS_GROUPS = new Set(['Project Settings', 'Support & Docs', 'CRM Settings', 'AI & Automations', 'Data']);

function getCollapsedSettingsGroups(): Set<string> {
  try {
    const raw = localStorage.getItem('settings_sidebar_collapsed');
    if (raw) return new Set(JSON.parse(raw));
  } catch {}
  // Default: all collapsible groups start collapsed
  return new Set(COLLAPSIBLE_SETTINGS_GROUPS);
}

function saveCollapsedSettingsGroups(groups: Set<string>) {
  try {
    localStorage.setItem('settings_sidebar_collapsed', JSON.stringify([...groups]));
  } catch {}
}

// ── Team sub-items config ──

const teamSubItems: { key: string; label: string; icon: LucideIcon; path: string }[] = [
  { key: 'stories', label: 'Stories', icon: LayoutList, path: 'stories' },
  { key: 'sprints', label: 'Sprints', icon: RefreshCw, path: 'sprints' },
  { key: 'epics', label: 'Epics', icon: Layers, path: 'epics' },
];

// ── Docs space collections (fetches only when parent space is expanded) ──

function DocsSpaceCollections({ wsId, spaceId, wsSlug, navigate, isActive, openCreate }: {
  wsId: string;
  spaceId: string;
  wsSlug: string;
  navigate: ReturnType<typeof useNavigate>;
  isActive: (link: string) => boolean;
  openCreate: (modal: 'docs_collection', options?: { spaceId?: string }) => void;
}) {
  const { data: collections } = useDocsCollections(wsId, spaceId);
  const { data: documents } = useDocsDocuments(wsId, { space_id: spaceId });
  const uncollectedCount = (documents ?? []).filter((d) => !d.collection_id).length;
  const uncollectedLink = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=__uncollected__`;

  return (
    <SidebarMenuSub>
      {(collections ?? []).map((col) => {
        const link = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=${col.id}`;
        return (
          <SidebarMenuSubItem key={col.id}>
            <Tooltip>
              <TooltipTrigger asChild>
                <SidebarMenuSubButton
                  asChild
                  size="sm"
                  isActive={isActive(link)}
                >
                  <a
                    href={link}
                    onClick={(event) => {
                      event.preventDefault();
                      navigate({
                        to: '/w/$slug/docs/spaces/$spaceId' as string,
                        params: { slug: wsSlug, spaceId },
                        search: { collection: col.id } as Record<string, string>,
                      });
                    }}
                  >
                    <SidebarCollectionIcon name={col.icon} />
                    <span className="truncate">{col.name}</span>
                  </a>
                </SidebarMenuSubButton>
              </TooltipTrigger>
              <TooltipContent side="right" align="center">
                {col.name}
              </TooltipContent>
            </Tooltip>
          </SidebarMenuSubItem>
        );
      })}
      {uncollectedCount > 0 && (
        <SidebarMenuSubItem>
          <Tooltip>
            <TooltipTrigger asChild>
              <SidebarMenuSubButton
                asChild
                size="sm"
                isActive={isActive(uncollectedLink)}
              >
                <a
                  href={uncollectedLink}
                  onClick={(event) => {
                    event.preventDefault();
                    navigate({
                      to: '/w/$slug/docs/spaces/$spaceId' as string,
                      params: { slug: wsSlug, spaceId },
                      search: { collection: '__uncollected__' } as Record<string, string>,
                    });
                  }}
                >
                  <Inbox className="h-3.5 w-3.5" />
                  <span className="truncate">Uncategorized</span>
                </a>
              </SidebarMenuSubButton>
            </TooltipTrigger>
            <TooltipContent side="right" align="center">
              Uncategorized ({uncollectedCount})
            </TooltipContent>
          </Tooltip>
        </SidebarMenuSubItem>
      )}
      <SidebarMenuSubItem>
        <SidebarMenuSubButton
          size="sm"
          className="text-muted-foreground/70 hover:text-foreground cursor-pointer"
          onClick={() => openCreate('docs_collection', { spaceId })}
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Add collection</span>
        </SidebarMenuSubButton>
      </SidebarMenuSubItem>
    </SidebarMenuSub>
  );
}

// ── Docs spaces nav (extracted to avoid conditional hook calls) ──

function DocsSpacesNav({ wsId, wsSlug, navigate, isActive, expandedTeams, toggleTeam: _toggleTeam, setExpandedTeams, openCreate }: {
  wsId: string;
  wsSlug: string;
  navigate: ReturnType<typeof useNavigate>;
  isActive: (link: string) => boolean;
  expandedTeams: Set<string>;
  toggleTeam: (id: string) => void;
  setExpandedTeams: React.Dispatch<React.SetStateAction<Set<string>>>;
  openCreate: (modal: 'docs_collection', options?: { spaceId?: string }) => void;
}) {
  const { data: spaces } = useDocsSpaces(wsId);

  const toggleDocSpace = (spaceKey: string) => {
    setExpandedTeams((prev) => {
      const next = new Set(prev);
      // Close all other doc spaces
      for (const key of prev) {
        if (key.startsWith('docs_space_') && key !== spaceKey) {
          next.delete(key);
        }
      }
      // Toggle the clicked one
      if (next.has(spaceKey)) next.delete(spaceKey);
      else next.add(spaceKey);
      return next;
    });
  };

  if (!spaces || spaces.length === 0) return null;

  const internalSpaces = spaces.filter((s) => s.type === 'internal');
  const externalSpaces = spaces.filter((s) => s.type === 'external_capable');

  const renderSpaceItem = (space: typeof spaces[number]) => {
    const spaceKey = `docs_space_${space.id}`;
    const isExpanded = expandedTeams.has(spaceKey);
    const spaceLink = `/w/${wsSlug}/docs/spaces/${space.id}`;

    return (
      <Collapsible.Root
        key={space.id}
        asChild
        open={isExpanded}
      >
        <SidebarMenuItem>
          <div className="group/space relative flex items-center">
            <Tooltip>
              <TooltipTrigger asChild>
                <SidebarMenuButton
                  className="h-8 rounded-md px-2 flex-1"
                  isActive={isActive(spaceLink)}
                  onClick={() => {
                    toggleDocSpace(spaceKey);
                    navigate({ to: '/w/$slug/docs/spaces/$spaceId' as string, params: { slug: wsSlug, spaceId: space.id } });
                  }}
                >
                  <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                  <span className="truncate">{space.name}</span>
                </SidebarMenuButton>
              </TooltipTrigger>
              <TooltipContent side="right" align="center">
                {space.name}
              </TooltipContent>
            </Tooltip>
          </div>
          <Collapsible.Content>
            <DocsSpaceCollections
              wsId={wsId}
              spaceId={space.id}
              wsSlug={wsSlug}
              navigate={navigate}
              isActive={isActive}
              openCreate={openCreate}
            />
          </Collapsible.Content>
        </SidebarMenuItem>
      </Collapsible.Root>
    );
  };

  return (
    <>
      {internalSpaces.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            Internal Spaces
          </SidebarGroupLabel>
          <SidebarMenu>
            {internalSpaces.map(renderSpaceItem)}
          </SidebarMenu>
        </SidebarGroup>
      )}
      {externalSpaces.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            External Spaces
          </SidebarGroupLabel>
          <SidebarMenu>
            {externalSpaces.map(renderSpaceItem)}
          </SidebarMenu>
        </SidebarGroup>
      )}
    </>
  );
}

function SidebarCollectionIcon({ name }: { name?: string | null }) {
  if (name) {
    const Icon = ICON_MAP[name];
    if (Icon) return <Icon className="h-3.5 w-3.5" />;
  }
  return <FolderOpen className="h-3.5 w-3.5" />;
}

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();

  const { user, signOut } = useAuthStore();
  const { theme, setTheme } = useTheme();
  const wsSlug = currentWorkspace?.slug ?? '';
  const workspaceId = currentWorkspace?.id;
  const activeRail = deriveActiveRail(location.pathname);
  const openCreate = useGlobalCreateStore((s) => s.openCreate);
  const initials = getInitials(user?.full_name || user?.email);
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { isAdmin, canManageSettings } = usePermissions(access);

  const { teams: allTeams } = useWorkspaceTeams(workspaceId);
  const myTeamMemberships = access?.team_memberships ?? [];
  const teams = useMemo(() => {
    if (isAdmin) return allTeams;
    const myTeamIds = new Set(myTeamMemberships.map((m) => m.team_id));
    return allTeams.filter((t) => myTeamIds.has(t.id));
  }, [allTeams, myTeamMemberships, isAdmin]);

  // ── Expanded teams state ──
  const [expandedTeams, setExpandedTeams] = useState<Set<string>>(() =>
    workspaceId ? getExpandedTeams(workspaceId) : new Set()
  );

  const toggleTeam = (teamId: string) => {
    setExpandedTeams((prev) => {
      const next = new Set(prev);
      if (next.has(teamId)) next.delete(teamId);
      else next.add(teamId);
      if (workspaceId) saveExpandedTeams(workspaceId, next);
      return next;
    });
  };

  // ── Collapsed settings groups state ──
  const [collapsedSettingsGroups, setCollapsedSettingsGroups] = useState<Set<string>>(getCollapsedSettingsGroups);

  const toggleSettingsGroup = (groupLabel: string) => {
    setCollapsedSettingsGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupLabel)) next.delete(groupLabel);
      else next.add(groupLabel);
      saveCollapsedSettingsGroups(next);
      return next;
    });
  };

  // Auto-expand settings group containing the active section
  useEffect(() => {
    if (activeRail !== 'settings') return;
    const currentNavGroups = panelNavGroups['settings'];
    for (const group of currentNavGroups) {
      if (!COLLAPSIBLE_SETTINGS_GROUPS.has(group.label)) continue;
      const hasActiveItem = group.items.some((item) => isActive(item.link));
      if (hasActiveItem && collapsedSettingsGroups.has(group.label)) {
        setCollapsedSettingsGroups((prev) => {
          const next = new Set(prev);
          next.delete(group.label);
          saveCollapsedSettingsGroups(next);
          return next;
        });
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, activeRail]);

  // ── Detect active team from URL search params ──
  const activeTeamParam = useMemo(() => {
    const search = location.search as Record<string, string | undefined>;
    return search.team ?? null;
  }, [location.search]);

  // ── Auto-expand team when navigating via direct URL ──
  useEffect(() => {
    if (activeTeamParam && workspaceId) {
      setExpandedTeams((prev) => {
        if (prev.has(activeTeamParam)) return prev;
        const next = new Set(prev);
        next.add(activeTeamParam);
        saveExpandedTeams(workspaceId, next);
        return next;
      });
    }
  }, [activeTeamParam, workspaceId]);

  const createOptions = [
    { key: 'story' as const, label: 'Story', icon: SquareKanban, pages: ['stories'] },
    { key: 'epic' as const, label: 'Epic', icon: Layers, pages: ['epics'] },
    { key: 'sprint' as const, label: 'Sprint', icon: RefreshCw, pages: ['sprints'] },
    { key: 'objective' as const, label: 'Objective', icon: Target, pages: ['objectives'] },
  ];

  const primaryCreate = useMemo(() => {
    const segments = location.pathname.split('/').filter(Boolean);
    if (segments[0] === 'w' && segments[2] === 'pm') {
      const sub = segments[3];
      const match = createOptions.find((o) => o.pages.includes(sub));
      if (match) return match;
    }
    return createOptions[0];
  }, [location.pathname]);

  const secondaryOptions = createOptions.filter((o) => o.key !== primaryCreate.key);

  const railItems: RailItem[] = [
    { id: 'projects', label: 'Projects', icon: FolderKanban, defaultLink: `/w/${wsSlug}/pm/my-work` },
    { id: 'crm', label: 'CRM', icon: Briefcase, defaultLink: `/w/${wsSlug}/crm/contacts` },
    { id: 'support', label: 'Support', icon: MessageSquare, defaultLink: `/w/${wsSlug}/support` },
// { id: 'rewards', label: 'Rewards', icon: Award, defaultLink: `/w/${wsSlug}/dashboard` },
    { id: 'docs', label: 'Docs', icon: FileText, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'settings', label: 'Settings', icon: Settings, defaultLink: `/w/${wsSlug}/settings/profile` },
  ];

  const panelNavGroups: Record<RailId, NavGroup[]> = {
    projects: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/my-work`, label: 'My Work', icon: ClipboardCheck },
          { link: `/w/${wsSlug}/pm/objectives`, label: 'Objectives', icon: Target },
          { link: `/w/${wsSlug}/pm/roadmap`, label: 'Roadmap', icon: GanttChart },
          { link: `/w/${wsSlug}/pm/reports`, label: 'Reports', icon: BarChart3 },
          { link: `/w/${wsSlug}/pm/agents`, label: 'Agents', icon: Bot },
        ],
      },
    ],
    crm: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/crm/contacts`, label: 'Contacts', icon: Users },
          { link: `/w/${wsSlug}/crm/companies`, label: 'Companies', icon: Building2 },
          { link: `/w/${wsSlug}/crm/deals`, label: 'Deals', icon: DollarSign },
          { link: `/w/${wsSlug}/crm/lists`, label: 'Lists', icon: LayoutList },
          { link: `/w/${wsSlug}/crm/sequences`, label: 'Sequences', icon: Play },
          { link: `/w/${wsSlug}/crm/review`, label: 'Review', icon: ClipboardCheck },
          { link: `/w/${wsSlug}/crm/insights`, label: 'Insights', icon: Lightbulb },
        ],
      },
    ],
    support: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/support`, label: 'All Conversations', icon: MessageSquare },
        ],
      },
    ],
    /* rewards: [
      {
        label: 'Workspace',
        items: [
          { link: `/w/${wsSlug}/dashboard`, label: 'Dashboard', icon: LayoutDashboard },
          { link: `/w/${wsSlug}/goals`, label: 'Company Goals', icon: Target },
          { link: `/w/${wsSlug}/team-goals`, label: 'Team Goals', icon: Users },
          { link: `/w/${wsSlug}/sprints`, label: 'Sprints', icon: Calendar },
        ],
      },
      {
        label: 'Performance',
        items: [
          { link: `/w/${wsSlug}/bonus`, label: 'Bonus Dashboard', icon: DollarSign },
          { link: `/w/${wsSlug}/my-quarter`, label: 'My Quarter', icon: User },
        ],
      },
    ], */
    docs: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/docs/recent`, label: 'Recent Docs', icon: Clock },
          { link: `/w/${wsSlug}/docs/my`, label: 'My Documents', icon: User },
          { link: `/w/${wsSlug}/docs`, label: 'All Docs', icon: FileText },
        ],
      },
    ],
    settings: [
      {
        label: 'My Account',
        items: [
          { link: `/w/${wsSlug}/settings/profile`, label: 'Profile', icon: User },
          { link: `/w/${wsSlug}/settings/account`, label: 'Account', icon: Building2 },
          { link: `/w/${wsSlug}/settings/notifications`, label: 'Notifications', icon: Bell },
        ],
      },
      {
        label: 'Workspace',
        items: [
          { link: `/w/${wsSlug}/settings/general`, label: 'General', icon: Settings2 },
          { link: `/w/${wsSlug}/settings/members`, label: 'Members', icon: Users },
          { link: `/w/${wsSlug}/settings/teams`, label: 'Teams', icon: Users },
        ],
      },
      {
        label: 'Project Settings',
        items: [
          { link: `/w/${wsSlug}/settings/labels`, label: 'Labels', icon: Tag },
          { link: `/w/${wsSlug}/settings/story-templates`, label: 'Story Templates', icon: FileText },
          { link: `/w/${wsSlug}/settings/automations`, label: 'Automations', icon: RefreshCw },
          { link: `/w/${wsSlug}/settings/delivery`, label: 'Delivery', icon: Globe },
          { link: `/w/${wsSlug}/settings/ai`, label: 'AI', icon: Bot },
        ],
      },
      {
        label: 'Support & Docs',
        items: [
          { link: `/w/${wsSlug}/settings/helpcenter`, label: 'Help Center', icon: Globe },
          { link: `/w/${wsSlug}/settings/chat-general`, label: 'Chat Widget', icon: MessageSquare },
          { link: `/w/${wsSlug}/settings/chat-ai`, label: 'AI & Routing', icon: Bot },
        ],
      },
      {
        label: 'CRM Settings',
        items: [
          { link: `/w/${wsSlug}/settings/crm-pipelines`, label: 'Pipelines', icon: FolderKanban },
          { link: `/w/${wsSlug}/settings/crm-email`, label: 'Email Accounts', icon: Mail },
          { link: `/w/${wsSlug}/settings/crm-autonomy`, label: 'Autonomy', icon: Sliders },
        ],
      },
      ...(canManageSettings ? [{
        label: 'AI & Automations',
        items: [
          { link: `/w/${wsSlug}/settings/ai-automations`, label: 'Inventory & Health', icon: Sparkles },
        ],
      }] : []),
      {
        label: 'Data',
        items: [
          { link: `/w/${wsSlug}/settings/import`, label: 'Import / Export', icon: Import },
        ],
      },
      /* {
        label: 'Reward Settings',
        items: [
          { link: `/w/${wsSlug}/settings/system`, label: 'Reward Defaults', icon: Settings2 },
          { link: `/w/${wsSlug}/settings/people`, label: 'People', icon: UserPlus },
          { link: `/w/${wsSlug}/settings/jobroles`, label: 'Job Roles', icon: Briefcase },
          { link: `/w/${wsSlug}/settings/tiers`, label: 'Bonus Tiers', icon: Award },
        ],
      }, */
    ],
  };

  const currentNavGroups = panelNavGroups[activeRail];
  const showProjects = false; // activeRail === 'rewards';

  const projectNames = useMemo(() => {
    return currentWorkspace?.name ? [currentWorkspace.name] : [];
  }, [currentWorkspace?.name]);

  const isActive = (link: string) => {
    const [linkPath, linkQuery] = link.split('?');
    const search = location.search as Record<string, string | undefined>;

    if (linkQuery) {
      // Link has query params (e.g. collection=abc) — must match pathname + param value
      if (location.pathname !== linkPath) return false;
      const params = new URLSearchParams(linkQuery);
      for (const [key, value] of params.entries()) {
        if (search[key] !== value) return false;
      }
      return true;
    }
    // Link has no query params — match pathname but NOT if a collection param is active
    if (location.pathname === linkPath) {
      if (search.collection) return false;
      return true;
    }
    // Don't prefix-match section roots (e.g. /docs) — they'd match every sub-page
    if (link.endsWith('/docs') || link.endsWith('/pm') || link.endsWith('/support')) return false;
    return location.pathname.startsWith(`${linkPath}/`);
  };


  const isTeamSubActive = (teamId: string, subPath: string) => {
    return isActive(`/w/${wsSlug}/pm/${subPath}`) && activeTeamParam === teamId;
  };

  return (
    <ShellSidebar collapsible="offcanvas" className="border-r border-border/70 bg-[#f0f0f2] dark:border-transparent dark:bg-sidebar">
      <SidebarHeader className="border-b border-border/70 dark:border-sidebar-border p-2">
        <div className="flex items-center gap-1">
          <div className="flex-1 min-w-0">
            <WorkspaceSwitcher />
          </div>
          <NotificationCenter />
        </div>
      </SidebarHeader>

      <SidebarContent className="gap-0">
        <div className="flex min-h-0 flex-1">
          <div className="flex w-16 shrink-0 flex-col border-r border-border/70 dark:border-sidebar-border py-2">
            <div className="flex flex-1 flex-col items-center gap-1.5">
              {railItems.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    aria-label={item.label}
                    onClick={() => navigate({ to: item.defaultLink as string })}
                    className={`flex w-12 flex-col items-center justify-center gap-0.5 rounded-md px-1.5 py-2 transition-colors ${
                      activeRail === item.id
                        ? 'bg-foreground/10 text-foreground'
                        : 'text-muted-foreground hover:bg-muted/80 hover:text-foreground'
                    }`}
                  >
                    <item.icon className="h-3.5 w-3.5" />
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
                    className="rounded-full transition-opacity hover:opacity-80"
                    aria-label="Account menu"
                  >
                    <Avatar className="size-8">
                      <AvatarFallback className="text-[11px]">
                        {initials}
                      </AvatarFallback>
                    </Avatar>
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="end" className="w-56">
                  <DropdownMenuLabel className="truncate">
                    {user?.full_name || user?.email || 'Account'}
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => navigate({ to: '/w/$slug/settings/$section', params: { slug: wsSlug, section: 'profile' } })}>
                    <User className="h-4 w-4" />
                    <span>Profile</span>
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => navigate({ to: '/workspaces' })}>
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

          <div className="min-w-0 flex-1 overflow-y-auto p-2">
            {activeRail === 'projects' && (
              <div className="mb-2 flex w-full">
                <Button
                  size="sm"
                  className="h-7 flex-1 rounded-r-none text-xs gap-1.5"
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
                    {secondaryOptions.map((opt) => (
                      <DropdownMenuItem key={opt.key} onClick={() => openCreate(opt.key, activeTeamParam ? { teamId: activeTeamParam } : undefined)}>
                        <opt.icon className="h-4 w-4" />
                        {opt.label}
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
                  className="h-7 flex-1 rounded-r-none text-xs gap-1.5"
                  onClick={() => openCreate('docs_document')}
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
                      <Globe className="h-4 w-4" />
                      Space
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => openCreate('docs_collection')}>
                      <FolderOpen className="h-4 w-4" />
                      Collection
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}

            {currentNavGroups.map((group, idx) => {
              const isCollapsible = activeRail === 'settings' && COLLAPSIBLE_SETTINGS_GROUPS.has(group.label);
              const isOpen = !collapsedSettingsGroups.has(group.label);

              const menuItems = (
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
                            navigate({ to: item.link as string });
                          }}
                        >
                          <item.icon />
                          <span>{item.label}</span>
                        </a>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              );

              if (isCollapsible) {
                return (
                  <Collapsible.Root
                    key={group.label}
                    open={isOpen}
                    onOpenChange={() => toggleSettingsGroup(group.label)}
                    asChild
                  >
                    <SidebarGroup className="p-0 pb-3">
                      <Collapsible.Trigger asChild>
                        <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90 cursor-pointer select-none hover:text-muted-foreground">
                          <span className="flex-1">{group.label}</span>
                          <ChevronRight className={`h-3 w-3 shrink-0 transition-transform duration-200 ${isOpen ? 'rotate-90' : ''}`} />
                        </SidebarGroupLabel>
                      </Collapsible.Trigger>
                      <Collapsible.Content>
                        {menuItems}
                      </Collapsible.Content>
                    </SidebarGroup>
                  </Collapsible.Root>
                );
              }

              return (
                <SidebarGroup key={group.label || idx} className="p-0 pb-3">
                  {group.label && (
                    <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
                      {group.label}
                    </SidebarGroupLabel>
                  )}
                  {menuItems}
                </SidebarGroup>
              );
            })}

            {/* ── Team-scoped navigation (projects rail only) ── */}
            {activeRail === 'projects' && (
              <SidebarGroup className="p-0 pb-3">
                <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
                  Your Teams
                </SidebarGroupLabel>
                <SidebarMenu>

                  {teams.map((team) => {
                    const isExpanded = expandedTeams.has(team.id);
                    return (
                      <Collapsible.Root
                        key={team.id}
                        asChild
                        open={isExpanded}
                        onOpenChange={() => toggleTeam(team.id)}
                      >
                        <SidebarMenuItem>
                          <div className="group/team relative flex items-center">
                            <Collapsible.Trigger asChild>
                              <SidebarMenuButton className="h-8 rounded-md px-2 flex-1">
                                <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                                <span className="truncate">{team.name}</span>
                              </SidebarMenuButton>
                            </Collapsible.Trigger>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <button
                                  type="button"
                                  className="absolute right-1 flex h-5 w-5 items-center justify-center rounded opacity-0 transition-opacity hover:bg-muted group-hover/team:opacity-100 data-[state=open]:opacity-100"
                                >
                                  <EllipsisVertical className="h-3.5 w-3.5 text-muted-foreground" />
                                </button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent side="right" align="start">
                                <DropdownMenuItem onClick={() => navigate({ to: '/w/$slug/settings/$section', params: { slug: wsSlug, section: 'teams' } })}>
                                  <Settings className="h-4 w-4" />
                                  Settings
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </div>
                          <Collapsible.Content>
                            <SidebarMenuSub>
                              {teamSubItems.map((sub) => {
                                const link = `/w/${wsSlug}/pm/${sub.path}?team=${team.id}`;
                                const active = isTeamSubActive(team.id, sub.path);
                                return (
                                  <SidebarMenuSubItem key={sub.key}>
                                    <SidebarMenuSubButton
                                      asChild
                                      size="sm"
                                      isActive={active}
                                    >
                                      <a
                                        href={link}
                                        onClick={(event) => {
                                          event.preventDefault();
                                          navigate({
                                            to: `/w/$slug/pm/${sub.path}` as string,
                                            params: { slug: wsSlug },
                                            search: { team: team.id },
                                          });
                                        }}
                                      >
                                        <sub.icon className="h-3.5 w-3.5" />
                                        <span>{sub.label}</span>
                                      </a>
                                    </SidebarMenuSubButton>
                                  </SidebarMenuSubItem>
                                );
                              })}
                            </SidebarMenuSub>
                          </Collapsible.Content>
                        </SidebarMenuItem>
                      </Collapsible.Root>
                    );
                  })}
                </SidebarMenu>
              </SidebarGroup>
            )}

            {showProjects && (
              <SidebarGroup className="p-0">
                <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
                  Projects
                </SidebarGroupLabel>
                <SidebarMenu>
                  {projectNames.map((name) => (
                    <SidebarMenuItem key={name}>
                      <SidebarMenuButton
                        isActive={name === currentWorkspace?.name}
                        className="h-8 rounded-md px-2"
                      >
                        <span className="h-2 w-2 rounded-full bg-muted-foreground/40" />
                        <span className="truncate">{name}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroup>
            )}

            {/* ── Docs spaces navigation (docs rail only) ── */}
            {activeRail === 'docs' && <DocsSpacesNav wsId={workspaceId ?? ''} wsSlug={wsSlug} navigate={navigate} isActive={isActive} expandedTeams={expandedTeams} toggleTeam={toggleTeam} setExpandedTeams={setExpandedTeams} openCreate={openCreate} />}
          </div>
        </div>
      </SidebarContent>
    </ShellSidebar>
  );
}
