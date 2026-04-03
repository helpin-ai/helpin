import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  Archive,
  ArrowRightLeft,
  Bot,
  CalendarDays,
  Check,
  CheckSquare,
  ChevronRight,
  Gauge,
  GitBranch,
  Hash,
  Hexagon,
  Layers,
  LayoutGrid,
  Link2,
  Loader2,
  Maximize2,
  MessageSquare,
  MoreVertical,
  Paperclip,
  Pencil,
  Play,
  RefreshCw,
  ShieldAlert,
  Tag,
  Target,
  Upload,
  User,
  Users,
  X,
} from 'lucide-react';
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
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
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
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TaskSidebarIdRow } from '@/components/pm/TaskSidebarIdRow';
import { Badge } from '@/components/ui/badge';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTeamFieldVisibilityForTeam, useAutomationRulesByWorkflow } from '@/hooks/queries';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { buildTaskCopyUrl, buildTaskPath } from '@/lib/pmTaskLinks';
import { CommentThread } from '@/components/pm/CommentThread';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { TaskRelationshipsSection } from '@/components/pm/TaskRelationshipsSection';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useTruncationDetection } from '@/hooks/useTruncationDetection';
import { shouldSuppressTaskOverlayOutsideDismiss } from '@/components/pm/task-detail/taskOverlayDismiss';
import { getFlushablePendingTaskPatch, hasPendingTaskSave } from '@/components/pm/task-detail/taskPendingPatch';
import { TaskStateSelectContent } from '@/components/pm/task-detail/TaskStateSelectContent';
import {
  isEpicSelectableForTaskTeam,
  isSprintSelectableForTaskTeam,
} from '@/components/pm/task-detail/taskPlanningScope';
import { syncTaskLabelsWithFeedback } from '@/components/pm/task-detail/taskLabelSync';
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
  owner_member_id: string;
  requester_member_id: string;
  blocker: string;
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
  owner_member_id: detail.task.owner_member_id ?? '',
  requester_member_id: detail.task.requester_member_id ?? '',
  blocker: detail.task.blocker ?? '',
});

// ── Helpers ────────────────────────────────────────────────────────

function formatRelativeTime(iso: string) {
  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return iso;
  }
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
      <span className="text-xs text-muted-foreground mt-0.5">{label}</span>
      <div className="min-w-0">{children}</div>
    </>
  );
}


// ── Timeline Entry ─────────────────────────────────────────────────

type TimelineItem =
  | { kind: 'activity'; data: ActivityLogEntry; time: string }
  | { kind: 'comment'; data: CommentWithAuthor; time: string };

