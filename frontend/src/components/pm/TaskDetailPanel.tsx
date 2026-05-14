import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  Activity01Icon,
  ArchiveIcon,
  ArrowLeftRightIcon,
  ArrowUpRight01Icon,
  BotIcon,
  Copy01Icon,
  DashboardSpeed01Icon,
  File01Icon,
  HashtagIcon,
  HexagonIcon,
  Layers01Icon,
  LayoutGridIcon,
  CheckmarkSquare02Icon,
  ArrowRight01Icon,
  GitBranchIcon,
  Link01Icon,
  Loading01Icon,
  Message01Icon,
  MoreVerticalIcon,
  AttachmentIcon,
  PencilEdit01Icon,
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
  StateTypeIcon,
  TASK_TYPE_CONFIG,
  TaskTypeIcon,
} from '@/lib/pmConstants';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { repositoryDefaultBranchLabel, taskBranchOptionLabel } from '@/lib/branchLabels';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { Attachments } from '@/components/pm/Attachments';
import { ChecklistItems } from '@/components/pm/ChecklistItems';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { TaskGitPanel } from '@/components/pm/TaskGitPanel';
import { useTaskDelivery } from '@/components/pm/TaskDeliveryPanel';
import { AgentRunPanel } from '@/components/pm/AgentRunPanel';
import { cn } from '@/lib/utils';
import { gitService } from '@/lib/services/gitService';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
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
import { MemberPickerPopover, MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { OwnerAvatarStack } from '@/components/pm/OwnerAvatarStack';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TaskSidebarIdRow } from '@/components/pm/TaskSidebarIdRow';
import { Badge } from '@/components/ui/badge';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTeamFieldVisibilityForTeam, useAutomationRulesByWorkflow, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { buildTaskCopyUrl, buildTaskPath } from '@/lib/pmTaskLinks';
import { CommentThread } from '@/components/pm/CommentThread';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { TaskRelationshipsSection } from '@/components/pm/TaskRelationshipsSection';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useTruncationDetection } from '@/hooks/useTruncationDetection';
import { shouldSuppressTaskOverlayOutsideDismiss } from '@/components/pm/task-detail/taskOverlayDismiss';
import { isInsideAskAgentsDock } from '@/lib/agentsDockGuard';
import { getFlushablePendingTaskPatch, hasPendingTaskSave } from '@/components/pm/task-detail/taskPendingPatch';
import { getTaskPatchSignature, isBlockedTaskPatch } from '@/components/pm/task-detail/taskAutosaveFailure';
import { queryKeys } from '@/lib/queryKeys';
import { TaskStateSelectContent } from '@/components/pm/task-detail/TaskStateSelectContent';
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
  ActivityLogEntry,
  CommentWithAuthor,
  EpicWithStats,
  AttachmentResponse,
  SprintWithStats,
  Label,
  Priority,
  RecurringTemplateDetail,
  Severity,
  TaskDetail,
  TaskRecurringSummary,
  TaskType,
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
});

const isInsideSonnerToast = (target: EventTarget | null) => (
  target instanceof HTMLElement && Boolean(target.closest('[data-sonner-toast], [data-sonner-toaster]'))
);

// ── Helpers ────────────────────────────────────────────────────────

function formatRelativeTime(iso: string) {
  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return iso;
  }
}

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


// ── Timeline Entry ─────────────────────────────────────────────────

type TimelineItem =
  | { kind: 'activity'; data: ActivityLogEntry; time: string }
  | { kind: 'comment'; data: CommentWithAuthor; time: string };

const ACTIVITY_ICON_MAP: Record<string, { icon: React.ElementType; color: string }> = {
  workflow_state_id: { icon: HashtagIcon, color: 'text-blue-500' },
  owner_member_id: { icon: UserIcon, color: 'text-violet-500' },
  owner_member_ids: { icon: UserIcon, color: 'text-violet-500' },
  team_id: { icon: UserGroupIcon, color: 'text-teal-500' },
  priority: { icon: DashboardSpeed01Icon, color: 'text-orange-500' },
  sprint_id: { icon: HexagonIcon, color: 'text-green-500' },
  epic_id: { icon: Layers01Icon, color: 'text-purple-500' },
  estimate: { icon: LayoutGridIcon, color: 'text-amber-500' },
  deadline: { icon: Calendar03Icon, color: 'text-red-500' },
  task_type: { icon: Tag01Icon, color: 'text-indigo-500' },
  labels: { icon: Tag01Icon, color: 'text-pink-500' },
  severity: { icon: Shield02Icon, color: 'text-red-500' },
  name: { icon: PencilEdit01Icon, color: 'text-muted-foreground' },
  description: { icon: PencilEdit01Icon, color: 'text-muted-foreground' },
  agent_run: { icon: BotIcon, color: 'text-indigo-500' },
};

