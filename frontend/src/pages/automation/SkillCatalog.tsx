import { useMemo, useRef, useState } from 'react';
import {
  ArrowDown01Icon,
  ArrowRight01Icon,
  BookOpen01Icon,
  Search01Icon,
  Tick01Icon,
  PlusSignIcon,
  Upload01Icon,
  Delete01Icon,
  PencilEdit02Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,

} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  useAutomationSkillCatalog,
  useCreateWorkspaceSkill,
  useImportWorkspaceSkill,
  useUpdateWorkspaceSkill,
  useDeleteWorkspaceSkill,
} from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import type { AgentPresetKey, SkillCatalogEntry, CreateWorkspaceSkillRequest, UpdateWorkspaceSkillRequest } from '@/lib/pmTypes';
import { PRESET_STYLES } from '@/lib/presetStyles';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const SOURCE_KIND_LABELS: Record<string, { label: string; className: string }> = {
  built_in: {
    label: 'Built-in',
    className: 'bg-sky-500/10 text-sky-700 dark:text-sky-400 border-sky-500/20',
  },
  workspace: {
    label: 'Workspace',
    className: 'bg-teal-500/10 text-teal-700 dark:text-teal-400 border-teal-500/20',
  },
  imported: {
    label: 'Imported',
    className: 'bg-orange-500/10 text-orange-700 dark:text-orange-400 border-orange-500/20',
  },
};

const SOURCE_GROUP_ORDER = ['built_in', 'workspace', 'imported'] as const;

const SOURCE_GROUP_LABELS: Record<string, string> = {
  built_in: 'Built-in',
  workspace: 'Workspace',
  imported: 'Imported',
};

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function SourceBadge({ sourceKind }: { sourceKind: string }) {
  const style = SOURCE_KIND_LABELS[sourceKind];
  if (!style) return null;
  return (
    <span className={cn('inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-medium', style.className)}>
      {style.label}
    </span>
  );
}

function PresetBadge({ preset }: { preset: AgentPresetKey }) {
  const style = PRESET_STYLES[preset];
  if (!style) return null;
  return (
    <span className={cn('inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-medium', style.className)}>
      {style.label}
    </span>
  );
}

