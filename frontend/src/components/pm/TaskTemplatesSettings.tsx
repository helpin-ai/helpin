import { TeamLabel } from '@/components/workspace/TeamLabel';
import { useCallback, useEffect, useMemo, useState, type KeyboardEvent } from 'react';
import { CheckListIcon, Copy01Icon, File01Icon, Loading01Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon, SparklesIcon, UserGroupIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { CreateTaskTemplateRequest, TaskTemplate } from '@/lib/pmTypes';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { toast } from 'sonner';

interface TaskTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
  canManageSharedTemplates?: boolean;
  managedTeamIds?: string[];
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
  teamColor,
  stateName,
  onEdit,
  onDuplicate,
  onDelete,
  canManage,
}: {
  template: TaskTemplate;
  teamName?: string;
  teamColor?: string | null;
  stateName?: string;
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  canManage: boolean;
}) {
  const desc = template.description?.replace(/<[^>]*>/g, '').trim();
  const defaultStateLabel = stateName ?? (template.workflow_state_id ? 'State unavailable' : 'No default state');
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!canManage) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onEdit();
    }
  };

  return (
    <div
      role={canManage ? 'button' : undefined}
      tabIndex={canManage ? 0 : undefined}
      onClick={canManage ? onEdit : undefined}
      onKeyDown={handleKeyDown}
      className={`group flex flex-col gap-3 rounded-lg border border-border/60 bg-card px-4 py-3 transition-colors hover:border-border hover:bg-muted/20 sm:flex-row sm:items-center sm:justify-between ${canManage ? 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2' : ''}`}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-medium text-foreground">{template.name}</span>
        </div>
        <p className="mt-1 line-clamp-1 text-xs leading-5 text-muted-foreground">
          {desc || 'No description'}
        </p>
      </div>

      <div className="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end">
        <span className="rounded-md border border-border/60 bg-background px-2 py-1 text-xs text-muted-foreground">
          {teamName ? <TeamLabel team={{name:teamName,color:teamColor}} /> : 'Shared'}
        </span>
        <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
          {defaultStateLabel}
        </span>
        {canManage && (
        <div className="flex items-center gap-0.5 sm:ml-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon-xs"
                className="text-muted-foreground hover:text-foreground"
                onClick={(event) => {
                  event.stopPropagation();
                  onEdit();
                }}
              >
                <PencilEdit01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Edit</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon-xs"
                className="text-muted-foreground hover:text-foreground"
                onClick={(event) => {
                  event.stopPropagation();
                  onDuplicate();
                }}
              >
                <Copy01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Duplicate</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon-xs"
                className="text-muted-foreground hover:text-destructive"
                onClick={(event) => {
                  event.stopPropagation();
                  onDelete();
                }}
              >
                <Delete01Icon className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Delete</TooltipContent>
          </Tooltip>
        </div>
        )}
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
  canCreate,
  canUseStarter,
}: {
  scopeLabel: string;
  isFiltered: boolean;
  onCreate: () => void;
  onUseStarter: (starter: StarterTemplate) => void;
  creatingStarterKey: string | null;
  canCreate: boolean;
  canUseStarter: boolean;
}) {
  const title = isFiltered ? `No templates for ${scopeLabel}` : 'No task templates yet';
  const description = isFiltered
    ? 'Create a team-specific template, or switch back to all templates to see shared templates from other teams.'
    : 'Turn repeatable work into a reusable starting point with the fields, owner, labels, checklist, and links already filled in.';

  return (
    <div className="overflow-hidden rounded-lg border border-border/70 bg-card">
      <div className={canUseStarter ? "grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]" : "grid gap-0"}>
        <div className="flex flex-col items-start gap-5 p-6 sm:p-8">
          <div className="flex h-11 w-11 items-center justify-center rounded-md border bg-muted/40">
            <File01Icon className="h-5 w-5 text-muted-foreground" />
          </div>

          <div className="max-w-2xl">
            <h3 className="text-base font-semibold">{title}</h3>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
          </div>

          {canCreate ? (
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" onClick={onCreate}>
                <PlusSignIcon className="h-3.5 w-3.5" />
                Create template
              </Button>
              {isFiltered ? (
                <p className="text-xs text-muted-foreground">This will open the template builder for the current workspace.</p>
              ) : null}
            </div>
          ) : null}
        </div>

        {canUseStarter ? (
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
        ) : null}
      </div>
    </div>
  );
}

