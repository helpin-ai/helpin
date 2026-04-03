import { useDroppable } from '@dnd-kit/core';
import { memo, useMemo, useState } from 'react';
import { ChevronDown, ChevronLeft, ChevronRight, Funnel, Inbox, PlusCircle } from 'lucide-react';
import { Collapsible } from 'radix-ui';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { AssignableMember } from '@/lib/types';
import type { Priority, SprintPlanningTaskPreview, StateType } from '@/lib/pmTypes';
import { SprintPlanningTaskCard } from './SprintPlanningTaskCard';
import { cn } from '@/lib/utils';

interface SprintPlanningBacklogPanelProps {
  open: boolean;
  onToggle: () => void;
  tasks: SprintPlanningTaskPreview[];
  total: number;
  ownerByMemberId: Map<string, AssignableMember>;
  canEdit: boolean;
  onOpenTask: (taskId: string) => void;
  onAddToActiveSprint: (task: SprintPlanningTaskPreview) => void;
  onCreateTask: () => void;
}

const PRIORITY_OPTIONS: Array<{ value: Priority | '__all__'; label: string }> = [
  { value: '__all__', label: 'All priorities' },
  { value: 'urgent', label: 'Urgent' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
  { value: 'none', label: 'None' },
];

const STATE_OPTIONS: Array<{ value: StateType | '__all__'; label: string }> = [
  { value: '__all__', label: 'All states' },
  { value: 'backlog', label: 'Backlog' },
  { value: 'unstarted', label: 'Unstarted' },
  { value: 'started', label: 'Started' },
];

export const SprintPlanningBacklogPanel = memo(function SprintPlanningBacklogPanel({
  open,
  onToggle,
  tasks,
  total,
  ownerByMemberId,
  canEdit,
  onOpenTask,
  onAddToActiveSprint,
  onCreateTask,
}: SprintPlanningBacklogPanelProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: 'backlog-dropzone',
  });
  const [priority, setPriority] = useState<Priority | '__all__'>('__all__');
  const [stateType, setStateType] = useState<StateType | '__all__'>('__all__');
  const [ownerMemberId, setOwnerMemberId] = useState<string>('__all__');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const hasActiveFilters = priority !== '__all__' || stateType !== '__all__' || ownerMemberId !== '__all__';

  const ownerOptions = useMemo(() => {
    const members = [...ownerByMemberId.values()];
    return members
      .filter((member, index, arr) => arr.findIndex((candidate) => candidate.id === member.id) === index)
      .sort((a, b) => a.display_name.localeCompare(b.display_name));
  }, [ownerByMemberId]);

  const filteredTasks = useMemo(() => {
    return (tasks ?? []).filter((task) => {
      if (priority !== '__all__' && task.priority !== priority) return false;
      if (stateType !== '__all__' && task.state_type !== stateType) return false;
      if (ownerMemberId !== '__all__' && task.owner_member_id !== ownerMemberId) return false;
      return true;
    });
  }, [ownerMemberId, priority, stateType,  tasks]);

  if (!open) {
    return (
      <button
        ref={setNodeRef}
        type="button"
        onClick={onToggle}
        className={cn(
          'hidden h-[calc(100vh-13rem)] w-10 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-border/60 bg-card transition-colors hover:bg-muted/50 xl:flex',
          isOver && 'border-primary/50 ring-2 ring-primary/20',
        )}
      >
        <div className="flex flex-col items-center gap-2">
          <ChevronLeft className="h-4 w-4 text-muted-foreground" />
          <span className="text-xs font-medium text-muted-foreground [writing-mode:vertical-lr]">
            Backlog ({total})
          </span>
        </div>
      </button>
    );
  }

  return (
      <Card
        ref={setNodeRef}
        className={cn(
          'hidden h-[calc(100vh-13rem)] w-[340px] shrink-0 border-border/60 xl:flex xl:flex-col',
          isOver && 'border-primary/50 ring-2 ring-primary/20',
        )}
      >
        <CardHeader className="space-y-3 pb-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">Backlog</CardTitle>
              <Badge variant="secondary" className="h-5 px-1.5 text-[11px] font-medium">
                {filteredTasks.length}/{total}
              </Badge>
            </div>
            <button
              type="button"
              onClick={onToggle}
              className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted/50"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
          <p className="text-xs text-muted-foreground">
            Unsprinted work you can pull into the current plan.
          </p>

          <Collapsible.Root open={filtersOpen} onOpenChange={setFiltersOpen}>
            <Collapsible.Trigger asChild>
              <button
                type="button"
                className={cn(
                  'flex w-full items-center justify-between gap-2 rounded-md border border-border/60 px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/40',
                  hasActiveFilters && 'border-primary/30 text-foreground',
                )}
              >
                <span className="flex items-center gap-2">
                  <Funnel className="h-3.5 w-3.5" />
                  Filters
                  {hasActiveFilters && (
                    <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  )}
                </span>
                <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', filtersOpen && 'rotate-180')} />
              </button>
            </Collapsible.Trigger>
            <Collapsible.Content className="space-y-2 pt-2">
              <Select value={priority} onValueChange={(value) => setPriority(value as Priority | '__all__')}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRIORITY_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value} className="text-xs">
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={stateType} onValueChange={(value) => setStateType(value as StateType | '__all__')}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {STATE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value} className="text-xs">
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={ownerMemberId} onValueChange={setOwnerMemberId}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="All assignees" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all__" className="text-xs">All assignees</SelectItem>
                  {ownerOptions.map((member) => (
                    <SelectItem key={member.id} value={member.id} className="text-xs">
                      {member.display_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Collapsible.Content>
          </Collapsible.Root>
        </CardHeader>

        <CardContent className="flex min-h-0 flex-1 flex-col gap-3">
          <div className="min-h-0 flex-1 overflow-y-auto pr-1">
            <div className="space-y-2">
              {filteredTasks.length > 0 ? (
                filteredTasks.map((task) => (
                  <SprintPlanningTaskCard
                    key={task.id}
                    task={task}
                    owner={task.owner_member_id ? ownerByMemberId.get(task.owner_member_id) : undefined}
                    compact
                    showBacklogAction={canEdit}
                    canDrag={canEdit}
                    onOpen={() => onOpenTask(task.id)}
                    onAddToSprint={canEdit ? () => onAddToActiveSprint(task) : undefined}
                  />
                ))
              ) : (
                <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border/60 bg-muted/10 p-6 text-center">
                  <Inbox className="h-5 w-5 text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">
                    {total === 0
                      ? 'All tasks are assigned to sprints.'
                      : 'No backlog tasks match the current filters.'}
                  </p>
                </div>
              )}
            </div>
          </div>

          {canEdit ? (
            <Button variant="outline" className="gap-2" onClick={onCreateTask}>
              <PlusCircle className="h-4 w-4" />
              Create task
            </Button>
          ) : null}
        </CardContent>
      </Card>
  );
});
