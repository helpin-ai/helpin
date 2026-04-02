import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useLocation, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  Archive,
  ArchiveRestore,
  ArrowLeft,
  CalendarDays,
  ChevronRight,
  Hash,
  Heart,
  Layers,
  Loader2,
  Pencil,
  Target as TargetIcon,
  User,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
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
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { useWorkflows, useEpicStates, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { AttachmentResponse, EpicWithStats, EpicHealth, GitRepository, Objective, Task, SprintWithStats, UpdateEpicRequest, StateType } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { EpicPlannerPanel } from '@/components/pm/EpicPlannerPanel';
import { ObjectivePicker, type ObjectivePickerSelection } from '@/components/pm/ObjectivePicker';
import { normalizeTeamType } from '@/lib/teamPresets';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/epics/$epicId');

const healthOptions: EpicHealth[] = ['no_health', 'on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

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
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [editingDescription, setEditingDescription] = useState(false);
  const savedDescriptionRef = useRef('');

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);

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
  const progress = useMemo(() => {
    if (!epic) return 0;
    const totalTasks = getEpicTaskCount(epic.stats);
    if (totalTasks === 0) return 0;
    return Math.round((getEpicDoneTaskCount(epic.stats) / totalTasks) * 100);
  }, [epic]);

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

  // Resources: unique people from task owners + epic team members
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string }>();

    for (const task of tasks) {
      const ownerKey = task.owner_member_id;
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
  }, [tasks, assignableMembers, assignableMemberNames, form?.team_id, getTeamMembers]);

  const openTask = useCallback(
    ( task: Task) => {
      openTaskRoute(navigate as never, location as never, slug, task.id);
    },
    [location, navigate, slug],
  );

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
      <div className="ui-divider-bottom-fade flex items-center gap-2 px-4 py-2.5">
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
            {epic.epic.archived ? <><ArchiveRestore className="h-3.5 w-3.5" /> Unarchive</> : <><Archive className="h-3.5 w-3.5" /> Archive</>}
          </Button>
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
              {getEpicDoneTaskCount(epic.stats)}/{getEpicTaskCount(epic.stats)} tasks done · {epic.stats.done_points}/{epic.stats.total_points} points
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

          {/* Tasks */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Tasks ({tasks.length})
            </h3>
            {tasks.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No tasks linked yet.</p>
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
                  externalTasks={tasks}
                  onOpenTask={openTask}
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
              {workspaceId ? (
                <EpicPlannerPanel
                  workspaceId={workspaceId}
                  epicId={epicId}
                  lastRunId={epic.epic.last_planning_run_id}
                  canEdit={canEdit}
                  onRunCompleted={handlePlannerRunCompleted}
                />
              ) : null}
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
                  { value: '__none__', label: 'Select team' },
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
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
            <MetadataRow icon={TargetIcon} label="Objective">
              <ObjectivePicker
                objectives={allObjectives}
                selectedObjectiveIds={(epic.objectives ?? []).map((objective) => objective.id)}
                selectedObjectives={selectedObjectives}
                onChange={updateObjectives}
                addLabel="Add objective"
                emptyLabel="None"
                className="min-h-6"
              />
            </MetadataRow>

            {/* Owner */}
            <MetadataRow icon={User} label="Owner">
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
            <MetadataRow icon={CalendarDays} label="Start date">
              <DatePicker
                value={form.planned_start_date}
                onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                placeholder="None"
                hideIcon
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={CalendarDays} label="Target date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                placeholder="None"
                hideIcon
                urgencyColor
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {showPlanningRepository ? (
              <>
                <Separator className="col-span-3 my-1" />

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
    </div>
  );
}
