import { CRMEmailSettingsTab } from '@/components/settings';
import { QuietTextAction } from '@/components/design-system/quiet';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMEmailAccountsSettingsPage() {
  return (
    <SettingsPageFrame section="crm-email" headerAction={({ currentWorkspaceSlug }) => (
      <QuietTextAction asChild><a href={`/w/${currentWorkspaceSlug}/crm/emails`}>Back to Email</a></QuietTextAction>
    )}>
      {({ workspaceId }) => <CRMEmailSettingsTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
