import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  CalendarDays,
  Hash,
  Heart,
  Loader2,
  User,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { CreateStoryModal } from '@/components/pm/CreateStoryModal';
import { CreateDocumentDialog } from '@/components/docs/CreateDocumentDialog';
import { CreateSpaceDialog } from '@/components/docs/CreateSpaceDialog';
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useEpicStates } from '@/hooks/queries/useWorkflows';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { EpicHealth, ObjectiveType, ObjectiveState, WorkflowWithStates } from '@/lib/pmTypes';
import { buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';

const healthOptions: EpicHealth[] = ['on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

// ── Story wrapper ────────────────────────────────────────────────────

function GlobalCreateStory({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const [workflow, setWorkflow] = useState<WorkflowWithStates | null>(null);
  const initialTeamId = useGlobalCreateStore((s) => s.initialTeamId);

  useEffect(() => {
    // Try board store first (already loaded if on stories page)
    const boardWorkflow = usePMBoardStore.getState().workflow;
    if (boardWorkflow) {
      setWorkflow(boardWorkflow);
      return;
    }
    pmWorkflowService.list(workspaceId).then((res) => {
      if (res.data?.[0]) setWorkflow(res.data[0]);
    });
  }, [workspaceId]);

  if (!workflow) return null;

  return (
    <CreateStoryModal
      open
      onOpenChange={(open) => !open && onClose()}
      workspaceId={workspaceId}
      workflow={workflow}
      initialStateId={workflow.states[0]?.id ?? ''}
      initialTeamId={initialTeamId}
      onCreate={async (payload) => {
        const { error } = await pmStoryService.create(payload);
        if (error) throw new Error(error);
        // Refresh the board if it's loaded
        const boardWs = usePMBoardStore.getState().workspaceId;
        if (boardWs) usePMBoardStore.getState().refreshBoard();
        window.dispatchEvent(new CustomEvent('story-created'));
      }}
    />
  );
}

// ── Epic dialog ──────────────────────────────────────────────────────

function GlobalCreateEpic({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const { data: epicStates = [] } = useEpicStates(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const ownerOptions = buildAssignableMemberOptions(assignableMembers);

  const [form, setForm] = useState({
    name: '',
    description: '',
    stateId: '',
    health: 'on_track' as EpicHealth,
    teamId: '',
    ownerMemberId: '',
    startDate: '',
    targetDate: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = async () => {
    if (!form.name.trim() || submitting) return;
    setSubmitting(true);
    const { error: createError } = await pmEpicService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      epic_state_id: form.stateId || undefined,
      team_id: form.teamId || undefined,
      owner_member_id: form.ownerMemberId || undefined,
      health: form.health,
      planned_start_date: form.startDate || undefined,
      deadline: form.targetDate || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('epic-created'));
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
            <span className="text-sm font-semibold">Create Epic</span>
            <Button className="ml-auto" size="sm" onClick={create} disabled={!form.name.trim() || submitting}>
              {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : 'Create Epic'}
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <input
                type="text"
                autoFocus
                aria-label="Epic title"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                placeholder="Epic title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => setForm((f) => ({ ...f, description: html }))}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none"
                  teams={teams}
                  members={assignableMembers}
                />
              </div>
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Epics are collections of stories that together represent a major initiative or feature.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <Users className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Team</span>
                <Select value={form.teamId || '__none__'} onValueChange={(v) => setForm((f) => ({ ...f, teamId: v === '__none__' ? '' : v }))}>
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

                <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Owner</span>
                <Select value={form.ownerMemberId || '__none__'} onValueChange={(v) => setForm((f) => ({ ...f, ownerMemberId: v === '__none__' ? '' : v }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue placeholder="None" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">None</SelectItem>
                    {ownerOptions.map((o) => (
                      <SelectItem key={o.id} value={o.id}>{o.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Hash className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">State</span>
                <Select value={form.stateId || '__none__'} onValueChange={(v) => setForm((f) => ({ ...f, stateId: v === '__none__' ? '' : v }))}>
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

                <Heart className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Health</span>
                <Select value={form.health} onValueChange={(v) => setForm((f) => ({ ...f, health: v as EpicHealth }))}>
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

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Target date</span>
                <DatePicker
                  value={form.targetDate}
                  onChange={(v) => setForm((f) => ({ ...f, targetDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />
              </div>
            </aside>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Sprint dialog ─────────────────────────────────────────────────

function GlobalCreateSprint({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const { teams } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const storeTeamId = useGlobalCreateStore((s) => s.initialTeamId);

  const [form, setForm] = useState({
    name: '',
    description: '',
    startDate: '',
    endDate: '',
    teamId: storeTeamId ?? '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = async () => {
    if (!form.name.trim() || !form.startDate || !form.endDate || submitting) return;
    setSubmitting(true);
    const { error: createError } = await pmSprintService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      start_date: form.startDate,
      end_date: form.endDate,
      team_id: form.teamId || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('sprint-created'));
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
            <span className="text-sm font-semibold">Create Sprint</span>
            <Button className="ml-auto" size="sm" onClick={create} disabled={!form.name.trim() || !form.startDate || !form.endDate || submitting}>
              {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : 'Create Sprint'}
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <input
                type="text"
                autoFocus
                aria-label="Sprint title"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                placeholder="Sprint title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => setForm((f) => ({ ...f, description: html }))}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none"
                  teams={teams}
                  members={assignableMembers}
                />
              </div>
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Sprints are time-boxed periods for planning and tracking work.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <Users className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Team</span>
                <Select value={form.teamId || '__none__'} onValueChange={(v) => setForm((f) => ({ ...f, teamId: v === '__none__' ? '' : v }))}>
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

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">End date</span>
                <DatePicker
                  value={form.endDate}
                  onChange={(v) => setForm((f) => ({ ...f, endDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />
              </div>
            </aside>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Objective dialog ──────────────────────────────────────────────────

const objectiveStateOptions: { value: ObjectiveState; label: string }[] = (
  Object.entries(OBJECTIVE_STATE_CONFIG) as [ObjectiveState, typeof OBJECTIVE_STATE_CONFIG[ObjectiveState]][]
).map(([value, cfg]) => ({ value, label: cfg.label }));

function MultiSelectPopover({
  items,
  selected,
  onChange,
  placeholder,
}: {
  items: { id: string; name: string }[];
  selected: string[];
  onChange: (ids: string[]) => void;
  placeholder: string;
}) {
  const [open, setOpen] = useState(false);

  const toggle = (id: string) => {
    onChange(selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id]);
  };

  const selectedNames = items.filter((i) => selected.includes(i.id)).map((i) => i.name);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex min-w-0 items-center gap-1 rounded-md px-1.5 py-1 text-xs transition-colors hover:bg-accent cursor-pointer truncate"
        >
          {selectedNames.length > 0 ? selectedNames.join(', ') : <span className="text-muted-foreground">{placeholder}</span>}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-1" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs cursor-pointer transition-colors ${
                selected.includes(item.id) ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              }`}
              onClick={() => toggle(item.id)}
            >
              <span className={`flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border ${
                selected.includes(item.id) ? 'bg-primary border-primary text-primary-foreground' : 'border-muted-foreground/30'
              }`}>
                {selected.includes(item.id) && <span className="text-[9px]">✓</span>}
              </span>
              <span className="truncate">{item.name}</span>
            </button>
          ))}
          {items.length === 0 && (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">No options</p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function GlobalCreateObjective({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const ownerOptions = buildAssignableMemberOptions(assignableMembers);

  // Close immediately if the user lacks edit permission
  useEffect(() => {
    if (!canEdit) onClose();
  }, [canEdit, onClose]);

  const [form, setForm] = useState({
    name: '',
    description: '',
    objectiveType: 'tactical' as ObjectiveType,
    state: 'not_started' as ObjectiveState,
    teamIds: [] as string[],
    ownerMemberIds: [] as string[],
    startDate: '',
    targetDate: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = async () => {
    if (!form.name.trim() || submitting) return;
    setSubmitting(true);
    const { error: createError } = await pmObjectiveService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      objective_type: form.objectiveType,
      state: form.state,
      team_ids: form.teamIds.length > 0 ? form.teamIds : undefined,
      owner_member_ids: form.ownerMemberIds.length > 0 ? form.ownerMemberIds : undefined,
      planned_start_date: form.startDate || undefined,
      deadline: form.targetDate || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('objective-created'));
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
            <span className="text-sm font-semibold">Create Objective</span>
            <Button className="ml-auto" size="sm" onClick={create} disabled={!form.name.trim() || submitting}>
              {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : 'Create Objective'}
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_280px]">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <input
                type="text"
                autoFocus
                aria-label="Objective title"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                placeholder="Objective title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => setForm((f) => ({ ...f, description: html }))}
                  placeholder="Add a description..."
                  className="border-transparent shadow-none"
                  teams={teams}
                  members={assignableMembers}
                />
              </div>

              {/* Objective Type Selector — commented out for now */}
              {/* <div className="mt-6">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Objective Type</p>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    className={`flex flex-col gap-1 rounded-lg border p-3 text-left transition-colors cursor-pointer ${
                      form.objectiveType === 'tactical'
                        ? 'border-primary bg-primary/5'
                        : 'border-border/60 hover:border-border'
                    }`}
                    onClick={() => setForm((f) => ({ ...f, objectiveType: 'tactical' }))}
                  >
                    <div className="flex items-center gap-1.5">
                      <Target className="h-4 w-4 text-blue-500" />
                      <span className="text-sm font-medium">Tactical</span>
                    </div>
                    <span className="text-xs text-muted-foreground">Track linked Epics</span>
                  </button>
                  <button
                    type="button"
                    className={`flex flex-col gap-1 rounded-lg border p-3 text-left transition-colors cursor-pointer ${
                      form.objectiveType === 'strategic'
                        ? 'border-primary bg-primary/5'
                        : 'border-border/60 hover:border-border'
                    }`}
                    onClick={() => setForm((f) => ({ ...f, objectiveType: 'strategic' }))}
                  >
                    <div className="flex items-center gap-1.5">
                      <Crosshair className="h-4 w-4 text-violet-500" />
                      <span className="text-sm font-medium">Strategic</span>
                    </div>
                    <span className="text-xs text-muted-foreground">Group Key Results &amp; Epics</span>
                  </button>
                </div>
              </div> */}
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Objectives define high-level goals. Tactical objectives track linked Epics; Strategic objectives combine Key Results and Epics.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <Hash className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">State</span>
                <Select value={form.state} onValueChange={(v) => setForm((f) => ({ ...f, state: v as ObjectiveState }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {objectiveStateOptions.map((s) => (
                      <SelectItem key={s.value} value={s.value}>{s.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Users className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Teams</span>
                <MultiSelectPopover
                  items={teams.map((t) => ({ id: t.id, name: t.name }))}
                  selected={form.teamIds}
                  onChange={(ids) => setForm((f) => ({ ...f, teamIds: ids }))}
                  placeholder="Select teams"
                />

                <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Owners</span>
                <MultiSelectPopover
                  items={ownerOptions}
                  selected={form.ownerMemberIds}
                  onChange={(ids) => setForm((f) => ({ ...f, ownerMemberIds: ids }))}
                  placeholder="Select owners"
                />

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Target date</span>
                <DatePicker
                  value={form.targetDate}
                  onChange={(v) => setForm((f) => ({ ...f, targetDate: v }))}
                  placeholder="Pick a date"
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />
              </div>
            </aside>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Main export ──────────────────────────────────────────────────────

export function GlobalCreateModals({ workspaceId }: { workspaceId: string }) {
  const { activeModal, closeCreate, initialSpaceId, initialCollectionId } = useGlobalCreateStore();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const wsSlug = workspace?.slug ?? '';

  if (!activeModal) return null;

  return (
    <>
      {activeModal === 'story' && <GlobalCreateStory workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'epic' && <GlobalCreateEpic workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'sprint' && <GlobalCreateSprint workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'objective' && <GlobalCreateObjective workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'docs_document' && (
        <CreateDocumentDialog
          wsId={workspaceId}
          open
          onOpenChange={(open) => !open && closeCreate()}
          defaultSpaceId={initialSpaceId}
          defaultCollectionId={initialCollectionId}
          onCreated={(docId) => {
            closeCreate();
            navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug: wsSlug, docId } });
          }}
        />
      )}
      {activeModal === 'docs_space' && (
        <CreateSpaceDialog
          wsId={workspaceId}
          open
          onOpenChange={(open) => !open && closeCreate()}
        />
      )}
      {activeModal === 'docs_collection' && (
        <CreateCollectionDialog
          wsId={workspaceId}
          spaceId={initialSpaceId}
          open
          onOpenChange={(open) => !open && closeCreate()}
        />
      )}
    </>
  );
}
