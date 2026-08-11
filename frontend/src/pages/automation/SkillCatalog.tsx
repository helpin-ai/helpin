import { useMemo, useRef, useState } from 'react';
import {
  ArrowRight01Icon,
  Search01Icon,
  Tick01Icon,
  PlusSignIcon,
  Upload01Icon,
  Delete01Icon,
  PencilEdit02Icon,
  HelpCircleIcon,
} from '@/lib/icons';

import { Button } from '@/components/ui/button';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
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
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
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

const SOURCE_GROUP_ORDER = ['workspace', 'built_in', 'imported'] as const;

const SOURCE_GROUP_LABELS: Record<string, string> = {
  built_in: 'Built-in',
  workspace: 'Your skills',
  imported: 'Imported',
};

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function presetLabel(preset: string): string {
  return PRESET_STYLES[preset as AgentPresetKey]?.label ?? preset;
}

function displaySkillTitle(skill: Pick<SkillCatalogEntry, 'title' | 'key'>) {
  return skill.title?.trim() || skill.key;
}

function skillKeyFromTitle(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
    .replace(/[^a-z0-9_]/g, '')
    .replace(/_+/g, '_')
    .replace(/^_+|_+$/g, '');
}

function skillMetaLine(skill: SkillCatalogEntry): string | null {
  const presets = skill.presets ?? [];
  if (presets.length > 0) {
    return presets.map(presetLabel).join(', ');
  }
  if (skill.source_kind === 'built_in') return 'All agents';
  if (skill.source_kind === 'workspace') return 'Custom';
  if (skill.source_kind === 'imported') return 'Imported';
  return null;
}

function RequiredFieldLabel({
  htmlFor,
  children,
  tooltip,
}: {
  htmlFor: string;
  children: string;
  tooltip: string;
}) {
  return (
    <Label htmlFor={htmlFor} className="inline-flex items-center gap-1.5">
      <span>{children}</span>
      <span aria-hidden="true" className="text-destructive">*</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="rounded-sm text-muted-foreground/70 hover:text-foreground">
            <HelpCircleIcon className="h-3.5 w-3.5" />
          </span>
        </TooltipTrigger>
        <TooltipContent side="right" className="max-w-72 text-xs leading-relaxed">
          {tooltip}
        </TooltipContent>
      </Tooltip>
    </Label>
  );
}

