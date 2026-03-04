import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import {
  CalendarDays,
  ChevronRight,
  Hash,
  Heart,
  Hexagon,
  Loader2,
  Minus,
  Settings2,
  Sun,
  User,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { usePMWorkflowStore } from '@/stores/pmWorkflowStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { EpicWithStats, Story, EpicHealth, UpdateEpicRequest } from '@/lib/pmTypes';

const healthOptions: EpicHealth[] = ['on_track', 'at_risk', 'off_track'];

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

// ── Column definitions ─────────────────────────────────────────────

type ColumnKey =
  | 'health'
  | 'status'
  | 'owner'
  | 'team'
  | 'target_date'
  | 'start_date'
  | 'stories'
  | 'points'
  | 'created'
  | 'updated'
  | 'labels';

interface ColumnDef {
  key: ColumnKey;
  label: string;
  width: string;
}

const ALL_COLUMNS: ColumnDef[] = [
  { key: 'health', label: 'Health', width: '120px' },
  { key: 'status', label: 'Status', width: '80px' },
  { key: 'owner', label: 'Owner', width: '80px' },
  { key: 'team', label: 'Team', width: '110px' },
  { key: 'target_date', label: 'Target date', width: '110px' },
  { key: 'start_date', label: 'Start date', width: '110px' },
  { key: 'stories', label: 'Stories', width: '90px' },
  { key: 'points', label: 'Points', width: '80px' },
  { key: 'created', label: 'Created', width: '100px' },
  { key: 'updated', label: 'Updated', width: '100px' },
  { key: 'labels', label: 'Labels', width: '120px' },
];

const DEFAULT_VISIBLE: ColumnKey[] = [
  'health',
  'status',
  'owner',
  'team',
  'target_date',
  'stories',
  'points',
];

// ── Cell renderer ──────────────────────────────────────────────────

function EpicCell({
  columnKey,
  entry,
  completionPct,
  findTeamName,
}: {
  columnKey: ColumnKey;
  entry: EpicWithStats;
  completionPct: number;
  findTeamName: (id: string | undefined) => string | undefined;
}) {
  switch (columnKey) {
    case 'health':
      return (
        <span className={`text-xs ${healthConfig[entry.epic.health]?.color ?? 'text-muted-foreground'}`}>
          {healthConfig[entry.epic.health]?.label ?? 'No updates'}
        </span>
      );
    case 'status':
      return (
        <div className="flex items-center gap-1.5 text-xs">
          <Sun className="h-3.5 w-3.5 text-amber-500" />
          <span>{completionPct}%</span>
        </div>
      );
    case 'owner':
      return entry.epic.owner_id ? (
        <Avatar className="h-6 w-6 border border-border/80">
          <AvatarFallback className="text-[9px] font-semibold bg-muted/60">
            {entry.epic.owner_id.slice(0, 2).toUpperCase()}
          </AvatarFallback>
        </Avatar>
      ) : (
        <User className="h-4 w-4 text-muted-foreground/50" />
      );
    case 'team':
      return (
        <span className="truncate text-xs text-muted-foreground">
          {findTeamName(entry.epic.team_id) || <Minus className="h-3.5 w-3.5" />}
        </span>
      );
    case 'target_date':
      return (
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          {entry.epic.deadline ? (
            <>
              <CalendarDays className="h-3.5 w-3.5" />
              <span>{format(parseISO(entry.epic.deadline), 'MMM d')}</span>
            </>
          ) : (
            <Minus className="h-3.5 w-3.5" />
          )}
        </div>
      );
    case 'start_date':
      return (
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          {entry.epic.planned_start_date ? (
            <>
              <CalendarDays className="h-3.5 w-3.5" />
              <span>{format(parseISO(entry.epic.planned_start_date), 'MMM d')}</span>
            </>
          ) : (
            <Minus className="h-3.5 w-3.5" />
          )}
        </div>
      );
    case 'stories':
      return (
        <span className="text-xs text-muted-foreground">
          {entry.stats.done_story_count}/{entry.stats.story_count}
        </span>
      );
    case 'points':
      return (
        <span className="text-xs text-muted-foreground">
          {entry.stats.done_points}/{entry.stats.total_points}
        </span>
      );
    case 'created':
      return (
        <span className="text-xs text-muted-foreground">
          {format(parseISO(entry.epic.created_at), 'MMM d')}
        </span>
      );
    case 'updated':
      return (
        <span className="text-xs text-muted-foreground">
          {format(parseISO(entry.epic.updated_at), 'MMM d')}
        </span>
      );
    case 'labels':
      return (
        <div className="flex flex-wrap gap-1">
          {(entry.labels ?? []).length > 0 ? (
            entry.labels.map((l) => (
              <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                {l.name}
              </span>
            ))
          ) : (
            <Minus className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </div>
      );
    default:
      return null;
  }
}

// ── Display Properties Toggle ──────────────────────────────────────

function DisplayPropertiesPopover({
  visible,
  onChange,
}: {
  visible: ColumnKey[];
  onChange: (next: ColumnKey[]) => void;
}) {
  const toggle = (key: ColumnKey) => {
    onChange(
      visible.includes(key)
        ? visible.filter((k) => k !== key)
        : [...visible, key]
    );
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-1.5 text-xs">
          <Settings2 className="h-3.5 w-3.5" />
          Display
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-3" align="end">
        <p className="mb-2.5 text-xs font-medium text-muted-foreground">Display properties</p>
        <div className="flex flex-wrap gap-1.5">
          {ALL_COLUMNS.map((col) => {
            const active = visible.includes(col.key);
            return (
              <button
                key={col.key}
                type="button"
                onClick={() => toggle(col.key)}
                className={`rounded-full border px-2.5 py-1 text-xs transition-colors cursor-pointer
                  ${active
                    ? 'border-border bg-accent font-medium text-foreground'
                    : 'border-border/60 text-muted-foreground hover:border-border hover:text-foreground'}
                `}
              >
                {col.label}
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

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

// ── Epic Detail Body ───────────────────────────────────────────────

interface EpicFormState {
  name: string;
  description: string;
  team_id: string;
  epic_state_id: string;
  health: EpicHealth;
  planned_start_date: string;
  deadline: string;
}

const buildEpicForm = (epic: EpicWithStats): EpicFormState => ({
  name: epic.epic.name,
  description: epic.epic.description ?? '',
  team_id: epic.epic.team_id ?? '',
  epic_state_id: epic.epic.epic_state_id ?? '',
  health: epic.epic.health,
  planned_start_date: epic.epic.planned_start_date ?? '',
  deadline: epic.epic.deadline ? epic.epic.deadline.slice(0, 10) : '',
});

function EpicDetailBody({
  workspaceId,
  epic,
  stories,
  onClose,
  onEpicUpdated,
}: {
  workspaceId: string;
  epic: EpicWithStats;
  stories: Story[];
  onClose: () => void;
  onEpicUpdated: (epic: EpicWithStats) => void;
}) {
  const [form, setForm] = useState<EpicFormState>(() => buildEpicForm(epic));
  const [pendingPatch, setPendingPatch] = useState<UpdateEpicRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const epicStates = usePMWorkflowStore((s) => s.epicStates);
  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);

  const progress = useMemo(() => {
    if (epic.stats.story_count === 0) return 0;
    return Math.round((epic.stats.done_story_count / epic.stats.story_count) * 100);
  }, [epic]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      const { data, error } = await pmEpicService.update(workspaceId, epic.epic.id, patch);
      if (error || !data) {
        setSaveError(error ?? 'Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      } else {
        setSaveError(null);
        onEpicUpdated(data);
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [workspaceId, epic.epic.id, pendingPatch, saving, onEpicUpdated]);

  const queuePatch = (patch: UpdateEpicRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof EpicFormState>(key: K, value: EpicFormState[K], patch: UpdateEpicRequest) => {
    setForm((current) => ({ ...current, [key]: value }));
    queuePatch(patch);
  };

  const currentStateName = useMemo(
    () => epicStates.find((s) => s.id === form.epic_state_id)?.name ?? 'No state',
    [epicStates, form.epic_state_id],
  );

  const currentTeamName = useMemo(
    () => (form.team_id ? findTeamName(form.team_id) ?? 'No team' : 'No team'),
    [form.team_id, findTeamName],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onClose}>
          <X className="h-4 w-4" />
        </Button>
        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <Hexagon className="h-3.5 w-3.5 shrink-0 text-violet-500" />
          <span className="shrink-0 font-medium text-foreground/80">Epic</span>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate">{form.name || 'Untitled'}</span>
        </div>
        <div className="ml-auto flex items-center gap-1 text-xs text-muted-foreground">
          {saving ? (
            <span className="inline-flex items-center gap-1">
              <Loader2 className="h-3 w-3 animate-spin" />
              Saving...
            </span>
          ) : (
            <span>All changes saved</span>
          )}
          {saveError && <span className="ml-2 text-destructive">{saveError}</span>}
        </div>
      </div>

      {/* Two-column grid */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
        {/* Left column */}
        <div className="min-h-0 overflow-y-auto px-8 py-5">
          <input
            type="text"
            aria-label="Epic title"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            placeholder="Untitled"
          />

          <div className="mt-4">
            <TiptapEditor
              content={form.description}
              onChange={(html) => updateField('description', html, { description: html })}
              placeholder="Add a description..."
              className="border-transparent shadow-none"
            />
          </div>

          <Separator className="my-6" />

          {/* Progress */}
          <div className="space-y-2">
            <Progress value={progress} />
            <p className="text-xs text-muted-foreground">
              {epic.stats.done_story_count}/{epic.stats.story_count} stories done · {epic.stats.done_points}/{epic.stats.total_points} points
            </p>
          </div>

          {/* Stories */}
          <div className="mt-6">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Stories in this epic</h3>
            {stories.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">No stories linked yet.</p>
            ) : (
              <div className="mt-3 space-y-1">
                {stories.map((story) => (
                  <article key={story.id} className="rounded-md border border-border/70 px-3 py-2 text-sm">
                    <p className="font-medium">TP-{story.display_id} · {story.name}</p>
                    <p className="text-xs text-muted-foreground">{story.priority} priority · {story.story_type}</p>
                  </article>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right column — metadata sidebar */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            {/* State */}
            <MetadataRow icon={Hash} label="State">
              <SidebarPopoverSelect
                value={form.epic_state_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No state' },
                  ...epicStates.map((s) => ({ value: s.id, label: s.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('epic_state_id', val, { epic_state_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentStateName}</span>}
              />
            </MetadataRow>

            {/* Health */}
            <MetadataRow icon={Heart} label="Health">
              <SidebarPopoverSelect
                value={form.health}
                options={healthOptions.map((h) => ({ value: h, label: healthConfig[h].label, className: healthConfig[h].color }))}
                onChange={(v) => updateField('health', v as EpicHealth, { health: v as EpicHealth })}
                renderTrigger={() => (
                  <span className={healthConfig[form.health]?.color}>{healthConfig[form.health]?.label}</span>
                )}
              />
            </MetadataRow>

            {/* Team */}
            <MetadataRow icon={Users} label="Team">
              <SidebarPopoverSelect
                value={form.team_id || '__none__'}
                options={[
                  { value: '__none__', label: 'No team' },
                  ...teams.map((t) => ({ value: t.id, label: t.name })),
                ]}
                onChange={(v) => {
                  const val = v === '__none__' ? '' : v;
                  updateField('team_id', val, { team_id: val || undefined });
                }}
                renderTrigger={() => <span>{currentTeamName}</span>}
              />
            </MetadataRow>

            {/* Owner — display only for now */}
            <MetadataRow icon={User} label="Owner">
              <span className="text-xs text-muted-foreground">
                {epic.epic.owner_id ? epic.epic.owner_id.slice(0, 8) : 'Nobody'}
              </span>
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
        </aside>
      </div>
    </div>
  );
}

// ── Main page ──────────────────────────────────────────────────────

export function EpicsPage() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const loadEpicStates = usePMWorkflowStore((state) => state.loadEpicStates);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [selectedEpic, setSelectedEpic] = useState<EpicWithStats | null>(null);
  const [selectedEpicStories, setSelectedEpicStories] = useState<Story[]>([]);

  const [visibleColumns, setVisibleColumns] = useState<ColumnKey[]>(DEFAULT_VISIBLE);

  const workspaceId = workspace?.id;
  const { findTeamName } = useWorkspaceTeams(workspaceId);

  const activeColumns = useMemo(
    () => ALL_COLUMNS.filter((col) => visibleColumns.includes(col.key)),
    [visibleColumns]
  );

  const gridTemplate = useMemo(
    () => `1fr ${activeColumns.map((c) => c.width).join(' ')}`,
    [activeColumns]
  );

  const loadData = async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const [epicsRes] = await Promise.all([
      pmEpicService.list(workspaceId, { archived: false }),
      loadEpicStates(workspaceId),
    ]);
    if (epicsRes.error || !epicsRes.data) {
      setError(epicsRes.error ?? 'Failed to load epics');
      setLoading(false);
      return;
    }
    setEpics(epicsRes.data);
    setLoading(false);
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId]);

  // Refresh when epic is created via global modal
  useEffect(() => {
    const handler = () => { loadData(); };
    window.addEventListener('epic-created', handler);
    return () => window.removeEventListener('epic-created', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId]);

  const openEpicDetail = async (epic: EpicWithStats) => {
    setSelectedEpic(epic);
    if (!workspaceId) return;
    const storiesRes = await pmEpicService.listStories(workspaceId, epic.epic.id);
    setSelectedEpicStories(storiesRes.data ?? []);
  };

  const completionPct = (entry: EpicWithStats) => {
    if (entry.stats.story_count === 0) return 0;
    return Math.round((entry.stats.done_story_count / entry.stats.story_count) * 100);
  };

  const handleEpicUpdated = (updated: EpicWithStats) => {
    setSelectedEpic(updated);
    setEpics((prev) => prev.map((e) => (e.epic.id === updated.epic.id ? updated : e)));
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Epics</h2>
          <p className="text-sm text-muted-foreground">Track long-running initiatives and their story progress.</p>
        </div>
        <DisplayPropertiesPopover visible={visibleColumns} onChange={setVisibleColumns} />
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading epics...</p>
      ) : epics.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No epics yet. Use the Create button in the header to add your first epic.
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border border-border/60">
          {/* Table header */}
          <div
            className="grid items-center gap-2 border-b border-border/60 bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground"
            style={{ gridTemplateColumns: gridTemplate }}
          >
            <span>Name</span>
            {activeColumns.map((col) => (
              <span key={col.key}>{col.label}</span>
            ))}
          </div>

          {/* Table rows */}
          {epics.map((entry) => (
            <div
              key={entry.epic.id}
              role="button"
              tabIndex={0}
              onClick={() => openEpicDetail(entry)}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openEpicDetail(entry); } }}
              className="grid cursor-pointer items-center gap-2 border-b border-border/40 px-4 py-3 text-sm transition-colors last:border-b-0 hover:bg-accent/40"
              style={{ gridTemplateColumns: gridTemplate }}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
                <span className="truncate font-medium">{entry.epic.name}</span>
              </div>
              {activeColumns.map((col) => (
                <div key={col.key} className="min-w-0">
                  <EpicCell
                    columnKey={col.key}
                    entry={entry}
                    completionPct={completionPct(entry)}
                    findTeamName={findTeamName}
                  />
                </div>
              ))}
            </div>
          ))}
        </div>
      )}

      <Sheet open={Boolean(selectedEpic)} onOpenChange={(open) => !open && setSelectedEpic(null)}>
        <SheetContent side="right" className="w-[85vw] !max-w-[85vw] p-0" showCloseButton={false}>
          <SheetTitle className="sr-only">Epic Detail</SheetTitle>
          {selectedEpic && workspaceId ? (
            <EpicDetailBody
              key={selectedEpic.epic.id}
              workspaceId={workspaceId}
              epic={selectedEpic}
              stories={selectedEpicStories}
              onClose={() => setSelectedEpic(null)}
              onEpicUpdated={handleEpicUpdated}
            />
          ) : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}
