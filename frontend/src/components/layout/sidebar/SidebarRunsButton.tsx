import { useMemo } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { formatDistanceToNow } from 'date-fns';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Loading01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import type { AgentRun, AgentRunStatus } from '@/lib/pm-types/agents';

const ACTIVE_STATUSES = new Set<AgentRunStatus>(['running', 'queued', 'paused']);

function StatusDot({ status }: { status: AgentRunStatus }) {
  if (ACTIVE_STATUSES.has(status)) {
    return <Loading01Icon className="h-3 w-3 animate-spin text-primary" />;
  }
  return (
    <span
      aria-hidden
      className={cn(
        'h-2 w-2 rounded-full',
        status === 'completed'
          ? 'bg-emerald-500'
          : status === 'failed'
            ? 'bg-destructive'
            : 'bg-muted-foreground/40',
      )}
    />
  );
}

function targetLabel(run: AgentRun): string {
  const t = run.target_info;
  if (t?.title) return t.title;
  if (t?.task_key) return t.task_key;
  if (run.target_type) return `${run.target_type.replace('_', ' ')} ${run.target_id.slice(0, 8)}`;
  return run.target_id.slice(0, 8);
}

function statusLabel(status: AgentRunStatus): string {
  return status.charAt(0).toUpperCase() + status.slice(1);
}

export function SidebarRunsButton() {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const setRailMode = useCommandBarRunStore((s) => s.setRailMode);
  const setRailFilter = useCommandBarRunStore((s) => s.setRailFilter);
  const setSelectedRunId = useCommandBarRunStore((s) => s.setSelectedRunId);

  const { active, failed, recent } = useMemo(() => {
    const all = (Object.values(runsById) as AgentRun[])
      .slice()
      .sort(
        (a, b) =>
          new Date(b.updated_at || b.created_at).getTime() -
          new Date(a.updated_at || a.created_at).getTime(),
      );
    let a = 0;
    let f = 0;
    for (const r of all) {
      if (ACTIVE_STATUSES.has(r.status)) a++;
      else if (r.status === 'failed') f++;
    }
    return { active: a, failed: f, recent: all.slice(0, 10) };
  }, [runsById]);

  const titleParts: string[] = [];
  if (active) titleParts.push(`${active} running`);
  if (failed) titleParts.push(`${failed} failed`);
  const title =
    titleParts.length > 0 ? `Command runs · ${titleParts.join(' · ')}` : 'Command runs';

  const onOpenInRail = () => {
    setRailFilter('all');
    setRailMode('open');
  };

  const onViewAll = () => {
    if (!workspace?.slug) return;
    navigate({
      to: '/w/$slug/automation/activity',
      params: { slug: workspace.slug },
      search: { page: 1 },
    });
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={title}
          title={title}
          className="relative flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
            className="h-3.5 w-3.5"
          >
            <path d="M3 12h3.28a1 1 0 0 1 .948.684l2.298 7.934a.5.5 0 0 0 .96-.044L13.82 4.771A1 1 0 0 1 14.792 4H21" />
          </svg>
          {active > 0 ? (
            <span className="absolute -right-1 -top-1 flex h-3.5 min-w-[14px] items-center justify-center rounded-full bg-primary px-0.5 text-[9px] font-bold leading-none text-primary-foreground">
              {active > 9 ? '9+' : active}
            </span>
          ) : failed > 0 ? (
            <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-destructive" />
          ) : null}
        </button>
      </PopoverTrigger>
      <PopoverContent side="right" align="end" sideOffset={8} className="w-[360px] p-0">
        <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3 py-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          <span>Recent runs</span>
          <button
            type="button"
            onClick={onOpenInRail}
            className="rounded px-1.5 py-0.5 text-[11px] font-medium normal-case tracking-normal text-muted-foreground transition hover:bg-muted hover:text-foreground"
          >
            Open rail →
          </button>
        </div>
        <div className="max-h-[420px] overflow-y-auto">
          {recent.length === 0 ? (
            <div className="flex flex-col items-center gap-1 px-4 py-8 text-center">
              <p className="text-sm font-medium text-foreground">No active runs</p>
              <p className="text-xs text-muted-foreground">
                Ask the dock to start one.
              </p>
            </div>
          ) : (
            recent.map((run) => (
              <button
                key={run.id}
                type="button"
                onClick={() => setSelectedRunId(run.id)}
                className="flex w-full items-start gap-2 border-b border-border/40 px-3 py-2 text-left transition last:border-b-0 hover:bg-muted/40"
              >
                <div className="mt-0.5 flex h-3 w-3 shrink-0 items-center justify-center">
                  <StatusDot status={run.status} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium text-foreground">
                      {targetLabel(run)}
                    </span>
                    <span
                      className={cn(
                        'shrink-0 text-[10px]',
                        run.status === 'failed'
                          ? 'text-destructive'
                          : 'text-muted-foreground',
                      )}
                    >
                      {statusLabel(run.status)}
                    </span>
                  </div>
                  <div className="mt-0.5 flex items-center gap-1 text-[11px] text-muted-foreground">
                    <span className="truncate">
                      {run.target_type.replace('_', ' ')}
                    </span>
                    <span className="opacity-60">·</span>
                    <span className="shrink-0">
                      {formatDistanceToNow(
                        new Date(run.updated_at || run.created_at),
                        { addSuffix: true },
                      )}
                    </span>
                  </div>
                </div>
              </button>
            ))
          )}
        </div>
        <div className="border-t border-border/60 px-3 py-2">
          <button
            type="button"
            onClick={onViewAll}
            className="w-full text-left text-[11px] font-medium text-muted-foreground transition hover:text-foreground"
          >
            View all activity in Automation →
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
