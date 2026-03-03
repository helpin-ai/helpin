import { useMemo } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  Award,
  BarChart3,
  Briefcase,
  Calendar,
  DollarSign,
  FileText,
  FolderKanban,
  GanttChart,
  Layers,
  LayoutDashboard,
  LayoutList,
  RefreshCw,
  Settings,
  Settings2,
  Target,
  User,
  UserPlus,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
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

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace, workspaces } = useWorkspaceStore();

  const wsSlug = currentWorkspace?.slug ?? '';
  const activeRail = deriveActiveRail(location.pathname);

  const railItems: RailItem[] = [
    { id: 'projects', label: 'Projects', icon: FolderKanban, defaultLink: `/w/${wsSlug}/pm/stories` },
    { id: 'rewards', label: 'Rewards', icon: Award, defaultLink: `/w/${wsSlug}/dashboard` },
    { id: 'docs', label: 'Docs', icon: FileText, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'settings', label: 'Settings', icon: Settings, defaultLink: `/w/${wsSlug}/settings/system` },
  ];

  const panelNavGroups: Record<RailId, NavGroup[]> = {
    projects: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/stories`, label: 'Stories', icon: LayoutList },
          { link: `/w/${wsSlug}/pm/epics`, label: 'Epics', icon: Layers },
          { link: `/w/${wsSlug}/pm/iterations`, label: 'Iterations', icon: RefreshCw },
          { link: `/w/${wsSlug}/pm/objectives`, label: 'Objectives', icon: Target },
          { link: `/w/${wsSlug}/pm/roadmap`, label: 'Roadmap', icon: GanttChart },
          { link: `/w/${wsSlug}/pm/reports`, label: 'Reports', icon: BarChart3 },
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
          { link: `/w/${wsSlug}/settings/system`, label: 'General', icon: Settings2 },
        ],
      },
      {
        label: 'Workspace Settings',
        items: [
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
