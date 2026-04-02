import { useMemo } from 'react';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { RoadmapEpic } from '@/lib/pmTypes';

const HEALTH_BAR_COLOR: Record<string, string> = {
  no_health: 'border-l-zinc-400 bg-zinc-500/6 hover:bg-zinc-500/12 dark:bg-zinc-500/8 dark:hover:bg-zinc-500/14',
  on_track: 'border-l-green-500 bg-green-500/8 hover:bg-green-500/15 dark:bg-green-500/10 dark:hover:bg-green-500/18',
  at_risk: 'border-l-yellow-500 bg-yellow-500/8 hover:bg-yellow-500/15 dark:bg-yellow-500/10 dark:hover:bg-yellow-500/18',
  off_track: 'border-l-red-500 bg-red-500/8 hover:bg-red-500/15 dark:bg-red-500/10 dark:hover:bg-red-500/18',
};

const HEALTH_DOT_COLOR: Record<string, string> = {
  no_health: 'bg-zinc-400',
  on_track: 'bg-green-500',
  at_risk: 'bg-yellow-500',
  off_track: 'bg-red-500',
};

const HEALTH_LABEL: Record<string, string> = {
  no_health: 'No health',
  on_track: 'On track',
  at_risk: 'At risk',
  off_track: 'Off track',
};

interface RoadmapEpicBarProps {
  epic: RoadmapEpic;
  left: number;
  width: number;
  slug: string;
  memberNameMap?: Map<string, string>;
}

export function RoadmapEpicBar({ epic, left, width, slug, memberNameMap }: RoadmapEpicBarProps) {
  const navigate = useNavigate();
  const e = epic.epic;
  const health = e.health || 'no_health';

  const progress = useMemo(() => {
    if (epic.stats.story_count === 0) return 0;
    return Math.round((epic.stats.done_story_count / epic.stats.story_count) * 100);
  }, [epic.stats]);

  const ownerName = e.owner_member_id && memberNameMap?.get(e.owner_member_id);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={() => navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: e.id } })}
          className={`absolute top-1 bottom-1 rounded border-l-[3px] transition-colors cursor-pointer flex items-center gap-1.5 px-2 min-w-0 ${HEALTH_BAR_COLOR[health]}`}
          style={{ left: `${left}%`, width: `${Math.max(width, 2)}%` }}
        >
          {/* Progress fill */}
          {progress > 0 && (
            <div
              className="absolute inset-0 rounded-r opacity-15 bg-current pointer-events-none"
              style={{ width: `${progress}%` }}
            />
          )}
          <span className="text-[11px] font-medium truncate relative z-[1] text-foreground">
            {e.name}
          </span>
          {ownerName && (
            <span className="shrink-0 relative z-[1]">
              <UserAvatar name={ownerName} className="h-4 w-4" fallbackClassName="text-[7px]" />
            </span>
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" align="start" className="max-w-sm p-0 overflow-hidden bg-popover text-popover-foreground border border-border shadow-lg">
        <div className="px-3 pt-3 pb-2">
          <div className="flex items-start gap-2">
            {ownerName && (
              <span className="shrink-0 mt-0.5">
                <UserAvatar name={ownerName} className="h-5 w-5" fallbackClassName="text-[8px]" />
              </span>
            )}
            <div className="min-w-0">
              <p className="font-medium text-sm leading-snug">{e.name}</p>
              {ownerName && (
                <p className="text-[11px] text-muted-foreground mt-0.5">{ownerName}</p>
              )}
            </div>
          </div>
        </div>

        {/* Stats row */}
        <div className="flex items-center gap-3 px-3 py-2 border-t border-border/40 bg-muted/30">
          <span className="flex items-center gap-1.5 text-[11px]">
            <span className={`h-2 w-2 rounded-full ${HEALTH_DOT_COLOR[health]}`} />
            <span className="font-medium">{HEALTH_LABEL[health]}</span>
          </span>
          <span className="text-muted-foreground/30">|</span>
          <span className="text-[11px] text-muted-foreground">
            {epic.stats.done_story_count}/{epic.stats.story_count} stories
          </span>
          <span className="text-muted-foreground/30">|</span>
          <span className="text-[11px] text-muted-foreground font-medium tabular-nums">{progress}%</span>
        </div>

        {/* Progress bar */}
        <div className="h-1 bg-muted">
          <div
            className={`h-full transition-all ${health === 'off_track' ? 'bg-red-500' : health === 'at_risk' ? 'bg-yellow-500' : health === 'on_track' ? 'bg-green-500' : 'bg-zinc-400'}`}
            style={{ width: `${progress}%` }}
          />
        </div>

        {/* Details */}
        {(e.planned_start_date || e.deadline || epic.objectives.length > 0) && (
          <div className="px-3 py-2 space-y-1.5 border-t border-border/40">
            {(e.planned_start_date || e.deadline) && (
              <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                <span className="text-muted-foreground/60">Dates</span>
                <span className="ml-auto tabular-nums">
                  {e.planned_start_date ? format(parseISO(e.planned_start_date), 'MMM d') : '—'}
                  {' \u2192 '}
                  {e.deadline ? format(parseISO(e.deadline), 'MMM d, yyyy') : '—'}
                </span>
              </div>
            )}
            {epic.objectives.length > 0 && (
              <div className="flex items-start gap-1.5 text-[11px] text-muted-foreground">
                <span className="text-muted-foreground/60 shrink-0">Obj</span>
                <span className="ml-auto text-right">
                  {epic.objectives.map((o) => o.name).join(', ')}
                </span>
              </div>
            )}
          </div>
        )}
      </TooltipContent>
    </Tooltip>
  );
}
