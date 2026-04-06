import { useCallback, useEffect, useMemo, useState } from 'react';
import { Copy01Icon, File01Icon, Loading01Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService';
import type { TaskTemplate } from '@/lib/pmTypes';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';

interface TaskTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
}

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

export function TaskTemplatesSettings({ workspaceId, initialTeamId }: TaskTemplatesSettingsProps) {
  const { teams } = useAccessibleTeams(workspaceId);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingTemplate, setEditingTemplate] = useState<TaskTemplate | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');

  const teamMap = useMemo(() => new Map(teams.map((t) => [t.id, t.name])), [teams]);

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
          onClick={() => {
            setEditingTemplate(null);
            setShowCreate(true);
          }}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
          Add template
        </Button>
      </div>

      {templates.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-12 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <File01Icon className="h-6 w-6 text-muted-foreground/60" />
          </div>
          <div>
            <p className="text-sm font-medium text-foreground">No task templates yet</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Templates let you pre-fill task fields so your team can create consistent tasks faster.
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setEditingTemplate(null);
              setShowCreate(true);
            }}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Create your first template
          </Button>
        </div>
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
