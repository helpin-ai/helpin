import { useEffect, useState } from 'react';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { WorkspaceTeam } from '@/lib/types';
import type { EpicWorkflowState, PMAutomation, AutomationType } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Info, X } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function AutomationsTab({ workspaceId, teams, editable = true }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable?: boolean;
}) {
  const [automations, setAutomations] = useState<PMAutomation[]>([]);
  const [epicStates, setEpicStates] = useState<EpicWorkflowState[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const [autoRes, statesRes] = await Promise.all([
          pmAutomationService.list(workspaceId),
          pmWorkflowService.listEpicStates(workspaceId),
        ]);
        if (cancelled) return;
        if (autoRes.data) setAutomations(autoRes.data);
        if (statesRes.data) setEpicStates(statesRes.data);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [workspaceId]);

  const getAuto = (type: AutomationType, teamId?: string): PMAutomation | undefined =>
    automations.find((a) => a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id));

  const upsert = async (type: AutomationType, enabled: boolean, opts?: { teamId?: string; configStateId?: string; configInt?: number; configInt2?: number; configInt3?: number }) => {
    const payload = {
      workspace_id: workspaceId,
      automation_type: type,
      enabled,
      team_id: opts?.teamId,
      config_state_id: opts?.configStateId,
      config_int: opts?.configInt,
      config_int2: opts?.configInt2,
      config_int3: opts?.configInt3,
    };

    // Snapshot for rollback
    const snapshot = automations;

    // Optimistic update
    setAutomations((prev) => {
      const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
      if (idx >= 0) {
        const updated = [...prev];
        updated[idx] = { ...updated[idx], enabled, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3 };
        return updated;
      }
      return [...prev, { id: 'temp-' + Date.now(), workspace_id: workspaceId, automation_type: type, enabled, team_id: opts?.teamId, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3, created_at: '', updated_at: '' }];
    });

    try {
      const res = await pmAutomationService.upsert(workspaceId, payload);
      if (res.error) {
        setAutomations(snapshot);
        toast.error(res.error);
      } else if (res.data) {
        // Replace temp/stale entry with server response
        setAutomations((prev) => {
          const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = res.data!;
            return updated;
          }
          return prev;
        });
      }
    } catch {
      setAutomations(snapshot);
      toast.error('Failed to save automation');
    }
  };

  const removeAuto = async (type: AutomationType, teamId?: string) => {
    const snapshot = automations;
    setAutomations((prev) => prev.filter((a) => !(a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id))));
    const res = await pmAutomationService.remove(workspaceId, type, teamId);
    if (res.error) {
      setAutomations(snapshot);
      toast.error(res.error);
    }
  };

  const updateSprintConfig = (cfg: PMAutomation, patch: { enabled?: boolean; configInt?: number; configInt2?: number; configInt3?: number }) =>
    upsert('sprint_auto_create', patch.enabled ?? cfg.enabled, {
      teamId: cfg.team_id!,
      configInt: patch.configInt ?? cfg.config_int ?? 2,
      configInt2: patch.configInt2 ?? cfg.config_int2 ?? 1,
      configInt3: patch.configInt3 ?? cfg.config_int3 ?? 1,
    });

  const startedStates = epicStates.filter((s) => s.state_type === 'started');
  const doneStates = epicStates.filter((s) => s.state_type === 'done');

  const autoStart = getAuto('epic_auto_start');
  const autoComplete = getAuto('epic_auto_complete');

  const sprintAutoCreateConfigs = automations.filter((a) => a.automation_type === 'sprint_auto_create' && a.team_id);
  const sprintMoveConfigs = automations.filter((a) => a.automation_type === 'sprint_move_unfinished' && a.team_id);

  const sprintAutoCreateTeamIds = new Set(sprintAutoCreateConfigs.map((a) => a.team_id!));
  const sprintMoveTeamIds = new Set(sprintMoveConfigs.map((a) => a.team_id!));

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* ── Epic Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Epic Automations</CardTitle>
          <CardDescription>Automatically transition epics based on story progress.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Epic Automations affect the entire workspace.
          </div>

          {/* Auto Start Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Start Epic</p>
              <p className="text-xs text-muted-foreground">
                When any story moves to a started state, auto-transition its parent epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {startedStates.length > 1 ? (
                <Select
                  value={autoStart?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_start', autoStart?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {startedStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : startedStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{startedStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoStart?.enabled ?? false}
                disabled={!editable}
                onCheckedChange={(checked) => {
                  const stateId = autoStart?.config_state_id ?? startedStates[0]?.id;
                  if (!stateId) { toast.error('No started epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_start', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>

          {/* Auto Complete Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Complete Epic</p>
              <p className="text-xs text-muted-foreground">
                When all stories in an epic reach a done state, auto-transition the epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {doneStates.length > 1 ? (
                <Select
                  value={autoComplete?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_complete', autoComplete?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {doneStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : doneStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{doneStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoComplete?.enabled ?? false}
                disabled={!editable}
                onCheckedChange={(checked) => {
                  const stateId = autoComplete?.config_state_id ?? doneStates[0]?.id;
                  if (!stateId) { toast.error('No done epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_complete', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Sprint Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Sprint Automations</CardTitle>
          <CardDescription>Automate sprint creation and story rollover per team.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Sprint Automations are specific to each Team.
          </div>

          {/* Auto-Create Future Sprints */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Auto-Create Future Sprints</p>
                <p className="text-xs text-muted-foreground">
                  Automatically create future sprints when a sprint completes.
                </p>
              </div>
              {editable && (
                <Select
                  value=""
                  onValueChange={(teamId) => upsert('sprint_auto_create', true, { teamId, configInt: 2, configInt2: 1, configInt3: 1 })}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue placeholder="Add Team..." />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.filter((t) => !sprintAutoCreateTeamIds.has(t.id)).map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
            {sprintAutoCreateConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium min-w-[100px]">{team?.name ?? 'Unknown'}</span>
                  <div className="flex items-center gap-2 text-xs flex-wrap">
                    <Label className="text-xs text-muted-foreground">Sprints:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={10}
                      value={cfg.config_int ?? 2}
                      onChange={(e) => updateSprintConfig(cfg, { configInt: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                      disabled={!editable}
                    />
                    <Label className="text-xs text-muted-foreground">Weeks:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={8}
                      value={cfg.config_int2 ?? 1}
                      onChange={(e) => updateSprintConfig(cfg, { configInt2: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                      disabled={!editable}
                    />
                    <Label className="text-xs text-muted-foreground">Start day:</Label>
                    <Select
                      value={String(cfg.config_int3 ?? 1)}
                      onValueChange={(val) => updateSprintConfig(cfg, { configInt3: Number(val) })}
                      disabled={!editable}
                    >
                      <SelectTrigger className="w-[100px] h-7 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'].map((day, i) => (
                          <SelectItem key={i} value={String(i)}>{day}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <Switch
                    checked={cfg.enabled}
                    disabled={!editable}
                    onCheckedChange={(checked) => updateSprintConfig(cfg, { enabled: checked })}
                  />
                  {editable && (
                    <button type="button" onClick={() => removeAuto('sprint_auto_create', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
              );
            })}
          </div>

          {/* Move Unfinished Stories */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Move Unfinished Stories to Next Sprint</p>
                <p className="text-xs text-muted-foreground">
                  When a sprint ends, move incomplete stories to the next sprint.
                </p>
              </div>
              {editable && (
                <Select
                  value=""
                  onValueChange={(teamId) => upsert('sprint_move_unfinished', true, { teamId })}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue placeholder="Add Team..." />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.filter((t) => !sprintMoveTeamIds.has(t.id)).map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
            {sprintMoveConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium flex-1">{team?.name ?? 'Unknown'}</span>
                  <Switch
                    checked={cfg.enabled}
                    disabled={!editable}
                    onCheckedChange={(checked) => upsert('sprint_move_unfinished', checked, { teamId: cfg.team_id! })}
                  />
                  {editable && (
                    <button type="button" onClick={() => removeAuto('sprint_move_unfinished', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
