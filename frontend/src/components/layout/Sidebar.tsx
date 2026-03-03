import { Link, useLocation } from '@tanstack/react-router';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  LayoutDashboard, Target, Users, Calendar, DollarSign,
  Settings, User, LogOut, ChevronLeft
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';

const navItems = [
  { to: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: 'goals', label: 'Company Goals', icon: Target },
  { to: 'team-goals', label: 'Team Goals', icon: Users },
  { to: 'sprints', label: 'Sprints', icon: Calendar },
  { to: 'bonus', label: 'Bonus Dashboard', icon: DollarSign },
  { to: 'my-quarter', label: 'My Quarter', icon: User },
  { to: 'settings', label: 'Settings', icon: Settings },
];

interface SidebarProps {
  onClose?: () => void;
}

export function Sidebar({ onClose }: SidebarProps) {
  const { user, signOut } = useAuthStore();
  const { currentWorkspace } = useWorkspaceStore();
  const location = useLocation();

  const initials = user?.full_name
    ? user.full_name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
    : user?.email?.slice(0, 2).toUpperCase() || '??';

  return (
    <div className="flex flex-col h-full bg-sidebar text-sidebar-foreground">
      <div className="p-4 flex items-center gap-2">
        <Link to="/workspaces" className="flex items-center gap-2 hover:opacity-80">
          <ChevronLeft className="h-4 w-4" />
          <span className="text-xs text-muted-foreground">Workspaces</span>
        </Link>
      </div>
      <div className="px-4 pb-3">
        <h2 className="font-semibold text-lg truncate">{currentWorkspace?.name || 'Workspace'}</h2>
      </div>
      <Separator />
      <ScrollArea className="flex-1 px-2 py-2">
        <nav className="space-y-1">
          {navItems.map(item => {
            const fullPath = `/w/${currentWorkspace?.slug}/${item.to}`;
            const isActive = location.pathname.startsWith(fullPath);
            return (
              <Link
                key={item.to}
                to={fullPath}
                onClick={onClose}
                className={cn(
                  'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                    : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground'
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
      </ScrollArea>
      <Separator />
      <div className="p-4 flex items-center gap-3">
        <Avatar className="h-8 w-8">
          <AvatarFallback className="text-xs">{initials}</AvatarFallback>
        </Avatar>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate">{user?.full_name || user?.email}</p>
          <p className="text-xs text-muted-foreground truncate">{user?.email}</p>
        </div>
        <Button variant="ghost" size="icon" onClick={signOut} title="Sign out">
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
