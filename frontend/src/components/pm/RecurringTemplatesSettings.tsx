import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Loader2, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { RecurringTemplateForm, type RecurringTemplateFormValue } from '@/components/pm/RecurringTemplateForm';
import { RecurringTemplateList } from '@/components/pm/RecurringTemplateList';
import { RecurringTemplateSummary } from '@/components/pm/RecurringTemplateSummary';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import type { RecurringTemplateDetail } from '@/lib/pmTypes';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { toast } from 'sonner';

interface RecurringTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
  editable?: boolean;
}

export function RecurringTemplatesSettings({ workspaceId, initialTeamId, editable = false }: RecurringTemplatesSettingsProps) {
  const navigate = useNavigate();
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

  const handleOpenStory = (storyId?: string) => {
    if (!storyId || !workspace?.slug) return;
    navigate({
      to: '/w/$slug/pm/stories/$storyId',
      params: { slug: workspace.slug, storyId },
    });
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
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search recurring templates"
          className="h-8 w-[240px]"
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
            <SelectItem value="stopped">Stopped</SelectItem>
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
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
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
          onStop={editable ? (item) => void runAction(() => pmRecurringTemplateService.stop(workspaceId, item.template.id), 'Recurring template stopped') : undefined}
          onSkipNext={editable ? (item) => void runAction(() => pmRecurringTemplateService.skipNext(workspaceId, item.template.id), 'Next occurrence skipped') : undefined}
          onGenerateNow={editable ? (item) => void runAction(() => pmRecurringTemplateService.generateNow(workspaceId, item.template.id), 'Recurring story generated') : undefined}
          onDuplicate={editable ? (item) => void runAction(() => pmRecurringTemplateService.duplicate(workspaceId, item.template.id), 'Recurring template duplicated') : undefined}
        />
      )}

      <Dialog open={!!viewTemplate} onOpenChange={(open) => !open && setViewTemplate(null)}>
        <DialogContent className="max-w-2xl">
          {viewTemplate ? (
            <>
              <DialogHeader>
                <DialogTitle>{viewTemplate.template.title}</DialogTitle>
                <DialogDescription>{viewTemplate.seed.name}</DialogDescription>
              </DialogHeader>
              <RecurringTemplateSummary
                title={viewTemplate.template.title}
                status={viewTemplate.template.status}
                ruleSummary={viewTemplate.rule_summary}
                nextRunAt={viewTemplate.template.next_run_at}
                generatedCount={viewTemplate.template.generated_count}
                lastError={viewTemplate.template.last_error}
                lastGeneratedStory={viewTemplate.last_generated_story ?? null}
                runs={viewTemplate.runs}
              />
              {viewTemplate.last_generated_story ? (
                <div className="flex justify-end">
                  <Button type="button" variant="outline" onClick={() => handleOpenStory(viewTemplate.last_generated_story?.id)}>
                    Open last generated story
                  </Button>
                </div>
              ) : null}
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
    </div>
  );
}
