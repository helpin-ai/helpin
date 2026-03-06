import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  Archive,
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
  MoreHorizontal,
  Paperclip,
  Pencil,
  Send,
  ShieldAlert,
  Tag,
  Target,
  Trash2,
  User,
  Users,
  X,
} from 'lucide-react';
import {
  PRIORITY_CONFIG,
  PriorityIcon,
  SEVERITY_CONFIG,
  SeverityIcon,
  StateTypeIcon,
  STORY_TYPE_CONFIG,
  StoryTypeIcon,
} from '@/lib/pmConstants';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
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
import { StoryGitPanel } from '@/components/pm/StoryGitPanel';
import { StoryDeliveryPanel } from '@/components/pm/StoryDeliveryPanel';
import { AgentRunPanel } from '@/components/pm/AgentRunPanel';
import { getInitials } from '@/lib/utils';
import { pmChecklistService } from '@/lib/services/pmChecklistService';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceMembers } from '@/hooks/useWorkspaceMembers';
import { DatePicker } from '@/components/ui/date-picker';
import { EstimatePicker } from '@/components/pm/EstimatePicker';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTeamFieldVisibilityStore } from '@/stores/teamFieldVisibilityStore';
import type {
  ActivityLogEntry,
  CommentWithAuthor,
  EpicWithStats,
  SprintWithStats,
  Label,
  Priority,
  Severity,
  StoryDetail,
  StoryType,
  UpdateStoryRequest,
  WorkflowState,
} from '@/lib/pmTypes';

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
  owner_id: string;
  requester_id: string;
  blocked: boolean;
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
  owner_id: story.story.owner_id ?? '',
  requester_id: story.story.requester_id ?? '',
  blocked: story.story.blocked,
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
  const { copied: linkCopied, copy: copyText } = useCopyToClipboard();
  const fieldVis = useTeamFieldVisibilityStore((s) => s.getForTeam(form.team_id));

  // Re-sync form when storyDetail changes externally (e.g. real-time WS update)
  const lastSyncedAt = useRef(storyDetail.story.updated_at);
  useEffect(() => {
    if (storyDetail.story.updated_at !== lastSyncedAt.current) {
      lastSyncedAt.current = storyDetail.story.updated_at;
      // Only reset form if no unsaved edits
      if (Object.keys(pendingPatch).length === 0 && !saving) {
        setForm(buildFormState(storyDetail));
      }
    }
  }, [storyDetail, pendingPatch, saving]);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [newComment, setNewComment] = useState('');
  const [commentLoading, setCommentLoading] = useState(false);
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const currentUser = useAuthStore((s) => s.user);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const [showAllActivity, setShowAllActivity] = useState(false);
  const [showChecklist, setShowChecklist] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { members } = useWorkspaceMembers(workspaceId);

  // ── URL sync ───────────────────────────────────────────────────
  useEffect(() => {
    const url = new URL(window.location.href);
    url.searchParams.set('story', `TP-${storyDetail.story.display_id}`);
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
    if (saving || Object.keys(pendingPatch).length === 0) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmStoryService.update(workspaceId, storyDetail.story.id, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save changes');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        onStoryUpdated(data);
      }
      setSaving(false);
    }, 650);

    return () => window.clearTimeout(timer);
  }, [workspaceId, storyDetail, pendingPatch, saving, onStoryUpdated]);

  const queuePatch = (patch: UpdateStoryRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateStoryRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

  // ── Comments ───────────────────────────────────────────────────
  const addComment = async () => {
    if (!newComment.trim()) return;
    setCommentLoading(true);
    const { data, error } = await pmCommentService.create(workspaceId, {
      entity_type: 'story',
      entity_id: storyDetail.story.id,
      body: newComment.trim(),
    });
    setCommentLoading(false);
    if (error || !data) return;
    setComments((current) => [...current, data]);
    setNewComment('');
  };

  const startEditComment = (comment: CommentWithAuthor) => {
    setEditingCommentId(comment.comment.id);
    setEditingCommentBody(comment.comment.body);
  };

  const cancelEditComment = () => {
    setEditingCommentId(null);
    setEditingCommentBody('');
  };

  const saveEditComment = async () => {
    if (!editingCommentId || !editingCommentBody.trim()) return;
    const { error } = await pmCommentService.update(workspaceId, editingCommentId, {
      body: editingCommentBody.trim(),
    });
    if (error) return;
    setComments((current) =>
      current.map((c) =>
        c.comment.id === editingCommentId
          ? { ...c, comment: { ...c.comment, body: editingCommentBody.trim() } }
          : c,
      ),
    );
    setEditingCommentId(null);
    setEditingCommentBody('');
  };

  const deleteComment = async (id: string) => {
    const { error } = await pmCommentService.remove(workspaceId, id);
    if (error) return;
    setComments((current) => current.filter((c) => c.comment.id !== id));
  };

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
    if (!form.owner_id) return 'No owner';
    return members.find((m) => m.user_id === form.owner_id)?.full_name ?? 'No owner';
  }, [form.owner_id, members]);

  const currentRequesterName = useMemo(() => {
    if (!form.requester_id) return 'No requester';
    return members.find((m) => m.user_id === form.requester_id)?.full_name ?? 'No requester';
  }, [form.requester_id, members]);

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
          {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0 font-medium text-foreground/80">TP-{storyDetail.story.display_id}</span>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          {linkCopied ? (
            <span className="flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-green-600">
              <Check className="h-3.5 w-3.5" />
              Copied!
            </span>
          ) : (
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={copyLink} title="Copy link">
              <Link2 className="h-3.5 w-3.5" />
            </Button>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={archiveStory} className="text-destructive focus:text-destructive">
                <Archive className="mr-2 h-4 w-4" />
                Archive
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          {workspace && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0"
              title="Open full page"
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
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => onOpenChange(false)}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* ── Two-column grid ─────────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
        {/* ── Left column (main content) ────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-5">
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
              uploadConfig={{ workspaceId, entityType: 'story', entityId: storyDetail.story.id }}
              teams={teams}
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

          {/* Checklist */}
          {showChecklist && (
            <div className="mt-6">
              <ChecklistItems workspaceId={workspaceId} storyId={storyDetail.story.id} />
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
            />
          </div>

          {/* Delivery */}
          <StoryDeliveryPanel
            workspaceId={workspaceId}
            storyDetail={storyDetail}
            onStoryUpdated={onStoryUpdated}
          />

          {/* Git Links */}
          <StoryGitPanel storyId={storyDetail.story.id} workspaceId={workspaceId} />

          {/* Agent Runs */}
          <AgentRunPanel
            storyId={storyDetail.story.id}
            workspaceId={workspaceId}
            assignedAgentId={storyDetail.story.assigned_agent_id}
          />

          {/* Separator */}
          <Separator className="my-6" />

          {/* Comments + Activity */}
          <div>
            {/* Comments card */}
            <div className="rounded-lg border border-border/60">
              {comments.map((entry, idx) => {
                const isOwn = currentUser?.id === entry.comment.author_id;
                const isEditing = editingCommentId === entry.comment.id;
                return (
                  <div key={entry.comment.id}>
                    {idx > 0 && <Separator />}
                    <div className="group px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-rose-800 text-[9px] font-medium text-white">
                          {userInitials(entry.author)}
                        </div>
                        <span className="text-xs font-semibold">{entry.author.full_name || entry.author.email}</span>
                        <span className="text-[11px] text-muted-foreground">{formatRelativeTime(entry.comment.created_at)}</span>
                        {isOwn && !isEditing && (
                          <div className="ml-auto flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                            <button
                              type="button"
                              className="h-6 w-6 flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
                              onClick={() => startEditComment(entry)}
                            >
                              <Pencil className="h-3 w-3" />
                            </button>
                            <button
                              type="button"
                              className="h-6 w-6 flex items-center justify-center rounded text-muted-foreground hover:text-destructive hover:bg-accent transition-colors cursor-pointer"
                              onClick={() => deleteComment(entry.comment.id)}
                            >
                              <Trash2 className="h-3 w-3" />
                            </button>
                          </div>
                        )}
                      </div>
                      {isEditing ? (
                        <div className="mt-1.5 pl-8">
                          <textarea
                            value={editingCommentBody}
                            rows={2}
                            className="w-full resize-none rounded-md border border-border/60 bg-transparent px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                            onChange={(e) => setEditingCommentBody(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                                e.preventDefault();
                                saveEditComment();
                              }
                              if (e.key === 'Escape') cancelEditComment();
                            }}
                          />
                          <div className="mt-1 flex items-center gap-1.5">
                            <Button variant="default" size="sm" className="h-6 px-2 text-xs" onClick={saveEditComment}>
                              Save
                            </Button>
                            <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={cancelEditComment}>
                              Cancel
                            </Button>
                          </div>
                        </div>
                      ) : (
                        <p className="mt-1.5 pl-8 text-sm">{entry.comment.body}</p>
                      )}
                    </div>
                  </div>
                );
              })}

              {/* Comment input */}
              {comments.length > 0 && <Separator />}
              <div className="px-4 py-3">
                <textarea
                  value={newComment}
                  placeholder="Leave a comment..."
                  rows={2}
                  className="w-full resize-none bg-transparent text-sm placeholder:text-muted-foreground/50 focus:outline-none"
                  onChange={(e) => setNewComment(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                      e.preventDefault();
                      addComment();
                    }
                  }}
                />
                <div className="flex items-center justify-end gap-1">
                  <button
                    type="button"
                    className="inline-flex h-7 w-7 items-center justify-center rounded-full border border-border/60 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer disabled:opacity-40"
                    disabled={commentLoading || !newComment.trim()}
                    onClick={addComment}
                  >
                    {commentLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Send className="h-3.5 w-3.5" />}
                  </button>
                </div>
              </div>
            </div>

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
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
          {/* Story ID */}
          <div className="mb-4">
            <span className="text-sm font-semibold text-foreground">TP-{storyDetail.story.display_id}</span>
          </div>

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
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

            {/* Owner */}
            <MetadataRow icon={User} label="Owner">
              <SidebarPopoverSelect
                value={form.owner_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No owner' },
                  ...members.map((m) => ({ value: m.user_id, label: m.full_name || m.email })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('owner_id', val, { owner_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentOwnerName}</span>}
              />
            </MetadataRow>

            {/* Requester */}
            <MetadataRow icon={User} label="Requester">
              <SidebarPopoverSelect
                value={form.requester_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No requester' },
                  ...members.map((m) => ({ value: m.user_id, label: m.full_name || m.email })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('requester_id', val, { requester_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentRequesterName}</span>}
              />
            </MetadataRow>

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
            <MetadataRow icon={GitBranch} label="Sprint">
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

            {/* Blocked */}
            {fieldVis.blocked && (
            <MetadataRow icon={ShieldAlert} label="Blocked">
              <div className="flex items-center gap-2">
                <Switch
                  size="sm"
                  checked={form.blocked}
                  onCheckedChange={(checked) =>
                    updateField('blocked', checked, { blocked: checked })
                  }
                />
                <span className="text-xs text-muted-foreground">
                  {form.blocked ? 'Yes' : 'No'}
                </span>
              </div>
            </MetadataRow>
            )}
          </div>
        </aside>
      </div>

      {/* ── Footer ──────────────────────────────────────────────── */}
      <div className="flex items-center justify-between border-t border-border/60 px-4 py-2 text-xs text-muted-foreground">
        <div>
          {saving ? (
            <span className="inline-flex items-center gap-1">
              <Loader2 className="h-3 w-3 animate-spin" />
              Saving...
            </span>
          ) : (
            <span>All changes saved</span>
          )}
          {saveError && <span className="ml-3 text-destructive">{saveError}</span>}
        </div>
        <Button variant="ghost" size="sm" className="h-7 text-xs text-muted-foreground hover:text-destructive" onClick={archiveStory}>
          <Archive className="mr-1 h-3 w-3" />
          Archive
        </Button>
      </div>
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
      <SheetContent side="right" className="w-[85vw] !max-w-[85vw] p-0" showCloseButton={false}>
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
