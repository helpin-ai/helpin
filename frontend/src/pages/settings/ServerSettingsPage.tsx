import { ServerAdminsCard } from '@/components/settings/server/ServerAdminsCard';
import { ServerSignupCard } from '@/components/settings/server/ServerSignupCard';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

/** Settings → Signup & admins (Community): server-wide, for server admins only. */
export function ServerSettingsPage() {
  return (
    <SettingsPageFrame section="server">
      {(context) => <ServerSettingsContent {...context} />}
    </SettingsPageFrame>
  );
}

function ServerSettingsContent({ currentWorkspaceSlug, permissions }: SettingsPageContext) {
  if (!permissions.isServerAdmin) {
    return <p className="py-5 text-sm text-quiet-text-tertiary">Only server admins can manage signup and server admins.</p>;
  }
  return (
    <div className="space-y-4">
      <ServerSignupCard slug={currentWorkspaceSlug} />
      <ServerAdminsCard />
    </div>
  );
}
