import { useEffect, useState, type FormEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useQuarterStore } from '@/stores/quarterStore';
import { useSessionStore } from '@/stores/sessionStore';
import { goalsService } from '@/lib/services/goalsService';
import { settingsService } from '@/lib/services/settingsService';
import type { CompanyGoal, WorkspaceTeam } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Plus, ChevronDown, ChevronRight } from 'lucide-react';
import { toast } from 'sonner';

export default function CompanyGoals() {
  useTitle('Company Goals');
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useQuarterStore();
  const { canEdit } = useSessionStore();
  const [goals, setGoals] = useState<CompanyGoal[]>([]);
  const [teams, setTeams] = useState<WorkspaceTeam[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);

  const load = async () => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    const q = useQuarterStore.getState().currentQuarter;
    if (!ws?.id || !q?.id) {
      setGoals([]);
      setTeams([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const [goalsRes, settingsRes] = await Promise.all([
        goalsService.list(ws.id, q.id),
        settingsService.getAll(ws.id),
      ]);
      if (goalsRes.data) setGoals(goalsRes.data);
      if (settingsRes.data) setTeams(settingsRes.data.teams);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id, currentQuarter?.id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-48" />)}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Company Goals</h1>
          <p className="text-muted-foreground">{currentQuarter?.name} &middot; {goals.length} goals</p>
        </div>
        {canEdit() && currentWorkspace?.id && currentQuarter?.id && (
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                Create Goal
              </Button>
            </DialogTrigger>
            <CreateGoalDialog
              workspaceId={currentWorkspace.id}
              quarterId={currentQuarter.id}
              teams={teams}
              onCreated={() => { setDialogOpen(false); load(); }}
              onCancel={() => setDialogOpen(false)}
            />
          </Dialog>
        )}
      </div>

      {!currentQuarter?.id ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">No quarter selected yet. Create or activate a quarter first.</p>
          </CardContent>
        </Card>
      ) : goals.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">No goals yet for this quarter.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {goals.map(goal => (
            <GoalCard key={goal.id} goal={goal} teams={teams} />
          ))}
        </div>
      )}
    </div>
  );
}

function GoalCard({ goal, teams }: { goal: CompanyGoal; teams: WorkspaceTeam[] }) {
  const [expanded, setExpanded] = useState(false);

  const progress = goal.goal_type === 'metric' && goal.target && goal.target > 0
    ? Math.min(100, Math.round(((goal.current_value ?? 0) / goal.target) * 100))
    : 0;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="space-y-1">
            <CardTitle className="text-base">{goal.title}</CardTitle>
            {goal.description && (
              <CardDescription className="text-xs line-clamp-2">{goal.description}</CardDescription>
            )}
          </div>
          <div className="flex gap-2">
            <Badge variant={goal.goal_type === 'metric' ? 'default' : 'outline'} className="text-xs">
              {goal.goal_type}
            </Badge>
            <Badge variant={goal.status === 'active' ? 'default' : 'secondary'} className="text-xs">
              {goal.status}
            </Badge>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {goal.goal_type === 'metric' && (
          <>
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">
                {goal.current_value ?? 0}{goal.unit ? ` ${goal.unit}` : ''} / {goal.target ?? 0}{goal.unit ? ` ${goal.unit}` : ''}
              </span>
              <span className="font-medium">{progress}%</span>
            </div>
            <Progress value={progress} className="h-2" />
            {goal.baseline != null && (
              <p className="text-xs text-muted-foreground">Baseline: {goal.baseline}{goal.unit ? ` ${goal.unit}` : ''}</p>
            )}
          </>
        )}

        {goal.team_contributions && goal.team_contributions.length > 0 && (
          <div>
            <button
              onClick={() => setExpanded(!expanded)}
              className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
              {goal.team_contributions.length} team contribution{goal.team_contributions.length !== 1 ? 's' : ''}
            </button>
            {expanded && (
              <div className="mt-2 space-y-2">
                {goal.team_contributions.map(tc => {
                  const teamName = tc.team_name || teams.find(t => t.id === tc.team_id)?.name || 'Unknown';
                  return (
                    <div key={tc.id} className="flex items-center justify-between text-sm border rounded-md px-3 py-2">
                      <span>{teamName}</span>
                      <Badge variant="outline" className="text-xs">{tc.contribution_pct}%</Badge>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function CreateGoalDialog({
  workspaceId,
  quarterId,
  teams: _teams,
  onCreated,
  onCancel,
}: {
  workspaceId: string;
  quarterId: string;
  teams: WorkspaceTeam[];
  onCreated: () => void;
  onCancel: () => void;
}) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [goalType, setGoalType] = useState<'metric' | 'milestone'>('metric');
  const [baseline, setBaseline] = useState('');
  const [target, setTarget] = useState('');
  const [unit, setUnit] = useState('');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const { error } = await goalsService.create({
      workspace_id: workspaceId,
      quarter_id: quarterId,
      title,
      description: description || undefined,
      goal_type: goalType,
      baseline: baseline ? Number(baseline) : undefined,
      target: target ? Number(target) : undefined,
      unit: unit || undefined,
    });
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Goal created');
      onCreated();
    }
  };

  return (
    <DialogContent className="sm:max-w-lg">
      <form onSubmit={handleSubmit}>
        <DialogHeader>
          <DialogTitle>Create Company Goal</DialogTitle>
          <DialogDescription>Define a new goal for this quarter.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="goal-title">Title</Label>
            <Input id="goal-title" placeholder="Increase MRR to $500k" value={title} onChange={e => setTitle(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="goal-desc">Description</Label>
            <Textarea id="goal-desc" placeholder="Describe the goal..." value={description} onChange={e => setDescription(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>Goal Type</Label>
            <Select value={goalType} onValueChange={v => setGoalType(v as 'metric' | 'milestone')}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="metric">Metric (measurable)</SelectItem>
                <SelectItem value="milestone">Milestone (binary)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {goalType === 'metric' && (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="goal-baseline">Baseline</Label>
                  <Input id="goal-baseline" type="number" placeholder="0" value={baseline} onChange={e => setBaseline(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="goal-target">Target</Label>
                  <Input id="goal-target" type="number" placeholder="100" value={target} onChange={e => setTarget(e.target.value)} />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="goal-unit">Unit</Label>
                <Input id="goal-unit" placeholder="e.g. USD, %, users" value={unit} onChange={e => setUnit(e.target.value)} />
              </div>
            </>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>Cancel</Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Creating...' : 'Create Goal'}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}
