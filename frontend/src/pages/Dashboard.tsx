import { useEffect, useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useRewardQuarterStore } from '@/stores/quarterStore';
import { useAuthStore } from '@/stores/authStore';
import { rewardGoalsService } from '@/lib/services/rewardGoalsService';
import { rewardSprintsService } from '@/lib/services/rewardSprintsService';
import { settingsService } from '@/lib/services/settingsService';
import type { RewardCompanyGoal, RewardSprint, WorkspaceSettings } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { Target, Calendar, Users, TrendingUp } from 'lucide-react';
import dayjs from 'dayjs';

export default function Dashboard() {
  useTitle('Dashboard');
  const { user } = useAuthStore();
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useRewardQuarterStore();
  const [goals, setGoals] = useState<RewardCompanyGoal[]>([]);
  const [sprints, setSprints] = useState<RewardSprint[]>([]);
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      if (!currentWorkspace?.id) {
        setGoals([]);
        setSprints([]);
        setSettings(null);
        setLoading(false);
        return;
      }

      setLoading(true);
      try {
        // If no quarter exists yet, still load settings.
        if (!currentQuarter?.id) {
          const settingsRes = await settingsService.getAll(currentWorkspace.id);
          if (settingsRes.data) setSettings(settingsRes.data);
          setGoals([]);
          setSprints([]);
          return;
        }

        const [goalsRes, sprintsRes, settingsRes] = await Promise.all([
          rewardGoalsService.list(currentWorkspace.id, currentQuarter.id),
          rewardSprintsService.list(currentQuarter.id),
          settingsService.getAll(currentWorkspace.id),
        ]);
        if (goalsRes.data) setGoals(goalsRes.data);
        if (sprintsRes.data) setSprints(sprintsRes.data);
        if (settingsRes.data) setSettings(settingsRes.data);
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, [currentWorkspace?.id, currentQuarter?.id]);

  const activeSprints = sprints.filter(s => s.status === 'active');
  const completedSprints = sprints.filter(s => s.status === 'completed' || s.status === 'locked');
  const teamCount = settings?.teams?.length ?? 0;

  const quarterProgress = () => {
    if (!currentQuarter) return 0;
    const start = dayjs(currentQuarter.start_date);
    const end = dayjs(currentQuarter.end_date);
    const now = dayjs();
    if (now.isBefore(start)) return 0;
    if (now.isAfter(end)) return 100;
    const total = end.diff(start, 'day');
    const elapsed = now.diff(start, 'day');
    return Math.round((elapsed / total) * 100);
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[1, 2, 3, 4].map(i => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Welcome back, {user?.full_name || 'there'}</h1>
        <p className="text-muted-foreground">
          {currentWorkspace?.name} &mdash; {currentQuarter?.name || 'No quarter selected'}
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          title="Company Goals"
          value={goals.length}
          description={`${goals.filter(g => g.status === 'active').length} active`}
          icon={<Target className="h-5 w-5 text-primary" />}
        />
        <StatsCard
          title="Sprints"
          value={sprints.length}
          description={`${activeSprints.length} active, ${completedSprints.length} completed`}
          icon={<Calendar className="h-5 w-5 text-primary" />}
        />
        <StatsCard
          title="Teams"
          value={teamCount}
          description={`${settings?.people?.length ?? 0} people`}
          icon={<Users className="h-5 w-5 text-primary" />}
        />
        <StatsCard
          title="Quarter Progress"
          value={`${quarterProgress()}%`}
          description={currentQuarter ? `${dayjs(currentQuarter.start_date).format('MMM D')} - ${dayjs(currentQuarter.end_date).format('MMM D')}` : ''}
          icon={<TrendingUp className="h-5 w-5 text-primary" />}
        />
      </div>

      {/* Quarter status */}
      {currentQuarter && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-lg">Quarter Status</CardTitle>
              <Badge variant={currentQuarter.status === 'active' ? 'default' : 'secondary'}>
                {currentQuarter.status}
              </Badge>
            </div>
            <CardDescription>
              {currentQuarter.name} &middot; {dayjs(currentQuarter.start_date).format('MMM D, YYYY')} &ndash; {dayjs(currentQuarter.end_date).format('MMM D, YYYY')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Progress value={quarterProgress()} className="h-3" />
            <p className="text-sm text-muted-foreground mt-2">
              {quarterProgress()}% of the quarter has elapsed
            </p>
          </CardContent>
        </Card>
      )}

      {/* Goals overview */}
      {goals.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Goals Overview</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {goals.slice(0, 5).map(goal => (
                <div key={goal.id} className="flex items-center justify-between py-2 border-b last:border-0">
                  <div className="flex items-center gap-3">
                    <Badge variant={goal.goal_type === 'metric' ? 'default' : 'outline'} className="text-xs">
                      {goal.goal_type}
                    </Badge>
                    <span className="text-sm font-medium">{goal.title}</span>
                  </div>
                  <Badge variant={goal.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                    {goal.status}
                  </Badge>
                </div>
              ))}
              {goals.length > 5 && (
                <p className="text-xs text-muted-foreground text-center pt-2">
                  +{goals.length - 5} more goals
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function StatsCard({ title, value, description, icon }: { title: string; value: string | number; description: string; icon: React.ReactNode }) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
        {icon}
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        <p className="text-xs text-muted-foreground mt-1">{description}</p>
      </CardContent>
    </Card>
  );
}
