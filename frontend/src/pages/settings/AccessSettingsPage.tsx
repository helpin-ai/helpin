import { ModuleAccessTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { settingsService } from '@/lib/services/settingsService';
import { queryClient } from '@/lib/queryClient';
import { queryKeys } from '@/lib/queryKeys';
import { toast } from 'sonner';

export function AccessSettingsPage() {
  return (
    <SettingsPageFrame section="access">
      {({ workspaceId, settings, access, permissions }) => (
        <div className="space-y-5">
          <SecurityEnforcementPanel
            workspaceId={workspaceId}
            enabled={!!settings.settings.enforce_two_factor}
            editable={permissions.canManageSettings}
            mfaEnabled={!!access?.security_policy?.mfa_enabled}
            mfaSatisfied={!!access?.security_policy?.mfa_satisfied}
          />
          <ModuleAccessTab
            workspaceId={workspaceId}
            teams={settings.teams}
            people={settings.people}
            editable={permissions.canManageModuleAccess}
          />
        </div>
      )}
    </SettingsPageFrame>
  );
}

function SecurityEnforcementPanel({
  workspaceId,
  enabled,
  editable,
  mfaEnabled,
  mfaSatisfied,
}: {
  workspaceId: string;
  enabled: boolean;
  editable: boolean;
  mfaEnabled: boolean;
  mfaSatisfied: boolean;
}) {
  const canToggle = editable && (enabled || (mfaEnabled && mfaSatisfied));

  const handleChange = async (checked: boolean) => {
    if (!canToggle) return;
    const { error } = await settingsService.updateSystem(workspaceId, { enforce_two_factor: checked });
    if (error) {
      toast.error(error);
      return;
    }
    await queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(workspaceId) });
    await queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.access(workspaceId) });
    toast.success(checked ? 'Two-factor enforcement enabled' : 'Two-factor enforcement disabled');
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Two-factor enforcement</CardTitle>
        <CardDescription>
          Require workspace members to verify with two-factor authentication before accessing workspace data.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-1">
            <p className="text-sm font-medium">{enabled ? 'Required for this workspace' : 'Not required'}</p>
            <p className="text-sm text-muted-foreground">
              {mfaEnabled
                ? 'Members who have not enabled 2FA will be guided through setup when they enter the workspace.'
                : 'Enable 2FA on your profile before requiring it for everyone else.'}
            </p>
          </div>
          <Switch checked={enabled} onCheckedChange={(checked) => void handleChange(checked)} disabled={!canToggle} />
        </div>
      </CardContent>
    </Card>
  );
}
