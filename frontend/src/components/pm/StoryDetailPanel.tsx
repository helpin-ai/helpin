import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  MoreVertical,
  Paperclip,
  Play,
  RefreshCw,
  ShieldAlert,
  Tag,
  Target,
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
  STORY_TYPE_CONFIG,
  StoryTypeIcon,
} from '@/lib/pmConstants';
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
import { Attachments } from '@/components/pm/Attachments';
import { ChecklistItems } from '@/components/pm/ChecklistItems';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { StoryGitPanel } from '@/components/pm/StoryGitPanel';
import { useStoryDelivery } from '@/components/pm/StoryDeliveryPanel';
import { AgentRunPanel } from '@/components/pm/AgentRunPanel';
import { cn, getInitials } from '@/lib/utils';
import { gitService } from '@/lib/services/gitService';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import { pmStoryService } from '@/lib/services/pmStoryService';
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
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { StorySidebarIdRow } from '@/components/pm/StorySidebarIdRow';
import { Badge } from '@/components/ui/badge';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTeamFieldVisibilityForTeam, useAutomationRulesByWorkflow } from '@/hooks/queries';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { CommentThread } from '@/components/pm/CommentThread';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { StoryRelationshipsSection } from '@/components/pm/StoryRelationshipsSection';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Input } from '@/components/ui/input';
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
  StoryDetail,
  StoryRecurringSummary,
  StoryType,
  UpdateStoryRequest,
  WorkflowState,
} from '@/lib/pmTypes';
import { toast } from 'sonner';

// ── Types ──────────────────────────────────────────────────────────

interface StoryDetailPanelProps {
  workspaceId: string;
  open: boolean;
  loading?: boolean;
  onOpenChange: (open: boolean) => void;
  storyDetail: StoryDetail | null;
  states: WorkflowState[];
  onStoryUpdated: (story: StoryDetail) => void;
  onStoryArchived: (storyId: string) => void;
}

interface FormState {
  name: string;
  description: string;
  story_type: StoryType;
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
const storyTypeOptions: StoryType[] = ['feature', 'bug', 'chore'];

const buildFormState = (story: StoryDetail): FormState => ({
  name: story.story.name,
  description: story.story.description ?? '',
  story_type: story.story.story_type,
  workflow_state_id: story.story.workflow_state_id,
  priority: story.story.priority,
  severity: story.story.severity,
  estimate:
    story.story.estimate === undefined || story.story.estimate === null
      ? ''
      : String(story.story.estimate),
  deadline: story.story.deadline ? story.story.deadline.slice(0, 10) : '',
  epic_id: story.story.epic_id ?? '',
  sprint_id: story.story.sprint_id ?? '',
  team_id: story.story.team_id ?? '',
  owner_member_id: story.story.owner_member_id ?? '',
  requester_member_id: story.story.requester_member_id ?? '',
  blocker: story.story.blocker ?? '',
});

// ── Helpers ────────────────────────────────────────────────────────

function formatRelativeTime(iso: string) {
  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return iso;
  }
}

