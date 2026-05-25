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
import { Calendar03Icon, ChevronDownIcon, UserAdd01Icon } from '@/lib/pmIcons';
import { ArchiveIcon, Delete01Icon, Loading01Icon, PencilEdit01Icon, Tag01Icon, UserRemove01Icon } from '@/lib/icons';
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
  const [open, setOpen] = useState(false);
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
        setOpen(false);
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

  const fieldRow = 'flex items-center gap-2';
  const fieldLabel = 'w-24 shrink-0 text-xs text-muted-foreground';

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="default"
            size="sm"
            className="h-7 shrink-0 gap-1.5 px-2.5 text-xs"
          >
            <PencilEdit01Icon className="h-3.5 w-3.5" />
            Edit {count} {count === 1 ? 'task' : 'tasks'}
            <ChevronDownIcon className="h-3 w-3 opacity-70" />
          </Button>
        </PopoverTrigger>
        <PopoverContent
          align="end"
          sideOffset={6}
          className="w-[360px] p-3"
          onClick={(event) => event.stopPropagation()}
        >
          <div className="mb-2 flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
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
                  {count} parallel requests
                </Badge>
              ) : null}
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs text-muted-foreground"
              disabled={loading}
              onClick={() => {
                onClearSelection();
                setOpen(false);
              }}
            >
              Cancel
            </Button>
          </div>

          <div className="flex flex-col gap-2">
            <div className={fieldRow}>
              <span className={fieldLabel}>Status</span>
              <Select
                size="sm"
                disabled={loading}
                onValueChange={(value) => void bulkUpdate({ workflow_state_id: value }, 'Updated')}
              >
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue placeholder="No change" />
                </SelectTrigger>
                <SelectContent>
                  {workflow.states.map((state) => (
                    <SelectItem key={state.id} value={state.id}>
                      {state.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <BulkOwnerActionRow
              mode="add"
              labelText="Add owners"
              memberIds={addOwnerIds}
              members={assignableMembers}
              disabled={loading}
              onChange={setAddOwnerIds}
              onApply={() => void bulkAddOwners(addOwnerIds)}
              labelClassName={fieldLabel}
              rowClassName={fieldRow}
            />

            <BulkOwnerActionRow
              mode="remove"
              labelText="Remove owners"
              memberIds={removeOwnerIds}
              members={assignableMembers}
              disabled={loading}
              onChange={setRemoveOwnerIds}
              onApply={() => void bulkRemoveOwners(removeOwnerIds)}
              labelClassName={fieldLabel}
              rowClassName={fieldRow}
            />

            <div className={fieldRow}>
              <span className={fieldLabel}>Priority</span>
              <Select
                size="sm"
                disabled={loading}
                onValueChange={(value) => void bulkUpdate({ priority: value as Priority }, 'Updated')}
              >
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue placeholder="No change" />
                </SelectTrigger>
                <SelectContent>
                  {ALL_PRIORITIES.map((priority) => (
                    <SelectItem key={priority} value={priority}>
                      {PRIORITY_CONFIG[priority].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Severity</span>
              <Select
                size="sm"
                disabled={loading}
                onValueChange={(value) => void bulkUpdate({ severity: value as Severity }, 'Updated')}
              >
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue placeholder="No change" />
                </SelectTrigger>
                <SelectContent>
                  {ALL_SEVERITIES.map((severity) => (
                    <SelectItem key={severity} value={severity}>
                      {SEVERITY_CONFIG[severity].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Epic</span>
              <Select
                size="sm"
                disabled={loading}
                onValueChange={(value) => void bulkUpdate({ epic_id: value === '__none__' ? '' : value }, 'Updated')}
              >
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue placeholder="No change" />
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
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Sprint</span>
              <Select
                size="sm"
                disabled={loading}
                onValueChange={(value) => void bulkUpdate({ sprint_id: value === '__none__' ? '' : value }, 'Updated')}
              >
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue placeholder="No change" />
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
            </div>

            <BulkLabelActionRow
              mode="add"
              labelText="Add labels"
              labelIds={addLabelIds}
              labels={labels}
              workspaceId={workspaceId}
              teamId={teamId ?? undefined}
              disabled={loading}
              onChange={setAddLabelIds}
              onApply={() => void bulkAddLabels(addLabelIds)}
              labelClassName={fieldLabel}
              rowClassName={fieldRow}
            />

            <BulkLabelActionRow
              mode="remove"
              labelText="Remove labels"
              labelIds={removeLabelIds}
              labels={labels}
              workspaceId={workspaceId}
              teamId={teamId ?? undefined}
              disabled={loading}
              onChange={setRemoveLabelIds}
              onApply={() => void bulkRemoveLabels(removeLabelIds)}
              labelClassName={fieldLabel}
              rowClassName={fieldRow}
            />

            <div className={fieldRow}>
              <span className={fieldLabel}>Deadline</span>
              <Popover open={deadlineOpen} onOpenChange={setDeadlineOpen}>
                <PopoverTrigger asChild>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 flex-1 justify-start px-2 text-xs"
                    disabled={loading}
                  >
                    <Calendar03Icon className="mr-1 h-3 w-3" />
                    Set deadline
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start" onClick={(event) => event.stopPropagation()}>
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
            </div>

            <div className="mt-2 flex items-center gap-2 border-t border-border/60 pt-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 flex-1 px-2 text-xs"
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
                className="h-7 flex-1 px-2 text-xs"
                disabled={loading}
                onClick={() => setDeleteConfirmOpen(true)}
              >
                <Delete01Icon className="mr-1 h-3 w-3" />
                Delete
              </Button>
            </div>

            {loading ? (
              <div className="flex items-center justify-center gap-1.5 pt-1 text-xs text-muted-foreground">
                <Loading01Icon className="h-3 w-3 animate-spin" />
                Applying...
              </div>
            ) : null}
          </div>
        </PopoverContent>
      </Popover>

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

function BulkOwnerActionRow({
  mode,
  labelText,
  memberIds,
  members,
  disabled,
  onChange,
  onApply,
  labelClassName,
  rowClassName,
}: {
  mode: 'add' | 'remove';
  labelText: string;
  memberIds: string[];
  members: AssignableMember[];
  disabled: boolean;
  onChange: (memberIds: string[]) => void;
  onApply: () => void;
  labelClassName: string;
  rowClassName: string;
}) {
  const shortLabel = mode === 'add' ? 'Add' : 'Remove';
  const Icon = mode === 'add' ? UserAdd01Icon : UserRemove01Icon;

  return (
    <div className={rowClassName}>
      <span className={labelClassName}>{labelText}</span>
      <div className="flex flex-1 items-center gap-1">
        <MultiMemberPickerPopover
          values={memberIds}
          members={members}
          disabled={disabled}
          onChange={onChange}
          triggerClassName={cn(
            'flex h-7 flex-1 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs',
            disabled && 'pointer-events-none opacity-50',
          )}
          triggerLabel={labelText}
          contentClassName="w-[260px]"
          renderTrigger={() => (
            <span className="flex items-center gap-1 text-muted-foreground">
              <Icon className="h-3 w-3" />
              {memberIds.length > 0 ? `${memberIds.length} selected` : 'Choose members'}
            </span>
          )}
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs"
          disabled={disabled || memberIds.length === 0}
          onClick={onApply}
          aria-label={labelText}
        >
          {memberIds.length > 0 ? `${shortLabel} ${memberIds.length}` : shortLabel}
        </Button>
      </div>
    </div>
  );
}

function BulkLabelActionRow({
  mode,
  labelText,
  labelIds,
  labels,
  workspaceId,
  teamId,
  disabled,
  onChange,
  onApply,
  labelClassName,
  rowClassName,
}: {
  mode: 'add' | 'remove';
  labelText: string;
  labelIds: string[];
  labels: Label[];
  workspaceId: string;
  teamId?: string;
  disabled: boolean;
  onChange: (labelIds: string[]) => void;
  onApply: () => void;
  labelClassName: string;
  rowClassName: string;
}) {
  const shortLabel = mode === 'add' ? 'Add' : 'Remove';

  return (
    <div className={rowClassName}>
      <span className={labelClassName}>{labelText}</span>
      <div className="flex flex-1 items-center gap-1">
        <LabelPicker
          workspaceId={workspaceId}
          teamId={teamId}
          selectedLabelIds={labelIds}
          onChange={onChange}
          labels={labels}
          triggerOnly
          className={cn(
            'h-7 flex-1 rounded-md border border-input px-1',
            disabled && 'pointer-events-none opacity-50',
          )}
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs"
          disabled={disabled || labelIds.length === 0}
          onClick={onApply}
          aria-label={labelText}
        >
          <Tag01Icon className="mr-1 h-3 w-3" />
          {labelIds.length > 0 ? `${shortLabel} ${labelIds.length}` : shortLabel}
        </Button>
      </div>
    </div>
  );
}