const ACTIVITY_ICON_MAP: Record<string, { icon: React.ElementType; color: string }> = {
  workflow_state_id: { icon: Hash, color: 'text-blue-500' },
  owner_member_id: { icon: User, color: 'text-violet-500' },
  team_id: { icon: Users, color: 'text-teal-500' },
  priority: { icon: Gauge, color: 'text-orange-500' },
  sprint_id: { icon: Hexagon, color: 'text-green-500' },
  epic_id: { icon: Layers, color: 'text-purple-500' },
  estimate: { icon: LayoutGrid, color: 'text-amber-500' },
  deadline: { icon: CalendarDays, color: 'text-red-500' },
  task_type: { icon: Tag, color: 'text-indigo-500' },
  labels: { icon: Tag, color: 'text-pink-500' },
  severity: { icon: ShieldAlert, color: 'text-red-500' },
  name: { icon: Pencil, color: 'text-muted-foreground' },
  description: { icon: Pencil, color: 'text-muted-foreground' },
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
  if (action?.includes('comment')) return { icon: MessageSquare, color: 'text-blue-500' };
  if (action?.includes('attachment') || action?.includes('file')) return { icon: Paperclip, color: 'text-muted-foreground' };
  if (action?.includes('label')) return { icon: Tag, color: 'text-pink-500' };
  if (action?.includes('moved') || action?.includes('state')) return { icon: Hash, color: 'text-blue-500' };
  if (action?.includes('priority')) return { icon: Gauge, color: 'text-orange-500' };
  if (action?.includes('owner') || action?.includes('assigned') || action?.includes('requester')) return { icon: User, color: 'text-violet-500' };
  if (action?.includes('team')) return { icon: Users, color: 'text-teal-500' };
  if (action?.includes('sprint')) return { icon: Hexagon, color: 'text-green-500' };
  if (action?.includes('epic')) return { icon: Layers, color: 'text-purple-500' };
  if (action?.includes('blocked')) return { icon: ShieldAlert, color: 'text-red-500' };
  if (action?.includes('archived')) return { icon: Archive, color: 'text-amber-500' };
  if (action?.includes('created')) return { icon: Play, color: 'text-green-500' };
  if (action?.includes('deadline') || action?.includes('due date')) return { icon: CalendarDays, color: 'text-red-500' };
  if (action?.includes('estimate')) return { icon: LayoutGrid, color: 'text-amber-500' };
  if (action?.includes('type')) return { icon: Tag, color: 'text-indigo-500' };
  if (action?.includes('severity')) return { icon: ShieldAlert, color: 'text-red-500' };
  return { icon: ArrowRightLeft, color: 'text-muted-foreground' };
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
          <ArrowRightLeft className="h-2.5 w-2.5 text-muted-foreground/50" />
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
  onTaskArchived,
}: {
  workspaceId: string;
  taskDetail: TaskDetail;
  states: WorkflowState[];
  initialRecurringSummary: TaskRecurringSummary | null;
  onOpenChange: (open: boolean) => void;
  onTaskUpdated: (task: TaskDetail) => void;
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
  const { copied: linkCopied, copy: copyText } = useCopyToClipboard();
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const [hasGitIntegration, setHasGitIntegration] = useState(false);
  const [recurringSummary, setRecurringSummary] = useState<TaskRecurringSummary | null>(initialRecurringSummary);
  const [recurringDetail, setRecurringDetail] = useState<RecurringTemplateDetail | null>(null);
  const [recurringDialogOpen, setRecurringDialogOpen] = useState(false);
  const [recurringSaving, setRecurringSaving] = useState(false);
  const [panelDragging, setPanelDragging] = useState(false);
  const openFilePickerRef = useRef<(() => void) | null>(null);
  const uploadFilesRef = useRef<((files: FileList | File[]) => Promise<void>) | null>(null);
  const dragCounterRef = useRef(0);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, form.team_id);
  const taskId = taskDetail.task.id;

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

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);

  const currentUser = useAuthStore((s) => s.user);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const [showAllActivity, setShowAllActivity] = useState(false);
  const [showChecklist, setShowChecklist] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [relationshipComposerOpen, setRelationshipComposerOpen] = useState(false);
  const relationshipButtonRef = useRef<HTMLButtonElement>(null);
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
  // ── URL sync (imperative, no effect loop) ──────────────────────
  const lastSyncedTaskRef = useRef<string | null>(null);
  if (taskDetail.task.id !== lastSyncedTaskRef.current) {
    lastSyncedTaskRef.current = taskDetail.task.id;
    const url = new URL(window.location.href);
    url.searchParams.set('task', `${taskDetail.task.display_id}`);
    window.history.replaceState({}, '', url.toString());
  }

  // ── Load comments + activity ───────────────────────────────────
  const reloadComments = useCallback(async () => {
    const res = await pmCommentService.list(workspaceId, 'task', taskDetail.task.id);
    setComments(res.data ?? []);
  }, [workspaceId, taskDetail.task.id]);

  const reloadActivity = useCallback(async () => {
    const res = await pmTaskService.listActivity(workspaceId, taskDetail.task.id, 1, 30);
    setActivity(res.data?.data ?? []);
  }, [workspaceId, taskDetail.task.id]);

  useEffect(() => {
    reloadComments();
    reloadActivity();
  }, [reloadComments, reloadActivity]);

  useEffect(() => {
    if (taskDetail.task.updated_at !== lastSyncedAt.current) {
      lastSyncedAt.current = taskDetail.task.updated_at;
      savedDescriptionRef.current = taskDetail.task.description ?? '';
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
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateTaskRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

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

  useEffect(() => {
    if (!form.epic_id) return;
    const selectedEpic = epics.find((entry) => entry.epic.id === form.epic_id);
    if (!selectedEpic) return;
    if (isEpicSelectableForTaskTeam(selectedEpic.epic.team_id ?? null, form.team_id || null)) {
      return;
    }
    updateField('epic_id', '', { epic_id: '' });
  }, [epics, form.epic_id, form.team_id]);

  useEffect(() => {
    if (!form.sprint_id) return;
    const selectedSprint = sprints.find((entry) => entry.sprint.id === form.sprint_id);
    if (!selectedSprint) return;
    if (isSprintSelectableForTaskTeam(selectedSprint.sprint.team_id ?? null, form.team_id || null)) {
      return;
    }
    updateField('sprint_id', '', { sprint_id: '' });
  }, [form.sprint_id, form.team_id, sprints]);

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
    if (
      saving ||
      !getFlushablePendingTaskPatch(pendingPatch, descriptionPendingUploads)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      const previousDescription = savedDescriptionRef.current;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmTaskService.update(workspaceId, taskId, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save changes');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        onTaskUpdated(data);
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

      void pmTaskService.update(workspaceId, taskId, patch).then(({ data }) => {
        if (!data) {
          return;
        }

        onTaskUpdated(data);
        if (
          patch.sprint_id !== undefined ||
          patch.workflow_state_id !== undefined ||
          patch.estimate !== undefined
        ) {
          queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints', 'planning'] });
        }
      });
    };
  }, [onTaskUpdated, queryClient, taskId, workspaceId]);

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

  // ── Copy link ──────────────────────────────────────────────────
  const copyLink = () =>
    copyText(
      buildTaskCopyUrl({
        currentHref: window.location.href,
        displayId: taskDetail.task.display_id,
        origin: window.location.origin,
        slug: workspace?.slug,
        taskId: taskDetail.task.id,
      }),
    );

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
    if (!form.owner_member_id) return 'No owner';
    return memberNameMap.get(form.owner_member_id) ?? 'No owner';
  }, [form.owner_member_id, memberNameMap]);

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
        <div className="flex min-w-0 flex-1 items-center gap-1 text-sm text-muted-foreground">
          {taskDetail.objective_name && taskDetail.objective_id && workspace && (
            <>
              <Target className="h-3.5 w-3.5 shrink-0 text-blue-500" />
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
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {taskDetail.epic_name && taskDetail.task.epic_id && workspace && (
            <>
              <Hexagon className="h-3.5 w-3.5 shrink-0 text-purple-500" />
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
              <ChevronRight className="h-3 w-3 shrink-0" />
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
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0 font-medium text-foreground">{taskDetail.task.display_id}</span>
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
              <Check className="h-3.5 w-3.5" />
              Copied!
            </span>
          ) : (
            <QuickTooltip label="Copy link">
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={copyLink}>
                <Link2 className="h-3.5 w-3.5" />
              </Button>
            </QuickTooltip>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreVertical className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => { void openRecurringDialog(); }}>
                <RefreshCw className="mr-2 h-4 w-4" />
                {recurringSummary ? 'Edit recurring' : 'Make recurring'}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
                <Archive className="mr-2 h-4 w-4 text-amber-500" />
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
                <Maximize2 className="h-3.5 w-3.5" />
              </Button>
            </QuickTooltip>
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => onOpenChange(false)}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* ── Two-column grid ─────────────────────────────────────── */}
      <div
        className="relative grid min-h-0 flex-1 grid-cols-[1fr_300px] overflow-hidden"
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
              <Upload className="h-8 w-8 text-primary" />
              <p className="text-sm font-medium text-foreground">Drop files to attach</p>
              <p className="text-xs text-muted-foreground">Max 10MB per file</p>
            </div>
          </div>
        )}
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
                          <Check className="h-2.5 w-2.5" />
                        ) : isAutomated ? (
                          <Bot className="h-2.5 w-2.5" />
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
          <div className="mt-4">
            {editingDescription ? (
              <div>
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => updateField('description', html, { description: html })}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none [&_.ProseMirror]:text-sm"
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  teams={mentionTeams}
                  members={assignableMembers}
                  compact
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
                    className="prose prose-sm dark:prose-invert max-w-none text-sm [&_p:empty]:h-1 [&_p:empty]:my-0"
                    onHtmlChange={(html) => updateField('description', html, { description: html })}
                  />
                ) : (
                  <p className="text-sm text-muted-foreground">No description yet</p>
                )}
                <button
                  type="button"
                  className="mt-2 inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                  onClick={() => setEditingDescription(true)}
                >
                  <Pencil className="h-3 w-3" />
                  Edit description
                </button>
              </div>
            )}
          </div>

          {/* Action bar — "Add to Task" */}
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <button
              type="button"
              className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                showChecklist
                  ? 'border-primary/30 bg-primary/10 text-primary'
                  : 'border-border/60 text-muted-foreground hover:bg-accent'
              }`}
              onClick={() => setShowChecklist((v) => !v)}
            >
              <CheckSquare className="h-3 w-3" />
              Checklist
            </button>
            <button
              ref={relationshipButtonRef}
              type="button"
              className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                relationshipComposerOpen
                  ? 'border-primary/30 bg-primary/10 text-primary'
                  : 'border-border/60 text-muted-foreground hover:bg-accent'
              }`}
              onClick={() => setRelationshipComposerOpen(true)}
            >
              <ArrowRightLeft className="h-3 w-3" />
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
              <Link2 className="h-3 w-3" />
              External Links
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-full border border-border/60 px-3 py-1 text-xs font-medium text-muted-foreground hover:bg-accent transition-colors cursor-pointer"
              onClick={() => openFilePickerRef.current?.()}
            >
              <Paperclip className="h-3 w-3" />
              Attach Files
            </button>
          </div>

          {/* Recurring info card */}
          {taskDetail.task.recurring_template_id && recurringSummary ? (
            <button
              type="button"
              className="mt-4 flex w-full items-center gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5 text-left transition-colors hover:bg-muted/40"
              onClick={() => void openRecurringDialog()}
            >
              <RefreshCw className="h-4 w-4 shrink-0 text-muted-foreground" />
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
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            </button>
          ) : null}

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
            externalTriggerRef={relationshipButtonRef}
          />

          {/* Checklist */}
          {showChecklist && (
            <div className="mt-6">
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
            <div className="mt-6">
              <ExternalLinks workspaceId={workspaceId} taskId={taskDetail.task.id} />
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
              onUploadReady={(fn) => { uploadFilesRef.current = fn; }}
            />
          </div>

          {/* Git Links & Agent Runs */}
          {hasGitIntegration && fieldVis.dev_history && (
            <>
              <TaskGitPanel taskId={taskDetail.task.id} workspaceId={workspaceId} />
              <AgentRunPanel
                taskId={taskDetail.task.id}
                workspaceId={workspaceId}
                assignedAgentId={taskDetail.task.assigned_agent_id}
              />
            </>
          )}

          {/* Separator */}
          <Separator className="my-6" />

          {/* Comments + Activity */}
          <div>
            {/* Comments card */}
            <CommentThread
              workspaceId={workspaceId}
              entityType="task"
              entityId={taskDetail.task.id}
              comments={comments}
              currentUserId={currentUser?.id}
              teams={mentionTeams}
              members={assignableMembers}
              onCommentsChange={setComments}
              uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
            />

            {/* Activity section */}
            {activity.length > 0 && (
              <div className="mt-6">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Activity</h3>
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
          <TaskSidebarIdRow displayId={taskDetail.task.display_id} taskName={taskDetail.task.name} taskType={taskDetail.task.task_type} />

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* Team */}
            <MetadataRow icon={Users} label="Team">
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
            <MetadataRow icon={Hash} label="State">
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
                    />
                  ) : null;
                }}
              />
            </MetadataRow>

            {/* ── People ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Owner */}
            <MetadataRow icon={User} label="Owner">
              <MemberPickerPopover
                value={form.owner_member_id || '__none__'}
                members={assignableMembers}
                noneLabel="No owner"
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('owner_member_id', val, { owner_member_id: val });
                }}
                renderTrigger={() => {
                  const selectedMember = findAssignableMember(assignableMembers, form.owner_member_id);
                  return (
                    <>
                      {selectedMember ? (
                        <UserAvatar
                          name={selectedMember.display_name || selectedMember.email}
                          avatarUrl={selectedMember.avatar_url}
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

            {/* Requester */}
            <MetadataRow icon={User} label="Requester">
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
            <MetadataRow icon={Gauge} label="Priority">
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
            <MetadataRow icon={ShieldAlert} label="Severity">
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
            <MetadataRow icon={Hash} label="Type">
              <SidebarPopoverSelect
                value={form.task_type}
                options={taskTypeOptions.map((t) => ({ value: t, label: TASK_TYPE_CONFIG[t].label }))}
                onChange={(v) => updateField('task_type', v as TaskType, { task_type: v as TaskType })}
                renderTrigger={() => (
                  <>
                    <TaskTypeIcon taskType={form.task_type} className="h-3.5 w-3.5" />
                    <span>{TASK_TYPE_CONFIG[form.task_type].label}</span>
                  </>
                )}
                renderOption={(v) => <><TaskTypeIcon taskType={v as TaskType} className="h-4 w-4 shrink-0" /><span>{TASK_TYPE_CONFIG[v as TaskType].label}</span></>}
              />
            </MetadataRow>
            )}

            {/* Labels */}
            {fieldVis.labels && (
            <MetadataRow icon={Tag} label="Labels">
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
            <MetadataRow icon={Layers} label="Epic">
              <SidebarPopoverSelect
                value={form.epic_id || '__none__'}
                options={[
                  { value: '__none__', label: 'None' },
                  ...availableEpics.map((e) => ({ value: e.epic.id, label: e.epic.name })),
                ]}
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
                options={[
                  { value: '__none__', label: 'None' },
                  ...availableSprints.map((i) => ({ value: i.sprint.id, label: i.sprint.name })),
                ]}
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
            <MetadataRow icon={LayoutGrid} label="Estimate">
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
            <MetadataRow icon={CalendarDays} label="Due date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
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

                <MetadataRow icon={Bot} label="Agent">
                  <SidebarPopoverSelect
                    value={delivery.selectedAgentId || '__none__'}
                    options={[
                      { value: '__none__', label: 'No agent' },
                      ...delivery.agents.map((a) => ({ value: a.id, label: `${a.name} · ${agentSummaryLabel(a)}` })),
                    ]}
                    onChange={(v) => {
                      const val = v === '__none__' ? '' : v;
                      if (val) {
                        delivery.handleAgentChange(val);
                      }
                    }}
                    renderTrigger={() => (
                      <>
                        {delivery.selectedAgent && <AgentAvatar agent={delivery.selectedAgent} className="h-5 w-5" />}
                        <span>{delivery.selectedAgent?.name ?? 'No agent'}</span>
                        {delivery.savingAssignment && <Loader2 className="h-3 w-3 animate-spin" />}
                      </>
                    )}
                    renderOption={(v) => {
                      const a = delivery.agents.find((ag) => ag.id === v);
                      if (!a) return null;
                      return <AgentAvatar agent={a} className="h-5 w-5" />;
                    }}
                  />
                </MetadataRow>

                <MetadataRow icon={GitBranch} label="Repository">
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

                <MetadataRow icon={GitBranch} label="Base">
                  <Popover>
                    <Tooltip open={isDeliveryTruncated('delivery-base') ? undefined : false}>
                      <TooltipTrigger asChild>
                        <PopoverTrigger asChild>
                          <button
                            type="button"
                            className="flex w-full min-w-0 items-center gap-1.5 rounded-md px-1.5 py-0.5 text-left text-xs transition-colors hover:bg-accent cursor-pointer"
                          >
                            <span
                              ref={(el) => checkDeliveryTruncation('delivery-base', el)}
                              className="block min-w-0 truncate font-mono"
                            >
                              {delivery.resolvedBaseBranch}
                            </span>
                          </button>
                        </PopoverTrigger>
                      </TooltipTrigger>
                      <TooltipContent align="start">{delivery.resolvedBaseBranch}</TooltipContent>
                    </Tooltip>
                    <PopoverContent className="w-56 p-2" align="start">
                      <Input
                        value={delivery.baseBranch}
                        onChange={(e) => delivery.setBaseBranch(e.target.value)}
                        placeholder={delivery.selectedRepository?.default_branch || 'main'}
                        className="h-7 text-xs"
                      />
                      <Button
                        size="xs"
                        variant="outline"
                        className="mt-1.5 w-full"
                        onClick={delivery.handleSaveDelivery}
                        disabled={delivery.savingTarget || !delivery.repositoryId}
                      >
                        {delivery.savingTarget ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
                        Save
                      </Button>
                    </PopoverContent>
                  </Popover>
                </MetadataRow>

                <MetadataRow icon={GitBranch} label="Branch">
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
                  <MetadataRow icon={Play} label="Status">
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
  onTaskArchived,
}: TaskDetailPanelProps) {
  const openedAtRef = useRef<number | null>(null);

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
        className="w-[80vw] !max-w-[1200px] p-0"
        showCloseButton={false}
        onOpenAutoFocus={(e) => e.preventDefault()}
        onPointerDownOutside={(event) => {
          if (shouldSuppressTaskOverlayOutsideDismiss(openedAtRef.current, Date.now())) {
            event.preventDefault();
          }
        }}
        onInteractOutside={(event) => {
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
            onTaskArchived={onTaskArchived}
          />
        ) : loading ? (
          <div className="flex h-full items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function agentSummaryLabel(agent: { preset_key?: string; runtime_kind?: string; role?: string }) {
  if (agent.role) return agent.role;
  switch (agent.preset_key) {
    case 'code_builder':
      return 'Code Builder';
    case 'review_agent':
      return 'Review Agent';
    case 'task_planner':
      return 'Task Planner';
    case 'story_planner':
      return 'Task Planner';
    case 'epic_planner':
      return 'Epic Planner';
    case 'support_agent':
      return 'Support Agent';
    case 'crm_operator':
      return 'CRM Operator';
    default:
      return agent.runtime_kind === 'native_sdk' ? 'Interactive Agent' : 'Autonomous Agent';
  }
}
