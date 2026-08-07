import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useLocation, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  ArchiveIcon,
  ArrowLeft02Icon,
  Calendar03Icon,
  ArrowRight01Icon,
  Loading01Icon,
  PencilEdit01Icon,
  ArrowReloadHorizontalIcon,
  UserGroupIcon,
  ArchiveRestoreIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useConfirm } from '@/components/ui/confirm-dialog';
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
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { useSprintCloseout, useWorkflows, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import type { AttachmentResponse, SprintWithStats, Task, EpicWithStats, UpdateSprintRequest } from '@/lib/pmTypes';
import { SPRINT_STATUS_CONFIG } from '@/lib/pmConstants';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { SprintCloseoutSummary } from '@/components/pm/sprints/SprintCloseoutSummary';
import { SprintRolledInBanner } from '@/components/pm/sprints/SprintRolledInBanner';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/sprints/$sprintId');

const SPRINT_MANAGER_TOOLTIP = 'Only team managers can edit sprints. Ask your team manager for access.';

function ManagerOnlyTooltip({
  disabled,
  className,
  children,
}: {
  disabled: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  if (!disabled) return <>{children}</>;
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={`inline-flex max-w-full ${className ?? ''}`}>{children}</span>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-[260px] text-xs">
          {SPRINT_MANAGER_TOOLTIP}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function formatSprintDateLabel(value: string) {
  if (!value) return 'None';
  return new Date(`${value}T00:00:00`).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
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
      <span className="text-[12px] text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center text-[12px]">{children}</div>
    </>
  );
}

// ── Main Page ──────────────────────────────────────────────────────

interface SprintFormState {
  name: string;
  description: string;
  team_id: string;
  start_date: string;
  end_date: string;
}

const buildForm = (iter: SprintWithStats): SprintFormState => ({
  name: iter.sprint.name,
  description: iter.sprint.description ?? '',
  team_id: iter.sprint.team_id ?? '',
  start_date: iter.sprint.start_date ? iter.sprint.start_date.slice(0, 10) : '',
  end_date: iter.sprint.end_date ? iter.sprint.end_date.slice(0, 10) : '',
});

export function SprintDetailPage() {
  const { sprintId, slug } = routeApi.useParams();
  const confirm = useConfirm();
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;

  const { data: workflows = [] } = useWorkflows(workspaceId ?? '');

  const [sprint, setSprint] = useState<SprintWithStats | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [allEpics, setAllEpics] = useState<EpicWithStats[]>([]);
  const [allSprints, setAllSprints] = useState<SprintWithStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<SprintFormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateSprintRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const savedDescriptionRef = useRef('');

  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit, canManageTeam } = usePermissions(access);
  const closeoutQuery = useSprintCloseout(workspaceId ?? '', sprintId);

  const { teams, findTeamName, getTeamMembers } = useAccessibleTeams(workspaceId ?? '');
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const assignableMemberNames = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form?.team_id ? [form.team_id] : []),
    [teams, form?.team_id],
  );

  useTitle(form?.name ? `${form.name} — Sprint` : 'Sprint');

  // Load sprint data + reference data
  useEffect(() => {
    if (!workspaceId) return;
    (async () => {
      setLoading(true);
      setError(null);
      const [sprintRes, tasksRes, epicsRes, sprintsRes] = await Promise.all([
        pmSprintService.get(workspaceId, sprintId),
        pmSprintService.listTasks(workspaceId, sprintId),
        pmEpicService.list(workspaceId, { archived: false }),
        pmSprintService.list(workspaceId, { archived: false }),
      ]);
      if (sprintRes.error || !sprintRes.data) {
        setError(sprintRes.error ?? 'Sprint not found');
        setLoading(false);
        return;
      }
      setSprint(sprintRes.data);
      savedDescriptionRef.current = sprintRes.data.sprint.description ?? '';
      setForm(buildForm(sprintRes.data));
      setTasks(tasksRes.data ?? []);
      setAllEpics(epicsRes.data ?? []);
      setAllSprints(sprintsRes.data ?? []);
      setLoading(false);
    })();
  }, [workspaceId, sprintId]);

  // Auto-save debounce
  useEffect(() => {
    if (
      saving ||
      Object.keys(pendingPatch).length === 0 ||
      !workspaceId ||
      !sprint ||
      (pendingPatch.description !== undefined && descriptionPendingUploads > 0)
    ) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      const previousDescription = savedDescriptionRef.current;
      setPendingPatch({});
      setSaving(true);
      const { data, error: err } = await pmSprintService.update(workspaceId, sprint.sprint.id, patch);
      if (err || !data) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setSprint(data);
        const nextDescription = data.sprint.description ?? '';
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
  }, [workspaceId, sprint, pendingPatch, saving, descriptionPendingUploads]);

  const queuePatch = (patch: UpdateSprintRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof SprintFormState>(key: K, value: SprintFormState[K], patch: UpdateSprintRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  const handleDescriptionAttachmentDelete = useCallback(
    async (entry: AttachmentResponse) => {
      if (!workspaceId || !sprint || !form) {
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

      const { data, error: err } = await pmSprintService.update(workspaceId, sprint.sprint.id, {
        description: nextDescription,
      });
      if (err || !data) {
        setForm((current) => (current ? { ...current, description: previousDescription } : current));
        setSaveError(err ?? 'Failed to save');
        setSaving(false);
        return 'prevent' as const;
      }

      setSaveError(null);
      setSprint(data);
      savedDescriptionRef.current = data.sprint.description ?? '';
      await pmAttachmentService.remove(workspaceId, entry.attachment.id);
      setSaving(false);
      return 'handled' as const;
    },
    [workspaceId, sprint, form],
  );

  // Derived data
  const progress = useMemo(() => {
    if (!sprint || sprint.stats.task_count === 0) return 0;
    return Math.round((sprint.stats.done_task_count / sprint.stats.task_count) * 100);
  }, [sprint]);

  const currentTeamName = useMemo(
    () => (form?.team_id ? findTeamName(form.team_id) ?? 'Select team' : 'Select team'),
    [form?.team_id, findTeamName],
  );

  const workflow = workflows[0] ?? null;

  // Resources: unique people from task owners + sprint team members
  const resources = useMemo(() => {
    const personMap = new Map<string, { id: string; name: string; email: string }>();

    for (const task of tasks) {
      const ownerKey = task.owner_member_ids?.[0];
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
    (task: Task) => {
      openTaskRoute(navigate as never, location as never, slug, task.id);
    },
    [location, navigate, slug],
  );

  // Refresh tasks when global panel updates/archives a task
  useEffect(() => {
    const refresh = () => {
      if (!workspaceId) return;
      pmSprintService.listTasks(workspaceId, sprintId).then((res) => {
        if (res.data) setTasks(res.data);
      });
    };
    window.addEventListener('task-panel-updated', refresh);
    window.addEventListener('task-panel-archived', refresh);
    return () => {
      window.removeEventListener('task-panel-updated', refresh);
      window.removeEventListener('task-panel-archived', refresh);
    };
  }, [workspaceId, sprintId]);

  const goBack = () => navigate({
    to: '/w/$slug/pm/sprints',
    params: { slug },
    search: sprint?.sprint.team_id ? { team: sprint.sprint.team_id } : {},
  });

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !sprint || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Sprint not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft02Icon className="mr-1 h-3.5 w-3.5" />
          Back to Sprints
        </Button>
      </div>
    );
  }

  const canManageSprint = canEdit && canManageTeam(form.team_id);

  return (
    <div className="flex h-full flex-col max-w-7xl 2xl:max-w-[1600px] min-[2560px]:max-w-[2000px] mx-auto">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="ui-divider-bottom-fade flex items-center gap-2 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft02Icon className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <ArrowReloadHorizontalIcon className="h-3.5 w-3.5 shrink-0 text-blue-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Sprints
          </button>
          <ArrowRight01Icon className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <ManagerOnlyTooltip disabled={!canManageSprint}>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 text-xs text-muted-foreground"
              disabled={!canManageSprint}
              onClick={async () => {
                if (!workspaceId || !sprint) return;
                if (!sprint.sprint.archived) {
                  setArchiveConfirmOpen(true);
                  return;
                }
                setSaving(true);
                const { data, error: err } = await pmSprintService.update(workspaceId, sprint.sprint.id, { archived: false });
                if (err || !data) {
                  setSaveError(err ?? 'Failed to update');
                } else {
                  setSprint(data);
                  setSaveError(null);
                }
                setSaving(false);
              }}
            >
              {sprint.sprint.archived ? <><ArchiveRestoreIcon className="h-3.5 w-3.5" /> Unarchive</> : <><ArchiveIcon className="h-3.5 w-3.5" /> Archive</>}
            </Button>
          </ManagerOnlyTooltip>
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
          {/* Title */}
          <ManagerOnlyTooltip disabled={!canManageSprint} className="w-full">
            <input
              type="text"
              aria-label="Sprint title"
              value={form.name}
              disabled={!canManageSprint}
              onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
              className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none disabled:cursor-not-allowed disabled:opacity-70"
              placeholder="Untitled"
            />
          </ManagerOnlyTooltip>

          {closeoutQuery.data?.rolled_in_from && closeoutQuery.data.rolled_in_from.length > 0 && (
            <div className="mt-4">
              <SprintRolledInBanner
                items={closeoutQuery.data.rolled_in_from}
                onOpenSprint={(sourceSprintId) => navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug, sprintId: sourceSprintId } })}
              />
            </div>
          )}

          {/* Description */}
          <div className="mt-4">
            {editingDescription ? (
              <div>
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => updateField('description', html, { description: html })}
                  placeholder="Add a description..."
                  uploadConfig={{ workspaceId: workspaceId!, entityType: 'editor_upload', entityId: workspaceId! }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  className="border-transparent shadow-none"
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
                  <ManagerOnlyTooltip disabled={!canManageSprint}>
                    <button
                      type="button"
                      disabled={!canManageSprint}
                      className="mt-2 inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
                      onClick={() => setEditingDescription(true)}
                    >
                      <PencilEdit01Icon className="h-3 w-3" />
                      Edit description
                    </button>
                  </ManagerOnlyTooltip>
                )}
              </div>
            )}
          </div>

          <div className="mt-6">
            <Attachments
              workspaceId={workspaceId!}
              entityType="sprint"
              entityId={sprint.sprint.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              editable={canManageSprint}
            />
          </div>

          <Separator className="my-6" />

          {closeoutQuery.data?.closeout && (
            <>
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Sprint closeout</h3>
                <div className="mt-3">
                  <SprintCloseoutSummary closeout={closeoutQuery.data.closeout} />
                </div>
              </div>
              <Separator className="my-6" />
            </>
          )}

          {/* Progress */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Progress</h3>
              <span className="text-xs text-muted-foreground">{progress}%</span>
            </div>
            <Progress value={progress} className="h-2 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
            <p className="text-xs text-muted-foreground">
              {sprint.stats.done_task_count}/{sprint.stats.task_count} tasks done
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
                  teamId={sprint.sprint.team_id ?? null}
                  sprintId={sprintId}
                  externalTasks={tasks}
                  onOpenTask={openTask}
                  onBulkOperationComplete={async () => {
                    if (!workspaceId) return;
                    const tasksRes = await pmSprintService.listTasks(workspaceId, sprintId);
                    if (tasksRes.data) setTasks(tasksRes.data);
                  }}
                />
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Loading workflow...</p>
            )}
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
            {/* Status — computed from dates, display only */}
            <MetadataRow icon={ArrowReloadHorizontalIcon} label="Status">
              <span className={`text-xs ${SPRINT_STATUS_CONFIG[sprint.sprint.status]?.color ?? ''}`}>
                {SPRINT_STATUS_CONFIG[sprint.sprint.status]?.label}
              </span>
            </MetadataRow>

            {/* Team */}
            <MetadataRow icon={UserGroupIcon} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  { value: '__none__', label: 'Select team' },
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('team_id', val, { team_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
                disabled={!canManageSprint}
              />
            </MetadataRow>

            {/* Start Date */}
            <MetadataRow icon={Calendar03Icon} label="Start date">
              {canManageSprint ? (
                <DatePicker
                  value={form.start_date}
                  onChange={(v) => updateField('start_date', v, { start_date: v || undefined })}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'End date',
                    kind: 'end',
                    value: form.end_date,
                    onChange: (v) => updateField('end_date', v, { end_date: v || undefined }),
                    placeholder: 'None',
                  }}
                  placeholder="None"
                  hideIcon
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : (
                <ManagerOnlyTooltip disabled>
                  <span className="px-1.5 py-0.5 text-xs text-muted-foreground">{formatSprintDateLabel(form.start_date)}</span>
                </ManagerOnlyTooltip>
              )}
            </MetadataRow>

            {/* End Date */}
            <MetadataRow icon={Calendar03Icon} label="End date">
              {canManageSprint ? (
                <DatePicker
                  value={form.start_date}
                  onChange={(v) => updateField('start_date', v, { start_date: v || undefined })}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'End date',
                    kind: 'end',
                    value: form.end_date,
                    onChange: (v) => updateField('end_date', v, { end_date: v || undefined }),
                    placeholder: 'None',
                  }}
                  triggerField="linked"
                  defaultActiveField="linked"
                  placeholder="None"
                  hideIcon
                  urgencyColor
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : (
                <ManagerOnlyTooltip disabled>
                  <span className="px-1.5 py-0.5 text-xs text-muted-foreground">{formatSprintDateLabel(form.end_date)}</span>
                </ManagerOnlyTooltip>
              )}
            </MetadataRow>

          </div>

        </aside>
      </div>


      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title="Archive sprint"
        description="This sprint will be hidden from the active list. You can view and restore it from the archived sprints view."
        confirmLabel="Archive"
        variant="default"
        onConfirm={async () => {
          if (!workspaceId || !sprint) return;
          setSaving(true);
          const { data, error: err } = await pmSprintService.update(workspaceId, sprint.sprint.id, { archived: true });
          if (err || !data) {
            setSaveError(err ?? 'Failed to archive');
          } else {
            setSaveError(null);
            navigate({ to: '/w/$slug/pm/sprints', params: { slug } });
          }
          setSaving(false);
        }}
      />
    </div>
  );
}
