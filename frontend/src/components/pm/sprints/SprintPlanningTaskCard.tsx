import { memo } from 'react';
import { useDraggable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import { PlusSignIcon, UserAdd01Icon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Button } from '@/components/ui/button';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { PRIORITY_BORDER_COLOR, PriorityIcon, StateTypeIcon, TASK_TYPE_CONFIG, TaskTypeIcon } from '@/lib/pmConstants';
import type { AssignableMember } from '@/lib/types';
import type { SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';

interface SprintPlanningTaskCardProps {
  task: SprintPlanningTaskPreview;
  owner?: AssignableMember;
  compact?: boolean;
  showBacklogAction?: boolean;
  canDrag?: boolean;
  isDragging?: boolean;
  onOpenTask?: (taskId: string) => void;
  onAddToSprint?: () => void;
}

export const SprintPlanningTaskCard = memo(function SprintPlanningTaskCard({
  task,
  owner,
  compact = false,
  showBacklogAction = false,
  canDrag = false,
  isDragging = false,
  onOpenTask,
  onAddToSprint,
}: SprintPlanningTaskCardProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging: dragActive } = useDraggable({
    id: task.id,
    data: { task },
    disabled: !canDrag,
  });
  const renderedDragging = isDragging || dragActive;
  const appliedTransform = dragActive ? null : transform;

  const taskTypeCfg = TASK_TYPE_CONFIG[(task as { task_type?: string }).task_type as keyof typeof TASK_TYPE_CONFIG] ?? null;

  return (
    <article
      ref={setNodeRef}
      {...(canDrag ? attributes : {})}
      {...(canDrag ? listeners : {})}
      role="button"
      tabIndex={0}
      onClick={() => onOpenTask?.(task.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onOpenTask?.(task.id);
        }
      }}
      style={{ transform: CSS.Translate.toString(appliedTransform) }}
      className={cn(
        'group/card relative shrink-0 rounded-lg border border-border/60 bg-card shadow-sm transition-all overflow-hidden',
        'hover:border-border hover:shadow-md',
        canDrag ? 'cursor-grab active:cursor-grabbing' : 'cursor-pointer',
        renderedDragging && 'opacity-40',
      )}
    >
      <div className={cn('min-w-0 flex-1', compact ? 'p-2.5' : 'p-3')}>
        {/* Row 1: type icon + state + spacer + drag handle + priority */}
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {taskTypeCfg && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="shrink-0">
                  <TaskTypeIcon taskType={(task as { task_type?: string }).task_type as import('@/lib/pmTypes').TaskType} className="h-3.5 w-3.5" />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{taskTypeCfg.label}</TooltipContent>
            </Tooltip>
          )}
          {task.state_type && task.state_name && (
            <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
              <StateTypeIcon stateType={task.state_type} className="h-3 w-3" />
              {task.state_name}
            </span>
          )}
          <span className="flex-1" />
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn('flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1', PRIORITY_BORDER_COLOR[task.priority])}>
                <PriorityIcon priority={task.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {task.priority}</TooltipContent>
          </Tooltip>
        </div>

        {/* Row 2: Title */}
        <h4 className={cn('mt-2 line-clamp-2 font-medium leading-snug text-foreground', compact ? 'mb-2 text-sm' : 'mb-3 text-sm')}>
          {task.name}
        </h4>

        {/* Row 3: Footer — estimate + assignee */}
        <div className="flex items-center gap-1.5">
          {typeof task.estimate === 'number' && (
            <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
              {task.estimate} pts
            </span>
          )}
          <span className="flex-1" />
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="shrink-0">
                {owner ? (
                  <UserAvatar
                    name={owner.display_name || owner.email}
                    avatarUrl={owner.avatar_url}
                    avatarStyle={owner.avatar_style}
                    avatarSeed={owner.avatar_seed}
                    avatarBackgroundMode={owner.avatar_background_mode}
                    avatarBackgroundColor={owner.avatar_background_color}
                    className="h-5 w-5"
                  />
                ) : (
                  <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground">
                    <UserAdd01Icon className="h-2.5 w-2.5" />
                  </span>
                )}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{owner?.display_name || 'Unassigned'}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* Hover action strip */}
      {showBacklogAction && onAddToSprint ? (
        <div className="border-t border-border/40 px-2 py-1.5 opacity-0 transition-opacity group-hover/card:opacity-100">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 w-full justify-center gap-1 px-2 text-[11px]"
            onClick={(e) => { e.stopPropagation(); onAddToSprint(); }}
          >
            <PlusSignIcon className="h-3 w-3" />
            Add to sprint
          </Button>
        </div>
      ) : null}
    </article>
  );
});
