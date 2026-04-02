import { useDraggable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import { Plus, UserPlus } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Button } from '@/components/ui/button';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { PRIORITY_BORDER_COLOR, PriorityIcon, StateTypeIcon, STORY_TYPE_CONFIG, StoryTypeIcon } from '@/lib/pmConstants';
import type { AssignableMember } from '@/lib/types';
import type { SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';

interface SprintPlanningTaskCardProps {
  story: SprintPlanningTaskPreview;
  owner?: AssignableMember;
  compact?: boolean;
  showBacklogAction?: boolean;
  canDrag?: boolean;
  isDragging?: boolean;
  onOpen?: () => void;
  onAddToSprint?: () => void;
}

export function SprintPlanningTaskCard({
  story,
  owner,
  compact = false,
  showBacklogAction = false,
  canDrag = false,
  isDragging = false,
  onOpen,
  onAddToSprint,
}: SprintPlanningTaskCardProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging: dragActive } = useDraggable({
    id: story.id,
    data: { story },
    disabled: !canDrag,
  });

  const storyTypeCfg = STORY_TYPE_CONFIG[(story as { task_type?: string }).task_type as keyof typeof STORY_TYPE_CONFIG] ?? null;

  return (
    <article
      ref={setNodeRef}
      {...(canDrag ? attributes : {})}
      {...(canDrag ? listeners : {})}
      role="button"
      tabIndex={0}
      onClick={() => onOpen?.()}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onOpen?.();
        }
      }}
      style={{ transform: CSS.Translate.toString(transform) }}
      className={cn(
        'group/card relative shrink-0 rounded-lg border border-border/60 bg-background shadow-sm transition-all overflow-hidden',
        'hover:border-border hover:shadow-md',
        canDrag ? 'cursor-grab active:cursor-grabbing' : 'cursor-pointer',
        (isDragging || dragActive) && 'opacity-40',
      )}
    >
      <div className={cn('min-w-0 flex-1', compact ? 'p-2.5' : 'p-3')}>
        {/* Row 1: type icon + state + spacer + drag handle + priority */}
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {storyTypeCfg && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="shrink-0">
                  <StoryTypeIcon storyType={(story as { task_type?: string }).task_type as import('@/lib/pmTypes').TaskType} className="h-3.5 w-3.5" />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{storyTypeCfg.label}</TooltipContent>
            </Tooltip>
          )}
          {story.state_type && story.state_name && (
            <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
              <StateTypeIcon stateType={story.state_type} className="h-3 w-3" />
              {story.state_name}
            </span>
          )}
          <span className="flex-1" />
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn('flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1', PRIORITY_BORDER_COLOR[story.priority])}>
                <PriorityIcon priority={story.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {story.priority}</TooltipContent>
          </Tooltip>
        </div>

        {/* Row 2: Title */}
        <h4 className={cn('mt-2 line-clamp-2 font-medium leading-snug text-foreground', compact ? 'mb-2 text-[13px]' : 'mb-3 text-[13px]')}>
          {story.name}
        </h4>

        {/* Row 3: Footer — estimate + assignee */}
        <div className="flex items-center gap-1.5">
          {typeof story.estimate === 'number' && (
            <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
              {story.estimate} pts
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
                    className="h-5 w-5"
                  />
                ) : (
                  <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground">
                    <UserPlus className="h-2.5 w-2.5" />
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
            <Plus className="h-3 w-3" />
            Add to sprint
          </Button>
        </div>
      ) : null}
    </article>
  );
}
