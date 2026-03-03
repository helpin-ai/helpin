import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Textarea } from '@/components/ui/textarea';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { usePMWorkflowStore } from '@/stores/pmWorkflowStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { EpicWithStats, Story, EpicHealth } from '@/lib/pmTypes';

const healthOptions: EpicHealth[] = ['on_track', 'at_risk', 'off_track'];

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

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [stateId, setStateId] = useState('');
  const [health, setHealth] = useState<EpicHealth>('on_track');

  const workspaceId = workspace?.id;

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

  const createEpic = async () => {
    if (!workspaceId || !name.trim()) return;
    const payload = {
      workspace_id: workspaceId,
      name: name.trim(),
      description: description.trim() || undefined,
      epic_state_id: stateId || undefined,
      health,
    };
    const { error: createError } = await pmEpicService.create(payload);
    if (createError) {
      setError(createError);
      return;
    }
    setCreateOpen(false);
    setName('');
    setDescription('');
    setStateId('');
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
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" />
          Create Epic
        </Button>
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
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {epics.map((entry) => (
            <Card
              key={entry.epic.id}
              className="cursor-pointer transition hover:shadow-md"
              onClick={() => openEpicDetail(entry)}
            >
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{entry.epic.name}</CardTitle>
                <p className="text-xs text-muted-foreground">Health: {entry.epic.health}</p>
              </CardHeader>
              <CardContent className="space-y-2">
                <Progress value={completionPct(entry)} />
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>
                    {entry.stats.done_story_count}/{entry.stats.story_count} stories
                  </span>
                  <span>
                    {entry.stats.done_points}/{entry.stats.total_points} pts
                  </span>
                </div>
                {entry.epic.deadline ? (
                  <p className="text-xs text-muted-foreground">
                    Due {format(parseISO(entry.epic.deadline), 'MMM d, yyyy')}
                  </p>
                ) : null}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Epic</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label>Name</Label>
              <Input value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label>Description</Label>
              <Textarea value={description} onChange={(event) => setDescription(event.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>State</Label>
                <Select value={stateId || '__none__'} onValueChange={(value) => setStateId(value === '__none__' ? '' : value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select state" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">No state</SelectItem>
                    {epicStates.map((state) => (
                      <SelectItem key={state.id} value={state.id}>
                        {state.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Health</Label>
                <Select value={health} onValueChange={(value) => setHealth(value as EpicHealth)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {healthOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button onClick={createEpic} disabled={!name.trim()}>
                Create
              </Button>
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
