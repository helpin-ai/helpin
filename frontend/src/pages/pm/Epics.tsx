import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import {
  CalendarDays,
  Hash,
  Heart,
  Hexagon,
  Loader2,
  Minus,
  Plus,
  Settings2,
  Sun,
  User,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { usePMWorkflowStore } from '@/stores/pmWorkflowStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { EpicWithStats, Story, EpicHealth } from '@/lib/pmTypes';

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
  width: string; // CSS grid width
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

// ── Main page ──────────────────────────────────────────────────────

export function EpicsPage() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const epicStates = usePMWorkflowStore((state) => state.epicStates);
  const loadEpicStates = usePMWorkflowStore((state) => state.loadEpicStates);

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [selectedEpic, setSelectedEpic] = useState<EpicWithStats | null>(null);
  const [selectedEpicStories, setSelectedEpicStories] = useState<Story[]>([]);

  const [createForm, setCreateForm] = useState({
    name: '',
    description: '',
    stateId: '',
    health: 'on_track' as EpicHealth,
    teamId: '',
    startDate: '',
    targetDate: '',
  });
  const [submitting, setSubmitting] = useState(false);

  const [visibleColumns, setVisibleColumns] = useState<ColumnKey[]>(DEFAULT_VISIBLE);

  const workspaceId = workspace?.id;
  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);

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

  const resetCreateForm = () =>
    setCreateForm({ name: '', description: '', stateId: '', health: 'on_track', teamId: '', startDate: '', targetDate: '' });

  const createEpic = async () => {
    if (!workspaceId || !createForm.name.trim() || submitting) return;
    setSubmitting(true);
    const payload = {
      workspace_id: workspaceId,
      name: createForm.name.trim(),
      description: createForm.description.trim() || undefined,
      epic_state_id: createForm.stateId || undefined,
      team_id: createForm.teamId || undefined,
      health: createForm.health,
      planned_start_date: createForm.startDate || undefined,
      deadline: createForm.targetDate || undefined,
    };
    const { error: createError } = await pmEpicService.create(payload);
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    setCreateOpen(false);
    resetCreateForm();
    await loadData();
  };

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

  const selectedEpicProgress = useMemo(() => {
    if (!selectedEpic) return 0;
    return completionPct(selectedEpic);
  }, [selectedEpic]);

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
        <div className="flex items-center gap-2">
          <DisplayPropertiesPopover visible={visibleColumns} onChange={setVisibleColumns} />
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
            Create Epic
          </Button>
        </div>
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
            No epics yet. Create your first epic to start organizing stories.
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
              {/* Name — always visible */}
              <div className="flex items-center gap-2.5 min-w-0">
                <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
                <span className="truncate font-medium">{entry.epic.name}</span>
              </div>

              {/* Dynamic columns */}
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

      <Dialog open={createOpen} onOpenChange={(open) => { setCreateOpen(open); if (!open) resetCreateForm(); }}>
        <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
          <div className="flex h-[80vh] flex-col">
            {/* Header bar */}
            <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
              <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => setCreateOpen(false)}>
                <X className="h-4 w-4" />
              </Button>
              <span className="text-sm font-semibold">Create Epic</span>
              <Button className="ml-auto" size="sm" onClick={createEpic} disabled={!createForm.name.trim() || submitting}>
                {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
                {submitting ? 'Creating...' : 'Create Epic'}
              </Button>
            </div>

            {/* Two-column layout */}
            <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
              {/* Left — Title + Description */}
              <div className="min-h-0 overflow-y-auto px-8 py-5">
                <input
                  type="text"
                  autoFocus
                  aria-label="Epic title"
                  value={createForm.name}
                  onChange={(e) => setCreateForm((f) => ({ ...f, name: e.target.value }))}
                  className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                  placeholder="Epic title"
                />

                <div className="mt-4">
                  <TiptapEditor
                    content={createForm.description}
                    onChange={(html) => setCreateForm((f) => ({ ...f, description: html }))}
                    placeholder="Add a description..."
                    className="border-transparent shadow-none"
                  />
                </div>
              </div>

              {/* Right — Metadata sidebar */}
              <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
                <p className="mb-4 text-xs text-muted-foreground">
                  Epics are collections of stories that together represent a major initiative or feature.
                </p>

                <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                  {/* Team */}
                  <Users className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Team</span>
                  <Select value={createForm.teamId || '__none__'} onValueChange={(v) => setCreateForm((f) => ({ ...f, teamId: v === '__none__' ? '' : v }))}>
                    <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                      <SelectValue placeholder="None" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {teams.map((t) => (
                        <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>

                  {/* Owner — placeholder for now */}
                  <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Owner</span>
                  <span className="text-xs text-muted-foreground px-1.5 py-1">Nobody</span>

                  {/* State */}
                  <Hash className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">State</span>
                  <Select value={createForm.stateId || '__none__'} onValueChange={(v) => setCreateForm((f) => ({ ...f, stateId: v === '__none__' ? '' : v }))}>
                    <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                      <SelectValue placeholder="None" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {epicStates.map((s) => (
                        <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>

                  {/* Health */}
                  <Heart className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Health</span>
                  <Select value={createForm.health} onValueChange={(v) => setCreateForm((f) => ({ ...f, health: v as EpicHealth }))}>
                    <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {healthOptions.map((h) => (
                        <SelectItem key={h} value={h}>
                          <span className={healthConfig[h].color}>{healthConfig[h].label}</span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>

                  {/* Start Date */}
                  <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Start date</span>
                  <div className="flex items-center gap-1">
                    <DatePicker
                      value={createForm.startDate}
                      onChange={(v) => setCreateForm((f) => ({ ...f, startDate: v }))}
                      placeholder="Pick a date"
                      className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                    />
                    {createForm.startDate && (
                      <button type="button" onClick={() => setCreateForm((f) => ({ ...f, startDate: '' }))} className="shrink-0 text-muted-foreground hover:text-foreground cursor-pointer">
                        <X className="h-3 w-3" />
                      </button>
                    )}
                  </div>

                  {/* Target Date */}
                  <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Target date</span>
                  <div className="flex items-center gap-1">
                    <DatePicker
                      value={createForm.targetDate}
                      onChange={(v) => setCreateForm((f) => ({ ...f, targetDate: v }))}
                      placeholder="Pick a date"
                      className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                    />
                    {createForm.targetDate && (
                      <button type="button" onClick={() => setCreateForm((f) => ({ ...f, targetDate: '' }))} className="shrink-0 text-muted-foreground hover:text-foreground cursor-pointer">
                        <X className="h-3 w-3" />
                      </button>
                    )}
                  </div>
                </div>
              </aside>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Sheet open={Boolean(selectedEpic)} onOpenChange={(open) => !open && setSelectedEpic(null)}>
        <SheetContent side="right" className="w-[90vw] sm:max-w-[760px]">
          {selectedEpic ? (
            <div className="space-y-4">
              <SheetHeader>
                <SheetTitle>{selectedEpic.epic.name}</SheetTitle>
              </SheetHeader>
              <div className="space-y-2">
                <p className="text-sm text-muted-foreground">{selectedEpic.epic.description || 'No description.'}</p>
                <Progress value={selectedEpicProgress} />
                <p className="text-xs text-muted-foreground">
                  {selectedEpic.stats.done_story_count}/{selectedEpic.stats.story_count} stories done · {selectedEpic.stats.done_points}/{selectedEpic.stats.total_points} points
                </p>
              </div>

              <section className="space-y-2">
                <h3 className="text-sm font-semibold">Stories in this epic</h3>
                {selectedEpicStories.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No stories linked yet.</p>
                ) : (
                  <div className="space-y-1">
                    {selectedEpicStories.map((story) => (
                      <article key={story.id} className="rounded-md border border-border/70 px-3 py-2 text-sm">
                        <p className="font-medium">TP-{story.display_id} · {story.name}</p>
                        <p className="text-xs text-muted-foreground">{story.priority} priority · {story.story_type}</p>
                      </article>
                    ))}
                  </div>
                )}
              </section>
            </div>
          ) : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}
