import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import {
  Activity01Icon,
  ArchiveIcon,
  ArrowUpRight01Icon,
  Copy01Icon,
  DashboardSpeed01Icon,
  File01Icon,
  GitBranchIcon,
  HashtagIcon,
  Layers01Icon,
  LayoutGridIcon,
  ArrowRight01Icon,
  Link01Icon,
  Loading01Icon,
  MoreVerticalIcon,
  AttachmentIcon,
  CheckListIcon,
  PlayIcon,
  Shield02Icon,
  Tag01Icon,
  Target01Icon,
  UserIcon,
  UserGroupIcon,
  Cancel01Icon,
} from '@/lib/icons';
import { ArrowReloadHorizontalIcon, Calendar03Icon, Tick01Icon } from '@/lib/pmIcons';
import {
  PRIORITY_CONFIG,
  PriorityIcon,
  SEVERITY_CONFIG,
  SeverityIcon,
  SprintIcon,
  TASK_TYPE_CONFIG,
  TaskTypeIcon,
  getWorkflowStateTone,
} from '@/lib/pmConstants';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  QuietBreadcrumbs,
  QuietDetailAction,
  QuietDetailHeader,
  QuietIconAction,
  QuietMetaLine,
  QuietStatusBadge,
  QuietTitleInput,
} from '@/components/design-system/quiet';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Separator } from '@/components/ui/separator';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { Attachments } from '@/components/pm/Attachments';
import { ChecklistItems } from '@/components/pm/ChecklistItems';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { useTaskDelivery } from '@/components/pm/TaskDeliveryPanel';
import { AgentRunPanel, getTaskAgentRunExecutionContextLockReason } from '@/components/pm/AgentRunPanel';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { cn } from '@/lib/utils';
import { gitService } from '@/lib/services/gitService';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { DatePicker } from '@/components/ui/date-picker';
import { EstimatePicker } from '@/components/pm/EstimatePicker';
import { EpicBadge } from '@/components/pm/EpicBadge';
import { EpicColorSwatch } from '@/components/pm/EpicColorSwatch';
import { EPIC_PICKER_WIDTH, groupEpicsByLifecycle } from '@/components/pm/epicPickerGroups';
import { MemberPickerPopover, MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { OwnerAvatarStack } from '@/components/pm/OwnerAvatarStack';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TaskSidebarIdRow } from '@/components/pm/TaskSidebarIdRow';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useInitializeTaskUpdatesRead, useTaskUpdates, useTeamFieldVisibilityForTeam, useAutomationRulesByWorkflow, useWorkspaceAccess, usePermissions, useWorkflows } from '@/hooks/queries';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { buildTaskCopyUrl, buildTaskPath } from '@/lib/pmTaskLinks';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { TaskRelationshipsSection } from '@/components/pm/TaskRelationshipsSection';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { shouldSuppressTaskOverlayOutsideDismiss } from '@/components/pm/task-detail/taskOverlayDismiss';
import { isInsideAskAgentsDock } from '@/lib/agentsDockGuard';
import { getFlushablePendingTaskPatch, hasPendingTaskSave } from '@/components/pm/task-detail/taskPendingPatch';
import { getTaskPatchSignature, isBlockedTaskPatch } from '@/components/pm/task-detail/taskAutosaveFailure';
import { queryKeys } from '@/lib/queryKeys';
import { isSprintOpenForPlanning } from '@/lib/pmSprintOptions';
import { repositoryDefaultBranchLabel } from '@/lib/branchLabels';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { TaskStateSelectContent } from '@/components/pm/task-detail/TaskStateSelectContent';
import { TaskUpdatesView } from '@/components/pm/task-detail/TaskUpdatesView';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';
import { DetailDescriptionEditorActions } from '@/components/pm/DetailDescriptionEditorActions';
import { DetailDescriptionEditButton } from '@/components/pm/DetailDescriptionEditButton';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { resolveTaskTeamWorkflow, resolveTaskWorkflowStates } from '@/components/pm/task-detail/taskWorkflowResolution';
import {
  isEpicSelectableForTaskTeam,
  isSprintSelectableForTaskTeam,
} from '@/components/pm/task-detail/taskPlanningScope';
import { syncTaskLabelsWithFeedback } from '@/components/pm/task-detail/taskLabelSync';
import {
  getAgentAutoRunStateChangeMessage,
  getAgentAutoRunStateChangeToastId,
  shouldNotifyAgentAutoRunStateChange,
} from '@/components/pm/agentAutoRunNotification';
import type {
  CommentWithAuthor,
  EpicWithStats,
  AttachmentResponse,
  AgentRunStatus,
  SprintWithStats,
  Label,
  Priority,
  RecurringTemplateDetail,
  Severity,
  TaskDetail,
  TaskRecurringSummary,
  TaskType,
  TaskDetailView,
  UpdateTaskRequest,
  WorkflowState,
} from '@/lib/pmTypes';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';

// ── Types ──────────────────────────────────────────────────────────

interface TaskDetailPanelProps {
  workspaceId: string;
  open: boolean;
  loading?: boolean;
  onOpenChange: (open: boolean) => void;
  taskDetail: TaskDetail | null;
  states: WorkflowState[];
  initialRecurringSummary?: TaskRecurringSummary | null;
  onTaskUpdated: (task: TaskDetail) => void;
  onTaskOpened: (task: TaskDetail) => void;
  onTaskArchived: (taskId: string) => void;
}

interface FormState {
  name: string;
  description: string;
  task_type: TaskType;
  workflow_id: string;
  workflow_state_id: string;
  priority: Priority;
  severity: Severity;
  estimate: string;
  deadline: string;
  epic_id: string;
  sprint_id: string;
  team_id: string;
  owner_member_ids: string[];
  requester_member_id: string;
  blocker: string;
  assigned_agent_id: string;
}

interface DuplicateNoticeState {
  taskDetail: TaskDetail;
}

// ── Constants ──────────────────────────────────────────────────────

const priorityOptions: Priority[] = ['none', 'low', 'medium', 'high', 'urgent'];
const severityOptions: Severity[] = ['none', 'minor', 'major', 'critical'];
const taskTypeOptions: TaskType[] = ['feature', 'bug', 'chore'];

const buildFormState = (detail: TaskDetail): FormState => ({
  name: detail.task.name,
  description: detail.task.description ?? '',
  task_type: detail.task.task_type,
  workflow_id: detail.task.workflow_id,
  workflow_state_id: detail.task.workflow_state_id,
  priority: detail.task.priority,
  severity: detail.task.severity,
  estimate:
    detail.task.estimate === undefined || detail.task.estimate === null
      ? ''
      : String(detail.task.estimate),
  deadline: detail.task.deadline ? detail.task.deadline.slice(0, 10) : '',
  epic_id: detail.task.epic_id ?? '',
  sprint_id: detail.task.sprint_id ?? '',
  team_id: detail.task.team_id ?? '',
  owner_member_ids: detail.task.owner_member_ids ?? [],
  requester_member_id: detail.task.requester_member_id ?? '',
  blocker: detail.task.blocker ?? '',
  assigned_agent_id: detail.task.assigned_agent_id ?? '',
});

const isInsideSonnerToast = (target: EventTarget | null) => (
  target instanceof HTMLElement && Boolean(target.closest('[data-sonner-toast], [data-sonner-toaster]'))
);

// ── Helpers ────────────────────────────────────────────────────────

function hasDraggedFiles(event: DragEvent) {
  return event.dataTransfer.types.includes('Files');
}

// ── Metadata Row ───────────────────────────────────────────────────

function MetadataRow({
  icon: Icon,
  label,
  children,
}: {
  icon: React.ElementType;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground mt-0.5" />
      <span className="text-[12px] text-muted-foreground mt-0.5">{label}</span>
      <div className="min-w-0 text-[12px]">{children}</div>
    </>
  );
}

type TaskDeliveryContext = ReturnType<typeof useTaskDelivery>;