const AGENT_RUN_ACTION_LABELS: Record<string, string> = {
  started: 'started',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'cancelled',
  paused: 'paused',
  resumed: 'resumed',
};

/** Map raw action strings like "comment_added" to readable labels. */
const ACTION_LABELS: Record<string, string> = {
  comment_added: 'added a comment',
  comment_updated: 'edited a comment',
  comment_deleted: 'deleted a comment',
  attachment_added: 'attached a file',
  attachment_removed: 'removed an attachment',
  task_created: 'created this task',
  created: 'created this task',
  label_added: 'added a label',
  label_removed: 'removed a label',
};

function formatAction(action?: string): string {
  if (!action) return '';
  return ACTION_LABELS[action] ?? action.replace(/_/g, ' ');
}

function getActivityIcon(action?: string, fieldName?: string): { icon: React.ElementType; color: string } {
  if (fieldName && ACTIVITY_ICON_MAP[fieldName]) return ACTIVITY_ICON_MAP[fieldName];
  if (action?.includes('comment')) return { icon: Message01Icon, color: 'text-blue-500' };
  if (action?.includes('attachment') || action?.includes('file')) return { icon: AttachmentIcon, color: 'text-muted-foreground' };
  if (action?.includes('label')) return { icon: Tag01Icon, color: 'text-pink-500' };
  if (action?.includes('moved') || action?.includes('state')) return { icon: HashtagIcon, color: 'text-blue-500' };
  if (action?.includes('priority')) return { icon: DashboardSpeed01Icon, color: 'text-orange-500' };
  if (action?.includes('owner') || action?.includes('assigned') || action?.includes('requester')) return { icon: UserIcon, color: 'text-violet-500' };
  if (action?.includes('team')) return { icon: UserGroupIcon, color: 'text-teal-500' };
  if (action?.includes('sprint')) return { icon: HexagonIcon, color: 'text-green-500' };
  if (action?.includes('epic')) return { icon: Layers01Icon, color: 'text-purple-500' };
  if (action?.includes('blocked')) return { icon: Shield02Icon, color: 'text-red-500' };
  if (action?.includes('archived')) return { icon: ArchiveIcon, color: 'text-amber-500' };
  if (action?.includes('created')) return { icon: PlayIcon, color: 'text-green-500' };
  if (action?.includes('deadline') || action?.includes('due date')) return { icon: Calendar03Icon, color: 'text-red-500' };
  if (action?.includes('estimate')) return { icon: LayoutGridIcon, color: 'text-amber-500' };
  if (action?.includes('type')) return { icon: Tag01Icon, color: 'text-indigo-500' };
  if (action?.includes('severity')) return { icon: Shield02Icon, color: 'text-red-500' };
  return { icon: ArrowLeftRightIcon, color: 'text-muted-foreground' };
}

