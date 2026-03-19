import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { useTitle } from '@/hooks/useTitle';
import {
  Archive,
  ArrowLeft,
  ArrowRightLeft,
  CalendarDays,
  Check,
  CheckSquare,
  ChevronRight,
  Gauge,
  Hash,
  Hexagon,
  Layers,
  LayoutGrid,
  Link2,
  Loader2,
  MoreHorizontal,
  Paperclip,
  ShieldAlert,
  Tag,
  Target,
  User,
  Users,
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
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { Attachments } from '@/components/pm/Attachments';
import { ChecklistItems } from '@/components/pm/ChecklistItems';
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { DatePicker } from '@/components/ui/date-picker';
import { getInitials } from '@/lib/utils';
import { gitService } from '@/lib/services/gitService';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { StorySidebarIdRow } from '@/components/pm/StorySidebarIdRow';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useAuthStore } from '@/stores/authStore';
import { useWorkflows } from '@/hooks/queries/useWorkflows';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type {
  ActivityLogEntry,
  CommentWithAuthor,
  EpicWithStats,
  Label,
  Priority,
  Severity,
  SprintWithStats,
  StoryDetail,
  StoryImplementationBrief,
  StoryType,
  UpdateStoryRequest,
  WorkflowState,
} from '@/lib/pmTypes';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { CommentThread } from '@/components/pm/CommentThread';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { StoryRelationshipsSection } from '@/components/pm/StoryRelationshipsSection';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { StoryDeliveryPanel } from '@/components/pm/StoryDeliveryPanel';
import { StoryGitPanel } from '@/components/pm/StoryGitPanel';
import { AgentRunPanel } from '@/components/pm/AgentRunPanel';
import { useTeamFieldVisibilityForTeam } from '@/hooks/queries/useSettings';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/stories/$storyId');

// ── Types ──────────────────────────────────────────────────────────

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

