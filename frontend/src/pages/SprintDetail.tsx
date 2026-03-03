import { useEffect, useState, type FormEvent } from 'react';
import { getRouteApi } from '@tanstack/react-router';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSessionStore } from '@/stores/sessionStore';
import { sprintsService } from '@/lib/services/sprintsService';
import { settingsService } from '@/lib/services/settingsService';
import { goalsService } from '@/lib/services/goalsService';
import { useQuarterStore } from '@/stores/quarterStore';
import type { Sprint, SprintGoal, WorkspaceSettings, IndividualCheck } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Calendar, Plus } from 'lucide-react';
import { toast } from 'sonner';
import dayjs from 'dayjs';

export default function SprintDetail() {
  const routeApi = getRouteApi('/_authenticated/w/$slug/sprints/$sprintId');
  const { sprintId } = routeApi.useParams();
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useQuarterStore();
  const { canEdit } = useSessionStore();
  const [sprint, setSprint] = useState<Sprint | null>(null);
  const [wsSettings, setWsSettings] = useState<WorkspaceSettings | null>(null);
  const [sprintGoals] = useState<SprintGoal[]>([]);
  const [checks, setChecks] = useState<IndividualCheck[]>([]);
  const [loading, setLoading] = useState(true);
  const [addGoalOpen, setAddGoalOpen] = useState(false);

  const load = async () => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    if (!sprintId || !ws?.id) return;
    setLoading(true);
    const [sprintRes, settingsRes, checksRes] = await Promise.all([
      sprintsService.get(sprintId),
      settingsService.getAll(ws.id),
      sprintsService.getIndividualChecks(sprintId, ws.id),
    ]);
    if (sprintRes.data) setSprint(sprintRes.data);
    if (settingsRes.data) setWsSettings(settingsRes.data);
    if (checksRes.data) setChecks(checksRes.data);
    setLoading(false);
  };

  useEffect(() => { load(); }, [sprintId, currentWorkspace?.id]);

  // Load sprint goals from the goals service (sprint-level)
  useEffect(() => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    const q = useQuarterStore.getState().currentQuarter;
    if (!ws?.id || !q?.id) return;
    goalsService.list(ws.id, q.id).then(_res => {
      // Sprint goals come from the sprint's associated data; for now we keep them separate
    });
  }, [currentWorkspace?.id, currentQuarter?.id]);

  const handleCheckToggle = async (employeeId: string, criteriaId: string, current: boolean) => {
    if (!sprintId || !currentWorkspace?.id) return;
    const { error } = await sprintsService.upsertIndividualCheck({
      sprint_id: sprintId,
      workspace_id: currentWorkspace.id,
      employee_id: employeeId,
      scored_by: '',
      criteria_id: criteriaId,
      answer: !current,
    });
    if (error) {
      toast.error(error);
    } else {
      setChecks(prev => {
        const existing = prev.find(c => c.employee_id === employeeId && c.criteria_id === criteriaId);
        if (existing) {
          return prev.map(c => c === existing ? { ...c, answer: !current } : c);
        }
        return [...prev, {
          id: `temp-${Date.now()}`,
          sprint_id: sprintId!,
          workspace_id: currentWorkspace!.id,
          employee_id: employeeId,
          scored_by: '',
          criteria_id: criteriaId,
          answer: !current,
        }];
      });
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64" />
      </div>
    );
  }

  if (!sprint) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Sprint not found.</p>
      </div>
    );
  }

  const teams = wsSettings?.teams ?? [];
  const people = wsSettings?.people ?? [];
  const jobCriteria = wsSettings?.job_role_criteria ?? [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Calendar className="h-5 w-5 text-muted-foreground" />
            Sprint {sprint.sprint_number}
          </h1>
          <p className="text-muted-foreground">
            {dayjs(sprint.start_date).format('MMM D')} &ndash; {dayjs(sprint.end_date).format('MMM D, YYYY')}
          </p>
        </div>
        <Badge variant={sprint.status === 'active' ? 'default' : sprint.status === 'locked' ? 'destructive' : 'secondary'}>
          {sprint.status}
        </Badge>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="goals">
        <TabsList>
          <TabsTrigger value="goals">Sprint Goals</TabsTrigger>
          <TabsTrigger value="scoring">Individual Scoring</TabsTrigger>
        </TabsList>

        {/* Sprint Goals Tab */}
        <TabsContent value="goals" className="space-y-4">
          {canEdit() && sprint.status !== 'locked' && (
            <div className="flex justify-end">
              <Button size="sm" onClick={() => setAddGoalOpen(true)}>
                <Plus className="h-4 w-4 mr-1" />
                Add Sprint Goal
              </Button>
            </div>
          )}

          {teams.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center">
                <p className="text-muted-foreground">No teams configured.</p>
              </CardContent>
            </Card>
          ) : (
            teams.map(team => {
              const teamGoals = sprintGoals.filter(sg => sg.team_id === team.id);
              return (
                <Card key={team.id}>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-base">{team.name}</CardTitle>
                    <CardDescription>{teamGoals.length} sprint goal{teamGoals.length !== 1 ? 's' : ''}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {teamGoals.length === 0 ? (
                      <p className="text-sm text-muted-foreground">No sprint goals for this team.</p>
                    ) : (
                      <div className="space-y-2">
                        {teamGoals.map(sg => (
                          <div key={sg.id} className="flex items-center justify-between border rounded-md px-3 py-2">
                            <div className="flex items-center gap-3">
                              <Checkbox checked={sg.done} disabled={sprint.status === 'locked'} />
                              <span className="text-sm">{sg.title}</span>
                            </div>
                            <Badge variant="outline" className="text-xs">Weight: {sg.weight}</Badge>
                          </div>
                        ))}
                      </div>
                    )}
                  </CardContent>
                </Card>
              );
            })
          )}

          {/* Add Sprint Goal Dialog */}
          <AddSprintGoalDialog
            open={addGoalOpen}
            onOpenChange={setAddGoalOpen}
            sprintId={sprint.id}
            teams={teams}
            onAdded={() => { setAddGoalOpen(false); load(); }}
          />
        </TabsContent>

        {/* Individual Scoring Tab */}
        <TabsContent value="scoring" className="space-y-4">
          {people.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center">
                <p className="text-muted-foreground">No people configured. Add team members in Settings.</p>
              </CardContent>
            </Card>
          ) : (
            people.filter(p => p.status === 'active').map(person => {
              const criteria = jobCriteria.filter(jc => jc.job_role === person.job_role && jc.enabled);
              return (
                <Card key={person.id}>
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="text-base">{person.name}</CardTitle>
                        <CardDescription className="text-xs">{person.job_role} &middot; {person.email}</CardDescription>
                      </div>
                      <Badge variant="outline" className="text-xs">
                        {criteria.length > 0
                          ? `${checks.filter(c => c.employee_id === person.id && c.answer).length}/${criteria.length}`
                          : 'No criteria'}
                      </Badge>
                    </div>
                  </CardHeader>
                  {criteria.length > 0 && (
                    <CardContent>
                      <div className="space-y-2">
                        {criteria.map(c => {
                          const check = checks.find(ch => ch.employee_id === person.id && ch.criteria_id === c.criteria_id);
                          const isChecked = check?.answer ?? false;
                          return (
                            <div key={c.criteria_id} className="flex items-start gap-3 py-1">
                              <Checkbox
                                checked={isChecked}
                                disabled={sprint.status === 'locked' || !canEdit()}
                                onCheckedChange={() => handleCheckToggle(person.id, c.criteria_id, isChecked)}
                              />
                              <div>
                                <p className="text-sm font-medium">{c.name}</p>
                                <p className="text-xs text-muted-foreground">{c.question}</p>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </CardContent>
                  )}
                </Card>
              );
            })
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function AddSprintGoalDialog({
  open,
  onOpenChange,
  sprintId,
  teams,
  onAdded,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  sprintId: string;
  teams: { id: string; name: string }[];
  onAdded: () => void;
}) {
  const [title, setTitle] = useState('');
  const [teamId, setTeamId] = useState('');
  const [weight, setWeight] = useState('1');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const { error } = await goalsService.upsertSprintGoal({
      sprint_id: sprintId,
      team_id: teamId,
      title,
      weight: Number(weight),
      done: false,
    });
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Sprint goal added');
      setTitle('');
      setTeamId('');
      setWeight('1');
      onAdded();
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Add Sprint Goal</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>Title</Label>
              <Input placeholder="Sprint goal title" value={title} onChange={e => setTitle(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label>Team</Label>
              <Select value={teamId} onValueChange={setTeamId} required>
                <SelectTrigger>
                  <SelectValue placeholder="Select team" />
                </SelectTrigger>
                <SelectContent>
                  {teams.map(t => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Weight</Label>
              <Input type="number" min="1" max="10" value={weight} onChange={e => setWeight(e.target.value)} required />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={saving || !teamId}>{saving ? 'Adding...' : 'Add Goal'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
