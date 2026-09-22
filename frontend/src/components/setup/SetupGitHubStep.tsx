import { GitHubAppCreateButton } from '@/components/git/GitHubAppCreateButton';
import { Skeleton } from '@/components/ui/skeleton';
import { useGitHubAppStatus } from '@/hooks/queries/useGitHubApp';
import { LinkSquare01Icon } from '@/lib/icons';
import type { Capability } from '@/lib/capabilityTypes';
import {
  CapabilityActionView,
  ServerConfigHint,
  SetupAdminHint,
  SetupSettingsLink,
  setupTextActionClassName,
} from './CapabilityActions';

export const GITHUB_APP_ENV_HINT =
  'Set GITHUB_APP_ID, GITHUB_APP_SLUG, GITHUB_APP_PRIVATE_KEY (base64 PEM) and GITHUB_APP_WEBHOOK_SECRET on the server, then restart it.';

export type SetupGitHubStepProps = {
  capability: Capability;
  workspaceId: string;
  slug: string;
  canManage: boolean;
  isOwner: boolean;
};

/**
 * Connects GitHub in the order the server needs it: an instance GitHub App
 * (created here from a manifest or configured on the server), an
 * installation for the organization, then repositories for this workspace.
 */
export function SetupGitHubStep({ capability, workspaceId, slug, canManage, isOwner }: SetupGitHubStepProps) {
  const settled = capability.status === 'ready' || capability.status === 'unavailable';
  // Refetch on focus: the App is created and installed on GitHub, in this tab or another.
  const appStatus = useGitHubAppStatus(workspaceId, { enabled: canManage && !settled, refetchOnWindowFocus: 'always' });

  if (settled) return null;
  if (!canManage) return <SetupAdminHint />;
  if (appStatus.isLoading) return <Skeleton className="h-8 w-56" />;
  const status = appStatus.data;
  if (!status) return <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />;

  if (!status.configured) {
    if (status.manifest_available) {
      return isOwner ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <GitHubAppCreateButton workspaceId={workspaceId} variant="outline" />
          <span className="text-[12px] text-quiet-text-tertiary">GitHub creates the App for this server and returns you to Helpin.</span>
        </div>
      ) : (
        <p className="text-[12.5px] text-quiet-text-tertiary">The workspace owner can create the GitHub App for this server from this page.</p>
      );
    }
    return <ServerConfigHint hint={GITHUB_APP_ENV_HINT} />;
  }

  const path = capability.action?.path;
  const needsInstall = path === 'settings/git-connections';
  return (
    <div className="space-y-1.5">
      {needsInstall && status.install_url ? (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <a href={status.install_url} target="_blank" rel="noopener noreferrer" className={setupTextActionClassName}>
            Install the GitHub App
            <LinkSquare01Icon className="h-3.5 w-3.5" aria-hidden="true" />
            <span className="sr-only">(opens GitHub in a new tab)</span>
          </a>
          <SetupSettingsLink slug={slug} path="settings/git-connections">Git connections</SetupSettingsLink>
        </div>
      ) : (
        <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />
      )}
      {!status.webhook_configured && (
        <p className="text-[12px] text-quiet-text-tertiary">GitHub webhooks aren’t configured on this server, so pushes and pull requests won’t update tasks.</p>
      )}
    </div>
  );
}
