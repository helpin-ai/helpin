import { useMemo } from 'react';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';

import { getEpicBadgeTextColor, resolveEpicColor } from './epicColor';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { RoadmapEpic } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';

const HEALTH_TONE: Record<string, { border: string; progress: string; dot: string; text: string }> = {
  no_health: {
    border: 'border-l-quiet-empty',
    progress: 'bg-quiet-text-tertiary',
    dot: 'bg-quiet-empty',
    text: 'text-quiet-text-tertiary',
  },
  on_track: {
    border: 'border-l-quiet-positive',
    progress: 'bg-quiet-positive',
    dot: 'bg-quiet-positive',
    text: 'text-quiet-positive',
  },
  at_risk: {
    border: 'border-l-quiet-accent',
    progress: 'bg-quiet-accent',
    dot: 'bg-quiet-accent',
    text: 'text-quiet-accent',
  },
  off_track: {
    border: 'border-l-quiet-accent',
    progress: 'bg-quiet-accent',
    dot: 'bg-quiet-accent',
    text: 'text-quiet-accent',
  },
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
  const entity = epic.epic;
  const health = entity.health || 'no_health';
  const tone = HEALTH_TONE[health] ?? HEALTH_TONE.no_health;
  const totalTasks = getEpicTaskCount(epic.stats);
  const completedTasks = getEpicDoneTaskCount(epic.stats);
  const progress = useMemo(
    () => (totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0),
    [completedTasks, totalTasks],
  );
  const ownerName = entity.owner_member_id ? memberNameMap?.get(entity.owner_member_id) : undefined;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={`Open ${entity.name}`}
          onClick={() => navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: entity.id } })}
          className={`absolute bottom-1.5 top-1.5 flex min-w-0 cursor-pointer items-center gap-1.5 overflow-hidden rounded-[3px] border-l-[3px] px-2 text-left transition-shadow hover:ring-1 hover:ring-foreground/20 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-quiet-text-primary ${tone.border}`}
          style={{
            left: `${left}%`,
            width: `${Math.max(width, 1)}%`,
            backgroundColor: resolveEpicColor(entity.color),
            color: getEpicBadgeTextColor(entity.color),
          }}
        >
          <span className="relative z-[1] min-w-0 flex-1 truncate text-[12px] font-medium">
            {entity.name}
          </span>
          {ownerName ? (
            <UserAvatar name={ownerName} className="relative z-[1] h-4 w-4 shrink-0" fallbackClassName="text-[7px]" />
          ) : null}
          <span
            aria-hidden="true"
            className={`absolute bottom-0 left-0 h-0.5 ${tone.progress}`}
            style={{ width: `${progress}%` }}
          />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" align="start" className="flex w-max min-w-[260px] max-w-md flex-col items-stretch gap-0 overflow-hidden rounded-lg border border-border bg-popover p-0 text-popover-foreground shadow-lg **:data-[slot=arrow]:bg-popover **:data-[slot=arrow]:fill-popover">
        <div className="px-3 pb-2 pt-3">
          <div className="flex items-start gap-2">
            {ownerName ? (
              <span className="mt-0.5 shrink-0">
                <UserAvatar name={ownerName} className="h-5 w-5" fallbackClassName="text-[8px]" />
              </span>
            ) : null}
            <div className="min-w-0">
              <p className="text-sm font-medium leading-snug">{entity.name}</p>
              {ownerName ? <p className="mt-0.5 text-[11px] text-muted-foreground">{ownerName}</p> : null}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-4 border-t border-border/40 bg-muted/30 px-3 py-2">
          <span className="flex items-center gap-1.5 text-xs">
            <span className={`h-2 w-2 rounded-full ${health === 'on_track' ? 'bg-green-500' : health === 'at_risk' ? 'bg-yellow-500' : health === 'off_track' ? 'bg-red-500' : 'bg-zinc-400'}`} />
            <span className="font-medium">{HEALTH_LABEL[health]}</span>
          </span>
          <span className="text-xs tabular-nums text-muted-foreground">{completedTasks}/{totalTasks} tasks</span>
          <span className="text-xs font-medium tabular-nums text-muted-foreground">{progress}%</span>
        </div>

        <div className="h-1 bg-muted">
          <div
            className={`h-full transition-all ${health === 'off_track' ? 'bg-red-500' : health === 'at_risk' ? 'bg-yellow-500' : health === 'on_track' ? 'bg-green-500' : 'bg-zinc-400'}`}
            style={{ width: `${progress}%` }}
          />
        </div>

        <div className="space-y-1.5 border-t border-border/40 px-3 py-2">
          <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span className="text-muted-foreground/60">Dates</span>
            <span className="whitespace-nowrap tabular-nums">
              {format(parseISO(entity.planned_start_date!), 'MMM d')}
              {' \u2192 '}
              {format(parseISO(entity.deadline!), 'MMM d, yyyy')}
            </span>
          </div>
          {epic.objectives.length > 0 ? (
            <div className="flex items-start justify-between gap-3 text-xs text-muted-foreground">
              <span className="shrink-0 text-muted-foreground/60">Objective</span>
              <span className="text-right">{epic.objectives.map((objective) => objective.name).join(', ')}</span>
            </div>
          ) : null}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
