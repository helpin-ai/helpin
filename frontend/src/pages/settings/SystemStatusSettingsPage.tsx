import { GitHubReturnNotice } from '@/components/setup/GitHubReturnNotice';
import { SystemStatusPanel } from '@/components/setup/SystemStatusPanel';
import { AppEmailSettingsCard } from '@/components/settings/server/AppEmailSettingsCard';
import { useGitHubReturnResult } from '@/hooks/useGitHubReturnResult';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

/** Settings → System status (Community): server services, for server admins. */
export function SystemStatusSettingsPage() {
  // Read before the frame renders, so the result survives its loading states.
  const githubReturn = useGitHubReturnResult();
  return (
    <SettingsPageFrame section="system-status">
      {(context) => (
        <>
          <GitHubReturnNotice result={githubReturn} className="pt-4" />
          <SystemStatusSettingsContent {...context} />
        </>
      )}
    </SettingsPageFrame>
  );
}

function SystemStatusSettingsContent({ workspaceId, currentWorkspaceSlug, access, permissions }: SettingsPageContext) {
  if (!permissions.isServerAdmin) {
    return <p className="py-5 text-sm text-quiet-text-tertiary">Only server admins can view system status.</p>;
  }
  return (
    <div className="space-y-8">
      <SystemStatusPanel
        workspaceId={workspaceId}
        slug={currentWorkspaceSlug}
        canManage
        isOwner={access?.membership?.role === 'owner'}
      />
      <AppEmailSettingsCard />
    </div>
  );
}