function ActivityEntry({ entry }: { entry: ActivityLogEntry }) {
  const { activity, actor } = entry;

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

// ── Main Page ──────────────────────────────────────────────────────

export function StoryDetailPage() {
  const { storyId, slug } = routeApi.useParams();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;
  const currentUser = useAuthStore((s) => s.user);

  const { data: workflows = [] } = useWorkflows(workspaceId ?? '');

  const [storyDetail, setStoryDetail] = useState<StoryDetail | null>(null);
  const [states, setStates] = useState<WorkflowState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateStoryRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const { copied: linkCopied, copy: copyText } = useCopyToClipboard();
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [hasGitIntegration, setHasGitIntegration] = useState(false);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);
  const [showAllActivity, setShowAllActivity] = useState(false);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);

  const [showChecklist, setShowChecklist] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [relationshipComposerOpen, setRelationshipComposerOpen] = useState(false);
  const relationshipButtonRef = useRef<HTMLButtonElement>(null);

  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId!, storyDetail?.story.team_id);
  const { teams } = useAccessibleTeams(workspaceId ?? '');
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId ?? '');
  const memberNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form?.team_id ? [form.team_id] : []),
    [teams, form?.team_id],
  );

  useTitle(form?.name ? `${storyDetail?.story.display_id} ${form.name}` : 'Story');

  // Check if GitHub is connected
  useEffect(() => {
    if (!workspaceId) return;
    gitService.listIntegrations(workspaceId).then((res) => {
      setHasGitIntegration((res.data ?? []).some((i) => i.active));
    });
  }, [workspaceId]);

  // ── Load all data in parallel ───────────────────────────────────
  useEffect(() => {
    if (!workspaceId) return;
    (async () => {
      setLoading(true);
      setError(null);
      const [storyRes, epicsRes, sprintsRes, labelsRes, commentsRes, activityRes, clRes, elRes] =
        await Promise.all([
          pmStoryService.get(workspaceId, storyId),
          pmEpicService.list(workspaceId, { archived: false }),
          pmSprintService.list(workspaceId, { archived: false }),
          pmLabelService.list(workspaceId),
          pmCommentService.list(workspaceId, 'story', storyId),
          pmStoryService.listActivity(workspaceId, storyId, 1, 30),
          pmChecklistService.list(workspaceId, storyId),
          pmExternalLinkService.list(workspaceId, storyId),
        ]);
      if (storyRes.error || !storyRes.data) {
        setError(storyRes.error ?? 'Story not found');
        setLoading(false);
        return;
      }
      const detail = storyRes.data;
      setStoryDetail(detail);
      setForm(buildFormState(detail));
      savedDescriptionRef.current = detail.story.description ?? '';

      setEpics(epicsRes.data ?? []);
      setSprints(sprintsRes.data ?? []);
      setAllLabels(labelsRes.data ?? []);
      setComments(commentsRes.data ?? []);
      setActivity(activityRes.data?.data ?? []);
      if (clRes.data && clRes.data.length > 0) setShowChecklist(true);
      if (elRes.data && elRes.data.length > 0) setShowExternalLinks(true);

      setLoading(false);
    })();
  }, [workspaceId, storyId]);

  // ── Sync workflow states from query data ──────────────────────────
  useEffect(() => {
    if (!storyDetail || workflows.length === 0) return;
    const wf = workflows.find((w) => w.workflow.id === storyDetail.story.workflow_id);
    setStates(wf?.states ?? []);
  }, [workflows, storyDetail]);

  // ── Reload helpers (for real-time events) ───────────────────────
  const reloadComments = useCallback(async () => {
    if (!workspaceId) return;
    const res = await pmCommentService.list(workspaceId, 'story', storyId);
    setComments(res.data ?? []);
  }, [workspaceId, storyId]);

  const reloadActivity = useCallback(async () => {
    if (!workspaceId) return;
    const res = await pmStoryService.listActivity(workspaceId, storyId, 1, 30);
    setActivity(res.data?.data ?? []);
  }, [workspaceId, storyId]);

  // ── Real-time event listeners ───────────────────────────────────
  useEffect(() => {
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
  }, [storyId, reloadComments, reloadActivity]);

  // ── Re-sync form when storyDetail changes externally ────────────
  const lastSyncedAt = useRef(storyDetail?.story.updated_at);
  const savedDescriptionRef = useRef('');
  useEffect(() => {
    if (!storyDetail) return;
    if (storyDetail.story.updated_at !== lastSyncedAt.current) {
      lastSyncedAt.current = storyDetail.story.updated_at;
      savedDescriptionRef.current = storyDetail.story.description ?? '';
      if (Object.keys(pendingPatch).length === 0 && !saving) {
        setForm(buildFormState(storyDetail));
      }
    }
  }, [storyDetail, pendingPatch, saving]);

  // ── Auto-save debounce ──────────────────────────────────────────
  const storyEntityId = storyDetail?.story.id;
  useEffect(() => {
    if (
      saving ||
      Object.keys(pendingPatch).length === 0 ||
      !workspaceId ||
      !storyEntityId ||
      (pendingPatch.description !== undefined && descriptionPendingUploads > 0)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      const previousDescription = savedDescriptionRef.current;
      setPendingPatch({});
      setSaving(true);
      const { data, error: err } = await pmStoryService.update(workspaceId, storyEntityId, patch);
      if (err || !data) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setStoryDetail(data);
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
  }, [workspaceId, storyEntityId, pendingPatch, saving, descriptionPendingUploads]);

  const queuePatch = (patch: UpdateStoryRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateStoryRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: import('@/lib/pmTypes').AttachmentResponse) => {
      if (!workspaceId || !storyDetail || !form) {
        return 'fallback' as const;
      }
      if (!extractInlineAttachmentIds(form.description).includes(entry.attachment.id)) {
        return 'fallback' as const;
      }
      if (!window.confirm('Delete this image from the description and attachments?')) {
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

      const { data, error: err } = await pmStoryService.update(workspaceId, storyDetail.story.id, {
        description: nextDescription,
      });
      if (err || !data) {
        setForm((current) => (current ? { ...current, description: previousDescription } : current));
        setSaveError(err ?? 'Failed to save');
        setSaving(false);
        return 'prevent' as const;
      }

      setSaveError(null);
      setStoryDetail(data);
      savedDescriptionRef.current = data.story.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [workspaceId, storyDetail, form],
  );

  // ── Archive ─────────────────────────────────────────────────────
  const archiveStory = async () => {
    if (!workspaceId || !storyDetail) return;
    const { error: err } = await pmStoryService.remove(workspaceId, storyDetail.story.id);
    if (err) {
      setSaveError(err);
      return;
    }
    navigate({ to: '/w/$slug/pm/stories', params: { slug } });
  };

  // ── Copy link ───────────────────────────────────────────────────
  const copyLink = () => copyText(window.location.href);

  // ── Derived data ────────────────────────────────────────────────
  const currentState = useMemo(
    () => states.find((s) => s.id === form?.workflow_state_id),
    [states, form?.workflow_state_id],
  );

  const currentEpicName = useMemo(() => {
    if (!form?.epic_id) return 'No epic';
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? 'No epic';
  }, [form?.epic_id, epics]);

  const currentSprintName = useMemo(() => {
    if (!form?.sprint_id) return 'No sprint';
    return sprints.find((i) => i.sprint.id === form.sprint_id)?.sprint.name ?? 'No sprint';
  }, [form?.sprint_id, sprints]);

  const currentTeamName = useMemo(() => {
    if (!form?.team_id) return 'No team';
    return teams.find((t) => t.id === form.team_id)?.name ?? 'No team';
  }, [form?.team_id, teams]);

  const currentOwnerName = useMemo(() => {
    if (!form?.owner_member_id) return 'No owner';
    return memberNameMap.get(form.owner_member_id) ?? 'No owner';
  }, [form?.owner_member_id, memberNameMap]);

  const currentRequesterName = useMemo(() => {
    if (!form?.requester_member_id) return 'No requester';
    return memberNameMap.get(form.requester_member_id) ?? 'No requester';
  }, [form?.requester_member_id, memberNameMap]);

  const storyLabels = storyDetail?.labels ?? [];

  useEffect(() => {
    if (!form) return;
    const validLabelIds = storyLabels
      .filter((label) => !label.team_id || (form.team_id ? label.team_id === form.team_id : false))
      .map((label) => label.id);
    if (validLabelIds.length === storyLabels.length) return;
    void (async () => {
      if (!workspaceId || !storyDetail) return;
      await pmStoryService.syncLabels(workspaceId, storyDetail.story.id, storyLabels.map((label) => label.id), validLabelIds);
      const res = await pmStoryService.get(workspaceId, storyDetail.story.id);
      if (res.data) {
        setStoryDetail(res.data);
        setForm(buildFormState(res.data));
      }
    })();
  }, [form?.team_id, storyLabels, storyDetail, workspaceId]);

  const goBack = () => navigate({ to: '/w/$slug/pm/stories', params: { slug } });

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !storyDetail || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Story not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Stories
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          {storyDetail.objective_name && storyDetail.objective_id && (
            <>
              <Target className="h-3.5 w-3.5 shrink-0 text-blue-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
                onClick={() => navigate({ to: '/w/$slug/pm/objectives/$objectiveId', params: { slug, objectiveId: storyDetail.objective_id! } })}
              >
                {storyDetail.objective_name}
              </button>
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {storyDetail.epic_name && storyDetail.story.epic_id && (
            <>
              <Hexagon className="h-3.5 w-3.5 shrink-0 text-purple-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
                onClick={() => navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: storyDetail.story.epic_id! } })}
              >
                {storyDetail.epic_name}
              </button>
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentSprintName !== 'No sprint' && form.sprint_id && (
            <>
              <SprintIcon className="h-3.5 w-3.5 shrink-0 text-green-500" />
              <button
                type="button"
                className="shrink-0 max-w-[160px] truncate hover:text-foreground transition-colors cursor-pointer"
                onClick={() => navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug, sprintId: form.sprint_id! } })}
              >
                {currentSprintName}
              </button>
              <ChevronRight className="h-3 w-3 shrink-0" />
            </>
          )}
          {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0 font-medium text-foreground">{storyDetail.story.display_id}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <FollowButton entityType="story" entityId={storyDetail.story.id} />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7 ml-2">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={copyLink}>
                <Link2 className="mr-2 h-4 w-4" />
                {linkCopied ? 'Copied!' : 'Copy link'}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
                <Archive className="mr-2 h-4 w-4 text-amber-500" />
                Archive
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column (main content) ────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
          {/* Title */}
          <input
            type="text"
            aria-label="Story title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Description */}
          <div className="mt-4">
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
          </div>

          {/* Implementation Brief (from planning) */}
          {storyDetail?.story.implementation_brief && (
            <StoryImplementationBriefPanel brief={storyDetail.story.implementation_brief} />
          )}

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
            workspaceId={workspaceId!}
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
                workspaceId={workspaceId!}
                storyId={storyDetail.story.id}
                members={assignableMembers}
                teams={mentionTeams}
              />
            </div>
          )}

          {/* External Links */}
          {showExternalLinks && (
            <div className="mt-6">
              <ExternalLinks workspaceId={workspaceId!} storyId={storyDetail.story.id} />
            </div>
          )}

          {/* Attachments */}
          <div className="mt-6" id="attachments-section">
            <Attachments
              workspaceId={workspaceId!}
              entityType="story"
              entityId={storyDetail.story.id}
              memberNameMap={memberNameMap}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
            />
          </div>

          <Separator className="my-6" />

          {/* Comments + Activity */}
          <div>
            {/* Comments card */}
            <CommentThread
              workspaceId={workspaceId!}
              entityType="story"
              entityId={storyDetail.story.id}
              comments={comments}
              currentUserId={currentUser?.id}
              teams={mentionTeams}
              members={assignableMembers}
              onCommentsChange={setComments}
              uploadConfig={{ workspaceId: workspaceId!, entityType: 'editor_upload', entityId: workspaceId! }}
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
                    <ActivityEntry key={`a-${entry.activity.id}`} entry={entry} />
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <StorySidebarIdRow displayId={storyDetail.story.display_id} />

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* Team */}
            <MetadataRow icon={Users} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No team' },
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
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Priority */}
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

            {/* Severity */}
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

            {/* Type */}
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

            {/* Labels */}
            <MetadataRow icon={Tag} label="Labels">
              <LabelPicker
                workspaceId={workspaceId!}
                teamId={form.team_id || undefined}
                labels={allLabels}
                selectedLabelIds={storyLabels.map((l) => l.id)}
                onLabelsChange={setAllLabels}
                onChange={async (labelIds) => {
                  await pmStoryService.syncLabels(workspaceId!, storyDetail.story.id, storyLabels.map((l) => l.id), labelIds);
                  const res = await pmStoryService.get(workspaceId!, storyDetail.story.id);
                  if (res.data) {
                    setStoryDetail(res.data);
                    setForm(buildFormState(res.data));
                  }
                }}
              />
            </MetadataRow>

            {/* ── Planning ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Epic */}
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

            {/* Sprint */}
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

            {/* Estimate */}
            <MetadataRow icon={LayoutGrid} label="Estimate">
              <Popover>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
                  >
                    {form.estimate ? `${form.estimate} pts` : 'None'}
                  </button>
                </PopoverTrigger>
                <PopoverContent className="w-36 p-3" align="start">
                  <Input
                    type="number"
                    min={0}
                    placeholder="Points"
                    className="h-8 text-sm"
                    value={form.estimate}
                    onChange={(e) => {
                      const next = e.target.value;
                      updateField('estimate', next, {
                        estimate: next === '' ? undefined : Number(next),
                      });
                    }}
                  />
                </PopoverContent>
              </Popover>
            </MetadataRow>

            {/* Due date */}
            <MetadataRow icon={CalendarDays} label="Due date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                placeholder="None"
                disablePast
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

          </div>

          <AssociationsPanel
            objectType="story"
            objectId={storyDetail.story.id}
            workspaceId={workspaceId!}
            includeStoryRelationships={false}
            className="mt-6"
          />

          {/* Delivery */}
          {hasGitIntegration && fieldVis.delivery && (
            <StoryDeliveryPanel
              workspaceId={workspaceId!}
              storyDetail={storyDetail}
              onStoryUpdated={setStoryDetail}
            />
          )}

          {/* Git Links & Agent Runs */}
          {hasGitIntegration && fieldVis.dev_history && (
            <>
              <StoryGitPanel storyId={storyDetail.story.id} workspaceId={workspaceId!} />
              <AgentRunPanel
                storyId={storyDetail.story.id}
                workspaceId={workspaceId!}
                assignedAgentId={storyDetail.story.assigned_agent_id}
                slug={slug}
              />
            </>
          )}
        </aside>
      </div>

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

function StoryImplementationBriefPanel({ brief }: { brief: StoryImplementationBrief }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="mt-4 rounded-md border border-border/60 p-3">
      <button
        type="button"
        className="flex w-full items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
        onClick={() => setOpen((v) => !v)}
      >
        <ChevronRight className={`h-3 w-3 transition-transform ${open ? 'rotate-90' : ''}`} />
        Implementation Brief ({brief.files_to_modify.length} files)
      </button>
      {open && (
        <div className="mt-2 space-y-1.5 text-xs">
          <p>{brief.approach}</p>
          {brief.files_to_modify.map((f) => (
            <div key={`${f.path}-${f.action}`} className="flex items-baseline gap-1.5 font-mono text-[11px]">
              <span className="shrink-0 rounded border border-border/60 bg-muted/30 px-1 text-[9px] uppercase">{f.action}</span>
              <span className="break-all">{f.path}</span>
              <span className="text-muted-foreground">{'\u2014'} {f.description}</span>
            </div>
          ))}
          <p className="text-muted-foreground">Tests: {brief.test_strategy}</p>
        </div>
      )}
    </div>
  );
}
