import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { format, parseISO } from 'date-fns';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { pmIterationService } from '@/lib/services/pmIterationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { IterationWithStats } from '@/lib/pmTypes';

export function IterationsPage() {
  useTitle('Iterations');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [iterations, setIterations] = useState<IterationWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { findTeamName } = useWorkspaceTeams(workspaceId);
  const navigate = useNavigate();

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

  const openIteration = (iteration: IterationWithStats) => {
    if (!workspace) return;
    navigate({ to: '/w/$slug/pm/iterations/$iterationId', params: { slug: workspace.slug, iterationId: iteration.iteration.id } });
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

    </div>
  );
}
