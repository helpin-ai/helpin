import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type SetStateAction } from 'react';
import { getRouteApi, useLocation, useNavigate } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useTitle } from '@/hooks/useTitle';
import {
  Activity01Icon,
  ArchiveIcon,
  ArrowLeft02Icon,
  Calendar03Icon,
  AttachmentIcon,
  CheckListIcon,
  FavouriteIcon,
  Link01Icon,
  Loading01Icon,
  PlusSignIcon,
  SourceCodeIcon,
  Target01Icon,
  UserIcon,
  UserGroupIcon,
  ViewIcon,
  ArchiveRestoreIcon,
  HashtagIcon,
  Layers01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  QuietBreadcrumbs,
  QuietDetailAction,
  QuietDetailHeader,
  QuietEmptyState,
  QuietMetaLine,
  QuietStatusBadge,
  QuietTextAction,
  QuietTitleInput,
} from '@/components/design-system/quiet';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { Separator } from '@/components/ui/separator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { Attachments } from '@/components/pm/Attachments';
import { DatePicker } from '@/components/ui/date-picker';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { TaskListView } from '@/components/pm/TaskListView';
import type { TaskListGroupByOption } from '@/components/pm/task-detail/taskListGrouping';
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import {
  useEpic,
  useEpics,
  useEpicTasks,
  useObjectives,
  useSprints,
  useWorkflows,
  useEpicStates,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { ActivityLogEntry, AttachmentResponse, CommentWithAuthor, CreateTaskRequest, EpicWithStats, EpicHealth, LinkEpicTasksResponse, Task, UpdateEpicRequest, StateType, WorkflowWithStates } from '@/lib/pmTypes';
import { getEpicTaskCount } from '@/lib/pmTypes';
import { getWorkflowStateTone, STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';
import { InlineCompletionProgress } from '@/components/pm/InlineCompletionProgress';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { EpicPlannerPanel } from '@/components/pm/EpicPlannerPanel';
import {
  EpicDeliveryPipelineButton,
  EpicDeliveryRunsPanel,
  EpicDeliveryStatusChip,
} from '@/components/pm/EpicDeliveryRunsPanel';
import { useEpicDeliveryPlan } from '@/components/pm/useEpicDeliveryPlan';
import { ObjectivePicker, type ObjectivePickerSelection } from '@/components/pm/ObjectivePicker';
import { normalizeTeamType } from '@/lib/teamPresets';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { useRegisterPageContext } from '@/components/command-bar/pageContext';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { LinkTasksToEpicDialog } from '@/components/pm/LinkTasksToEpicDialog';
import { getLinkTasksDisabledReason } from '@/components/pm/epicTaskLinking';
import { EpicUpdatesView } from '@/components/pm/epic-detail/EpicUpdatesView';
import { DetailDescriptionEditorActions } from '@/components/pm/DetailDescriptionEditorActions';
import { DetailDescriptionEditButton } from '@/components/pm/DetailDescriptionEditButton';
import { TaskOwnerDistribution } from '@/components/pm/TaskOwnerDistribution';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { cn } from '@/lib/utils';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { getEpicTaskDisplayCount, shouldShowEpicLoading } from '@/components/pm/epic-detail/epicDetailLoadState';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/epics/$epicId');

const healthOptions: EpicHealth[] = ['no_health', 'on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};
const NO_HEALTH_DATES_TOOLTIP = 'No suggestion yet: set a start date and deadline.';
const CODE_REPO_TOOLTIP = 'Gives agents code context for planning and execution.';
const EMPTY_ITEMS: never[] = [];

function hasDraggedFiles(event: DragEvent) {
  return event.dataTransfer.types.includes('Files');
}

// ── Metadata Row ───────────────────────────────────────────────────

function MetadataRow({
  icon: Icon,
  label,
  tooltip,
  children,
}: {
  icon: React.ElementType;
  label: string;
  tooltip?: string;
  children: React.ReactNode;
}) {
  const labelNode = <span className="mt-0.5 text-[12px] text-muted-foreground">{label}</span>;

  return (
    <>
      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      {tooltip ? (
        <QuickTooltip label={tooltip} side="left">
          {labelNode}
        </QuickTooltip>
      ) : labelNode}
      <div className="min-w-0 text-[12px]">{children}</div>
    </>
  );
}

// ── Main Page ──────────────────────────────────────────────────────

interface EpicFormState {
  name: string;
  description: string;
  team_id: string;
  epic_state_id: string;
  owner_member_id: string;
  planning_repository_id: string;
  health: EpicHealth;
  planned_start_date: string;
  deadline: string;
}

const buildForm = (epic: EpicWithStats): EpicFormState => ({
  name: epic.epic.name,
  description: epic.epic.description ?? '',
  team_id: epic.epic.team_id ?? '',
  epic_state_id: epic.epic.epic_state_id ?? '',
  owner_member_id: epic.epic.owner_member_id ?? '',
  planning_repository_id: epic.epic.planning_repository_id ?? '',
  health: epic.epic.health,
  planned_start_date: epic.epic.planned_start_date ?? '',
  deadline: epic.epic.deadline ? epic.epic.deadline.slice(0, 10) : '',
});

function startOfDayUTC(value: string | Date): Date | null {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
}

function getNoHealthSuggestionMessage(epic: EpicWithStats | null): string | null {
  if (!epic || epic.suggested_health !== 'no_health') return null;

  const start = epic.epic.planned_start_date ? startOfDayUTC(epic.epic.planned_start_date) : null;
  const end = epic.epic.deadline ? startOfDayUTC(epic.epic.deadline) : null;
  const today = startOfDayUTC(new Date());

  if (!start || !end) {
    return NO_HEALTH_DATES_TOOLTIP;
  }
  if (end < start) {
    return 'No suggestion yet: fix the schedule dates.';
  }
  if (getEpicTaskCount(epic.stats) === 0) {
    return 'No suggestion yet: add tasks with workflow states to this epic.';
  }
  if (today && today < start) {
    return 'No suggestion yet: this epic has not started yet.';
  }

  return 'No suggestion yet: more planning data is needed.';
}

export function EpicDetailPage() {
  const { epicId, slug } = routeApi.useParams();
  const confirm = useConfirm();
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const currentUser = useAuthStore((s) => s.user);

  const workspaceId = workspace?.id;
  const queryClient = useQueryClient();

  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const { data: workflows = [] } = useWorkflows(workspaceId ?? '');
  const referenceFilters = useMemo(() => ({ archived: false }), []);
  const epicQuery = useEpic(workspaceId ?? '', epicId);
  const tasksQuery = useEpicTasks(workspaceId ?? '', epicId);
  const epicsQuery = useEpics(workspaceId ?? '', referenceFilters);
  const sprintsQuery = useSprints(workspaceId ?? '', referenceFilters);
  const objectivesQuery = useObjectives(workspaceId ?? '', referenceFilters);
  const repositoriesQuery = useQuery({
    queryKey: queryKeys.git.repositories(workspaceId ?? ''),
    queryFn: async () => unwrap(await gitService.listRepositories(workspaceId!)),
    enabled: !!workspaceId,
  });
  const refetchEpic = epicQuery.refetch;
  const refetchTasks = tasksQuery.refetch;

  const epic = epicQuery.data ?? null;
  const setEpic = useCallback((next: SetStateAction<EpicWithStats | null>) => {
    queryClient.setQueryData<EpicWithStats>(queryKeys.pm.epic(workspaceId ?? '', epicId), (current) => {
      const data = typeof next === 'function' ? next(current ?? null) : next;
      return data ?? current;
    });
  }, [epicId, queryClient, workspaceId]);

  const tasks = tasksQuery.data ?? EMPTY_ITEMS;
  const setTasks = useCallback((next: SetStateAction<Task[]>) => {
    queryClient.setQueryData<Task[]>(queryKeys.pm.epicTasks(workspaceId ?? '', epicId), (current) => {
      return typeof next === 'function' ? next(current ?? EMPTY_ITEMS) : next;
    });
  }, [epicId, queryClient, workspaceId]);
  const allEpics = epicsQuery.data ?? EMPTY_ITEMS;
  const allSprints = sprintsQuery.data ?? EMPTY_ITEMS;
  const allObjectives = useMemo(
    () => (objectivesQuery.data ?? EMPTY_ITEMS).map((entry) => entry.objective),
    [objectivesQuery.data],
  );
  const repositories = repositoriesQuery.data ?? EMPTY_ITEMS;

  const [formOverride, setFormOverride] = useState<{ epicId: string; data: EpicFormState } | null>(null);
  const form = formOverride?.epicId === epicId ? formOverride.data : (epic ? buildForm(epic) : null);
  const setForm = useCallback((next: SetStateAction<EpicFormState | null>) => {
    setFormOverride((current) => {
      const currentForm = current?.epicId === epicId ? current.data : (epic ? buildForm(epic) : null);
      const data = typeof next === 'function' ? next(currentForm) : next;
      return data ? { epicId, data } : null;
    });
  }, [epic, epicId]);
  const [pendingPatch, setPendingPatch] = useState<UpdateEpicRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [taskTableSaving, setTaskTableSaving] = useState(false);
  const [taskTableSaveError, setTaskTableSaveError] = useState<string | null>(null);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
  const [linkTasksOpen, setLinkTasksOpen] = useState(false);
  const [createTaskWorkflow, setCreateTaskWorkflow] = useState<WorkflowWithStates | null>(null);
  const [openingCreateTask, setOpeningCreateTask] = useState(false);
  const [pendingTeamChange, setPendingTeamChange] = useState<{
    newTeamId: string;
    newTeamName: string;
    oldTeamId: string;
    oldTeamName: string;
    affectedTaskCount: number;
  } | null>(null);
  const [movingTasks, setMovingTasks] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [editingDescription, setEditingDescription] = useState(false);
  const [descriptionDragging, setDescriptionDragging] = useState(false);
  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [commentsLoading, setCommentsLoading] = useState(true);
  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);
  const [activityLoading, setActivityLoading] = useState(true);
  const [taskListGroupBy, setTaskListGroupBy] = useState<TaskListGroupByOption>('none');
  const savedDescriptionRef = useRef('');
  const savedDescriptionEpicIdRef = useRef('');
  const descriptionUploadRef = useRef<((files: FileList | File[], insertPos?: number) => Promise<void>) | null>(null);
  const queuedDescriptionDropRef = useRef<File[] | null>(null);
  const descriptionDragCounterRef = useRef(0);
  const descriptionEditStartRef = useRef('');

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);

  const delivery = useEpicDeliveryPlan(workspaceId ?? '', epicId);
  const routeSearch = routeApi.useSearch();
  const activeView = routeSearch.epic_view === 'delivery' ? 'delivery' : 'overview';

  const selectView = useCallback((view: 'overview' | 'delivery') => {
    navigate({
      to: '.',
      search: (previous) => ({
        ...previous,
        epic_view: view === 'delivery' ? 'delivery' : undefined,
      }),
      replace: true,
    });
  }, [navigate]);

  const beginDescriptionEditing = useCallback(() => {
    if (!form) return;
    descriptionEditStartRef.current = form.description;
    setEditingDescription(true);
  }, [form]);

  const resetDescriptionDrag = useCallback(() => {
    descriptionDragCounterRef.current = 0;
    setDescriptionDragging(false);
  }, []);

  const handleDescriptionUploadReady = useCallback((upload: ((files: FileList | File[], insertPos?: number) => Promise<void>) | null) => {
    descriptionUploadRef.current = canEdit ? upload : null;
    if (!canEdit) {
      queuedDescriptionDropRef.current = null;
      return;
    }

    const queuedFiles = queuedDescriptionDropRef.current;
    if (!upload || !queuedFiles?.length) return;
    queuedDescriptionDropRef.current = null;
    void upload(queuedFiles);
  }, [canEdit]);

  const handleDescriptionDragEnter = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!canEdit || !hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    descriptionDragCounterRef.current++;
    setDescriptionDragging(true);
  }, [canEdit]);

  const handleDescriptionDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!canEdit || !hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
  }, [canEdit]);

  const handleDescriptionDragLeave = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!canEdit || !hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    descriptionDragCounterRef.current = Math.max(0, descriptionDragCounterRef.current - 1);
    if (descriptionDragCounterRef.current === 0) {
      setDescriptionDragging(false);
    }
  }, [canEdit]);

  const handleDescriptionDrop = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!canEdit || !hasDraggedFiles(event)) return;
    const alreadyHandled = event.defaultPrevented;
    event.preventDefault();
    event.stopPropagation();
    resetDescriptionDrag();
    if (alreadyHandled) return;

    const files = Array.from(event.dataTransfer.files);
    if (files.length === 0) return;
    if (descriptionUploadRef.current) {
      void descriptionUploadRef.current(files);
      return;
    }

    queuedDescriptionDropRef.current = files;
    beginDescriptionEditing();
  }, [beginDescriptionEditing, canEdit, resetDescriptionDrag]);

  const { teams, findTeamName } = useAccessibleTeams(workspaceId ?? '');
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const assignableMemberNames = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form?.team_id ? [form.team_id] : []),
    [teams, form?.team_id],
  );
  const commandBarContext = useMemo(() => {
    if (!epic) return null;
    return {
      entity_type: 'epic' as const,
      entity_id: epic.epic.id,
      display_title: epic.epic.name,
      related_ids: { task_ids: tasks.map((task) => task.id) },
    };
  }, [epic, tasks]);
  useRegisterPageContext(commandBarContext, 10);

  useTitle(form?.name ? `${form.name} — Epic` : 'Epic');

  const fetchData = useCallback(async () => {
    if (!workspaceId) return;
    await Promise.all([refetchEpic(), refetchTasks()]);
  }, [refetchEpic, refetchTasks, workspaceId]);

  const handlePlannerRunCompleted = useCallback(() => {
    void fetchData();
  }, [fetchData]);

  const reloadComments = useCallback(async () => {
    if (!workspaceId) {
      setComments([]);
      setCommentsLoading(false);
      return;
    }
    const res = await pmCommentService.list(workspaceId, 'epic', epicId);
    setComments(res.data ?? []);
    setCommentsLoading(false);
  }, [workspaceId, epicId]);

  const reloadActivity = useCallback(async () => {
    if (!workspaceId) {
      setActivity([]);
      setActivityLoading(false);
      return;
    }
    const res = await pmEpicService.listActivity(workspaceId, epicId, 1, 30);
    setActivity(res.data?.data ?? []);
    setActivityLoading(false);
  }, [workspaceId, epicId]);

  useEffect(() => {
    setCommentsLoading(true);
    setActivityLoading(true);
    void reloadComments();
    void reloadActivity();
  }, [reloadComments, reloadActivity]);

  useEffect(() => {
    const handleEpicChildUpdated = (event: Event) => {
      const detail = (event as CustomEvent).detail as { entity?: string; parent_id?: string } | undefined;
      if (detail?.parent_id !== epicId) return;
      if (detail.entity === 'comment') {
        void reloadComments();
      }
      void reloadActivity();
    };

    const handleEpicUpdated = (event: Event) => {
      const detail = (event as CustomEvent).detail as { entity_id?: string } | undefined;
      if (detail?.entity_id === epicId) {
        void reloadActivity();
      }
    };

    window.addEventListener('epic-child-updated', handleEpicChildUpdated);
    window.addEventListener('epic-updated', handleEpicUpdated);
    return () => {
      window.removeEventListener('epic-child-updated', handleEpicChildUpdated);
      window.removeEventListener('epic-updated', handleEpicUpdated);
    };
  }, [epicId, reloadComments, reloadActivity]);

  // Auto-save debounce
  useEffect(() => {
    if (
      saving ||
      Object.keys(pendingPatch).length === 0 ||
      !workspaceId ||
      !epic ||
      (pendingPatch.description !== undefined && descriptionPendingUploads > 0)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      const previousDescription = savedDescriptionEpicIdRef.current === epic.epic.id
        ? savedDescriptionRef.current
        : (epic.epic.description ?? '');
      setPendingPatch({});
      setSaving(true);
      const { data, error: err } = await pmEpicService.update(workspaceId, epic.epic.id, patch);
      if (err || !data) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setEpic(data);
        const nextDescription = data.epic.description ?? '';
        savedDescriptionEpicIdRef.current = data.epic.id;
        savedDescriptionRef.current = nextDescription;
        if (patch.description !== undefined) {
          const removedAttachmentIds = diffRemovedInlineAttachmentIds(previousDescription, nextDescription);
          if (removedAttachmentIds.length > 0) {
            await Promise.allSettled(
              removedAttachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
            );
          }
        }
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, epic, pendingPatch, saving, descriptionPendingUploads, setEpic]);

  const queuePatch = (patch: UpdateEpicRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof EpicFormState>(key: K, value: EpicFormState[K], patch: UpdateEpicRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  const cancelDescriptionEditing = () => {
    if (!form) return;
    const initialDescription = descriptionEditStartRef.current;
    if (form.description !== initialDescription) {
      updateField('description', initialDescription, { description: initialDescription });
    }
    setEditingDescription(false);
  };

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: AttachmentResponse) => {
      if (!workspaceId || !epic || !form) {
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

      const previousDescription = form.description;
      const nextDescription = removeInlineImagesByAttachmentIds(previousDescription, [entry.attachment.id]);

      setForm((current) => (current ? { ...current, description: nextDescription } : current));
      setPendingPatch((current) => {
        const rest = { ...current };
        delete rest.description;
        return rest;
      });
      setSaving(true);

      const { data, error: err } = await pmEpicService.update(workspaceId, epic.epic.id, {
        description: nextDescription,
      });
      if (err || !data) {
        setForm((current) => (current ? { ...current, description: previousDescription } : current));
        setSaveError(err ?? 'Failed to save');
        setSaving(false);
        return 'prevent' as const;
      }

      setSaveError(null);
      setEpic(data);
      savedDescriptionEpicIdRef.current = data.epic.id;
      savedDescriptionRef.current = data.epic.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [workspaceId, epic, form, confirm, setEpic, setForm],
  );

  // Derived data
  const doneTasks = useMemo(() => tasks.filter((task) => task.completed).length, [tasks]);

  const defaultEpicState = epicStates.find((s) => s.is_default) ?? epicStates[0];
  const currentEpicState = useMemo(
    () => epicStates.find((s) => s.id === form?.epic_state_id) ?? defaultEpicState,
    [epicStates, form?.epic_state_id, defaultEpicState],
  );
  const currentStateName = currentEpicState?.name ?? '';
  const currentStateColor = currentEpicState ? STATE_TYPE_ICON_CONFIG[currentEpicState.state_type as StateType]?.color : '';

  const currentTeamName = useMemo(
    () => (form?.team_id ? findTeamName(form.team_id) ?? 'Select team' : 'Select team'),
    [form?.team_id, findTeamName],
  );
  const selectedTeam = useMemo(
    () => teams.find((team) => team.id === form?.team_id),
    [teams, form?.team_id],
  );
  const showPlanningRepository = normalizeTeamType(selectedTeam?.team_type) === 'engineering';
  const noHealthSuggestionMessage = useMemo(() => getNoHealthSuggestionMessage(epic), [epic]);
  const currentOwnerName = useMemo(() => {
    if (!form?.owner_member_id) return 'No owner';
    return assignableMemberNames.get(form.owner_member_id) ?? 'Unknown';
  }, [form?.owner_member_id, assignableMemberNames]);
  const currentPlanningRepositoryName = !form?.planning_repository_id
    ? 'Not configured'
    : repositories.find((repo) => repo.id === form.planning_repository_id)?.full_name
      ?? (repositoriesQuery.isLoading ? 'Loading repository…' : 'Unknown repository');

  const workflow = workflows[0] ?? null;
  const canCreateTask = canEdit && teams.length > 0;
  const createTaskDisabledReason = !canEdit
    ? 'You need PM edit access to add tasks.'
    : teams.length === 0
      ? 'Join a team to add tasks to this epic.'
      : null;
  const linkTasksTeamId = epic?.epic.team_id ?? '';
  const linkTasksTeamName = linkTasksTeamId
    ? findTeamName(linkTasksTeamId) ?? 'this team'
    : '';
  const linkTasksDisabledReason = getLinkTasksDisabledReason(canEdit, linkTasksTeamId);
  const preferredCreateTaskTeamId = useMemo(() => {
    if (epic?.epic.team_id && teams.some((team) => team.id === epic.epic.team_id)) {
      return epic.epic.team_id;
    }
    if (form?.team_id && teams.some((team) => team.id === form.team_id)) {
      return form.team_id;
    }
    return teams[0]?.id ?? '';
  }, [epic?.epic.team_id, form?.team_id, teams]);

  const openTask = useCallback(
    ( task: Task) => {
      openTaskRoute(navigate as never, location as never, slug, task.id);
    },
    [location, navigate, slug],
  );

  const handleStartCreateTask = useCallback(async () => {
    if (!workspaceId || !canCreateTask) return;

    const teamId = preferredCreateTaskTeamId;
    if (!teamId) {
      toast.error('No team available for task creation');
      return;
    }

    setOpeningCreateTask(true);
    try {
      const resolved = await pmWorkflowService.resolveTeamWorkflow(workspaceId, teamId);
      if (resolved.error || !resolved.data) {
        throw new Error(resolved.error ?? 'Failed to open task creator');
      }
      setCreateTaskWorkflow(resolved.data);
      setCreateTaskOpen(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to open task creator');
    } finally {
      setOpeningCreateTask(false);
    }
  }, [workspaceId, canCreateTask, preferredCreateTaskTeamId]);

  const handleCreateTask = useCallback(async (payload: CreateTaskRequest) => {
    const { data, error: err } = await pmTaskService.create(payload);
    if (err || !data) {
      throw new Error(err ?? 'Failed to create task');
    }
    await fetchData();
    return data.task
      ? {
          id: data.task.task.id,
          task: {
            id: data.task.task.id,
            name: data.task.task.name,
            display_id: data.task.task.display_id,
            task_key: data.task.task.task_key,
          },
        }
      : undefined;
  }, [fetchData]);

  const handleTasksLinked = useCallback(async (result: LinkEpicTasksResponse) => {
    await Promise.all([fetchData(), reloadActivity(), delivery.reload()]);
    const linkedLabel = `${result.linked_count} task${result.linked_count === 1 ? '' : 's'}`;
    if (result.moved_count > 0) {
      toast.success(`Linked ${linkedLabel}; ${result.moved_count} moved from another epic.`);
      return;
    }
    toast.success(`Linked ${linkedLabel} to this epic.`);
  }, [delivery, fetchData, reloadActivity]);

  const selectedObjectives = useMemo<ObjectivePickerSelection[]>(
    () => (epic?.objectives ?? []).map((objective) => ({
      id: objective.id,
      name: objective.name,
      archived: objectivesQuery.isSuccess && !allObjectives.some((candidate) => candidate.id === objective.id),
    })),
    [allObjectives, epic?.objectives, objectivesQuery.isSuccess],
  );

  const updateObjectives = useCallback(async (nextObjectiveIds: string[]) => {
    if (!workspaceId || !epic) return;

    setSaveError(null);
    setSaving(true);

    const currentObjectiveIds = (epic.objectives ?? []).map((objective) => objective.id);
    const currentSet = new Set(currentObjectiveIds);
    const nextSet = new Set(nextObjectiveIds);
    const toAdd = nextObjectiveIds.filter((id) => !currentSet.has(id));
    const toRemove = currentObjectiveIds.filter((id) => !nextSet.has(id));

    setEpic((current) => current ? {
      ...current,
      objectives: allObjectives
        .filter((objective) => nextSet.has(objective.id))
        .map((objective) => ({ id: objective.id, name: objective.name })),
    } : current);

    const results = await Promise.all([
      ...toAdd.map((objectiveId) => pmObjectiveService.addEpic(workspaceId, objectiveId, epic.epic.id)),
      ...toRemove.map((objectiveId) => pmObjectiveService.removeEpic(workspaceId, objectiveId, epic.epic.id)),
    ]);

    const failed = results.find((result) => result.error);
    await fetchData();
    if (failed) {
      setSaveError(failed.error ?? 'Failed to update objectives');
      setSaving(false);
      return;
    }

    setSaving(false);
  }, [allObjectives, epic, fetchData, setEpic, workspaceId]);

  // Refresh tasks when global panel updates/archives a task
  useEffect(() => {
    const refresh = () => {
      if (!workspaceId) return;
      void refetchTasks();
    };
    window.addEventListener('task-panel-updated', refresh);
    window.addEventListener('task-panel-archived', refresh);
    return () => {
      window.removeEventListener('task-panel-updated', refresh);
      window.removeEventListener('task-panel-archived', refresh);
    };
  }, [workspaceId, epicId, refetchTasks]);

  const goBack = () => navigate({
    to: '/w/$slug/pm/epics',
    params: { slug },
    search: epic?.epic.team_id ? { team: epic.epic.team_id } : {},
  });

  const viewEpicTasksPage = () => {
    if (!epic) return;
    navigate({
      to: '/w/$slug/pm/tasks',
      params: { slug },
      search: {
        epic: epic.epic.id,
        ...(epic.epic.team_id ? { team: epic.epic.team_id } : {}),
      },
    });
  };

  const renderTaskHeaderAddButton = () => (
    <div className="flex items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground"
        onClick={viewEpicTasksPage}
      >
        <ViewIcon />
        View on Tasks page
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground"
        onClick={() => setLinkTasksOpen(true)}
        disabled={linkTasksDisabledReason !== null}
        title={linkTasksDisabledReason ?? undefined}
      >
        <Link01Icon />
        Link tasks
      </Button>
      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground"
        onClick={() => void handleStartCreateTask()}
        disabled={!canCreateTask || openingCreateTask}
        title={createTaskDisabledReason ?? undefined}
      >
        {openingCreateTask ? (
          <Loading01Icon className="animate-spin" />
        ) : (
          <PlusSignIcon />
        )}
        Create task
      </Button>
    </div>
  );

  const renderEmptyTaskActions = () => (
    <div className="mt-2 flex flex-wrap items-center gap-1">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground"
        onClick={() => setLinkTasksOpen(true)}
        disabled={linkTasksDisabledReason !== null}
        title={linkTasksDisabledReason ?? undefined}
      >
        <Link01Icon />
        <span>Link existing tasks</span>
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground"
        onClick={() => void handleStartCreateTask()}
        disabled={!canCreateTask || openingCreateTask}
        title={createTaskDisabledReason ?? undefined}
      >
        {openingCreateTask ? (
          <Loading01Icon className="animate-spin" />
        ) : (
          <PlusSignIcon />
        )}
        <span>Create task</span>
      </Button>
    </div>
  );

  const handleArchiveToggle = async () => {
    if (!workspaceId || !epic) return;
    if (!epic.epic.archived) {
      setArchiveConfirmOpen(true);
      return;
    }
    setSaving(true);
    const { data, error: err } = await pmEpicService.update(workspaceId, epic.epic.id, { archived: false });
    if (err || !data) {
      setSaveError(err ?? 'Failed to update');
    } else {
      setEpic(data);
      setSaveError(null);
    }
    setSaving(false);
  };

  const initialEpicLoading = !workspaceId || shouldShowEpicLoading(epic, epicQuery.isLoading);
  const loadError = !epic && epicQuery.error instanceof Error ? epicQuery.error.message : null;
  const tasksUnavailable = tasksQuery.data === undefined;
  const areTasksPending = tasksQuery.isLoading && tasksUnavailable;
  const taskLoadError = tasksQuery.isError && tasksUnavailable;
  const taskDisplayCount = epic ? getEpicTaskDisplayCount(epic, tasks, tasksUnavailable) : 0;

  if (initialEpicLoading) {
    return (
      <div className="flex h-full flex-col overflow-hidden">
        <QuietDetailHeader
          className="lg:px-10"
          breadcrumbs={<QuietBreadcrumbs items={[{ id: 'epics', label: 'Epics', icon: <Layers01Icon className="h-3.5 w-3.5 text-quiet-muted" />, onClick: goBack }]} onBack={goBack} backLabel="Back to epics" />}
          title={<div className="h-6 w-64 max-w-full animate-pulse bg-quiet-icon-well" />}
          meta={<div className="h-3 w-40 max-w-full animate-pulse bg-quiet-icon-well" />}
        />
        <div className="flex flex-1 items-center justify-center">
          <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  if (loadError || !epic || !form) {
    return (
      <div className="flex h-full flex-col overflow-hidden">
        <QuietDetailHeader
          className="lg:px-10"
          breadcrumbs={<QuietBreadcrumbs items={[{ id: 'epics', label: 'Epics', icon: <Layers01Icon className="h-3.5 w-3.5 text-quiet-muted" />, onClick: goBack }]} onBack={goBack} backLabel="Back to epics" />}
          title="Epic"
        />
        <div className="flex-1 overflow-auto p-4 sm:p-6">
          <QuietEmptyState
            title={loadError ?? 'Epic not found'}
            description="This epic may have been moved, archived, deleted, or is no longer available to you."
            action={<QuietTextAction onClick={goBack}><ArrowLeft02Icon className="h-3.5 w-3.5" />Back to Epics</QuietTextAction>}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <QuietDetailHeader
        className="lg:px-10"
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'epics', label: 'Epics', icon: <Layers01Icon className="h-3.5 w-3.5 text-quiet-muted" />, onClick: goBack }]} onBack={goBack} backLabel="Back to epics" />}
        title={(
          <QuietTitleInput
            type="text"
            aria-label="Epic title"
            presentation="header"
            className="max-w-[42rem] border-b-transparent hover:border-quiet-field focus-visible:border-quiet-text-primary"
            value={form.name}
            onChange={(event) => updateField('name', event.target.value, { name: event.target.value })}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                event.currentTarget.blur();
              }
            }}
            placeholder="Untitled"
          />
        )}
        meta={<QuietMetaLine items={[selectedTeam?.name ?? 'No team', `${tasks.length} task${tasks.length === 1 ? '' : 's'}`]} />}
        status={(
          <QuietStatusBadge className="lg:hidden" tone={epic.epic.archived ? 'neutral' : getWorkflowStateTone(currentEpicState?.state_type as StateType | undefined)} color={epic.epic.archived ? undefined : currentEpicState?.color}>
            {epic.epic.archived ? 'Archived' : currentStateName || 'No state'}
          </QuietStatusBadge>
        )}
        state={<SaveIndicator saving={saving || taskTableSaving} error={saveError || taskTableSaveError} presentation="quiet" />}
        actions={(
          <>
            <FollowButton entityType="epic" entityId={epic.epic.id} presentation="detail-header" />
            <QuietDetailAction
              icon={epic.epic.archived ? <ArchiveRestoreIcon className="h-3.5 w-3.5" /> : <ArchiveIcon className="h-3.5 w-3.5" />}
              label={epic.epic.archived ? 'Unarchive' : 'Archive'}
              onClick={() => void handleArchiveToggle()}
            />
          </>
        )}
      />

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="relative grid min-h-0 flex-1 grid-cols-1 overflow-y-auto lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="flex min-h-0 min-w-0 flex-col lg:overflow-hidden">
          <Tabs value={activeView} onValueChange={(value) => selectView(value as 'overview' | 'delivery')} className="gap-0">
            <TabsList variant="quiet" aria-label="Epic detail views" className="w-full justify-start px-4 sm:px-6 lg:px-10">
            {(['overview', 'delivery'] as const).map((view) => (
              <TabsTrigger
                key={view}
                value={view}
                className="capitalize"
              >
                {view}
              </TabsTrigger>
            ))}
            </TabsList>
          </Tabs>

          <div className="min-h-0 min-w-0 flex-1 overflow-x-hidden px-4 pt-5 sm:px-6 lg:overflow-y-auto lg:px-10">
          {activeView === 'overview' ? (
          <>
          {/* Description */}
          <div
            className={cn(
              'group/desc relative rounded-lg pb-3 transition-[box-shadow,background-color]',
              descriptionDragging && 'bg-primary/5 ring-1 ring-primary/50',
            )}
            onDragEnter={handleDescriptionDragEnter}
            onDragOver={handleDescriptionDragOver}
            onDragLeave={handleDescriptionDragLeave}
            onDrop={handleDescriptionDrop}
          >
            {descriptionDragging && (
              <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-lg border border-dashed border-primary bg-background/80">
                <div className="flex items-center gap-2 rounded-md bg-background px-3 py-1.5 text-xs font-medium text-foreground shadow-sm">
                  <AttachmentIcon className="h-3.5 w-3.5 text-primary" />
                  Drop to insert here
                </div>
              </div>
            )}
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
                  onUploadReady={handleDescriptionUploadReady}
                  teams={mentionTeams}
                  members={assignableMembers}
                />
                <DetailDescriptionEditorActions
                  onCancel={cancelDescriptionEditing}
                  onDone={() => setEditingDescription(false)}
                />
              </div>
            ) : (
              <div className={cn('relative min-h-9', canEdit && 'pr-12')}>
                {form.description ? (
                  <RichTextMentionContent
                    html={form.description}
                    members={assignableMembers}
                    teams={mentionTeams}
                    variant="pm"
                  />
                ) : (
                  <p className="text-sm text-muted-foreground">{canEdit ? 'No description yet' : 'No description'}</p>
                )}
                {canEdit && <DetailDescriptionEditButton onClick={beginDescriptionEditing} />}
              </div>
            )}
          </div>

          {/* Attachments: compact action when empty, full section once populated. */}
          <div id="attachments-section">
            <Attachments
              workspaceId={workspaceId!}
              entityType="epic"
              entityId={epic.epic.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              editable={canEdit}
              showAddAction
              emptyPresentation="inline-action"
            />
          </div>

          {/* Task ownership distribution */}
          <div className="mt-6">
            <TaskOwnerDistribution tasks={tasks} members={assignableMembers} />
          </div>

          <Separator className="my-6" />

          {/* Tasks */}
          <div>
            <TaskDetailSectionHeading
              title={`Tasks (${taskDisplayCount})`}
              icon={CheckListIcon}
              meta={!areTasksPending && tasks.length > 0 ? (
                <div className="flex min-w-0 flex-1 items-center">
                  <InlineCompletionProgress
                    completed={doneTasks}
                    total={tasks.length}
                    showCount={false}
                    className="ml-1.5"
                    testIdPrefix="epic-tasks"
                  />
                  <div className="ml-auto pl-6">{renderTaskHeaderAddButton()}</div>
                </div>
              ) : undefined}
            />
            {taskLoadError ? (
              <div className="mt-3 flex items-center gap-2 text-sm text-muted-foreground" role="alert">
                <span>Tasks could not be loaded.</span>
                <Button variant="ghost" size="sm" onClick={() => void refetchTasks()}>Retry</Button>
              </div>
            ) : areTasksPending ? (
              <div className="mt-3 space-y-2" aria-label="Loading epic tasks">
                {[0, 1, 2].map((row) => (
                  <div key={row} className="h-10 animate-pulse rounded-md bg-quiet-icon-well" />
                ))}
              </div>
            ) : tasks.length === 0 ? (
              <div className="mt-3">
                <p className="text-sm italic text-muted-foreground">No tasks linked yet.</p>
                {renderEmptyTaskActions()}
              </div>
            ) : workflow ? (
              <div className="mt-3 -mx-3">
                <TaskListView
                  workspaceId={workspaceId!}
                  workflow={workflow}
                  workflows={workflows}
                  teams={teams}
                  assignableMembers={assignableMembers}
                  epics={allEpics}
                  sprints={allSprints}
                  teamId={epic.epic.team_id ?? null}
                  epicId={epicId}
                  externalTasks={tasks}
                  onExternalTasksChange={setTasks}
                  onInlineUpdateSavingChange={setTaskTableSaving}
                  onInlineUpdateError={setTaskTableSaveError}
                  onOpenTask={openTask}
                  groupBy={taskListGroupBy}
                  onGroupByChange={setTaskListGroupBy}
                  showLocalTaskControls
                  onBulkOperationComplete={fetchData}
                />
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Loading workflow...</p>
            )}
          </div>

          <Separator className="my-6" />

          <TaskDetailSectionHeading title="Updates" icon={Activity01Icon} className="mb-4" />
          {workspaceId ? (
            <EpicUpdatesView
              workspaceId={workspaceId}
              epicId={epicId}
              comments={comments}
              activity={activity}
              commentsLoading={commentsLoading}
              activityLoading={activityLoading}
              currentUserId={currentUser?.id}
              teams={mentionTeams}
              members={assignableMembers}
              onCommentsChange={setComments}
            />
          ) : null}
          </>
          ) : (
          <div className="space-y-8">
            <section>
              <TaskDetailSectionHeading
                title="Delivery pipeline"
                icon={SourceCodeIcon}
                meta={canEdit && workspaceId ? (
                  <EpicDeliveryPipelineButton
                    workspaceId={workspaceId}
                    epicId={epicId}
                    onStarted={delivery.reload}
                    disabled={tasks.length === 0}
                    disabledReason="Add at least one task before running delivery."
                  />
                ) : undefined}
              />

              {tasks.length === 0 ? (
                <div className="mt-3 border-t border-border/60 py-8">
                  <p className="text-sm font-medium text-foreground">Add tasks before running delivery</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    The delivery pipeline implements, reviews, and merges the open tasks linked to this epic.
                  </p>
                </div>
              ) : delivery.plan && workspaceId ? (
                <div className="mt-3">
                  <EpicDeliveryRunsPanel
                    workspaceId={workspaceId}
                    plan={delivery.plan}
                    runsById={delivery.runsById}
                    onReload={delivery.reload}
                    canEdit={canEdit}
                  />
                </div>
              ) : (
                <div className="mt-3 border-t border-border/60 py-8">
                  <p className="text-sm font-medium text-foreground">No delivery runs yet</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Run the delivery pipeline to execute this epic's open tasks in dependency order.
                  </p>
                </div>
              )}
            </section>

            <Separator />

            {workspaceId ? (
              <EpicPlannerPanel
                workspaceId={workspaceId}
                epicId={epicId}
                epicTeamId={form.team_id || null}
                lastRunId={epic.epic.last_planning_run_id}
                canEdit={canEdit}
                onRunCompleted={handlePlannerRunCompleted}
              />
            ) : null}
          </div>
          )}
          <div className="h-40 shrink-0" aria-hidden="true" />
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 border-t border-border/60 px-5 py-5 pb-40 lg:overflow-y-auto lg:border-t-0 lg:border-l">
          {delivery.plan ? (
            <EpicDeliveryStatusChip
              plan={delivery.plan}
              runsById={delivery.runsById}
              onClick={() => selectView('delivery')}
            />
          ) : null}

          <div className="mt-5 grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* State */}
            <MetadataRow icon={HashtagIcon} label="State">
              <SidebarPopoverSelect
                value={form.epic_state_id || defaultEpicState?.id || ''}
                options={epicStates.map((s) => ({ value: s.id, label: s.name, className: STATE_TYPE_ICON_CONFIG[s.state_type as StateType]?.color }))}
                onChange={(v) => {
                  updateField('epic_state_id', v, { epic_state_id: v });
                }}
                renderTrigger={() => <span className={currentStateColor}>{currentStateName}</span>}
              />
            </MetadataRow>

            {/* Health */}
            <MetadataRow icon={FavouriteIcon} label="Health">
              <div className="flex flex-col gap-1">
                <SidebarPopoverSelect
                  value={form.health}
                  options={healthOptions.map((h) => ({ value: h, label: healthConfig[h].label, className: healthConfig[h].color }))}
                  onChange={(v) => updateField('health', v as EpicHealth, { health: v as EpicHealth })}
                  renderTrigger={() => {
                    const healthLabel = (
                      <span className={healthConfig[form.health]?.color}>{healthConfig[form.health]?.label}</span>
                    );
                    return noHealthSuggestionMessage ? (
                      <QuickTooltip label={noHealthSuggestionMessage}>
                        {healthLabel}
                      </QuickTooltip>
                    ) : healthLabel;
                  }}
                />
                {!noHealthSuggestionMessage && epic?.suggested_health && epic.suggested_health !== form.health && (
                  <button
                    type="button"
                    className="text-[10px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer text-left"
                    onClick={() => updateField('health', epic.suggested_health as EpicHealth, { health: epic.suggested_health as EpicHealth })}
                  >
                    Suggested: <span className={healthConfig[epic.suggested_health]?.color}>{healthConfig[epic.suggested_health]?.label}</span>
                  </button>
                )}
              </div>
            </MetadataRow>

            {/* Team */}
            <MetadataRow icon={UserGroupIcon} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  { value: '__none__', label: 'Select team' },
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  if (val === form.team_id) return;

                  // Check if tasks exist on the current team
                  const oldTeamId = form.team_id;
                  const tasksOnOldTeam = tasks.filter((t) => t.team_id === oldTeamId);

                  if (tasksOnOldTeam.length > 0 && oldTeamId) {
                    const newTeam = teams.find((team) => team.id === val);
                    const oldTeam = teams.find((team) => team.id === oldTeamId);
                    setPendingTeamChange({
                      newTeamId: val,
                      newTeamName: newTeam?.name ?? 'the new team',
                      oldTeamId,
                      oldTeamName: oldTeam?.name ?? 'the current team',
                      affectedTaskCount: tasksOnOldTeam.length,
                    });
                    return;
                  }

                  // No tasks affected — apply directly
                  const nextTeam = teams.find((team) => team.id === val);
                  const nextIsEngineering = normalizeTeamType(nextTeam?.team_type) === 'engineering';
                  const nextPlanningRepositoryId = nextIsEngineering ? form.planning_repository_id : '';

                  setForm((current) => current ? {
                    ...current,
                    team_id: val,
                    planning_repository_id: nextPlanningRepositoryId,
                  } : current);
                  queuePatch({
                    team_id: val || undefined,
                    planning_repository_id: nextIsEngineering ? (form.planning_repository_id || undefined) : undefined,
                  });
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
              />
            </MetadataRow>

            {/* Objectives */}
            <MetadataRow icon={Target01Icon} label="Objective">
              <ObjectivePicker
                objectives={allObjectives}
                selectedObjectiveIds={(epic.objectives ?? []).map((objective) => objective.id)}
                selectedObjectives={selectedObjectives}
                onChange={updateObjectives}
                addLabel="Add objective"
                className="min-h-6"
              />
            </MetadataRow>

            {/* Owner */}
            <MetadataRow icon={UserIcon} label="Owner">
              <MemberPickerPopover
                value={form.owner_member_id || '__none__'}
                members={assignableMembers}
                noneLabel="No owner"
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('owner_member_id', val, { owner_member_id: val || undefined });
                }}
                renderTrigger={() => {
                  const selectedMember = findAssignableMember(assignableMembers, form.owner_member_id);
                  return (
                    <>
                      {selectedMember ? (
                        <UserAvatar
                          name={selectedMember.display_name || selectedMember.email}
                          avatarUrl={selectedMember.avatar_url}
                          avatarStyle={selectedMember.avatar_style}
                          avatarSeed={selectedMember.avatar_seed}
                          avatarBackgroundMode={selectedMember.avatar_background_mode}
                          avatarBackgroundColor={selectedMember.avatar_background_color}
                          className="h-4 w-4"
                          fallbackClassName="text-[7px]"
                        />
                      ) : null}
                      <span>{currentOwnerName}</span>
                    </>
                  );
                }}
              />
            </MetadataRow>

            {/* Start Date */}
            <MetadataRow icon={Calendar03Icon} label="Start date">
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
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={Calendar03Icon} label="Target date">
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
            </MetadataRow>

            {showPlanningRepository ? (
              <>
                <Separator className="col-span-3 my-1" />

                {/* Code repo */}
                <MetadataRow icon={SourceCodeIcon} label="Code repo" tooltip={CODE_REPO_TOOLTIP}>
                  <SidebarPopoverSelect
                    value={form.planning_repository_id || '__none__'}
                    options={[
                      { value: '__none__', label: 'Not configured' },
                      ...repositories.map((repo) => ({ value: repo.id, label: repo.full_name })),
                    ]}
                    onChange={(v) => {
                      const val = v === '__none__' ? '' : v;
                      updateField('planning_repository_id', val, { planning_repository_id: val || undefined });
                    }}
                    renderTrigger={() => (
                      <span className="block whitespace-normal break-words text-left leading-tight">
                        {currentPlanningRepositoryName}
                      </span>
                    )}
                  />
                </MetadataRow>
              </>
            ) : null}
          </div>

          {/* Labels */}
          {epic.labels && epic.labels.length > 0 && (
            <div className="mt-6">
              <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Labels</h4>
              <div className="flex flex-wrap gap-1">
                {epic.labels.map((l) => (
                  <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                    {l.name}
                  </span>
                ))}
              </div>
            </div>
          )}

          {workspaceId ? (
            <details id="epic-related-section" className="-mx-5 mt-4 border-t border-border/60 px-5 pt-4" open>
              <summary className="cursor-pointer select-none text-xs font-semibold uppercase tracking-wide text-foreground/70">
                Related
              </summary>
              <div className="mt-3 space-y-0">
                <AssociationsPanel
                  objectType="epic"
                  objectId={epicId}
                  workspaceId={workspaceId}
                  section="docs"
                />

                <div className="my-2 h-px bg-border/60" />
                <ExternalLinks
                  workspaceId={workspaceId}
                  entityType="epic"
                  entityId={epicId}
                  flat
                />

                <div className="my-2 h-px bg-border/60" />
                <AssociationsPanel
                  objectType="epic"
                  objectId={epicId}
                  workspaceId={workspaceId}
                  excludeDocs
                  section="support"
                />

                <div className="my-2 h-px bg-border/60" />
                <AssociationsPanel
                  objectType="epic"
                  objectId={epicId}
                  workspaceId={workspaceId}
                  excludeDocs
                  section="crm"
                />
              </div>
            </details>
          ) : null}
        </aside>
      </div>

      {/* Team change confirmation — shown when epic has tasks on the old team */}
      <Dialog open={pendingTeamChange !== null} onOpenChange={(open) => { if (!open) setPendingTeamChange(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Change team?</DialogTitle>
            <DialogDescription>
              This epic has {pendingTeamChange?.affectedTaskCount} task{pendingTeamChange?.affectedTaskCount === 1 ? '' : 's'} on {pendingTeamChange?.oldTeamName}. What should happen to them?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex-col gap-2 sm:flex-col">
            <Button
              disabled={movingTasks}
              onClick={async () => {
                if (!pendingTeamChange || !workspaceId) return;
                setMovingTasks(true);
                // Apply team change to epic
                const nextTeam = teams.find((t) => t.id === pendingTeamChange.newTeamId);
                const nextIsEngineering = normalizeTeamType(nextTeam?.team_type) === 'engineering';
                setForm((current) => current ? { ...current, team_id: pendingTeamChange.newTeamId, planning_repository_id: nextIsEngineering ? current.planning_repository_id : '' } : current);
                queuePatch({ team_id: pendingTeamChange.newTeamId || undefined });
                // Move tasks to new team
                const tasksToMove = tasks.filter((t) => t.team_id === pendingTeamChange.oldTeamId);
                await Promise.allSettled(
                  tasksToMove.map((t) => pmTaskService.update(workspaceId, t.id, { team_id: pendingTeamChange.newTeamId }))
                );
                setMovingTasks(false);
                setPendingTeamChange(null);
                toast.success(`Epic and ${tasksToMove.length} task${tasksToMove.length === 1 ? '' : 's'} moved to ${pendingTeamChange.newTeamName}`);
              }}
            >
              {movingTasks ? 'Moving...' : `Move ${pendingTeamChange?.affectedTaskCount} task${pendingTeamChange?.affectedTaskCount === 1 ? '' : 's'} to ${pendingTeamChange?.newTeamName}`}
            </Button>
            <Button
              variant="outline"
              disabled={movingTasks}
              onClick={() => {
                if (!pendingTeamChange) return;
                const nextTeam = teams.find((t) => t.id === pendingTeamChange.newTeamId);
                const nextIsEngineering = normalizeTeamType(nextTeam?.team_type) === 'engineering';
                setForm((current) => current ? { ...current, team_id: pendingTeamChange.newTeamId, planning_repository_id: nextIsEngineering ? current.planning_repository_id : '' } : current);
                queuePatch({ team_id: pendingTeamChange.newTeamId || undefined });
                setPendingTeamChange(null);
                toast.success(`Epic moved to ${pendingTeamChange.newTeamName}. Tasks remain on ${pendingTeamChange.oldTeamName}.`);
              }}
            >
              Keep tasks on {pendingTeamChange?.oldTeamName}
            </Button>
            <Button variant="ghost" disabled={movingTasks} onClick={() => setPendingTeamChange(null)}>
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title="Archive epic"
        description="This epic will be hidden from the active list. You can restore it later from the archived view."
        confirmLabel="Archive"
        variant="default"
        onConfirm={async () => {
          if (!workspaceId || !epic) return;
          setSaving(true);
          const { data, error: err } = await pmEpicService.update(workspaceId, epic.epic.id, { archived: true });
          if (err || !data) {
            setSaveError(err ?? 'Failed to archive');
          } else {
            setEpic(data);
            setSaveError(null);
          }
          setSaving(false);
        }}
      />

      {workspaceId && linkTasksTeamId ? (
        <LinkTasksToEpicDialog
          open={linkTasksOpen}
          onOpenChange={setLinkTasksOpen}
          workspaceId={workspaceId}
          epicId={epic.epic.id}
          epicName={epic.epic.name}
          teamId={linkTasksTeamId}
          teamName={linkTasksTeamName}
          onLinked={(result) => void handleTasksLinked(result)}
        />
      ) : null}

      {createTaskWorkflow ? (
        <CreateTaskModal
          open={createTaskOpen}
          onOpenChange={setCreateTaskOpen}
          workspaceId={workspaceId!}
          workflow={createTaskWorkflow}
          initialStateId={
            createTaskWorkflow.workflow.default_state_id ??
            createTaskWorkflow.states.find((state) => state.is_default)?.id ??
            createTaskWorkflow.states[0]?.id ??
            ''
          }
          initialTeamId={preferredCreateTaskTeamId || createTaskWorkflow.workflow.team_id}
          initialEpicId={epic.epic.id}
          onCreate={handleCreateTask}
        />
      ) : null}
    </div>
  );
}
