import { useCallback, useEffect, useMemo, useState } from 'react';
import { CheckListIcon, Copy01Icon, File01Icon, Loading01Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon, SparklesIcon, UserGroupIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService';
import type { CreateTaskTemplateRequest, TaskTemplate } from '@/lib/pmTypes';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { toast } from 'sonner';

interface TaskTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
}

type StarterTemplate = {
  key: string;
  icon: typeof CheckListIcon;
  label: string;
  detail: string;
  payload: Omit<CreateTaskTemplateRequest, 'workspace_id' | 'team_id'>;
};

const STARTER_TEMPLATES: StarterTemplate[] = [
  {
    key: 'bug-report',
    icon: CheckListIcon,
    label: 'Bug report',
    detail: 'Severity, reproduction steps, expected behavior, and owner handoff.',
    payload: {
      name: 'Bug report',
      description: '<p>Use this when something is broken, regressed, or behaving unexpectedly.</p>',
      task_type: 'bug',
      priority: 'high',
      severity: 'major',
      checklist_items: JSON.stringify([
        { text: 'Add reproduction steps', position: 0 },
        { text: 'Document expected behavior', position: 1 },
        { text: 'Document actual behavior', position: 2 },
        { text: 'Attach logs, screenshots, or a failing test', position: 3 },
      ]),
    },
  },
  {
    key: 'customer-request',
    icon: UserGroupIcon,
    label: 'Customer request',
    detail: 'Triage, customer context, acceptance criteria, and follow-up notes.',
    payload: {
      name: 'Customer request',
      description: '<p>Use this to turn customer feedback or support conversations into trackable work.</p>',
      task_type: 'feature',
      priority: 'medium',
      checklist_items: JSON.stringify([
        { text: 'Summarize the customer need', position: 0 },
        { text: 'Link the support conversation or CRM record', position: 1 },
        { text: 'Define acceptance criteria', position: 2 },
        { text: 'Confirm customer follow-up owner', position: 3 },
      ]),
    },
  },
  {
    key: 'launch-task',
    icon: SparklesIcon,
    label: 'Launch task',
    detail: 'Review steps, rollout checklist, owner, and launch-readiness defaults.',
    payload: {
      name: 'Launch task',
      description: '<p>Use this for release, campaign, or operational launch work that needs a final readiness pass.</p>',
      task_type: 'chore',
      priority: 'medium',
      checklist_items: JSON.stringify([
        { text: 'Confirm scope and launch owner', position: 0 },
        { text: 'Complete QA or peer review', position: 1 },
        { text: 'Prepare rollout notes', position: 2 },
        { text: 'Confirm post-launch monitoring plan', position: 3 },
      ]),
    },
  },
];

function TemplateCard({
  template,
  teamName,
  onEdit,
  onDuplicate,
  onDelete,
}: {
  template: TaskTemplate;
  teamName?: string;
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
}) {
  const desc = template.description?.replace(/<[^>]*>/g, '').trim();

  return (
    <div className="group relative flex flex-col gap-2 rounded-lg border border-border/60 bg-card p-3.5 transition-colors hover:border-border hover:bg-muted/30">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-foreground truncate">{template.name}</span>
            <span className="shrink-0 text-[10px] uppercase tracking-wide text-muted-foreground rounded bg-muted px-1.5 py-0.5">
              {teamName ?? 'Shared'}
            </span>
          </div>
          {desc && (
            <p className="mt-1 text-xs text-muted-foreground line-clamp-2">{desc}</p>
          )}
        </div>
        <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" onClick={onEdit}>
                <PencilEdit01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Edit</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" onClick={onDuplicate}>
                <Copy01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Duplicate</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-destructive" onClick={onDelete}>
                <Delete01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Delete</TooltipContent>
          </Tooltip>
        </div>
      </div>

    </div>
  );
}