function SkillCard({
  skill,
  onEdit,
  onDelete,
  onPreview,
}: {
  skill: SkillCatalogEntry;
  onEdit?: () => void;
  onDelete?: () => void;
  onPreview?: () => void;
}) {
  const isEditable = skill.source_kind === 'workspace';
  const isDeletable = skill.source_kind === 'workspace' || skill.source_kind === 'imported';
  const hasInstructions = !!skill.instructions?.trim();
  const meta = skillMetaLine(skill);

  return (
    <div className="group relative flex flex-col rounded-lg border border-border/70 bg-card px-4 py-3.5 transition-all hover:border-border hover:shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium tracking-tight text-foreground">
            {displaySkillTitle(skill)}
          </p>
        </div>
        {(isEditable || isDeletable) && (
          <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            {isEditable && onEdit && (
              <button
                type="button"
                className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                onClick={onEdit}
                title="Edit skill"
              >
                <PencilEdit02Icon className="h-3.5 w-3.5" />
              </button>
            )}
            {isDeletable && onDelete && (
              <button
                type="button"
                className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                onClick={onDelete}
                title="Delete skill"
              >
                <Delete01Icon className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}
      </div>
      <p className="mt-1.5 text-[13px] leading-snug text-muted-foreground line-clamp-2">
        {skill.description}
      </p>
      {(meta || hasInstructions) && (
        <div className="mt-3 flex items-center justify-between gap-3 text-[11px] text-muted-foreground/80">
          <div className="min-w-0">
            {meta && <span className="truncate">{meta}</span>}
          </div>
          {hasInstructions && onPreview && (
            <button
              type="button"
              className="ml-auto inline-flex shrink-0 items-center gap-0.5 rounded-sm text-muted-foreground/80 transition-colors hover:text-foreground"
              onClick={onPreview}
            >
              <ArrowRight01Icon className="h-3 w-3" />
              View details
            </button>
          )}
        </div>
      )}
    </div>
  );
}

function SourceSection({
  sourceKind,
  skills,
  onEdit,
  onDelete,
  onPreview,
}: {
  sourceKind: string;
  skills: SkillCatalogEntry[];
  onEdit: (skill: SkillCatalogEntry) => void;
  onDelete: (skill: SkillCatalogEntry) => void;
  onPreview: (skill: SkillCatalogEntry) => void;
}) {
  const label = SOURCE_GROUP_LABELS[sourceKind] ?? sourceKind;

  return (
    <section>
      <div className="mb-2.5 flex items-baseline gap-1.5 px-0.5 text-[10.5px] font-medium uppercase tracking-[0.08em] text-muted-foreground/80">
        <span>{label}</span>
        <span aria-hidden className="text-muted-foreground/40">·</span>
        <span className="tabular-nums text-muted-foreground/60">{skills.length}</span>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {skills.map((skill) => (
          <SkillCard
            key={skill.id ?? skill.key}
            skill={skill}
            onEdit={() => onEdit(skill)}
            onDelete={() => onDelete(skill)}
            onPreview={() => onPreview(skill)}
          />
        ))}
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Preview Skill Dialog
// ---------------------------------------------------------------------------

function PreviewSkillDialog({
  skill,
  open,
  onOpenChange,
}: {
  skill: SkillCatalogEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  if (!skill) return null;
  const instructions = skill.instructions?.trim() ?? '';
  const meta = skillMetaLine(skill);
  const tools = skill.required_tools ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl gap-0 p-0 sm:max-w-4xl">
        <DialogHeader className="space-y-2 border-b border-border/60 px-6 py-4">
          <DialogTitle className="text-base font-semibold">{displaySkillTitle(skill)}</DialogTitle>
          <div className="flex items-baseline gap-1.5 text-[11px] text-muted-foreground/80">
            <code className="font-mono text-foreground/70">{skill.key}</code>
            {meta && (
              <>
                <span aria-hidden>·</span>
                <span>{meta}</span>
              </>
            )}
          </div>
          <DialogDescription className="text-[13px] leading-snug">
            {skill.description}
          </DialogDescription>
          {tools.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
              <span className="text-[10.5px] font-medium uppercase tracking-[0.08em] text-muted-foreground/70">
                Requires
              </span>
              {tools.map((tool) => (
                <code
                  key={tool}
                  className="rounded border border-border/70 bg-muted/50 px-1.5 py-0.5 font-mono text-[11px] text-foreground/80"
                >
                  {tool}
                </code>
              ))}
            </div>
          )}
        </DialogHeader>
        <div className="max-h-[60vh] overflow-y-auto px-6 py-5">
          {instructions ? (
            <MarkdownContent content={instructions} />
          ) : (
            <p className="text-sm text-muted-foreground">No instructions defined.</p>
          )}
        </div>
        <DialogFooter className="border-t border-border/60 px-6 py-3">
          <DialogClose asChild>
            <Button variant="outline" size="sm">
              Close
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
  const generatedKey = skillKeyFromTitle(form.title ?? '');

  const handleCreate = async () => {
    const title = (form.title ?? '').trim();
    const key = skillKeyFromTitle(title);
    if (!title) {
      toast.error('Title is required');
      return;
    }
    if (!key) {
      toast.error('Title must include letters or numbers');
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
      await createMutation.mutateAsync({ ...form, title, key });
      toast.success(`Skill "${title}" created`);
      onOpenChange(false);
      setForm({ key: '', title: '', description: '', instructions: '' });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to create skill');
    }
  };

  return (
    <TooltipProvider>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-3xl">
          <DialogHeader>
            <div className="space-y-1.5 border-b border-border/60 px-6 py-4">
              <DialogTitle>Create Workspace Skill</DialogTitle>
              <DialogDescription>
                Define a reusable behavioral instruction module for your agents.
              </DialogDescription>
            </div>
          </DialogHeader>
          <div className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-4 overflow-y-auto px-6 py-5">
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="skill-title" tooltip="The human-readable name shown in skill cards, pickers, and selected skill pills.">
                Title
              </RequiredFieldLabel>
              <Input
                id="skill-title"
                placeholder="My Custom Skill"
                value={form.title ?? ''}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
              <p className="text-[11px] text-muted-foreground">
                Key: <code className="font-mono">{generatedKey || 'generated_from_title'}</code>
              </p>
            </div>
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="skill-description" tooltip="A short summary used in the catalog, skill pickers, and available-skill search so agents can decide when this skill is relevant.">
                Description
              </RequiredFieldLabel>
              <Textarea
                id="skill-description"
                placeholder="What this skill does..."
                rows={2}
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="skill-instructions" tooltip="The full behavior guidance an agent reads when it chooses this skill during a run.">
                Instructions
              </RequiredFieldLabel>
              <Textarea
                id="skill-instructions"
                placeholder="Behavioral instructions for agents using this skill..."
                rows={18}
                className="min-h-[22rem] resize-y font-mono text-xs leading-relaxed"
                value={form.instructions}
                onChange={(e) => setForm({ ...form, instructions: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter className="border-t border-border/60 px-6 py-3">
            <DialogClose asChild>
              <Button variant="outline" size="sm">Cancel</Button>
            </DialogClose>
            <Button size="sm" onClick={handleCreate} disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </TooltipProvider>
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
      toast.success(`Skill "${displaySkillTitle(skill)}" updated`);
      onOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to update skill');
    }
  };

  return (
    <TooltipProvider>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-3xl">
          <DialogHeader className="border-b border-border/60 px-6 py-4">
            <DialogTitle>Edit Skill: {displaySkillTitle(skill)}</DialogTitle>
            <DialogDescription>
              Key: <code className="font-mono">{skill.key}</code>
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-4 overflow-y-auto px-6 py-5">
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="edit-skill-title" tooltip="The human-readable name shown in skill cards, pickers, and selected skill pills.">
                Title
              </RequiredFieldLabel>
              <Input
                id="edit-skill-title"
                value={form.title ?? ''}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="edit-skill-description" tooltip="A short summary used in the catalog, skill pickers, and available-skill search so agents can decide when this skill is relevant.">
                Description
              </RequiredFieldLabel>
              <Textarea
                id="edit-skill-description"
                rows={2}
                value={form.description ?? ''}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <RequiredFieldLabel htmlFor="edit-skill-instructions" tooltip="The full behavior guidance an agent reads when it chooses this skill during a run. Leave empty only if you want to keep the current instructions unchanged.">
                Instructions
              </RequiredFieldLabel>
              <Textarea
                id="edit-skill-instructions"
                rows={18}
                className="min-h-[22rem] resize-y font-mono text-xs leading-relaxed"
                placeholder="Leave empty to keep existing instructions"
                value={form.instructions ?? ''}
                onChange={(e) => setForm({ ...form, instructions: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter className="border-t border-border/60 px-6 py-3">
            <DialogClose asChild>
              <Button variant="outline" size="sm">Cancel</Button>
            </DialogClose>
            <Button size="sm" onClick={handleUpdate} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </TooltipProvider>
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
      toast.success(`Skill "${displaySkillTitle(skill)}" deleted`);
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
            Are you sure you want to delete <span className="font-semibold">{skill ? displaySkillTitle(skill) : 'this skill'}</span>? Agents using this skill will lose access to it.
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
  const [previewSkill, setPreviewSkill] = useState<SkillCatalogEntry | null>(null);
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
    <div className="space-y-5">
      {!embedded && (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h1 className="text-xl font-semibold">Skill Catalog</h1>
            <p className="text-[13px] text-muted-foreground">
              Reusable prompt fragments agents compose at runtime.
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 text-xs"
              onClick={() => setImportOpen(true)}
            >
              <Upload01Icon className="h-3.5 w-3.5" />
              Import
            </Button>
            <Button
              size="sm"
              className="h-8 gap-1.5 text-xs"
              onClick={() => setCreateOpen(true)}
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              New skill
            </Button>
          </div>
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/70" />
        <Input
          placeholder="Search skills..."
          className="h-10 border-border/70 bg-muted/30 pl-9 text-[13px] placeholder:text-muted-foreground/70 focus-visible:bg-background"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Skill list */}
      {grouped.length === 0 ? (
        <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border/60 text-sm text-muted-foreground">
          No skills match your search.
        </div>
      ) : (
        <div className="space-y-7">
          {grouped.map(({ sourceKind, skills }) => (
            <SourceSection
              key={sourceKind}
              sourceKind={sourceKind}
              skills={skills}
              onEdit={(skill) => setEditSkill(skill)}
              onDelete={(skill) => setDeleteSkill(skill)}
              onPreview={(skill) => setPreviewSkill(skill)}
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
      <PreviewSkillDialog
        skill={previewSkill}
        open={!!previewSkill}
        onOpenChange={(v) => { if (!v) setPreviewSkill(null); }}
      />
    </div>
  );
}

export function SkillCatalogPage() {
  useTitle('Skill Catalog');
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id) ?? '';

  return <SkillCatalogContent workspaceId={workspaceId} />;
}
