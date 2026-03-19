import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  ArrowLeft,
  Bot,
  CalendarDays,
  ChevronRight,
  Hash,
  Heart,
  Layers,
  Loader2,
  Pencil,
  Play,
  User,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { Attachments } from '@/components/pm/Attachments';
import { DatePicker } from '@/components/ui/date-picker';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '@/components/pm/editorImageAttachments';
import { StoryListView } from '@/components/pm/StoryListView';
import { useStoryPanelStore } from '@/stores/storyPanelStore';
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { agentService } from '@/lib/services/agentService';
import { useWorkflows, useEpicStates, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useFlowRun, useStartFlowRun } from '@/hooks/queries/useFlow';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { AttachmentResponse, EpicWithStats, EpicHealth, GitRepository, Story, SprintWithStats, UpdateEpicRequest, StateType, Agent, StartFlowRunRequest, StartEpicPlanningFlowInput } from '@/lib/pmTypes';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { FlowRunDetailSheet, RUN_STATUS_CONFIG, NODE_LABELS } from '@/pages/pm/Flows';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/epics/$epicId');

const healthOptions: EpicHealth[] = ['no_health', 'on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

// ── Sidebar Popover Select ─────────────────────────────────────────

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
}: {
  value: T;
  options: { value: T; label: string; className?: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
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
              onClick={() => { onChange(option.value); setOpen(false); }}
            >
              <span className={`truncate ${option.className ?? ''}`}>{option.label}</span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
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
    return 'No suggestion yet: set a start date and deadline.';
  }
  if (end < start) {
    return 'No suggestion yet: fix the schedule dates.';
  }
  if (epic.stats.story_count === 0) {
    return 'No suggestion yet: add stories with workflow states to this epic.';
  }
  if (today && today < start) {
    return 'No suggestion yet: this epic has not started yet.';
  }

  return 'No suggestion yet: more planning data is needed.';
}

