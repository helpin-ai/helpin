import { UserGroupIcon, UserIcon } from '@/lib/icons';

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
  const distribution = buildTaskOwnerDistribution(tasks, members);

  return (
    <section aria-label="Task owners">
      <TaskDetailSectionHeading
        title={`Task owners (${distribution.ownerCount})`}
        icon={UserGroupIcon}
      />

      {distribution.totalTasks === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">Add tasks to see ownership.</p>
      ) : (
        <div className="mt-3 overflow-x-auto pb-1">
          <div
            className="w-full"
            style={{ minWidth: `${Math.max(320, distribution.entries.length * 136)}px` }}
          >
            <div
              className="grid items-start"
              style={{ gridTemplateColumns: distribution.entries.map((entry) => `${entry.taskCount}fr`).join(' ') }}
              data-testid="task-owner-labels"
            >
              {distribution.entries.map((entry) => {
                const tooltip = `${entry.name}: ${formatTaskCount(entry.taskCount)} · ${entry.percentage}%`;
                return (
                  <QuickTooltip key={entry.id} label={tooltip}>
                    <div className="flex min-w-0 items-start gap-1.5 overflow-hidden pr-2 last:pr-0">
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
                        <span className="block truncate text-[11px] leading-4 tabular-nums text-muted-foreground">
                          {formatTaskCount(entry.taskCount)} · {entry.percentage}%
                        </span>
                      </span>
                    </div>
                  </QuickTooltip>
                );
              })}
            </div>

            <div
              className="mt-2 grid h-2.5 overflow-hidden rounded-full bg-muted"
              style={{ gridTemplateColumns: distribution.entries.map((entry) => `${entry.taskCount}fr`).join(' ') }}
              role="img"
              aria-label={distribution.entries
                .map((entry) => `${entry.name} ${entry.percentage}%`)
                .join(', ')}
              data-testid="task-owner-bar"
            >
              {distribution.entries.map((entry) => (
                <span
                  key={entry.id}
                  className={`${getSegmentColor(entry)} border-r border-background/80 last:border-r-0`}
                  aria-hidden="true"
                  data-testid={`task-owner-segment-${entry.id}`}
                />
              ))}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
