import { useCallback, useEffect, useMemo, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  ArrowLeft,
  CalendarDays,
  ChevronRight,
  Crosshair,
  Hash,
  Heart,
  Hexagon,
  Loader2,
  Plus,
  Target,
  Trash2,
  User,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceMembers } from '@/hooks/useWorkspaceMembers';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import type { MemberWithUser, WorkspacePerson } from '@/lib/types';
import type {
  EpicWithStats,
  KeyResult,
  KeyResultType,
  ObjectiveHealth,
  ObjectiveState,
  ObjectiveWithDetails,
  UpdateObjectiveRequest,
} from '@/lib/pmTypes';

function mergeOwnerOptions(members: MemberWithUser[], people: WorkspacePerson[]) {
  const seen = new Set<string>();
  const result: { id: string; name: string }[] = [];
  for (const m of members) {
    const key = m.email.toLowerCase();
    if (!seen.has(key)) {
      seen.add(key);
      result.push({ id: m.user_id, name: m.full_name || m.email });
    }
  }
  for (const p of people) {
    const key = p.email.toLowerCase();
    if (!seen.has(key)) {
      seen.add(key);
      result.push({ id: p.id, name: p.name || p.email });
    }
  }
  return result;
}

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/objectives/$objectiveId');

const stateOptions: { value: ObjectiveState; label: string }[] = [
  { value: 'not_started', label: 'Not Started' },
  { value: 'active', label: 'Active' },
  { value: 'closed', label: 'Closed' },
];

const healthOptions: { value: ObjectiveHealth; label: string; color: string }[] = [
  { value: 'on_track', label: 'On Track', color: 'text-green-600' },
  { value: 'at_risk', label: 'At Risk', color: 'text-amber-600' },
  { value: 'off_track', label: 'Off Track', color: 'text-red-600' },
];