function TaskDeliveryRailSection({
  workspaceId,
  delivery,
  open,
  onOpenChange,
  canEdit,
  lockReason,
}: {
  workspaceId: string;
  delivery: TaskDeliveryContext;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  canEdit: boolean;
  lockReason: string | null;
}) {
  const repositoryName = delivery.selectedRepository?.full_name ?? delivery.target?.repo_full_name ?? '';
  const defaultRepository = delivery.repositories.find((repository) => repository.selected && repository.active && !repository.archived)
    ?? delivery.repositories.find((repository) => repository.active && !repository.archived)
    ?? delivery.repositories[0]
    ?? null;
  const controlsDisabled = !canEdit || Boolean(lockReason) || delivery.savingTarget;
  const canUseDefaultRepository = !controlsDisabled && !delivery.repositoryId && Boolean(defaultRepository);
  const summary = delivery.loading ? 'Loading…' : repositoryName || 'Not configured';

  return (
    <section
      id="task-delivery-section"
      data-testid="task-delivery-section"
      className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4"
    >
      <button
        type="button"
        className="flex w-full items-center gap-1.5 text-left"
        aria-expanded={open}
        aria-controls="task-delivery-settings"
        onClick={() => onOpenChange(!open)}
      >
        <ArrowRight01Icon className={cn('h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
        <span className="shrink-0 text-xs font-semibold uppercase tracking-wide text-foreground/70">
          Delivery
        </span>
        {delivery.savingTarget ? <Loading01Icon className="h-3 w-3 shrink-0 animate-spin text-muted-foreground" /> : null}
        <span className="ml-auto min-w-0 truncate text-[11px] text-muted-foreground">
          {summary}
        </span>
      </button>

      {open ? (
        <div id="task-delivery-settings" data-testid="task-delivery-settings" className="mt-3">
          {delivery.loading ? (
            <div className="flex items-center gap-2 py-1 text-xs text-muted-foreground">
              <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
              Loading delivery settings…
            </div>
          ) : (
            <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
              {lockReason ? (
                <p className="col-span-3 text-[11px] leading-4 text-muted-foreground" role="status">
                  {lockReason}
                </p>
              ) : null}

              <MetadataRow icon={GitBranchIcon} label="Repository">
                <SidebarPopoverSelect
                  value={delivery.repositoryId || '__none__'}
                  options={delivery.repositories.map((repository) => ({ value: repository.id, label: repository.full_name }))}
                  onChange={(value) => { void delivery.handleRepoChange(value); }}
                  renderTrigger={() => (
                    <span className="block min-w-0 truncate">
                      {repositoryName || 'None'}
                    </span>
                  )}
                  disabled={controlsDisabled}
                  width="w-72"
                  searchPlaceholder="Search repositories…"
                  emptyContent={<span className="px-2 py-1.5 text-xs text-muted-foreground">No repositories available</span>}
                />
              </MetadataRow>

              {canUseDefaultRepository ? (
                <MetadataRow icon={GitBranchIcon} label="Default">
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={delivery.savingTarget}
                    onClick={() => { void delivery.handleRepoChange(defaultRepository!.id); }}
                    title={`Use ${defaultRepository!.full_name}`}
                  >
                    Use default
                  </Button>
                </MetadataRow>
              ) : null}

              <MetadataRow icon={GitBranchIcon} label="Base branch">
                <RepositoryBranchPicker
                  workspaceId={workspaceId}
                  repositoryId={delivery.repositoryId || undefined}
                  value={delivery.baseBranch}
                  onChange={(value) => { void delivery.handleBaseBranchChange(value); }}
                  placeholder={delivery.selectedRepository?.default_branch || 'main'}
                  emptyLabel={repositoryDefaultBranchLabel(delivery.selectedRepository?.default_branch)}
                  extraOptions={delivery.branchOptions}
                  disabled={controlsDisabled}
                  variant="sidebar"
                  width="w-72"
                />
              </MetadataRow>

              <MetadataRow icon={GitBranchIcon} label="Task branch">
                <span className="block min-w-0 truncate px-1.5 py-0.5 font-mono text-xs">
                  {delivery.branchPreview}
                </span>
              </MetadataRow>

              {delivery.deliveryStateCfg ? (
                <MetadataRow icon={PlayIcon} label="Status">
                  <span className={cn('inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium leading-none', delivery.deliveryStateCfg.className)}>
                    {delivery.deliveryStateCfg.label}
                  </span>
                </MetadataRow>
              ) : null}

              {delivery.canUseEpicTarget ? (
                <MetadataRow icon={GitBranchIcon} label="Epic branch">
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={controlsDisabled}
                    onClick={() => { void delivery.handleUseEpicTarget(); }}
                  >
                    {delivery.savingTarget ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <GitBranchIcon className="h-3 w-3" />}
                    Use epic branch
                  </Button>
                </MetadataRow>
              ) : null}
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}


// ── Main Body ──────────────────────────────────────────────────────

function TaskDetailPanelBody({
  workspaceId,
  taskDetail,
  states,
  initialRecurringSummary,
  panelOpen,
  onOpenChange,
  onTaskUpdated,
  onTaskOpened,
  onTaskArchived,
}: {
  workspaceId: string;
  taskDetail: TaskDetail;
  states: WorkflowState[];
  initialRecurringSummary: TaskRecurringSummary | null;
  panelOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onTaskUpdated: (task: TaskDetail) => void;
  onTaskOpened: (task: TaskDetail) => void;
  onTaskArchived: (taskId: string) => void;
}) {
  const confirm = useConfirm();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [form, setForm] = useState<FormState>(() => buildFormState(taskDetail));
  const [pendingPatch, setPendingPatch] = useState<UpdateTaskRequest>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [labelSaving, setLabelSaving] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const pendingPatchRef = useRef<UpdateTaskRequest>({});
  const descriptionPendingUploadsRef = useRef(0);
  const blockedAutosavePatchSignatureRef = useRef<string | null>(null);
  const { copied: linkCopied, copy: copyText } = useCopyToClipboard();
  const { copied: duplicateKeyCopied, copy: copyDuplicateKey } = useCopyToClipboard();
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [duplicateConfirmOpen, setDuplicateConfirmOpen] = useState(false);
  const [duplicateNotice, setDuplicateNotice] = useState<DuplicateNoticeState | null>(null);
  const [duplicating, setDuplicating] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const [descriptionDragging, setDescriptionDragging] = useState(false);
  const [hasGitIntegration, setHasGitIntegration] = useState(false);
  const [recurringSummary, setRecurringSummary] = useState<TaskRecurringSummary | null>(initialRecurringSummary);
  const [recurringDetail, setRecurringDetail] = useState<RecurringTemplateDetail | null>(null);
  const [recurringDialogOpen, setRecurringDialogOpen] = useState(false);
  const [recurringSaving, setRecurringSaving] = useState(false);
  const descriptionUploadRef = useRef<((files: FileList | File[], insertPos?: number) => Promise<void>) | null>(null);
  const queuedDescriptionDropRef = useRef<File[] | null>(null);
  const descriptionDragCounterRef = useRef(0);
  const descriptionEditStartRef = useRef(form.description);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, form.team_id);
  const { data: workflows = [] } = useWorkflows(workspaceId);
  const { data: workspaceAccess } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(workspaceAccess);
  const { canEdit } = permissions;
  const taskId = taskDetail.task.id;
  const [activeView, setActiveView] = useState<TaskDetailView>(() => {
    const value = new URLSearchParams(window.location.search).get('task_view');
    return value === 'delivery' ? 'delivery' : 'overview';
  });
  const updatesSummary = useTaskUpdates(workspaceId, taskId, 'all');
  const initializeUpdatesRead = useInitializeTaskUpdatesRead(workspaceId, taskId);
  const canSaveAsTemplate = permissions.isAdmin || (!!taskDetail.task.team_id && permissions.isTeamManager(taskDetail.task.team_id));
  const [saveTemplateDialogOpen, setSaveTemplateDialogOpen] = useState(false);
  const [saveTemplateName, setSaveTemplateName] = useState(taskDetail.task.name);
  const [saveTemplateSaving, setSaveTemplateSaving] = useState(false);
  const [deliverySectionOpen, setDeliverySectionOpen] = useState(false);

  useEffect(() => {
    if (panelOpen) setDeliverySectionOpen(false);
  }, [panelOpen, taskId]);

  const selectView = useCallback((view: TaskDetailView) => {
    setActiveView(view);
    const url = new URL(window.location.href);
    if (view === 'overview') url.searchParams.delete('task_view');
    else url.searchParams.set('task_view', view);
    window.history.replaceState({}, '', url.toString());
  }, []);

  const openDelivery = useCallback((runId?: string | null) => {
    setActiveView('delivery');
    navigate({
      to: '.',
      search: (previous: Record<string, unknown>) => ({
        ...previous,
        task_view: 'delivery',
        run: runId || undefined,
      }),
      replace: true,
    } as any);
  }, [navigate]);

  const deliverySidebarRef = useRef<HTMLElement>(null);

  const openDeliveryContext = useCallback(() => {
    setDeliverySectionOpen(true);
    window.requestAnimationFrame(() => {
      const section = deliverySidebarRef.current?.querySelector<HTMLElement>('#task-delivery-section');
      section?.scrollIntoView({ block: 'nearest' });
      section?.querySelector<HTMLElement>('#task-delivery-settings button:not([disabled])')?.focus({ preventScroll: true });
    });
  }, []);

  useEffect(() => {
    const onPopState = () => {
      const value = new URLSearchParams(window.location.search).get('task_view');
      setActiveView(value === 'delivery' ? 'delivery' : 'overview');
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    const updates = updatesSummary.data;
    if (!updates?.high_water || updates.read_initialized || initializeUpdatesRead.isPending) return;
    initializeUpdatesRead.mutate(updates.high_water);
  }, [initializeUpdatesRead, updatesSummary.data]);

  useEffect(() => {
    pendingPatchRef.current = pendingPatch;
  }, [pendingPatch]);

  useEffect(() => {
    descriptionPendingUploadsRef.current = descriptionPendingUploads;
  }, [descriptionPendingUploads]);

  // Check if GitHub is connected
  useEffect(() => {
    gitService.listIntegrations(workspaceId).then((res) => {
      setHasGitIntegration((res.data ?? []).some((i) => i.active));
    });
  }, [workspaceId]);

  const loadRecurringSummary = useCallback(async (currentTaskId: string) => {
    const { data } = await pmRecurringTemplateService.getByTask(workspaceId, currentTaskId);
    setRecurringSummary(data ?? null);
  }, [workspaceId]);

  const openRecurringDialog = useCallback(async () => {
    if (recurringSummary?.template_id) {
      const { data, error } = await pmRecurringTemplateService.get(workspaceId, recurringSummary.template_id);
      if (error || !data) {
        toast.error(error ?? 'Failed to load recurring template');
        return;
      }
      setRecurringDetail(data);
    } else {
      setRecurringDetail(null);
    }
    setRecurringDialogOpen(true);
  }, [workspaceId, recurringSummary?.template_id]);

  const openSaveTemplateDialog = useCallback(() => {
    setSaveTemplateName(taskDetail.task.name);
    setSaveTemplateDialogOpen(true);
  }, [taskDetail.task.name]);

  const handleSaveAsTemplate = useCallback(async () => {
    const name = saveTemplateName.trim();
    if (!name || saveTemplateSaving) return;
    setSaveTemplateSaving(true);
    const { error } = await pmTaskService.saveAsTemplate(workspaceId, taskDetail.task.id, { name });
    setSaveTemplateSaving(false);
    if (error) {
      toast.error(error);
      return;
    }
    setSaveTemplateDialogOpen(false);
    toast.success('Template created');
    queryClient.invalidateQueries({ queryKey: queryKeys.pm.templates(workspaceId) });
  }, [queryClient, saveTemplateName, saveTemplateSaving, taskDetail.task.id, workspaceId]);

  const handleRecurringSubmit = useCallback(async (value: RecurringTemplateFormValue) => {
    setRecurringSaving(true);
    const response = recurringSummary?.template_id
      ? await pmRecurringTemplateService.update(workspaceId, recurringSummary.template_id, {
          title: value.title,
          description: value.description || undefined,
          task_id: taskDetail.task.id,
          config: value.config,
        })
      : await pmRecurringTemplateService.create({
          workspace_id: workspaceId,
          task_id: taskDetail.task.id,
          title: value.title,
          description: value.description || undefined,
          config: value.config,
        });
    setRecurringSaving(false);
    if (response.error || !response.data) {
      toast.error(response.error ?? 'Failed to save recurring template');
      return;
    }
    setRecurringDetail(response.data);
    await loadRecurringSummary(taskDetail.task.id);
    setRecurringDialogOpen(false);
    toast.success(recurringSummary?.template_id ? 'Recurring template updated' : 'Task is now recurring');
  }, [workspaceId, taskDetail.task.id, recurringSummary?.template_id, loadRecurringSummary]);


  // ── Delivery (sidebar rows) ──────────────────────────────────────
  const delivery = useTaskDelivery(workspaceId, taskDetail, onTaskUpdated);
  const deliveryLockReason = useMemo(() => {
    const status = taskDetail.task.latest_run_status as AgentRunStatus | null | undefined;
    if (!status || !ACTIVE_RUN_STATUSES.has(status)) return null;
    return getTaskAgentRunExecutionContextLockReason({
      activeRun: {
        status,
        pause_reason: taskDetail.task.latest_run_pause_reason ?? 'none',
        approval_state: 'not_required',
      },
      activeRunAgentName: null,
    });
  }, [taskDetail.task.latest_run_pause_reason, taskDetail.task.latest_run_status]);

  // Re-sync form when taskDetail changes externally (e.g. real-time WS update)
  const lastSyncedAt = useRef(taskDetail.task.updated_at);
  const savedDescriptionRef = useRef(taskDetail.task.description ?? '');
  const savedWorkflowStateIdRef = useRef(taskDetail.task.workflow_state_id);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [commentsLoading, setCommentsLoading] = useState(true);

  const currentUser = useAuthStore((s) => s.user);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const [relationshipComposerOpen, setRelationshipComposerOpen] = useState(false);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const memberNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const teamSprintsEnabled = useMemo(() => teams.find((t) => t.id === form.team_id)?.sprints_enabled !== false, [teams, form.team_id]);
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form.team_id ? [form.team_id] : []),
    [teams, form.team_id],
  );
  const lastSyncedTaskRef = useRef<string | null>(null);
  useEffect(() => {
    if (taskDetail.task.id === lastSyncedTaskRef.current) {
      return;
    }
    lastSyncedTaskRef.current = taskDetail.task.id;
    const url = new URL(window.location.href);
    url.searchParams.set('task', taskDetail.task.task_key ?? `${taskDetail.task.display_id}`);
    window.history.replaceState({}, '', url.toString());
  }, [taskDetail.task.display_id, taskDetail.task.id, taskDetail.task.task_key]);

  // ── Load comments + activity ───────────────────────────────────
  const reloadComments = useCallback(async () => {
    const res = await pmCommentService.list(workspaceId, 'task', taskDetail.task.id);
    setComments(res.data ?? []);
    setCommentsLoading(false);
  }, [workspaceId, taskDetail.task.id]);

  const reloadActivity = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks', taskDetail.task.id, 'updates'] });
  }, [queryClient, workspaceId, taskDetail.task.id]);

  useEffect(() => {
    reloadComments();
    reloadActivity();
  }, [reloadComments, reloadActivity]);

  useEffect(() => {
    if (taskDetail.task.updated_at !== lastSyncedAt.current) {
      lastSyncedAt.current = taskDetail.task.updated_at;
      savedDescriptionRef.current = taskDetail.task.description ?? '';
      savedWorkflowStateIdRef.current = taskDetail.task.workflow_state_id;
      void reloadActivity();
      // Only reset form if no unsaved edits
      if (Object.keys(pendingPatchRef.current).length === 0) {
        setForm(buildFormState(taskDetail));
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- use ref for pendingPatch to avoid resetting form mid-edit
  }, [taskDetail, reloadActivity]);

  // Re-fetch comments when comment events arrive; activity on any task change
  useEffect(() => {
    const taskId = taskDetail.task.id;
    const onChildEvent = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === taskId && d?.entity === 'comment') {
        reloadComments();
        reloadActivity();
      }
    };
    const onTaskEvent = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.entity_id === taskId) reloadActivity();
    };
    window.addEventListener('task-child-updated', onChildEvent);
    window.addEventListener('task-updated', onTaskEvent);
    return () => {
      window.removeEventListener('task-child-updated', onChildEvent);
      window.removeEventListener('task-updated', onTaskEvent);
    };
  }, [taskDetail.task.id, reloadComments, reloadActivity]);

  // ── Load epics, sprints, labels ─────────────────────────────
  useEffect(() => {
    (async () => {
      const [epicsRes, sprintsRes, labelsRes] = await Promise.all([
        pmEpicService.list(workspaceId, { archived: false }),
        pmSprintService.list(workspaceId, { archived: false }),
        pmLabelService.list(workspaceId),
      ]);
      setEpics(epicsRes.data ?? []);
      setSprints(sprintsRes.data ?? []);
      setAllLabels(labelsRes.data ?? []);
    })();
  }, [workspaceId]);

  const queuePatch = (patch: UpdateTaskRequest) => {
    setPendingPatch((current) => {
      const next = { ...current, ...patch };
      if (!isBlockedTaskPatch(next, blockedAutosavePatchSignatureRef.current)) {
        blockedAutosavePatchSignatureRef.current = null;
      }
      return next;
    });
  };

  // ── Pipeline automation rules ──────────────────────────────────
  const effectiveStates = useMemo(
    () => resolveTaskWorkflowStates(workflows, states, form.workflow_id),
    [form.workflow_id, states, workflows],
  );
  const workflowId = effectiveStates[0]?.workflow_id;
  const { data: pipelineRules } = useAutomationRulesByWorkflow(workspaceId, workflowId);
  const automatedStateIds = useMemo(() => {
    const ids = new Set<string>();
    if (!pipelineRules) return ids;
    for (const rule of pipelineRules) {
      if (
        rule.enabled &&
        rule.trigger_type === 'task.state_entered' &&
        rule.action_type === 'start_agent_run'
      ) {
        const stateId = rule.trigger_config?.state_id;
        if (stateId) ids.add(stateId);
      }
    }
    return ids;
  }, [pipelineRules]);
  const notifyAgentAutoRunStateChange = useCallback((fromStateId: string | null | undefined, toStateId: string | null | undefined) => {
    if (!shouldNotifyAgentAutoRunStateChange({ fromStateId, toStateId, automatedStateIds })) return;
    if (!toStateId) return;
    const stateName = effectiveStates.find((state) => state.id === toStateId)?.name ?? 'this state';
    toast.info(getAgentAutoRunStateChangeMessage(stateName), { id: getAgentAutoRunStateChangeToastId(toStateId) });
  }, [automatedStateIds, effectiveStates]);

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateTaskRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

  const beginDescriptionEditing = useCallback(() => {
    descriptionEditStartRef.current = form.description;
    setEditingDescription(true);
  }, [form.description]);

  const cancelDescriptionEditing = () => {
    const initialDescription = descriptionEditStartRef.current;
    if (form.description !== initialDescription) {
      updateField('description', initialDescription, { description: initialDescription });
    }
    setEditingDescription(false);
  };

  const resetDescriptionDrag = useCallback(() => {
    descriptionDragCounterRef.current = 0;
    setDescriptionDragging(false);
  }, []);

  const handleDescriptionUploadReady = useCallback((upload: ((files: FileList | File[], insertPos?: number) => Promise<void>) | null) => {
    descriptionUploadRef.current = upload;
    const queuedFiles = queuedDescriptionDropRef.current;
    if (!upload || !queuedFiles?.length) return;
    queuedDescriptionDropRef.current = null;
    void upload(queuedFiles);
  }, []);

  const handleDescriptionDragEnter = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    descriptionDragCounterRef.current++;
    setDescriptionDragging(true);
  }, []);

  const handleDescriptionDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
  }, []);

  const handleDescriptionDragLeave = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    descriptionDragCounterRef.current = Math.max(0, descriptionDragCounterRef.current - 1);
    if (descriptionDragCounterRef.current === 0) {
      setDescriptionDragging(false);
    }
  }, []);

  const handleDescriptionDrop = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!hasDraggedFiles(event)) return;
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
  }, [beginDescriptionEditing, resetDescriptionDrag]);

  const availableEpics = useMemo(
    () =>
      epics.filter((entry) =>
        isEpicSelectableForTaskTeam(entry.epic.team_id ?? null, form.team_id || null),
      ),
    [epics, form.team_id],
  );

  const availableSprints = useMemo(
    () =>
      sprints.filter((entry) =>
        (isSprintOpenForPlanning(entry.sprint.status) || entry.sprint.id === form.sprint_id)
        && isSprintSelectableForTaskTeam(entry.sprint.team_id ?? null, form.team_id || null),
      ),
    [form.sprint_id, form.team_id, sprints],
  );

  // Group epics by lifecycle: not started → in progress → completed.
  // Order within each group matches `availableEpics` (server-supplied order).
  const epicGroups = useMemo(() => {
    return [
      { label: undefined as string | undefined, options: [{ value: '__none__', label: 'None' }] },
      ...groupEpicsByLifecycle(availableEpics.map(({ epic }) => epic)).map((group) => ({
        label: group.label,
        options: group.epics.map((epic) => ({ value: epic.id, label: epic.name })),
      })),
    ];
  }, [availableEpics]);

  // Group sprints by lifecycle status: unstarted → started → done.
  const sprintGroups = useMemo(() => {
    const unstarted: typeof availableSprints = [];
    const started: typeof availableSprints = [];
    const done: typeof availableSprints = [];
    for (const entry of availableSprints) {
      if (entry.sprint.status === 'done') done.push(entry);
      else if (entry.sprint.status === 'started') started.push(entry);
      else unstarted.push(entry);
    }
    return [
      { label: undefined as string | undefined, options: [{ value: '__none__', label: 'None' }] },
      { label: 'Not started', options: unstarted.map((s) => ({ value: s.sprint.id, label: s.sprint.name })) },
      { label: 'In progress', options: started.map((s) => ({ value: s.sprint.id, label: s.sprint.name })) },
      { label: 'Completed', options: done.map((s) => ({ value: s.sprint.id, label: s.sprint.name })) },
    ];
  }, [availableSprints]);

  // Track the team_id we've already validated against so the planning-link
  // cleanup only fires when the user actually changes the team — not on initial
  // mount or when the epics/sprints lists finish loading. Without this, a task
  // imported with a sprint/epic whose team scope doesn't match the task's team
  // (e.g. workspace-level sprint on a team task, or vice versa) would silently
  // unset its sprint/epic the first time the panel renders.
  const validatedTeamIdRef = useRef<string | null>(null);

  useEffect(() => {
    if (validatedTeamIdRef.current === null) {
      validatedTeamIdRef.current = form.team_id;
      return;
    }
    if (validatedTeamIdRef.current === form.team_id) return;
    validatedTeamIdRef.current = form.team_id;

    if (form.epic_id) {
      const selectedEpic = epics.find((entry) => entry.epic.id === form.epic_id);
      if (selectedEpic && !isEpicSelectableForTaskTeam(selectedEpic.epic.team_id ?? null, form.team_id || null)) {
        updateField('epic_id', '', { epic_id: '' });
      }
    }

    if (form.sprint_id) {
      const selectedSprint = sprints.find((entry) => entry.sprint.id === form.sprint_id);
      if (selectedSprint && !isSprintSelectableForTaskTeam(selectedSprint.sprint.team_id ?? null, form.team_id || null)) {
        updateField('sprint_id', '', { sprint_id: '' });
      }
    }
  }, [form.team_id, form.epic_id, form.sprint_id, epics, sprints]);

  // ── Auto-save debounce ─────────────────────────────────────────
  useEffect(() => {
    const flushablePatch = getFlushablePendingTaskPatch(pendingPatch, descriptionPendingUploads);
    if (
      saving ||
      !flushablePatch ||
      isBlockedTaskPatch(flushablePatch, blockedAutosavePatchSignatureRef.current)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = flushablePatch;
      const previousDescription = savedDescriptionRef.current;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmTaskService.update(workspaceId, taskId, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save changes');
        blockedAutosavePatchSignatureRef.current = getTaskPatchSignature(patch);
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        blockedAutosavePatchSignatureRef.current = null;
        setSaveError(null);
        onTaskUpdated(data);
        if (patch.workflow_state_id !== undefined) {
          notifyAgentAutoRunStateChange(savedWorkflowStateIdRef.current, data.task.workflow_state_id);
          savedWorkflowStateIdRef.current = data.task.workflow_state_id;
        }
        void reloadActivity();
        // Invalidate sprint planning if sprint/state/estimate changed
        if (patch.sprint_id !== undefined || patch.workflow_state_id !== undefined || patch.estimate !== undefined) {
          queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints', 'planning'] });
        }
        const nextDescription = data.task.description ?? '';
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
  }, [
    descriptionPendingUploads,
    onTaskUpdated,
    notifyAgentAutoRunStateChange,
    pendingPatch,
    queryClient,
    reloadActivity,
    saving,
    taskId,
    workspaceId,
  ]);

  useEffect(() => {
    return () => {
      const patch = getFlushablePendingTaskPatch(
        pendingPatchRef.current,
        descriptionPendingUploadsRef.current,
      );
      if (!patch) {
        return;
      }
      if (isBlockedTaskPatch(patch, blockedAutosavePatchSignatureRef.current)) {
        return;
      }

      void pmTaskService.update(workspaceId, taskId, patch).then(({ data }) => {
        if (!data) {
          return;
        }

        onTaskUpdated(data);
        if (patch.workflow_state_id !== undefined) {
          notifyAgentAutoRunStateChange(savedWorkflowStateIdRef.current, data.task.workflow_state_id);
          savedWorkflowStateIdRef.current = data.task.workflow_state_id;
        }
        if (
          patch.sprint_id !== undefined ||
          patch.workflow_state_id !== undefined ||
          patch.estimate !== undefined
        ) {
          queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints', 'planning'] });
        }
      });
    };
  }, [notifyAgentAutoRunStateChange, onTaskUpdated, queryClient, taskId, workspaceId]);

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: AttachmentResponse) => {
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

      setForm((current) => ({ ...current, description: nextDescription }));
      setPendingPatch((current) => {
        const { description, ...rest } = current;
        return rest;
      });
      setSaving(true);

      const { data, error } = await pmTaskService.update(workspaceId, taskId, {
        description: nextDescription,
      });
      if (error || !data) {
        setForm((current) => ({ ...current, description: previousDescription }));
        setSaveError(error ?? 'Failed to save changes');
        setSaving(false);
        return 'prevent' as const;
      }

      setSaveError(null);
      onTaskUpdated(data);
      savedDescriptionRef.current = data.task.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [confirm, form.description, onTaskUpdated, taskId, workspaceId],
  );

  // ── Archive ────────────────────────────────────────────────────
  const archiveTask = async () => {
    const { error } = await pmTaskService.remove(workspaceId, taskId);
    if (error) {
      setSaveError(error);
      return;
    }
    onTaskArchived(taskId);
    onOpenChange(false);
  };

  const duplicateTask = async () => {
    if (duplicating) return;
    setDuplicateConfirmOpen(false);
    setDuplicating(true);
    const { data, error } = await pmTaskService.duplicate(workspaceId, taskId);
    setDuplicating(false);
    if (error || !data) {
      toast.error(error ?? 'Failed to duplicate task');
      return;
    }
    window.dispatchEvent(new CustomEvent('task-created', { detail: { task: data.task } }));
    setDuplicateNotice({ taskDetail: data });
  };

  const requestDuplicateTask = () => {
    if (automatedStateIds.has(taskDetail.task.workflow_state_id)) {
      setDuplicateConfirmOpen(true);
      return;
    }
    void duplicateTask();
  };

  // ── Copy link ──────────────────────────────────────────────────
  const copyLink = () =>
    copyText(
      buildTaskCopyUrl({
        currentHref: window.location.href,
        displayId: taskDetail.task.display_id,
        taskKey: taskDetail.task.task_key,
        origin: window.location.origin,
        slug: workspace?.slug,
        taskId: taskDetail.task.id,
      }),
    );

  // ── Derived data ───────────────────────────────────────────────
  const currentState = useMemo(
    () => effectiveStates.find((s) => s.id === form.workflow_state_id),
    [effectiveStates, form.workflow_state_id],
  );

  const currentEpic = useMemo(
    () => epics.find((e) => e.epic.id === form.epic_id)?.epic,
    [form.epic_id, epics],
  );

  const currentSprintName = useMemo(() => {
    if (!form.sprint_id) return 'None';
    return sprints.find((i) => i.sprint.id === form.sprint_id)?.sprint.name ?? 'None';
  }, [form.sprint_id, sprints]);

  const currentTeamName = useMemo(() => {
    if (!form.team_id) return 'Select team';
    return teams.find((t) => t.id === form.team_id)?.name ?? 'Select team';
  }, [form.team_id, teams]);

  const currentOwnerName = useMemo(() => {
    if (form.owner_member_ids.length === 0) return 'No owner';
    return form.owner_member_ids
      .map((ownerId) => memberNameMap.get(ownerId) ?? 'Unknown')
      .join(', ');
  }, [form.owner_member_ids, memberNameMap]);

  const currentRequesterName = useMemo(() => {
    if (!form.requester_member_id) return 'No requester';
    return memberNameMap.get(form.requester_member_id) ?? 'No requester';
  }, [form.requester_member_id, memberNameMap]);

  const taskLabels = taskDetail.labels ?? [];
  const isSaving = saving || labelSaving || hasPendingTaskSave(pendingPatch, descriptionPendingUploads);

  useEffect(() => {
    if (!form) return;
    const validLabelIds = taskLabels
      .filter((label) => !label.team_id || (form.team_id ? label.team_id === form.team_id : false))
      .map((label) => label.id);
    if (validLabelIds.length === taskLabels.length) return;
    void syncTaskLabelsWithFeedback({
      workspaceId,
      taskId: taskDetail.task.id,
      currentLabelIds: taskLabels.map((label) => label.id),
      nextLabelIds: validLabelIds,
      syncLabels: pmTaskService.syncLabels,
      reloadTask: pmTaskService.get,
      onTaskUpdated: onTaskUpdated,
      onSaved: reloadActivity,
      setSaving: setLabelSaving,
      setSaveError,
    });
  }, [form?.team_id, onTaskUpdated, taskDetail.task.id, taskLabels, workspaceId]);

  const taskBreadcrumbSprintName = currentSprintName !== 'None'
    ? currentSprintName
    : taskDetail.sprint_name;
  const taskBreadcrumbItems = [
    {
      id: 'tasks',
      label: 'Tasks',
      icon: <CheckListIcon className="h-3.5 w-3.5 text-quiet-muted" />,
      onClick: () => {
        if (!workspace) return;
        onOpenChange(false);
        navigate({
          to: '/w/$slug/pm/tasks',
          params: { slug: workspace.slug },
          search: form.team_id ? { team: form.team_id } : {},
        });
      },
    },
    ...(taskDetail.objective_name && taskDetail.objective_id && workspace ? [{
      id: `objective-${taskDetail.objective_id}`,
      label: taskDetail.objective_name,
      icon: <Target01Icon className="h-3.5 w-3.5 text-quiet-muted" />,
      onClick: () => {
        onOpenChange(false);
        navigate({
          to: '/w/$slug/pm/objectives/$objectiveId',
          params: { slug: workspace.slug, objectiveId: taskDetail.objective_id! },
        });
      },
    }] : []),
    ...(taskDetail.epic_name && taskDetail.task.epic_id && workspace ? [{
      id: `epic-${taskDetail.task.epic_id}`,
      label: taskDetail.epic_name,
      icon: <Layers01Icon className="h-3.5 w-3.5 text-quiet-muted" />,
      onClick: () => {
        onOpenChange(false);
        navigate({
          to: '/w/$slug/pm/epics/$epicId',
          params: { slug: workspace.slug, epicId: taskDetail.task.epic_id! },
        });
      },
    }] : []),
    ...(taskBreadcrumbSprintName && form.sprint_id && workspace && teamSprintsEnabled ? [{
      id: `sprint-${form.sprint_id}`,
      label: taskBreadcrumbSprintName,
      icon: <SprintIcon className="h-3.5 w-3.5 text-quiet-muted" />,
      onClick: () => {
        onOpenChange(false);
        navigate({
          to: '/w/$slug/pm/sprints/$sprintId',
          params: { slug: workspace.slug, sprintId: form.sprint_id! },
        });
      },
    }] : []),
  ];

  return (
    <div className="flex h-full min-w-0 flex-col overflow-hidden">
      <QuietDetailHeader
        className="lg:px-10"
        breadcrumbs={<QuietBreadcrumbs items={taskBreadcrumbItems} />}
        title={(
          <QuietTitleInput
            type="text"
            aria-label="Task title"
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
        meta={(
          <QuietMetaLine items={[
            <span className="font-mono">{taskDetail.task.task_key}</span>,
            form.team_id ? currentTeamName : 'No team',
            taskDetail.task.recurring_template_id
              ? `Recurring${taskDetail.task.recurring_occurrence_number ? ` #${taskDetail.task.recurring_occurrence_number}` : ''}`
              : null,
          ]} />
        )}
        status={(
          <QuietStatusBadge className="lg:hidden" tone={getWorkflowStateTone(currentState?.state_type)} color={currentState?.color}>
            {currentState?.name ?? 'No state'}
          </QuietStatusBadge>
        )}
        state={<SaveIndicator saving={isSaving} error={saveError} presentation="quiet" />}
        actions={(
          <>
            <QuietDetailAction
              iconOnly
              icon={linkCopied ? <Tick01Icon className="h-3.5 w-3.5 text-quiet-positive" /> : <Link01Icon className="h-3.5 w-3.5" />}
              label={linkCopied ? 'Link copied' : 'Copy link'}
              onClick={copyLink}
            />
            <DropdownMenu>
              <QuickTooltip label="More actions">
                <DropdownMenuTrigger asChild>
                  <QuietIconAction className="size-8 rounded-full" aria-label="More actions">
                    <MoreVerticalIcon className="h-4 w-4" />
                  </QuietIconAction>
                </DropdownMenuTrigger>
              </QuickTooltip>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={requestDuplicateTask} disabled={duplicating}>
                  <Copy01Icon className="mr-2 h-4 w-4" />
                  Duplicate
                </DropdownMenuItem>
                {canSaveAsTemplate && (
                  <DropdownMenuItem onSelect={openSaveTemplateDialog}>
                    <File01Icon className="mr-2 h-4 w-4" />
                    Save as template
                  </DropdownMenuItem>
                )}
                <DropdownMenuItem onSelect={() => { void openRecurringDialog(); }}>
                  <ArrowReloadHorizontalIcon className="mr-2 h-4 w-4" />
                  {recurringSummary ? 'Edit recurring' : 'Make recurring'}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
                  <ArchiveIcon className="mr-2 h-4 w-4 text-amber-500" />
                  Archive
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {workspace && (
              <QuietDetailAction
                iconOnly
                icon={<ArrowUpRight01Icon className="h-3.5 w-3.5" />}
                label="Open in new tab"
                onClick={() => {
                  window.open(
                    buildTaskPath(workspace.slug, taskDetail.task.id),
                    '_blank',
                    'noopener,noreferrer',
                  );
                }}
              />
            )}
            <QuietDetailAction
              iconOnly
              icon={<Cancel01Icon className="h-4 w-4" />}
              label="Close task"
              onClick={() => onOpenChange(false)}
            />
          </>
        )}
      />

      {duplicateNotice ? (
        <div className="border-b border-border/70 bg-primary/5 px-4 py-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="min-w-0 flex-1">
              <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                Task duplicated
              </p>
              <p className="mt-0.5 truncate text-sm font-medium text-foreground">
                {[duplicateNotice.taskDetail.task.task_key, duplicateNotice.taskDetail.task.name].filter(Boolean).join(' · ')}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              {duplicateNotice.taskDetail.task.task_key ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="h-8 rounded-full px-3 text-xs"
                  onClick={() => copyDuplicateKey(duplicateNotice.taskDetail.task.task_key!)}
                >
                  {duplicateKeyCopied ? <Tick01Icon className="mr-1.5 h-3.5 w-3.5" /> : <Copy01Icon className="mr-1.5 h-3.5 w-3.5" />}
                  {duplicateKeyCopied ? 'Task ID copied' : 'Copy Task ID'}
                </Button>
              ) : null}
              <Button
                type="button"
                size="sm"
                className="h-8 rounded-full px-3 text-xs"
                onClick={() => {
                  if (!workspace?.slug) return;
                  const duplicatedTask = duplicateNotice.taskDetail;
                  setDuplicateNotice(null);
                  onTaskOpened(duplicatedTask);
                }}
              >
                <ArrowUpRight01Icon className="mr-1.5 h-3.5 w-3.5" />
                Open duplicate
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-8 w-8 rounded-full"
                onClick={() => setDuplicateNotice(null)}
              >
                <Cancel01Icon className="h-4 w-4" />
                <span className="sr-only">Dismiss duplicate notification</span>
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── Two-column grid ─────────────────────────────────────── */}
      <div className="relative grid min-h-0 min-w-0 flex-1 grid-cols-1 overflow-x-hidden overflow-y-auto lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden">
        {/* ── Left column (main content) ────────────────────────── */}
        <div className="flex min-w-0 flex-col lg:min-h-0">
          <Tabs value={activeView} onValueChange={(value) => selectView(value as TaskDetailView)} className="gap-0">
            <TabsList variant="quiet" aria-label="Task detail views" className="w-full justify-start px-4 sm:px-6 lg:px-10">
            {(['overview', 'delivery'] as TaskDetailView[]).map((view) => (
              <TabsTrigger
                key={view}
                value={view}
                className="capitalize"
              >
                {view}
                {view === 'delivery' && taskDetail.task.latest_run_status === 'failed' && (
                  <span className="h-1.5 w-1.5 rounded-full bg-destructive" aria-label="Latest agent run failed" />
                )}
              </TabsTrigger>
            ))}
            </TabsList>
          </Tabs>

          <div className="min-w-0 flex-1 overflow-x-hidden px-4 pt-5 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:px-10">
          {activeView === 'overview' && (
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
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
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
              <div className="relative min-h-9 pr-12">
                {form.description ? (
                  <RichTextMentionContent
                    html={form.description}
                    members={assignableMembers}
                    teams={mentionTeams}
                    variant="pm"
                    className="[&_p:empty]:h-1 [&_p:empty]:my-0"
                    onHtmlChange={(html) => updateField('description', html, { description: html })}
                  />
                ) : (
                  <p className="text-sm text-muted-foreground">No description yet</p>
                )}
                <DetailDescriptionEditButton onClick={beginDescriptionEditing} />
              </div>
            )}
          </div>

          {/* Attachments: compact action when empty, full section once populated. */}
          <div id="attachments-section">
            <Attachments
              workspaceId={workspaceId}
              entityType="task"
              entityId={taskDetail.task.id}
              memberNameMap={memberNameMap}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              editable={canEdit}
              showAddAction
              emptyPresentation="inline-action"
            />
          </div>

          <div className="mt-6 border-t border-border/60 pt-6" data-testid="checklist-section">
            <ChecklistItems
              workspaceId={workspaceId}
              taskId={taskDetail.task.id}
              members={assignableMembers}
              teams={mentionTeams}
            />
          </div>

          <Separator className="my-6" />

          <TaskDetailSectionHeading title="Updates" icon={Activity01Icon} className="mb-4" />
          {commentsLoading ? (
            <div className="space-y-3 py-4">
              {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
            </div>
          ) : (
            <TaskUpdatesView
              workspaceId={workspaceId}
              taskId={taskDetail.task.id}
              comments={comments}
              onCommentsChange={setComments}
              currentUserId={currentUser?.id}
              teams={mentionTeams}
              members={assignableMembers}
              onOpenDelivery={openDelivery}
            />
          )}

          </>
          )}

          {activeView === 'delivery' && (
            <div>
              <AgentRunPanel
                taskId={taskDetail.task.id}
                workspaceId={workspaceId}
                taskTeamId={taskDetail.task.team_id}
                latestRunAgentId={taskDetail.task.latest_run_agent_id}
                delivery={delivery}
                showDevelopmentHistory={hasGitIntegration && fieldVis.dev_history}
                onEditDeliveryContext={openDeliveryContext}
                deliveryContextEditReason={!canEdit ? 'You need permission to edit this task.' : deliveryLockReason || (!hasGitIntegration || delivery.hidden ? 'Connect a repository in workspace settings to configure delivery.' : undefined)}
              />
            </div>
          )}

          <div className="h-20 shrink-0 lg:h-40" aria-hidden="true" />
          </div>
        </div>

        {/* ── Right column (sidebar) ────────────────────────────── */}
        <aside ref={deliverySidebarRef} className="border-t border-border/60 px-4 py-5 pb-16 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:border-l lg:border-t-0 lg:px-5 lg:pb-40">
          <TaskSidebarIdRow displayId={taskDetail.task.display_id} taskKey={taskDetail.task.task_key} taskName={taskDetail.task.name} taskType={taskDetail.task.task_type} />

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* Team */}
            <MetadataRow icon={UserGroupIcon} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  ...(teams.length === 0 ? [{ value: '__none__', label: 'Select team' }] : []),
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  const nextWorkflow = resolveTaskTeamWorkflow(workflows, val || null);
                  const nextState =
                    nextWorkflow?.states.find((state) => state.is_default)
                    ?? nextWorkflow?.states[0]
                    ?? null;
                  if (nextWorkflow && nextState) {
                    setForm((current) => ({
                      ...current,
                      team_id: val,
                      workflow_id: nextWorkflow.workflow.id,
                      workflow_state_id: nextState.id,
                    }));
                    queuePatch({
                      team_id: val,
                      workflow_id: nextWorkflow.workflow.id,
                      workflow_state_id: nextState.id,
                    });
                  } else {
                    updateField('team_id', val, { team_id: val });
                  }
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
              />
            </MetadataRow>

            {/* State */}
            <MetadataRow icon={HashtagIcon} label="State">
              <SidebarPopoverSelect
                value={form.workflow_state_id}
                options={effectiveStates.map((s) => ({ value: s.id, label: s.name }))}
                onChange={(v) => updateField('workflow_state_id', v, { workflow_state_id: v })}
                renderTrigger={() => (
                  currentState ? (
                    <TaskStateSelectContent
                      stateType={currentState.state_type}
                      label={currentState.name}
                      color={currentState.color}
                      autoRunEnabled={automatedStateIds.has(currentState.id)}
                    />
                  ) : (
                    <span>Select</span>
                  )
                )}
                renderOption={(v) => {
                  const s = effectiveStates.find((st) => st.id === v);
                  return s ? (
                    <TaskStateSelectContent
                      stateType={s.state_type}
                      label={s.name}
                      color={s.color}
                      autoRunEnabled={automatedStateIds.has(s.id)}
                    />
                  ) : null;
                }}
              />
            </MetadataRow>

            {/* ── People ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Owner */}
            <MetadataRow icon={UserIcon} label="Owners">
              <MultiMemberPickerPopover
                values={form.owner_member_ids}
                members={assignableMembers}
                onChange={(nextOwnerIds) => {
                  updateField('owner_member_ids', nextOwnerIds, { owner_member_ids: nextOwnerIds });
                }}
                renderTrigger={() => {
                  return (
                    <>
                      {form.owner_member_ids.length > 0 ? (
                        <OwnerAvatarStack
                          memberIds={form.owner_member_ids}
                          nameMap={memberNameMap}
                          members={assignableMembers}
                          size="sm"
                          max={3}
                          singleAvatarClassName="h-4 w-4"
                          singleFallbackClassName="text-[7px]"
                        />
                      ) : (
                        <span>{currentOwnerName}</span>
                      )}
                    </>
                  );
                }}
              />
            </MetadataRow>

            {/* Requester */}
            <MetadataRow icon={UserIcon} label="Requester">
              <MemberPickerPopover
                value={form.requester_member_id || '__none__'}
                members={assignableMembers}
                noneLabel="No requester"
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('requester_member_id', val, { requester_member_id: val });
                }}
                renderTrigger={() => {
                  const selectedMember = findAssignableMember(assignableMembers, form.requester_member_id);
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
                      <span>{currentRequesterName}</span>
                    </>
                  );
                }}
              />
            </MetadataRow>

            {/* ── Classification ── */}
            {(fieldVis.severity || fieldVis.labels) && <div className="col-span-3 h-px bg-border/40 my-1" />}

            {/* Priority */}
            {fieldVis.priority && (
            <MetadataRow icon={DashboardSpeed01Icon} label="Priority">
              <SidebarPopoverSelect
                value={form.priority}
                options={priorityOptions.map((p) => ({ value: p, label: PRIORITY_CONFIG[p].label }))}
                onChange={(v) => updateField('priority', v as Priority, { priority: v as Priority })}
                renderTrigger={() => (
                  <>
                    <PriorityIcon priority={form.priority} className="h-3.5 w-3.5" />
                    <span>{PRIORITY_CONFIG[form.priority].label}</span>
                  </>
                )}
                renderOption={(v) => <><PriorityIcon priority={v as Priority} className="h-4 w-4 shrink-0" /><span>{PRIORITY_CONFIG[v as Priority].label}</span></>}
              />
            </MetadataRow>
            )}

            {/* Severity */}
            {fieldVis.severity && (
            <MetadataRow icon={Shield02Icon} label="Severity">
              <SidebarPopoverSelect
                value={form.severity}
                options={severityOptions.map((s) => ({ value: s, label: SEVERITY_CONFIG[s].label }))}
                onChange={(v) => updateField('severity', v as Severity, { severity: v as Severity })}
                renderTrigger={() => (
                  <>
                    <SeverityIcon severity={form.severity} className="h-3.5 w-3.5" />
                    <span>{SEVERITY_CONFIG[form.severity].label}</span>
                  </>
                )}
                renderOption={(v) => <><SeverityIcon severity={v as Severity} className="h-4 w-4 shrink-0" /><span>{SEVERITY_CONFIG[v as Severity].label}</span></>}
              />
            </MetadataRow>
            )}

            {/* Type */}
            {fieldVis.task_type && (
            <MetadataRow icon={HashtagIcon} label="Type">
              <SidebarPopoverSelect
                value={form.task_type}
                options={taskTypeOptions.map((t) => ({ value: t, label: TASK_TYPE_CONFIG[t].label }))}
                onChange={(v) => updateField('task_type', v as TaskType, { task_type: v as TaskType })}
                renderTrigger={() => (
                  <>
                    <TaskTypeIcon taskType={form.task_type} className="h-4 w-4" />
                    <span>{TASK_TYPE_CONFIG[form.task_type].label}</span>
                  </>
                )}
                renderOption={(v) => <><TaskTypeIcon taskType={v as TaskType} className="h-[18px] w-[18px] shrink-0" /><span>{TASK_TYPE_CONFIG[v as TaskType].label}</span></>}
              />
            </MetadataRow>
            )}

            {/* Labels */}
            {fieldVis.labels && (
            <MetadataRow icon={Tag01Icon} label="Labels">
              <LabelPicker
                workspaceId={workspaceId}
                teamId={form.team_id || undefined}
                labels={allLabels}
                selectedLabelIds={taskLabels.map((l) => l.id)}
                onLabelsChange={setAllLabels}
                onChange={async (labelIds) => {
                  await syncTaskLabelsWithFeedback({
                    workspaceId,
                    taskId: taskDetail.task.id,
                    currentLabelIds: taskLabels.map((label) => label.id),
                    nextLabelIds: labelIds,
                    syncLabels: pmTaskService.syncLabels,
                    reloadTask: pmTaskService.get,
                    onTaskUpdated: onTaskUpdated,
                    onSaved: reloadActivity,
                    setSaving: setLabelSaving,
                    setSaveError,
                  });
                }}
              />
            </MetadataRow>
            )}

            {/* ── Planning ── */}
            {(fieldVis.epic || (fieldVis.sprint && teamSprintsEnabled) || fieldVis.estimate || fieldVis.due_date || fieldVis.blocked) && <div className="col-span-3 h-px bg-border/40 my-1" />}

            {/* Epic */}
            {fieldVis.epic && (
            <MetadataRow icon={Layers01Icon} label="Epic">
              <SidebarPopoverSelect
                value={form.epic_id || '__none__'}
                groups={epicGroups}
                width={EPIC_PICKER_WIDTH}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('epic_id', val, { epic_id: val });
                }}
                renderTrigger={() => currentEpic ? (
                  <EpicBadge name={currentEpic.name} color={currentEpic.color} />
                ) : <span>None</span>}
                renderOption={(value) => {
                  const epic = availableEpics.find((entry) => entry.epic.id === value)?.epic;
                  return epic ? (
                    <span className="flex min-w-0 items-center gap-2">
                      <EpicColorSwatch color={epic.color} />
                      <span className="truncate">{epic.name}</span>
                    </span>
                  ) : <span>None</span>;
                }}
              />
            </MetadataRow>
            )}

            {/* Sprint */}
            {fieldVis.sprint && teamSprintsEnabled && (
            <MetadataRow icon={SprintIcon} label="Sprint">
              <SidebarPopoverSelect
                value={form.sprint_id || '__none__'}
                groups={sprintGroups}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('sprint_id', val, { sprint_id: val });
                }}
                renderTrigger={() => <span className="truncate">{currentSprintName}</span>}
              />
            </MetadataRow>
            )}

            <div className="col-span-3 border-t border-border/60" />

            {/* Estimate */}
            {fieldVis.estimate && (
            <MetadataRow icon={LayoutGridIcon} label="Estimate">
              <EstimatePicker
                value={form.estimate}
                teamId={form.team_id}
                onChange={(displayValue, apiValue) => {
                  updateField('estimate', displayValue, {
                    estimate: apiValue ?? -1,
                  });
                }}
              />
            </MetadataRow>
            )}

            {/* Due date */}
            {fieldVis.due_date && (
            <MetadataRow icon={Calendar03Icon} label="Due date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                kind="due"
                label="Due date"
                placeholder="None"
                disablePast
                hideIcon
                urgencyColor
                completed={taskDetail.task.completed}
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>
            )}

            {fieldVis.blocked && (
            <MetadataRow icon={Shield02Icon} label="Blocker">
              <input
                value={form.blocker}
                onChange={(event) => {
                  const value = event.target.value;
                  updateField('blocker', value, { blocker: value || undefined });
                }}
                placeholder="None"
                className="w-full bg-transparent px-1.5 py-0.5 text-xs outline-none placeholder:text-muted-foreground"
              />
            </MetadataRow>
            )}

            <div className="col-span-3 h-px bg-border/40 my-1" />
            <MetadataRow icon={ArrowReloadHorizontalIcon} label="Recurrence">
              <button type="button" className="truncate rounded px-1.5 py-0.5 text-left hover:bg-accent" onClick={() => void openRecurringDialog()}>
                {recurringSummary?.rule_summary ?? 'None'}
              </button>
            </MetadataRow>

          </div>

          {hasGitIntegration && (fieldVis.delivery || deliverySectionOpen) && !delivery.hidden ? (
            <TaskDeliveryRailSection
              workspaceId={workspaceId}
              delivery={delivery}
              open={deliverySectionOpen}
              onOpenChange={setDeliverySectionOpen}
              canEdit={canEdit}
              lockReason={deliveryLockReason}
            />
          ) : null}

          <details id="task-related-section" className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4" open>
            <summary className="cursor-pointer select-none text-xs font-semibold uppercase tracking-wide text-foreground/70">
              Related
            </summary>
            <div className="mt-3 space-y-0">
              <TaskRelationshipsSection
                workspaceId={workspaceId}
                taskId={taskDetail.task.id}
                taskName={taskDetail.task.name}
                taskDisplayId={taskDetail.task.display_id}
                workflowId={taskDetail.task.workflow_id}
                workflowStateId={taskDetail.task.workflow_state_id}
                epicId={taskDetail.task.epic_id}
                sprintId={taskDetail.task.sprint_id}
                teamId={taskDetail.task.team_id}
                taskType={taskDetail.task.task_type}
                priority={taskDetail.task.priority}
                severity={taskDetail.task.severity}
                externalBlocker={form.blocker}
                onExternalBlockerChange={(value) => updateField('blocker', value, { blocker: value || undefined })}
                composerOpen={relationshipComposerOpen}
                onComposerOpenChange={setRelationshipComposerOpen}
                hideDocs
                flat
                showExternalBlocker={false}
                visible
              />

              <div className="my-2 h-px bg-border/60" />
              <AssociationsPanel
                objectType="task"
                objectId={taskDetail.task.id}
                workspaceId={workspaceId}
                section="docs"
              />

              <div className="my-2 h-px bg-border/60" />
              <ExternalLinks
                workspaceId={workspaceId}
                entityType="task"
                entityId={taskDetail.task.id}
                flat
              />

              <div className="my-2 h-px bg-border/60" />
              <AssociationsPanel
                objectType="task"
                objectId={taskDetail.task.id}
                workspaceId={workspaceId}
                excludeDocs
                section="support"
              />

              <div className="my-2 h-px bg-border/60" />
              <AssociationsPanel
                objectType="task"
                objectId={taskDetail.task.id}
                workspaceId={workspaceId}
                excludeDocs
                section="crm"
              />
            </div>
          </details>
        </aside>
      </div>

      <Dialog open={saveTemplateDialogOpen} onOpenChange={setSaveTemplateDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Save as template</DialogTitle>
            <DialogDescription>
              Create a reusable task template from the current task.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <Input
              value={saveTemplateName}
              onChange={(event) => setSaveTemplateName(event.target.value)}
              placeholder="Template title"
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setSaveTemplateDialogOpen(false)} disabled={saveTemplateSaving}>
                Cancel
              </Button>
              <Button onClick={() => void handleSaveAsTemplate()} disabled={!saveTemplateName.trim() || saveTemplateSaving}>
                {saveTemplateSaving ? 'Saving...' : 'Save template'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={recurringDialogOpen} onOpenChange={setRecurringDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{recurringSummary ? 'Edit recurring template' : 'Make task recurring'}</DialogTitle>
            <DialogDescription>
              {recurringSummary
                ? 'Update the schedule and future generation behavior for this recurring task.'
                : 'Create a recurring template from this task so future occurrences are generated automatically.'}
            </DialogDescription>
          </DialogHeader>
          <RecurringTemplateForm
            initialValue={{
              title: recurringDetail?.template.title ?? recurringSummary?.template_title ?? form.name,
              description: recurringDetail?.template.description ?? '',
              config: recurringDetail?.config ?? recurringSummary?.config ?? {
                schedule_type: 'time',
                frequency: 'weekly',
                interval: 1,
                weekdays: [1],
                due_date_mode: 'scheduled_date',
                sprint_assignment_mode: 'none',
              },
            }}
            submitLabel={recurringSummary ? 'Save changes' : 'Create recurring template'}
            saving={recurringSaving}
            onCancel={() => setRecurringDialogOpen(false)}
            onSubmit={(value) => void handleRecurringSubmit(value)}
          />
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title="Archive task"
        description="This task will be hidden from the board and lists. You can restore it later from archived items."
        confirmLabel="Archive"
        variant="default"
        onConfirm={archiveTask}
      />

      <ConfirmDialog
        open={duplicateConfirmOpen}
        onOpenChange={setDuplicateConfirmOpen}
        title="Duplicate task and start agent?"
        description="This task is in an auto-run state. Duplicating it will create a copy in the same state and start the assigned agent automatically."
        confirmLabel="Duplicate and start agent"
        variant="default"
        onConfirm={duplicateTask}
      />
    </div>
  );
}

