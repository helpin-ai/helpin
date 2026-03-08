import { useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Check, ChevronsUpDown, Plus } from 'lucide-react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useWorkspaces } from '@/hooks/queries';
import type { Workspace } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/components/ui/sidebar';
import { UserAvatar } from '@/components/pm/UserAvatar';

function workspaceRouteFromCurrentPath(pathname: string, slug: string): string {
  const match = pathname.match(/^\/w\/[^/]+\/?(.*)$/);
  const rest = match?.[1] ? match[1] : 'dashboard';
  return `/w/${slug}/${rest}`;
}

export function WorkspaceSwitcher() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isMobile } = useSidebar();
  const { currentWorkspace, setCurrentWorkspace } = useWorkspaceStore();
  const currentOrganization = useOrganizationStore((s) => s.currentOrganization);
  const { data: workspaces = [] } = useWorkspaces(currentOrganization?.id);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return workspaces;
    return workspaces.filter((workspace) => workspace.name.toLowerCase().includes(normalized));
  }, [query, workspaces]);

  if (!currentWorkspace) return null;

  const handleWorkspaceSelect = (workspace: Workspace) => {
    setCurrentWorkspace(workspace);
    navigate({ to: workspaceRouteFromCurrentPath(location.pathname, workspace.slug) as string });
    setQuery('');
    setOpen(false);
  };

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="h-10 rounded-md border border-transparent px-2 data-[state=open]:bg-sidebar-accent/80 data-[state=open]:text-sidebar-accent-foreground hover:border-border/70"
            >
              <UserAvatar
                name={currentWorkspace.name}
                avatarUrl={currentWorkspace.logo_url}
                className="h-8 w-8 rounded-md"
                fallbackClassName="text-xs rounded-md"
              />
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">{currentWorkspace.name}</span>
              </div>
              <ChevronsUpDown className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
            </SidebarMenuButton>
          </PopoverTrigger>
          <PopoverContent
            className="w-80 p-0"
            align="start"
            side={isMobile ? 'bottom' : 'right'}
            sideOffset={4}
          >
            <div className="p-2">
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Filter workspaces"
                className="h-8"
              />
            </div>
            <Separator />
            <div className="max-h-[320px] overflow-auto p-1">
              {filtered.length === 0 ? (
                <p className="p-3 text-sm text-muted-foreground">No workspaces found.</p>
              ) : (
                filtered.map((workspace) => {
                  const isActive = workspace.id === currentWorkspace.id;
                  return (
                    <button
                      key={workspace.id}
                      type="button"
                      onClick={() => handleWorkspaceSelect(workspace)}
                      className={cn(
                        'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent',
                        isActive && 'bg-accent'
                      )}
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        <UserAvatar
                          name={workspace.name}
                          avatarUrl={workspace.logo_url}
                          className="h-5 w-5 shrink-0 rounded"
                          fallbackClassName="text-[8px] rounded"
                        />
                        <span className="truncate">{workspace.name}</span>
                      </div>
                      {isActive && <Check className="h-4 w-4 text-green-500" />}
                    </button>
                  );
                })
              )}
            </div>
            <Separator />
            <div className="grid grid-cols-2 gap-1.5 p-1.5">
              <button
                type="button"
                onClick={() => {
                  navigate({ to: '/workspaces' });
                  setOpen(false);
                }}
                className="flex items-center justify-center gap-1 rounded-md border border-border px-2 py-1 text-xs font-medium hover:bg-accent transition-colors"
              >
                View All
              </button>
              <button
                type="button"
                onClick={() => {
                  navigate({ to: '/workspaces', search: { create: true } });
                  setOpen(false);
                }}
                className="flex items-center justify-center gap-1 rounded-md bg-primary px-2 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
              >
                <Plus className="h-3 w-3" />
                Create
              </button>
            </div>
          </PopoverContent>
        </Popover>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
