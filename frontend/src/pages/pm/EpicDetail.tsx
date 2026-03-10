import { useCallback, useEffect, useMemo, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  ArrowLeft,
  CalendarDays,
  ChevronRight,
  Hash,
  Heart,
  Hexagon,
  Loader2,
  User,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { StoryListView } from '@/components/pm/StoryListView';
import { StoryDetailPanel } from '@/components/pm/StoryDetailPanel';
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { useWorkflows, useEpicStates } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { EpicWithStats, EpicHealth, GitRepository, Story, SprintWithStats, UpdateEpicRequest, StateType } from '@/lib/pmTypes';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import { EpicOrchestrationPanel } from '@/components/pm/EpicOrchestrationPanel';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { buildAssignableMemberNameMap, buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/epics/$epicId');

const healthOptions: EpicHealth[] = ['on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
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

  const { teams, getTeamMembers, findTeamName } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const ownerOptions = useMemo(
    () => buildAssignableMemberOptions(assignableMembers),
    [assignableMembers],
  );
  const assignableMemberNames = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );

  // Story detail panel
  const [selectedStory, setSelectedStory] = useState<Awaited<ReturnType<typeof pmStoryService.get>>['data'] | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);

  useTitle(form?.name ? `${form.name} — Epic` : 'Epic');

  const fetchData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
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
    setForm(buildForm(epicRes.data));
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
    if (saving || Object.keys(pendingPatch).length === 0 || !workspaceId || !epic) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data, error: err } = await pmEpicService.update(workspaceId, epic.epic.id, patch);
      if (err || !data) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setEpic(data);
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, epic, pendingPatch, saving]);

  const queuePatch = (patch: UpdateEpicRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof EpicFormState>(key: K, value: EpicFormState[K], patch: UpdateEpicRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

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
  const currentOwnerName = useMemo(() => {
    if (!form?.owner_member_id) return 'Nobody';
    return assignableMemberNames.get(form.owner_member_id) ?? 'Unknown';
  }, [form?.owner_member_id, assignableMemberNames]);
  const currentPlanningRepositoryName = useMemo(() => {
    if (!form?.planning_repository_id) return 'Not configured';
    return repositories.find((repo) => repo.id === form.planning_repository_id)?.full_name ?? 'Unknown repository';
  }, [form?.planning_repository_id, repositories]);

  const workflow = workflows[0] ?? null;
  const selectedStoryStates = useMemo(() => {
    const storyWorkflowID = selectedStory?.story.workflow_id;
    if (!storyWorkflowID) return workflow?.states ?? [];
    return workflows.find((candidate) => candidate.workflow.id === storyWorkflowID)?.states ?? workflow?.states ?? [];
  }, [selectedStory?.story.workflow_id, workflow?.states, workflows]);

  // Resources: unique people from story owners + epic team members
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string }>();

    for (const story of stories) {
      const ownerKey = story.owner_member_id;
      if (ownerKey) {
        const assignable = assignableMembers.find((member) => member.id === ownerKey);
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
    async (story: Story) => {
      if (!workspaceId) return;
      setSelectedStory(null);
      setDetailLoading(true);
      setDetailOpen(true);
      const detail = await pmStoryService.get(workspaceId, story.id);
      setDetailLoading(false);
      if (detail.data) setSelectedStory(detail.data);
    },
    [workspaceId],
  );

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
          <Hexagon className="h-3.5 w-3.5 shrink-0 text-violet-500" />
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
            <TiptapEditor
              content={form.description}
              onChange={(html) => updateField('description', html, { description: html })}
              placeholder="Add a description..."
              className="border-transparent shadow-none"
              teams={teams}
              members={assignableMembers}
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

          {/* Orchestration */}
          <EpicOrchestrationPanel
            epic={epic.epic}
            workspaceId={workspaceId!}
            workspaceSlug={slug}
            onStoriesCreated={() => fetchData()}
          />
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
                {epic?.suggested_health && epic.suggested_health !== form.health && (
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
              <SidebarPopoverSelect
                value={form.owner_member_id || '__none__'}
                options={[
                  { value: '__none__', label: 'Nobody' },
                  ...ownerOptions.map((owner) => ({ value: owner.id, label: owner.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('owner_member_id', val, { owner_member_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentOwnerName}</span>}
              />
            </MetadataRow>

            {/* Planning Repo */}
            <MetadataRow icon={Hexagon} label="Plan repo">
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

      {/* Story Detail Panel */}
      {workspaceId && workflow && (
        <StoryDetailPanel
          workspaceId={workspaceId}
          storyDetail={selectedStory}
          open={detailOpen}
          loading={detailLoading}
          onOpenChange={setDetailOpen}
          states={selectedStoryStates}
          onStoryUpdated={async (updated) => {
            setSelectedStory(updated);
            const res = await pmEpicService.listStories(workspaceId, epicId);
            if (res.data) setStories(res.data);
          }}
          onStoryArchived={() => {
            setDetailOpen(false);
            pmEpicService.listStories(workspaceId!, epicId).then((res) => {
              if (res.data) setStories(res.data);
            });
          }}
        />
      )}
    </div>
  );
}