function TimelineEntry({ item, states = [] }: { item: TimelineItem; states?: WorkflowState[] }) {
  if (item.kind === 'comment') {
    const { comment, author } = item.data;
    return (
      <div className="flex items-start gap-2.5">
        <UserAvatar
          name={author.full_name || author.email}
          avatarUrl={author.avatar_url}
          className="h-5 w-5"
          fallbackClassName="text-[7px]"
        />
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-1.5">
            <span className="text-xs font-medium">{author.full_name || author.email}</span>
            <span className="text-[11px] text-muted-foreground">{formatRelativeTime(comment.created_at)}</span>
          </div>
          <p className="mt-0.5 text-xs text-foreground/90">{comment.body}</p>
        </div>
      </div>
    );
  }

  const { activity, actor } = item.data;
  const iconConfig = getActivityIcon(activity.action, activity.field_name);
  const ActivityIconEl = iconConfig.icon;
  const label = formatAction(activity.action);

  // For state changes, extract the target state name and use its color
  const stateMatch = activity.action?.match(/moved this (?:task|story) to (.+)/);
  const isStateChange = !!stateMatch;
  const targetStateName = stateMatch?.[1] ?? null;
  const matchedState = targetStateName ? states.find((s) => s.name === targetStateName) : null;
  const stateColor = matchedState?.color ?? null;

  // Always show avatar; fall back to icon for system/no-actor entries
  const marker = actor ? (
    <span className="relative z-10">
      <UserAvatar
        name={actor.full_name || actor.email}
        avatarUrl={actor.avatar_url}
        className="h-5 w-5"
        fallbackClassName="text-[7px]"
      />
    </span>
  ) : (
    <span className="relative z-10 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted ring-2 ring-background">
      <ActivityIconEl className={`h-3 w-3 ${iconConfig.color}`} />
    </span>
  );

  // Build rich inline label
  let richLabel: React.ReactNode = null;

  const isAgentRunActivity = activity.field_name === 'agent_run';

  if (isStateChange && targetStateName) {
    richLabel = (
      <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
        moved to
        <span
          className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
          style={stateColor ? { backgroundColor: `${stateColor}18`, color: stateColor, border: `1px solid ${stateColor}30` } : undefined}
        >
          {matchedState && <StateTypeIcon stateType={matchedState.state_type} className="h-3 w-3" />}
          {targetStateName}
        </span>
      </span>
    );
  } else if (isAgentRunActivity) {
    const meta = activity.metadata as { agent_name?: string } | undefined;
    const rawAction = activity.new_value ?? 'started';
    const actionLabel = AGENT_RUN_ACTION_LABELS[rawAction] ?? rawAction;
    const agentName = meta?.agent_name ?? 'agent';

    richLabel = (
      <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
        {actionLabel} agent run
        <span className="inline-flex items-center gap-1 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-1.5 py-0.5 text-[10px] font-medium text-indigo-700 dark:text-indigo-300">
          <AgentAvatar name={agentName} className="h-3 w-3 rounded-none border-0 bg-transparent shadow-none" genericBare />
          {agentName}
        </span>
      </span>
    );
  } else {
    // Parse "changed X from Y to Z" patterns
    const changeMatch = activity.action?.match(/changed (type|priority|severity) from (\S+) to (\S+)/);
    if (changeMatch) {
      const [, field, oldVal, newVal] = changeMatch;
      const renderBadge = (value: string) => {
        if (field === 'type') {
          const cfg = TASK_TYPE_CONFIG[value as TaskType];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <TaskTypeIcon taskType={value as TaskType} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        if (field === 'priority') {
          const cfg = PRIORITY_CONFIG[value as Priority];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <PriorityIcon priority={value as Priority} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        if (field === 'severity') {
          const cfg = SEVERITY_CONFIG[value as Severity];
          if (cfg) {
            return (
              <span className="inline-flex items-center gap-0.5 rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">
                <SeverityIcon severity={value as Severity} className="h-3 w-3" />
                {cfg.label}
              </span>
            );
          }
        }
        return <span className="rounded-full border border-border/60 px-1.5 py-0.5 text-[10px] font-medium">{value}</span>;
      };

      richLabel = (
        <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground flex-wrap">
          changed {field} from {renderBadge(oldVal)}
          <ArrowLeftRightIcon className="h-2.5 w-2.5 text-muted-foreground/50" />
          {renderBadge(newVal)}
        </span>
      );
    }
  }

  return (
    <div className="flex items-center gap-2.5">
      {marker}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="text-xs font-medium shrink-0">{actor?.full_name || actor?.email || 'System'}</span>
          {richLabel ?? <span className="text-[11px] text-muted-foreground truncate">{label}</span>}
          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground/70">{formatRelativeTime(activity.created_at)}</span>
        </div>
      </div>
    </div>
  );
}

// ── Main Body ──────────────────────────────────────────────────────

function TaskDetailPanelBody({
  workspaceId,
  taskDetail,
  states,
  initialRecurringSummary,
  onOpenChange,
  onTaskUpdated,
  onTaskOpened,
  onTaskArchived,
}: {
  workspaceId: string;
  taskDetail: TaskDetail;
  states: WorkflowState[];
  initialRecurringSummary: TaskRecurringSummary | null;
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
  const openFilePickerRef = useRef<(() => void) | null>(null);
  const descriptionUploadRef = useRef<((files: FileList | File[], insertPos?: number) => Promise<void>) | null>(null);
  const queuedDescriptionDropRef = useRef<File[] | null>(null);
  const descriptionDragCounterRef = useRef(0);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, form.team_id);
  const { data: workspaceAccess } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(workspaceAccess);
  const { canEdit } = permissions;
  const taskId = taskDetail.task.id;
  const canSaveAsTemplate = permissions.isAdmin || (!!taskDetail.task.team_id && permissions.isTeamManager(taskDetail.task.team_id));
  const [saveTemplateDialogOpen, setSaveTemplateDialogOpen] = useState(false);
  const [saveTemplateName, setSaveTemplateName] = useState(taskDetail.task.name);
  const [saveTemplateSaving, setSaveTemplateSaving] = useState(false);

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
  const { checkRef: checkDeliveryTruncation, isTruncated: isDeliveryTruncated } = useTruncationDetection();

  // Re-sync form when taskDetail changes externally (e.g. real-time WS update)
  const lastSyncedAt = useRef(taskDetail.task.updated_at);
  const savedDescriptionRef = useRef(taskDetail.task.description ?? '');
  const savedWorkflowStateIdRef = useRef(taskDetail.task.workflow_state_id);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [commentsLoading, setCommentsLoading] = useState(true);

  const currentUser = useAuthStore((s) => s.user);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);
  const [activityLoading, setActivityLoading] = useState(true);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const [showAllActivity, setShowAllActivity] = useState(false);
  const [showChecklist, setShowChecklist] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [showRelationships, setShowRelationships] = useState(false);
  const [relationshipComposerOpen, setRelationshipComposerOpen] = useState(false);
  const relationshipButtonRef = useRef<HTMLButtonElement>(null);
  const relationshipsToggleActive = showRelationships || relationshipComposerOpen;
  const hasOptionalTaskSections = relationshipsToggleActive || showChecklist || showExternalLinks;
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
    const res = await pmTaskService.listActivity(workspaceId, taskDetail.task.id, 1, 30);
    setActivity(res.data?.data ?? []);
    setActivityLoading(false);
  }, [workspaceId, taskDetail.task.id]);

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
  const workflowId = states[0]?.workflow_id;
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
  const hasPipeline = automatedStateIds.size > 0;

  const notifyAgentAutoRunStateChange = useCallback((fromStateId: string | null | undefined, toStateId: string | null | undefined) => {
    if (!shouldNotifyAgentAutoRunStateChange({ fromStateId, toStateId, automatedStateIds })) return;
    if (!toStateId) return;
    const stateName = states.find((state) => state.id === toStateId)?.name ?? 'this state';
    toast.info(getAgentAutoRunStateChangeMessage(stateName), { id: getAgentAutoRunStateChangeToastId(toStateId) });
  }, [automatedStateIds, states]);

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateTaskRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
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
    setEditingDescription(true);
  }, [resetDescriptionDrag]);

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
        isSprintSelectableForTaskTeam(entry.sprint.team_id ?? null, form.team_id || null),
      ),
    [form.team_id, sprints],
  );

  // Group epics by lifecycle: not started → in progress → completed.
  // Order within each group matches `availableEpics` (server-supplied order).
  const epicGroups = useMemo(() => {
    const notStarted: typeof availableEpics = [];
    const inProgress: typeof availableEpics = [];
    const completed: typeof availableEpics = [];
    for (const entry of availableEpics) {
      if (entry.epic.completed) completed.push(entry);
      else if (entry.epic.started) inProgress.push(entry);
      else notStarted.push(entry);
    }
    return [
      { label: undefined as string | undefined, options: [{ value: '__none__', label: 'None' }] },
      { label: 'Not started', options: notStarted.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
      { label: 'In progress', options: inProgress.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
      { label: 'Completed', options: completed.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
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

  // ── Auto-show checklist / external links if items exist ────────
  useEffect(() => {
    (async () => {
      const [clRes, elRes] = await Promise.all([
        pmChecklistService.list(workspaceId, taskDetail.task.id),
        pmExternalLinkService.list(workspaceId, taskDetail.task.id),
      ]);
      if (clRes.data && clRes.data.length > 0) setShowChecklist(true);
      if (elRes.data && elRes.data.length > 0) setShowExternalLinks(true);
    })();
  }, [workspaceId, taskDetail]);

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
    () => states.find((s) => s.id === form.workflow_state_id),
    [states, form.workflow_state_id],
  );

  const currentEpicName = useMemo(() => {
    if (!form.epic_id) return 'None';
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? 'None';
  }, [form.epic_id, epics]);

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

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="ui-divider-bottom-fade flex items-center gap-2 px-4 py-2.5">
        <div className="flex min-w-0 flex-1 items-center gap-1 text-xs text-muted-foreground">
          {taskDetail.objective_name && taskDetail.objective_id && workspace && (
            <>
              <Target01Icon className="h-3.5 w-3.5 shrink-0 text-blue-500" />
              <button
                type="button"
                className="max-w-[220px] truncate hover:text-foreground transition-colors cursor-pointer xl:max-w-[320px]"
                title={taskDetail.objective_name}
                onClick={() => {
                  onOpenChange(false);
                  navigate({ to: '/w/$slug/pm/objectives/$objectiveId', params: { slug: workspace.slug, objectiveId: taskDetail.objective_id! } });
                }}
              >
                {taskDetail.objective_name}
              </button>
              <ArrowRight01Icon className="h-3 w-3 shrink-0" />
            </>
          )}
          {taskDetail.epic_name && taskDetail.task.epic_id && workspace && (
            <>
              <HexagonIcon className="h-3.5 w-3.5 shrink-0 text-purple-500" />
              <button
                type="button"
                className="max-w-[220px] truncate hover:text-foreground transition-colors cursor-pointer xl:max-w-[320px]"
                title={taskDetail.epic_name}
                onClick={() => {
                  onOpenChange(false);
                  navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug: workspace.slug, epicId: taskDetail.task.epic_id! } });
                }}
              >
                {taskDetail.epic_name}
              </button>
              <ArrowRight01Icon className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentSprintName !== 'No sprint' && form.sprint_id && workspace && teamSprintsEnabled && (
            <>
              <SprintIcon className="h-3.5 w-3.5 shrink-0 text-green-500" />
              <button
                type="button"
                className="max-w-[220px] truncate hover:text-foreground transition-colors cursor-pointer xl:max-w-[320px]"
                title={currentSprintName}
                onClick={() => {
                  onOpenChange(false);
                  navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug: workspace.slug, sprintId: form.sprint_id! } });
                }}
              >
                {currentSprintName}
              </button>
              <ArrowRight01Icon className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0 text-xs font-medium text-foreground">{taskDetail.task.task_key}</span>
          {taskDetail.task.recurring_template_id ? (
            <RecurringTemplateBadge
              compact
              occurrenceNumber={taskDetail.task.recurring_occurrence_number}
            />
          ) : null}
        </div>

        <div className="ml-2 flex shrink-0 items-center gap-1">
          <SaveIndicator saving={isSaving} error={saveError} />
          {linkCopied ? (
            <span className="flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-green-600">
              <Tick01Icon className="h-3.5 w-3.5" />
              Copied!
            </span>
          ) : (
            <QuickTooltip label="Copy link">
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={copyLink}>
                <Link01Icon className="h-3.5 w-3.5" />
              </Button>
            </QuickTooltip>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreVerticalIcon className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
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
            <QuickTooltip label="Open in new tab">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                onClick={() => {
                  window.open(
                    buildTaskPath(workspace.slug, taskDetail.task.id),
                    '_blank',
                    'noopener,noreferrer',
                  );
                }}
              >
                <ArrowUpRight01Icon className="h-3.5 w-3.5" />
              </Button>
            </QuickTooltip>
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => onOpenChange(false)}>
            <Cancel01Icon className="h-4 w-4" />
          </Button>
        </div>
      </div>

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
                  {duplicateKeyCopied ? 'Story ID copied' : 'Copy Story ID'}
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
      <div className="relative grid min-h-0 flex-1 grid-cols-[1fr_300px] overflow-hidden">
        {/* ── Left column (main content) ────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-10 py-5 pb-40">
          {/* Pipeline step indicator */}
          {hasPipeline && (
            <div className="mb-4 flex items-center gap-0">
              {states.map((state, idx) => {
                const currentIdx = states.findIndex((s) => s.id === form.workflow_state_id);
                const isPast = idx < currentIdx;
                const isCurrent = idx === currentIdx;
                const isAutomated = automatedStateIds.has(state.id);
                return (
                  <div key={state.id} className="flex items-center">
                    {idx > 0 && (
                      <div className={`h-[2px] w-4 ${isPast || isCurrent ? 'bg-primary' : 'bg-border'}`} />
                    )}
                    <QuickTooltip label={`${state.name}${isAutomated ? ' (automated)' : ''}`}>
                      <div className={`flex h-5 w-5 items-center justify-center rounded-full border-2 transition-colors ${
                        isCurrent
                          ? 'border-primary bg-primary text-primary-foreground'
                          : isPast
                            ? 'border-primary bg-primary/20 text-primary'
                            : 'border-border bg-background text-muted-foreground'
                      }`}>
                        {isPast ? (
                          <Tick01Icon className="h-2.5 w-2.5" />
                        ) : isAutomated ? (
                          <BotIcon className="h-2.5 w-2.5" />
                        ) : (
                          <span className="h-1.5 w-1.5 rounded-full bg-current" />
                        )}
                      </div>
                    </QuickTooltip>
                  </div>
                );
              })}
            </div>
          )}

          {/* Title */}
          <input
            type="text"
            aria-label="Task title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Description */}
          <div
            className={cn(
              'group/desc relative mt-4 rounded-lg pb-3 transition-[box-shadow,background-color]',
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
              <div>
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => updateField('description', html, { description: html })}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none [&_.ProseMirror]:text-sm"
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  onUploadReady={handleDescriptionUploadReady}
                  teams={mentionTeams}
                  members={assignableMembers}
                />
                <div className="mt-2 flex justify-start">
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setEditingDescription(false)}>
                    Done
                  </Button>
                </div>
              </div>
            ) : (
              <div className="relative">
                {form.description ? (
                  <RichTextMentionContent
                    html={form.description}
                    members={assignableMembers}
                    teams={mentionTeams}
                    className="prose prose-sm dark:prose-invert max-w-none text-sm text-foreground/80 prose-p:text-foreground/80 prose-li:text-foreground/80 prose-strong:text-foreground/90 [&_p:empty]:h-1 [&_p:empty]:my-0"
                    onHtmlChange={(html) => updateField('description', html, { description: html })}
                  />
                ) : (
                  <p className="text-sm text-muted-foreground">No description yet</p>
                )}
                <div className="mt-3 flex justify-start opacity-0 transition-opacity group-hover/desc:opacity-100 group-focus-within/desc:opacity-100">
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                    onClick={() => setEditingDescription(true)}
                  >
                    <PencilEdit01Icon className="h-3 w-3" />
                    Edit description
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Action bar — "Add to Task" */}
          <div className="border-t border-border/60 pt-3">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                  showChecklist
                    ? 'border-primary/30 bg-primary/10 text-primary'
                    : 'border-border/60 text-muted-foreground hover:bg-accent'
                }`}
                onClick={() => setShowChecklist((v) => !v)}
              >
                <CheckmarkSquare02Icon className="h-3 w-3" />
                Checklist
              </button>
              <button
                ref={relationshipButtonRef}
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                  relationshipsToggleActive
                    ? 'border-primary/30 bg-primary/10 text-primary'
                    : 'border-border/60 text-muted-foreground hover:bg-accent'
                }`}
                onClick={() => {
                  setShowRelationships((open) => {
                    const nextOpen = !open;
                    if (!nextOpen) {
                      setRelationshipComposerOpen(false);
                    }
                    return nextOpen;
                  });
                }}
              >
                <ArrowLeftRightIcon className="h-3 w-3" />
                Relationships
              </button>
              <button
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                  showExternalLinks
                    ? 'border-primary/30 bg-primary/10 text-primary'
                    : 'border-border/60 text-muted-foreground hover:bg-accent'
                }`}
                onClick={() => setShowExternalLinks((v) => !v)}
              >
                <Link01Icon className="h-3 w-3" />
                External Links
              </button>
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-full border border-border/60 px-3 py-1 text-xs font-medium text-muted-foreground hover:bg-accent transition-colors cursor-pointer"
                onClick={() => openFilePickerRef.current?.()}
              >
                <AttachmentIcon className="h-3 w-3" />
                Attach Files
              </button>
            </div>
          </div>

          {/* Recurring info card */}
          {taskDetail.task.recurring_template_id && recurringSummary ? (
            <button
              type="button"
              className="mt-4 flex w-full items-center gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5 text-left transition-colors hover:bg-muted/40"
              onClick={() => void openRecurringDialog()}
            >
              <ArrowReloadHorizontalIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-sm">
                  <span className="font-medium">{recurringSummary.rule_summary}</span>
                  <Badge variant="outline" className={cn('text-[10px] capitalize', recurringSummary.status === 'active' ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-200' : recurringSummary.status === 'paused' ? 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-200' : 'border-border')}>
                    {recurringSummary.status}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  {recurringSummary.occurrence_number ? `#${recurringSummary.occurrence_number} in series` : ''}{recurringSummary.occurrence_number && recurringSummary.generated_count ? ' · ' : ''}{recurringSummary.generated_count ? `${recurringSummary.generated_count} generated` : ''}
                </p>
              </div>
              <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
            </button>
          ) : null}

          {hasOptionalTaskSections && (
            <div className="mt-8 space-y-8">
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
                visible={showRelationships}
                externalTriggerRef={relationshipButtonRef}
              />

              {/* Checklist */}
              {showChecklist && (
                <div>
                  <ChecklistItems
                    workspaceId={workspaceId}
                    taskId={taskDetail.task.id}
                    members={assignableMembers}
                    teams={mentionTeams}
                  />
                </div>
              )}

              {/* External Links */}
              {showExternalLinks && (
                <div>
                  <ExternalLinks workspaceId={workspaceId} entityType="task" entityId={taskDetail.task.id} />
                </div>
              )}
            </div>
          )}

          {/* Attachments */}
          <div className="mt-6" id="attachments-section">
            <Attachments
              workspaceId={workspaceId}
              entityType="task"
              entityId={taskDetail.task.id}
              memberNameMap={memberNameMap}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              onFilePickerReady={(fn) => { openFilePickerRef.current = fn; }}
            />
          </div>

          {/* Git Links & Agent Runs */}
          {hasGitIntegration && fieldVis.dev_history && (
            <>
              <TaskGitPanel taskId={taskDetail.task.id} workspaceId={workspaceId} />
              <AgentRunPanel
                taskId={taskDetail.task.id}
                workspaceId={workspaceId}
                latestRunAgentId={taskDetail.task.latest_run_agent_id}
                delivery={delivery}
                canEditDelivery={canEdit && fieldVis.delivery}
              />
            </>
          )}

          {/* Separator */}
          {(commentsLoading || comments.length > 0) && <Separator className="my-6 bg-border/60" />}

          {/* Comments + Activity */}
          <div>
            {/* Comments card */}
            {commentsLoading ? (
              <div className="space-y-3 rounded-lg border border-border/60 p-4">
                {[1, 2].map((i) => (
                  <div key={i} className="flex items-start gap-3">
                    <Skeleton className="h-8 w-8 rounded-full shrink-0" />
                    <div className="flex-1 space-y-2">
                      <Skeleton className="h-3 w-32" />
                      <Skeleton className="h-3 w-full" />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <>
                {comments.length > 0 && (
                  <TaskDetailSectionHeading title="Comments" icon={Message01Icon} className="mb-3" />
                )}
              <CommentThread
                workspaceId={workspaceId}
                entityType="task"
                entityId={taskDetail.task.id}
                comments={comments}
                currentUserId={currentUser?.id}
                teams={mentionTeams}
                members={assignableMembers}
                onCommentsChange={setComments}
                hideEmptyState
              />
              </>
            )}

            {/* Activity section */}
            {activityLoading ? (
              <div className="mt-6 space-y-3">
                <Skeleton className="h-3 w-20" />
                {[1, 2, 3].map((i) => (
                  <div key={i} className="flex items-center gap-3">
                    <Skeleton className="h-4 w-4 rounded-full shrink-0" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                ))}
              </div>
            ) : null}
            {!activityLoading && activity.length > 0 && (
              <div className="mt-6">
                <TaskDetailSectionHeading title="Activity" icon={Activity01Icon} />
                <div className="relative mt-3">
                  {/* Vertical timeline line */}
                  <div className="absolute left-[9px] top-3 bottom-3 w-px bg-border/60" />
                  <div className="space-y-3">
                    {!showAllActivity && activity.length > 5 && (
                      <button
                        type="button"
                        className="relative z-10 ml-6 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                        onClick={() => setShowAllActivity(true)}
                      >
                        Show {activity.length - 5} older entries...
                      </button>
                    )}
                    {(showAllActivity ? activity : activity.slice(0, 5)).map((entry) => (
                      <TimelineEntry key={`a-${entry.activity.id}`} item={{ kind: 'activity', data: entry, time: entry.activity.created_at }} states={states} />
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* ── Right column (sidebar) ────────────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-5 py-5 pb-40">
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
                  updateField('team_id', val, { team_id: val });
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
              />
            </MetadataRow>

            {/* State */}
            <MetadataRow icon={HashtagIcon} label="State">
              <SidebarPopoverSelect
                value={form.workflow_state_id}
                options={states.map((s) => ({ value: s.id, label: s.name }))}
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
                  const s = states.find((st) => st.id === v);
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
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('epic_id', val, { epic_id: val });
                }}
                renderTrigger={() => <span className="truncate">{currentEpicName}</span>}
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

            {/* ── Delivery ── */}
            {hasGitIntegration && fieldVis.delivery && !delivery.hidden && !delivery.loading && (
              <>
                <div className="col-span-3 h-px bg-border/40 my-1" />

                <MetadataRow icon={GitBranchIcon} label="Repository">
                  <SidebarPopoverSelect
                    value={delivery.repositoryId || '__none__'}
                    options={[
                      { value: '__none__', label: 'None' },
                      ...delivery.repositories.map((r) => ({ value: r.id, label: r.full_name })),
                    ]}
                    onChange={(v) => {
                      const val = v === '__none__' ? '' : v;
                      delivery.handleRepoChange(val);
                    }}
                    renderTrigger={() => (
                      <span className="min-w-0 whitespace-normal [overflow-wrap:anywhere]">
                        {delivery.selectedRepository?.full_name ?? 'None'}
                      </span>
                    )}
                  />
                </MetadataRow>

            <MetadataRow icon={GitBranchIcon} label="Base branch">
              <RepositoryBranchPicker
                workspaceId={workspaceId}
                repositoryId={delivery.repositoryId || undefined}
                value={delivery.baseBranch}
                onChange={(value) => {
                  void delivery.handleBaseBranchChange(value);
                }}
                placeholder={delivery.selectedRepository?.default_branch || 'main'}
                emptyLabel={repositoryDefaultBranchLabel(delivery.selectedRepository?.default_branch)}
                extraOptions={
                  delivery.branchPreview
                    ? [{ value: delivery.branchPreview, label: taskBranchOptionLabel(delivery.branchPreview) }]
                    : []
                }
                disabled={delivery.savingTarget}
                variant="sidebar"
                width="w-72"
                triggerLabel={(
                  <Tooltip open={isDeliveryTruncated('delivery-base') ? undefined : false}>
                    <TooltipTrigger asChild>
                      <span
                        ref={(el) => checkDeliveryTruncation('delivery-base', el)}
                        className="block min-w-0 truncate font-mono"
                      >
                        {delivery.resolvedBaseBranch}
                      </span>
                    </TooltipTrigger>
                    <TooltipContent align="start">{delivery.resolvedBaseBranch}</TooltipContent>
                  </Tooltip>
                )}
              />
                </MetadataRow>

                <MetadataRow icon={GitBranchIcon} label="Task branch">
                  <Tooltip open={isDeliveryTruncated('delivery-branch') ? undefined : false}>
                    <TooltipTrigger asChild>
                      <span
                        ref={(el) => checkDeliveryTruncation('delivery-branch', el)}
                        className="block min-w-0 truncate font-mono text-xs px-1.5 py-0.5"
                      >
                        {delivery.branchPreview}
                      </span>
                    </TooltipTrigger>
                    <TooltipContent align="start">{delivery.branchPreview}</TooltipContent>
                  </Tooltip>
                </MetadataRow>

                {delivery.deliveryStateCfg && (
                  <MetadataRow icon={PlayIcon} label="Status">
                    <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${delivery.deliveryStateCfg.className}`}>
                      {delivery.deliveryStateCfg.label}
                    </span>
                  </MetadataRow>
                )}
              </>
            )}

          </div>

          <AssociationsPanel
            objectType="task"
            objectId={taskDetail.task.id}
            workspaceId={workspaceId}
            includeTaskRelationships={false}
            className="-mx-4 mt-4 border-t border-border/60"
          />
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
        className="p-0 data-[side=right]:w-[80vw] data-[side=right]:!max-w-[1200px]"
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
            onOpenChange={onOpenChange}
            onTaskUpdated={onTaskUpdated}
            onTaskOpened={onTaskOpened}
            onTaskArchived={onTaskArchived}
          />
        ) : loading ? (
          <div className="flex h-full items-center justify-center">
            <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
