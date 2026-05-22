import { useCallback, useMemo, useState } from 'react';
import { format } from 'date-fns';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { PRIORITY_CONFIG, SEVERITY_CONFIG } from '@/lib/pmConstants';
import { ArchiveIcon, Calendar03Icon, Delete01Icon, Loading01Icon, UserAdd01Icon } from '@/lib/icons';
import type {
  EpicWithStats,
  Label,
  Priority,
  Severity,
  SprintWithStats,
  Task,
  TaskDetail,
  UpdateTaskRequest,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';

const PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];
const NONE_VALUE = '__none__';

export type TaskBulkActionKind = 'update' | 'archive' | 'unarchive' | 'delete';

export interface TaskBulkActionResult {
  action: TaskBulkActionKind;
  successfulTaskIds: string[];
  updatedTasks: Task[];
}

interface TaskBulkActionsBarProps {
  selectedTasks: Task[];
  workspaceId: string;
  teamId?: string | null;
  workflow: WorkflowWithStates;
  assignableMembers: AssignableMember[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  labels: Label[];
  onLabelsChange?: (labels: Label[]) => void;
  onComplete: (result: TaskBulkActionResult) => void | Promise<void>;
  onClearSelection: () => void;
}

type TaskMutationResult = Awaited<ReturnType<typeof pmTaskService.update>>;
type TaskDeleteResult = Awaited<ReturnType<typeof pmTaskService.remove>>;

function detailToTask(detail?: TaskDetail | null): Task | null {
  if (!detail?.task) return null;
  return {
    ...detail.task,
    labels: detail.labels ?? detail.task.labels,
    epic_name: detail.epic_name ?? detail.task.epic_name,
    sprint_name: detail.sprint_name ?? detail.task.sprint_name,
  };
}

function getSuccessfulTaskIds<T extends TaskMutationResult | TaskDeleteResult>(
  results: PromiseSettledResult<T>[],
  selectedTasks: Task[],
) {
  return results.flatMap((result, index) => {
    if (result.status !== 'fulfilled' || result.value.error) return [];
    return [selectedTasks[index].id];
  });
}

function getUpdatedTasks(results: PromiseSettledResult<TaskMutationResult>[]) {
  return results.flatMap((result) => {
    if (result.status !== 'fulfilled' || result.value.error) return [];
    const updated = detailToTask(result.value.data);
    return updated ? [updated] : [];
  });
}

function reportBulkResult(verb: string, successCount: number, total: number) {
  const failedCount = total - successCount;
  if (successCount === total) {
    toast.success(`${verb} ${successCount} ${successCount === 1 ? 'task' : 'tasks'}`);
    return;
  }
  if (successCount > 0) {
    toast.warning(`${verb} ${successCount} of ${total} tasks`, {
      description: `${failedCount} ${failedCount === 1 ? 'task' : 'tasks'} failed.`,
    });
    return;
  }
  toast.error(`Failed to ${verb.toLowerCase()} ${total === 1 ? 'task' : 'tasks'}`);
}

export function TaskBulkActionsBar({
  selectedTasks,
  workspaceId,
  teamId,
  workflow,
  assignableMembers,
  epics,
  sprints,
  labels,
  onLabelsChange,
  onComplete,
  onClearSelection,
}: TaskBulkActionsBarProps) {
  const [loading, setLoading] = useState(false);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deadlineOpen, setDeadlineOpen] = useState(false);
  const [addLabelIds, setAddLabelIds] = useState<string[]>([]);
  const [removeLabelIds, setRemoveLabelIds] = useState<string[]>([]);
  const count = selectedTasks.length;
  const nextArchived = !selectedTasks.every((task) => task.archived);
  const visibleSprints = useMemo(
    () => sprints.filter((sprint) => !teamId || !sprint.sprint.team_id || sprint.sprint.team_id === teamId),
    [sprints, teamId],
  );

  const finish = useCallback(
    async (result: TaskBulkActionResult) => {
      await onComplete(result);
      onClearSelection();
    },
    [onClearSelection, onComplete],
  );

  const bulkUpdate = useCallback(
    async (patch: UpdateTaskRequest, verb = 'Updated', action: TaskBulkActionKind = 'update') => {
      if (loading || selectedTasks.length === 0) return;
      setLoading(true);
      try {
        const results = await Promise.allSettled(
          selectedTasks.map((task) => pmTaskService.update(workspaceId, task.id, patch)),
        );
        const successfulTaskIds = getSuccessfulTaskIds(results, selectedTasks);
        reportBulkResult(verb, successfulTaskIds.length, selectedTasks.length);
        await finish({
          action,
          successfulTaskIds,
          updatedTasks: getUpdatedTasks(results),
        });
      } finally {
        setLoading(false);
      }
    },
    [finish, loading, selectedTasks, workspaceId],
  );

