import { useEffect, useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceSettings } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function SystemTab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [sprintDuration, setSprintDuration] = useState(config.sprint_duration_weeks);
  const [teamWeight, setTeamWeight] = useState(config.team_weight);
  const [notifications, setNotifications] = useState(config.notifications_enabled);
  const [autoCalc, setAutoCalc] = useState(config.auto_calculate_bonuses);
  useEffect(() => {
    setSprintDuration(config.sprint_duration_weeks);
    setTeamWeight(config.team_weight);
    setNotifications(config.notifications_enabled);
    setAutoCalc(config.auto_calculate_bonuses);
  }, [config]);

  const handleSave = async () => {
    const { error } = await settingsService.updateSystem(workspaceId, {
      sprint_duration_weeks: sprintDuration,
      team_weight: teamWeight,
      notifications_enabled: notifications,
      auto_calculate_bonuses: autoCalc,
    });
    if (error) toast.error(error);
    else { toast.success('System settings updated'); onRefresh(); }
  };

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Reward Defaults</CardTitle>
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
                <Label>Sprint event emails</Label>
                <p className="text-xs text-muted-foreground">Control workspace-level emails for sprint-related events.</p>
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
        </CardContent>
      </Card>
      {editable && (
        <div className="flex justify-end">
          <Button onClick={handleSave}>Save</Button>
        </div>
      )}
    </div>
  );
}