// ── Export wrapper ──────────────────────────────────────────────────

export function TaskDetailPanel({
  workspaceId,
  open,
  loading,
  onOpenChange,
  taskDetail,
  states,
  initialRecurringSummary,
  onTaskUpdated,
  onTaskOpened,
  onTaskArchived,
}: TaskDetailPanelProps) {
  const openedAtRef = useRef<number | null>(null);
  const taskTitle = open && taskDetail
    ? [taskDetail.task.task_key, taskDetail.task.name].filter(Boolean).join(' ')
    : undefined;
  useTitle(taskTitle);

  useEffect(() => {
    if (open) {
      openedAtRef.current = Date.now();
      return;
    }
    openedAtRef.current = null;
  }, [open]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="h-dvh overflow-hidden p-0 data-[side=right]:w-screen data-[side=right]:!max-w-none lg:data-[side=right]:w-[80vw] lg:data-[side=right]:!max-w-[1200px]"
        showCloseButton={false}
        onOpenAutoFocus={(e) => e.preventDefault()}
        onPointerDownOutside={(event) => {
          if (isInsideSonnerToast(event.target)) {
            event.preventDefault();
            return;
          }
          if (isInsideAskAgentsDock(event.target)) {
            event.preventDefault();
            return;
          }
          if (shouldSuppressTaskOverlayOutsideDismiss(openedAtRef.current, Date.now())) {
            event.preventDefault();
          }
        }}
        onInteractOutside={(event) => {
          if (isInsideSonnerToast(event.target)) {
            event.preventDefault();
            return;
          }
          if (isInsideAskAgentsDock(event.target)) {
            event.preventDefault();
            return;
          }
          if (shouldSuppressTaskOverlayOutsideDismiss(openedAtRef.current, Date.now())) {
            event.preventDefault();
          }
        }}
      >
        <SheetTitle className="sr-only">Task Detail</SheetTitle>
        {taskDetail ? (
          <TaskDetailPanelBody
            key={taskDetail.task.id}
            workspaceId={workspaceId}
            taskDetail={taskDetail}
            states={states}
            initialRecurringSummary={initialRecurringSummary ?? null}
            panelOpen={open}
            onOpenChange={onOpenChange}
            onTaskUpdated={onTaskUpdated}
            onTaskOpened={onTaskOpened}
            onTaskArchived={onTaskArchived}
          />
        ) : loading ? (
          <div className="flex h-full flex-col overflow-hidden">
            <QuietDetailHeader
              className="lg:px-10"
              breadcrumbs={<QuietBreadcrumbs items={[{ id: 'tasks', label: 'Tasks', icon: <CheckListIcon className="h-3.5 w-3.5 text-quiet-muted" /> }]} />}
              title={<div className="h-6 w-64 max-w-full animate-pulse bg-quiet-icon-well" />}
              meta={<div className="h-3 w-40 max-w-full animate-pulse bg-quiet-icon-well" />}
              actions={(
                <QuietDetailAction
                  iconOnly
                  icon={<Cancel01Icon className="h-4 w-4" />}
                  label="Close task"
                  onClick={() => onOpenChange(false)}
                />
              )}
            />
            <div className="flex flex-1 items-center justify-center">
              <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
