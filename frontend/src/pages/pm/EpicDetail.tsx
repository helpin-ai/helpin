import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useTitle } from '@/hooks/useTitle';
import {
  Activity01Icon,
  ArchiveIcon,
  ArrowLeft02Icon,
  Calendar03Icon,
  ArrowRight01Icon,
  AttachmentIcon,
  ChartColumnIcon,
  CheckListIcon,
  FavouriteIcon,
  Link01Icon,
  Loading01Icon,
  Message01Icon,
  PencilEdit01Icon,
  PlusSignIcon,
  SourceCodeIcon,
  Target01Icon,
  Upload01Icon,
  UserIcon,
  UserGroupIcon,
  ViewIcon,
  ArchiveRestoreIcon,
  BotIcon,
  HashtagIcon,
  Layers01Icon,
  LayoutTable01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
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
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { useWorkflows, useEpicStates, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { ActivityLogEntry, AttachmentResponse, CommentWithAuthor, CreateTaskRequest, EpicWithStats, EpicHealth, GitRepository, Objective, Task, SprintWithStats, UpdateEpicRequest, StateType, WorkflowWithStates } from '@/lib/pmTypes';
import { getEpicTaskCount } from '@/lib/pmTypes';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { ActivityTimeline } from '@/components/pm/ActivityTimeline';
import { CommentThread } from '@/components/pm/CommentThread';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { EpicPlannerPanel } from '@/components/pm/EpicPlannerPanel';
import {
  EPIC_DELIVERY_PANEL_ID,
  EpicDeliveryRunsPanel,
  EpicDeliveryStatusChip,
  deliveryDotState,
} from '@/components/pm/EpicDeliveryRunsPanel';
import { useEpicDeliveryPlan } from '@/components/pm/useEpicDeliveryPlan';
import { StatusDot } from '@/components/agents/dock/StatusDot';
import { ObjectivePicker, type ObjectivePickerSelection } from '@/components/pm/ObjectivePicker';
import { normalizeTeamType } from '@/lib/teamPresets';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { useRegisterPageContext } from '@/components/command-bar/pageContext';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

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
  const labelNode = <span className="text-[12px] text-muted-foreground self-center">{label}</span>;

  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
      {tooltip ? (
        <QuickTooltip label={tooltip} side="left">
          {labelNode}
        </QuickTooltip>
      ) : labelNode}
      <div className="min-w-0 self-center text-[12px]">{children}</div>
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

  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const { data: workflows = [] } = useWorkflows(workspaceId ?? '');

  const [epic, setEpic] = useState<EpicWithStats | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [allEpics, setAllEpics] = useState<EpicWithStats[]>([]);
  const [allSprints, setAllSprints] = useState<SprintWithStats[]>([]);
  const [allObjectives, setAllObjectives] = useState<Objective[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<EpicFormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateEpicRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [taskTableSaving, setTaskTableSaving] = useState(false);
  const [taskTableSaveError, setTaskTableSaveError] = useState<string | null>(null);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
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
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [hasExternalLinkItems, setHasExternalLinkItems] = useState(false);
  const [externalLinkCount, setExternalLinkCount] = useState(0);
  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [commentsLoading, setCommentsLoading] = useState(true);
  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);
  const [activityLoading, setActivityLoading] = useState(true);
  const [showAllActivity, setShowAllActivity] = useState(false);
  const [taskListGroupBy, setTaskListGroupBy] = useState<TaskListGroupByOption>('none');
  const [panelDragging, setPanelDragging] = useState(false);
  const savedDescriptionRef = useRef('');
  const openFilePickerRef = useRef<(() => void) | null>(null);
  const uploadFilesRef = useRef<((files: FileList | File[]) => Promise<void>) | null>(null);
  const dragCounterRef = useRef(0);

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);

  const delivery = useEpicDeliveryPlan(workspaceId ?? '', epicId);
  const [tasksView, setTasksView] = useState<'list' | 'delivery'>('list');
  // Delivery is an event, not a permanent projection — fall back to the task
  // list whenever the epic has no delivery plan.
  const effectiveTasksView = delivery.plan ? tasksView : 'list';

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
      related_ids: { task_ids: tasks.map((task) => task.id), story_ids: tasks.map((task) => task.id) },
    };
  }, [epic, tasks]);
  useRegisterPageContext(commandBarContext, 10);

  useTitle(form?.name ? `${form.name} — Epic` : 'Epic');

  const fetchData = useCallback(async (showLoading = true) => {
    if (!workspaceId) return;
    if (showLoading) setLoading(true);
    setError(null);
    const [epicRes, tasksRes, epicsRes, sprintsRes, objectivesRes, reposRes] = await Promise.all([
      pmEpicService.get(workspaceId, epicId),
      pmEpicService.listTasks(workspaceId, epicId),
      pmEpicService.list(workspaceId, { archived: false }),
      pmSprintService.list(workspaceId, { archived: false }),
      pmObjectiveService.list(workspaceId, { archived: false }),
      gitService.listRepositories(workspaceId),
    ]);
    if (epicRes.error || !epicRes.data) {
      setError(epicRes.error ?? 'Epic not found');
      setLoading(false);
      return;
    }
    setEpic(epicRes.data);
    savedDescriptionRef.current = epicRes.data.epic.description ?? '';
    setForm((current) => current ? current : buildForm(epicRes.data!));
    setTasks(tasksRes.data ?? []);
    setAllEpics(epicsRes.data ?? []);
    setAllSprints(sprintsRes.data ?? []);
    setAllObjectives((objectivesRes.data ?? []).map((entry) => entry.objective));
    setRepositories(reposRes.data ?? []);
    setLoading(false);
  }, [workspaceId, epicId]);

  const handlePlannerRunCompleted = useCallback(() => {
    void fetchData(false);
  }, [fetchData]);

  // Load epic data + reference data
  useEffect(() => {
    fetchData();
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
    setShowAllActivity(false);
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

  // Auto-show external links if they exist
  useEffect(() => {
    if (!workspaceId || !epic?.epic?.id) return;
    pmExternalLinkService.listByEntity(workspaceId, 'epic', epic.epic.id).then(({ data }) => {
      const count = data?.length ?? 0;
      setHasExternalLinkItems(count > 0);
      setExternalLinkCount(count);
      if (count > 0) setShowExternalLinks(true);
    });
  }, [workspaceId, epic?.epic?.id]);

  const handleExternalLinkContentChange = useCallback((hasContent: boolean) => {
    setHasExternalLinkItems(hasContent);
    if (hasContent) setShowExternalLinks(true);
  }, []);

  const handleExternalLinkCountChange = useCallback((count: number) => {
    setExternalLinkCount(count);
  }, []);

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
      const previousDescription = savedDescriptionRef.current;
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
  }, [workspaceId, epic, pendingPatch, saving, descriptionPendingUploads]);

  const queuePatch = (patch: UpdateEpicRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof EpicFormState>(key: K, value: EpicFormState[K], patch: UpdateEpicRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
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
        const { description, ...rest } = current;
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
      savedDescriptionRef.current = data.epic.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [workspaceId, epic, form],
  );

  // Derived data
  const progressSummary = useMemo(() => {
    const totalTasks = tasks.length;
    const doneTasks = tasks.filter((task) => task.completed).length;
    const progress = totalTasks === 0 ? 0 : Math.round((doneTasks / totalTasks) * 100);
    return {
      progress,
      doneTasks,
      totalTasks,
      remainingTasks: Math.max(totalTasks - doneTasks, 0),
    };
  }, [tasks]);
  const { progress, doneTasks, totalTasks, remainingTasks } = progressSummary;

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
  const currentPlanningRepositoryName = useMemo(() => {
    if (!form?.planning_repository_id) return 'Not configured';
    return repositories.find((repo) => repo.id === form.planning_repository_id)?.full_name ?? 'Unknown repository';
  }, [form?.planning_repository_id, repositories]);

  const workflow = workflows[0] ?? null;
  const canCreateTask = canEdit && teams.length > 0;
  const createTaskDisabledReason = !canEdit
    ? 'You need PM edit access to add tasks.'
    : teams.length === 0
      ? 'Join a team to add tasks to this epic.'
      : null;
  const preferredCreateTaskTeamId = useMemo(() => {
    if (epic?.epic.team_id && teams.some((team) => team.id === epic.epic.team_id)) {
      return epic.epic.team_id;
    }
    if (form?.team_id && teams.some((team) => team.id === form.team_id)) {
      return form.team_id;
    }
    return teams[0]?.id ?? '';
  }, [epic?.epic.team_id, form?.team_id, teams]);

  // Resources: task owner workload summary.
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string; taskCount: number; percentage: number }>();
    const totalTasks = tasks.length;
    let unassignedTaskCount = 0;

    for (const task of tasks) {
      const ownerIds = task.owner_member_ids ?? [];
      if (ownerIds.length === 0) {
        unassignedTaskCount += 1;
        continue;
      }
      for (const ownerKey of ownerIds) {
        const assignable = findAssignableMember(assignableMembers, ownerKey);
        if (assignable) {
          const existing = personMap.get(assignable.id);
          if (existing) {
            existing.taskCount += 1;
          } else {
            personMap.set(assignable.id, {
              id: assignable.id,
              name: assignableMemberNames.get(assignable.id) ?? assignable.display_name,
              email: assignable.email,
              taskCount: 1,
              percentage: 0,
            });
          }
        }
      }
    }

    const owners = Array.from(personMap.values())
      .map((person) => ({
        ...person,
        percentage: totalTasks > 0 ? Math.round((person.taskCount / totalTasks) * 100) : 0,
      }))
      .sort((a, b) => b.taskCount - a.taskCount || (a.name || a.email).localeCompare(b.name || b.email));

    if (unassignedTaskCount === 0) {
      return owners;
    }

    return [
      {
        id: '__unassigned__',
        name: 'Unassigned',
        email: '',
        taskCount: unassignedTaskCount,
        percentage: totalTasks > 0 ? Math.round((unassignedTaskCount / totalTasks) * 100) : 0,
      },
      ...owners,
    ];
  }, [tasks, assignableMembers, assignableMemberNames]);

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
    await fetchData(false);
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

  const selectedObjectives = useMemo<ObjectivePickerSelection[]>(
    () => (epic?.objectives ?? []).map((objective) => ({
      id: objective.id,
      name: objective.name,
      archived: !allObjectives.some((candidate) => candidate.id === objective.id),
    })),
    [allObjectives, epic?.objectives],
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
    await fetchData(false);
    if (failed) {
      setSaveError(failed.error ?? 'Failed to update objectives');
      setSaving(false);
      return;
    }

    setSaving(false);
  }, [allObjectives, epic, fetchData, workspaceId]);

  // Refresh tasks when global panel updates/archives a task
  useEffect(() => {
    const refresh = () => {
      if (!workspaceId) return;
      pmEpicService.listTasks(workspaceId, epicId).then((res) => {
        if (res.data) setTasks(res.data);
      });
    };
    window.addEventListener('task-panel-updated', refresh);
    window.addEventListener('task-panel-archived', refresh);
    return () => {
      window.removeEventListener('task-panel-updated', refresh);
      window.removeEventListener('task-panel-archived', refresh);
    };
  }, [workspaceId, epicId]);

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

  const renderTasksViewSwitcher = () => {
    if (!delivery.plan) return null;
    return (
      <span className="mr-1 inline-flex h-7 items-center gap-0.5 rounded-md border border-border/70 bg-muted/30 p-0.5">
        <QuickTooltip label="Task list">
          <Button
            variant="ghost"
            size="icon"
            aria-pressed={effectiveTasksView === 'list'}
            className={`h-6 w-6 rounded-sm ${
              effectiveTasksView === 'list'
                ? 'bg-background text-foreground shadow-sm hover:bg-background'
                : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => setTasksView('list')}
          >
            <LayoutTable01Icon className="h-3.5 w-3.5" />
          </Button>
        </QuickTooltip>
        <QuickTooltip label="Delivery">
          <Button
            variant="ghost"
            size="icon"
            aria-pressed={effectiveTasksView === 'delivery'}
            className={`relative h-6 w-6 rounded-sm ${
              effectiveTasksView === 'delivery'
                ? 'bg-background text-foreground shadow-sm hover:bg-background'
                : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => setTasksView('delivery')}
          >
            <BotIcon className="h-3.5 w-3.5" />
            <StatusDot
              state={deliveryDotState(delivery.plan, delivery.runsById)}
              className="absolute -right-0.5 -top-0.5 h-1.5 w-1.5"
            />
          </Button>
        </QuickTooltip>
      </span>
    );
  };

  const renderTaskHeaderAddButton = () => (
    <div className="flex items-center gap-1.5">
      {renderTasksViewSwitcher()}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-7 gap-1.5 px-2 text-xs text-muted-foreground hover:text-foreground"
        onClick={viewEpicTasksPage}
      >
        <ViewIcon className="h-3.5 w-3.5" />
        View on Tasks page
      </Button>
      <Button
        variant="ghost"
        size="sm"
        className="h-7 gap-1.5 px-2 text-xs text-muted-foreground hover:text-foreground"
        onClick={() => void handleStartCreateTask()}
        disabled={!canCreateTask || openingCreateTask}
        title={createTaskDisabledReason ?? undefined}
      >
        {openingCreateTask ? (
          <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <PlusSignIcon className="h-3.5 w-3.5" />
        )}
        Create task
      </Button>
    </div>
  );

  const renderGhostAddTaskRow = (className: string) => (
    <button
      type="button"
      className={className}
      onClick={() => void handleStartCreateTask()}
      disabled={!canCreateTask || openingCreateTask}
      title={createTaskDisabledReason ?? undefined}
    >
      {openingCreateTask ? (
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
      ) : (
        <PlusSignIcon className="h-3.5 w-3.5" />
      )}
      <span>Add task</span>
    </button>
  );

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !epic || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Epic not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft02Icon className="mr-1 h-3.5 w-3.5" />
          Back to Epics
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="ui-divider-bottom-fade flex items-center gap-2 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft02Icon className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <Layers01Icon className="h-3.5 w-3.5 shrink-0 text-violet-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Epics
          </button>
          <ArrowRight01Icon className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving || taskTableSaving} error={saveError || taskTableSaveError} />
          <FollowButton entityType="epic" entityId={epic.epic.id} />
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1.5 text-xs text-muted-foreground"
            onClick={async () => {
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
            }}
          >
            {epic.epic.archived ? <><ArchiveRestoreIcon className="h-3.5 w-3.5" /> Unarchive</> : <><ArchiveIcon className="h-3.5 w-3.5" /> Archive</>}
          </Button>
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div
        className="relative grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]"
        onDragEnter={(e) => {
          e.preventDefault();
          dragCounterRef.current++;
          if (e.dataTransfer.types.includes('Files')) setPanelDragging(true);
        }}
        onDragOver={(e) => e.preventDefault()}
        onDragLeave={() => {
          dragCounterRef.current--;
          if (dragCounterRef.current === 0) setPanelDragging(false);
        }}
        onDrop={(e) => {
          e.preventDefault();
          dragCounterRef.current = 0;
          setPanelDragging(false);
          if (e.dataTransfer.files.length > 0) {
            uploadFilesRef.current?.(e.dataTransfer.files);
          }
        }}
      >
        {panelDragging && (
          <div className="absolute inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
            <div className="flex flex-col items-center gap-2 rounded-xl border-2 border-dashed border-primary px-10 py-8">
              <Upload01Icon className="h-8 w-8 text-primary" />
              <p className="text-sm font-medium text-foreground">Drop files to attach</p>
              <p className="text-xs text-muted-foreground">Max 50MB per file</p>
            </div>
          </div>
        )}
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 pb-24 pt-6">
          {/* Title */}
          <input
            type="text"
            aria-label="Epic title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Description */}
          <div className="mt-4">
            {editingDescription ? (
              <div>
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => updateField('description', html, { description: html })}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none"
                  uploadConfig={{ workspaceId: workspaceId!, entityType: 'editor_upload', entityId: workspaceId! }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  teams={mentionTeams}
                  members={assignableMembers}
                />
                <div className="mt-2 flex justify-end">
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setEditingDescription(false)}>
                    Done
                  </Button>
                </div>
              </div>
            ) : (
                <div className="group/desc relative">
                  {form.description ? (
                    <RichTextMentionContent
                      html={form.description}
                      members={assignableMembers}
                      teams={mentionTeams}
                      className="prose prose-sm dark:prose-invert max-w-none text-sm"
                    />
                  ) : (
                    <p className="text-sm text-muted-foreground">{canEdit ? 'No description yet' : 'No description'}</p>
                  )}
                {canEdit && (
                  <button
                    type="button"
                    className="mt-2 inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                    onClick={() => setEditingDescription(true)}
                  >
                    <PencilEdit01Icon className="h-3 w-3" />
                    Edit description
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Action bar */}
          {canEdit && (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <button
                type="button"
                className={getOptionalSectionActionClass(hasExternalLinkItems ? 'locked' : showExternalLinks ? 'open' : 'available')}
                disabled={hasExternalLinkItems}
                onClick={() => setShowExternalLinks((v) => !v)}
              >
                <Link01Icon className="h-3 w-3" />
                External Links
                {hasExternalLinkItems ? (
                  <span className="text-[10px] opacity-70">{externalLinkCount}</span>
                ) : null}
              </button>
              <button
                type="button"
                className={getOptionalSectionActionClass('available')}
                onClick={() => openFilePickerRef.current?.()}
              >
                <AttachmentIcon className="h-3 w-3" />
                Attach Files
              </button>
            </div>
          )}

          {showExternalLinks && (
            <div className="mt-4">
              <ExternalLinks
                workspaceId={workspaceId!}
                entityType="epic"
                entityId={epic.epic.id}
                onContentChange={handleExternalLinkContentChange}
                onCountChange={handleExternalLinkCountChange}
              />
            </div>
          )}

          <div className="mt-6">
            <Attachments
              workspaceId={workspaceId!}
              entityType="epic"
              entityId={epic.epic.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              onFilePickerReady={(fn) => { openFilePickerRef.current = fn; }}
              onUploadReady={(fn) => { uploadFilesRef.current = fn; }}
            />
          </div>

          {/* Resources */}
          <div className="mt-6">
            <TaskDetailSectionHeading title={`Task owners (${resources.length})`} icon={UserGroupIcon} />
            {resources.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No task owners yet.</p>
            ) : (
              <div className="mt-3 flex flex-wrap gap-2">
                {resources.map((person) => (
                  <div key={person.id} className="inline-flex w-fit max-w-full items-center gap-2 rounded-md border border-border/60 px-3 py-2 sm:max-w-[14rem]">
                    <UserAvatar name={person.name || person.email} className="h-6 w-6 border-border/60" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-xs font-medium">{person.name || person.email}</div>
                      <div className="text-[11px] text-muted-foreground">
                        {person.taskCount} {person.taskCount === 1 ? 'task' : 'tasks'} · {person.percentage}%
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator className="my-6" />

          {/* Tasks */}
          <div>
            <TaskDetailSectionHeading
              title={`Tasks (${tasks.length})`}
              icon={CheckListIcon}
              meta={<div className="ml-auto">{renderTaskHeaderAddButton()}</div>}
            />
            {effectiveTasksView === 'delivery' && delivery.plan && workspaceId ? (
              <div className="mt-3">
                <EpicDeliveryRunsPanel
                  workspaceId={workspaceId}
                  plan={delivery.plan}
                  runsById={delivery.runsById}
                  onReload={delivery.reload}
                  canEdit={canEdit}
                />
              </div>
            ) : tasks.length === 0 ? (
              <div className="mt-3 overflow-hidden rounded-lg border border-border/60 bg-card">
                <div className="px-3 py-3">
                  <p className="text-sm text-muted-foreground">No tasks linked yet.</p>
                </div>
                {renderGhostAddTaskRow(
                  'flex h-9 w-full items-center gap-2 border-t border-dashed border-border/60 px-3 text-sm text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground disabled:cursor-not-allowed disabled:opacity-60',
                )}
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
                  onBulkOperationComplete={() => fetchData(false)}
                />
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Loading workflow...</p>
            )}
          </div>

          <Separator className="my-6" />

          {/* AI Agents */}
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

          <div className={comments.length > 0 ? 'mt-10' : 'mt-8'}>
            {commentsLoading ? (
              <div className="space-y-3 rounded-lg border border-border/60 p-4">
                {[1, 2].map((i) => (
                  <div key={i} className="flex items-start gap-3">
                    <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
                    <div className="flex-1 space-y-2">
                      <Skeleton className="h-3 w-32" />
                      <Skeleton className="h-3 w-full" />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <>
                <TaskDetailSectionHeading title="Comments" icon={Message01Icon} className="mb-3" />
                {workspaceId ? (
                  <CommentThread
                    workspaceId={workspaceId}
                    entityType="epic"
                    entityId={epicId}
                    comments={comments}
                    currentUserId={currentUser?.id}
                    teams={mentionTeams}
                    members={assignableMembers}
                    onCommentsChange={setComments}
                    hideEmptyState
                  />
                ) : null}
              </>
            )}

            {activityLoading ? (
              <div className="mt-6 space-y-3">
                <Skeleton className="h-3 w-20" />
                {[1, 2, 3].map((i) => (
                  <div key={i} className="flex items-center gap-3">
                    <Skeleton className="h-4 w-4 shrink-0 rounded-full" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                ))}
              </div>
            ) : null}
            {!activityLoading && activity.length > 0 && (
              <div className="mt-6">
                <TaskDetailSectionHeading title="Activity" icon={Activity01Icon} />
                <ActivityTimeline
                  activity={activity}
                  showAll={showAllActivity}
                  onShowAll={() => setShowAllActivity(true)}
                  entityLabel="epic"
                />
              </div>
            )}
          </div>

        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <section className="rounded-lg border border-emerald-500/15 bg-emerald-500/[0.035] p-3">
            <div className="flex items-start justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-emerald-500/10 text-emerald-600">
                  <ChartColumnIcon className="h-3.5 w-3.5" />
                </span>
                <div className="min-w-0">
                  <h3 className="text-xs font-semibold text-foreground">Progress</h3>
                  <p className="mt-0.5 text-[11px] text-muted-foreground">
                    {totalTasks > 0 ? `${doneTasks}/${totalTasks} tasks complete` : 'No tasks yet'}
                  </p>
                </div>
              </div>
              <span className="text-lg font-semibold tabular-nums leading-none text-foreground">{progress}%</span>
            </div>
            <Progress value={progress} className="mt-3 h-2 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
            <div className="mt-2 flex items-center justify-between gap-3 text-[11px] text-muted-foreground">
              <span className="tabular-nums">{doneTasks} done</span>
              <span className="tabular-nums">{remainingTasks} remaining</span>
            </div>
          </section>

          {delivery.plan ? (
            <EpicDeliveryStatusChip
              plan={delivery.plan}
              runsById={delivery.runsById}
              onClick={() => {
                setTasksView('delivery');
                // The panel mounts only after the view switches.
                requestAnimationFrame(() => {
                  document
                    .getElementById(EPIC_DELIVERY_PANEL_ID)
                    ?.scrollIntoView({ behavior: 'smooth', block: 'center' });
                });
              }}
            />
          ) : null}

          <div className="mt-5 grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
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
            <>
              <AssociationsPanel objectType="epic" objectId={epicId} workspaceId={workspaceId} className="-mx-4 mt-4 border-t border-border/60" />
            </>
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
