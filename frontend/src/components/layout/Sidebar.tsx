import { useMemo } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  BookOpenText,
  Bot,
  BriefcaseBusiness,
  Calendar,
  DollarSign,
  FolderKanban,
  LayoutDashboard,
  LogOut,
  Settings,
  Target,
  User,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import {
  Sidebar as ShellSidebar,
  SidebarContent,
  SidebarFooter,
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

type RailItem = {
  id: string;
  label: string;
  icon: LucideIcon;
  link: string;
};

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, signOut } = useAuthStore();
  const { currentWorkspace, workspaces } = useWorkspaceStore();

  const wsSlug = currentWorkspace?.slug ?? '';

  const navGroups: { label: string; items: NavItem[] }[] = [
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
  ];

  const railItems: RailItem[] = [
    { id: 'projects', label: 'Projects', icon: FolderKanban, link: `/w/${wsSlug}/dashboard` },
    { id: 'wiki', label: 'Wiki', icon: BookOpenText, link: `/w/${wsSlug}/tasks` },
    { id: 'ai', label: 'AI', icon: Bot, link: `/w/${wsSlug}/my-quarter` },
    { id: 'desk', label: 'Desk', icon: BriefcaseBusiness, link: `/w/${wsSlug}/team-goals` },
    { id: 'settings', label: 'Settings', icon: Settings, link: `/w/${wsSlug}/settings` },
  ];

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

  const initials = user?.full_name
    ? user.full_name
      .split(' ')
      .map((part) => part[0])
      .join('')
      .toUpperCase()
      .slice(0, 2)
    : user?.email?.slice(0, 2).toUpperCase() || '??';

  const isActive = (link: string) =>
    location.pathname === link || location.pathname.startsWith(`${link}/`);

  return (
    <ShellSidebar collapsible="offcanvas" className="border-r border-border/70 bg-[#f7f7f8]">
      <SidebarHeader className="border-b border-border/70 p-2">
        <WorkspaceSwitcher />
      </SidebarHeader>

      <SidebarContent className="gap-0">
        <div className="flex min-h-0 flex-1">
          <div className="w-14 shrink-0 border-r border-border/70 py-2">
            <div className="flex flex-col items-center gap-1.5">
              {railItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  aria-label={item.label}
                  title={item.label}
                  onClick={() => navigate({ to: item.link as string })}
                  className={`flex w-11 flex-col items-center justify-center gap-0.5 rounded-md py-1 transition-colors ${
                    isActive(item.link)
                      ? 'bg-foreground text-background'
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
            {navGroups.map((group) => (
              <SidebarGroup key={group.label} className="p-0 pb-3">
                <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
                  {group.label}
                </SidebarGroupLabel>
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
          </div>
        </div>
      </SidebarContent>

      <SidebarFooter className="gap-2 border-t border-border/70 p-2">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild className="h-8 rounded-md px-2">
              <a
                href="/workspaces"
                onClick={(event) => {
                  event.preventDefault();
                  navigate({ to: '/workspaces' });
                }}
              >
                <Users />
                <span>All Workspaces</span>
              </a>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>

        <div className="flex items-center gap-2 rounded-md border border-border/70 bg-background px-2 py-1.5">
          <Avatar className="h-8 w-8 rounded-md">
            <AvatarFallback className="rounded-md text-xs">{initials}</AvatarFallback>
          </Avatar>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{user?.full_name || user?.email}</p>
            <p className="truncate text-xs text-muted-foreground">{user?.email}</p>
          </div>
          <button
            type="button"
            aria-label="Sign out"
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            onClick={signOut}
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </SidebarFooter>
    </ShellSidebar>
  );
}
