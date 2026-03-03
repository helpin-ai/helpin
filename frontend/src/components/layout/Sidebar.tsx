import { useLocation, useNavigate } from '@tanstack/react-router';
import { Calendar, DollarSign, LayoutDashboard, LogOut, Settings, Target, User, Users } from 'lucide-react';
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
  SidebarRail,
  SidebarSeparator,
} from '@/components/ui/sidebar';
import { WorkspaceSwitcher } from '@/components/layout/WorkspaceSwitcher';

type NavItem = {
  link: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
};

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, signOut } = useAuthStore();
  const { currentWorkspace } = useWorkspaceStore();

  const wsSlug = currentWorkspace?.slug ?? '';

  const navGroups: { label: string; items: NavItem[] }[] = [
    {
      label: 'Main',
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
    {
      label: 'Workspace',
      items: [{ link: `/w/${wsSlug}/settings`, label: 'Settings', icon: Settings }],
    },
  ];

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
    <ShellSidebar collapsible="icon">
      <SidebarHeader>
        <WorkspaceSwitcher />
      </SidebarHeader>

      <SidebarContent>
        {navGroups.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarMenu>
              {group.items.map((item) => (
                <SidebarMenuItem key={item.link}>
                  <SidebarMenuButton
                    asChild
                    tooltip={item.label}
                    isActive={isActive(item.link)}
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
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              tooltip="All Workspaces"
            >
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

        <SidebarSeparator />

        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild size="lg">
              <div>
                <Avatar className="h-8 w-8 rounded-lg">
                  <AvatarFallback className="rounded-lg text-xs">{initials}</AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{user?.full_name || user?.email}</span>
                  <span className="truncate text-xs text-muted-foreground">{user?.email}</span>
                </div>
                <button
                  type="button"
                  aria-label="Sign out"
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  onClick={(event) => {
                    event.stopPropagation();
                    signOut();
                  }}
                >
                  <LogOut className="h-4 w-4" />
                </button>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      <SidebarRail />
    </ShellSidebar>
  );
}
