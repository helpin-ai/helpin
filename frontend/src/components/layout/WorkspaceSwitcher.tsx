import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Building2, Check, ChevronsUpDown, ExternalLink } from 'lucide-react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { Workspace } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/components/ui/sidebar';

function workspaceRouteFromCurrentPath(pathname: string, slug: string): string {
  const match = pathname.match(/^\/w\/[^/]+\/?(.*)$/);
  const rest = match?.[1] ? match[1] : 'dashboard';
  return `/w/${slug}/${rest}`;
}

export function WorkspaceSwitcher() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isMobile } = useSidebar();
  const { workspaces, currentWorkspace, setCurrentWorkspace, loadWorkspaces } = useWorkspaceStore();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');

  useEffect(() => {
    if (workspaces.length === 0) {
      void loadWorkspaces();
    }
  }, [workspaces.length, loadWorkspaces]);

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
              <div className="flex h-8 w-8 items-center justify-center rounded-md bg-black text-white">
                <Building2 className="h-4 w-4" />
              </div>
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
            <div className="flex items-center gap-2 p-2">
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Filter workspaces"
                className="h-8"
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  navigate({ to: '/workspaces' });
                  setOpen(false);
                }}
              >
                <ExternalLink className="h-4 w-4" />
              </Button>
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
                        <Building2 className="h-4 w-4 shrink-0 text-muted-foreground" />
                        <span className="truncate">{workspace.name}</span>
                      </div>
                      {isActive && <Check className="h-4 w-4 text-green-500" />}
                    </button>
                  );
                })
              )}
            </div>
          </PopoverContent>
        </Popover>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
