import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useAuthStore } from '@/stores/authStore';
import { authService } from '@/lib/services/authService';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser, Workspace } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Favicon } from '@/components/ui/favicon';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useSupportUnreadByWorkspace } from '@/hooks/queries/useSupport';
import { workspaceBillingBadge as billingBadge } from '@edition';
import { billingEnabled } from '@edition/config';
import { MoreHorizontalIcon, Settings02Icon, StarIcon, UserGroupIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

const MAX_VISIBLE_AVATARS = 5;

interface WorkspaceSelectorProps {
  workspaces: Workspace[];
}

function canManageBilling(ws: Workspace): boolean {
  return ['owner', 'admin', 'manager'].includes((ws.role ?? '').toLowerCase());
}

export function getWorkspaceBillingBadgeClasses(className: string): string {
  return cn(
    'max-w-[12.5rem] min-w-0 shrink-0 truncate px-2.5 py-1 text-[11px] leading-none',
    className,
  );
}

export function WorkspaceSelector({ workspaces }: WorkspaceSelectorProps) {
  const { user } = useAuthStore();
  const defaultWsId = user?.default_workspace_id;
  const [membersMap, setMembersMap] = useState<Record<string, MemberWithUser[]>>({});
  const { data: supportUnread = [] } = useSupportUnreadByWorkspace();
  const unreadByWorkspace = useMemo(() => {
    const map = new Map<string, number>();
    for (const entry of supportUnread) {
      map.set(entry.workspace_id, entry.unread_count);
    }
    return map;
  }, [supportUnread]);

  useEffect(() => {
    workspaces.forEach((ws) => {
      workspacesService.listMembers(ws.id).then(({ data }) => {
        if (data) {
          setMembersMap((prev) => ({ ...prev, [ws.id]: data }));
        }
      });
    });
  }, [workspaces]);

  const handleSetDefault = async (e: { stopPropagation: () => void }, wsId: string) => {
    e.stopPropagation();
    if (wsId === defaultWsId) {
      return;
    }
    const { data, error } = await authService.updateProfile({
      default_workspace_id: wsId,
    });
    if (error) {
      toast.error(error);
    } else {
      if (data) useAuthStore.setState({ user: data });
      toast.success('Default workspace set');
    }
  };

  return (
    <TooltipProvider>
      <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {workspaces.map((ws) => (
          <WorkspaceCard
            key={ws.id}
            ws={ws}
            isDefault={defaultWsId === ws.id}
            members={membersMap[ws.id] ?? []}
            unread={unreadByWorkspace.get(ws.id) ?? 0}
            onSetDefault={handleSetDefault}
          />
        ))}
      </div>
    </TooltipProvider>
  );
}

function WorkspaceCard({
  ws,
  isDefault,
  members,
  unread,
  onSetDefault,
}: {
  ws: Workspace;
  isDefault: boolean;
  members: MemberWithUser[];
  unread: number;
  onSetDefault: (event: { stopPropagation: () => void }, wsId: string) => void;
}) {
  const navigate = useNavigate();
  const billing = ws.billing;
  const plan = billingBadge(billing);
  const canManage = billingEnabled && canManageBilling(ws);
  const visibleMembers = members.slice(0, MAX_VISIBLE_AVATARS);
  const overflowMembers = members.slice(MAX_VISIBLE_AVATARS);
  const overflowCount = members.length - MAX_VISIBLE_AVATARS;

  const openWorkspace = () =>
    navigate({
      to: billingEnabled && billing?.locked ? `/w/${ws.slug}/settings/billing` : `/w/${ws.slug}/pm/my-work`,
    });
  const openBilling = (event: { stopPropagation: () => void }) => {
    event.stopPropagation();
    navigate({ to: `/w/${ws.slug}/settings/billing` });
  };
  const openSettings = (event: { stopPropagation: () => void }) => {
    event.stopPropagation();
    navigate({ to: `/w/${ws.slug}/settings/general` });
  };
  const openMembersSettings = (event: { stopPropagation: () => void }) => {
    event.stopPropagation();
    navigate({ to: `/w/${ws.slug}/settings/members` });
  };

  return (
    <Card
      className={`group relative cursor-pointer transition-all duration-150 hover:border-primary/50 hover:shadow-sm ${
        isDefault ? 'border-primary/30 bg-primary/[0.02]' : ''
      }`}
      onClick={openWorkspace}
    >
      <CardHeader className="pb-3">
        <div className="flex items-start gap-3">
          <Favicon
            src={ws.logo_url}
            url={ws.website_url}
            name={ws.name}
            size={128}
            className="h-9 w-9 shrink-0 rounded-md"
            fallbackClassName="text-xs"
          />
          <div className="min-w-0 flex-1">
            <CardTitle className="truncate text-base">{ws.name}</CardTitle>
            <div className="mt-0.5 flex min-w-0 items-center gap-1.5">
              <CardDescription className="min-w-0 truncate text-xs">{ws.slug}</CardDescription>
              {isDefault && (
                <Badge variant="secondary" className="shrink-0 gap-1 bg-primary/10 text-primary text-[10px] px-1.5 py-0">
                  <StarIcon className="h-2.5 w-2.5 fill-current" />
                  Default
                </Badge>
              )}
            </div>
          </div>
          <div className="flex shrink-0 items-start gap-1.5">
            {unread > 0 && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className="mt-1 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold leading-none text-white"
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
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  onClick={(event) => event.stopPropagation()}
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label={`More options for ${ws.name}`}
                >
                  <MoreHorizontalIcon className="h-4 w-4" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuItem onClick={openSettings}>
                  <Settings02Icon className="h-4 w-4" />
                  <span>Settings</span>
                </DropdownMenuItem>
                {canManage && (
                  <DropdownMenuItem onClick={openBilling}>
                    <Settings02Icon className="h-4 w-4" />
                    <span>Manage billing</span>
                  </DropdownMenuItem>
                )}
                {!isDefault && (
                  <DropdownMenuItem onClick={(event) => onSetDefault(event, ws.id)}>
                    <StarIcon className="h-4 w-4" />
                    <span>Set as default</span>
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </CardHeader>

      <CardContent className="pt-0">
        <div className="flex min-h-9 items-center justify-between gap-4">
          <div
            className="flex min-w-0 cursor-default items-center -space-x-1.5"
            onClick={(event) => event.stopPropagation()}
          >
            {visibleMembers.map((member) => (
              <Tooltip key={member.id}>
                <TooltipTrigger asChild>
                  <div>
                    <UserAvatar
                      name={member.full_name}
                      avatarUrl={member.avatar_url}
                      avatarStyle={member.avatar_style}
                      avatarSeed={member.avatar_seed}
                      avatarBackgroundMode={member.avatar_background_mode}
                      avatarBackgroundColor={member.avatar_background_color}
                      className="h-7 w-7 ring-2 ring-background"
                    />
                  </div>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="text-xs">
                  {member.full_name}
                </TooltipContent>
              </Tooltip>
            ))}
            {overflowCount > 0 && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <div className="flex h-7 min-w-8 items-center justify-center rounded-full border border-border bg-background px-2 shadow-sm ring-2 ring-background">
                    <span className="text-[10px] font-semibold text-foreground">
                      +{overflowCount}
                    </span>
                  </div>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-56 flex-col items-stretch gap-2 py-2 text-xs">
                  <div className="space-y-1">
                    {overflowMembers.slice(0, 5).map((member) => (
                      <p key={member.id} className="truncate font-medium text-background">{member.full_name}</p>
                    ))}
                    {overflowMembers.length > 5 && (
                      <p className="text-background/80">and {overflowMembers.length - 5} more</p>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={openMembersSettings}
                    className="inline-flex items-center gap-1.5 text-[11px] font-medium text-background underline underline-offset-2 transition-opacity hover:opacity-80"
                  >
                    <UserGroupIcon className="h-3.5 w-3.5" />
                    View all members
                  </button>
                </TooltipContent>
              </Tooltip>
            )}
          </div>
          {plan && <Badge variant="outline" className={getWorkspaceBillingBadgeClasses(plan.className)}>
            {plan.label}
          </Badge>}
        </div>
      </CardContent>

    </Card>
  );
}
