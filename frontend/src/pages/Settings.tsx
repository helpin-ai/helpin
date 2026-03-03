import { useEffect, useState, type FormEvent } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSessionStore } from '@/stores/sessionStore';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, JobRoleCriteria, BonusTierConfig } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Plus, Pencil, Trash2, Users, UserPlus, Briefcase, Award, Settings2 } from 'lucide-react';
import { toast } from 'sonner';

export default function Settings() {
  const { currentWorkspace } = useWorkspaceStore();
  const { isAdmin } = useSessionStore();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    if (!ws?.id) {
      setSettings(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const { data } = await settingsService.getAll(ws.id);
      if (data) setSettings(data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not load workspace settings.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">{currentWorkspace?.name} workspace configuration</p>
      </div>

      <Tabs defaultValue="teams">
        <TabsList className="flex-wrap">
          <TabsTrigger value="teams" className="gap-1"><Users className="h-3.5 w-3.5" /> Teams</TabsTrigger>
          <TabsTrigger value="people" className="gap-1"><UserPlus className="h-3.5 w-3.5" /> People</TabsTrigger>
          <TabsTrigger value="jobroles" className="gap-1"><Briefcase className="h-3.5 w-3.5" /> Job Roles</TabsTrigger>
          <TabsTrigger value="tiers" className="gap-1"><Award className="h-3.5 w-3.5" /> Bonus Tiers</TabsTrigger>
          <TabsTrigger value="system" className="gap-1"><Settings2 className="h-3.5 w-3.5" /> System</TabsTrigger>
        </TabsList>

        <TabsContent value="teams">
          <TeamsTab
            workspaceId={currentWorkspace!.id}
            teams={settings.teams}
            editable={isAdmin()}
            onRefresh={load}
          />
        </TabsContent>

        <TabsContent value="people">
          <PeopleTab
            workspaceId={currentWorkspace!.id}
            people={settings.people}
            teams={settings.teams}
            editable={isAdmin()}
            onRefresh={load}
          />
        </TabsContent>

        <TabsContent value="jobroles">
          <JobRolesTab
            workspaceId={currentWorkspace!.id}
            criteria={settings.job_role_criteria}
            editable={isAdmin()}
            onRefresh={load}
          />
        </TabsContent>

        <TabsContent value="tiers">
          <BonusTiersTab
            workspaceId={currentWorkspace!.id}
            tiers={settings.bonus_tiers}
            editable={isAdmin()}
            onRefresh={load}
          />
        </TabsContent>

        <TabsContent value="system">
          <SystemTab
            workspaceId={currentWorkspace!.id}
            config={settings.settings}
            editable={isAdmin()}
            onRefresh={load}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}

/* ============ Teams Tab ============ */

function TeamsTab({ workspaceId, teams, editable, onRefresh }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setDescription('');
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setDescription(team.description ?? '');
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    if (editTeam) {
      const { error } = await settingsService.updateTeam(editTeam.id, { name, description: description || undefined });
      if (error) toast.error(error);
      else { toast.success('Team updated'); setDialogOpen(false); onRefresh(); }
    } else {
      const { error } = await settingsService.createTeam({ workspace_id: workspaceId, name, description: description || undefined });
      if (error) toast.error(error);
      else { toast.success('Team created'); setDialogOpen(false); onRefresh(); }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deleteTeam(id);
    if (error) toast.error(error);
    else { toast.success('Team deleted'); onRefresh(); }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Teams</CardTitle>
            <CardDescription>{teams.length} team{teams.length !== 1 ? 's' : ''}</CardDescription>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" /> Add Team
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {teams.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No teams yet.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                {editable && <TableHead className="w-24">Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {teams.map(team => (
                <TableRow key={team.id}>
                  <TableCell className="font-medium">{team.name}</TableCell>
                  <TableCell className="text-muted-foreground">{team.description || '--'}</TableCell>
                  {editable && (
                    <TableCell>
                      <div className="flex gap-1">
                        <Button size="icon" variant="ghost" onClick={() => openEdit(team)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button size="icon" variant="ghost" onClick={() => handleDelete(team.id)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{editTeam ? 'Edit Team' : 'Add Team'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={description} onChange={e => setDescription(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ People Tab ============ */

function PeopleTab({ workspaceId, people, teams: _teams, editable, onRefresh }: {
  workspaceId: string;
  people: WorkspacePerson[];
  teams: WorkspaceTeam[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editPerson, setEditPerson] = useState<WorkspacePerson | null>(null);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<WorkspacePerson['role']>('employee');
  const [jobRole, setJobRole] = useState('');
  const [salary, setSalary] = useState('0');
  const [saving, setSaving] = useState(false);

  const openCreate = () => {
    setEditPerson(null);
    setName(''); setEmail(''); setRole('employee'); setJobRole(''); setSalary('0');
    setDialogOpen(true);
  };

  const openEdit = (p: WorkspacePerson) => {
    setEditPerson(p);
    setName(p.name); setEmail(p.email); setRole(p.role); setJobRole(p.job_role); setSalary(String(p.base_salary));
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    if (editPerson) {
      const { error } = await settingsService.updatePerson(editPerson.id, {
        name, email, role, job_role: jobRole, base_salary: Number(salary),
      });
      if (error) toast.error(error);
      else { toast.success('Person updated'); setDialogOpen(false); onRefresh(); }
    } else {
      const { error } = await settingsService.createPerson({
        workspace_id: workspaceId, name, email, role, job_role: jobRole,
        base_salary: Number(salary), hire_date: new Date().toISOString().split('T')[0],
        status: 'active', active_for_bonus: true, active_for_evaluation: true, is_account_owner: false,
      });
      if (error) toast.error(error);
      else { toast.success('Person added'); setDialogOpen(false); onRefresh(); }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deletePerson(id);
    if (error) toast.error(error);
    else { toast.success('Person removed'); onRefresh(); }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">People</CardTitle>
            <CardDescription>{people.length} member{people.length !== 1 ? 's' : ''}</CardDescription>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" /> Add Person
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {people.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No people added yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Job Role</TableHead>
                  <TableHead>Status</TableHead>
                  {editable && <TableHead className="w-24">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {people.map(p => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="text-muted-foreground">{p.email}</TableCell>
                    <TableCell><Badge variant="outline" className="text-xs">{p.role}</Badge></TableCell>
                    <TableCell>{p.job_role}</TableCell>
                    <TableCell>
                      <Badge variant={p.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                        {p.status}
                      </Badge>
                    </TableCell>
                    {editable && (
                      <TableCell>
                        <div className="flex gap-1">
                          <Button size="icon" variant="ghost" onClick={() => openEdit(p)}>
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button size="icon" variant="ghost" onClick={() => handleDelete(p.id)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{editPerson ? 'Edit Person' : 'Add Person'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Email</Label>
                <Input type="email" value={email} onChange={e => setEmail(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={v => setRole(v as WorkspacePerson['role'])}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="executive">Executive</SelectItem>
                    <SelectItem value="manager">Manager</SelectItem>
                    <SelectItem value="employee">Employee</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Job Role</Label>
                <Input placeholder="e.g. Frontend Developer" value={jobRole} onChange={e => setJobRole(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Base Salary</Label>
                <Input type="number" value={salary} onChange={e => setSalary(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ Job Roles Tab ============ */

function JobRolesTab({ workspaceId, criteria, editable, onRefresh }: {
  workspaceId: string;
  criteria: JobRoleCriteria[];
  editable: boolean;
  onRefresh: () => void;
}) {
  // Group criteria by job_role
  const grouped = criteria.reduce<Record<string, JobRoleCriteria[]>>((acc, c) => {
    if (!acc[c.job_role]) acc[c.job_role] = [];
    acc[c.job_role].push(c);
    return acc;
  }, {});
  const jobRoles = Object.keys(grouped).sort();

  const handleDeleteRole = async (jobRole: string) => {
    const { error } = await settingsService.deleteJobRole(workspaceId, jobRole);
    if (error) toast.error(error);
    else { toast.success(`"${jobRole}" criteria deleted`); onRefresh(); }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Job Role Criteria</CardTitle>
        <CardDescription>Individual scoring criteria by job role</CardDescription>
      </CardHeader>
      <CardContent>
        {jobRoles.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No job role criteria configured.</p>
        ) : (
          <div className="space-y-4">
            {jobRoles.map(role => (
              <div key={role} className="border rounded-md p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-medium">{role}</h3>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {grouped[role].length} criteria
                    </Badge>
                    {editable && (
                      <Button size="icon" variant="ghost" onClick={() => handleDeleteRole(role)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  {grouped[role].map(c => (
                    <div key={c.criteria_id} className="flex items-center justify-between text-sm py-1">
                      <div>
                        <span className="font-medium">{c.name}</span>
                        <span className="text-muted-foreground ml-2">- {c.question}</span>
                      </div>
                      <Badge variant={c.enabled ? 'default' : 'secondary'} className="text-xs">
                        {c.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ Bonus Tiers Tab ============ */

function BonusTiersTab({ workspaceId, tiers, editable, onRefresh }: {
  workspaceId: string;
  tiers: BonusTierConfig[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [localTiers, setLocalTiers] = useState(tiers);
  const [saving, setSaving] = useState(false);

  useEffect(() => { setLocalTiers(tiers); }, [tiers]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateBonusTiers(
      workspaceId,
      localTiers.map(t => ({
        tier: t.tier,
        min_score: t.min_score,
        max_score: t.max_score,
        salary_multiplier: t.salary_multiplier,
        description: t.description,
        editable: t.editable,
      })),
    );
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('Bonus tiers updated'); setEditing(false); onRefresh(); }
  };

  const updateTier = (idx: number, field: string, value: string) => {
    setLocalTiers(prev => prev.map((t, i) => i === idx ? { ...t, [field]: field === 'description' ? value : Number(value) } : t));
  };

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      default: return 'destructive';
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Bonus Tiers</CardTitle>
            <CardDescription>Configure A/B/C tier thresholds and multipliers</CardDescription>
          </div>
          {editable && !editing && (
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil className="h-3.5 w-3.5 mr-1" /> Edit
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {localTiers.map((tier, idx) => (
            <div key={tier.tier} className="border rounded-md p-4 space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant={tierVariant(tier.tier)}>Tier {tier.tier}</Badge>
                {tier.description && <span className="text-sm text-muted-foreground">{tier.description}</span>}
              </div>
              {editing ? (
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs">Min Score</Label>
                    <Input type="number" value={tier.min_score} onChange={e => updateTier(idx, 'min_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Max Score</Label>
                    <Input type="number" value={tier.max_score} onChange={e => updateTier(idx, 'max_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Multiplier</Label>
                    <Input type="number" step="0.1" value={tier.salary_multiplier} onChange={e => updateTier(idx, 'salary_multiplier', e.target.value)} />
                  </div>
                </div>
              ) : (
                <div className="flex gap-6 text-sm">
                  <span>Score: {tier.min_score} - {tier.max_score}</span>
                  <span>Multiplier: {tier.salary_multiplier}x</span>
                </div>
              )}
            </div>
          ))}
        </div>
        {editing && (
          <div className="flex gap-2 mt-4 justify-end">
            <Button variant="outline" onClick={() => { setEditing(false); setLocalTiers(tiers); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ System Tab ============ */

function SystemTab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [sprintDuration, setSprintDuration] = useState(config.sprint_duration_weeks);
  const [teamWeight, setTeamWeight] = useState(config.team_weight);
  const [notifications, setNotifications] = useState(config.notifications_enabled);
  const [autoCalc, setAutoCalc] = useState(config.auto_calculate_bonuses);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setSprintDuration(config.sprint_duration_weeks);
    setTeamWeight(config.team_weight);
    setNotifications(config.notifications_enabled);
    setAutoCalc(config.auto_calculate_bonuses);
  }, [config]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateSystem(workspaceId, {
      sprint_duration_weeks: sprintDuration,
      team_weight: teamWeight,
      notifications_enabled: notifications,
      auto_calculate_bonuses: autoCalc,
    });
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('System settings updated'); onRefresh(); }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">System Settings</CardTitle>
        <CardDescription>General workspace configuration</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label>Sprint Duration (weeks)</Label>
            <Input
              type="number"
              min="1"
              max="4"
              value={sprintDuration}
              onChange={e => setSprintDuration(Number(e.target.value))}
              disabled={!editable}
            />
          </div>
          <div className="space-y-2">
            <Label>Team Weight (%)</Label>
            <Input
              type="number"
              min="0"
              max="100"
              value={teamWeight}
              onChange={e => setTeamWeight(Number(e.target.value))}
              disabled={!editable}
            />
            <p className="text-xs text-muted-foreground">Individual weight: {100 - teamWeight}%</p>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label>Notifications</Label>
              <p className="text-xs text-muted-foreground">Send email notifications for sprint events</p>
            </div>
            <Switch checked={notifications} onCheckedChange={setNotifications} disabled={!editable} />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <Label>Auto-calculate Bonuses</Label>
              <p className="text-xs text-muted-foreground">Automatically recalculate bonuses when scores change</p>
            </div>
            <Switch checked={autoCalc} onCheckedChange={setAutoCalc} disabled={!editable} />
          </div>
        </div>

        {editable && (
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save Settings'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
