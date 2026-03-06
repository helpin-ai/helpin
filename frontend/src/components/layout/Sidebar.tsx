import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Collapsible } from 'radix-ui';
import {
  Award,
  BarChart3,
  Bot,
  Briefcase,
  Calendar,
  ChevronDown,
  ChevronRight,
  DollarSign,
  FileText,
  FolderKanban,
  GanttChart,
  Hexagon,
  Layers,
  LayoutDashboard,
  MessageSquare,
  LayoutList,
  Plus,
  RefreshCw,
  Settings,
  Settings2,
  SquareKanban,
  Target,
  User,
  UserPlus,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
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
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';

type NavItem = {
  link: string;
  label: string;
  icon: LucideIcon;
};

type NavGroup = {
  label: string;
  items: NavItem[];
};

type RailId = 'projects' | 'rewards' | 'docs' | 'settings';

type RailItem = {
  id: RailId;
  label: string;
  icon: LucideIcon;
  defaultLink: string;
};

function deriveActiveRail(pathname: string): RailId {
  if (pathname.includes('/pm/') || pathname.endsWith('/pm')) return 'projects';
  if (pathname.includes('/docs')) return 'docs';
  if (pathname.includes('/settings')) return 'settings';
  return 'rewards';
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

// ── Team sub-items config ──

const teamSubItems: { key: string; label: string; icon: LucideIcon; path: string }[] = [
  { key: 'stories', label: 'Stories', icon: LayoutList, path: 'stories' },
  { key: 'sprints', label: 'Sprints', icon: RefreshCw, path: 'sprints' },
  { key: 'epics', label: 'Epics', icon: Layers, path: 'epics' },
];

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace, workspaces } = useWorkspaceStore();

  const wsSlug = currentWorkspace?.slug ?? '';
  const workspaceId = currentWorkspace?.id;
  const activeRail = deriveActiveRail(location.pathname);
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const { teams } = useWorkspaceTeams(workspaceId);

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

  // ── Detect active team from URL search params ──
  const activeTeamParam = useMemo(() => {
    const params = new URLSearchParams(location.search);
    return params.get('team') ?? null;
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
    { key: 'epic' as const, label: 'Epic', icon: Hexagon, pages: ['epics'] },
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
    { id: 'projects', label: 'Projects', icon: FolderKanban, defaultLink: `/w/${wsSlug}/pm/stories` },
    { id: 'rewards', label: 'Rewards', icon: Award, defaultLink: `/w/${wsSlug}/dashboard` },
    { id: 'docs', label: 'Docs', icon: FileText, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'settings', label: 'Settings', icon: Settings, defaultLink: `/w/${wsSlug}/settings/profile` },
  ];

  const panelNavGroups: Record<RailId, NavGroup[]> = {
    projects: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/roadmap`, label: 'Roadmap', icon: GanttChart },
          { link: `/w/${wsSlug}/pm/objectives`, label: 'Objectives', icon: Target },
          { link: `/w/${wsSlug}/pm/reports`, label: 'Reports', icon: BarChart3 },
          { link: `/w/${wsSlug}/pm/agents`, label: 'Agents', icon: Bot },
          { link: `/w/${wsSlug}/pm/support`, label: 'Support', icon: MessageSquare },
        ],
      },
    ],
    rewards: [
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
    ],
    docs: [
      {
        label: 'Documentation',
        items: [
          { link: `/w/${wsSlug}/docs`, label: 'All Docs', icon: FileText },
        ],
      },
    ],
    settings: [
      {
        label: 'My Account',
        items: [
          { link: `/w/${wsSlug}/settings/profile`, label: 'Profile', icon: User },
          { link: `/w/${wsSlug}/settings/system`, label: 'General', icon: Settings2 },
        ],
      },
      {
        label: 'Workspace Settings',
        items: [
          { link: `/w/${wsSlug}/settings/members`, label: 'Members', icon: Users },
          { link: `/w/${wsSlug}/settings/teams`, label: 'Teams', icon: Users },
          { link: `/w/${wsSlug}/settings/people`, label: 'People', icon: UserPlus },
          { link: `/w/${wsSlug}/settings/jobroles`, label: 'Job Roles', icon: Briefcase },
          { link: `/w/${wsSlug}/settings/workflows`, label: 'Workflows', icon: FolderKanban },
          { link: `/w/${wsSlug}/settings/workflowstates`, label: 'Workflow States', icon: LayoutList },
          { link: `/w/${wsSlug}/settings/tiers`, label: 'Bonus Tiers', icon: Award },
        ],
      },
    ],
  };

  const currentNavGroups = panelNavGroups[activeRail];
  const showProjects = activeRail === 'rewards';

  const projectNames = useMemo(() => {
    const names = workspaces.map((workspace) => workspace.name);
    if (currentWorkspace?.name && !names.includes(currentWorkspace.name)) {
      names.unshift(currentWorkspace.name);
    }
    if (names.length === 0 && currentWorkspace?.name) {
      names.push(currentWorkspace.name);
    }
    return names.slice(0, 5);
  }, [workspaces, currentWorkspace?.name]);

  const isActive = (link: string) => {
    return location.pathname === link || location.pathname.startsWith(`${link}/`);
  };

  const isTeamSubActive = (teamId: string, subPath: string) => {
    return isActive(`/w/${wsSlug}/pm/${subPath}`) && activeTeamParam === teamId;
  };

  return (
    <ShellSidebar collapsible="offcanvas" className="border-r border-border/70 bg-[#f7f7f8]">
      <SidebarHeader className="border-b border-border/70 p-2">
        <WorkspaceSwitcher />
      </SidebarHeader>

      <SidebarContent className="gap-0">
        <div className="flex min-h-0 flex-1">
          <div className="w-16 shrink-0 border-r border-border/70 py-2">
            <div className="flex flex-col items-center gap-1.5">
              {railItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  aria-label={item.label}
                  title={item.label}
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
          </div>

          <div className="min-w-0 flex-1 overflow-y-auto p-2">
            {activeRail === 'projects' && (
              <div className="mb-2 flex w-full">
                <Button
                  size="sm"
                  className="h-8 flex-1 rounded-r-none text-xs gap-1.5"
                  onClick={() => openCreate(primaryCreate.key)}
                >
                  <Plus className="h-3.5 w-3.5" />
                  {primaryCreate.label}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="sm" className="h-8 rounded-l-none border-l border-primary-foreground/20 px-1.5">
                      <ChevronDown className="h-3.5 w-3.5" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {secondaryOptions.map((opt) => (
                      <DropdownMenuItem key={opt.key} onClick={() => openCreate(opt.key)}>
                        <opt.icon className="h-4 w-4" />
                        {opt.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}

            {currentNavGroups.map((group, idx) => (
              <SidebarGroup key={group.label || idx} className="p-0 pb-3">
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
                        className="h-8 rounded-md px-2"
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
              </SidebarGroup>
            ))}

            {/* ── Team-scoped navigation (projects rail only) ── */}
            {activeRail === 'projects' && teams.length > 0 && (
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
                          <Collapsible.Trigger asChild>
                            <SidebarMenuButton className="h-8 rounded-md px-2">
                              <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                              <span className="truncate">{team.name}</span>
                            </SidebarMenuButton>
                          </Collapsible.Trigger>
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
          </div>
        </div>
      </SidebarContent>
    </ShellSidebar>
  );
}