function SkillCard({
  skill,
  onEdit,
  onDelete,
}: {
  skill: SkillCatalogEntry;
  onEdit?: () => void;
  onDelete?: () => void;
}) {
  const isEditable = skill.source_kind === 'workspace';
  const isDeletable = skill.source_kind === 'workspace' || skill.source_kind === 'imported';

  return (
    <div className="rounded-lg border border-border/60 bg-card/80 transition-colors hover:border-border">
      <div className="px-4 py-3">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <code className="text-sm font-semibold">{skill.key}</code>
              <SourceBadge sourceKind={skill.source_kind} />
              {(skill.presets ?? []).map((preset) => (
                <PresetBadge key={preset} preset={preset as AgentPresetKey} />
              ))}
            </div>
            {skill.title && skill.title !== skill.key && (
              <p className="text-xs font-medium text-foreground/80">{skill.title}</p>
            )}
            <p className="text-xs text-muted-foreground leading-relaxed">{skill.description}</p>
            {(skill.required_tools ?? []).length > 0 && (
              <div className="flex flex-wrap items-center gap-1 pt-0.5">
                <span className="text-[10px] text-muted-foreground/70">Requires:</span>
                {skill.required_tools!.map((tool) => (
                  <Badge key={tool} variant="outline" className="px-1.5 py-0 text-[10px] font-mono">
                    {tool}
                  </Badge>
                ))}
              </div>
            )}
          </div>

          {(isEditable || isDeletable) && (
            <div className="flex shrink-0 items-center gap-1">
              {isEditable && onEdit && (
                <button
                  type="button"
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  onClick={onEdit}
                  title="Edit skill"
                >
                  <PencilEdit02Icon className="h-3.5 w-3.5" />
                </button>
              )}
              {isDeletable && onDelete && (
                <button
                  type="button"
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                  onClick={onDelete}
                  title="Delete skill"
                >
                  <Delete01Icon className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function SourceSection({
  sourceKind,
  skills,
  defaultOpen,
  onEdit,
  onDelete,
}: {
  sourceKind: string;
  skills: SkillCatalogEntry[];
  defaultOpen: boolean;
  onEdit: (skill: SkillCatalogEntry) => void;
  onDelete: (skill: SkillCatalogEntry) => void;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const label = SOURCE_GROUP_LABELS[sourceKind] ?? sourceKind;

  return (
    <div>
      <button
        type="button"
        className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left transition-colors hover:bg-accent/40"
        onClick={() => setOpen(!open)}
      >
        {open ? <ArrowDown01Icon className="h-4 w-4 text-muted-foreground" /> : <ArrowRight01Icon className="h-4 w-4 text-muted-foreground" />}
        <BookOpen01Icon className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">{label}</span>
        <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-[10px]">
          {skills.length}
        </Badge>
      </button>

      {open && (
        <div className="mt-1 ml-8 space-y-2">
          {skills.map((skill) => (
            <SkillCard
              key={skill.id ?? skill.key}
              skill={skill}
              onEdit={() => onEdit(skill)}
              onDelete={() => onDelete(skill)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Create Skill Dialog
// ---------------------------------------------------------------------------

function CreateSkillDialog({
  workspaceId,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const createMutation = useCreateWorkspaceSkill(workspaceId);
  const [form, setForm] = useState<CreateWorkspaceSkillRequest>({
    key: '',
    title: '',
    description: '',
    instructions: '',
  });

  const handleCreate = async () => {
    const key = form.key.toLowerCase().replace(/[\s-]+/g, '_').replace(/[^a-z0-9_]/g, '');
    if (!key) {
      toast.error('Skill key is required');
      return;
    }
    if (!form.description.trim()) {
      toast.error('Description is required');
      return;
    }
    if (!form.instructions.trim()) {
      toast.error('Instructions are required');
      return;
    }
    try {
      await createMutation.mutateAsync({ ...form, key });
      toast.success(`Skill "${key}" created`);
      onOpenChange(false);
      setForm({ key: '', title: '', description: '', instructions: '' });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to create skill');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Workspace Skill</DialogTitle>
          <DialogDescription>
            Define a reusable behavioral instruction module for your agents.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="skill-key">Key</Label>
            <Input
              id="skill-key"
              placeholder="my_custom_skill"
              value={form.key}
              onChange={(e) => setForm({ ...form, key: e.target.value })}
            />
            <p className="text-[11px] text-muted-foreground">Lowercase letters, numbers, and underscores only.</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="skill-title">Title</Label>
            <Input
              id="skill-title"
              placeholder="My Custom Skill"
              value={form.title ?? ''}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="skill-description">Description</Label>
            <Textarea
              id="skill-description"
              placeholder="What this skill does..."
              rows={2}
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="skill-instructions">Instructions</Label>
            <Textarea
              id="skill-instructions"
              placeholder="Behavioral instructions for agents using this skill..."
              rows={6}
              className="font-mono text-xs"
              value={form.instructions}
              onChange={(e) => setForm({ ...form, instructions: e.target.value })}
            />
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" size="sm">Cancel</Button>
          </DialogClose>
          <Button size="sm" onClick={handleCreate} disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Edit Skill Dialog
// ---------------------------------------------------------------------------

function EditSkillDialog({
  workspaceId,
  skill,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  skill: SkillCatalogEntry;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const updateMutation = useUpdateWorkspaceSkill(workspaceId);
  const [form, setForm] = useState<UpdateWorkspaceSkillRequest>({
    title: skill.title,
    description: skill.description,
    instructions: '',
  });

  const handleUpdate = async () => {
    if (!skill.id) return;
    try {
      await updateMutation.mutateAsync({ skillId: skill.id, data: form });
      toast.success(`Skill "${skill.key}" updated`);
      onOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to update skill');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Edit Skill: {skill.key}</DialogTitle>
          <DialogDescription>
            Update the workspace skill configuration.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="edit-skill-title">Title</Label>
            <Input
              id="edit-skill-title"
              value={form.title ?? ''}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="edit-skill-description">Description</Label>
            <Textarea
              id="edit-skill-description"
              rows={2}
              value={form.description ?? ''}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="edit-skill-instructions">Instructions</Label>
            <Textarea
              id="edit-skill-instructions"
              rows={6}
              className="font-mono text-xs"
              placeholder="Leave empty to keep existing instructions"
              value={form.instructions ?? ''}
              onChange={(e) => setForm({ ...form, instructions: e.target.value })}
            />
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" size="sm">Cancel</Button>
          </DialogClose>
          <Button size="sm" onClick={handleUpdate} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Import Skill Dialog
// ---------------------------------------------------------------------------

function ImportSkillDialog({
  workspaceId,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const importMutation = useImportWorkspaceSkill(workspaceId);
  const [file, setFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleImport = async () => {
    if (!file) {
      toast.error('Please select a skill archive (.zip)');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      toast.error('Archive must be under 10 MB');
      return;
    }
    try {
      await importMutation.mutateAsync({ file });
      toast.success(`Skill imported from "${file.name}"`);
      onOpenChange(false);
      setFile(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to import skill');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Import Skill</DialogTitle>
          <DialogDescription>
            Upload a skill package archive (.zip). Maximum 10 MB.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Skill Archive</Label>
            <div
              className={cn(
                'flex cursor-pointer items-center justify-center rounded-lg border-2 border-dashed p-6 transition-colors',
                file ? 'border-primary/40 bg-primary/5' : 'border-border hover:border-primary/30 hover:bg-accent/30',
              )}
              onClick={() => fileInputRef.current?.click()}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip"
                className="hidden"
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              />
              <div className="text-center">
                {file ? (
                  <div className="space-y-1">
                    <Tick01Icon className="mx-auto h-6 w-6 text-primary" />
                    <p className="text-sm font-medium">{file.name}</p>
                    <p className="text-[11px] text-muted-foreground">
                      {(file.size / 1024).toFixed(1)} KB
                    </p>
                  </div>
                ) : (
                  <div className="space-y-1">
                    <Upload01Icon className="mx-auto h-6 w-6 text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">Click to select .zip archive</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" size="sm">Cancel</Button>
          </DialogClose>
          <Button size="sm" onClick={handleImport} disabled={importMutation.isPending || !file}>
            {importMutation.isPending ? 'Importing...' : 'Import'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Delete Confirm Dialog
// ---------------------------------------------------------------------------

function DeleteSkillDialog({
  workspaceId,
  skill,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  skill: SkillCatalogEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const deleteMutation = useDeleteWorkspaceSkill(workspaceId);

  const handleDelete = async () => {
    if (!skill?.id) return;
    try {
      await deleteMutation.mutateAsync(skill.id);
      toast.success(`Skill "${skill.key}" deleted`);
      onOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete skill');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Delete Skill</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <code className="font-semibold">{skill?.key}</code>? Agents using this skill will lose access to it.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" size="sm">Cancel</Button>
          </DialogClose>
          <Button variant="destructive" size="sm" onClick={handleDelete} disabled={deleteMutation.isPending}>
            {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function SkillCatalogContent({
  workspaceId,
  embedded = false,
}: {
  workspaceId: string;
  embedded?: boolean;
}) {
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [editSkill, setEditSkill] = useState<SkillCatalogEntry | null>(null);
  const [deleteSkill, setDeleteSkill] = useState<SkillCatalogEntry | null>(null);
  const { data: catalog, isLoading: loading } = useAutomationSkillCatalog(workspaceId);

  const filtered = useMemo(() => {
    if (!catalog) return [];
    const q = search.toLowerCase().trim();
    return catalog.skills.filter((skill) => {
      if (q && !skill.key.toLowerCase().includes(q) && !skill.title.toLowerCase().includes(q) && !skill.description.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [catalog, search]);

  const grouped = useMemo(() => {
    const map = new Map<string, SkillCatalogEntry[]>();
    for (const skill of filtered) {
      const list = map.get(skill.source_kind) ?? [];
      list.push(skill);
      map.set(skill.source_kind, list);
    }
    const result: { sourceKind: string; skills: SkillCatalogEntry[] }[] = [];
    for (const kind of SOURCE_GROUP_ORDER) {
      const skills = map.get(kind);
      if (skills?.length) {
        result.push({ sourceKind: kind, skills: skills.sort((a, b) => a.key.localeCompare(b.key)) });
      }
    }
    return result;
  }, [filtered]);

  if (loading) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        Loading skill catalog...
      </div>
    );
  }

  if (!catalog || catalog.skills.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        No skills available.
      </div>
    );
  }

  return (
    <div className={cn('space-y-4', !embedded && 'mx-auto max-w-3xl')}>
      {!embedded && (
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-lg font-semibold">Skill Catalog</h1>
            <p className="text-xs text-muted-foreground">Behavioral instruction modules available to agents</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" className="h-8 gap-1.5 text-xs" onClick={() => setImportOpen(true)}>
              <Upload01Icon className="h-3.5 w-3.5" />
              Import
            </Button>
            <Button size="sm" className="h-8 gap-1.5 text-xs" onClick={() => setCreateOpen(true)}>
              <PlusSignIcon className="h-3.5 w-3.5" />
              Create
            </Button>
          </div>
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search01Icon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search skills..."
          className="h-9 pl-9 text-sm"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Skill list */}
      {grouped.length === 0 ? (
        <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
          No skills match your search.
        </div>
      ) : (
        <div className="space-y-4">
          {grouped.map(({ sourceKind, skills }) => (
            <SourceSection
              key={sourceKind}
              sourceKind={sourceKind}
              skills={skills}
              defaultOpen={grouped.length === 1}
              onEdit={(skill) => setEditSkill(skill)}
              onDelete={(skill) => setDeleteSkill(skill)}
            />
          ))}
        </div>
      )}

      <CreateSkillDialog workspaceId={workspaceId} open={createOpen} onOpenChange={setCreateOpen} />
      <ImportSkillDialog workspaceId={workspaceId} open={importOpen} onOpenChange={setImportOpen} />
      {editSkill && (
        <EditSkillDialog
          workspaceId={workspaceId}
          skill={editSkill}
          open={!!editSkill}
          onOpenChange={(v) => { if (!v) setEditSkill(null); }}
        />
      )}
      <DeleteSkillDialog
        workspaceId={workspaceId}
        skill={deleteSkill}
        open={!!deleteSkill}
        onOpenChange={(v) => { if (!v) setDeleteSkill(null); }}
      />
    </div>
  );
}

export function SkillCatalogPage() {
  useTitle('Skill Catalog');
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id) ?? '';

  return <SkillCatalogContent workspaceId={workspaceId} />;
}