// ── Sidebar Popover Select ─────────────────────────────────────────

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
}: {
  value: T;
  options: { value: T; label: string; className?: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors cursor-pointer
                ${value === option.value ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
              `}
              onClick={() => { onChange(option.value); setOpen(false); }}
            >
              <span className={`truncate ${option.className ?? ''}`}>{option.label}</span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
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
      <span className="text-xs text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

// ── Multi-value list (teams/owners) ─────────────────────────────────

function MultiValueList({
  items,
  allOptions,
  onAdd,
  onRemove,
  placeholder,
}: {
  items: string[];
  allOptions: { id: string; name: string }[];
  onAdd: (id: string) => void;
  onRemove: (id: string) => void;
  placeholder: string;
}) {
  const [open, setOpen] = useState(false);
  const available = allOptions.filter((o) => !items.includes(o.id));
  const selected = allOptions.filter((o) => items.includes(o.id));

  return (
    <div className="space-y-1">
      {selected.map((item) => (
        <div key={item.id} className="flex items-center justify-between rounded-md bg-muted/50 px-2 py-0.5 text-xs">
          <span className="truncate">{item.name}</span>
          <button type="button" className="text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => onRemove(item.id)}>
            <X className="h-3 w-3" />
          </button>
        </div>
      ))}
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent cursor-pointer"
          >
            <Plus className="h-3 w-3" />
            {placeholder}
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-48 p-1" align="start">
          <div className="flex max-h-48 flex-col overflow-y-auto">
            {available.map((opt) => (
              <button
                key={opt.id}
                type="button"
                className="flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground cursor-pointer"
                onClick={() => { onAdd(opt.id); setOpen(false); }}
              >
                <span className="truncate">{opt.name}</span>
              </button>
            ))}
            {available.length === 0 && (
              <p className="px-2 py-1.5 text-xs text-muted-foreground">No more options</p>
            )}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}

// ── Key Result Row ─────────────────────────────────────────────────

function KeyResultRow({
  kr,
  workspaceId,
  onUpdate,
  onDelete,
}: {
  kr: KeyResult;
  workspaceId: string;
  onUpdate: (updated: KeyResult) => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(kr.name);
  const [currentValue, setCurrentValue] = useState(String(kr.current_value));

  const saveValue = async () => {
    const val = parseFloat(currentValue);
    if (isNaN(val) || val === kr.current_value) return;
    const { data } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, { current_value: val });
    if (data) onUpdate(data);
  };

  const saveName = async () => {
    if (name.trim() === kr.name || !name.trim()) {
      setName(kr.name);
      setEditing(false);
      return;
    }
    const { data } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, { name: name.trim() });
    if (data) onUpdate(data);
    setEditing(false);
  };

  return (
    <div className="group flex items-center gap-3 rounded-md border border-border/60 px-3 py-2">
      <div className="min-w-0 flex-1">
        {editing ? (
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={saveName}
            onKeyDown={(e) => e.key === 'Enter' && saveName()}
            className="w-full bg-transparent text-sm font-medium focus:outline-none"
            autoFocus
          />
        ) : (
          <button
            type="button"
            className="text-sm font-medium text-foreground hover:underline cursor-pointer text-left"
            onClick={() => setEditing(true)}
          >
            {kr.name}
          </button>
        )}
        <div className="mt-1 flex items-center gap-2">
          <span className="text-[10px] text-muted-foreground uppercase">{kr.result_type}</span>
          {kr.result_type !== 'boolean' && (
            <span className="text-[10px] text-muted-foreground">
              {kr.initial_value} → {kr.target_value}
            </span>
          )}
        </div>
      </div>

      <div className="flex items-center gap-2">
        {kr.result_type === 'boolean' ? (
          <button
            type="button"
            className={`rounded-full px-2 py-0.5 text-[10px] font-medium cursor-pointer ${
              kr.progress >= 100
                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                : 'bg-muted text-muted-foreground'
            }`}
            onClick={async () => {
              const newVal = kr.current_value >= kr.target_value ? 0 : kr.target_value;
              const { data } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, { current_value: newVal });
              if (data) onUpdate(data);
            }}
          >
            {kr.progress >= 100 ? 'Done' : 'Not done'}
          </button>
        ) : (
          <input
            type="number"
            value={currentValue}
            onChange={(e) => setCurrentValue(e.target.value)}
            onBlur={saveValue}
            onKeyDown={(e) => e.key === 'Enter' && saveValue()}
            className="w-16 rounded border border-border/60 bg-transparent px-1.5 py-0.5 text-xs text-right focus:outline-none focus:ring-1 focus:ring-primary"
          />
        )}
        <div className="w-20">
          <Progress value={kr.progress} className="h-1.5" />
        </div>
        <span className="w-8 text-right text-xs text-muted-foreground">{Math.round(kr.progress)}%</span>
        <button
          type="button"
          className="text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive cursor-pointer transition-opacity"
          onClick={onDelete}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

// ── Link Epic Popover ──────────────────────────────────────────────

function LinkEpicPopover({
  workspaceId,
  linkedEpicIds,
  onLink,
}: {
  workspaceId: string;
  linkedEpicIds: string[];
  onLink: (epicId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [allEpics, setAllEpics] = useState<EpicWithStats[]>([]);

  useEffect(() => {
    if (!open) return;
    pmEpicService.list(workspaceId, { archived: false }).then(({ data }) => {
      setAllEpics(data ?? []);
    });
  }, [open, workspaceId]);

  const available = allEpics.filter((e) => !linkedEpicIds.includes(e.epic.id));

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="text-xs">
          <Plus className="mr-1 h-3 w-3" />
          Link Epic
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-64 p-1" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {available.map((e) => (
            <button
              key={e.epic.id}
              type="button"
              className="flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground cursor-pointer"
              onClick={() => { onLink(e.epic.id); setOpen(false); }}
            >
              <Hexagon className="h-3 w-3 shrink-0 text-violet-500" />
              <span className="truncate">{e.epic.name}</span>
            </button>
          ))}
          {available.length === 0 && (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">No available epics</p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Main Page ──────────────────────────────────────────────────────

interface FormState {
  name: string;
  description: string;
  objective_type: string;
  state: ObjectiveState;
  health: ObjectiveHealth;
  planned_start_date: string;
  deadline: string;
}

const buildForm = (obj: ObjectiveWithDetails): FormState => ({
  name: obj.objective.name,
  description: obj.objective.description ?? '',
  objective_type: obj.objective.objective_type,
  state: obj.objective.state,
  health: obj.objective.health,
  planned_start_date: obj.objective.planned_start_date?.slice(0, 10) ?? '',
  deadline: obj.objective.deadline?.slice(0, 10) ?? '',
});

export function ObjectiveDetailPage() {
  const { objectiveId, slug } = routeApi.useParams();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;

  const [data, setData] = useState<ObjectiveWithDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateObjectiveRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const { teams, people } = useWorkspaceTeams(workspaceId);
  const { members } = useWorkspaceMembers(workspaceId);
  const ownerOptions = mergeOwnerOptions(members, people);

  useTitle(form?.name ? `${form.name} — Objective` : 'Objective');

  // ── New key result form state
  const [newKrName, setNewKrName] = useState('');
  const [newKrType, setNewKrType] = useState<KeyResultType>('boolean');

  // Load data
  const loadData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const { data: obj, error: err } = await pmObjectiveService.get(workspaceId, objectiveId);
    if (err || !obj) {
      setError(err ?? 'Objective not found');
      setLoading(false);
      return;
    }
    setData(obj);
    setForm(buildForm(obj));
    setLoading(false);
  }, [workspaceId, objectiveId]);

  useEffect(() => { loadData(); }, [loadData]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !workspaceId || !data) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data: updated, error: err } = await pmObjectiveService.update(workspaceId, data.objective.id, patch);
      if (err || !updated) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        setData(updated);
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, data, pendingPatch, saving]);

  const queuePatch = (patch: UpdateObjectiveRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateObjectiveRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  // Key result handlers
  const handleCreateKeyResult = async () => {
    if (!workspaceId || !data || !newKrName.trim()) return;
    const { data: kr } = await pmObjectiveService.createKeyResult(workspaceId, data.objective.id, {
      name: newKrName.trim(),
      result_type: newKrType,
      target_value: newKrType === 'boolean' ? 1 : 100,
    });
    if (kr) {
      setData((prev) => prev ? { ...prev, key_results: [...prev.key_results, kr] } : prev);
      setNewKrName('');
    }
  };

  const handleUpdateKeyResult = (updated: KeyResult) => {
    setData((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        key_results: prev.key_results.map((kr) => kr.id === updated.id ? updated : kr),
      };
    });
  };

  const handleDeleteKeyResult = async (id: string) => {
    if (!workspaceId) return;
    await pmObjectiveService.deleteKeyResult(workspaceId, id);
    setData((prev) => {
      if (!prev) return prev;
      return { ...prev, key_results: prev.key_results.filter((kr) => kr.id !== id) };
    });
  };

  // Team/Owner handlers
  const handleAddTeam = async (teamId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.addTeam(workspaceId, data.objective.id, teamId);
    setData((prev) => prev ? { ...prev, teams: [...prev.teams, teamId] } : prev);
  };

  const handleRemoveTeam = async (teamId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.removeTeam(workspaceId, data.objective.id, teamId);
    setData((prev) => prev ? { ...prev, teams: prev.teams.filter((t) => t !== teamId) } : prev);
  };

  const handleAddOwner = async (userId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.addOwner(workspaceId, data.objective.id, userId);
    setData((prev) => prev ? { ...prev, owners: [...prev.owners, userId] } : prev);
  };

  const handleRemoveOwner = async (userId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.removeOwner(workspaceId, data.objective.id, userId);
    setData((prev) => prev ? { ...prev, owners: prev.owners.filter((o) => o !== userId) } : prev);
  };

  // Epic handlers
  const handleLinkEpic = async (epicId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.addEpic(workspaceId, data.objective.id, epicId);
    loadData();
  };

  const handleUnlinkEpic = async (epicId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.removeEpic(workspaceId, data.objective.id, epicId);
    setData((prev) => prev ? { ...prev, epics: prev.epics.filter((e) => e.epic.id !== epicId) } : prev);
  };

  // Derived
  const isStrategic = form?.objective_type === 'strategic';
  const currentStateName = useMemo(
    () => stateOptions.find((s) => s.value === form?.state)?.label ?? 'Not Started',
    [form?.state],
  );
  const currentHealth = useMemo(
    () => healthOptions.find((h) => h.value === form?.health) ?? healthOptions[0],
    [form?.health],
  );
  const suggestedHealth = data?.suggested_health;
  const suggestedLabel = healthOptions.find((h) => h.value === suggestedHealth);

  const goBack = () => navigate({ to: '/w/$slug/pm/objectives', params: { slug } });

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !data || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Objective not found'}</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Objectives
        </Button>
      </div>
    );
  }

  const krAvgProgress = data.key_results.length > 0
    ? Math.round(data.key_results.reduce((sum, kr) => sum + kr.progress, 0) / data.key_results.length)
    : 0;

  const epicProgress = data.stats.epic_story_count > 0
    ? Math.round((data.stats.epic_done_stories / data.stats.epic_story_count) * 100)
    : 0;

  return (
    <div className="flex h-full flex-col">
      {/* ── Header bar ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          {isStrategic
            ? <Crosshair className="h-3.5 w-3.5 shrink-0 text-violet-500" />
            : <Target className="h-3.5 w-3.5 shrink-0 text-blue-500" />}
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Objectives
          </button>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
          {/* Title */}
          <input
            type="text"
            aria-label="Objective title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          {/* Description */}
          <div className="mt-4">
            <TiptapEditor
              content={form.description}
              onChange={(html) => updateField('description', html, { description: html })}
              placeholder="Add a description..."
              className="border-transparent shadow-none"
              teams={teams}
            />
          </div>

          <Separator className="my-6" />

          {/* Key Results section */}
          <div>
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Key Results</h3>
              {data.key_results.length > 0 && (
                <span className="text-xs text-muted-foreground">{krAvgProgress}% avg progress</span>
              )}
            </div>
            {data.key_results.length > 0 && (
              <div className="mt-3 space-y-2">
                {data.key_results.map((kr) => (
                  <KeyResultRow
                    key={kr.id}
                    kr={kr}
                    workspaceId={workspaceId!}
                    onUpdate={handleUpdateKeyResult}
                    onDelete={() => handleDeleteKeyResult(kr.id)}
                  />
                ))}
              </div>
            )}

            {/* Add Key Result */}
            <div className="mt-3 flex items-center gap-2">
              <input
                type="text"
                value={newKrName}
                onChange={(e) => setNewKrName(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleCreateKeyResult()}
                placeholder="Add a key result..."
                className="flex-1 rounded-md border border-border/60 bg-transparent px-3 py-1.5 text-sm placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary"
              />
              <Select value={newKrType} onValueChange={(v) => setNewKrType(v as KeyResultType)}>
                <SelectTrigger className="h-8 w-28 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="boolean">Boolean</SelectItem>
                  <SelectItem value="percent">Percent</SelectItem>
                  <SelectItem value="numeric">Numeric</SelectItem>
                </SelectContent>
              </Select>
              <Button size="sm" onClick={handleCreateKeyResult} disabled={!newKrName.trim()}>
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>

          <Separator className="my-6" />

          {/* Linked Epics section */}
          <div>
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Linked Epics</h3>
              <span className="text-xs text-muted-foreground">{epicProgress}% progress</span>
            </div>

            {data.epics.length > 0 && (
              <div className="mt-3 space-y-2">
                {data.epics.map((e) => {
                  const pct = e.stats.story_count > 0
                    ? Math.round((e.stats.done_story_count / e.stats.story_count) * 100)
                    : 0;
                  return (
                    <div key={e.epic.id} className="group flex items-center gap-3 rounded-md border border-border/60 px-3 py-2">
                      <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium truncate">{e.epic.name}</p>
                        <p className="text-[10px] text-muted-foreground">
                          {e.stats.done_story_count}/{e.stats.story_count} stories done
                        </p>
                      </div>
                      <div className="w-20">
                        <Progress value={pct} className="h-1.5" />
                      </div>
                      <span className="w-8 text-right text-xs text-muted-foreground">{pct}%</span>
                      <button
                        type="button"
                        className="text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive cursor-pointer transition-opacity"
                        onClick={() => handleUnlinkEpic(e.epic.id)}
                      >
                        <X className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  );
                })}
              </div>
            )}

            <div className="mt-3">
              <LinkEpicPopover
                workspaceId={workspaceId!}
                linkedEpicIds={data.epics.map((e) => e.epic.id)}
                onLink={handleLinkEpic}
              />
            </div>
          </div>

          <Separator className="my-6" />

          {/* Progress summary */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Key Result Progress</span>
                <span>{krAvgProgress}%</span>
              </div>
              <Progress value={krAvgProgress} />
            </div>
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Epic Progress</span>
                <span>{epicProgress}%</span>
              </div>
              <Progress value={epicProgress} />
            </div>
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_80px_1fr] items-start gap-x-2 gap-y-3">
            {/* State */}
            <MetadataRow icon={Hash} label="State">
              <SidebarPopoverSelect
                value={form.state}
                options={stateOptions}
                onChange={(v) => updateField('state', v as ObjectiveState, { state: v as ObjectiveState })}
                renderTrigger={() => <span>{currentStateName}</span>}
              />
            </MetadataRow>

            {/* Health */}
            {form.state !== 'closed' && (
            <MetadataRow icon={Heart} label="Health">
              <div className="flex flex-col gap-1">
                <SidebarPopoverSelect
                  value={form.health}
                  options={healthOptions.map((h) => ({ value: h.value, label: h.label }))}
                  onChange={(v) => updateField('health', v as ObjectiveHealth, { health: v as ObjectiveHealth })}
                  renderTrigger={() => (
                    <span className={currentHealth.color}>{currentHealth.label}</span>
                  )}
                />
                {suggestedLabel && suggestedHealth !== form.health && (
                  <button
                    type="button"
                    className="text-[10px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer text-left"
                    onClick={() => updateField('health', suggestedHealth!, { health: suggestedHealth })}
                  >
                    Suggested: <span className={suggestedLabel.color}>{suggestedLabel.label}</span>
                  </button>
                )}
              </div>
            </MetadataRow>
            )}

            {/* Teams */}
            <MetadataRow icon={Users} label="Teams">
              <MultiValueList
                items={data.teams}
                allOptions={teams.map((t) => ({ id: t.id, name: t.name }))}
                onAdd={handleAddTeam}
                onRemove={handleRemoveTeam}
                placeholder="Add team"
              />
            </MetadataRow>

            {/* Owners */}
            <MetadataRow icon={User} label="Owners">
              <MultiValueList
                items={data.owners}
                allOptions={ownerOptions}
                onAdd={handleAddOwner}
                onRemove={handleRemoveOwner}
                placeholder="Add owner"
              />
            </MetadataRow>

            {/* Start Date */}
            <MetadataRow icon={CalendarDays} label="Start date">
              <DatePicker
                value={form.planned_start_date}
                onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={CalendarDays} label="Target date">
              <DatePicker
                value={form.deadline}
                onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                placeholder="None"
                className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
              />
            </MetadataRow>
          </div>

          {/* Labels */}
          {data.labels && data.labels.length > 0 && (
            <div className="mt-6">
              <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Labels</h4>
              <div className="flex flex-wrap gap-1">
                {data.labels.map((l) => (
                  <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                    {l.name}
                  </span>
                ))}
              </div>
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}
