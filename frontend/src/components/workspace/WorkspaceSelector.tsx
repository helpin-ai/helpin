import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useAuthStore } from '@/stores/authStore';
import { authService } from '@/lib/services/authService';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser, Workspace, WorkspaceBillingSummary } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Favicon } from '@/components/ui/favicon';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useSupportUnreadByWorkspace } from '@/hooks/queries/useSupport';
import { daysUntil, PLAN_LABEL } from '@/lib/billingUtils';
import { Settings02Icon, StarIcon, UserGroupIcon } from '@/lib/icons';
import { toast } from 'sonner';

const MAX_VISIBLE_AVATARS = 5;

interface WorkspaceSelectorProps {
  workspaces: Workspace[];
}

function billingBadge(billing?: WorkspaceBillingSummary | null): {
  label: string;
  className: string;
} {
  if (!billing) {
    return {
      label: 'Trial pending',
      className: 'bg-muted text-muted-foreground border-transparent',
    };
  }

  if (billing.status === 'past_due') {
    return {
      label: 'Past due',
      className: 'bg-destructive/10 text-destructive border-destructive/20',
    };
  }

  if (billing.locked || billing.status === 'trial_expired' || billing.status === 'canceled') {
    return {
      label: billing.status === 'trial_expired' ? 'Trial ended' : 'Locked',
      className: 'bg-destructive/10 text-destructive border-destructive/20',
    };
  }

  const plan = PLAN_LABEL[billing.plan] ?? billing.plan;
  if (billing.trialing) {
    const days = daysUntil(billing.trial_ends_at);
    return {
      label: days > 0 ? `${plan} trial · ${days}d left` : `${plan} trial`,
      className: 'bg-amber-500/10 text-amber-700 border-amber-500/20 dark:text-amber-300',
    };
  }

  return {
    label: plan,
    className: billing.locked
      ? 'bg-destructive/10 text-destructive border-destructive/20'
      : 'bg-primary/10 text-primary border-primary/15',
  };
}

function canManageBilling(ws: Workspace): boolean {
  return ['owner', 'admin', 'manager'].includes((ws.role ?? '').toLowerCase());
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

  const handleSetDefault = async (e: React.MouseEvent, wsId: string) => {
    e.stopPropagation();
    const newId = wsId === defaultWsId ? '' : wsId;
    const { data, error } = await authService.updateProfile({
      default_workspace_id: newId || undefined,
    });
    if (error) {
      toast.error(error);
    } else {
      if (data) useAuthStore.setState({ user: data });
      toast.success(newId ? 'Default workspace set' : 'Default workspace cleared');
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
  onSetDefault: (event: React.MouseEvent, wsId: string) => void;
}) {
  const navigate = useNavigate();
  const billing = ws.billing;
  const plan = billingBadge(billing);
  const canManage = canManageBilling(ws);
  const visibleMembers = members.slice(0, MAX_VISIBLE_AVATARS);
  const overflowCount = members.length - MAX_VISIBLE_AVATARS;

  const openWorkspace = () =>
    navigate({
      to: billing?.locked ? `/w/${ws.slug}/settings/billing` : `/w/${ws.slug}/pm/my-work`,
    });
  const openBilling = (event: React.MouseEvent) => {
    event.stopPropagation();
    navigate({ to: `/w/${ws.slug}/settings/billing` });
  };

  return (
    <Card
      className={`group relative cursor-pointer transition-all duration-150 hover:border-primary/50 hover:shadow-sm ${
        isDefault ? 'border-primary/30 bg-primary/[0.02]' : ''
      }`}
      onClick={openWorkspace}
    >
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <Favicon
            src={ws.logo_url}
            url={ws.website_url}
            name={ws.name}
            size={128}
            className="h-11 w-11 shrink-0 rounded-lg"
            fallbackClassName="text-sm"
          />
          <div className="min-w-0 flex-1">
            <CardTitle className="text-base truncate">{ws.name}</CardTitle>
            <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5">
              <CardDescription className="text-xs truncate">{ws.slug}</CardDescription>
              <Badge variant="outline" className={`shrink-0 px-1.5 py-0 text-[10px] ${plan.className}`}>
                {plan.label}
              </Badge>
              {isDefault && (
                <Badge variant="secondary" className="shrink-0 gap-1 bg-primary/10 text-primary text-[10px] px-1.5 py-0">
                  <StarIcon className="h-2.5 w-2.5 fill-current" />
                  Default
                </Badge>
              )}
            </div>
          </div>
          {unread > 0 && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className="shrink-0 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold leading-none text-white"
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
        </div>
      </CardHeader>

      <CardContent className="pt-0">
        <div className="flex min-h-8 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center -space-x-1.5">
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
                  <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted ring-2 ring-background">
                    <span className="text-[10px] font-medium text-muted-foreground">
                      +{overflowCount}
                    </span>
                  </div>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="text-xs">
                  {overflowCount} more member{overflowCount !== 1 ? 's' : ''}
                </TooltipContent>
              </Tooltip>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {canManage && (
              <button
                type="button"
                onClick={openBilling}
                className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-background px-2.5 text-xs font-medium text-foreground shadow-sm transition-opacity hover:bg-muted sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
                aria-label={`Manage billing for ${ws.name}`}
              >
                <Settings02Icon className="h-3.5 w-3.5" />
                Manage billing
              </button>
            )}
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              <UserGroupIcon className="h-3.5 w-3.5" />
              <span>{members.length}</span>
            </div>
          </div>
        </div>
      </CardContent>

      <button
        type="button"
        onClick={(e) => onSetDefault(e, ws.id)}
        className={`absolute top-2.5 right-2.5 flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium transition-opacity duration-150 ${
          isDefault
            ? 'opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-foreground hover:bg-muted'
            : 'opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-primary hover:bg-primary/10'
        }`}
      >
        <StarIcon className="h-3 w-3" />
        {isDefault ? 'Remove default' : 'Set as default'}
      </button>
    </Card>
  );
}