  const bulkLabelUpdate = useCallback(
    async (labelIds: string[], mode: 'add' | 'remove') => {
      if (loading || selectedTasks.length === 0 || labelIds.length === 0) return;
      setLoading(true);
      try {
        const idsToChange = new Set(labelIds);
        const results = await Promise.allSettled(
          selectedTasks.map((task) => {
            const currentIds = (task.labels ?? []).map((label) => label.id);
            const nextIds = mode === 'add'
              ? Array.from(new Set([...currentIds, ...labelIds]))
              : currentIds.filter((id) => !idsToChange.has(id));
            return pmTaskService.update(workspaceId, task.id, { label_ids: nextIds });
          }),
        );
        const successfulTaskIds = getSuccessfulTaskIds(results, selectedTasks);
        reportBulkResult(mode === 'add' ? 'Added labels to' : 'Removed labels from', successfulTaskIds.length, selectedTasks.length);
        await finish({
          action: 'update',
          successfulTaskIds,
          updatedTasks: getUpdatedTasks(results),
        });
      } finally {
        setLoading(false);
      }
    },
    [finish, loading, selectedTasks, workspaceId],
  );

  const bulkDelete = useCallback(async () => {
    if (loading || selectedTasks.length === 0) return;
    setLoading(true);
    try {
      const results = await Promise.allSettled(
        selectedTasks.map((task) => pmTaskService.remove(workspaceId, task.id)),
      );
      const successfulTaskIds = getSuccessfulTaskIds(results, selectedTasks);
      reportBulkResult('Deleted', successfulTaskIds.length, selectedTasks.length);
      await finish({
        action: 'delete',
        successfulTaskIds,
        updatedTasks: [],
      });
    } finally {
      setLoading(false);
    }
  }, [finish, loading, selectedTasks, workspaceId]);

  const handleLabelSelectionChange = (nextIds: string[], mode: 'add' | 'remove') => {
    if (mode === 'add') setAddLabelIds(nextIds);
    else setRemoveLabelIds(nextIds);
  };

  if (count === 0) return null;