function userInitials(user?: { full_name?: string; email?: string } | null): string {
  return getInitials(user?.full_name || user?.email);
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
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
      <span className="text-xs text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

// ── Sidebar Popover Select ─────────────────────────────────────────

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
  renderOption,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
  renderOption?: (value: T) => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors cursor-pointer
                ${value === option.value ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
              `}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              {renderOption ? renderOption(option.value) : null}
              <span className="truncate">{option.label}</span>
              {value === option.value && <Check className="ml-auto h-3 w-3 shrink-0" />}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Timeline Entry ─────────────────────────────────────────────────

type TimelineItem =
  | { kind: 'activity'; data: ActivityLogEntry; time: string }
  | { kind: 'comment'; data: CommentWithAuthor; time: string };

function TimelineEntry({ item }: { item: TimelineItem }) {
  if (item.kind === 'comment') {
    const { comment, author } = item.data;
    return (
      <div className="flex items-start gap-2">
        <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent text-[8px] font-medium">
          {userInitials(author)}
        </div>
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

  return (
    <div className="flex items-center gap-2">
      <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[8px] font-medium text-muted-foreground">
        {userInitials(actor)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-1.5">
          <span className="text-xs font-medium">{actor?.full_name || actor?.email || 'System'}</span>
          <span className="text-[11px] text-muted-foreground">{activity.action}</span>
          <span className="ml-auto shrink-0 text-[11px] text-muted-foreground">{formatRelativeTime(activity.created_at)}</span>
        </div>
      </div>
    </div>
  );
}

// ── Main Body ──────────────────────────────────────────────────────

function StoryDetailPanelBody({
  workspaceId,
  storyDetail,
  states,
  onOpenChange,
  onStoryUpdated,
  onStoryArchived,
}: {
  workspaceId: string;
  storyDetail: StoryDetail;
  states: WorkflowState[];
  onOpenChange: (open: boolean) => void;
  onStoryUpdated: (story: StoryDetail) => void;
  onStoryArchived: (storyId: string) => void;
}) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [form, setForm] = useState<FormState>(() => buildFormState(storyDetail));
  const [pendingPatch, setPendingPatch] = useState<UpdateStoryRequest>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const { copied: linkCopied, copy: copyText } = useCopyToClipboard();
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [hasGitIntegration, setHasGitIntegration] = useState(false);
  const [recurringSummary, setRecurringSummary] = useState<StoryRecurringSummary | null>(null);
  const [recurringDetail, setRecurringDetail] = useState<RecurringTemplateDetail | null>(null);
  const [recurringDialogOpen, setRecurringDialogOpen] = useState(false);
  const [recurringSaving, setRecurringSaving] = useState(false);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, form.team_id);

  // Check if GitHub is connected
  useEffect(() => {
    gitService.listIntegrations(workspaceId).then((res) => {
      setHasGitIntegration((res.data ?? []).some((i) => i.active));
    });
  }, [workspaceId]);

  const loadRecurringSummary = useCallback(async (currentStoryId: string) => {
    const { data, error, status } = await pmRecurringTemplateService.getByStory(workspaceId, currentStoryId);
    if (error && status !== 204) {
      toast.error(error);
      return;
    }
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
          story_id: storyDetail.story.id,
          config: value.config,
        })
      : await pmRecurringTemplateService.create({
          workspace_id: workspaceId,
          story_id: storyDetail.story.id,
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
    await loadRecurringSummary(storyDetail.story.id);
    setRecurringDialogOpen(false);
    toast.success(recurringSummary?.template_id ? 'Recurring template updated' : 'Story is now recurring');
  }, [workspaceId, storyDetail.story.id, recurringSummary?.template_id, loadRecurringSummary]);

  useEffect(() => {
    void loadRecurringSummary(storyDetail.story.id);
  }, [storyDetail.story.id, loadRecurringSummary]);

  // ── Delivery (sidebar rows) ──────────────────────────────────────
  const delivery = useStoryDelivery(workspaceId, storyDetail, onStoryUpdated);

  // Re-sync form when storyDetail changes externally (e.g. real-time WS update)
  const lastSyncedAt = useRef(storyDetail.story.updated_at);
  const savedDescriptionRef = useRef(storyDetail.story.description ?? '');
  useEffect(() => {
    if (storyDetail.story.updated_at !== lastSyncedAt.current) {
      lastSyncedAt.current = storyDetail.story.updated_at;
      savedDescriptionRef.current = storyDetail.story.description ?? '';
      // Only reset form if no unsaved edits
      if (Object.keys(pendingPatch).length === 0 && !saving) {
        setForm(buildFormState(storyDetail));
      }
    }
  }, [storyDetail, pendingPatch, saving]);

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
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form.team_id ? [form.team_id] : []),
    [teams, form.team_id],
  );

  // ── URL sync ───────────────────────────────────────────────────
  useEffect(() => {
    const url = new URL(window.location.href);
    url.searchParams.set('story', `${storyDetail.story.display_id}`);
    window.history.replaceState({}, '', url.toString());

    return () => {
      const cleanupUrl = new URL(window.location.href);
      cleanupUrl.searchParams.delete('story');
      window.history.replaceState({}, '', cleanupUrl.toString());
    };
  }, [storyDetail]);

  // ── Load comments + activity ───────────────────────────────────
  const reloadComments = useCallback(async () => {
    const res = await pmCommentService.list(workspaceId, 'story', storyDetail.story.id);
    setComments(res.data ?? []);
  }, [workspaceId, storyDetail.story.id]);

  const reloadActivity = useCallback(async () => {
    const res = await pmStoryService.listActivity(workspaceId, storyDetail.story.id, 1, 30);
    setActivity(res.data?.data ?? []);
  }, [workspaceId, storyDetail.story.id]);

  useEffect(() => {
    reloadComments();
    reloadActivity();
  }, [reloadComments, reloadActivity]);

  // Re-fetch comments when comment events arrive; activity on any story change
  useEffect(() => {
    const storyId = storyDetail.story.id;
    const onChildEvent = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === storyId && d?.entity === 'comment') {
        reloadComments();
        reloadActivity();
      }
    };
    const onStoryEvent = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.entity_id === storyId) reloadActivity();
    };
    window.addEventListener('story-child-updated', onChildEvent);
    window.addEventListener('story-updated', onStoryEvent);
    return () => {
      window.removeEventListener('story-child-updated', onChildEvent);
      window.removeEventListener('story-updated', onStoryEvent);
    };
  }, [storyDetail.story.id, reloadComments, reloadActivity]);

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

  // ── Auto-show checklist / external links if items exist ────────
  useEffect(() => {
    (async () => {
      const [clRes, elRes] = await Promise.all([
        pmChecklistService.list(workspaceId, storyDetail.story.id),
        pmExternalLinkService.list(workspaceId, storyDetail.story.id),
      ]);
      if (clRes.data && clRes.data.length > 0) setShowChecklist(true);
      if (elRes.data && elRes.data.length > 0) setShowExternalLinks(true);
    })();
  }, [workspaceId, storyDetail]);

  // ── Auto-save debounce ─────────────────────────────────────────
  useEffect(() => {
    if (
      saving ||
      Object.keys(pendingPatch).length === 0 ||
      (pendingPatch.description !== undefined && descriptionPendingUploads > 0)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      const previousDescription = savedDescriptionRef.current;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmStoryService.update(workspaceId, storyDetail.story.id, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save changes');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        onStoryUpdated(data);
        const nextDescription = data.story.description ?? '';
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
  }, [workspaceId, storyDetail, pendingPatch, saving, onStoryUpdated, descriptionPendingUploads]);

  const queuePatch = (patch: UpdateStoryRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateStoryRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: AttachmentResponse) => {
      if (!extractInlineAttachmentIds(form.description).includes(entry.attachment.id)) {
        return 'fallback' as const;
      }
      if (!window.confirm('Delete this image from the description and attachments?')) {
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

      const { data, error } = await pmStoryService.update(workspaceId, storyDetail.story.id, {
        description: nextDescription,
      });
      if (error || !data) {
        setForm((current) => ({ ...current, description: previousDescription }));
        setSaveError(error ?? 'Failed to save changes');
        setSaving(false);
        return 'prevent' as const;
      }

      setSaveError(null);
      onStoryUpdated(data);
      savedDescriptionRef.current = data.story.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [form.description, onStoryUpdated, storyDetail.story.id, workspaceId],
  );

  // ── Archive ────────────────────────────────────────────────────
  const archiveStory = async () => {
    const { error } = await pmStoryService.remove(workspaceId, storyDetail.story.id);
    if (error) {
      setSaveError(error);
      return;
    }
    onStoryArchived(storyDetail.story.id);
    onOpenChange(false);
  };

  // ── Copy link ──────────────────────────────────────────────────
  const copyLink = () => copyText(window.location.href);

  // ── Pipeline automation rules ──────────────────────────────────
  const workflowId = states[0]?.workflow_id;
  const { data: pipelineRules } = useAutomationRulesByWorkflow(workspaceId, workflowId);
  const automatedStateIds = useMemo(() => {
    const ids = new Set<string>();
    if (!pipelineRules) return ids;
    for (const rule of pipelineRules) {
      if (rule.enabled && rule.trigger_type === 'story.state_entered' && rule.action_type === 'run_agent') {
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
    if (!form.epic_id) return 'No epic';
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? 'No epic';
  }, [form.epic_id, epics]);

  const currentSprintName = useMemo(() => {
    if (!form.sprint_id) return 'No sprint';
    return sprints.find((i) => i.sprint.id === form.sprint_id)?.sprint.name ?? 'No sprint';
  }, [form.sprint_id, sprints]);

  const currentTeamName = useMemo(() => {
    if (!form.team_id) return 'No team';
    return teams.find((t) => t.id === form.team_id)?.name ?? 'No team';
  }, [form.team_id, teams]);

  const currentOwnerName = useMemo(() => {
    if (!form.owner_member_id) return 'No owner';
    return memberNameMap.get(form.owner_member_id) ?? 'No owner';
  }, [form.owner_member_id, memberNameMap]);

  const currentRequesterName = useMemo(() => {
    if (!form.requester_member_id) return 'No requester';
    return memberNameMap.get(form.requester_member_id) ?? 'No requester';
  }, [form.requester_member_id, memberNameMap]);

  const storyLabels = storyDetail.labels ?? [];

  useEffect(() => {
    if (!form) return;
    const validLabelIds = storyLabels
      .filter((label) => !label.team_id || (form.team_id ? label.team_id === form.team_id : false))
      .map((label) => label.id);
    if (validLabelIds.length === storyLabels.length) return;
    void (async () => {
      await pmStoryService.syncLabels(workspaceId, storyDetail.story.id, storyLabels.map((label) => label.id), validLabelIds);
      const res = await pmStoryService.get(workspaceId, storyDetail.story.id);
      if (res.data) onStoryUpdated(res.data);
    })();
  }, [form?.team_id, onStoryUpdated, storyDetail.story.id, storyLabels, workspaceId]);

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          {storyDetail.objective_name && storyDetail.objective_id && workspace && (
            <>
              <Target className="h-3.5 w-3.5 shrink-0 text-blue-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
                onClick={() => {
                  onOpenChange(false);
                  navigate({ to: '/w/$slug/pm/objectives/$objectiveId', params: { slug: workspace.slug, objectiveId: storyDetail.objective_id! } });
                }}
              >
                {storyDetail.objective_name}
              </button>
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {storyDetail.epic_name && storyDetail.story.epic_id && workspace && (
            <>
              <Hexagon className="h-3.5 w-3.5 shrink-0 text-purple-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
                onClick={() => {
                  onOpenChange(false);
                  navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug: workspace.slug, epicId: storyDetail.story.epic_id! } });
                }}
              >
                {storyDetail.epic_name}
              </button>
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentSprintName !== 'No sprint' && form.sprint_id && workspace && (
            <>
              <SprintIcon className="h-3.5 w-3.5 shrink-0 text-green-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
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
          <span className="shrink-0 font-medium text-foreground">{storyDetail.story.display_id}</span>
          {storyDetail.story.recurring_template_id ? (
            <RecurringTemplateBadge
              compact
              occurrenceNumber={storyDetail.story.recurring_occurrence_number}
            />
          ) : null}
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
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
            <QuickTooltip label="Open full page">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                onClick={() => {
                  onOpenChange(false);
                  navigate({
                    to: '/w/$slug/pm/stories/$storyId',
                    params: { slug: workspace.slug, storyId: storyDetail.story.id },
                  });
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
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
        {/* ── Left column (main content) ────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-5 pb-40">
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
            aria-label="Story title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Recurring info card */}
          {recurringSummary ? (
            <button
              type="button"
              className="mt-3 flex w-full items-center gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5 text-left transition-colors hover:bg-muted/40"
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

          {/* Description */}
          <div className="mt-4">
            <TiptapEditor
              content={form.description}
              onChange={(html) => updateField('description', html, { description: html })}
              placeholder="Add a description..."
              className="border-transparent shadow-none"
              uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
              onUploadStateChange={setDescriptionPendingUploads}
              teams={mentionTeams}
              members={assignableMembers}
            />
          </div>

          {/* Action bar — "Add to Story" */}
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
              onClick={() => {
                document.getElementById('attachments-section')?.scrollIntoView({ behavior: 'smooth' });
              }}
            >
              <Paperclip className="h-3 w-3" />
              Attach Files
            </button>
          </div>

          <StoryRelationshipsSection
            workspaceId={workspaceId}
            storyId={storyDetail.story.id}
            storyName={storyDetail.story.name}
            storyDisplayId={storyDetail.story.display_id}
            workflowId={storyDetail.story.workflow_id}
            workflowStateId={storyDetail.story.workflow_state_id}
            epicId={storyDetail.story.epic_id}
            sprintId={storyDetail.story.sprint_id}
            teamId={storyDetail.story.team_id}
            storyType={storyDetail.story.story_type}
            priority={storyDetail.story.priority}
            severity={storyDetail.story.severity}
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
                storyId={storyDetail.story.id}
                members={assignableMembers}
                teams={mentionTeams}
              />
            </div>
          )}

          {/* External Links */}
          {showExternalLinks && (
            <div className="mt-6">
              <ExternalLinks workspaceId={workspaceId} storyId={storyDetail.story.id} />
            </div>
          )}

          {/* Attachments */}
          <div className="mt-6" id="attachments-section">
            <Attachments
              workspaceId={workspaceId}
              entityType="story"
              entityId={storyDetail.story.id}
              memberNameMap={memberNameMap}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
            />
          </div>

          {/* Git Links & Agent Runs */}
          {hasGitIntegration && fieldVis.dev_history && (
            <>
              <StoryGitPanel storyId={storyDetail.story.id} workspaceId={workspaceId} />
              <AgentRunPanel
                storyId={storyDetail.story.id}
                workspaceId={workspaceId}
                assignedAgentId={storyDetail.story.assigned_agent_id}
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
              entityType="story"
              entityId={storyDetail.story.id}
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
                <div className="mt-3 space-y-2">
                  {!showAllActivity && activity.length > 5 && (
                    <button
                      type="button"
                      className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                      onClick={() => setShowAllActivity(true)}
                    >
                      Show {activity.length - 5} older entries...
                    </button>
                  )}
                  {(showAllActivity ? activity : activity.slice(0, 5)).map((entry) => (
                    <TimelineEntry key={`a-${entry.activity.id}`} item={{ kind: 'activity', data: entry, time: entry.activity.created_at }} />
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* ── Right column (sidebar) ────────────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5 pb-40">
          <StorySidebarIdRow displayId={storyDetail.story.display_id} />

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* Team */}
            <MetadataRow icon={Users} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  ...(teams.length === 0 ? [{ value: '__none__', label: 'No team' }] : []),
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('team_id', val, { team_id: val || undefined });
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
                  <>
                    {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5" />}
                    <span>{currentState?.name ?? 'Select'}</span>
                  </>
                )}
                renderOption={(v) => {
                  const s = states.find((st) => st.id === v);
                  return s ? <StateTypeIcon stateType={s.state_type} className="h-4 w-4 shrink-0" /> : null;
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
                renderOption={(v) => <PriorityIcon priority={v as Priority} className="h-4 w-4 shrink-0" />}
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
                renderOption={(v) => <SeverityIcon severity={v as Severity} className="h-4 w-4 shrink-0" />}
              />
            </MetadataRow>
            )}

            {/* Type */}
            {fieldVis.story_type && (
            <MetadataRow icon={Hash} label="Type">
              <SidebarPopoverSelect
                value={form.story_type}
                options={storyTypeOptions.map((t) => ({ value: t, label: STORY_TYPE_CONFIG[t].label }))}
                onChange={(v) => updateField('story_type', v as StoryType, { story_type: v as StoryType })}
                renderTrigger={() => (
                  <>
                    <StoryTypeIcon storyType={form.story_type} className="h-3.5 w-3.5" />
                    <span>{STORY_TYPE_CONFIG[form.story_type].label}</span>
                  </>
                )}
                renderOption={(v) => <StoryTypeIcon storyType={v as StoryType} className="h-4 w-4 shrink-0" />}
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
                selectedLabelIds={storyLabels.map((l) => l.id)}
                onLabelsChange={setAllLabels}
                onChange={async (labelIds) => {
                  await pmStoryService.syncLabels(workspaceId, storyDetail.story.id, storyLabels.map((l) => l.id), labelIds);
                  const res = await pmStoryService.get(workspaceId, storyDetail.story.id);
                  if (res.data) onStoryUpdated(res.data);
                }}
              />
            </MetadataRow>
            )}

            {/* ── Planning ── */}
            {(fieldVis.epic || fieldVis.sprint || fieldVis.estimate || fieldVis.due_date || fieldVis.blocked) && <div className="col-span-3 h-px bg-border/40 my-1" />}

            {/* Epic */}
            {fieldVis.epic && (
            <MetadataRow icon={Layers} label="Epic">
              <SidebarPopoverSelect
                value={form.epic_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No epic' },
                  ...epics.map((e) => ({ value: e.epic.id, label: e.epic.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('epic_id', val, { epic_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentEpicName}</span>}
              />
            </MetadataRow>
            )}

            {/* Sprint */}
            {fieldVis.sprint && (
            <MetadataRow icon={SprintIcon} label="Sprint">
              <SidebarPopoverSelect
                value={form.sprint_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No sprint' },
                  ...sprints.map((i) => ({ value: i.sprint.id, label: i.sprint.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('sprint_id', val, { sprint_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentSprintName}</span>}
              />
            </MetadataRow>
            )}

            {/* Estimate */}
            {fieldVis.estimate && (
            <MetadataRow icon={LayoutGrid} label="Estimate">
              <EstimatePicker
                value={form.estimate}
                teamId={form.team_id}
                onChange={(displayValue, apiValue) => {
                  updateField('estimate', displayValue, {
                    estimate: apiValue,
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
                      ...delivery.agents.map((a) => ({ value: a.id, label: `${a.name} · ${a.capability_profile}` })),
                    ]}
                    onChange={(v) => {
                      const val = v === '__none__' ? '' : v;
                      if (val) {
                        delivery.handleAgentChange(val);
                      }
                    }}
                    renderTrigger={() => (
                      <>
                        {delivery.selectedAgent && <Bot className="h-3.5 w-3.5 text-muted-foreground" />}
                        <span>{delivery.selectedAgent?.name ?? 'No agent'}</span>
                        {delivery.savingAssignment && <Loader2 className="h-3 w-3 animate-spin" />}
                      </>
                    )}
                    renderOption={(v) => {
                      const a = delivery.agents.find((ag) => ag.id === v);
                      if (!a) return null;
                      return <Bot className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />;
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
                      <span className="truncate">{delivery.selectedRepository?.full_name ?? 'None'}</span>
                    )}
                  />
                </MetadataRow>

                <MetadataRow icon={GitBranch} label="Base">
                  <Popover>
                    <PopoverTrigger asChild>
                      <button
                        type="button"
                        className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
                      >
                        <span className="truncate font-mono">{delivery.resolvedBaseBranch}</span>
                      </button>
                    </PopoverTrigger>
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
                  <span className="truncate font-mono text-xs px-1.5 py-0.5">{delivery.branchPreview}</span>
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
            objectType="story"
            objectId={storyDetail.story.id}
            workspaceId={workspaceId}
            includeStoryRelationships={false}
          />
        </aside>
      </div>

      <Dialog open={recurringDialogOpen} onOpenChange={setRecurringDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{recurringSummary ? 'Edit recurring template' : 'Make story recurring'}</DialogTitle>
            <DialogDescription>
              {recurringSummary
                ? 'Update the schedule and future generation behavior for this recurring story.'
                : 'Create a recurring template from this story so future occurrences are generated automatically.'}
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
        title="Archive story"
        description="This story will be hidden from the board and lists. You can restore it later from archived items."
        confirmLabel="Archive"
        variant="default"
        onConfirm={archiveStory}
      />
    </div>
  );
}

// ── Export wrapper ──────────────────────────────────────────────────

export function StoryDetailPanel({
  workspaceId,
  open,
  loading,
  onOpenChange,
  storyDetail,
  states,
  onStoryUpdated,
  onStoryArchived,
}: StoryDetailPanelProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[75vw] !max-w-[75vw] p-0" showCloseButton={false}>
        <SheetTitle className="sr-only">Story Detail</SheetTitle>
        {storyDetail ? (
          <StoryDetailPanelBody
            key={storyDetail.story.id}
            workspaceId={workspaceId}
            storyDetail={storyDetail}
            states={states}
            onOpenChange={onOpenChange}
            onStoryUpdated={onStoryUpdated}
            onStoryArchived={onStoryArchived}
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
