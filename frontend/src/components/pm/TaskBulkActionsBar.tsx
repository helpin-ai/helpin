import { EpicBadge } from './EpicBadge';
import { groupEpicsByLifecycle } from './epicPickerGroups';
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
  SelectGroup,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/quiet-dropdown-select';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { LabelBadge, LabelPicker } from '@/components/pm/LabelPicker';
import { MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { PRIORITY_CONFIG, SEVERITY_CONFIG } from '@/lib/pmConstants';
import { Calendar03Icon, ChevronDownIcon } from '@/lib/pmIcons';
import { ArchiveIcon, Cancel01Icon, Loading01Icon, PencilEdit01Icon, PlusSignIcon } from '@/lib/icons';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { useSprints } from '@/hooks/queries/useSprints';
import { useEpics } from '@/hooks/queries/useEpics';
import { useLabels } from '@/hooks/queries/useLabels';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
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
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];
export const BULK_SOFT_CAP = 25;
const MIXED = Symbol('mixed');

// Stable empty fallbacks so memoized derivations don't see a new array ref each render.
const EMPTY_SPRINTS: SprintWithStats[] = [];
const EMPTY_EPICS: EpicWithStats[] = [];
const EMPTY_LABELS: Label[] = [];

type BulkOperationResult = Promise<{ error: string | null }>;

