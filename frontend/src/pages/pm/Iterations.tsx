import { useEffect, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { IterationWithStats, Story } from '@/lib/pmTypes';

export function IterationsPage() {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [iterations, setIterations] = useState<IterationWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { findTeamName } = useWorkspaceTeams(workspaceId);

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

  // Refresh when iteration is created via global modal
  useEffect(() => {
    const handler = () => { loadData(); };
    window.addEventListener('iteration-created', handler);
    return () => window.removeEventListener('iteration-created', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId]);

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
            No iterations yet. Use the Create button in the header to add one.
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
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span>Status: {entry.iteration.status}</span>
                  {findTeamName(entry.iteration.team_id) && (
                    <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                      {findTeamName(entry.iteration.team_id)}
                    </span>
                  )}
                </div>
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
