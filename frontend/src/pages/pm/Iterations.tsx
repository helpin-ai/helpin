import { useEffect, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Textarea } from '@/components/ui/textarea';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { IterationWithStats, Story } from '@/lib/pmTypes';

export function IterationsPage() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [iterations, setIterations] = useState<IterationWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  const [selectedIteration, setSelectedIteration] = useState<IterationWithStats | null>(null);
  const [selectedStories, setSelectedStories] = useState<Story[]>([]);

  const loadData = async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const res = await pmIterationService.list(workspaceId, { archived: false });
    if (res.error || !res.data) {
      setError(res.error ?? 'Failed to load iterations');
      setLoading(false);
      return;
    }
    setIterations(res.data);
    setLoading(false);
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId]);

  const createIteration = async () => {
    if (!workspaceId || !name.trim() || !startDate || !endDate) return;
    const res = await pmIterationService.create({
      workspace_id: workspaceId,
      name: name.trim(),
      description: description.trim() || undefined,
      start_date: startDate,
      end_date: endDate,
    });
    if (res.error) {
      setError(res.error);
      return;
    }
    setCreateOpen(false);
    setName('');
    setDescription('');
    setStartDate('');
    setEndDate('');
    await loadData();
  };

  const openIteration = async (iteration: IterationWithStats) => {
    setSelectedIteration(iteration);
    if (!workspaceId) return;
    const storiesRes = await pmIterationService.listStories(workspaceId, iteration.iteration.id);
    setSelectedStories(storiesRes.data ?? []);
  };

  const pct = (iteration: IterationWithStats) => {
    if (iteration.stats.story_count === 0) return 0;
    return Math.round((iteration.stats.done_story_count / iteration.stats.story_count) * 100);
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Iterations</h2>
          <p className="text-sm text-muted-foreground">Plan cycles and monitor story completion.</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" />
          Create Iteration
        </Button>
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading iterations...</p>
      ) : iterations.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No iterations yet. Create one to group upcoming work.
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {iterations.map((entry) => (
            <Card
              key={entry.iteration.id}
              className="cursor-pointer transition hover:shadow-md"
              onClick={() => openIteration(entry)}
            >
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{entry.iteration.name}</CardTitle>
                <p className="text-xs text-muted-foreground">Status: {entry.iteration.status}</p>
              </CardHeader>
              <CardContent className="space-y-2">
                <Progress value={pct(entry)} />
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{entry.stats.done_story_count}/{entry.stats.story_count} stories</span>
                  <span>{entry.stats.done_points}/{entry.stats.total_points} pts</span>
                </div>
                <p className="text-xs text-muted-foreground">
                  {format(parseISO(entry.iteration.start_date), 'MMM d')} - {format(parseISO(entry.iteration.end_date), 'MMM d, yyyy')}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Iteration</DialogTitle>
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
                <Label>Start date</Label>
                <Input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
              </div>
              <div className="space-y-1.5">
                <Label>End date</Label>
                <Input type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} />
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button onClick={createIteration} disabled={!name.trim() || !startDate || !endDate}>
                Create
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Sheet open={Boolean(selectedIteration)} onOpenChange={(open) => !open && setSelectedIteration(null)}>
        <SheetContent side="right" className="w-[90vw] sm:max-w-[720px]">
          {selectedIteration ? (
            <div className="space-y-4">
              <SheetHeader>
                <SheetTitle>{selectedIteration.iteration.name}</SheetTitle>
              </SheetHeader>
              <p className="text-sm text-muted-foreground">
                {selectedIteration.iteration.description || 'No description.'}
              </p>
              <Progress value={pct(selectedIteration)} />
              <p className="text-xs text-muted-foreground">
                {selectedIteration.stats.done_story_count}/{selectedIteration.stats.story_count} stories done · {selectedIteration.stats.done_points}/{selectedIteration.stats.total_points} points
              </p>

              <section className="space-y-2">
                <h3 className="text-sm font-semibold">Stories in this iteration</h3>
                {selectedStories.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No stories linked yet.</p>
                ) : (
                  <div className="space-y-1">
                    {selectedStories.map((story) => (
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
