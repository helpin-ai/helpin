import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { WorkspaceTeam } from '@/lib/types';
import type { EpicWorkflowState, PMAutomation, AutomationType } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Info } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function AutomationsTab({ workspaceId, teams, editable = true }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable?: boolean;
}) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
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
      {/* ── Sprint Settings (read-only overview) ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Sprint Settings</CardTitle>
          <CardDescription>Sprint settings are configured per team. Use team settings to enable sprints and configure automation.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {teams.length === 0 && (
            <p className="text-sm text-muted-foreground">No teams found.</p>
          )}
          {teams.map((team) => {
            const autoCreateCfg = sprintAutoCreateConfigs.find((a) => a.team_id === team.id);
            const hasAutoCreate = sprintAutoCreateTeamIds.has(team.id);
            const hasRollOver = sprintMoveTeamIds.has(team.id);
            const durationWeeks = autoCreateCfg?.config_int2 ?? 2;
            const startDayNum = autoCreateCfg?.config_int3 ?? 1;
            const startDayLabel = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'][startDayNum] ?? 'Monday';

            return (
              <div key={team.id} className="flex items-center justify-between rounded-lg border border-border/60 px-4 py-3">
                <div>
                  <p className="text-sm font-medium">{team.name}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {hasAutoCreate
                      ? <>{`${durationWeeks}-week sprints · Starts ${startDayLabel}`}{hasAutoCreate && <> · <span className="text-emerald-600">Auto-create ✓</span></>}{hasRollOver && <> · <span className="text-emerald-600">Roll over ✓</span></>}</>
                      : 'No sprint automations configured'}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-xs"
                  onClick={() => navigate({ to: '/w/$slug/settings/teams', params: { slug: workspace?.slug ?? '' }, search: { team: team.id, section: 'sprints' } })}
                >
                  Edit &rarr;
                </Button>
              </div>
            );
          })}
        </CardContent>
      </Card>

      {/* ── Epic Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Epic Automations</CardTitle>
          <CardDescription>Automatically transition epics based on story progress.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800 dark:bg-blue-950/50 dark:text-blue-300">
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
    </div>
  );
}