function TemplateEmptyState({
  scopeLabel,
  isFiltered,
  onCreate,
  onUseStarter,
  creatingStarterKey,
}: {
  scopeLabel: string;
  isFiltered: boolean;
  onCreate: () => void;
  onUseStarter: (starter: StarterTemplate) => void;
  creatingStarterKey: string | null;
}) {
  const title = isFiltered ? `No templates for ${scopeLabel}` : 'No task templates yet';
  const description = isFiltered
    ? 'Create a team-specific template, or switch back to all templates to see shared templates from other teams.'
    : 'Turn repeatable work into a reusable starting point with the fields, owner, labels, checklist, and links already filled in.';

  return (
    <div className="overflow-hidden rounded-lg border border-border/70 bg-card">
      <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex flex-col items-start gap-5 p-6 sm:p-8">
          <div className="flex h-11 w-11 items-center justify-center rounded-md border bg-muted/40">
            <File01Icon className="h-5 w-5 text-muted-foreground" />
          </div>

          <div className="max-w-2xl">
            <h3 className="text-base font-semibold">{title}</h3>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" onClick={onCreate}>
              <PlusSignIcon className="h-3.5 w-3.5" />
              Create template
            </Button>
            {isFiltered ? (
              <p className="text-xs text-muted-foreground">This will open the template builder for the current workspace.</p>
            ) : null}
          </div>
        </div>

        <div className="border-t bg-muted/20 p-4 lg:border-t-0 lg:border-l">
          <p className="px-1 pb-3 text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">
            One-click starters
          </p>
          <div className="space-y-2">
            {STARTER_TEMPLATES.map((starter) => {
              const creating = creatingStarterKey === starter.key;
              return (
              <button
                key={starter.key}
                type="button"
                className="group flex w-full gap-3 rounded-md border bg-background p-3 text-left transition-colors hover:border-border hover:bg-accent/40 disabled:cursor-wait disabled:opacity-70"
                disabled={Boolean(creatingStarterKey)}
                onClick={() => onUseStarter(starter)}
              >
                <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted">
                  {creating ? (
                    <Loading01Icon className="h-4 w-4 animate-spin text-muted-foreground" />
                  ) : (
                    <starter.icon className="h-4 w-4 text-muted-foreground" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{starter.label}</p>
                  <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{starter.detail}</p>
                </div>
                <span className="self-center text-xs font-medium text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                  Create
                </span>
              </button>
            );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

export function TaskTemplatesSettings({ workspaceId, initialTeamId }: TaskTemplatesSettingsProps) {
  const { teams } = useAccessibleTeams(workspaceId);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingTemplate, setEditingTemplate] = useState<TaskTemplate | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');
  const [creatingStarterKey, setCreatingStarterKey] = useState<string | null>(null);

  const teamMap = useMemo(() => new Map(teams.map((t) => [t.id, t.name])), [teams]);
  const selectedScopeLabel = useMemo(() => {
    if (scopeFilter === '__all__') return 'all templates';
    if (scopeFilter === '__shared__') return 'shared templates';
    return teamMap.get(scopeFilter) ?? 'this team';
  }, [scopeFilter, teamMap]);
  const openCreateModal = () => {
    setEditingTemplate(null);
    setShowCreate(true);
  };
  const starterTeamId = useMemo(() => {
    if (scopeFilter !== '__all__' && scopeFilter !== '__shared__') return scopeFilter;
    return initialTeamId ?? teams[0]?.id;
  }, [initialTeamId, scopeFilter, teams]);

  const reload = useCallback(async () => {
    const teamId = scopeFilter === '__all__' || scopeFilter === '__shared__' ? undefined : scopeFilter;
    const includeShared = scopeFilter !== '__shared__';
    const { data } = await pmTaskTemplateService.list(workspaceId, { teamId, includeShared, archived: false });
    if (data) setTemplates(data);
    setLoading(false);
  }, [workspaceId, scopeFilter]);

  useEffect(() => {
    reload();
  }, [reload]);

  const handleDelete = async (id: string) => {
    setDeleteConfirmId(null);
    setTemplates((prev) => prev.filter((t) => t.id !== id));
    const { error } = await pmTaskTemplateService.remove(workspaceId, id);
    if (error) reload();
  };

  const handleDuplicate = async (tmpl: TaskTemplate) => {
    const { error } = await pmTaskTemplateService.create({
      workspace_id: workspaceId,
      team_id: tmpl.team_id,
      name: `${tmpl.name} (copy)`,
      description: tmpl.description,
      task_type: tmpl.task_type,
      priority: tmpl.priority,
      severity: tmpl.severity,
      estimate: tmpl.estimate,
      label_ids: tmpl.label_ids,
      owner_member_id: tmpl.owner_member_id,
      epic_id: tmpl.epic_id,
      sprint_id: tmpl.sprint_id,
      deadline: tmpl.deadline,
      checklist_items: tmpl.checklist_items,
      external_links: tmpl.external_links,
    });
    if (!error) reload();
  };

  const handleUseStarter = async (starter: StarterTemplate) => {
    setCreatingStarterKey(starter.key);
    const { error } = await pmTaskTemplateService.create({
      workspace_id: workspaceId,
      team_id: starterTeamId,
      ...starter.payload,
    });
    setCreatingStarterKey(null);

    if (error) {
      toast.error(error);
      return;
    }

    toast.success(`${starter.label} template created`);
    await reload();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Select value={scopeFilter} onValueChange={setScopeFilter}>
          <SelectTrigger className="h-8 w-[220px] text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All templates</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} templates
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={openCreateModal}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
          Add template
        </Button>
      </div>

      {templates.length === 0 && (
        <TemplateEmptyState
          scopeLabel={selectedScopeLabel}
          isFiltered={scopeFilter !== '__all__'}
          onCreate={openCreateModal}
          onUseStarter={handleUseStarter}
          creatingStarterKey={creatingStarterKey}
        />
      )}

      {templates.length > 0 && (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {templates.map((template) =>
            deleteConfirmId === template.id ? (
              <div
                key={template.id}
                className="flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-3.5 py-3"
              >
                <span className="flex-1 text-sm text-foreground">
                  Delete <span className="font-medium">{template.name}</span>?
                </span>
                <div className="flex items-center gap-1">
                  <Button variant="destructive" size="xs" onClick={() => handleDelete(template.id)}>
                    Delete
                  </Button>
                  <Button variant="ghost" size="xs" onClick={() => setDeleteConfirmId(null)}>
                    Cancel
                  </Button>
                </div>
              </div>
            ) : (
              <TemplateCard
                key={template.id}
                template={template}
                teamName={template.team_id ? teamMap.get(template.team_id) : undefined}
                onEdit={() => {
                  setEditingTemplate(template);
                  setShowCreate(true);
                }}
                onDuplicate={() => handleDuplicate(template)}
                onDelete={() => setDeleteConfirmId(template.id)}
              />
            ),
          )}
        </div>
      )}

      <CreateTaskModal
        open={showCreate}
        onOpenChange={setShowCreate}
        workspaceId={workspaceId}
        initialTeamId={initialTeamId}
        mode="template"
        editingTemplate={editingTemplate}
        onSaveTemplate={() => reload()}
      />
    </div>
  );
}
