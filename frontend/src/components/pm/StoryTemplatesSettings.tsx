import { useCallback, useEffect, useState } from 'react';
import { ChevronDown, ChevronRight, Loader2, Pencil, Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CreateStoryModal } from '@/components/pm/CreateStoryModal';
import { pmStoryTemplateService } from '@/lib/services/pmStoryTemplateService';
import type { StoryTemplate } from '@/lib/pmTypes';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';

interface StoryTemplatesSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
}

function TemplateRow({
  template,
  onEdit,
  onDelete,
}: {
  template: StoryTemplate;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="group rounded-md border-b border-border/30 last:border-b-0">
      <div className="flex items-center gap-3 px-3 py-3.5 hover:bg-accent/50 transition-colors">
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="text-muted-foreground hover:text-foreground shrink-0"
        >
          {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        </button>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-foreground">{template.name}</span>
            <span className="text-[10px] uppercase tracking-wide text-muted-foreground">
              {template.team_id ? 'Team' : 'Shared'}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" onClick={onEdit}>
            <Pencil className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-destructive" onClick={onDelete}>
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </div>
      {expanded && template.description && (
        <div className="px-3 pb-3 pl-10">
          <p className="text-xs text-muted-foreground whitespace-pre-line">{template.description.replace(/<[^>]*>/g, '')}</p>
        </div>
      )}
    </div>
  );
}

export function StoryTemplatesSettings({ workspaceId, initialTeamId }: StoryTemplatesSettingsProps) {
  const { teams } = useWorkspaceTeams(workspaceId);
  const [templates, setTemplates] = useState<StoryTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingTemplate, setEditingTemplate] = useState<StoryTemplate | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');

  const reload = useCallback(async () => {
    const teamId = scopeFilter === '__all__' || scopeFilter === '__shared__' ? undefined : scopeFilter;
    const includeShared = scopeFilter !== '__shared__';
    const { data } = await pmStoryTemplateService.list(workspaceId, { teamId, includeShared, archived: false });
    if (data) setTemplates(data);
    setLoading(false);
  }, [workspaceId, scopeFilter]);

  useEffect(() => {
    reload();
  }, [reload]);

  const handleDelete = async (id: string) => {
    setDeleteConfirmId(null);
    setTemplates((prev) => prev.filter((t) => t.id !== id));
    const { error } = await pmStoryTemplateService.remove(workspaceId, id);
    if (error) reload();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
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
            <SelectItem value="__shared__">For everyone</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} templates
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Button
        variant="outline"
        size="sm"
        onClick={() => {
          setEditingTemplate(null);
          setShowCreate(true);
        }}
      >
        <Plus className="h-3.5 w-3.5" />
        Add template
      </Button>

      <div>
        {templates.length === 0 && (
          <p className="text-sm text-muted-foreground py-6 text-center">
            No story templates yet. Create one to get started.
          </p>
        )}
        {templates.map((template) =>
          deleteConfirmId === template.id ? (
            <div
              key={template.id}
              className="flex items-center gap-3 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2.5"
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
            <TemplateRow
              key={template.id}
              template={template}
              onEdit={() => {
                setEditingTemplate(template);
                setShowCreate(true);
              }}
              onDelete={() => setDeleteConfirmId(template.id)}
            />
          ),
        )}
      </div>

      <CreateStoryModal
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
