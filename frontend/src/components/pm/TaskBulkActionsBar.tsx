import { useCallback, useMemo, useState } from 'react';
import { format } from 'date-fns';
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
import { MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { PRIORITY_CONFIG, SEVERITY_CONFIG } from '@/lib/pmConstants';
import { Calendar03Icon, UserAdd01Icon } from '@/lib/pmIcons';
import { ArchiveIcon, Delete01Icon, Loading01Icon, Tag01Icon, UserRemove01Icon } from '@/lib/icons';
import { pmTaskService } from '@/lib/services/pmTaskService';
import type {
  EpicWithStats,
  Label,
  Priority,
  Severity,
  SprintWithStats,
  Task,
  UpdateTaskRequest,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];
export const BULK_SOFT_CAP = 25;

type BulkOperationResult = Promise<{ error: string | null }>;

interface TaskBulkActionsBarProps {
  selectedTasks: Task[];
  workspaceId: string;
  teamId?: string | null;
  workflow: WorkflowWithStates;
  assignableMembers: AssignableMember[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  labels: Label[];
  onComplete: () => void | Promise<void>;
  onClearSelection: () => void;
}

export function getLabelIdsAfterAdd(task: Pick<Task, 'labels'>, labelIdsToAdd: string[]) {
  const currentIds = (task.labels ?? []).map((label) => label.id);
  return Array.from(new Set([...currentIds, ...labelIdsToAdd]));
}

export function getLabelIdsAfterRemove(task: Pick<Task, 'labels'>, labelIdsToRemove: string[]) {
  const removeSet = new Set(labelIdsToRemove);
  return (task.labels ?? []).map((label) => label.id).filter((id) => !removeSet.has(id));
}

export function getOwnerIdsAfterAdd(task: Pick<Task, 'owner_member_ids'>, memberIdsToAdd: string[]) {
  const current = task.owner_member_ids ?? [];
  return Array.from(new Set([...current, ...memberIdsToAdd]));
}

export function getOwnerIdsAfterRemove(task: Pick<Task, 'owner_member_ids'>, memberIdsToRemove: string[]) {
  const removeSet = new Set(memberIdsToRemove);
  return (task.owner_member_ids ?? []).filter((id) => !removeSet.has(id));
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
  onComplete,
  onClearSelection,
}: TaskBulkActionsBarProps) {
  const [loading, setLoading] = useState(false);
  const [addLabelIds, setAddLabelIds] = useState<string[]>([]);
  const [removeLabelIds, setRemoveLabelIds] = useState<string[]>([]);
  const [addOwnerIds, setAddOwnerIds] = useState<string[]>([]);
  const [removeOwnerIds, setRemoveOwnerIds] = useState<string[]>([]);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deadlineOpen, setDeadlineOpen] = useState(false);
  const count = selectedTasks.length;
  const overSoftCap = count >= BULK_SOFT_CAP;
  const allArchived = useMemo(
    () => selectedTasks.length > 0 && selectedTasks.every((task) => task.archived),
    [selectedTasks],
  );

  const reportResults = useCallback(
    async (
      results: PromiseSettledResult<{ error: string | null }>[],
      verb: string,
    ) => {
      const successCount = results.filter((result) => (
        result.status === 'fulfilled' && !result.value.error
      )).length;
      const failureCount = results.length - successCount;

      if (failureCount === 0) {
        toast.success(`${verb} ${successCount} ${successCount === 1 ? 'task' : 'tasks'}`);
      } else if (successCount > 0) {
        toast.warning(`${verb} ${successCount} of ${results.length} tasks`, {
          description: `${failureCount} ${failureCount === 1 ? 'task failed' : 'tasks failed'}.`,
        });
      } else {
        toast.error('Bulk operation failed', {
          description: `${failureCount} ${failureCount === 1 ? 'request failed' : 'requests failed'}.`,
        });
      }

      await onComplete();
      if (successCount > 0) {
        onClearSelection();
      }
    },
    [onClearSelection, onComplete],
  );

  const runBulkOperation = useCallback(
    async (verb: string, operation: () => Promise<PromiseSettledResult<{ error: string | null }>[]>): Promise<void> => {
      if (loading || count === 0) return;
      setLoading(true);
      try {
        const results = await operation();
        await reportResults(results, verb);
      } finally {
        setLoading(false);
      }
    },
    [count, loading, reportResults],
  );

  const bulkUpdate = useCallback(
    (patch: UpdateTaskRequest, verb = 'Updated') => runBulkOperation(
      verb,
      () => Promise.allSettled(
        selectedTasks.map((task) => pmTaskService.update(workspaceId, task.id, patch) as BulkOperationResult),
      ),
    ),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  const bulkAddLabels = useCallback(
    (labelIdsToAdd: string[]) => runBulkOperation(
      'Added labels to',
      () => Promise.allSettled(
        selectedTasks.map((task) =>
          pmTaskService.update(workspaceId, task.id, {
            label_ids: getLabelIdsAfterAdd(task, labelIdsToAdd),
          }) as BulkOperationResult,
        ),
      ),
    ).then(() => setAddLabelIds([])),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  const bulkRemoveLabels = useCallback(
    (labelIdsToRemove: string[]) => runBulkOperation(
      'Removed labels from',
      () => Promise.allSettled(
        selectedTasks.map((task) =>
          pmTaskService.update(workspaceId, task.id, {
            label_ids: getLabelIdsAfterRemove(task, labelIdsToRemove),
          }) as BulkOperationResult,
        ),
      ),
    ).then(() => setRemoveLabelIds([])),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  const bulkAddOwners = useCallback(
    (memberIdsToAdd: string[]) => runBulkOperation(
      'Added owners to',
      () => Promise.allSettled(
        selectedTasks.map((task) =>
          pmTaskService.update(workspaceId, task.id, {
            owner_member_ids: getOwnerIdsAfterAdd(task, memberIdsToAdd),
          }) as BulkOperationResult,
        ),
      ),
    ).then(() => setAddOwnerIds([])),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  const bulkRemoveOwners = useCallback(
    (memberIdsToRemove: string[]) => runBulkOperation(
      'Removed owners from',
      () => Promise.allSettled(
        selectedTasks.map((task) =>
          pmTaskService.update(workspaceId, task.id, {
            owner_member_ids: getOwnerIdsAfterRemove(task, memberIdsToRemove),
          }) as BulkOperationResult,
        ),
      ),
    ).then(() => setRemoveOwnerIds([])),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  const bulkDelete = useCallback(
    () => runBulkOperation(
      'Deleted',
      () => Promise.allSettled(
        selectedTasks.map((task) => pmTaskService.remove(workspaceId, task.id) as BulkOperationResult),
      ),
    ),
    [runBulkOperation, selectedTasks, workspaceId],
  );

  if (count === 0) return null;

  return (
    <>
      <div className="animate-in slide-in-from-bottom-2 absolute bottom-4 left-1/2 z-20 flex max-w-[calc(100%-2rem)] -translate-x-1/2 items-center gap-2 overflow-x-auto rounded-lg border bg-card px-3 py-2 shadow-lg">
        <Badge variant="secondary" className="shrink-0 text-xs">
          {count} selected
        </Badge>
        {overSoftCap ? (
          <Badge
            variant="outline"
            className="shrink-0 border-amber-300 bg-amber-50 text-[10px] text-amber-700 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200"
            role="status"
            aria-label="Large selection warning"
          >
            Large selection — {count} parallel requests
          </Badge>
        ) : null}

        <Select
          size="sm"
          disabled={loading}
          onValueChange={(value) => void bulkUpdate({ workflow_state_id: value }, 'Updated')}
        >
          <SelectTrigger className="h-7 w-[130px] text-xs">
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

        <BulkOwnerAction
          mode="add"
          memberIds={addOwnerIds}
          members={assignableMembers}
          disabled={loading}
          onChange={setAddOwnerIds}
          onApply={() => void bulkAddOwners(addOwnerIds)}
        />

        <BulkOwnerAction
          mode="remove"
          memberIds={removeOwnerIds}
          members={assignableMembers}
          disabled={loading}
          onChange={setRemoveOwnerIds}
          onApply={() => void bulkRemoveOwners(removeOwnerIds)}
        />

        <Select
          size="sm"
          disabled={loading}
          onValueChange={(value) => void bulkUpdate({ priority: value as Priority }, 'Updated')}
        >
          <SelectTrigger className="h-7 w-[105px] text-xs">
            <SelectValue placeholder="Priority" />
          </SelectTrigger>
          <SelectContent>
            {ALL_PRIORITIES.map((priority) => (
              <SelectItem key={priority} value={priority}>
                {PRIORITY_CONFIG[priority].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          size="sm"
          disabled={loading}
          onValueChange={(value) => void bulkUpdate({ severity: value as Severity }, 'Updated')}
        >
          <SelectTrigger className="h-7 w-[105px] text-xs">
            <SelectValue placeholder="Severity" />
          </SelectTrigger>
          <SelectContent>
            {ALL_SEVERITIES.map((severity) => (
              <SelectItem key={severity} value={severity}>
                {SEVERITY_CONFIG[severity].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          size="sm"
          disabled={loading}
          onValueChange={(value) => void bulkUpdate({ epic_id: value === '__none__' ? '' : value }, 'Updated')}
        >
          <SelectTrigger className="h-7 w-[110px] text-xs">
            <SelectValue placeholder="Epic" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__none__">No epic</SelectItem>
            {epics.map((entry) => (
              <SelectItem key={entry.epic.id} value={entry.epic.id}>
                {entry.epic.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          size="sm"
          disabled={loading}
          onValueChange={(value) => void bulkUpdate({ sprint_id: value === '__none__' ? '' : value }, 'Updated')}
        >
          <SelectTrigger className="h-7 w-[110px] text-xs">
            <SelectValue placeholder="Sprint" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__none__">No sprint</SelectItem>
            {sprints.map((entry) => (
              <SelectItem key={entry.sprint.id} value={entry.sprint.id}>
                {entry.sprint.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <BulkLabelAction
          mode="add"
          labelIds={addLabelIds}
          labels={labels}
          workspaceId={workspaceId}
          teamId={teamId ?? undefined}
          disabled={loading}
          onChange={setAddLabelIds}
          onApply={() => void bulkAddLabels(addLabelIds)}
        />

        <BulkLabelAction
          mode="remove"
          labelIds={removeLabelIds}
          labels={labels}
          workspaceId={workspaceId}
          teamId={teamId ?? undefined}
          disabled={loading}
          onChange={setRemoveLabelIds}
          onApply={() => void bulkRemoveLabels(removeLabelIds)}
        />

        <Popover open={deadlineOpen} onOpenChange={setDeadlineOpen}>
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 shrink-0 px-2 text-xs"
              disabled={loading}
            >
              <Calendar03Icon className="mr-1 h-3 w-3" />
              Deadline
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto p-0" align="center" onClick={(event) => event.stopPropagation()}>
            <Calendar
              mode="single"
              onSelect={(date) => {
                if (!date) return;
                setDeadlineOpen(false);
                void bulkUpdate({ deadline: format(date, 'yyyy-MM-dd') }, 'Updated');
              }}
            />
          </PopoverContent>
        </Popover>

        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs"
          disabled={loading}
          onClick={() => {
            if (allArchived) {
              void bulkUpdate({ archived: false }, 'Unarchived');
            } else {
              setArchiveConfirmOpen(true);
            }
          }}
        >
          <ArchiveIcon className="mr-1 h-3 w-3" />
          {allArchived ? 'Unarchive' : 'Archive'}
        </Button>

        <Button
          type="button"
          variant="destructive"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs"
          disabled={loading}
          onClick={() => setDeleteConfirmOpen(true)}
        >
          <Delete01Icon className="mr-1 h-3 w-3" />
          Delete
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs text-muted-foreground"
          disabled={loading}
          onClick={onClearSelection}
        >
          {loading ? <Loading01Icon className="mr-1 h-3 w-3 animate-spin" /> : null}
          Cancel
        </Button>
      </div>

      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title={`Archive ${count} ${count === 1 ? 'task' : 'tasks'}?`}
        description="Archived tasks are hidden from active task lists until they are unarchived."
        confirmLabel="Archive"
        onConfirm={() => {
          setArchiveConfirmOpen(false);
          void bulkUpdate({ archived: true }, 'Archived');
        }}
      />

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={`Delete ${count} ${count === 1 ? 'task' : 'tasks'}?`}
        description="This cannot be undone."
        confirmLabel="Delete"
        onConfirm={() => {
          setDeleteConfirmOpen(false);
          void bulkDelete();
        }}
      />
    </>
  );
}

function BulkOwnerAction({
  mode,
  memberIds,
  members,
  disabled,
  onChange,
  onApply,
}: {
  mode: 'add' | 'remove';
  memberIds: string[];
  members: AssignableMember[];
  disabled: boolean;
  onChange: (memberIds: string[]) => void;
  onApply: () => void;
}) {
  const label = mode === 'add' ? 'Add owners' : 'Remove owners';
  const shortLabel = mode === 'add' ? 'Add' : 'Remove';
  const Icon = mode === 'add' ? UserAdd01Icon : UserRemove01Icon;

  return (
    <div className="flex shrink-0 items-center gap-1">
      <MultiMemberPickerPopover
        values={memberIds}
        members={members}
        disabled={disabled}
        onChange={onChange}
        triggerClassName={cn(
          'flex h-7 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs',
          disabled && 'pointer-events-none opacity-50',
        )}
        triggerLabel={label}
        contentClassName="w-[240px]"
        renderTrigger={() => (
          <span className="flex items-center gap-1 text-muted-foreground">
            <Icon className="h-3 w-3" />
            {mode === 'add' ? 'Owners' : 'Unassign'}
          </span>
        )}
      />
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="h-7 px-2 text-xs"
        disabled={disabled || memberIds.length === 0}
        onClick={onApply}
        aria-label={label}
      >
        <Icon className="mr-1 h-3 w-3" />
        {memberIds.length > 0 ? `${shortLabel} ${memberIds.length}` : shortLabel}
      </Button>
    </div>
  );
}

function BulkLabelAction({
  mode,
  labelIds,
  labels,
  workspaceId,
  teamId,
  disabled,
  onChange,
  onApply,
}: {
  mode: 'add' | 'remove';
  labelIds: string[];
  labels: Label[];
  workspaceId: string;
  teamId?: string;
  disabled: boolean;
  onChange: (labelIds: string[]) => void;
  onApply: () => void;
}) {
  const label = mode === 'add' ? 'Add labels' : 'Remove labels';
  const shortLabel = mode === 'add' ? 'Add' : 'Remove';

  return (
    <div className="flex shrink-0 items-center gap-1">
      <LabelPicker
        workspaceId={workspaceId}
        teamId={teamId}
        selectedLabelIds={labelIds}
        onChange={onChange}
        labels={labels}
        triggerOnly
        className={cn(
          'h-7 rounded-md border border-input px-1',
          disabled && 'pointer-events-none opacity-50',
        )}
      />
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="h-7 px-2 text-xs"
        disabled={disabled || labelIds.length === 0}
        onClick={onApply}
        aria-label={label}
      >
        <Tag01Icon className="mr-1 h-3 w-3" />
        {labelIds.length > 0 ? `${shortLabel} ${labelIds.length}` : shortLabel}
      </Button>
    </div>
  );
}
