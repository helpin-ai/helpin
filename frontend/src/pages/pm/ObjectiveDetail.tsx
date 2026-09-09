import { EPIC_PICKER_WIDTH } from '@/components/pm/epicPickerGroups';
import { DetailMetadataRow as MetadataRow } from '@/components/pm/DetailMetadataRow';
import { DetailDescriptionEditButton } from '@/components/pm/DetailDescriptionEditButton';
import { DetailDescriptionEditorActions } from '@/components/pm/DetailDescriptionEditorActions';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useObjective } from '@/hooks/queries/useObjectives';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { useObjectiveAutosave } from './useObjectiveAutosave';
import { QuietBreadcrumbs, QuietDetailHeader, QuietDetailLayout, QuietEmptyState, QuietMetricBlock, QuietMetricGrid, QuietPrimaryAction, QuietSectionHeader, QuietStatusText, QuietTextAction, QuietTitleInput, QuietUnderlineInput } from '@/components/design-system/quiet';
import { EpicColorSwatch } from '@/components/pm/EpicColorSwatch';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useNavigate, useBlocker } from '@tanstack/react-router';
import { differenceInDays, format, formatDistanceToNow, parseISO } from 'date-fns';
import { useTitle } from '@/hooks/useTitle';
import {
  Calendar03Icon,
  FavouriteIcon,
  InformationCircleIcon,
  Loading01Icon,
  PlusSignIcon,
  Delete01Icon,
  UserIcon,
  UserGroupIcon,
  Cancel01Icon,
  HashtagIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { Attachments } from '@/components/pm/Attachments';
import { DatePicker } from '@/components/ui/date-picker';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import type {
  AttachmentResponse,
  KeyResult,
  KeyResultType,
  ObjectiveHealth,
  ObjectiveState,
  ObjectiveWithDetails,
  UpdateObjectiveRequest,
  UpdateKeyResultRequest,
} from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/objectives/$objectiveId');

const stateOptions: { value: ObjectiveState; label: string; className: string }[] = (
  Object.entries(OBJECTIVE_STATE_CONFIG) as [ObjectiveState, typeof OBJECTIVE_STATE_CONFIG[ObjectiveState]][]
).map(([value, cfg]) => ({ value, label: cfg.label, className: cfg.color }));

const healthOptions: { value: ObjectiveHealth; label: string; color: string }[] = [
  { value: 'on_track', label: 'On track', color: 'text-green-600' },
  { value: 'at_risk', label: 'At risk', color: 'text-yellow-600' },
  { value: 'off_track', label: 'Off track', color: 'text-red-600' },
];

const OBJECTIVE_MANAGER_TOOLTIP = 'Only team managers can edit objectives. Ask your team manager for access.';

function ManagerOnlyTooltip({
  disabled,
  className,
  children,
}: {
  disabled: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  if (!disabled) return <>{children}</>;
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={`inline-flex max-w-full ${className ?? ''}`}>{children}</span>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-[260px] text-xs">
          {OBJECTIVE_MANAGER_TOOLTIP}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

// ── Multi-value list (teams/owners) ─────────────────────────────────

function MultiValueList({
  items,
  allOptions,
  onAdd,
  onRemove,
  placeholder,
  readOnly,
}: {
  items: string[];
  allOptions: { id: string; name: string }[];
  onAdd: (id: string) => void;
  onRemove: (id: string) => void;
  placeholder: string;
  readOnly?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const available = allOptions.filter((o) => !items.includes(o.id));
  const selected = allOptions.filter((o) => items.includes(o.id));

  return (
    <div className="space-y-1">
      {selected.length === 0 && readOnly && (
        <span className="text-xs text-muted-foreground px-1.5 py-0.5">None</span>
      )}
      {selected.map((item) => (
        <div key={item.id} className="flex items-center justify-between gap-2 py-0.5 text-xs">
          <span className="truncate">{item.name}</span>
          <ManagerOnlyTooltip disabled={!!readOnly}>
            <button
              type="button"
              disabled={readOnly}
              className="text-muted-foreground hover:text-destructive cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:text-muted-foreground"
              onClick={() => onRemove(item.id)}
            >
              <Cancel01Icon className="h-3 w-3" />
            </button>
          </ManagerOnlyTooltip>
        </div>
      ))}
      {readOnly ? (
        <ManagerOnlyTooltip disabled>
          <button
            type="button"
            disabled
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-60"
          >
            <PlusSignIcon className="h-3 w-3" />
            {placeholder}
          </button>
        </ManagerOnlyTooltip>
      ) : (
        <QuietDropdown label={placeholder} open={open} onOpenChange={setOpen} onSelect={onAdd}
          options={available.map(option => ({ value: option.id, label: option.name }))} empty="No more options"
          trigger={<button type="button" className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-ui text-muted-foreground transition-colors hover:bg-accent cursor-pointer">
            <PlusSignIcon className="h-3 w-3" />{placeholder}
          </button>} />
      )}
    </div>
  );
}

// ── Key Result Row ─────────────────────────────────────────────────

function KeyResultRow({
  kr,
  workspaceId,
  memberMap,
  onUpdate,
  onDelete,
  readOnly,
  registerSave,
  onDirtyChange,
}: {
  kr: KeyResult;
  workspaceId: string;
  memberMap: Map<string, string>;
  onUpdate: (updated: KeyResult) => void;
  onDelete: () => void;
  readOnly?: boolean;
  registerSave: (id: string, save: (() => Promise<void>) | null) => void;
  onDirtyChange: (id: string, dirty: boolean) => void;
}) {
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setName] = useState<string | null>(null);
  const [valueDraft, setCurrentValue] = useState<string | null>(null);
  const name = nameDraft ?? kr.name;
  const currentValue = valueDraft ?? String(kr.current_value);
  const [source, setSource] = useState(kr);

  const { queuePatch, flush, saving, dirty, error } = useObjectiveAutosave<UpdateKeyResultRequest>({
    pendingUploads: 0,
    debounceMs: null,
    save: async patch => {
      const { data, error: saveError } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, patch);
      if (saveError || !data) throw new Error(saveError ?? 'Could not save key result');
      onUpdate(data);
    },
  });
  if (source !== kr && !dirty) {
    setSource(kr); setName(null); setCurrentValue(null);
  }
  useEffect(() => {
    registerSave(kr.id, flush);
    return () => registerSave(kr.id, null);
  }, [registerSave, kr.id, flush]);
  useEffect(() => {
    onDirtyChange(kr.id, dirty);
    return () => onDirtyChange(kr.id, false);
  }, [onDirtyChange, kr.id, dirty]);

  const saveValue = () => { void flush().catch(() => undefined); };
  const saveName = async () => {
    try { await flush(); setEditingName(false); } catch { /* Keep the edit and show Retry. */ }
  };

  const lastUpdated = formatDistanceToNow(parseISO(kr.updated_at), { addSuffix: true });
  const updatedByName = kr.updated_by ? memberMap.get(kr.updated_by) : undefined;

  return (
    <div className="group flex flex-wrap items-center gap-3 border-b border-quiet-divider-light py-3">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          {!readOnly && editingName ? (
            <input
              type="text"
              value={name}
              onChange={(e) => { setName(e.target.value); queuePatch({ name: e.target.value.trim() || kr.name }); }}
              onBlur={saveName}
              onKeyDown={(e) => e.key === 'Enter' && saveName()}
              className="w-full bg-transparent text-sm font-medium focus:outline-none"
              autoFocus
            />
          ) : !readOnly ? (
            <button
              type="button"
              className="text-sm font-medium text-foreground hover:underline cursor-pointer text-left truncate"
              onClick={() => setEditingName(true)}
            >
              {kr.name}
            </button>
          ) : (
            <span className="text-sm font-medium text-foreground text-left truncate">{kr.name}</span>
          )}
        </div>
        <div className="mt-0.5 flex items-center gap-1.5">
          <span className="text-[11px] text-quiet-text-tertiary">{kr.result_type}</span>
          {kr.result_type === 'boolean' ? (
            readOnly ? (
              <span className={`rounded px-1 py-0.5 text-xs ${
                kr.progress >= 100
                  ? 'text-quiet-positive'
                  : 'text-quiet-text-tertiary'
              }`}>
                {kr.progress >= 100 ? 'Done' : 'Not done'}
              </span>
            ) : (
              <button
                type="button"
                className={`rounded px-1 py-0.5 text-xs cursor-pointer ${
                  kr.progress >= 100
                    ? 'text-quiet-positive'
                    : 'text-quiet-text-tertiary'
                }`}
                aria-label={`Mark ${kr.name} ${kr.progress >= 100 ? 'not done' : 'done'}`}
                disabled={saving}
                onClick={async () => {
                  const newVal = kr.current_value >= kr.target_value ? 0 : kr.target_value;
                  queuePatch({ current_value: newVal });
                  await flush().catch(() => undefined);
                }}
              >
                {kr.progress >= 100 ? 'Done' : 'Not done'}
              </button>
            )
          ) : (
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span>{kr.initial_value}</span>
              <span>→</span>
              {readOnly ? (
                <span className="w-14 text-center font-medium text-foreground">{kr.current_value}</span>
              ) : (
                <QuickTooltip label="Current value — edit to update progress">
                  <input
                    type="number"
                    value={currentValue}
                    aria-label={`Current value for ${kr.name}`}
                    onChange={(e) => {
                      setCurrentValue(e.target.value);
                      const value = Number(e.target.value);
                      queuePatch({ current_value: e.target.value.trim() && Number.isFinite(value) ? value : kr.current_value });
                    }}
                    onBlur={saveValue}
                    onKeyDown={(e) => e.key === 'Enter' && saveValue()}
                    className="w-14 border-0 border-b border-quiet-field bg-transparent px-1 py-0.5 text-xs text-center font-medium text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-quiet-text-primary"
                  />
                </QuickTooltip>
              )}
              <span>→ {kr.target_value}</span>
            </div>
          )}
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground tabular-nums">{Math.round(kr.progress)}%</span>
          <div className="w-24">
            <Progress value={kr.progress} className="h-1.5 bg-quiet-divider-light [&>[data-slot=progress-indicator]]:bg-quiet-positive" />
          </div>
          {!readOnly && (
            <button
              type="button"
              className="text-quiet-text-tertiary sm:opacity-0 sm:group-hover:opacity-100 focus-visible:opacity-100 hover:text-destructive cursor-pointer transition-opacity"
              onClick={onDelete}
              aria-label={`Delete key result ${kr.name}`}
            >
              <Delete01Icon className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        {error && <div className="flex items-center gap-2 text-xs text-destructive" role="alert">{error}<QuietTextAction onClick={() => void flush().catch(() => undefined)}>Retry</QuietTextAction></div>}
        <span className={`text-[11px] text-muted-foreground ${readOnly ? '' : 'pr-6'}`}>
          {updatedByName ? `${updatedByName}, ${lastUpdated}` : `Updated ${lastUpdated}`}
        </span>
      </div>
    </div>
  );
}

// ── Link Epic Popover ──────────────────────────────────────────────

function LinkEpicPopover({
  workspaceId,
  linkedEpicIds,
  onLink,
  disabled = false,
}: {
  workspaceId: string;
  linkedEpicIds: string[];
  onLink: (epicId: string) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const { data: allEpics = [], isLoading: loading, isError: loadError } = useQuery({
    queryKey: [...queryKeys.pm.epics(workspaceId), { archived: false }],
    queryFn: async () => unwrap(await pmEpicService.list(workspaceId, { archived: false })),
    enabled: open,
  });

  const available = allEpics.filter((e) => !linkedEpicIds.includes(e.epic.id));

  return (
    <QuietDropdown label="Epics" open={open} onOpenChange={setOpen} disabled={disabled}
      loading={loading} error={loadError ? 'Could not load epics. Close and reopen to retry.' : undefined}
      empty="No available epics" onSelect={onLink} contentClassName={EPIC_PICKER_WIDTH}
      options={available.map(({ epic }) => ({ value: epic.id, label: epic.name, leading: <EpicColorSwatch color={epic.color} /> }))}
      trigger={<QuietTextAction disabled={disabled}>
        <PlusSignIcon className="mr-1 h-3 w-3" />Add Epics
      </QuietTextAction>} />
  );
}

// ── Main Page ──────────────────────────────────────────────────────

interface FormState {
  name: string;
  description: string;
  objective_type: string;
  state: ObjectiveState;
  health: ObjectiveHealth;
  planned_start_date: string;
  deadline: string;
}

const buildForm = (obj: ObjectiveWithDetails): FormState => ({
  name: obj.objective.name,
  description: obj.objective.description ?? '',
  objective_type: obj.objective.objective_type,
  state: obj.objective.state,
  health: obj.objective.health,
  planned_start_date: obj.objective.planned_start_date?.slice(0, 10) ?? '',
  deadline: obj.objective.deadline?.slice(0, 10) ?? '',
});

export function ObjectiveDetailPage() {
  const { objectiveId, slug } = routeApi.useParams();
  const confirm = useConfirm();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit, isAdmin, canManageTeam } = usePermissions(access);

  const queryClient = useQueryClient();
  const objectiveQuery = useObjective(workspaceId ?? '', objectiveId);
  const { data, isLoading: loading, error } = objectiveQuery;
  const setData = useCallback((next: ObjectiveWithDetails | ((previous: ObjectiveWithDetails | undefined) => ObjectiveWithDetails | undefined)) => {
    queryClient.setQueryData<ObjectiveWithDetails>(queryKeys.pm.objective(workspaceId ?? '', objectiveId), next);
  }, [queryClient, workspaceId, objectiveId]);
  const refreshObjectives = useCallback(() => queryClient.invalidateQueries({ queryKey: queryKeys.pm.objectives(workspaceId ?? ''), refetchType: 'all' }), [queryClient, workspaceId]);

  const [form, setForm] = useState<FormState | null>(null);
  const [formSource, setFormSource] = useState<ObjectiveWithDetails>();
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const savedDescriptionRef = useRef('');

  const { teams } = useAccessibleTeams(workspaceId ?? '');
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const memberMap = useMemo(() => {
    return buildAssignableMemberNameMap(assignableMembers);
  }, [assignableMembers]);
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, data?.teams ?? []),
    [teams, data?.teams],
  );

  useTitle(form?.name ? `${form.name} — Objective` : 'Objective');

  // ── Key result modal state
  const [krModalOpen, setKrModalOpen] = useState(false);
  const [newKrName, setNewKrName] = useState('');
  const [newKrType, setNewKrType] = useState<KeyResultType>('percent');
  const [newKrStart, setNewKrStart] = useState('0');
  const [newKrTarget, setNewKrTarget] = useState('100');
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const descriptionEditStartRef = useRef('');
  const removedDescriptionAttachments = useRef(new Set<string>());

  const keyResultSaves = useRef(new Map<string, () => Promise<void>>());
  const [dirtyKeyResults, setDirtyKeyResults] = useState<Set<string>>(() => new Set());
  const registerKeyResultSave = useCallback((id: string, save: (() => Promise<void>) | null) => {
    if (save) keyResultSaves.current.set(id, save);
    else keyResultSaves.current.delete(id);
  }, []);
  const onKeyResultDirtyChange = useCallback((id: string, isDirty: boolean) => {
    setDirtyKeyResults(current => {
      if (current.has(id) === isDirty) return current;
      const next = new Set(current);
      if (isDirty) next.add(id); else next.delete(id);
      return next;
    });
  }, []);

  const cleanupDescriptionAttachments = useCallback(async (description: string) => {
    if (!workspaceId) return;
    const retained = new Set(extractInlineAttachmentIds(description));
    const removed = [...removedDescriptionAttachments.current].filter(id => !retained.has(id));
    await Promise.allSettled(removed.map(async id => {
      const result = await pmAttachmentService.remove(workspaceId, id);
      if (!result.error) removedDescriptionAttachments.current.delete(id);
    }));
    for (const id of retained) removedDescriptionAttachments.current.delete(id);
  }, [workspaceId]);

  const { queuePatch, flush, saving, error: saveError, dirty } = useObjectiveAutosave({
    pendingUploads: descriptionPendingUploads,
    save: async (patch) => {
      if (!workspaceId) throw new Error('Workspace unavailable');
      const previousDescription = savedDescriptionRef.current;
      const { data: updated, error: updateError } = await pmObjectiveService.update(workspaceId, objectiveId, patch);
      if (updateError || !updated) throw new Error(updateError ?? 'Failed to save objective');
      setData(updated);
      const nextDescription = updated.objective.description ?? '';
      savedDescriptionRef.current = nextDescription;
      if (patch.description !== undefined) {
        for (const id of diffRemovedInlineAttachmentIds(previousDescription, nextDescription)) removedDescriptionAttachments.current.add(id);
        // Cancel can restore the description that was present when editing began.
        if (!editingDescription) await cleanupDescriptionAttachments(nextDescription);
      }
      await refreshObjectives();
    },
  });

  // Adopt remote changes only when the local draft is clean.
  if (data && !dirty && formSource !== data) {
    setFormSource(data);
    setForm(buildForm(data));
  }
  useEffect(() => {
    if (data && !dirty) savedDescriptionRef.current = data.objective.description ?? '';
  }, [data, dirty]);

  const flushAll = async () => {
    await flush();
    await Promise.all([...keyResultSaves.current.values()].map(save => save()));
    await cleanupDescriptionAttachments(form?.description ?? savedDescriptionRef.current);
  };
  useBlocker({
    shouldBlockFn: async () => {
      if (!dirty && dirtyKeyResults.size === 0 && !editingDescription) return false;
      try { await flushAll(); return false; } catch { return true; }
    },
    enableBeforeUnload: dirty || dirtyKeyResults.size > 0,
  });


  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateObjectiveRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  const beginDescriptionEditing = () => {
    if (!form || !canEdit) return;
    descriptionEditStartRef.current = form.description;
    setEditingDescription(true);
  };
  const finishDescriptionEditing = async () => {
    try {
      await flush();
      await cleanupDescriptionAttachments(form?.description ?? savedDescriptionRef.current);
      setEditingDescription(false);
    } catch { /* Keep the editor open so the failed save can be retried. */ }
  };
  const cancelDescriptionEditing = () => {
    if (!form) return;
    const initialDescription = descriptionEditStartRef.current;
    if (form.description !== initialDescription) updateField('description', initialDescription, { description: initialDescription });
    setEditingDescription(false);
  };

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: AttachmentResponse) => {
      if (!workspaceId || !data || !form) {
        return 'fallback' as const;
      }
      if (!extractInlineAttachmentIds(form.description).includes(entry.attachment.id)) {
        return 'fallback' as const;
      }
      const ok = await confirm({
        title: 'Delete image?',
        description: 'This will remove the image from the description and attachments.',
        confirmText: 'Delete',
        variant: 'destructive',
      });
      if (!ok) {
        return 'prevent' as const;
      }

      const nextDescription = removeInlineImagesByAttachmentIds(form.description, [entry.attachment.id]);
      setForm(current => current ? { ...current, description: nextDescription } : current);
      queuePatch({ description: nextDescription });
      try {
        await flush();
        return 'handled' as const;
      } catch {
        return 'prevent' as const;
      }
    },
    [workspaceId, data, form, confirm, queuePatch, flush],
  );

  const runMutation = async <T,>(action: () => Promise<{ data: T | null; error: string | null }>) => {
    try {
      await flush();
      const result = await action();
      if (result.error) throw new Error(result.error);
      await refreshObjectives();
      return { ok: true, data: result.data };
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update objective');
      return { ok: false, data: null };
    }
  };

  const handleCreateKeyResult = async () => {
    if (!workspaceId || !data || !newKrName.trim()) return;
    const startVal = Number(newKrStart);
    const targetVal = newKrType === 'boolean' ? 1 : Number(newKrTarget);
    if (!Number.isFinite(startVal) || !Number.isFinite(targetVal)) {
      toast.error('Enter valid initial and target values');
      return;
    }
    const result = await runMutation(() => pmObjectiveService.createKeyResult(workspaceId, objectiveId, {
      name: newKrName.trim(), result_type: newKrType,
      initial_value: startVal, current_value: startVal, target_value: targetVal,
    }));
    if (result.ok) {
      setNewKrName(''); setNewKrType('percent'); setNewKrStart('0'); setNewKrTarget('100'); setKrModalOpen(false);
    }
  };

  const handleUpdateKeyResult = (updated: KeyResult) => {
    setData(previous => previous ? { ...previous, key_results: previous.key_results.map(kr => kr.id === updated.id ? updated : kr) } : previous);
    void refreshObjectives();
  };
  const handleDeleteKeyResult = async (id: string) => {
    if (workspaceId) await runMutation(() => pmObjectiveService.deleteKeyResult(workspaceId, id));
  };
  const handleAddTeam = async (teamId: string) => {
    if (workspaceId) await runMutation(() => pmObjectiveService.addTeam(workspaceId, objectiveId, teamId));
  };
  const handleRemoveTeam = async (teamId: string) => {
    if (workspaceId) await runMutation(() => pmObjectiveService.removeTeam(workspaceId, objectiveId, teamId));
  };
  const handleOwnerSelectionChange = async (nextOwnerIds: string[]) => {
    if (!workspaceId) return;
    await runMutation(() => pmObjectiveService.update(workspaceId, objectiveId, { owner_member_ids: nextOwnerIds }));
  };
  const handleLinkEpic = async (epicId: string) => {
    if (workspaceId) await runMutation(() => pmObjectiveService.addEpic(workspaceId, objectiveId, epicId));
  };
  const handleUnlinkEpic = async (epicId: string) => {
    if (workspaceId) await runMutation(() => pmObjectiveService.removeEpic(workspaceId, objectiveId, epicId));
  };

  // Derived
  const isStrategic = form?.objective_type === 'strategic';
  const currentState = useMemo(
    () => stateOptions.find((s) => s.value === form?.state) ?? stateOptions[0],
    [form?.state],
  );
  const currentHealth = useMemo(
    () => healthOptions.find((h) => h.value === form?.health) ?? healthOptions[0],
    [form?.health],
  );
  const suggestedHealth = data?.suggested_health;
  const suggestedLabel = healthOptions.find((h) => h.value === suggestedHealth);

  const goBack = () => navigate({ to: '/w/$slug/pm/objectives', params: { slug } });

  if (loading || (!form && data)) {
    return <div className="flex h-full flex-col"><QuietDetailHeader title="Objective" breadcrumbs={<QuietBreadcrumbs items={[{ id: 'objectives', label: 'Objectives', onClick: goBack }]} onBack={goBack} backLabel="Back to objectives" />} /><div role="status" className="flex items-center gap-2 px-6 py-8 text-sm text-quiet-text-tertiary"><Loading01Icon className="h-4 w-4 animate-spin" />Loading objective…</div></div>;
  }
  if (error || !data || !form) {
    return <div className="flex h-full flex-col"><QuietDetailHeader title="Objective" breadcrumbs={<QuietBreadcrumbs items={[{ id: 'objectives', label: 'Objectives', onClick: goBack }]} onBack={goBack} backLabel="Back to objectives" />} /><QuietEmptyState title="Couldn’t load objective" description={error?.message ?? 'This objective is unavailable.'} action={<QuietTextAction onClick={() => void objectiveQuery.refetch()}>Retry</QuietTextAction>} /></div>;
  }

  const krAvgProgress = data.key_results.length > 0
    ? Math.round(data.key_results.reduce((sum, kr) => sum + kr.progress, 0) / data.key_results.length)
    : 0;

  const epicProgress = data.stats.epic_task_count > 0
    ? Math.round((data.stats.epic_done_tasks / data.stats.epic_task_count) * 100)
    : 0;
  const deadlineProgress = form.deadline ? (() => {
    const start = parseISO(form.planned_start_date || data.objective.created_at);
    const totalDays = Math.max(differenceInDays(parseISO(form.deadline), start), 1);
    return Math.min(Math.max(Math.round(differenceInDays(new Date(), start) / totalDays * 100), 0), 100);
  })() : 0;
  const ownerIds = data.owner_member_ids ?? data.owners;
  const canManageObjective = canEdit && (isAdmin || data.teams.some((teamId) => canManageTeam(teamId)));

  return (
    <div className="flex h-full flex-col">
      <QuietDetailHeader
        className="lg:px-10"
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'objectives', label: 'Objectives', onClick: goBack }]} onBack={goBack} backLabel="Back to objectives" />}
        title={<div className="flex min-w-0 items-center gap-0.5">{canEdit ? <ManagerOnlyTooltip disabled={!canManageObjective} className="w-full">
          <QuietTitleInput presentation="header" aria-label="Objective title" value={form.name}
            className="max-w-[42rem] border-b-transparent hover:border-quiet-field focus-visible:border-quiet-text-primary"
            onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); event.currentTarget.blur(); } }}
            disabled={!canManageObjective} placeholder="Untitled"
            onChange={event => updateField('name', event.target.value, { name: event.target.value })} />
        </ManagerOnlyTooltip> : <h1>{form.name}</h1>}</div>}
        meta={<span className="text-xs text-quiet-text-tertiary">{isStrategic ? 'Strategic' : 'Tactical'} objective</span>}
        status={<QuietStatusText className={currentState.className}>{currentState.label}</QuietStatusText>}
        actions={<FollowButton entityType="objective" entityId={data.objective.id} presentation="detail-header" />}
        state={<div className="flex items-center gap-2"><SaveIndicator saving={saving || (dirty && !saveError)} error={saveError} presentation="quiet" />
          {saveError && <QuietTextAction onClick={() => void flush().catch(() => undefined)}>Retry</QuietTextAction>}</div>}
      />

      {/* ── Two-column layout ───────────────────────────────────── */}
      <QuietDetailLayout className="flex flex-col overflow-y-auto lg:grid lg:overflow-hidden [&>div]:shrink-0 [&>div]:overflow-visible lg:[&>div]:overflow-hidden"
        railClassName="shrink-0 px-5 py-5 pb-40"
        main={(
        <div className="min-h-0 px-4 py-5 sm:px-6 lg:h-full lg:overflow-y-auto lg:px-10">
          <section aria-label="Description" className="group/desc relative rounded-lg pb-3">
            {editingDescription ? (
              <div className="group/description-editor">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => updateField('description', html, { description: html })}
                  placeholder="Add a description..."
                  variant="divider"
                  contentVariant="pm"
                  className="min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0"
                  uploadConfig={{ workspaceId: workspaceId!, entityType: 'editor_upload', entityId: workspaceId! }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  teams={mentionTeams}
                  members={assignableMembers}
                />
                <DetailDescriptionEditorActions onCancel={cancelDescriptionEditing} onDone={() => void finishDescriptionEditing()} />
              </div>
            ) : (
              <div className={`relative min-h-9 ${canManageObjective ? 'pr-12' : ''}`}>
                {form.description ? <RichTextMentionContent html={form.description} members={assignableMembers} teams={mentionTeams} variant="pm" />
                  : <p className="text-sm text-muted-foreground">{canManageObjective ? 'No description yet' : 'No description'}</p>}
                {canManageObjective && <DetailDescriptionEditButton onClick={beginDescriptionEditing} />}
              </div>
            )}
          </section>

          <div className="mt-6">
            <Attachments
              workspaceId={workspaceId!}
              entityType="objective"
              entityId={data.objective.id}
              memberNameMap={memberMap}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              editable={canManageObjective}
            />
          </div>

          <section className="mt-6" aria-label="Progress summary">
            <QuietSectionHeader title="Progress" />
            <QuietMetricGrid className="mt-2 md:grid-cols-3 xl:grid-cols-3">
              <QuietMetricBlock label="Epic progress" value={`${epicProgress}%`} tone="positive" className="items-start [&>span:first-child]:w-full"
                description={<><Progress value={epicProgress} aria-valuenow={epicProgress} aria-label="Epic progress" className="my-3 h-1.5 bg-quiet-divider-light" indicatorClassName="bg-quiet-positive" /><span>{data.stats.epic_done_tasks} of {data.stats.epic_task_count} linked tasks completed</span></>} />
              <QuietMetricBlock label="Key results" className="items-start" value={data.key_results.length ? `${krAvgProgress}%` : '—'}
                description={data.key_results.length ? `Average across ${data.key_results.length} key results` : 'No key results yet'} />
              <QuietMetricBlock label="Target date" value={form.deadline ? format(parseISO(form.deadline), 'MMM d, yyyy') : 'Not set'} className="items-start [&>span:first-child]:w-full"
                description={form.deadline ? <><Progress value={deadlineProgress} aria-valuenow={deadlineProgress} aria-label="Time elapsed toward target date" className="my-3 h-1.5 bg-quiet-divider-light" indicatorClassName="bg-sky-500" /><span>{(() => {
                  const days = differenceInDays(parseISO(form.deadline), new Date());
                  return form.state === 'closed' ? 'Completed' : days > 0 ? `${days} days remaining` : days === 0 ? 'Due today' : `${Math.abs(days)} days overdue`;
                })()}</span></> : 'Set a target date in properties'} />
            </QuietMetricGrid>
            {data.key_results.length > 0 && Math.abs(epicProgress - krAvgProgress) >= 20 && (
              <p className="mt-3 text-xs text-quiet-text-tertiary">
                {epicProgress > krAvgProgress
                  ? `${epicProgress}% of work is done and ${krAvgProgress}% of outcomes achieved.`
                  : `${krAvgProgress}% of outcomes achieved with ${epicProgress}% of work done.`}
                {' '}Based on linked task completion and average key-result progress.
              </p>
            )}
          </section>

          {/* ── Key Results ─────────────────────────────────────── */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <QuietSectionHeader title="Key Results" count={data.key_results.length} />
              <div className="flex items-center gap-2">
                {data.key_results.length > 0 && (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="text-xs text-muted-foreground cursor-help border-b border-dotted border-muted-foreground/40">{krAvgProgress}% outcome progress</span>
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[240px] text-xs">
                        Average progress across all key results. Each key result's progress is: (current − initial) ÷ (target − initial).
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                )}
                {canEdit && (
                  <ManagerOnlyTooltip disabled={!canManageObjective}>
                    <QuietTextAction
                      disabled={!canManageObjective}
                      onClick={() => setKrModalOpen(true)}
                    >
                      <PlusSignIcon className="mr-1 h-3 w-3" />
                      Add Key Results
                    </QuietTextAction>
                  </ManagerOnlyTooltip>
                )}
              </div>
            </div>
            {data.key_results.length > 0 ? (
              <div className="space-y-2">
                {data.key_results.map((kr) => (
                  <KeyResultRow
                    key={kr.id}
                    kr={kr}
                    workspaceId={workspaceId!}
                    memberMap={memberMap}
                    onUpdate={handleUpdateKeyResult}
                    onDelete={() => handleDeleteKeyResult(kr.id)}
                    registerSave={registerKeyResultSave}
                    onDirtyChange={onKeyResultDirtyChange}
                    readOnly={!canManageObjective}
                  />
                ))}
              </div>
            ) : (
              <div className="border-y border-quiet-divider-light py-5 text-left">
                <p className="text-sm text-muted-foreground">No key results yet</p>
                <p className="mt-1 text-xs text-muted-foreground/60">Add key results to track outcome progress</p>
              </div>
            )}
          </div>

          {/* ── Epics ───────────────────────────────────────────── */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <QuietSectionHeader title="Linked Epics" count={data.epics.length} />
              {canEdit && (
                <ManagerOnlyTooltip disabled={!canManageObjective}>
                  <span>
                    <LinkEpicPopover
                      workspaceId={workspaceId!}
                      linkedEpicIds={data.epics.map((e) => e.epic.id)}
                      onLink={handleLinkEpic}
                      disabled={!canManageObjective}
                    />
                  </span>
                </ManagerOnlyTooltip>
              )}
            </div>

            {data.epics.length > 0 ? (
              <div className="space-y-2">
                {data.epics.map((e) => {
                  const totalTasks = getEpicTaskCount(e.stats);
                  const pct = totalTasks > 0
                    ? Math.round((getEpicDoneTaskCount(e.stats) / totalTasks) * 100)
                    : 0;
                  const epicState = e.epic.completed ? 'Done' : e.epic.started ? 'In Progress' : 'Not Started';
                  const epicStateColor = e.epic.completed ? 'text-quiet-positive' : e.epic.started ? 'text-quiet-accent' : 'text-quiet-muted';
                  const epicUpdated = formatDistanceToNow(parseISO(e.epic.updated_at), { addSuffix: true });
                  return (
                    <div
                      key={e.epic.id}
                      role="button"
                      tabIndex={0}
                      className="group flex w-full flex-wrap items-center gap-3 border-b border-quiet-divider-light py-3 text-left transition-colors hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-quiet-text-primary cursor-pointer"
                      onClick={() => navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: e.epic.id } })}
                      onKeyDown={(event) => {
                        if (event.target === event.currentTarget && (event.key === 'Enter' || event.key === ' ')) {
                          event.preventDefault();
                          void navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: e.epic.id } });
                        }
                      }}
                    >
                      <EpicColorSwatch color={e.epic.color} />
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium truncate" title={e.epic.name}>{e.epic.name}</p>
                        <div className="mt-0.5 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                          <span className={epicStateColor}>{epicState.toLowerCase()}</span>
                        </div>
                      </div>
                      <div className="flex shrink-0 flex-col items-end gap-1">
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground tabular-nums">{pct}%</span>
                          <div className="w-24">
                            <Progress value={pct} className="h-1.5 bg-quiet-divider-light [&>[data-slot=progress-indicator]]:bg-quiet-positive" />
                          </div>
                          {canEdit && (
                            <ManagerOnlyTooltip disabled={!canManageObjective}>
                              <button
                                type="button"
                                disabled={!canManageObjective}
                                className="text-quiet-text-tertiary sm:opacity-0 sm:group-hover:opacity-100 focus-visible:opacity-100 hover:text-destructive cursor-pointer transition-opacity disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:text-muted-foreground"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void handleUnlinkEpic(e.epic.id);
                                }}
                              >
                                <Cancel01Icon className="h-3.5 w-3.5" />
                              </button>
                            </ManagerOnlyTooltip>
                          )}
                        </div>
                        <span className={`text-[11px] text-muted-foreground ${canEdit ? 'pr-6' : ''}`}>Updated {epicUpdated}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="border-y border-quiet-divider-light py-5 text-left">
                <p className="text-sm text-muted-foreground">No epics linked yet</p>
              </div>
            )}
          </div>
          <div className="h-20 shrink-0 lg:h-40" aria-hidden="true" />
        </div>
        )}
        rail={(
        <>
          <div className="mt-5 grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* State */}
            <MetadataRow icon={HashtagIcon} label="State">
              {canEdit ? (
                <ManagerOnlyTooltip disabled={!canManageObjective}>
                  <SidebarPopoverSelect
                    value={form.state}
                    options={stateOptions}
                    onChange={(v) => updateField('state', v as ObjectiveState, { state: v as ObjectiveState })}
                    renderTrigger={() => <span className={currentState.className}>{currentState.label}</span>}
                    disabled={!canManageObjective}
                  />
                </ManagerOnlyTooltip>
              ) : (
                <span className={`text-xs ${currentState.className}`}>{currentState.label}</span>
              )}
            </MetadataRow>

            {/* Health */}
            {form.state !== 'closed' && (
            <MetadataRow icon={FavouriteIcon} label="Health">
              <div className="flex flex-col gap-1">
                {canEdit ? (
                  <ManagerOnlyTooltip disabled={!canManageObjective}>
                    <SidebarPopoverSelect
                      value={form.health}
                      options={healthOptions.map((h) => ({ value: h.value, label: h.label, className: h.color }))}
                      onChange={(v) => updateField('health', v as ObjectiveHealth, { health: v as ObjectiveHealth })}
                      renderTrigger={() => (
                        <span className={currentHealth.color}>{currentHealth.label}</span>
                      )}
                      disabled={!canManageObjective}
                    />
                  </ManagerOnlyTooltip>
                ) : (
                  <span className={`text-xs px-1.5 py-0.5 ${currentHealth.color}`}>{currentHealth.label}</span>
                )}
                {canEdit && suggestedLabel && suggestedHealth !== form.health && (
                  <ManagerOnlyTooltip disabled={!canManageObjective}>
                    <button
                      type="button"
                      disabled={!canManageObjective}
                      className="text-left text-[10px] text-muted-foreground transition-colors hover:text-foreground cursor-pointer disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:text-muted-foreground"
                      onClick={() => updateField('health', suggestedHealth!, { health: suggestedHealth })}
                    >
                      Suggested: <span className={suggestedLabel.color}>{suggestedLabel.label}</span>
                    </button>
                  </ManagerOnlyTooltip>
                )}
              </div>
            </MetadataRow>
            )}

            {/* ── People ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Teams */}
            <MetadataRow icon={UserGroupIcon} label="Teams">
              <MultiValueList
                items={data.teams}
                allOptions={teams.map((t) => ({ id: t.id, name: t.name }))}
                onAdd={handleAddTeam}
                onRemove={handleRemoveTeam}
                placeholder="Add team"
                readOnly={!canManageObjective}
              />
            </MetadataRow>

            {/* Owners */}
            <MetadataRow icon={UserIcon} label="Owners">
              <ManagerOnlyTooltip disabled={!canManageObjective}>
                <MultiMemberPickerPopover
                  values={ownerIds}
                  members={assignableMembers}
                  disabled={!canManageObjective}
                  onChange={(nextOwnerIds) => {
                    void handleOwnerSelectionChange(nextOwnerIds);
                  }}
                  renderTrigger={() => {
                    const selectedMembers = assignableMembers.filter((member) => ownerIds.includes(member.id));
                    if (selectedMembers.length === 0) {
                      return <span className="text-muted-foreground">{canManageObjective ? 'Add owners' : 'None'}</span>;
                    }

                    const label = selectedMembers
                      .map((member) => member.display_name || member.email)
                      .join(', ');

                    return (
                      <>
                        <div className="flex items-center -space-x-1">
                          {selectedMembers.slice(0, 2).map((member) => (
                            <UserAvatar
                              key={member.id}
                              name={member.display_name || member.email}
                              avatarUrl={member.avatar_url}
                              avatarStyle={member.avatar_style}
                              avatarSeed={member.avatar_seed}
                              avatarBackgroundMode={member.avatar_background_mode}
                              avatarBackgroundColor={member.avatar_background_color}
                              className="h-4 w-4"
                              fallbackClassName="text-[7px]"
                            />
                          ))}
                        </div>
                        <span className="truncate">{label}</span>
                      </>
                    );
                  }}
                  triggerClassName="inline-flex max-w-full items-center gap-1.5 overflow-hidden rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
                  contentClassName="w-[260px]"
                />
              </ManagerOnlyTooltip>
            </MetadataRow>

            {/* ── Planning ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Start Date */}
            <MetadataRow icon={Calendar03Icon} label="Start date">
              {canEdit && canManageObjective ? (
                <DatePicker
                  value={form.planned_start_date}
                  onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: form.deadline,
                    onChange: (v) => updateField('deadline', v, { deadline: v || undefined }),
                    placeholder: 'None',
                  }}
                  placeholder="None"
                  hideIcon
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : canEdit ? (
                <ManagerOnlyTooltip disabled>
                  <span className="text-xs px-1.5 py-0.5">
                    {form.planned_start_date ? format(parseISO(form.planned_start_date), 'MMM d, yyyy') : 'None'}
                  </span>
                </ManagerOnlyTooltip>
              ) : (
                <span className="text-xs px-1.5 py-0.5">
                  {form.planned_start_date ? format(parseISO(form.planned_start_date), 'MMM d, yyyy') : 'None'}
                </span>
              )}
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={Calendar03Icon} label="Target date">
              {canEdit && canManageObjective ? (
                <DatePicker
                  value={form.planned_start_date}
                  onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: form.deadline,
                    onChange: (v) => updateField('deadline', v, { deadline: v || undefined }),
                    placeholder: 'None',
                  }}
                  triggerField="linked"
                  defaultActiveField="linked"
                  placeholder="None"
                  hideIcon
                  urgencyColor
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : canEdit ? (
                <ManagerOnlyTooltip disabled>
                  <span className="text-xs px-1.5 py-0.5">
                    {form.deadline ? format(parseISO(form.deadline), 'MMM d, yyyy') : 'None'}
                  </span>
                </ManagerOnlyTooltip>
              ) : (
                <span className="text-xs px-1.5 py-0.5">
                  {form.deadline ? format(parseISO(form.deadline), 'MMM d, yyyy') : 'None'}
                </span>
              )}
            </MetadataRow>

            {/* ── Classification ── */}
            {data.labels && data.labels.length > 0 && (
              <>
                <div className="col-span-3 h-px bg-border/40 my-1" />
                <MetadataRow icon={InformationCircleIcon} label="Labels">
                  <div className="flex flex-wrap gap-1">
                    {data.labels.map((l) => (
                      <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                        {l.name}
                      </span>
                    ))}
                  </div>
                </MetadataRow>
              </>
            )}
          </div>

          {/* Delete objective — admin only */}
          {isAdmin && (
            <div className="mt-8 border-t border-border/40 pt-4">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs text-destructive hover:text-destructive hover:bg-destructive/10"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                <Delete01Icon className="mr-1 h-3 w-3" />
                Delete objective
              </Button>
            </div>
          )}
        </>
        )}
      />

      {/* ── Add Key Result Modal ─────────────────────────────────── */}
      <Dialog open={krModalOpen} onOpenChange={setKrModalOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Add Key Result</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <label htmlFor="key-result-name" className="text-sm font-medium text-quiet-text-secondary">Name</label>
              <QuietUnderlineInput
                id="key-result-name"
                type="text"
                value={newKrName}
                onChange={(e) => setNewKrName(e.target.value)}
                placeholder="e.g., Increase activation rate"
                className="mt-1"
                autoFocus
              />
            </div>
            <div className={`grid gap-3 ${newKrType === 'boolean' ? 'grid-cols-1' : 'grid-cols-3'}`}>
              <div>
                <div className="flex items-center gap-1">
                  <label htmlFor="key-result-type" className="text-sm font-medium text-quiet-text-secondary">Measure as</label>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <InformationCircleIcon className="h-3 w-3 text-muted-foreground/60 cursor-help" />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[220px] text-xs">
                        <p className="font-medium mb-1">Measurement types:</p>
                        <p><strong>Boolean</strong> — Done / Not done</p>
                        <p><strong>Percent</strong> — 0–100%</p>
                        <p><strong>Numeric</strong> — Custom range (e.g. 0→50 users)</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <Select value={newKrType} onValueChange={(v) => {
                  setNewKrType(v as KeyResultType);
                  if (v === 'boolean') { setNewKrStart('0'); setNewKrTarget('1'); }
                  else if (v === 'percent') { setNewKrStart('0'); setNewKrTarget('100'); }
                }}>
                  <SelectTrigger id="key-result-type" variant="underline" className="mt-1 w-full px-0.5">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="percent">Percent</SelectItem>
                    <SelectItem value="numeric">Numeric</SelectItem>
                    <SelectItem value="boolean">Boolean</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {newKrType !== 'boolean' && (
                <>
                  <div>
                    <label htmlFor="key-result-start" className="text-sm font-medium text-quiet-text-secondary">Starting value</label>
                    <div className="relative mt-1">
                      <QuietUnderlineInput
                        id="key-result-start"
                        type="number"
                        value={newKrStart}
                        onChange={(e) => setNewKrStart(e.target.value)}
                        className={newKrType === 'percent' ? 'pr-7' : undefined}
                      />
                      {newKrType === 'percent' && (
                        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground pointer-events-none">%</span>
                      )}
                    </div>
                  </div>
                  <div>
                    <label htmlFor="key-result-target" className="text-sm font-medium text-quiet-text-secondary">Target value</label>
                    <div className="relative mt-1">
                      <QuietUnderlineInput
                        id="key-result-target"
                        type="number"
                        value={newKrTarget}
                        onChange={(e) => setNewKrTarget(e.target.value)}
                        className={newKrType === 'percent' ? 'pr-7' : undefined}
                      />
                      {newKrType === 'percent' && (
                        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground pointer-events-none">%</span>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>
          <DialogFooter>
            <QuietTextAction onClick={() => setKrModalOpen(false)}>Cancel</QuietTextAction>
            <QuietPrimaryAction onClick={handleCreateKeyResult} disabled={!canManageObjective || !newKrName.trim()}>
              Add Key Result
            </QuietPrimaryAction>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete objective"
        description="This objective and all its key results will be archived. This action cannot be easily undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={async () => {
          if (!workspaceId || !data) return;
          const result = await runMutation(() => pmObjectiveService.remove(workspaceId, data.objective.id));
          if (result.ok) void goBack();
        }}
      />
    </div>
  );
}
