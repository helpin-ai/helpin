import { useEffect, useState } from 'react';
import {
  CalendarDays,
  Hash,
  Heart,
  Loader2,
  Users,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { CreateStoryModal } from '@/components/pm/CreateStoryModal';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { usePMWorkflowStore } from '@/stores/pmWorkflowStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { EpicHealth, WorkflowWithStates } from '@/lib/pmTypes';

const healthOptions: EpicHealth[] = ['on_track', 'at_risk', 'off_track'];
const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

// ── Story wrapper ────────────────────────────────────────────────────

function GlobalCreateStory({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const [workflow, setWorkflow] = useState<WorkflowWithStates | null>(null);

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
  const epicStates = usePMWorkflowStore((s) => s.epicStates);
  const loadEpicStates = usePMWorkflowStore((s) => s.loadEpicStates);
  const { teams } = useWorkspaceTeams(workspaceId);

  const [form, setForm] = useState({
    name: '',
    description: '',
    stateId: '',
    health: 'on_track' as EpicHealth,
    teamId: '',
    startDate: '',
    targetDate: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadEpicStates(workspaceId);
  }, [workspaceId, loadEpicStates]);

  const create = async () => {
    if (!form.name.trim() || submitting) return;
    setSubmitting(true);
    const { error: createError } = await pmEpicService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      epic_state_id: form.stateId || undefined,
      team_id: form.teamId || undefined,
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

// ── Iteration dialog ─────────────────────────────────────────────────

function GlobalCreateIteration({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const { teams } = useWorkspaceTeams(workspaceId);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [teamId, setTeamId] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = async () => {
    if (!name.trim() || !startDate || !endDate || submitting) return;
    setSubmitting(true);
    const { error: createError } = await pmIterationService.create({
      workspace_id: workspaceId,
      name: name.trim(),
      description: description.trim() || undefined,
      start_date: startDate,
      end_date: endDate,
      team_id: teamId || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('iteration-created'));
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Iteration</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Description</Label>
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Start date</Label>
              <DatePicker value={startDate} onChange={setStartDate} placeholder="Start date" className="w-full" />
            </div>
            <div className="space-y-1.5">
              <Label>End date</Label>
              <DatePicker value={endDate} onChange={setEndDate} placeholder="End date" className="w-full" />
            </div>
          </div>
          {teams.length > 0 && (
            <div className="space-y-1.5">
              <Label>Team</Label>
              <Select value={teamId || '__none__'} onValueChange={(v) => setTeamId(v === '__none__' ? '' : v)}>
                <SelectTrigger>
                  <SelectValue placeholder="Select team" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No team</SelectItem>
                  {teams.map((t) => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={onClose}>Cancel</Button>
            <Button onClick={create} disabled={!name.trim() || !startDate || !endDate || submitting}>
              {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              Create
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Main export ──────────────────────────────────────────────────────

export function GlobalCreateModals({ workspaceId }: { workspaceId: string }) {
  const { activeModal, closeCreate } = useGlobalCreateStore();

  if (!activeModal) return null;

  return (
    <>
      {activeModal === 'story' && <GlobalCreateStory workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'epic' && <GlobalCreateEpic workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'iteration' && <GlobalCreateIteration workspaceId={workspaceId} onClose={closeCreate} />}
    </>
  );
}
