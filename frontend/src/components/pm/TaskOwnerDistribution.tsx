import { useId, useState } from 'react';
import { UserGroupIcon, UserIcon } from '@/lib/icons';

import { QuietTextAction } from '@/components/design-system/quiet';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import {
  buildTaskOwnerDistribution,
  type TaskOwnerDistributionEntry,
  type TaskOwnerInput,
} from '@/components/pm/task-owner-distribution';
import type { AssignableMember } from '@/lib/types';

interface TaskOwnerDistributionProps {
  tasks: TaskOwnerInput[];
  members: AssignableMember[];
}

const VISIBLE_OWNER_LIMIT = 8;

const SEGMENT_COLORS = [
  'bg-[#1b7f4e]',
  'bg-[#4a9bd8]',
  'bg-[#c98a3e]',
  'bg-[#b8614a]',
  'bg-[#6f8f55]',
  'bg-[#7b6f9b]',
  'bg-[#3f8c85]',
];

function stableHash(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = ((hash << 5) - hash + value.charCodeAt(index)) | 0;
  }
  return Math.abs(hash);
}

function getSegmentColor(entry: TaskOwnerDistributionEntry): string {
  if (entry.kind === 'unassigned') return 'bg-muted-foreground/35';
  if (entry.kind === 'unknown') return 'bg-zinc-500/70';
  return SEGMENT_COLORS[stableHash(entry.id) % SEGMENT_COLORS.length];
}

function formatTaskCount(taskCount: number): string {
  return `${taskCount} ${taskCount === 1 ? 'task' : 'tasks'}`;
}

export function TaskOwnerDistribution({ tasks, members }: TaskOwnerDistributionProps) {
  const [expanded, setExpanded] = useState(false);
  const labelsId = useId();
  const distribution = buildTaskOwnerDistribution(tasks, members);
  const hiddenCount = Math.max(0, distribution.entries.length - VISIBLE_OWNER_LIMIT);
  const unassigned = distribution.entries.find((entry) => entry.kind === 'unassigned');
  // Keep unassigned work visible even when the smaller owner groups are collapsed.
  const collapsedEntries = hiddenCount > 0 && unassigned
    ? [...distribution.entries.slice(0, VISIBLE_OWNER_LIMIT - 1), unassigned]
    : distribution.entries.slice(0, VISIBLE_OWNER_LIMIT);
  const visibleEntries = expanded ? distribution.entries : collapsedEntries;

  return (
    <section className="min-w-0" aria-label="Task owners">
      <TaskDetailSectionHeading
        title={`Task owners (${distribution.ownerCount})`}
        icon={UserGroupIcon}
      />

      {distribution.totalTasks === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">Add tasks to see ownership.</p>
      ) : (
        <div className="mt-3">
          <div
            className="grid h-2.5 overflow-hidden rounded-full bg-muted"
            style={{ gridTemplateColumns: distribution.entries.map((entry) => `${entry.taskCount}fr`).join(' ') }}
            role="img"
            aria-label={distribution.entries
              .map((entry) => `${entry.name} ${entry.percentage}%`)
              .join(', ')}
            data-testid="task-owner-bar"
          >
            {distribution.entries.map((entry) => (
              <QuickTooltip key={entry.id} label={`${entry.name}: ${formatTaskCount(entry.taskCount)} · ${entry.percentage}%`}>
                <span
                  className={`${getSegmentColor(entry)} min-w-0 border-r border-background/80 last:border-r-0`}
                  aria-hidden="true"
                  data-testid={`task-owner-segment-${entry.id}`}
                />
              </QuickTooltip>
            ))}
          </div>

          <div
            id={labelsId}
            className="mt-3 grid gap-x-4 gap-y-3"
            style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 11rem), 1fr))' }}
            data-testid="task-owner-labels"
          >
            {visibleEntries.map((entry) => {
              const tooltip = `${entry.name}: ${formatTaskCount(entry.taskCount)} · ${entry.percentage}%`;
              return (
                <QuickTooltip key={entry.id} label={tooltip}>
                  <div className="flex min-w-0 items-center gap-2">
                    {entry.member ? (
                      <UserAvatar
                        name={entry.member.display_name || entry.member.email}
                        avatarUrl={entry.member.avatar_url}
                        avatarStyle={entry.member.avatar_style}
                        avatarSeed={entry.member.avatar_seed}
                        avatarBackgroundMode={entry.member.avatar_background_mode}
                        avatarBackgroundColor={entry.member.avatar_background_color}
                        className="h-5 w-5"
                        fallbackClassName="text-[8px]"
                      />
                    ) : (
                      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                        <UserIcon className="h-3.5 w-3.5" />
                      </span>
                    )}
                    <span className="min-w-0">
                      <span className="block truncate text-ui text-foreground/90">{entry.name}</span>
                      <span className="flex items-center gap-1.5 text-[11px] leading-4 tabular-nums text-muted-foreground">
                        <span className={`${getSegmentColor(entry)} h-1.5 w-1.5 shrink-0 rounded-full`} aria-hidden="true" />
                        {formatTaskCount(entry.taskCount)} · {entry.percentage}%
                      </span>
                    </span>
                  </div>
                </QuickTooltip>
              );
            })}
          </div>

          {hiddenCount > 0 && (
            <QuietTextAction
              type="button"
              className="mt-3"
              aria-expanded={expanded}
              aria-controls={labelsId}
              onClick={() => setExpanded((value) => !value)}
            >
              {expanded ? 'Show less' : `Show ${hiddenCount} more`}
            </QuietTextAction>
          )}
        </div>
      )}
    </section>
  );
}
