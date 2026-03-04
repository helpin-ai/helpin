import { useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  Archive,
  CalendarDays,
  Check,
  ChevronRight,
  Copy,
  Gauge,
  GitBranch,
  Hash,
  Layers,
  LayoutGrid,
  Link2,
  Loader2,
  MoreHorizontal,
  Send,
  ShieldAlert,
  Star,
  Tag,
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
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
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
import { pmCommentService } from '@/lib/services/pmCommentService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { DatePicker } from '@/components/ui/date-picker';
import type {
  ActivityLogEntry,
  CommentWithAuthor,
  EpicWithStats,
  IterationWithStats,
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
  iteration_id: string;
  team_id: string;
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
  iteration_id: story.story.iteration_id ?? '',
  team_id: story.story.team_id ?? '',
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

function getInitials(user?: { full_name?: string; email?: string } | null): string {
  const name = user?.full_name || user?.email || '?';
  return name.slice(0, 2).toUpperCase();
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
          {getInitials(author)}
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
        {getInitials(actor)}
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
  const [form, setForm] = useState<FormState>(() => buildFormState(storyDetail));
  const [pendingPatch, setPendingPatch] = useState<UpdateStoryRequest>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [linkCopied, setLinkCopied] = useState(false);

  const [comments, setComments] = useState<CommentWithAuthor[]>([]);
  const [newComment, setNewComment] = useState('');
  const [commentLoading, setCommentLoading] = useState(false);

  const [activity, setActivity] = useState<ActivityLogEntry[]>([]);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [iterations, setIterations] = useState<IterationWithStats[]>([]);
  const [_labels, _setLabels] = useState<Label[]>([]);
  const [showAllActivity, setShowAllActivity] = useState(false);
  const { teams } = useWorkspaceTeams(workspaceId);

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
  useEffect(() => {
    (async () => {
      const [commentsRes, activityRes] = await Promise.all([
        pmCommentService.list(workspaceId, 'story', storyDetail.story.id),
        pmStoryService.listActivity(workspaceId, storyDetail.story.id, 1, 30),
      ]);
      setComments(commentsRes.data ?? []);
      setActivity(activityRes.data?.data ?? []);
    })();
  }, [workspaceId, storyDetail]);

  // ── Load epics, iterations, labels ─────────────────────────────
  useEffect(() => {
    (async () => {
      const [epicsRes, iterationsRes, labelsRes] = await Promise.all([
        pmEpicService.list(workspaceId, { archived: false }),
        pmIterationService.list(workspaceId, { archived: false }),
        pmLabelService.list(workspaceId),
      ]);
      setEpics(epicsRes.data ?? []);
      setIterations(iterationsRes.data ?? []);
      _setLabels(labelsRes.data ?? []);
    })();
  }, [workspaceId]);

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
  const copyLink = () => {
    const url = new URL(window.location.href);
    url.searchParams.set('story', `TP-${storyDetail.story.display_id}`);
    navigator.clipboard.writeText(url.toString());
    setLinkCopied(true);
    setTimeout(() => setLinkCopied(false), 2000);
  };

  // ── Derived data ───────────────────────────────────────────────
  const currentState = useMemo(
    () => states.find((s) => s.id === form.workflow_state_id),
    [states, form.workflow_state_id],
  );

  const currentEpicName = useMemo(() => {
    if (!form.epic_id) return 'No epic';
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? 'No epic';
  }, [form.epic_id, epics]);

  const currentIterationName = useMemo(() => {
    if (!form.iteration_id) return 'No iteration';
    return iterations.find((i) => i.iteration.id === form.iteration_id)?.iteration.name ?? 'No iteration';
  }, [form.iteration_id, iterations]);

  const currentTeamName = useMemo(() => {
    if (!form.team_id) return 'No team';
    return teams.find((t) => t.id === form.team_id)?.name ?? 'No team';
  }, [form.team_id, teams]);

  const storyLabels = storyDetail.labels ?? [];

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => onOpenChange(false)}>
          <X className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          {currentState && <StateTypeIcon stateType={currentState.state_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="shrink-0 font-medium text-foreground/80">TP-{storyDetail.story.display_id}</span>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="icon" className="h-7 w-7">
            <Star className="h-3.5 w-3.5" />
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={copyLink}>
                <Link2 className="mr-2 h-4 w-4" />
                Copy link
              </DropdownMenuItem>
              <DropdownMenuItem onClick={archiveStory} className="text-destructive focus:text-destructive">
                <Archive className="mr-2 h-4 w-4" />
                Archive
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
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
            />
          </div>

          {/* Separator */}
          <Separator className="my-6" />

          {/* Comments + Activity */}
          <div>
            {/* Comments card */}
            <div className="rounded-lg border border-border/60">
              {comments.map((entry, idx) => (
                <div key={entry.comment.id}>
                  {idx > 0 && <Separator />}
                  <div className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-rose-800 text-[9px] font-medium text-white">
                        {getInitials(entry.author)}
                      </div>
                      <span className="text-xs font-semibold">{entry.author.full_name || entry.author.email}</span>
                      <span className="text-[11px] text-muted-foreground">{formatRelativeTime(entry.comment.created_at)}</span>
                    </div>
                    <p className="mt-1.5 pl-8 text-sm">{entry.comment.body}</p>
                  </div>
                </div>
              ))}

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
          {/* Story ID + copy */}
          <div className="mb-4 flex items-center justify-between">
            <span className="text-sm font-semibold text-foreground">TP-{storyDetail.story.display_id}</span>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={copyLink}>
              {linkCopied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
            </Button>
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

            {/* Assignee (display only) */}
            <MetadataRow icon={Users} label="Assignee">
              <span className="text-xs text-muted-foreground">
                {storyDetail.owners.length > 0
                  ? storyDetail.owners.map((o) => o.full_name || o.email).join(', ')
                  : 'Unassigned'}
              </span>
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

            {/* Labels */}
            <MetadataRow icon={Tag} label="Labels">
              {storyLabels.length > 0 ? (
                <div className="flex flex-wrap gap-1">
                  {storyLabels.map((l) => (
                    <Badge key={l.id} variant="secondary" className="px-1.5 py-0 text-[10px]">
                      {l.name}
                    </Badge>
                  ))}
                </div>
              ) : (
                <span className="text-xs text-muted-foreground">None</span>
              )}
            </MetadataRow>

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

            {/* Iteration */}
            <MetadataRow icon={GitBranch} label="Iteration">
              <SidebarPopoverSelect
                value={form.iteration_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No iteration' },
                  ...iterations.map((i) => ({ value: i.iteration.id, label: i.iteration.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('iteration_id', val, { iteration_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentIterationName}</span>}
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
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {/* Blocked */}
            <MetadataRow icon={ShieldAlert} label="Blocked">
              <div className="flex items-center gap-2">
                <Switch
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
