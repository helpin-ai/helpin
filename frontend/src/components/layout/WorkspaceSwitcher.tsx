import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Tick01Icon, ArrowUpDownIcon, PlusSignIcon } from '@/lib/icons';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useWorkspaces, useOrganizations } from '@/hooks/queries';
import { useSupportUnreadByWorkspace } from '@/hooks/queries/useSupport';
import type { Workspace } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Favicon } from '@/components/ui/favicon';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/components/ui/sidebar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  flattenGroupedWorkspaces,
  isEditableShortcutTarget,
  isMacPlatform,
  workspaceShortcutLabel,
} from '@/components/layout/workspaceSwitcherShortcuts';

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
  const { setCurrentOrganization } = useOrganizationStore();
  const { data: allWorkspaces = [] } = useWorkspaces(); // Fetch all workspaces across orgs
  const { data: organizations = [] } = useOrganizations();
  const { data: supportUnread = [] } = useSupportUnreadByWorkspace();
  const unreadByWorkspace = useMemo(() => {
    const map = new Map<string, number>();
    for (const entry of supportUnread) {
      map.set(entry.workspace_id, entry.unread_count);
    }
    return map;
  }, [supportUnread]);
  const otherWorkspaceUnread = useMemo(() => {
    if (!currentWorkspace) return 0;
    let total = 0;
    for (const [wsId, count] of unreadByWorkspace.entries()) {
      if (wsId !== currentWorkspace.id) total += count;
    }
    return total;
  }, [unreadByWorkspace, currentWorkspace]);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return allWorkspaces;
    return allWorkspaces.filter((workspace) => workspace.name.toLowerCase().includes(normalized));
  }, [query, allWorkspaces]);

  const multiOrg = organizations.length > 1;

  const groupedWorkspaces = useMemo(() => {
    if (!multiOrg) return [{ org: null, workspaces: filtered }];
    const groups: { org: { id: string; name: string } | null; workspaces: Workspace[] }[] = [];
    for (const org of organizations) {
      const orgWs = filtered.filter((ws) => ws.organization_id === org.id);
      if (orgWs.length > 0) groups.push({ org, workspaces: orgWs });
    }
    const ungrouped = filtered.filter((ws) => !organizations.some((o) => o.id === ws.organization_id));
    if (ungrouped.length > 0) groups.push({ org: null, workspaces: ungrouped });
    return groups;
  }, [filtered, organizations, multiOrg]);

  const shortcutWorkspaces = useMemo(() => flattenGroupedWorkspaces(groupedWorkspaces).slice(0, 9), [groupedWorkspaces]);
  const workspaceShortcutIndexes = useMemo(() => {
    const indexes = new Map<string, number>();
    shortcutWorkspaces.forEach((workspace, index) => indexes.set(workspace.id, index));
    return indexes;
  }, [shortcutWorkspaces]);
  const isMac = useMemo(() => isMacPlatform(), []);

  const handleWorkspaceSelect = useCallback(
    (workspace: Workspace) => {
      setCurrentWorkspace(workspace);
      // Update org if switching to a workspace from a different organization
      if (workspace.organization_id && workspace.organization_id !== currentOrganization?.id) {
        const newOrg = organizations.find((o) => o.id === workspace.organization_id);
        if (newOrg) setCurrentOrganization(newOrg);
      }
      navigate({ to: workspaceRouteFromCurrentPath(location.pathname, workspace.slug) as string });
      setQuery('');
      setOpen(false);
    },
    [currentOrganization?.id, location.pathname, navigate, organizations, setCurrentOrganization, setCurrentWorkspace]
  );

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (isEditableShortcutTarget(event.target) || event.altKey || event.shiftKey || !(event.metaKey || event.ctrlKey)) {
        return;
      }

      if (!/^[1-9]$/.test(event.key)) return;

      const workspace = shortcutWorkspaces[Number(event.key) - 1];
      if (!workspace) return;

      event.preventDefault();
      handleWorkspaceSelect(workspace);
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleWorkspaceSelect, shortcutWorkspaces]);

  if (!currentWorkspace) return null;

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <Popover
          open={open}
          onOpenChange={(nextOpen) => {
            setOpen(nextOpen);
            if (!nextOpen) setQuery('');
          }}
        >
          <PopoverTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="h-10 rounded-md border border-transparent px-2 data-[state=open]:bg-sidebar-accent/80 data-[state=open]:text-sidebar-accent-foreground hover:border-border/70"
            >
              <Favicon
                src={currentWorkspace.logo_url}
                url={currentWorkspace.website_url}
                name={currentWorkspace.name}
                size={128}
                className="h-8 w-8 rounded-md"
                fallbackClassName="text-xs"
              />
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">{currentWorkspace.name}</span>
              </div>
              {otherWorkspaceUnread > 0 && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span
                      className="ml-auto inline-flex h-[15px] min-w-[15px] items-center justify-center rounded-full bg-red-500 px-[3px] text-[9px] font-semibold leading-none text-white"
                      aria-label={`${otherWorkspaceUnread} unread conversations in other workspaces`}
                    >
                      {otherWorkspaceUnread > 99 ? '99+' : otherWorkspaceUnread}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="right" className="text-xs">
                    {otherWorkspaceUnread} unread conversation{otherWorkspaceUnread === 1 ? '' : 's'} in other workspaces
                  </TooltipContent>
                </Tooltip>
              )}
              <ArrowUpDownIcon className={`${otherWorkspaceUnread > 0 ? 'ml-1' : 'ml-auto'} h-3.5 w-3.5 text-muted-foreground`} />
            </SidebarMenuButton>
          </PopoverTrigger>
          <PopoverContent
            className="w-80 p-0"
            align="start"
            side={isMobile ? 'bottom' : 'right'}
            sideOffset={4}
          >
            <div className="px-2 pt-2 pb-1.5">
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Filter workspaces"
                className="h-8"
              />
            </div>
            <div className="max-h-[320px] overflow-auto px-1 pb-1">
              {filtered.length === 0 ? (
                <p className="p-3 text-sm text-muted-foreground">No workspaces found.</p>
              ) : (
                groupedWorkspaces.map((group) => (
                  <div key={group.org?.id ?? '__ungrouped'}>
                    {group.org && (
                      <p className="px-2 pt-3 pb-1 text-[11px] font-medium text-muted-foreground/60">
                        {group.org.name}
                      </p>
                    )}
                    {group.workspaces.map((workspace) => {
                      const isActive = workspace.id === currentWorkspace.id;
                      const unread = unreadByWorkspace.get(workspace.id) ?? 0;
                      const shortcutIndex = workspaceShortcutIndexes.get(workspace.id);
                      const shortcut = shortcutIndex === undefined ? null : workspaceShortcutLabel(shortcutIndex, isMac);
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
                            <Favicon
                              src={workspace.logo_url}
                              url={workspace.website_url}
                              name={workspace.name}
                              size={32}
                              className="h-5 w-5 shrink-0 rounded"
                              fallbackClassName="text-[8px]"
                            />
                            <span className="truncate">{workspace.name}</span>
                          </div>
                          <div className="flex shrink-0 items-center gap-1.5">
                            {shortcut && (
                              <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[10px] leading-none text-muted-foreground/70">
                                {shortcut}
                              </kbd>
                            )}
                            {unread > 0 && (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <span
                                    className="inline-flex h-[15px] min-w-[15px] items-center justify-center rounded-full bg-red-500 px-[3px] text-[9px] font-semibold leading-none text-white"
                                    aria-label={`${unread} unread conversations`}
                                  >
                                    {unread > 99 ? '99+' : unread}
                                  </span>
                                </TooltipTrigger>
                                <TooltipContent side="top" className="text-xs">
                                  {unread} unread conversation{unread === 1 ? '' : 's'}
                                </TooltipContent>
                              </Tooltip>
                            )}
                            {isActive && <Tick01Icon className="h-4 w-4 text-green-500" />}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                ))
              )}
            </div>
            <div className="grid grid-cols-2 gap-1.5 border-t border-border px-1.5 py-1">
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
                <PlusSignIcon className="h-3 w-3" />
                Create
              </button>
            </div>
          </PopoverContent>
        </Popover>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
