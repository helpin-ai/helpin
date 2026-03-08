import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { differenceInDays, format, formatDistanceToNow, parseISO } from 'date-fns';
import { useTitle } from '@/hooks/useTitle';
import {
  ArrowLeft,
  CalendarDays,
  Crosshair,
  Hash,
  Heart,
  Hexagon,
  Info,
  Loader2,
  Pencil,
  Plus,
  Target,
  Trash2,
  User,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { buildAssignableMemberNameMap, buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { FollowButton } from '@/components/notifications/FollowButton';
import type {
  EpicWithStats,
  KeyResult,
  KeyResultType,
  ObjectiveHealth,
  ObjectiveState,
  ObjectiveWithDetails,
  UpdateObjectiveRequest,
} from '@/lib/pmTypes';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';

const routeApi = getRouteApi('/_authenticated/w/$slug/pm/objectives/$objectiveId');

const stateOptions: { value: ObjectiveState; label: string; className: string }[] = (
  Object.entries(OBJECTIVE_STATE_CONFIG) as [ObjectiveState, typeof OBJECTIVE_STATE_CONFIG[ObjectiveState]][]
).map(([value, cfg]) => ({ value, label: cfg.label, className: cfg.color }));

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
  readOnly,
}: {
  items: string[];
  allOptions: { id: string; name: string }[];
  onAdd: (id: string) => void;
  onRemove: (id: string) => void;
  placeholder: string;
  readOnly?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const available = allOptions.filter((o) => !items.includes(o.id));
  const selected = allOptions.filter((o) => items.includes(o.id));

  return (
    <div className="space-y-1">
      {selected.length === 0 && readOnly && (
        <span className="text-xs text-muted-foreground px-1.5 py-0.5">None</span>
      )}
      {selected.map((item) => (
        <div key={item.id} className="flex items-center justify-between rounded-md bg-muted/50 px-2 py-0.5 text-xs">
          <span className="truncate">{item.name}</span>
          {!readOnly && (
            <button type="button" className="text-muted-foreground hover:text-destructive cursor-pointer" onClick={() => onRemove(item.id)}>
              <X className="h-3 w-3" />
            </button>
          )}
        </div>
      ))}
      {!readOnly && (
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
      )}
    </div>
  );
}

// ── Key Result Row ─────────────────────────────────────────────────

function KeyResultRow({
  kr,
  workspaceId,
  memberMap,
  onUpdate,
  onDelete,
  readOnly,
}: {
  kr: KeyResult;
  workspaceId: string;
  memberMap: Map<string, string>;
  onUpdate: (updated: KeyResult) => void;
  onDelete: () => void;
  readOnly?: boolean;
}) {
  const [editingName, setEditingName] = useState(false);
  const [name, setName] = useState(kr.name);
  const [currentValue, setCurrentValue] = useState(String(kr.current_value));
  // Sync local state when kr prop changes (after save)
  useEffect(() => { setName(kr.name); }, [kr.name]);
  useEffect(() => { setCurrentValue(String(kr.current_value)); }, [kr.current_value]);

  const saveValue = async () => {
    const val = parseFloat(currentValue);
    if (isNaN(val) || val === kr.current_value) return;
    const { data } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, { current_value: val });
    if (data) onUpdate(data);
  };

  const saveName = async () => {

    if (name.trim() === kr.name || !name.trim()) {
      setName(kr.name);
      setEditingName(false);
      return;
    }
    const { data } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, { name: name.trim() });
    if (data) onUpdate(data);
    setEditingName(false);
  };


  const lastUpdated = formatDistanceToNow(parseISO(kr.updated_at), { addSuffix: true });
  const updatedByName = kr.updated_by ? memberMap.get(kr.updated_by) : undefined;

  return (
    <div className="group flex items-center gap-3 rounded-lg border border-border/60 px-4 py-3">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          {!readOnly && editingName ? (
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={saveName}
              onKeyDown={(e) => e.key === 'Enter' && saveName()}
              className="w-full bg-transparent text-sm font-medium focus:outline-none"
              autoFocus
            />
          ) : !readOnly ? (
            <button
              type="button"
              className="text-sm font-medium text-foreground hover:underline cursor-pointer text-left truncate"
              onClick={() => setEditingName(true)}
            >
              {kr.name}
            </button>
          ) : (
            <span className="text-sm font-medium text-foreground text-left truncate">{kr.name}</span>
          )}
        </div>
        <div className="mt-0.5 flex items-center gap-1.5">
          <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground uppercase">{kr.result_type}</span>
          {kr.result_type === 'boolean' ? (
            readOnly ? (
              <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                kr.progress >= 100
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-muted text-muted-foreground'
              }`}>
                {kr.progress >= 100 ? 'Done' : 'Not done'}
              </span>
            ) : (
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
            )
          ) : (
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span>{kr.initial_value}</span>
              <span>→</span>
              {readOnly ? (
                <span className="w-14 text-center font-medium text-foreground">{kr.current_value}</span>
              ) : (
                <input
                  type="number"
                  value={currentValue}
                  onChange={(e) => setCurrentValue(e.target.value)}
                  onBlur={saveValue}
                  onKeyDown={(e) => e.key === 'Enter' && saveValue()}
                  title="Current value — edit to update progress"
                  className="w-14 rounded border border-border bg-transparent px-1.5 py-0.5 text-[11px] text-center font-medium text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                />
              )}
              <span>→ {kr.target_value}</span>
            </div>
          )}
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground tabular-nums">{Math.round(kr.progress)}%</span>
          <div className="w-24">
            <Progress value={kr.progress} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
          </div>
          {!readOnly && (
            <button
              type="button"
              className="text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive cursor-pointer transition-opacity"
              onClick={onDelete}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        <span className={`text-[11px] text-muted-foreground ${readOnly ? '' : 'pr-6'}`}>
          {updatedByName ? `${updatedByName}, ${lastUpdated}` : `Updated ${lastUpdated}`}
        </span>
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
        <Button variant="outline" size="sm" className="h-7 text-xs">
          <Plus className="mr-1 h-3 w-3" />
          Add Epics
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
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit, isAdmin } = usePermissions(access);

  const [data, setData] = useState<ObjectiveWithDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateObjectiveRequest>({});
  const pendingPatchRef = useRef<UpdateObjectiveRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const { teams } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const ownerOptions = useMemo(
    () => buildAssignableMemberOptions(assignableMembers),
    [assignableMembers],
  );
  const memberMap = useMemo(() => {
    return buildAssignableMemberNameMap(assignableMembers);
  }, [assignableMembers]);

  useTitle(form?.name ? `${form.name} — Objective` : 'Objective');

  // ── Key result modal state
  const [krModalOpen, setKrModalOpen] = useState(false);
  const [newKrName, setNewKrName] = useState('');
  const [newKrType, setNewKrType] = useState<KeyResultType>('percent');
  const [newKrStart, setNewKrStart] = useState('0');
  const [newKrTarget, setNewKrTarget] = useState('100');
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);

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
      pendingPatchRef.current = {};
      setSaving(true);
      const { data: updated, error: err } = await pmObjectiveService.update(workspaceId, data.objective.id, patch);
      if (err || !updated) {
        setSaveError(err ?? 'Failed to save');
        setPendingPatch((current) => {
          const next = { ...patch, ...current };
          pendingPatchRef.current = next;
          return next;
        });
      } else {
        setSaveError(null);
        setData(updated);
        // Re-sync form from server, but don't overwrite fields the user edited during the save
        const fresh = buildForm(updated);
        const stillPending = pendingPatchRef.current;
        setForm((current) => {
          if (!current) return current;
          const synced = { ...current };
          for (const key of Object.keys(fresh) as (keyof FormState)[]) {
            if (!(key in stillPending)) {
              (synced as any)[key] = fresh[key];
            }
          }
          return synced;
        });
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, data, pendingPatch, saving]);

  const queuePatch = (patch: UpdateObjectiveRequest) => {
    setPendingPatch((current) => {
      const next = { ...current, ...patch };
      pendingPatchRef.current = next;
      return next;
    });
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateObjectiveRequest) => {
    setForm((current) => current ? { ...current, [key]: value } : current);
    queuePatch(patch);
  };

  // Key result handlers
  const handleCreateKeyResult = async () => {
    if (!workspaceId || !data || !newKrName.trim()) return;
    const startVal = parseFloat(newKrStart) || 0;
    const targetVal = newKrType === 'boolean' ? 1 : (parseFloat(newKrTarget) || 100);
    const { data: kr } = await pmObjectiveService.createKeyResult(workspaceId, data.objective.id, {
      name: newKrName.trim(),
      result_type: newKrType,
      initial_value: startVal,
      current_value: startVal,
      target_value: targetVal,
    });
    if (kr) {
      setData((prev) => prev ? { ...prev, key_results: [...prev.key_results, kr] } : prev);
      setNewKrName('');
      setNewKrType('percent');
      setNewKrStart('0');
      setNewKrTarget('100');
      setKrModalOpen(false);
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

  const handleAddOwner = async (workspaceMemberId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.addOwner(workspaceId, data.objective.id, workspaceMemberId);
    setData((prev) => prev ? {
      ...prev,
      owner_member_ids: [...new Set([...(prev.owner_member_ids ?? []), workspaceMemberId])],
    } : prev);
  };

  const handleRemoveOwner = async (workspaceMemberId: string) => {
    if (!workspaceId || !data) return;
    await pmObjectiveService.removeOwner(workspaceId, data.objective.id, workspaceMemberId);
    setData((prev) => prev ? {
      ...prev,
      owner_member_ids: (prev.owner_member_ids ?? []).filter((id) => id !== workspaceMemberId),
    } : prev);
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
  const currentState = useMemo(
    () => stateOptions.find((s) => s.value === form?.state) ?? stateOptions[0],
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
  const ownerIds = data.owner_member_ids ?? data.owners;

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
          <span className="text-xs font-medium uppercase tracking-wide">
            {isStrategic ? 'Strategic' : 'Tactical'} Objective
          </span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <FollowButton entityType="objective" entityId={data.objective.id} />
        </div>
      </div>

      {/* ── Two-column layout ───────────────────────────────────── */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* ── Left column ────────────────────────────────────────── */}
        <div className="min-h-0 overflow-y-auto px-8 py-8">
          {/* ── Objective Header Card ──────────────────────────── */}
          <div className="rounded-lg border border-border/60 p-6">
            {canEdit ? (
              <input
                type="text"
                aria-label="Objective title"
                value={form.name}
                onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
                className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                placeholder="Untitled"
              />
            ) : (
              <h1 className="text-2xl font-bold text-foreground">{form.name}</h1>
            )}
            <div className="mt-3">
              {editingDescription ? (
                <div>
                  <TiptapEditor
                    content={form.description}
                    onChange={(html) => updateField('description', html, { description: html })}
                    placeholder="Add a description..."
                    className="border-transparent shadow-none"
                    teams={teams}
                  />
                  <div className="mt-2 flex justify-end">
                    <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setEditingDescription(false)}>
                      Done
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="group/desc relative">
                  {form.description ? (
                    <div className="prose prose-sm dark:prose-invert max-w-none text-sm" dangerouslySetInnerHTML={{ __html: form.description }} />
                  ) : (
                    <p className="text-sm text-muted-foreground">{canEdit ? 'No description yet' : 'No description'}</p>
                  )}
                  {canEdit && (
                    <button
                      type="button"
                      className="mt-2 inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                      onClick={() => setEditingDescription(true)}
                    >
                      <Pencil className="h-3 w-3" />
                      Edit description
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* ── Progress Summary ────────────────────────────────── */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-foreground">Progress Summary</h3>
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Heart className="h-3.5 w-3.5" />
                <span>Health:</span>
                {canEdit ? (
                  <SidebarPopoverSelect
                    value={form.health}
                    options={healthOptions.map((h) => ({ value: h.value, label: h.label }))}
                    onChange={(v) => updateField('health', v as ObjectiveHealth, { health: v as ObjectiveHealth })}
                    renderTrigger={() => (
                      <span className={currentHealth.color}>{currentHealth.label}</span>
                    )}
                  />
                ) : (
                  <span className={currentHealth.color}>{currentHealth.label}</span>
                )}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              {/* Epic Progress Card */}
              <div className="rounded-lg border border-border/60 p-5">
                <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                  <Hexagon className="h-4 w-4 text-violet-500" />
                  Epic Progress
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Info className="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[240px] text-xs">
                        Percentage of done stories across all linked epics: done stories ÷ total stories.
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <div className="mt-3">
                  <span className="text-3xl font-bold">{epicProgress}%</span>
                  <span className="ml-1.5 text-sm text-muted-foreground">Complete</span>
                </div>
                <Progress value={epicProgress} className="mt-3 h-2.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
                <p className="mt-2 text-xs text-muted-foreground">
                  Last updated {data.epics.length > 0 ? formatDistanceToNow(parseISO(data.epics.reduce((latest, e) => e.epic.updated_at > latest ? e.epic.updated_at : latest, data.epics[0].epic.updated_at)), { addSuffix: true }) : 'never'}
                </p>
              </div>

              {/* Target Date Card */}
              <div className="rounded-lg border border-border/60 p-5">
                <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                  <CalendarDays className="h-4 w-4 text-blue-500" />
                  Target Date
                </div>
                <div className="mt-3">
                  {form.deadline ? (
                    <>
                      <span className="text-3xl font-bold">{format(parseISO(form.deadline), 'MMM d,')}</span>
                      <span className="ml-1.5 text-lg text-muted-foreground">{format(parseISO(form.deadline), 'yyyy')}</span>
                    </>
                  ) : (
                    <span className="text-lg text-muted-foreground">No target date</span>
                  )}
                </div>
                {form.deadline && (() => {
                  const start = form.planned_start_date ? parseISO(form.planned_start_date) : data.objective.created_at ? parseISO(data.objective.created_at) : new Date();
                  const end = parseISO(form.deadline);
                  const now = new Date();
                  const totalDays = Math.max(differenceInDays(end, start), 1);
                  const elapsed = Math.max(differenceInDays(now, start), 0);
                  const timePct = Math.min(Math.round((elapsed / totalDays) * 100), 100);
                  const daysLeft = differenceInDays(end, now);
                  return (
                    <>
                      <Progress value={timePct} className="mt-3 h-2.5 bg-sky-500/15 [&>[data-slot=progress-indicator]]:bg-sky-500" />
                      <p className={`mt-2 text-xs ${form.state === 'closed' ? 'text-muted-foreground' : daysLeft <= 7 ? 'text-red-500 font-medium' : daysLeft <= 14 ? 'text-amber-500' : 'text-muted-foreground'}`}>
                        {form.state === 'closed'
                          ? 'Completed'
                          : daysLeft > 0
                          ? `Time remaining: ${daysLeft} day${daysLeft !== 1 ? 's' : ''}`
                          : daysLeft === 0 ? 'Due today' : `${Math.abs(daysLeft)} day${Math.abs(daysLeft) !== 1 ? 's' : ''} overdue`}
                      </p>
                    </>
                  );
                })()}
              </div>
            </div>

            {/* Progress insight */}
            {data.key_results.length > 0 && Math.abs(epicProgress - krAvgProgress) >= 20 && (
              <p className="mt-3 rounded-lg border border-border/60 p-3 text-xs text-muted-foreground">
                {epicProgress > krAvgProgress
                  ? `${epicProgress}% of work is done but only ${krAvgProgress}% of outcomes achieved — results may be lagging behind effort.`
                  : `${krAvgProgress}% of outcomes achieved with ${epicProgress}% of work done — good outcome efficiency.`}
              </p>
            )}
          </div>

          {/* ── Key Results ─────────────────────────────────────── */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-foreground">Key Results</h3>
              <div className="flex items-center gap-2">
                {data.key_results.length > 0 && (
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="text-xs text-muted-foreground cursor-help border-b border-dotted border-muted-foreground/40">{krAvgProgress}% outcome progress</span>
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[240px] text-xs">
                        Average progress across all key results. Each key result's progress is: (current − initial) ÷ (target − initial).
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                )}
                {canEdit && (
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setKrModalOpen(true)}>
                    <Plus className="mr-1 h-3 w-3" />
                    Add Key Results
                  </Button>
                )}
              </div>
            </div>
            {data.key_results.length > 0 ? (
              <div className="space-y-2">
                {data.key_results.map((kr) => (
                  <KeyResultRow
                    key={kr.id}
                    kr={kr}
                    workspaceId={workspaceId!}
                    memberMap={memberMap}
                    onUpdate={handleUpdateKeyResult}
                    onDelete={() => handleDeleteKeyResult(kr.id)}
                    readOnly={!canEdit}
                  />
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/60 p-6 text-center">
                <p className="text-sm text-muted-foreground">No key results yet</p>
                <p className="mt-1 text-xs text-muted-foreground/60">Add key results to track outcome progress</p>
              </div>
            )}
          </div>

          {/* ── Epics ───────────────────────────────────────────── */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-foreground">Epics</h3>
              {canEdit && (
                <LinkEpicPopover
                  workspaceId={workspaceId!}
                  linkedEpicIds={data.epics.map((e) => e.epic.id)}
                  onLink={handleLinkEpic}
                />
              )}
            </div>

            {data.epics.length > 0 ? (
              <div className="space-y-2">
                {data.epics.map((e) => {
                  const pct = e.stats.story_count > 0
                    ? Math.round((e.stats.done_story_count / e.stats.story_count) * 100)
                    : 0;
                  const epicState = e.epic.completed ? 'Done' : e.epic.started ? 'In Progress' : 'Not Started';
                  const epicStateColor = e.epic.completed ? 'text-green-500' : e.epic.started ? 'text-amber-500' : 'text-zinc-400';
                  const epicUpdated = formatDistanceToNow(parseISO(e.epic.updated_at), { addSuffix: true });
                  return (
                    <div key={e.epic.id} className="group flex items-center gap-3 rounded-lg border border-border/60 px-4 py-3">
                      <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium truncate">{e.epic.name}</p>
                        <div className="mt-0.5 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                          <span className={epicStateColor}>{epicState.toLowerCase()}</span>
                        </div>
                      </div>
                      <div className="flex shrink-0 flex-col items-end gap-1">
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground tabular-nums">{pct}%</span>
                          <div className="w-24">
                            <Progress value={pct} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
                          </div>
                          {canEdit && (
                            <button
                              type="button"
                              className="text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive cursor-pointer transition-opacity"
                              onClick={() => handleUnlinkEpic(e.epic.id)}
                            >
                              <X className="h-3.5 w-3.5" />
                            </button>
                          )}
                        </div>
                        <span className={`text-[11px] text-muted-foreground ${canEdit ? 'pr-6' : ''}`}>Updated {epicUpdated}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/60 p-8 text-center">
                <p className="text-sm text-muted-foreground">No epics linked yet</p>
              </div>
            )}
          </div>
        </div>

        {/* ── Right column — metadata sidebar ────────────────────── */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* State */}
            <MetadataRow icon={Hash} label="State">
              {canEdit ? (
                <SidebarPopoverSelect
                  value={form.state}
                  options={stateOptions}
                  onChange={(v) => updateField('state', v as ObjectiveState, { state: v as ObjectiveState })}
                  renderTrigger={() => <span className={currentState.className}>{currentState.label}</span>}
                />
              ) : (
                <span className={`text-xs ${currentState.className}`}>{currentState.label}</span>
              )}
            </MetadataRow>

            {/* Health */}
            {form.state !== 'closed' && (
            <MetadataRow icon={Heart} label="Health">
              <div className="flex flex-col gap-1">
                {canEdit ? (
                  <SidebarPopoverSelect
                    value={form.health}
                    options={healthOptions.map((h) => ({ value: h.value, label: h.label }))}
                    onChange={(v) => updateField('health', v as ObjectiveHealth, { health: v as ObjectiveHealth })}
                    renderTrigger={() => (
                      <span className={currentHealth.color}>{currentHealth.label}</span>
                    )}
                  />
                ) : (
                  <span className={`text-xs px-1.5 py-0.5 ${currentHealth.color}`}>{currentHealth.label}</span>
                )}
                {canEdit && suggestedLabel && suggestedHealth !== form.health && (
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

            {/* ── People ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Teams */}
            <MetadataRow icon={Users} label="Teams">
              <MultiValueList
                items={data.teams}
                allOptions={teams.map((t) => ({ id: t.id, name: t.name }))}
                onAdd={handleAddTeam}
                onRemove={handleRemoveTeam}
                placeholder="Add team"
                readOnly={!canEdit}
              />
            </MetadataRow>

            {/* Owners */}
              <MetadataRow icon={User} label="Owners">
                <MultiValueList
                  items={ownerIds}
                  allOptions={ownerOptions}
                  onAdd={handleAddOwner}
                  onRemove={handleRemoveOwner}
                placeholder="Add owner"
                readOnly={!canEdit}
              />
            </MetadataRow>

            {/* ── Planning ── */}
            <div className="col-span-3 h-px bg-border/40 my-1" />

            {/* Start Date */}
            <MetadataRow icon={CalendarDays} label="Start date">
              {canEdit ? (
                <DatePicker
                  value={form.planned_start_date}
                  onChange={(v) => updateField('planned_start_date', v, { planned_start_date: v || undefined })}
                  placeholder="None"
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : (
                <span className="text-xs px-1.5 py-0.5">
                  {form.planned_start_date ? format(parseISO(form.planned_start_date), 'MMM d, yyyy') : 'None'}
                </span>
              )}
            </MetadataRow>

            {/* Target Date */}
            <MetadataRow icon={CalendarDays} label="Target date">
              {canEdit ? (
                <DatePicker
                  value={form.deadline}
                  onChange={(v) => updateField('deadline', v, { deadline: v || undefined })}
                  placeholder="None"
                  className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                />
              ) : (
                <span className="text-xs px-1.5 py-0.5">
                  {form.deadline ? format(parseISO(form.deadline), 'MMM d, yyyy') : 'None'}
                </span>
              )}
            </MetadataRow>

            {/* ── Classification ── */}
            {data.labels && data.labels.length > 0 && (
              <>
                <div className="col-span-3 h-px bg-border/40 my-1" />
                <MetadataRow icon={Info} label="Labels">
                  <div className="flex flex-wrap gap-1">
                    {data.labels.map((l) => (
                      <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                        {l.name}
                      </span>
                    ))}
                  </div>
                </MetadataRow>
              </>
            )}
          </div>

          {/* Delete objective — admin only */}
          {isAdmin && (
            <div className="mt-8 border-t border-border/40 pt-4">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs text-destructive hover:text-destructive hover:bg-destructive/10"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                <Trash2 className="mr-1 h-3 w-3" />
                Delete objective
              </Button>
            </div>
          )}
        </aside>
      </div>

      {/* ── Add Key Result Modal ─────────────────────────────────── */}
      <Dialog open={krModalOpen} onOpenChange={setKrModalOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Add Key Result</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <label className="text-xs font-medium text-muted-foreground">Name</label>
              <input
                type="text"
                value={newKrName}
                onChange={(e) => setNewKrName(e.target.value)}
                placeholder="e.g., Increase activation rate"
                className="mt-1 w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary"
                autoFocus
              />
            </div>
            <div className={`grid gap-3 ${newKrType === 'boolean' ? 'grid-cols-1' : 'grid-cols-3'}`}>
              <div>
                <div className="flex items-center gap-1">
                  <label className="text-xs font-medium text-muted-foreground">Measure as</label>
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Info className="h-3 w-3 text-muted-foreground/60 cursor-help" />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[220px] text-xs">
                        <p className="font-medium mb-1">Measurement types:</p>
                        <p><strong>Boolean</strong> — Done / Not done</p>
                        <p><strong>Percent</strong> — 0–100%</p>
                        <p><strong>Numeric</strong> — Custom range (e.g. 0→50 users)</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <Select value={newKrType} onValueChange={(v) => {
                  setNewKrType(v as KeyResultType);
                  if (v === 'boolean') { setNewKrStart('0'); setNewKrTarget('1'); }
                  else if (v === 'percent') { setNewKrStart('0'); setNewKrTarget('100'); }
                }}>
                  <SelectTrigger className="mt-1 h-9 text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="percent">Percent</SelectItem>
                    <SelectItem value="numeric">Numeric</SelectItem>
                    <SelectItem value="boolean">Boolean</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {newKrType !== 'boolean' && (
                <>
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">Starting value</label>
                    <div className="relative mt-1">
                      <input
                        type="number"
                        value={newKrStart}
                        onChange={(e) => setNewKrStart(e.target.value)}
                        className="w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                      {newKrType === 'percent' && (
                        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground pointer-events-none">%</span>
                      )}
                    </div>
                  </div>
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">Target value</label>
                    <div className="relative mt-1">
                      <input
                        type="number"
                        value={newKrTarget}
                        onChange={(e) => setNewKrTarget(e.target.value)}
                        className="w-full rounded-md border border-border bg-transparent px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                      {newKrType === 'percent' && (
                        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground pointer-events-none">%</span>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setKrModalOpen(false)}>Cancel</Button>
            <Button size="sm" onClick={handleCreateKeyResult} disabled={!newKrName.trim()}>
              Add Key Result
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete objective"
        description="This objective and all its key results will be archived. This action cannot be easily undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={async () => {
          if (!workspaceId || !data) return;
          await pmObjectiveService.remove(workspaceId, data.objective.id);
          goBack();
        }}
      />
    </div>
  );
}
