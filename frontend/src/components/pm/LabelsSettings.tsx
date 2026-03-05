import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, Loader2, Pencil, Plus, Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { pmLabelService } from '@/lib/services/pmLabelService';
import type { Label } from '@/lib/pmTypes';

const PRESET_COLORS = [
  '#3b82f6', // blue
  '#16a34a', // green
  '#ec4899', // pink
  '#64748b', // slate
  '#ef4444', // red
  '#f97316', // orange
  '#eab308', // yellow
  '#14b8a6', // teal
  '#8b5cf6', // violet
  '#6366f1', // indigo
  '#06b6d4', // cyan
  '#d946ef', // fuchsia
  '#84cc16', // lime
  '#f43f5e', // rose
  '#0ea5e9', // sky
  '#a855f7', // purple
];

interface LabelsSettingsProps {
  workspaceId: string;
}

interface LabelFormState {
  name: string;
  description: string;
  color: string;
}

const emptyForm: LabelFormState = { name: '', description: '', color: PRESET_COLORS[0] };

function ColorPicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (color: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {PRESET_COLORS.map((c) => (
        <button
          key={c}
          type="button"
          className={cn(
            'h-6 w-6 rounded-full border-2 transition-all cursor-pointer',
            value === c
              ? 'border-foreground scale-110'
              : 'border-transparent hover:border-muted-foreground/40',
          )}
          style={{ backgroundColor: c }}
          onClick={() => onChange(c)}
        />
      ))}
    </div>
  );
}

function LabelRow({
  label,
  onEdit,
  onDelete,
}: {
  label: Label;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="group flex items-center gap-3 rounded-md px-3 py-2.5 hover:bg-accent/50 transition-colors">
      <span
        className="h-3 w-3 rounded-full shrink-0"
        style={{ backgroundColor: label.color || '#64748b' }}
      />
      <div className="min-w-0 flex-1">
        <span className="text-sm font-medium text-foreground">{label.name}</span>
        {label.description && (
          <span className="ml-2 text-xs text-muted-foreground">{label.description}</span>
        )}
      </div>
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <Button
          variant="ghost"
          size="icon-xs"
          className="text-muted-foreground hover:text-foreground"
          onClick={onEdit}
        >
          <Pencil className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          className="text-muted-foreground hover:text-destructive"
          onClick={onDelete}
        >
          <Trash2 className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

function LabelForm({
  initial,
  onSave,
  onCancel,
  saving,
}: {
  initial: LabelFormState;
  onSave: (form: LabelFormState) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const [form, setForm] = useState<LabelFormState>(initial);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    onSave({ ...form, name: form.name.trim(), description: form.description.trim() });
  };

  return (
    <form onSubmit={handleSubmit} className="rounded-md border p-3 space-y-3 bg-muted/30">
      <div className="flex items-center gap-2">
        <span
          className="h-3 w-3 rounded-full shrink-0"
          style={{ backgroundColor: form.color || '#64748b' }}
        />
        <Input
          ref={nameRef}
          placeholder="Label name"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          className="h-8 text-sm"
        />
      </div>
      <Input
        placeholder="Description (optional)"
        value={form.description}
        onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
        className="h-8 text-sm"
      />
      <ColorPicker value={form.color} onChange={(c) => setForm((f) => ({ ...f, color: c }))} />
      <div className="flex items-center gap-2 pt-1">
        <Button type="submit" size="sm" disabled={saving || !form.name.trim()}>
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
          Save
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel} disabled={saving}>
          <X className="h-3.5 w-3.5" />
          Cancel
        </Button>
      </div>
    </form>
  );
}

export function LabelsSettings({ workspaceId }: LabelsSettingsProps) {
  const [labels, setLabels] = useState<Label[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const { data } = await pmLabelService.list(workspaceId);
    if (data) setLabels(data.filter((l) => !l.archived));
    setLoading(false);
  }, [workspaceId]);

  useEffect(() => {
    reload();
  }, [reload]);

  const handleCreate = async (form: LabelFormState) => {
    setSaving(true);
    const { data } = await pmLabelService.create({
      workspace_id: workspaceId,
      name: form.name,
      description: form.description || undefined,
      color: form.color,
    });
    setSaving(false);
    if (data) {
      setLabels((prev) => [...prev, data]);
      setShowCreate(false);
    }
  };

  const handleUpdate = async (id: string, form: LabelFormState) => {
    setSaving(true);
    const { data } = await pmLabelService.update(workspaceId, id, {
      name: form.name,
      description: form.description || undefined,
      color: form.color,
    });
    setSaving(false);
    if (data) {
      setLabels((prev) => prev.map((l) => (l.id === id ? data : l)));
      setEditingId(null);
    }
  };

  const handleDelete = async (id: string) => {
    setDeleteConfirmId(null);
    setLabels((prev) => prev.filter((l) => l.id !== id));
    await pmLabelService.remove(workspaceId, id);
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
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-foreground">Labels</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Manage labels for organizing stories, epics, and sprints.
          </p>
        </div>
        {!showCreate && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setShowCreate(true);
              setEditingId(null);
            }}
          >
            <Plus className="h-3.5 w-3.5" />
            Add label
          </Button>
        )}
      </div>

      {showCreate && (
        <LabelForm
          initial={emptyForm}
          onSave={handleCreate}
          onCancel={() => setShowCreate(false)}
          saving={saving}
        />
      )}

      <div className="space-y-0.5">
        {labels.length === 0 && !showCreate && (
          <p className="text-sm text-muted-foreground py-6 text-center">
            No labels yet. Create one to get started.
          </p>
        )}
        {labels.map((label) =>
          editingId === label.id ? (
            <LabelForm
              key={label.id}
              initial={{
                name: label.name,
                description: label.description || '',
                color: label.color || PRESET_COLORS[0],
              }}
              onSave={(form) => handleUpdate(label.id, form)}
              onCancel={() => setEditingId(null)}
              saving={saving}
            />
          ) : deleteConfirmId === label.id ? (
            <div
              key={label.id}
              className="flex items-center gap-3 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2.5"
            >
              <span
                className="h-3 w-3 rounded-full shrink-0"
                style={{ backgroundColor: label.color || '#64748b' }}
              />
              <span className="flex-1 text-sm text-foreground">
                Delete <span className="font-medium">{label.name}</span>?
              </span>
              <div className="flex items-center gap-1">
                <Button
                  variant="destructive"
                  size="xs"
                  onClick={() => handleDelete(label.id)}
                >
                  Delete
                </Button>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => setDeleteConfirmId(null)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <LabelRow
              key={label.id}
              label={label}
              onEdit={() => {
                setEditingId(label.id);
                setShowCreate(false);
              }}
              onDelete={() => setDeleteConfirmId(label.id)}
            />
          ),
        )}
      </div>
    </div>
  );
}
