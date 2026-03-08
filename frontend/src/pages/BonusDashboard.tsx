import { useEffect, useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSession, useSessionRole, useQuarters } from '@/hooks/queries';
import { rewardBonusService } from '@/lib/services/rewardBonusService';
import { settingsService } from '@/lib/services/settingsService';
import type { RewardBonusCalculation, RewardFinanceSettings, RewardQuarter, WorkspacePerson, WorkspaceTeam } from '@/lib/types';
import { formatCurrencyUSD, formatPercentage } from '@/lib/formatters';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { DollarSign, Lock, Unlock } from 'lucide-react';
import { toast } from 'sonner';

export default function BonusDashboard() {
  useTitle('Bonus Dashboard');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: membership } = useSession(wsId);
  const { isAdmin } = useSessionRole(membership);
  const { data: quarters } = useQuarters(wsId);
  const [currentQuarter, setCurrentQuarter] = useState<RewardQuarter | null>(null);
  const [calculations, setCalculations] = useState<RewardBonusCalculation[]>([]);
  const [finance, setFinance] = useState<RewardFinanceSettings | null>(null);
  const [people, setPeople] = useState<WorkspacePerson[]>([]);
  const [, setTeams] = useState<WorkspaceTeam[]>([]);
  const [loading, setLoading] = useState(true);

  // Initialize currentQuarter from query data
  useEffect(() => {
    if (quarters?.length && !currentQuarter) {
      setCurrentQuarter(quarters[0]);
    }
  }, [quarters, currentQuarter]);

  const load = async () => {
    const ws = currentWorkspace;
    const q = currentQuarter;
    if (!ws?.id || !q?.id) {
      setCalculations([]);
      setFinance(null);
      setPeople([]);
      setTeams([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const [calcRes, finRes, settingsRes] = await Promise.all([
        rewardBonusService.getCalculations(ws.id, q.id),
        rewardBonusService.getFinance(ws.id, q.id),
        settingsService.getAll(ws.id),
      ]);
      if (calcRes.data) setCalculations(calcRes.data);
      if (finRes.data) setFinance(finRes.data);
      if (settingsRes.data) {
        setPeople(settingsRes.data.people);
        setTeams(settingsRes.data.teams);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id, currentQuarter?.id]);

  const handleLock = async () => {
    if (!currentWorkspace?.id || !currentQuarter?.id) return;
    const { error } = await rewardBonusService.lock(currentWorkspace.id, currentQuarter.id);
    if (error) toast.error(error);
    else { toast.success('Bonuses locked'); load(); }
  };

  const handleUnlock = async () => {
    if (!currentWorkspace?.id || !currentQuarter?.id) return;
    const { error } = await rewardBonusService.unlock(currentWorkspace.id, currentQuarter.id);
    if (error) toast.error(error);
    else { toast.success('Bonuses unlocked'); load(); }
  };

  const handleFinanceUpdate = async (field: keyof RewardFinanceSettings, value: string) => {
    if (!currentWorkspace?.id || !currentQuarter?.id || !finance) return;
    const updated = { ...finance, [field]: Number(value) };
    const { error } = await rewardBonusService.upsertFinance({
      ...updated,
      workspace_id: currentWorkspace.id,
      quarter_id: currentQuarter.id,
    });
    if (error) toast.error(error);
    else setFinance(updated);
  };

  const getPersonName = (id: string) => people.find(p => p.id === id)?.name ?? 'Unknown';
  const getPersonTeam = (id: string) => {
    const person = people.find(p => p.id === id);
    // Find team via memberships - simplified: match by checking teams
    return person?.job_role ?? '';
  };

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      case 'C': return 'destructive';
      default: return 'secondary';
    }
  };

  const isLocked = finance?.locked_at != null;

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 lg:grid-cols-3">
          <Skeleton className="h-64 lg:col-span-2" />
          <Skeleton className="h-64" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Bonus Dashboard</h1>
          <p className="text-muted-foreground">{currentQuarter?.name}</p>
        </div>
        {isAdmin && (
          isLocked ? (
            <Button variant="outline" onClick={handleUnlock}>
              <Unlock className="h-4 w-4 mr-2" />
              Unlock Bonuses
            </Button>
          ) : (
            <Button onClick={handleLock}>
              <Lock className="h-4 w-4 mr-2" />
              Lock Bonuses
            </Button>
          )
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Employee table */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">Employee Bonus Calculations</CardTitle>
              <Badge variant="outline" className="text-xs font-normal">{calculations.length}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            {calculations.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-8">No bonus calculations available yet.</p>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Job Role</TableHead>
                      <TableHead className="text-right">TQI</TableHead>
                      <TableHead className="text-right">IQI</TableHead>
                      <TableHead className="text-right">Score</TableHead>
                      <TableHead>Tier</TableHead>
                      <TableHead className="text-right">Bonus</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {calculations.map(calc => (
                      <TableRow key={calc.id}>
                        <TableCell className="font-medium">{getPersonName(calc.employee_id)}</TableCell>
                        <TableCell className="text-muted-foreground">{getPersonTeam(calc.employee_id)}</TableCell>
                        <TableCell className="text-right">{formatPercentage(calc.team_tqi)}</TableCell>
                        <TableCell className="text-right">{formatPercentage(calc.individual_iqi)}</TableCell>
                        <TableCell className="text-right font-medium">{formatPercentage(calc.final_score)}</TableCell>
                        <TableCell>
                          <Badge variant={tierVariant(calc.bonus_tier)}>Tier {calc.bonus_tier}</Badge>
                        </TableCell>
                        <TableCell className="text-right font-medium">{formatCurrencyUSD(calc.final_amount)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Finance settings panel */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <DollarSign className="h-4 w-4" />
              Finance Settings
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FinanceField
              label="MRR Start"
              value={finance?.mrr_start ?? 0}
              disabled={isLocked || !isAdmin}
              onChange={v => handleFinanceUpdate('mrr_start', v)}
              prefix="$"
            />
            <FinanceField
              label="MRR End"
              value={finance?.mrr_end ?? 0}
              disabled={isLocked || !isAdmin}
              onChange={v => handleFinanceUpdate('mrr_end', v)}
              prefix="$"
            />
            <FinanceField
              label="Pool %"
              value={finance?.bonus_pool_percentage ?? 0}
              disabled={isLocked || !isAdmin}
              onChange={v => handleFinanceUpdate('bonus_pool_percentage', v)}
              suffix="%"
            />
            <div className="pt-2 border-t space-y-3">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Total Pool</span>
                <span className="font-medium">{formatCurrencyUSD(finance?.total_pool ?? 0)}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Total Paid</span>
                <span className="font-medium">{formatCurrencyUSD(finance?.total_paid ?? 0)}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Utilization</span>
                <span className="font-medium">{formatPercentage(finance?.pool_utilization ?? 0)}</span>
              </div>
            </div>
            {isLocked && (
              <Badge variant="destructive" className="w-full justify-center">Locked</Badge>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function FinanceField({
  label,
  value,
  disabled,
  onChange,
  prefix,
  suffix,
}: {
  label: string;
  value: number;
  disabled: boolean;
  onChange: (v: string) => void;
  prefix?: string;
  suffix?: string;
}) {
  const [localVal, setLocalVal] = useState(String(value));

  useEffect(() => {
    setLocalVal(String(value));
  }, [value]);

  return (
    <div className="space-y-1">
      <Label className="text-xs">{label}</Label>
      <div className="flex items-center gap-1">
        {prefix && <span className="text-sm text-muted-foreground">{prefix}</span>}
        <Input
          type="number"
          value={localVal}
          disabled={disabled}
          onChange={e => setLocalVal(e.target.value)}
          onBlur={() => onChange(localVal)}
          className="h-8"
        />
        {suffix && <span className="text-sm text-muted-foreground">{suffix}</span>}
      </div>
    </div>
  );
}
