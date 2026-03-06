import { useEffect, useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useRewardQuarterStore } from '@/stores/quarterStore';
import { useAuthStore } from '@/stores/authStore';
import { rewardBonusService } from '@/lib/services/rewardBonusService';
import { rewardSprintsService } from '@/lib/services/rewardSprintsService';
import { settingsService } from '@/lib/services/settingsService';
import type { RewardSprint, RewardBonusCalculation, RewardIndividualCheck, WorkspaceSettings } from '@/lib/types';
import { formatCurrencyUSD, formatPercentage } from '@/lib/formatters';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { User, Trophy, TrendingUp } from 'lucide-react';
import dayjs from 'dayjs';

export default function MyQuarter() {
  useTitle('My Quarter');
  const { user } = useAuthStore();
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useRewardQuarterStore();
  const [sprints, setSprints] = useState<RewardSprint[]>([]);
  const [myCalc, setMyCalc] = useState<RewardBonusCalculation | null>(null);
  const [checks, setChecks] = useState<Map<string, RewardIndividualCheck[]>>(new Map());
  const [, setWsSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      const ws = useWorkspaceStore.getState().currentWorkspace;
      const q = useRewardQuarterStore.getState().currentQuarter;
      const u = useAuthStore.getState().user;
      if (!ws?.id || !q?.id || !u?.id) {
        setSprints([]);
        setMyCalc(null);
        setChecks(new Map());
        setLoading(false);
        return;
      }

      setLoading(true);
      try {
        const [sprintsRes, calcRes, settingsRes] = await Promise.all([
          rewardSprintsService.list(q.id),
          rewardBonusService.getCalculations(ws.id, q.id),
          settingsService.getAll(ws.id),
        ]);

        const sprintList = sprintsRes.data ?? [];
        setSprints(sprintList.sort((a, b) => a.sprint_number - b.sprint_number));

        if (settingsRes.data) setWsSettings(settingsRes.data);

        // Find the person record that matches the current user
        const personRecord = settingsRes.data?.people.find(p => p.email === u.email);

        if (calcRes.data && personRecord) {
          const mine = calcRes.data.find(c => c.employee_id === personRecord.id);
          if (mine) setMyCalc(mine);
        }

        // Load individual checks for each sprint
        const checksMap = new Map<string, RewardIndividualCheck[]>();
        for (const s of sprintList) {
          const res = await rewardSprintsService.getIndividualChecks(s.id, ws.id);
          if (res.data) {
            const myChecks = personRecord
              ? res.data.filter(c => c.employee_id === personRecord.id)
              : [];
            checksMap.set(s.id, myChecks);
          }
        }
        setChecks(checksMap);
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, [currentWorkspace?.id, currentQuarter?.id, user?.id, user?.email]);

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      default: return 'destructive';
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 sm:grid-cols-3">
          {[1, 2, 3].map(i => <Skeleton key={i} className="h-32" />)}
        </div>
        <Skeleton className="h-64" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <User className="h-5 w-5 text-muted-foreground" />
          My Quarter
        </h1>
        <p className="text-muted-foreground">{currentQuarter?.name} &middot; {user?.full_name}</p>
      </div>

      {/* Summary cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <TrendingUp className="h-4 w-4" />
              Final Score
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {myCalc ? formatPercentage(myCalc.final_score) : '--'}
            </div>
            {myCalc && (
              <p className="text-xs text-muted-foreground mt-1">
                TQI: {formatPercentage(myCalc.team_tqi)} | IQI: {formatPercentage(myCalc.individual_iqi)}
              </p>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Trophy className="h-4 w-4" />
              Tier
            </CardTitle>
          </CardHeader>
          <CardContent>
            {myCalc ? (
              <Badge variant={tierVariant(myCalc.bonus_tier)} className="text-lg px-3 py-1">
                Tier {myCalc.bonus_tier}
              </Badge>
            ) : (
              <span className="text-2xl font-bold">--</span>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Estimated Bonus</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {myCalc ? formatCurrencyUSD(myCalc.final_amount) : '--'}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Sprint-by-sprint scores */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Sprint Scores</CardTitle>
        </CardHeader>
        <CardContent>
          {sprints.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-6">No sprints available.</p>
          ) : (
            <div className="space-y-3">
              {sprints.map(sprint => {
                const sprintChecks = checks.get(sprint.id) ?? [];
                const passed = sprintChecks.filter(c => c.answer).length;
                const total = sprintChecks.length;
                const pct = total > 0 ? Math.round((passed / total) * 100) : 0;

                return (
                  <div key={sprint.id} className="border rounded-md p-3 space-y-2">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium">Sprint {sprint.sprint_number}</p>
                        <p className="text-xs text-muted-foreground">
                          {dayjs(sprint.start_date).format('MMM D')} &ndash; {dayjs(sprint.end_date).format('MMM D')}
                        </p>
                      </div>
                      <div className="text-right">
                        <Badge variant={sprint.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                          {sprint.status}
                        </Badge>
                        {total > 0 && (
                          <p className="text-xs text-muted-foreground mt-1">{passed}/{total} criteria met</p>
                        )}
                      </div>
                    </div>
                    {total > 0 && <Progress value={pct} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
