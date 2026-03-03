import { useEffect, useState } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useQuarterStore } from '@/stores/quarterStore';
import { goalsService } from '@/lib/services/goalsService';
import { settingsService } from '@/lib/services/settingsService';
import { sprintsService } from '@/lib/services/sprintsService';
import type { CompanyGoal, WorkspaceTeam, Sprint } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { Users } from 'lucide-react';

interface TeamGoalSummary {
  team: WorkspaceTeam;
  contributions: {
    goal: CompanyGoal;
    contributionPct: number;
    targetValue?: number;
    currentValue?: number;
  }[];
  totalContribution: number;
}

export default function TeamGoals() {
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useQuarterStore();
  const [goals, setGoals] = useState<CompanyGoal[]>([]);
  const [teams, setTeams] = useState<WorkspaceTeam[]>([]);
  const [sprints, setSprints] = useState<Sprint[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    const q = useQuarterStore.getState().currentQuarter;
    if (!ws?.id || !q?.id) return;
    setLoading(true);
    Promise.all([
      goalsService.list(ws.id, q.id),
      settingsService.getAll(ws.id),
      sprintsService.list(q.id),
    ]).then(([goalsRes, settingsRes, sprintsRes]) => {
      if (goalsRes.data) setGoals(goalsRes.data);
      if (settingsRes.data) setTeams(settingsRes.data.teams);
      if (sprintsRes.data) setSprints(sprintsRes.data);
      setLoading(false);
    });
  }, [currentWorkspace?.id, currentQuarter?.id]);

  const teamSummaries: TeamGoalSummary[] = teams.map(team => {
    const contributions = goals
      .filter(g => g.team_contributions?.some(tc => tc.team_id === team.id))
      .map(g => {
        const tc = g.team_contributions!.find(tc => tc.team_id === team.id)!;
        return {
          goal: g,
          contributionPct: tc.contribution_pct,
          targetValue: tc.target_value,
          currentValue: tc.current_value,
        };
      });
    const totalContribution = contributions.reduce((sum, c) => sum + c.contributionPct, 0);
    return { team, contributions, totalContribution };
  });

  const completedSprintCount = sprints.filter(s => s.status === 'completed' || s.status === 'locked').length;

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="space-y-4">
          {[1, 2, 3].map(i => <Skeleton key={i} className="h-64" />)}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Team Goals</h1>
        <p className="text-muted-foreground">Goal contributions by team for {currentQuarter?.name}</p>
      </div>

      {/* Sprint progress summary */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Sprint Progress</CardTitle>
          <CardDescription>
            {completedSprintCount} of {sprints.length} sprints completed
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Progress value={sprints.length > 0 ? (completedSprintCount / sprints.length) * 100 : 0} className="h-2" />
        </CardContent>
      </Card>

      {teamSummaries.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">No teams configured yet. Add teams in Settings.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {teamSummaries.map(({ team, contributions }) => (
            <Card key={team.id}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="h-9 w-9 rounded-lg bg-primary/10 flex items-center justify-center">
                      <Users className="h-4 w-4 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-base">{team.name}</CardTitle>
                      {team.description && (
                        <CardDescription className="text-xs">{team.description}</CardDescription>
                      )}
                    </div>
                  </div>
                  <Badge variant="outline">
                    {contributions.length} goal{contributions.length !== 1 ? 's' : ''}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent>
                {contributions.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No goals assigned to this team yet.</p>
                ) : (
                  <div className="space-y-3">
                    {contributions.map(({ goal, contributionPct, targetValue, currentValue }) => {
                      const progress = targetValue && targetValue > 0
                        ? Math.min(100, Math.round(((currentValue ?? 0) / targetValue) * 100))
                        : 0;
                      return (
                        <div key={goal.id} className="border rounded-md p-3 space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="text-sm font-medium">{goal.title}</span>
                            <Badge variant="secondary" className="text-xs">{contributionPct}% contribution</Badge>
                          </div>
                          {goal.goal_type === 'metric' && targetValue != null && (
                            <div className="space-y-1">
                              <div className="flex justify-between text-xs text-muted-foreground">
                                <span>{currentValue ?? 0} / {targetValue}{goal.unit ? ` ${goal.unit}` : ''}</span>
                                <span>{progress}%</span>
                              </div>
                              <Progress value={progress} className="h-1.5" />
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