  return (
    <div className="animate-in slide-in-from-bottom-2 absolute bottom-4 left-1/2 z-20 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-center gap-2 overflow-x-auto rounded-lg border bg-card px-3 py-2 shadow-lg">
      <Badge variant="secondary" className="shrink-0 text-xs">
        {count} selected
      </Badge>

      {loading ? <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" /> : null}

      <Select size="sm" disabled={loading} onValueChange={(value) => void bulkUpdate({ workflow_state_id: value })}>
        <SelectTrigger className="h-7 w-[116px] text-xs">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          {workflow.states.map((state) => (
            <SelectItem key={state.id} value={state.id}>
              {state.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <MemberPickerPopover
        value={NONE_VALUE}
        members={assignableMembers}
        noneLabel="Unassigned"
        disabled={loading}
        onChange={(value) => {
          void bulkUpdate({ owner_member_ids: value === NONE_VALUE ? [] : [value] });
        }}
        triggerClassName="flex h-7 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs"
        contentClassName="w-[220px]"
        renderTrigger={() => (
          <span className="flex items-center gap-1 text-muted-foreground">
            <UserAdd01Icon className="h-3 w-3" /> Owner
          </span>
        )}
      />

      <Select size="sm" disabled={loading} onValueChange={(value) => void bulkUpdate({ priority: value as Priority })}>
        <SelectTrigger className="h-7 w-[104px] text-xs">
          <SelectValue placeholder="Priority" />
        </SelectTrigger>
        <SelectContent>
          {PRIORITIES.map((priority) => (
            <SelectItem key={priority} value={priority}>
              {PRIORITY_CONFIG[priority].label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select size="sm" disabled={loading} onValueChange={(value) => void bulkUpdate({ severity: value as Severity })}>
        <SelectTrigger className="h-7 w-[104px] text-xs">
          <SelectValue placeholder="Severity" />
        </SelectTrigger>
        <SelectContent>
          {SEVERITIES.map((severity) => (
            <SelectItem key={severity} value={severity}>
              {SEVERITY_CONFIG[severity].label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        size="sm"
        disabled={loading}
        onValueChange={(value) => void bulkUpdate({ epic_id: value === NONE_VALUE ? '' : value })}
      >
        <SelectTrigger className="h-7 w-[104px] text-xs">
          <SelectValue placeholder="Epic" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NONE_VALUE}>No Epic</SelectItem>
          {epics.map((epic) => (
            <SelectItem key={epic.epic.id} value={epic.epic.id}>
              {epic.epic.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        size="sm"
        disabled={loading}
        onValueChange={(value) => void bulkUpdate({ sprint_id: value === NONE_VALUE ? '' : value })}
      >
        <SelectTrigger className="h-7 w-[104px] text-xs">
          <SelectValue placeholder="Sprint" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NONE_VALUE}>No Sprint</SelectItem>
          {visibleSprints.map((sprint) => (
            <SelectItem key={sprint.sprint.id} value={sprint.sprint.id}>
              {sprint.sprint.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="flex shrink-0 items-center gap-1" onClick={(event) => event.stopPropagation()}>
        <LabelPicker
          workspaceId={workspaceId}
          teamId={teamId ?? undefined}
          labels={labels}
          selectedLabelIds={addLabelIds}
          onLabelsChange={onLabelsChange}
          onChange={(nextIds) => handleLabelSelectionChange(nextIds, 'add')}
          triggerOnly
          triggerLabel="Add labels"
          className={loading ? 'pointer-events-none opacity-50' : undefined}
        />
        {addLabelIds.length > 0 ? (
          <Button
            variant="secondary"
            size="sm"
            className="h-7 px-2 text-xs"
            disabled={loading}
            onClick={() => void bulkLabelUpdate(addLabelIds, 'add')}
          >
            Apply
          </Button>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-1" onClick={(event) => event.stopPropagation()}>
        <LabelPicker
          workspaceId={workspaceId}
          teamId={teamId ?? undefined}
          labels={labels}
          selectedLabelIds={removeLabelIds}
          onLabelsChange={onLabelsChange}
          onChange={(nextIds) => handleLabelSelectionChange(nextIds, 'remove')}
          triggerOnly
          triggerLabel="Remove labels"
          className={loading ? 'pointer-events-none opacity-50' : undefined}
        />
        {removeLabelIds.length > 0 ? (
          <Button
            variant="secondary"
            size="sm"
            className="h-7 px-2 text-xs"
            disabled={loading}
            onClick={() => void bulkLabelUpdate(removeLabelIds, 'remove')}
          >
            Apply
          </Button>
        ) : null}
      </div>

      <Popover open={deadlineOpen} onOpenChange={setDeadlineOpen}>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" className="h-7 gap-1 px-2 text-xs" disabled={loading}>
            <Calendar03Icon className="h-3 w-3" />
            Deadline
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="center" onClick={(event) => event.stopPropagation()}>
          <Calendar
            mode="single"
            onSelect={(date) => {
              if (!date) return;
              setDeadlineOpen(false);
              void bulkUpdate({ deadline: format(date, 'yyyy-MM-dd') });
            }}
          />
        </PopoverContent>
      </Popover>

      <Button
        variant="outline"
        size="sm"
        className="h-7 gap-1 px-2 text-xs"
        disabled={loading}
        onClick={() => setArchiveConfirmOpen(true)}
      >
        <ArchiveIcon className="h-3 w-3" />
        {nextArchived ? 'Archive' : 'Unarchive'}
      </Button>

      <Button
        variant="destructive"
        size="sm"
        className="h-7 gap-1 px-2 text-xs"
        disabled={loading}
        onClick={() => setDeleteConfirmOpen(true)}
      >
        <Delete01Icon className="h-3 w-3" />
        Delete
      </Button>

      <Button
        variant="ghost"
        size="sm"
        className="h-7 px-2 text-xs text-muted-foreground"
        disabled={loading}
        onClick={onClearSelection}
      >
        Cancel
      </Button>

      {archiveConfirmOpen ? (
        <ConfirmDialog
          open={archiveConfirmOpen}
          onOpenChange={setArchiveConfirmOpen}
          title={`${nextArchived ? 'Archive' : 'Unarchive'} ${count} ${count === 1 ? 'task' : 'tasks'}?`}
          description={nextArchived ? 'Archived tasks will be hidden from active task lists.' : 'Unarchived tasks will return to active task lists.'}
          confirmLabel={nextArchived ? 'Archive' : 'Unarchive'}
          onConfirm={() => bulkUpdate({ archived: nextArchived }, nextArchived ? 'Archived' : 'Unarchived', nextArchived ? 'archive' : 'unarchive')}
        />
      ) : null}

      {deleteConfirmOpen ? (
        <ConfirmDialog
          open={deleteConfirmOpen}
          onOpenChange={setDeleteConfirmOpen}
          title={`Delete ${count} ${count === 1 ? 'task' : 'tasks'}?`}
          description="This cannot be undone."
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={bulkDelete}
        />
      ) : null}
    </div>
  );
}