interface TaskBulkActionsBarProps {
  selectedTasks: Task[];
  workspaceId: string;
  teamId?: string | null;
  workflow: WorkflowWithStates;
  /** All workflows in the workspace — used to re-scope Status on a team change. */
  workflows?: WorkflowWithStates[];
  /** Workspace teams — used to populate the bulk Team selector. */
  teams?: WorkspaceTeam[];
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

/**
 * Staged bulk-edit changes. `undefined` scalar = no change; `null` for
 * epic/sprint/deadline = clear. `teamId` set means the team is being changed,
 * which re-scopes and resets the team-scoped fields (see buildBulkPatch).
 */
export interface BulkStagedChanges {
  status: string | undefined;
  priority: Priority | undefined;
  severity: Severity | undefined;
  epicId: string | null | undefined;
  sprintId: string | null | undefined;
  deadline: string | null | undefined;
  /** Staged team change (undefined = unchanged). */
  teamId: string | undefined;
  /** Workflow id of the staged team — sent alongside the chosen state. */
  newTeamWorkflowId: string | undefined;
  ownerAdds: Set<string>;
  ownerRemoves: Set<string>;
  labelAdds: Set<string>;
  labelRemoves: Set<string>;
}

/**
 * Resolves the workflow to use for a staged team change. Prefers a workflow
 * owned by the team, then the shared/default workflow (no team_id) — mirroring
 * the backend's GetDefaultWorkflow ordering (team_id IS NULL is the general
 * workflow). Returns null only when no usable workflow exists.
 */
export function resolveTeamWorkflow(
  workflows: WorkflowWithStates[] | undefined,
  teamId: string,
): WorkflowWithStates | null {
  if (!workflows || workflows.length === 0) return null;
  return (
    workflows.find((w) => w.workflow.team_id === teamId)
    ?? workflows.find((w) => !w.workflow.team_id)
    ?? null
  );
}

/**
 * Resolves the workflow that owns the selected tasks' current statuses. List
 * views can be scoped by a page-level workflow while still showing tasks from
 * team workflows, so status edits should follow the selected task when the
 * selection is unambiguous.
 */
export function resolveSelectionWorkflow(
  selectedTasks: Pick<Task, 'workflow_id'>[],
  workflows: WorkflowWithStates[] | undefined,
  fallbackWorkflow: WorkflowWithStates,
): WorkflowWithStates {
  if (selectedTasks.length === 0) return fallbackWorkflow;
  const firstWorkflowId = selectedTasks[0]?.workflow_id;
  if (!firstWorkflowId) return fallbackWorkflow;
  if (selectedTasks.some((task) => task.workflow_id !== firstWorkflowId)) {
    return fallbackWorkflow;
  }
  return workflows?.find((candidate) => candidate.workflow.id === firstWorkflowId) ?? fallbackWorkflow;
}

/**
 * Builds the per-task UpdateTaskRequest from staged changes.
 *
 * On a team change the backend revalidates the task's existing epic/sprint and
 * the state↔workflow pairing against the new team (server/internal/service/
 * pm_task.go), so we must explicitly clear team-scoped links unless the user
 * picked replacements: epic_id/sprint_id are cleared, all labels are cleared
 * (keeping only newly chosen ones), and workflow_id is sent with the required
 * new-team state. Team-independent fields (priority/severity/deadline/owners)
 * apply in both cases.
 */
export function buildBulkPatch(
  task: Pick<Task, 'owner_member_ids' | 'labels'>,
  staged: BulkStagedChanges,
): UpdateTaskRequest {
  const patch: UpdateTaskRequest = {};
  const teamChanged = staged.teamId !== undefined;

  if (teamChanged) {
    patch.team_id = staged.teamId;
    patch.epic_id = staged.epicId ?? '';
    patch.sprint_id = staged.sprintId ?? '';
    patch.label_ids = Array.from(staged.labelAdds);
    if (staged.newTeamWorkflowId !== undefined) patch.workflow_id = staged.newTeamWorkflowId;
    if (staged.status !== undefined) patch.workflow_state_id = staged.status;
  } else {
    if (staged.status !== undefined) patch.workflow_state_id = staged.status;
    if (staged.epicId !== undefined) patch.epic_id = staged.epicId ?? '';
    if (staged.sprintId !== undefined) patch.sprint_id = staged.sprintId ?? '';
    if (staged.labelAdds.size > 0 || staged.labelRemoves.size > 0) {
      const current = (task.labels ?? []).map((l) => l.id);
      patch.label_ids = Array.from(new Set([
        ...current.filter((id) => !staged.labelRemoves.has(id)),
        ...staged.labelAdds,
      ]));
    }
  }

  if (staged.priority !== undefined) patch.priority = staged.priority;
  if (staged.severity !== undefined) patch.severity = staged.severity;
  if (staged.deadline !== undefined) patch.deadline = staged.deadline ?? undefined;
  if (staged.ownerAdds.size > 0 || staged.ownerRemoves.size > 0) {
    const current = task.owner_member_ids ?? [];
    patch.owner_member_ids = Array.from(new Set([
      ...current.filter((id) => !staged.ownerRemoves.has(id)),
      ...staged.ownerAdds,
    ]));
  }

  return patch;
}

type SetField = {
  intersection: string[];
  union: string[];
  partial: string[];
  shared: boolean;
};

export function deriveSetField(values: string[][]): SetField {
  if (values.length === 0) {
    return { intersection: [], union: [], partial: [], shared: true };
  }
  const sets = values.map((v) => new Set(v));
  const union = Array.from(new Set(values.flat()));
  const intersection = union.filter((id) => sets.every((s) => s.has(id)));
  const partial = union.filter((id) => !intersection.includes(id));
  return { intersection, union, partial, shared: partial.length === 0 };
}

type ChipState = 'shared' | 'partial' | 'pending-add';

function deriveChipDisplay(
  field: SetField,
  pendingAdds: Set<string>,
  pendingRemoves: Set<string>,
): { id: string; state: ChipState }[] {
  const result: { id: string; state: ChipState }[] = [];
  const seen = new Set<string>();
  // First, items from the union (in stable order).
  for (const id of field.union) {
    if (pendingRemoves.has(id)) continue;
    seen.add(id);
    if (pendingAdds.has(id)) result.push({ id, state: 'pending-add' });
    else if (field.intersection.includes(id)) result.push({ id, state: 'shared' });
    else result.push({ id, state: 'partial' });
  }
  // Then, brand-new pendingAdds that weren't in the union (added via "+ Add").
  for (const id of pendingAdds) {
    if (seen.has(id)) continue;
    if (pendingRemoves.has(id)) continue;
    result.push({ id, state: 'pending-add' });
  }
  return result;
}

export function TaskBulkActionsBar({
  selectedTasks,
  workspaceId,
  teamId,
  workflow,
  workflows,
  teams,
  assignableMembers,
  epics,
  sprints,
  labels,
  onComplete,
  onClearSelection,
}: TaskBulkActionsBarProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  // Staged scalar field changes (undefined = no change). `null` for epic/sprint/deadline = clear.
  const [stagedStatus, setStagedStatus] = useState<string | undefined>();
  const [stagedPriority, setStagedPriority] = useState<Priority | undefined>();
  const [stagedSeverity, setStagedSeverity] = useState<Severity | undefined>();
  const [stagedEpicId, setStagedEpicId] = useState<string | null | undefined>();
  const [stagedSprintId, setStagedSprintId] = useState<string | null | undefined>();
  const [stagedDeadline, setStagedDeadline] = useState<string | null | undefined>();
  const [stagedTeamId, setStagedTeamId] = useState<string | undefined>();

  // Staged set-field deltas (owners + labels).
  const [ownerAdds, setOwnerAdds] = useState<Set<string>>(new Set());
  const [ownerRemoves, setOwnerRemoves] = useState<Set<string>>(new Set());
  const [labelAdds, setLabelAdds] = useState<Set<string>>(new Set());
  const [labelRemoves, setLabelRemoves] = useState<Set<string>>(new Set());

  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [deadlineOpen, setDeadlineOpen] = useState(false);
  const [addOwnerOpen, setAddOwnerOpen] = useState(false);

  const teamChanged = stagedTeamId !== undefined;

  // When a team is staged, re-scope the team-scoped fields to that team. Passing
  // an empty workspaceId disables the query (hooks gate on `enabled: !!wsId`),
  // so nothing is fetched until the user actually changes the team.
  const teamSprintsQuery = useSprints(
    teamChanged ? workspaceId : '',
    teamChanged ? { team_id: stagedTeamId } : undefined,
  );
  const teamEpicsQuery = useEpics(
    teamChanged ? workspaceId : '',
    teamChanged ? { team_id: stagedTeamId } : undefined,
  );
  const teamLabelsQuery = useLabels(
    teamChanged ? workspaceId : '',
    teamChanged ? { teamId: stagedTeamId, includeShared: true } : undefined,
  );

  const effectiveWorkflow = teamChanged
    ? resolveTeamWorkflow(workflows, stagedTeamId)
    : resolveSelectionWorkflow(selectedTasks, workflows, workflow);
  const effectiveSprints = teamChanged ? (teamSprintsQuery.data ?? EMPTY_SPRINTS) : sprints;
  const effectiveEpics = teamChanged ? (teamEpicsQuery.data ?? EMPTY_EPICS) : epics;
  const effectiveLabels = teamChanged ? (teamLabelsQuery.data ?? EMPTY_LABELS) : labels;
  const effectiveTeamId = teamChanged ? stagedTeamId : (teamId ?? undefined);
  const teamScopedLoading = teamChanged && (
    teamSprintsQuery.isLoading || teamEpicsQuery.isLoading || teamLabelsQuery.isLoading
  );

  const count = selectedTasks.length;
  const overSoftCap = count >= BULK_SOFT_CAP;
  const allArchived = useMemo(
    () => selectedTasks.length > 0 && selectedTasks.every((task) => task.archived),
    [selectedTasks],
  );

  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const memberById = useMemo(() => {
    const map = new Map<string, AssignableMember>();
    for (const m of assignableMembers) map.set(m.id, m);
    return map;
  }, [assignableMembers]);

  const ownersField = useMemo(
    () => deriveSetField(selectedTasks.map((t) => t.owner_member_ids ?? [])),
    [selectedTasks],
  );
  const labelsField = useMemo(
    () => deriveSetField(selectedTasks.map((t) => (t.labels ?? []).map((l) => l.id))),
    [selectedTasks],
  );

  const labelById = useMemo(() => {
    const map = new Map<string, Label>();
    for (const l of effectiveLabels) map.set(l.id, l);
    return map;
  }, [effectiveLabels]);

  const ownerChips = useMemo(
    () => deriveChipDisplay(ownersField, ownerAdds, ownerRemoves),
    [ownersField, ownerAdds, ownerRemoves],
  );
  // On a team change the existing (old-team) labels are cleared, so the chip
  // list shows only labels the user newly picks for the new team.
  const labelChips = useMemo(
    () => deriveChipDisplay(
      teamChanged ? { intersection: [], union: [], partial: [], shared: true } : labelsField,
      labelAdds,
      labelRemoves,
    ),
    [teamChanged, labelsField, labelAdds, labelRemoves],
  );

  const commonValues = useMemo(() => {
    function shared<V>(getter: (task: Task) => V): V | typeof MIXED {
      if (selectedTasks.length === 0) return MIXED;
      const first = getter(selectedTasks[0]);
      for (let i = 1; i < selectedTasks.length; i += 1) {
        if (getter(selectedTasks[i]) !== first) return MIXED;
      }
      return first;
    }
    return {
      workflow_state_id: shared((t) => t.workflow_state_id),
      priority: shared((t) => t.priority),
      severity: shared((t) => t.severity),
      epic_id: shared((t) => t.epic_id ?? ''),
      sprint_id: shared((t) => t.sprint_id ?? ''),
      deadline: shared((t) => t.deadline ?? ''),
      team_id: shared((t) => t.team_id ?? ''),
    };
  }, [selectedTasks]);

  const resetStaged = useCallback(() => {
    setStagedStatus(undefined);
    setStagedPriority(undefined);
    setStagedSeverity(undefined);
    setStagedEpicId(undefined);
    setStagedSprintId(undefined);
    setStagedDeadline(undefined);
    setStagedTeamId(undefined);
    setOwnerAdds(new Set());
    setOwnerRemoves(new Set());
    setLabelAdds(new Set());
    setLabelRemoves(new Set());
  }, []);

  // Changing the team resets the team-scoped fields: epic/sprint are cleared,
  // labels are cleared, and a fresh status pick from the new team's workflow is
  // required before Apply is enabled.
  const handleTeamChange = useCallback((nextTeamId: string) => {
    setStagedTeamId(nextTeamId);
    setStagedStatus(undefined);
    setStagedEpicId(null);
    setStagedSprintId(null);
    setLabelAdds(new Set());
    setLabelRemoves(new Set());
  }, []);

  const hasStagedChanges =
    stagedStatus !== undefined
    || stagedPriority !== undefined
    || stagedSeverity !== undefined
    || stagedEpicId !== undefined
    || stagedSprintId !== undefined
    || stagedDeadline !== undefined
    || stagedTeamId !== undefined
    || ownerAdds.size > 0
    || ownerRemoves.size > 0
    || labelAdds.size > 0
    || labelRemoves.size > 0;

  // On a team change a valid status from the new team's workflow is mandatory.
  const canApply = hasStagedChanges && !(teamChanged && stagedStatus === undefined);

  // The shared team of the selection, but only if it's a team the current user
  // can actually pick (present in `teams`). Otherwise the Select would render a
  // blank trigger for an id with no matching option.
  const commonTeamId = commonValues.team_id !== MIXED && commonValues.team_id
    ? (commonValues.team_id as string)
    : undefined;
  const selectableCommonTeamId = commonTeamId && (teams ?? []).some((t) => t.id === commonTeamId)
    ? commonTeamId
    : undefined;

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
        resetStaged();
        onClearSelection();
        setOpen(false);
      }
    },
    [onClearSelection, onComplete, resetStaged],
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

  // Single Apply commits every staged change in one round of per-task PUTs.
  const handleApply = useCallback(async () => {
    if (!canApply || loading || count === 0) return;
    const staged: BulkStagedChanges = {
      status: stagedStatus,
      priority: stagedPriority,
      severity: stagedSeverity,
      epicId: stagedEpicId,
      sprintId: stagedSprintId,
      deadline: stagedDeadline,
      teamId: stagedTeamId,
      newTeamWorkflowId: teamChanged ? effectiveWorkflow?.workflow.id : undefined,
      ownerAdds,
      ownerRemoves,
      labelAdds,
      labelRemoves,
    };
    setLoading(true);
    try {
      const results = await Promise.allSettled(
        selectedTasks.map((task) => (
          pmTaskService.update(workspaceId, task.id, buildBulkPatch(task, staged)) as BulkOperationResult
        )),
      );
      await reportResults(results, 'Updated');
    } finally {
      setLoading(false);
    }
  }, [
    canApply,
    count,
    effectiveWorkflow,
    labelAdds,
    labelRemoves,
    loading,
    ownerAdds,
    ownerRemoves,
    reportResults,
    selectedTasks,
    stagedDeadline,
    stagedEpicId,
    stagedPriority,
    stagedSeverity,
    stagedSprintId,
    stagedStatus,
    stagedTeamId,
    teamChanged,
    workspaceId,
  ]);

  // Owner chip handlers
  const promoteOwner = (id: string) => {
    setOwnerAdds((prev) => new Set([...prev, id]));
    setOwnerRemoves((prev) => {
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };
  const removeOwnerChip = (id: string, state: ChipState) => {
    if (state === 'pending-add') {
      // Undo the promote (or remove the newly added). If it was originally part of union, falls back to partial display.
      setOwnerAdds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      // If the id was NOT in the original union (i.e. it was a brand-new add), we don't need to mark it removed.
      if (!ownersField.union.includes(id)) return;
      // Otherwise (was originally partial), revert leaves it as partial — no remove queued.
      return;
    }
    setOwnerRemoves((prev) => new Set([...prev, id]));
  };

  // Label chip handlers
  const promoteLabel = (id: string) => {
    setLabelAdds((prev) => new Set([...prev, id]));
    setLabelRemoves((prev) => {
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };
  const removeLabelChip = (id: string, state: ChipState) => {
    if (state === 'pending-add') {
      setLabelAdds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      if (!labelsField.union.includes(id)) return;
      return;
    }
    setLabelRemoves((prev) => new Set([...prev, id]));
  };

  if (count === 0) return null;

  const fieldRow = 'flex items-center gap-2';
  const fieldLabel = 'w-24 shrink-0 text-xs text-muted-foreground';

  return (
    <>
      <Popover
        open={open}
        onOpenChange={(next) => {
          if (!next) resetStaged();
          setOpen(next);
        }}
      >
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
          align="start"
          sideOffset={6}
          className="w-[400px] p-3"
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
          </div>

          <div className="flex flex-col gap-2">
            {teams && teams.length > 0 ? (
              <div className={fieldRow}>
                <span className={fieldLabel}>Team</span>
                <Select
                  size="sm"
                  disabled={loading}
                  value={stagedTeamId ?? selectableCommonTeamId}
                  onValueChange={handleTeamChange}
                >
                  <SelectTrigger className={cn(
                    'h-7 flex-1 text-xs',
                    stagedTeamId !== undefined && 'border-primary',
                  )}>
                    <SelectValue placeholder={commonValues.team_id === MIXED ? 'Multiple' : 'Select team'} />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        {team.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ) : null}
            {teamChanged ? (
              <p className="-mt-1 pl-24 text-[10px] text-muted-foreground">
                Switching team clears each task's epic, sprint, and labels.
              </p>
            ) : null}

            <div className={fieldRow}>
              <span className={fieldLabel}>Status</span>
              <Select
                size="sm"
                disabled={loading || (teamChanged && !effectiveWorkflow)}
                value={stagedStatus
                  ?? (teamChanged
                    ? undefined
                    : (commonValues.workflow_state_id === MIXED ? undefined : (commonValues.workflow_state_id as string)))}
                onValueChange={(value) => setStagedStatus(value)}
              >
                <SelectTrigger className={cn(
                  'h-7 flex-1 text-xs',
                  stagedStatus !== undefined && 'border-primary',
                  teamChanged && stagedStatus === undefined && 'border-amber-400',
                )}>
                  <SelectValue placeholder={teamChanged
                    ? 'Select status'
                    : (commonValues.workflow_state_id === MIXED ? 'Multiple' : 'No change')} />
                </SelectTrigger>
                <SelectContent>
                  {(effectiveWorkflow?.states ?? []).map((state) => (
                    <SelectItem key={state.id} value={state.id}>
                      {state.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {teamChanged && stagedStatus === undefined ? (
              <p className="-mt-1 pl-24 text-[10px] text-amber-600 dark:text-amber-400">
                Pick a status for the new team to apply.
              </p>
            ) : null}

            <div className={fieldRow}>
              <span className={cn(fieldLabel, 'self-start pt-1')}>Owners</span>
              <div className="flex flex-1 flex-wrap items-center gap-1">
                {ownerChips.length === 0 ? (
                  <span className="text-xs text-muted-foreground">No owners</span>
                ) : null}
                {ownerChips.map(({ id, state }) => (
                  <OwnerChip
                    key={id}
                    memberId={id}
                    state={state}
                    nameMap={ownerNameMap}
                    member={memberById.get(id)}
                    onClick={state === 'partial' ? () => promoteOwner(id) : undefined}
                    onRemove={() => removeOwnerChip(id, state)}
                  />
                ))}
                <MultiMemberPickerPopover
                  values={[]}
                  members={assignableMembers}
                  disabled={loading}
                  open={addOwnerOpen}
                  onOpenChange={setAddOwnerOpen}
                  onChange={(selected) => {
                    // Picker fires onChange per toggle; treat the last toggled id as the one to add.
                    // Easier model: use the difference between selected and "[]" — but onChange gives us the new array.
                    // We translate each id in `selected` as "add to all tasks".
                    if (selected.length === 0) return;
                    setOwnerAdds((prev) => {
                      const next = new Set(prev);
                      for (const id of selected) next.add(id);
                      return next;
                    });
                    setOwnerRemoves((prev) => {
                      let mutated = false;
                      const next = new Set(prev);
                      for (const id of selected) {
                        if (next.delete(id)) mutated = true;
                      }
                      return mutated ? next : prev;
                    });
                    setAddOwnerOpen(false);
                  }}
                  triggerLabel="Add owner"
                  contentClassName="w-[260px]"
                  renderTrigger={() => (
                    <span className="inline-flex items-center gap-1 rounded-md border border-dashed border-input px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground">
                      <PlusSignIcon className="h-3 w-3" /> Add
                    </span>
                  )}
                />
              </div>
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Priority</span>
              <Select
                size="sm"
                disabled={loading}
                value={stagedPriority
                  ?? (commonValues.priority === MIXED ? undefined : (commonValues.priority as Priority))}
                onValueChange={(value) => setStagedPriority(value as Priority)}
              >
                <SelectTrigger className={cn(
                  'h-7 flex-1 text-xs',
                  stagedPriority !== undefined && 'border-primary',
                )}>
                  <SelectValue placeholder={commonValues.priority === MIXED ? 'Multiple' : 'No change'} />
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
                value={stagedSeverity
                  ?? (commonValues.severity === MIXED ? undefined : (commonValues.severity as Severity))}
                onValueChange={(value) => setStagedSeverity(value as Severity)}
              >
                <SelectTrigger className={cn(
                  'h-7 flex-1 text-xs',
                  stagedSeverity !== undefined && 'border-primary',
                )}>
                  <SelectValue placeholder={commonValues.severity === MIXED ? 'Multiple' : 'No change'} />
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
                value={(() => {
                  if (stagedEpicId !== undefined) return stagedEpicId === null ? '__none__' : stagedEpicId;
                  if (commonValues.epic_id === MIXED) return undefined;
                  return (commonValues.epic_id as string) || '__none__';
                })()}
                onValueChange={(value) => setStagedEpicId(value === '__none__' ? null : value)}
              >
                <SelectTrigger className={cn(
                  'h-7 flex-1 text-xs',
                  stagedEpicId !== undefined && 'border-primary',
                )}>
                  <SelectValue placeholder={commonValues.epic_id === MIXED ? 'Multiple' : 'No change'} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No epic</SelectItem>
                  {groupEpicsByLifecycle(effectiveEpics.map(entry => entry.epic)).map(group => (
                    <SelectGroup key={group.label}>
                      <SelectLabel>{group.label}</SelectLabel>
                      {group.epics.map(epic => (
                        <SelectItem key={epic.id} value={epic.id} textValue={epic.name}>
                          <EpicBadge name={epic.name} color={epic.color} />
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Sprint</span>
              <Select
                size="sm"
                disabled={loading}
                value={(() => {
                  if (stagedSprintId !== undefined) return stagedSprintId === null ? '__none__' : stagedSprintId;
                  if (commonValues.sprint_id === MIXED) return undefined;
                  return (commonValues.sprint_id as string) || '__none__';
                })()}
                onValueChange={(value) => setStagedSprintId(value === '__none__' ? null : value)}
              >
                <SelectTrigger className={cn(
                  'h-7 flex-1 text-xs',
                  stagedSprintId !== undefined && 'border-primary',
                )}>
                  <SelectValue placeholder={commonValues.sprint_id === MIXED ? 'Multiple' : 'No change'} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No sprint</SelectItem>
                  {effectiveSprints.map((entry) => (
                    <SelectItem key={entry.sprint.id} value={entry.sprint.id}>
                      {entry.sprint.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className={fieldRow}>
              <span className={cn(fieldLabel, 'self-start pt-1')}>Labels</span>
              <div className="flex flex-1 flex-wrap items-center gap-1">
                {labelChips.length === 0 ? (
                  <span className="text-xs text-muted-foreground">No labels</span>
                ) : null}
                {labelChips.map(({ id, state }) => {
                  const label = labelById.get(id);
                  if (!label) return null;
                  return (
                    <LabelChip
                      key={id}
                      label={label}
                      state={state}
                      onClick={state === 'partial' ? () => promoteLabel(id) : undefined}
                      onRemove={() => removeLabelChip(id, state)}
                    />
                  );
                })}
                <LabelAdder
                  workspaceId={workspaceId}
                  teamId={effectiveTeamId}
                  labels={effectiveLabels}
                  disabled={loading}
                  excludeIds={labelChips.map((c) => c.id)}
                  onAdd={(id) => {
                    setLabelAdds((prev) => new Set([...prev, id]));
                    setLabelRemoves((prev) => {
                      if (!prev.has(id)) return prev;
                      const next = new Set(prev);
                      next.delete(id);
                      return next;
                    });
                  }}
                />
              </div>
            </div>

            <div className={fieldRow}>
              <span className={fieldLabel}>Deadline</span>
              <Popover open={deadlineOpen} onOpenChange={setDeadlineOpen}>
                <PopoverTrigger asChild>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className={cn(
                      'h-7 flex-1 justify-start px-2 text-xs',
                      stagedDeadline !== undefined && 'border-primary',
                    )}
                    disabled={loading}
                  >
                    <Calendar03Icon className="mr-1 h-3 w-3" />
                    {stagedDeadline !== undefined
                      ? (stagedDeadline === null
                          ? 'Clear deadline'
                          : format(new Date(stagedDeadline), 'MMM d, yyyy'))
                      : commonValues.deadline === MIXED
                        ? <span className="italic text-muted-foreground">Multiple</span>
                        : commonValues.deadline
                          ? format(new Date(commonValues.deadline as string), 'MMM d, yyyy')
                          : 'Set deadline'}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start" onClick={(event) => event.stopPropagation()}>
                  <Calendar
                    mode="single"
                    onSelect={(date) => {
                      if (!date) return;
                      setDeadlineOpen(false);
                      setStagedDeadline(format(date, 'yyyy-MM-dd'));
                    }}
                  />
                </PopoverContent>
              </Popover>
            </div>

            <div className="mt-2 flex items-center gap-2 border-t border-border/60 pt-3">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 px-2 text-xs"
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
                variant="ghost"
                size="sm"
                className="ml-auto h-7 px-2 text-xs text-muted-foreground"
                disabled={loading}
                onClick={() => {
                  resetStaged();
                  onClearSelection();
                  setOpen(false);
                }}
              >
                Cancel
              </Button>
              <Button
                type="button"
                variant="default"
                size="sm"
                className="h-7 px-3 text-xs"
                disabled={loading || teamScopedLoading || !canApply}
                onClick={() => void handleApply()}
              >
                {loading ? <Loading01Icon className="mr-1 h-3 w-3 animate-spin" /> : null}
                Apply
              </Button>
            </div>
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

    </>
  );
}

function OwnerChip({
  memberId,
  state,
  nameMap,
  member,
  onClick,
  onRemove,
}: {
  memberId: string;
  state: ChipState;
  nameMap: Map<string, string>;
  member?: AssignableMember;
  onClick?: () => void;
  onRemove: () => void;
}) {
  const name = member?.display_name || member?.email || nameMap.get(memberId) || 'Owner';
  return (
    <span className="group/owner relative inline-flex">
      <button
        type="button"
        title={state === 'partial' ? `${name} (some tasks) — click to add to all` : name}
        onClick={onClick}
        disabled={!onClick}
        className={cn(
          'inline-flex h-5 w-5 items-center justify-center rounded-full transition-opacity',
          state === 'partial' && 'italic opacity-60 hover:opacity-100',
          state === 'pending-add' && 'ring-2 ring-primary/60 ring-offset-1 ring-offset-popover',
          onClick && 'cursor-pointer',
          !onClick && 'cursor-default',
        )}
      >
        <UserAvatar
          name={name}
          avatarUrl={member?.avatar_url}
          avatarStyle={member?.avatar_style}
          avatarSeed={member?.avatar_seed}
          avatarBackgroundMode={member?.avatar_background_mode}
          avatarBackgroundColor={member?.avatar_background_color}
          className="h-5 w-5"
          fallbackClassName="text-[8px]"
        />
      </button>
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          onRemove();
        }}
        className="absolute -top-1 -right-1 hidden h-3.5 w-3.5 items-center justify-center rounded-full border border-border bg-background text-muted-foreground shadow-sm group-hover/owner:flex hover:text-foreground"
        aria-label={`Remove ${name}`}
      >
        <Cancel01Icon className="h-2.5 w-2.5" />
      </button>
    </span>
  );
}

function LabelChip({
  label,
  state,
  onClick,
  onRemove,
}: {
  label: Label;
  state: ChipState;
  onClick?: () => void;
  onRemove: () => void;
}) {
  const content = (
    <LabelBadge
      label={label}
      onRemove={onRemove}
      className={cn(
        state === 'partial' && 'italic opacity-60',
        state === 'pending-add' && 'ring-2 ring-primary/60',
      )}
    />
  );
  if (!onClick) return content;
  return (
    <span
      role="button"
      tabIndex={0}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onClick();
        }
      }}
      title={`${label.name} (some tasks) — click to add to all`}
      className="inline-flex cursor-pointer rounded-sm"
    >
      {content}
    </span>
  );
}

function LabelAdder({
  workspaceId,
  teamId,
  labels,
  disabled,
  excludeIds,
  onAdd,
}: {
  workspaceId: string;
  teamId?: string;
  labels: Label[];
  disabled: boolean;
  excludeIds: string[];
  onAdd: (id: string) => void;
}) {
  return (
    <LabelPicker
      workspaceId={workspaceId}
      teamId={teamId}
      selectedLabelIds={excludeIds}
      onChange={(next) => {
        // Picker returns the new selected set. Anything in `next` that's not in excludeIds is a new add.
        for (const id of next) {
          if (!excludeIds.includes(id)) onAdd(id);
        }
      }}
      labels={labels}
      triggerOnly
      className={cn(
        'inline-flex h-5 items-center',
        disabled && 'pointer-events-none opacity-50',
      )}
    />
  );
}
