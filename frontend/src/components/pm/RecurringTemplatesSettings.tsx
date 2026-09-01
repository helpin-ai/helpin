import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { ArrowLeft01Icon, ArrowRight01Icon, Loading01Icon, PlayCircleIcon, ArrowReloadHorizontalIcon } from '@/lib/icons';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { RecurringTemplateList } from '@/components/pm/RecurringTemplateList';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import type { RecurringRun, RecurringTemplateDetail } from '@/lib/pmTypes';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { toast } from 'sonner';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';

const RUN_PAGE_SIZE = 10;

function formatRunRelative(iso?: string) {
  if (!iso) return '';
  try { return formatDistanceToNow(parseISO(iso), { addSuffix: true }); } catch { return iso; }
}

function formatRunTrigger(run: RecurringRun) {
  switch (run.trigger_type) {
    case 'schedule': return 'Scheduled';
    case 'completion': return 'On completion';
    case 'generate_now': return 'Manual';
    default: return 'Seed';
  }
}

function RunHistory({ runs, onOpenTask }: { runs: RecurringRun[]; onOpenTask?: (taskId: string) => void }) {
  const [page, setPage] = useState(0);
  const totalPages = Math.ceil(runs.length / RUN_PAGE_SIZE);
  const visible = runs.slice(page * RUN_PAGE_SIZE, (page + 1) * RUN_PAGE_SIZE);
  const start = page * RUN_PAGE_SIZE + 1;
  const end = Math.min((page + 1) * RUN_PAGE_SIZE, runs.length);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">
          {totalPages <= 1 ? `${runs.length} run${runs.length === 1 ? '' : 's'}` : `${start}–${end} of ${runs.length} runs`}
        </span>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon" className="h-6 w-6" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
              <ArrowLeft01Icon className="h-3.5 w-3.5" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="h-6 w-6" disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
              <ArrowRight01Icon className="h-3.5 w-3.5" />
            </Button>
          </div>
        )}
      </div>
      <div className="space-y-1.5">
        {visible.map((run) => (
          <div key={run.id} className="flex items-center gap-2 rounded-md bg-muted/30 px-2.5 py-2 text-xs">
            <PlayCircleIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{formatRunTrigger(run)}</span>
            <span className="shrink-0 capitalize text-muted-foreground">{run.status}</span>
            <span className="shrink-0 text-muted-foreground">{formatRunRelative(run.finished_at ?? run.started_at ?? run.created_at)}</span>
            {run.generated_task_id && onOpenTask ? (
              <button type="button" className="shrink-0 text-primary hover:underline" onClick={() => onOpenTask(run.generated_task_id!)}>
                View task
              </button>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

interface RecurringTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
  editable?: boolean;
}

export function RecurringTemplatesSettings({ workspaceId, initialTeamId, editable = false }: RecurringTemplatesSettingsProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const ownerNames = useMemo(() => buildAssignableMemberNameMap(members), [members]);
  const teamNames = useMemo(() => new Map(teams.map((team) => [team.id, team.name])), [teams]);

  const [templates, setTemplates] = useState<RecurringTemplateDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('__all__');
  const [teamFilter, setTeamFilter] = useState(initialTeamId ?? '__all__');
  const [viewTemplate, setViewTemplate] = useState<RecurringTemplateDetail | null>(null);
  const [editTemplate, setEditTemplate] = useState<RecurringTemplateDetail | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<RecurringTemplateDetail | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    const { data, error } = await pmRecurringTemplateService.list(workspaceId, {
      status: status === '__all__' ? undefined : status,
      teamId: teamFilter === '__all__' ? undefined : teamFilter,
      search: search.trim() || undefined,
    });
    if (error) {
      toast.error(error);
    } else {
      setTemplates(data ?? []);
    }
    setLoading(false);
  }, [workspaceId, status, teamFilter, search]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const loadTemplateDetail = useCallback(async (id: string) => {
    const { data, error } = await pmRecurringTemplateService.get(workspaceId, id);
    if (error || !data) {
      toast.error(error ?? 'Failed to load recurring template');
      return null;
    }
    return data;
  }, [workspaceId]);

  const runAction = useCallback(
    async (action: () => Promise<{ data: RecurringTemplateDetail | null; error: string | null }>, successMessage: string) => {
      setSaving(true);
      const { error } = await action();
      setSaving(false);
      if (error) {
        toast.error(error);
        return;
      }
      toast.success(successMessage);
      await reload();
      if (viewTemplate) {
        const refreshed = await loadTemplateDetail(viewTemplate.template.id);
        if (refreshed) setViewTemplate(refreshed);
      }
      if (editTemplate) {
        const refreshed = await loadTemplateDetail(editTemplate.template.id);
        if (refreshed) setEditTemplate(refreshed);
      }
    },
    [editTemplate, loadTemplateDetail, reload, viewTemplate],
  );

  const handleOpenTask = (taskId?: string) => {
    if (!taskId || !workspace?.slug) return;
    openTaskRoute(navigate as never, location as never, workspace.slug, taskId);
  };

  const handleEditSubmit = async (value: RecurringTemplateFormValue) => {
    if (!editTemplate) return;
    setSaving(true);
    const { data, error } = await pmRecurringTemplateService.update(workspaceId, editTemplate.template.id, {
      title: value.title,
      description: value.description || undefined,
      config: value.config,
    });
    setSaving(false);
    if (error || !data) {
      toast.error(error ?? 'Failed to update recurring template');
      return;
    }
    toast.success('Recurring template updated');
    setEditTemplate(null);
    await reload();
    if (viewTemplate?.template.id === data.template.id) {
      setViewTemplate(data);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <QuietSearchInput
          containerClassName="w-[240px]"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search recurring templates"
        />
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="h-8 w-[180px]">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All statuses</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="paused">Paused</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
          </SelectContent>
        </Select>
        <Select value={teamFilter} onValueChange={setTeamFilter}>
          <SelectTrigger className="h-8 w-[180px]">
            <SelectValue placeholder="All teams" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All teams</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button type="button" variant="outline" size="sm" className="ml-auto" onClick={() => void reload()} disabled={loading}>
          <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
          <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
          Loading recurring templates...
        </div>
      ) : (
        <RecurringTemplateList
          templates={templates}
          teamNames={teamNames}
          ownerNames={ownerNames}
          onView={async (item) => {
            const detail = await loadTemplateDetail(item.template.id);
            if (detail) setViewTemplate(detail);
          }}
          onEdit={editable ? async (item) => {
            const detail = await loadTemplateDetail(item.template.id);
            if (detail) setEditTemplate(detail);
          } : undefined}
          onPause={editable ? (item) => void runAction(() => pmRecurringTemplateService.pause(workspaceId, item.template.id), 'Recurring template paused') : undefined}
          onResume={editable ? (item) => void runAction(() => pmRecurringTemplateService.resume(workspaceId, item.template.id), 'Recurring template resumed') : undefined}
          onStop={editable ? (item) => setDeleteConfirm(item) : undefined}
          onSkipNext={editable ? (item) => void runAction(() => pmRecurringTemplateService.skipNext(workspaceId, item.template.id), 'Next occurrence skipped') : undefined}
          onGenerateNow={editable ? (item) => void runAction(() => pmRecurringTemplateService.generateNow(workspaceId, item.template.id), 'Recurring task generated') : undefined}
          onDuplicate={editable ? (item) => void runAction(() => pmRecurringTemplateService.duplicate(workspaceId, item.template.id), 'Recurring template duplicated') : undefined}
        />
      )}

      <Dialog open={!!viewTemplate} onOpenChange={(open) => !open && setViewTemplate(null)}>
        <DialogContent className="max-w-lg">
          {viewTemplate ? (
            <>
              <DialogHeader>
                <DialogTitle>Run History</DialogTitle>
                <DialogDescription>{viewTemplate.template.title}</DialogDescription>
              </DialogHeader>
              {viewTemplate.runs && viewTemplate.runs.length > 0 ? (
                <RunHistory runs={viewTemplate.runs} onOpenTask={(taskId) => handleOpenTask(taskId)} />
              ) : (
                <p className="py-6 text-center text-sm text-muted-foreground">No runs yet. Tasks will appear here after the first scheduled run.</p>
              )}
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={!!editTemplate} onOpenChange={(open) => !open && setEditTemplate(null)}>
        <DialogContent className="max-w-2xl">
          {editTemplate ? (
            <>
              <DialogHeader>
                <DialogTitle>Edit recurring template</DialogTitle>
                <DialogDescription>Update the schedule and future generation behavior for this template.</DialogDescription>
              </DialogHeader>
              <RecurringTemplateForm
                initialValue={{
                  title: editTemplate.template.title,
                  description: editTemplate.template.description ?? '',
                  config: editTemplate.config,
                }}
                submitLabel="Save changes"
                saving={saving}
                onCancel={() => setEditTemplate(null)}
                onSubmit={(value) => void handleEditSubmit(value)}
              />
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirm !== null}
        onOpenChange={(open) => { if (!open) setTimeout(() => setDeleteConfirm(null), 150); }}
        title="Delete recurring template"
        description={`This will permanently stop "${deleteConfirm?.template.title ?? ''}" from creating new tasks. Existing tasks will not be affected.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          if (!deleteConfirm) return;
          void runAction(() => pmRecurringTemplateService.stop(workspaceId, deleteConfirm.template.id), 'Recurring template deleted');
          setDeleteConfirm(null);
        }}
      />
    </div>
  );
}
