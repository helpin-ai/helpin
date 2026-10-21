import { CustomerPortalSettings } from '@/components/settings/CustomerPortalSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CustomerPortalSettingsPage() {
  return (
    <SettingsPageFrame section="customer-portal">
      {({ workspaceId, currentWorkspaceSlug, permissions }) => (
        <CustomerPortalSettings
          workspaceId={workspaceId}
          workspaceSlug={currentWorkspaceSlug}
          editable={permissions.has('support.admin')}
        />
      )}
    </SettingsPageFrame>
  );
}