export function EpicDetailPage() {
  const { epicId, slug } = routeApi.useParams();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  const workspaceId = workspace?.id;

  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const { data: workflows = [] } = useWorkflows(workspaceId ?? '');

  const [epic, setEpic] = useState<EpicWithStats | null>(null);
  const [stories, setStories] = useState<Story[]>([]);
  const [allEpics, setAllEpics] = useState<EpicWithStats[]>([]);
  const [allSprints, setAllSprints] = useState<SprintWithStats[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<EpicFormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateEpicRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [editingDescription, setEditingDescription] = useState(false);
  const savedDescriptionRef = useRef('');

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);

  // Flow state
  const [flowRunId, setFlowRunId] = useState<string | null>(null);
  const [showFlowSheet, setShowFlowSheet] = useState(false);
  const [showStartFlowDialog, setShowStartFlowDialog] = useState(false);
  const { data: flowRunView } = useFlowRun(workspaceId ?? '', flowRunId ?? undefined);

  // Sync flowRunId with epic data
  useEffect(() => {
    if (epic?.epic.active_flow_run_id) {
      setFlowRunId(epic.epic.active_flow_run_id);
    }
  }, [epic?.epic.active_flow_run_id]);

  // Refresh stories when flow completes
  useEffect(() => {
    if (flowRunView && flowRunView.run.status === 'completed' && workspaceId) {
      fetchData(false);
    }
  }, [flowRunView?.run.status]);

  const { teams, getTeamMembers, findTeamName } = useAccessibleTeams(workspaceId ?? '');
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const assignableMemberNames = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form?.team_id ? [form.team_id] : []),
    [teams, form?.team_id],
  );

  const openStoryPanel = useStoryPanelStore((s) => s.openStory);

  useTitle(form?.name ? `${form.name} — Epic` : 'Epic');

  const fetchData = useCallback(async (showLoading = true) => {
    if (!workspaceId) return;
    if (showLoading) setLoading(true);
    setError(null);
    const [epicRes, storiesRes, epicsRes, sprintsRes, reposRes] = await Promise.all([
      pmEpicService.get(workspaceId, epicId),
      pmEpicService.listStories(workspaceId, epicId),
      pmEpicService.list(workspaceId, { archived: false }),
      pmSprintService.list(workspaceId, { archived: false }),
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
    setStories(storiesRes.data ?? []);
    setAllEpics(epicsRes.data ?? []);
    setAllSprints(sprintsRes.data ?? []);
    setRepositories(reposRes.data ?? []);
    setLoading(false);
  }, [workspaceId, epicId]);

  // Load epic data + reference data
  useEffect(() => {
    fetchData();
  }, [fetchData]);

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
  const progress = useMemo(() => {
    if (!epic || epic.stats.story_count === 0) return 0;
    return Math.round((epic.stats.done_story_count / epic.stats.story_count) * 100);
  }, [epic]);

  const defaultEpicState = epicStates.find((s) => s.is_default) ?? epicStates[0];
  const currentEpicState = useMemo(
    () => epicStates.find((s) => s.id === form?.epic_state_id) ?? defaultEpicState,
    [epicStates, form?.epic_state_id, defaultEpicState],
  );
  const currentStateName = currentEpicState?.name ?? '';
  const currentStateColor = currentEpicState ? STATE_TYPE_ICON_CONFIG[currentEpicState.state_type as StateType]?.color : '';

  const currentTeamName = useMemo(
    () => (form?.team_id ? findTeamName(form.team_id) ?? 'No team' : 'No team'),
    [form?.team_id, findTeamName],
  );
  const noHealthSuggestionMessage = useMemo(() => getNoHealthSuggestionMessage(epic), [epic]);
  const currentOwnerName = useMemo(() => {
    if (!form?.owner_member_id) return 'Nobody';
    return assignableMemberNames.get(form.owner_member_id) ?? 'Unknown';
  }, [form?.owner_member_id, assignableMemberNames]);
  const currentPlanningRepositoryName = useMemo(() => {
    if (!form?.planning_repository_id) return 'Not configured';
    return repositories.find((repo) => repo.id === form.planning_repository_id)?.full_name ?? 'Unknown repository';
  }, [form?.planning_repository_id, repositories]);

  const workflow = workflows[0] ?? null;

  // Resources: unique people from story owners + epic team members
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string }>();

    for (const story of stories) {
      const ownerKey = story.owner_member_id;
      if (ownerKey) {
        const assignable = findAssignableMember(assignableMembers, ownerKey);
        if (assignable) {
          personMap.set(assignable.id, {
            id: assignable.id,
            name: assignableMemberNames.get(assignable.id) ?? assignable.display_name,
            email: assignable.email,
          });
        }
      }
    }

    if (form?.team_id) {
      for (const member of getTeamMembers(form.team_id)) {
        if (!personMap.has(member.id)) {
          personMap.set(member.id, { id: member.id, name: member.name, email: member.email });
        }
      }
    }

    return Array.from(personMap.values());
  }, [stories, assignableMembers, assignableMemberNames, form?.team_id, getTeamMembers]);

  const openStory = useCallback(
    (story: Story) => openStoryPanel(story.id),
    [openStoryPanel],
  );

  // Refresh stories when global panel updates/archives a story
  useEffect(() => {
    const refresh = () => {
      if (!workspaceId) return;
      pmEpicService.listStories(workspaceId, epicId).then((res) => {
        if (res.data) setStories(res.data);
      });
    };
    window.addEventListener('story-panel-updated', refresh);
    window.addEventListener('story-panel-archived', refresh);
    return () => {
      window.removeEventListener('story-panel-updated', refresh);
      window.removeEventListener('story-panel-archived', refresh);
    };
  }, [workspaceId, epicId]);

  const goBack = () => navigate({ to: '/w/$slug/pm/epics', params: { slug } });

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !epic || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Epic not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Epics
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
          <Layers className="h-3.5 w-3.5 shrink-0 text-violet-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Epics
          </button>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <FollowButton entityType="epic" entityId={epic.epic.id} />
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
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
                    <Pencil className="h-3 w-3" />
                    Edit description
                  </button>
                )}
              </div>
            )}
          </div>

          <div className="mt-6">
            <Attachments
              workspaceId={workspaceId!}
              entityType="epic"
              entityId={epic.epic.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
            />
          </div>

          <Separator className="my-6" />

          {/* Progress */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Progress</h3>
              <span className="text-xs text-muted-foreground">{progress}%</span>
            </div>
            <Progress value={progress} />
            <p className="text-xs text-muted-foreground">
              {epic.stats.done_story_count}/{epic.stats.story_count} stories done · {epic.stats.done_points}/{epic.stats.total_points} points
            </p>
          </div>

          <Separator className="my-6" />

          {/* Resources */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Resources</h3>
            {resources.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No people assigned yet.</p>
            ) : (
              <div className="mt-3 flex flex-wrap gap-2">
                {resources.map((person) => (
                  <div key={person.id} className="flex items-center gap-2 rounded-md border border-border/60 px-3 py-1.5">
                    <UserAvatar name={person.name || person.email} className="h-6 w-6 border-border/60" />
                    <span className="text-xs font-medium">{person.name || person.email}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator className="my-6" />

          {/* Stories */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Stories ({stories.length})
            </h3>
            {stories.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No stories linked yet.</p>
            ) : workflow ? (
              <div className="mt-3 -mx-3">
                <StoryListView
                  workspaceId={workspaceId!}
                  workflow={workflow}
                  workflows={workflows}
                  teams={teams}
                  assignableMembers={assignableMembers}
                  epics={allEpics}
                  sprints={allSprints}
                  externalStories={stories}
                  onOpenStory={openStory}
                />
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Loading workflow...</p>
            )}
          </div>

          <Separator className="my-6" />

          {/* AI Planning */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">AI Planning</h3>
            <div className="mt-3">
              {flowRunId && flowRunView ? (
                <button
                  type="button"
                  className="w-full rounded-lg border border-border/60 p-3 text-left hover:bg-muted/40 transition-colors"
                  onClick={() => setShowFlowSheet(true)}
                >
                  <div className="flex items-center gap-2">
                    <Bot className="h-4 w-4 text-muted-foreground shrink-0" />
                    <span className="text-sm font-medium flex-1 truncate">
                      {NODE_LABELS[flowRunView.run.current_node_id ?? ''] ?? flowRunView.run.current_node_id ?? 'Planning flow'}
                    </span>
                    <Badge variant="secondary" className={`text-[10px] shrink-0 ${RUN_STATUS_CONFIG[flowRunView.run.status]?.className ?? ''}`}>
                      {RUN_STATUS_CONFIG[flowRunView.run.status]?.label ?? flowRunView.run.status}
                    </Badge>
                    <ChevronRight className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                  </div>
                </button>
              ) : canEdit ? (
                <Button variant="outline" size="sm" onClick={() => setShowStartFlowDialog(true)}>
                  <Bot className="mr-1.5 h-3.5 w-3.5" /> Start Planning
                </Button>
              ) : (
                <p className="text-sm text-muted-foreground">No active planning flow.</p>
              )}
            </div>
          </div>

        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
            {/* State */}
            <MetadataRow icon={Hash} label="State">
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
            <MetadataRow icon={Heart} label="Health">
              <div className="flex flex-col gap-1">
                <SidebarPopoverSelect
                  value={form.health}
                  options={healthOptions.map((h) => ({ value: h, label: healthConfig[h].label, className: healthConfig[h].color }))}
                  onChange={(v) => updateField('health', v as EpicHealth, { health: v as EpicHealth })}
                  renderTrigger={() => (
                    <span className={healthConfig[form.health]?.color}>{healthConfig[form.health]?.label}</span>
                  )}
                />
                {noHealthSuggestionMessage ? (
                  <p className="text-[10px] text-muted-foreground">
                    {noHealthSuggestionMessage}
                  </p>
                ) : epic?.suggested_health && epic.suggested_health !== form.health && (
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

            {/* Owner */}
            <MetadataRow icon={User} label="Owner">
              <MemberPickerPopover
                value={form.owner_member_id || '__none__'}
                members={assignableMembers}
                noneLabel="Nobody"
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

            {/* Planning Repo */}
            <MetadataRow icon={Layers} label="Plan repo">
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
                renderTrigger={() => <span>{currentPlanningRepositoryName}</span>}
              />
            </MetadataRow>

            {/* Start Date */}
            <MetadataRow icon={CalendarDays} label="Start date">
              <DatePicker
                value={form.planned_start_date}
                onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={CalendarDays} label="Target date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>
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
            <AssociationsPanel objectType="epic" objectId={epicId} workspaceId={workspaceId} />
          ) : null}
        </aside>
      </div>

      {/* Flow detail sheet */}
      {workspaceId && (
        <FlowRunDetailSheet
          open={showFlowSheet}
          onOpenChange={setShowFlowSheet}
          runView={flowRunView ?? null}
          workspaceId={workspaceId}
        />
      )}

      {/* Start epic flow dialog */}
      {workspaceId && (
        <StartEpicFlowDialog
          open={showStartFlowDialog}
          onOpenChange={setShowStartFlowDialog}
          workspaceId={workspaceId}
          epicId={epicId}
          onStarted={(runId) => {
            setFlowRunId(runId);
            setShowFlowSheet(true);
          }}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// StartEpicFlowDialog — Simplified dialog for starting epic planning from epic detail
// ---------------------------------------------------------------------------

function StartEpicFlowDialog({
  open,
  onOpenChange,
  workspaceId,
  epicId,
  onStarted,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  epicId: string;
  onStarted: (runId: string) => void;
}) {
  const [specPlannerId, setSpecPlannerId] = useState('');
  const [storyPlannerId, setStoryPlannerId] = useState('');
  const [context, setContext] = useState('');
  const [agents, setAgents] = useState<Agent[]>([]);

  const plannerAgents = useMemo(
    () => agents.filter((a) => a.agent_class === 'product_planner'),
    [agents],
  );

  const startMutation = useStartFlowRun(workspaceId);

  useEffect(() => {
    if (!open || !workspaceId) return;
    agentService.list(workspaceId).then((res) => {
      if (res.data) setAgents(res.data);
    });
  }, [open, workspaceId]);

  const handleStart = async () => {
    if (!specPlannerId) return;
    const input: StartEpicPlanningFlowInput = {
      spec_planner_agent_id: specPlannerId,
      ...(storyPlannerId ? { story_planner_agent_id: storyPlannerId } : {}),
      ...(context.trim() ? { additional_context: context.trim() } : {}),
    };
    const req: StartFlowRunRequest = {
      template_id: 'pm.epic_planning_v2',
      target_type: 'epic',
      target_id: epicId,
      input,
    };
    const result = await startMutation.mutateAsync(req);
    onOpenChange(false);
    setSpecPlannerId('');
    setStoryPlannerId('');
    setContext('');
    onStarted(result.run.id);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Start Epic Planning</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>Spec Planner Agent</Label>
            <Select value={specPlannerId} onValueChange={setSpecPlannerId}>
              <SelectTrigger><SelectValue placeholder="Select a planner agent..." /></SelectTrigger>
              <SelectContent>
                {plannerAgents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Story Planner Agent <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Select value={storyPlannerId || '_none'} onValueChange={(v) => setStoryPlannerId(v === '_none' ? '' : v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">Same as spec planner</SelectItem>
                {plannerAgents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Additional Context <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Textarea
              value={context}
              onChange={(e) => setContext(e.target.value)}
              placeholder="Any extra instructions for the planning agents..."
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleStart} disabled={!specPlannerId || startMutation.isPending}>
            {startMutation.isPending ? (
              <><Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> Starting...</>
            ) : (
              <><Play className="mr-1.5 h-4 w-4" /> Start</>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
