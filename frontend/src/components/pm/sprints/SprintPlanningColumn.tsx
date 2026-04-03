import { memo, useEffect, useMemo, useRef } from 'react';
import type { InfiniteData } from '@tanstack/react-query';
import { useQueryClient } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useDroppable } from '@dnd-kit/core';
import { CalendarDays, Loader2, Plus } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { useInfiniteSprintPreviewTasks } from '@/hooks/queries/useSprints';
import { SPRINT_STATUS_CONFIG } from '@/lib/pmConstants';
import { queryKeys } from '@/lib/queryKeys';
import type { AssignableMember } from '@/lib/types';
import type { PaginatedResponse, SprintPlanningCard, SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { SprintPlanningTaskCard } from './SprintPlanningTaskCard';
import { cn } from '@/lib/utils';

const SPRINT_PREVIEW_PAGE_SIZE = 20;
const SPRINT_TASK_ESTIMATE_HEIGHT = 116;

interface SprintPlanningColumnProps {
  card: SprintPlanningCard;
  workspaceId: string;
  ownerByMemberId: Map<string, AssignableMember>;
  canEdit: boolean;
  isDropTargetActive?: boolean;
  onOpenSprint: (sprintId: string) => void;
  onOpenTask: (taskId: string) => void;
  onCreateTask: (sprintId: string) => void;
}

function formatSprintRange(startDate: string | null, endDate: string | null) {
  if (!startDate || !endDate) return 'No dates set';
  return `${format(parseISO(startDate), 'MMM d')} – ${format(parseISO(endDate), 'MMM d')}`;
}

export const SprintPlanningColumn = memo(function SprintPlanningColumn({
  card,
  workspaceId,
  ownerByMemberId,
  canEdit,
  isDropTargetActive = false,
  onOpenSprint,
  onOpenTask,
  onCreateTask,
}: SprintPlanningColumnProps) {
  const queryClient = useQueryClient();
  const { setNodeRef, isOver } = useDroppable({
    id: `sprint:${card.sprint.id}`,
  });
  const showDropIndicator = isOver || isDropTargetActive;
  const statusConfig = SPRINT_STATUS_CONFIG[card.sprint.status];
  const total = card.stats.task_count;
  const done = card.stats.done_task_count;
  const pctDone = total > 0 ? Math.round((done / total) * 100) : 0;
  const previewTasks = card.preview_tasks ?? [];
  const previewQuery = useInfiniteSprintPreviewTasks(workspaceId, card.sprint.id, previewTasks, total, SPRINT_PREVIEW_PAGE_SIZE);
  const listRef = useRef<HTMLDivElement | null>(null);
  const previewSignature = useMemo(
    () => `${total}:${previewTasks.map((task) => task.id).join(',')}`,
    [previewTasks, total],
  );
  const seededFirstPage = useMemo<PaginatedResponse<SprintPlanningTaskPreview[]>>(
    () => ({
      data: previewTasks,
      total,
      page: 1,
      per_page: SPRINT_PREVIEW_PAGE_SIZE,
      total_pages: total > 0 ? Math.ceil(total / SPRINT_PREVIEW_PAGE_SIZE) : 0,
    }),
    [previewTasks, total],
  );

  useEffect(() => {
    if (total > 0 && previewTasks.length === 0) {
      return;
    }
    queryClient.setQueryData<InfiniteData<PaginatedResponse<SprintPlanningTaskPreview[]>>>(
      queryKeys.pm.sprintPreviewTasks(workspaceId, card.sprint.id),
      (existing) => {
        if (!existing || existing.pages.length === 0) {
          return {
            pageParams: [1],
            pages: [seededFirstPage],
          };
        }
        const existingFirstPage = existing.pages[0];
        const existingIds = existingFirstPage.data.map((task) => task.id);
        const seededIds = seededFirstPage.data.map((task) => task.id);
        const firstPageUnchanged =
          existingFirstPage.total === seededFirstPage.total &&
          existingFirstPage.per_page === seededFirstPage.per_page &&
          existingIds.length === seededIds.length &&
          existingIds.every((id, index) => id === seededIds[index]);
        if (firstPageUnchanged) {
          return existing;
        }
        return {
          pageParams: existing.pageParams.length > 0 ? existing.pageParams : [1],
          pages: [seededFirstPage, ...existing.pages.slice(1)],
        };
      },
    );
  }, [card.sprint.id, previewSignature, previewTasks.length, queryClient, seededFirstPage, total, workspaceId]);

  const tasks = useMemo(() => {
    const flattened = previewQuery.data?.pages.flatMap((page) => page.data) ?? [];
    if (flattened.length > 0 || total === 0) {
      return flattened;
    }
    return previewTasks;
  }, [previewQuery.data?.pages, previewTasks, total]);
  const hasMore = Boolean(previewQuery.hasNextPage);
  const isLoadingMore = previewQuery.isFetchingNextPage;
  const virtualizer = useVirtualizer({
    count: tasks.length + (hasMore ? 1 : 0),
    getScrollElement: () => listRef.current,
    estimateSize: (index) => index === tasks.length ? 40 : SPRINT_TASK_ESTIMATE_HEIGHT,
    overscan: 6,
  });

  useEffect(() => {
    const el = listRef.current;
    if (!el || !hasMore || isLoadingMore) return;
    if (el.scrollHeight <= el.clientHeight + 48) {
      void previewQuery.fetchNextPage();
    }
  }, [hasMore, isLoadingMore, previewQuery, tasks.length]);

  const handleScroll = () => {
    const el = listRef.current;
    if (!el || !hasMore || isLoadingMore) return;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - 240) {
      void previewQuery.fetchNextPage();
    }
  };

  return (
      <Card
        ref={setNodeRef}
        className={cn(
          'relative flex h-[calc(100vh-13rem)] w-[320px] shrink-0 flex-col border-border/60 bg-card/80 backdrop-blur-sm transition-all',
          showDropIndicator && 'border-primary/50 ring-2 ring-primary/20',
        )}
      >
        {showDropIndicator && (
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
              <span>{done}/{total} tasks done</span>
              <span>{card.stats.done_points}/{card.stats.total_points} pts</span>
            </div>
          </div>
        </CardHeader>

        <CardContent className="flex min-h-0 flex-1 flex-col gap-2 whitespace-normal">
          {tasks.length > 0 ? (
            <div ref={listRef} className="min-h-0 flex-1 overflow-y-auto pr-1" onScroll={handleScroll}>
              <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const task = tasks[virtualRow.index];
                  const isLoaderRow = virtualRow.index >= tasks.length;
                  return (
                    <div
                      key={isLoaderRow ? 'loader' : task.id}
                      data-index={virtualRow.index}
                      ref={!isLoaderRow ? virtualizer.measureElement : undefined}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                    >
                      {isLoaderRow ? (
                        <div className="flex items-center justify-center gap-2 py-2 text-xs text-muted-foreground">
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          Loading more tasks…
                        </div>
                      ) : (
                        <SprintPlanningTaskCard
                          task={task}
                          owner={task.owner_member_id ? ownerByMemberId.get(task.owner_member_id) : undefined}
                          canDrag={canEdit}
                          onOpenTask={onOpenTask}
                        />
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          ) : (
            <div className={cn(
              'flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-center text-sm transition-colors',
              showDropIndicator
                ? 'border-primary/40 bg-primary/5 text-primary/60'
                : 'border-border/60 bg-muted/10 text-muted-foreground',
            )}>
              {showDropIndicator ? 'Drop into sprint' : 'No tasks yet'}
            </div>
          )}

          {total > tasks.length ? (
            <div className="text-center text-[11px] text-muted-foreground">
              Showing {tasks.length} of {total} tasks
            </div>
          ) : null}

          {canEdit && (
            <Button
              variant="ghost"
              size="sm"
              className="mt-auto w-full gap-2 text-muted-foreground"
              onClick={() => onCreateTask(card.sprint.id)}
            >
              <Plus className="h-4 w-4" />
              Create task
            </Button>
          )}
        </CardContent>
      </Card>
  );
});
