import { useDroppable } from '@dnd-kit/core';
import { CalendarDays, Plus } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { SPRINT_STATUS_CONFIG } from '@/lib/pmConstants';
import type { AssignableMember } from '@/lib/types';
import type { SprintPlanningCard } from '@/lib/pmTypes';
import { SprintPlanningTaskCard } from './SprintPlanningTaskCard';
import { cn } from '@/lib/utils';

interface SprintPlanningColumnProps {
  card: SprintPlanningCard;
  ownerByMemberId: Map<string, AssignableMember>;
  canEdit: boolean;
  onOpenSprint: (sprintId: string) => void;
  onOpenStory: (storyId: string) => void;
  onCreateStory: (sprintId: string) => void;
}

function formatSprintRange(startDate: string | null, endDate: string | null) {
  if (!startDate || !endDate) return 'No dates set';
  return `${format(parseISO(startDate), 'MMM d')} – ${format(parseISO(endDate), 'MMM d')}`;
}

export function SprintPlanningColumn({
  card,
  ownerByMemberId,
  canEdit,
  onOpenSprint,
  onOpenStory,
  onCreateStory,
}: SprintPlanningColumnProps) {
  const isCompleted = card.sprint.status === 'done';
  const { setNodeRef, isOver } = useDroppable({
    id: `sprint:${card.sprint.id}`,
    disabled: isCompleted,
  });
  const statusConfig = SPRINT_STATUS_CONFIG[card.sprint.status];
  const total = card.stats.story_count;
  const done = card.stats.done_story_count;
  const pctDone = total > 0 ? Math.round((done / total) * 100) : 0;
  const stories = card.preview_stories ?? [];

  return (
      <Card
        ref={setNodeRef}
        className={cn(
          'relative flex h-[calc(100vh-13rem)] w-[320px] shrink-0 flex-col border-border/60 bg-card/80 backdrop-blur-sm transition-all',
          isOver && 'border-primary/50 ring-2 ring-primary/20',
        )}
      >
        {isOver && (
          <div className="pointer-events-none absolute inset-0 z-10 rounded-xl bg-muted/70 ring-1 ring-primary/20">
            <div className="flex h-full items-center justify-center">
              <div className="rounded-md border border-primary/25 bg-background/90 px-3 py-1.5 text-xs font-medium text-foreground shadow-sm">
                Drop into sprint
              </div>
            </div>
          </div>
        )}
        <CardHeader className="space-y-3 whitespace-normal pb-3">
          <div className="flex items-start justify-between gap-3">
            <button type="button" className="min-w-0 cursor-pointer text-left" onClick={() => onOpenSprint(card.sprint.id)}>
              <h3 className="truncate text-base font-semibold hover:underline">{card.sprint.name}</h3>
              <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                <CalendarDays className="h-3.5 w-3.5" />
                {formatSprintRange(card.sprint.start_date, card.sprint.end_date)}
              </p>
            </button>
            <Badge className={cn('shrink-0 border-0', statusConfig.badge)}>{statusConfig.label}</Badge>
          </div>

          <div className="space-y-1.5">
            <div className="flex h-1.5 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-700">
              {pctDone > 0 && <div className="bg-emerald-500 transition-all" style={{ width: `${pctDone}%` }} />}
            </div>
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{done}/{total} stories done</span>
              <span>{card.stats.done_points}/{card.stats.total_points} pts</span>
            </div>
          </div>
        </CardHeader>

        <CardContent className="flex min-h-0 flex-1 flex-col gap-2 whitespace-normal">
          {stories.length > 0 ? (
            <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
              {stories.map((story) => (
                <SprintPlanningTaskCard
                  key={story.id}
                  story={story}
                  owner={story.owner_member_id ? ownerByMemberId.get(story.owner_member_id) : undefined}
                  onOpen={() => onOpenStory(story.id)}
                />
              ))}
            </div>
          ) : (
            <div className={cn(
              'flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-center text-sm transition-colors',
              isOver
                ? 'border-primary/40 bg-primary/5 text-primary/60'
                : 'border-border/60 bg-muted/10 text-muted-foreground',
            )}>
              {isOver ? 'Drop into sprint' : 'No stories yet'}
            </div>
          )}

          {card.story_preview_overflow > 0 && (
            <p className="text-center text-xs text-muted-foreground">+{card.story_preview_overflow} more stories</p>
          )}

          {canEdit && (
            <Button
              variant="ghost"
              size="sm"
              className="mt-auto w-full gap-2 text-muted-foreground"
              onClick={() => onCreateStory(card.sprint.id)}
            >
              <Plus className="h-4 w-4" />
              Create story
            </Button>
          )}
        </CardContent>
      </Card>
  );
}