export function TaskTemplatesSettings({
  workspaceId,
  initialTeamId,
  canManageSharedTemplates = true,
  managedTeamIds = [],
}: TaskTemplatesSettingsProps) {
  const { teams } = useAccessibleTeams(workspaceId);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingTemplate, setEditingTemplate] = useState<TaskTemplate | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');
  const [creatingStarterKey, setCreatingStarterKey] = useState<string | null>(null);
  const [workflowStateMap, setWorkflowStateMap] = useState<Map<string, string>>(new Map());

  const teamMap = useMemo(() => new Map(teams.map((t) => [t.id, t.name])), [teams]);
  const selectedScopeLabel = useMemo(() => {
    if (scopeFilter === '__all__') return 'all templates';
    if (scopeFilter === '__shared__') return 'shared templates';
    return teamMap.get(scopeFilter) ?? 'this team';
  }, [scopeFilter, teamMap]);
  const starterTeamId = useMemo(() => {
    if (scopeFilter !== '__all__' && scopeFilter !== '__shared__') return scopeFilter;
    return initialTeamId ?? teams[0]?.id;
  }, [initialTeamId, scopeFilter, teams]);
  const managedTeamIdSet = useMemo(() => new Set(managedTeamIds), [managedTeamIds]);
  const canManageTeamTemplate = useCallback(
    (teamId?: string) => {
      if (!teamId) return canManageSharedTemplates;
      return canManageSharedTemplates || managedTeamIdSet.has(teamId);
    },
    [canManageSharedTemplates, managedTeamIdSet],
  );
  const canCreateAnyTemplate = canManageSharedTemplates || managedTeamIds.length > 0;
  const canCreateStarter = canManageTeamTemplate(starterTeamId);
  const openCreateModal = () => {
    if (!canCreateAnyTemplate) return;
    setEditingTemplate(null);
    setShowCreate(true);
  };

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

  useEffect(() => {
    let cancelled = false;

    const loadWorkflowStates = async () => {
      const { data } = await pmWorkflowService.list(workspaceId);
      if (cancelled || !data) return;

      const next = new Map<string, string>();
      data.forEach((workflow) => {
        workflow.states.forEach((state) => {
          next.set(state.id, state.name);
        });
      });
      setWorkflowStateMap(next);
    };

    loadWorkflowStates();

    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  const handleDelete = async (id: string) => {
    const tmpl = templates.find((entry) => entry.id === id);
    if (!tmpl || !canManageTeamTemplate(tmpl.team_id)) return;
    setDeleteConfirmId(null);
    setTemplates((prev) => prev.filter((t) => t.id !== id));
    const { error } = await pmTaskTemplateService.remove(workspaceId, id);
    if (error) reload();
  };

  const handleDuplicate = async (tmpl: TaskTemplate) => {
    if (!canManageTeamTemplate(tmpl.team_id)) return;
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
      owner_member_ids: tmpl.owner_member_ids,
      epic_id: tmpl.epic_id,
      sprint_id: tmpl.sprint_id,
      workflow_state_id: tmpl.workflow_state_id,
      deadline: tmpl.deadline,
      checklist_items: tmpl.checklist_items,
      external_links: tmpl.external_links,
    });
    if (!error) reload();
  };

  const handleUseStarter = async (starter: StarterTemplate) => {
    if (!canCreateStarter) return;
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
                <TeamLabel team={team} /> templates
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {canCreateAnyTemplate ? (
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={openCreateModal}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Create template
          </Button>
        ) : null}
      </div>

      {templates.length === 0 && (
        <TemplateEmptyState
          scopeLabel={selectedScopeLabel}
          isFiltered={scopeFilter !== '__all__'}
          onCreate={openCreateModal}
          onUseStarter={handleUseStarter}
          creatingStarterKey={creatingStarterKey}
          canCreate={canCreateAnyTemplate}
          canUseStarter={canCreateStarter}
        />
      )}

      {templates.length > 0 && (
        <div className="space-y-2">
          {templates.map((template) =>
            deleteConfirmId === template.id ? (
              <div
                key={template.id}
                className="flex flex-col gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 sm:flex-row sm:items-center"
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
                teamName={template.team_id ? teamMap.get(template.team_id) : undefined} teamColor={teams.find(team=>team.id===template.team_id)?.color}
                stateName={template.workflow_state_id ? workflowStateMap.get(template.workflow_state_id) : undefined}
                canManage={canManageTeamTemplate(template.team_id)}
                onEdit={() => {
                  if (!canManageTeamTemplate(template.team_id)) return;
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
