import { useState } from 'react';
import { GitHubAppCreateButton } from '@/components/git/GitHubAppCreateButton';
import { gitHubAppOwnerNote } from '@/components/git/githubApp';
import { GitProviderIcon } from '@/components/git/GitProviderIcon';
import { Skeleton } from '@/components/ui/skeleton';
import { useGitHubAppStatus } from '@/hooks/queries/useGitHubApp';
import { assignBrowserLocation } from '@/lib/githubReturn';
import { ArrowRight01Icon, Loading01Icon } from '@/lib/icons';
import type { Capability } from '@/lib/capabilityTypes';
import type { GitHubReturnTo } from '@/lib/pmTypes';
import { gitService } from '@/lib/services/gitService';
import {
  CapabilityActionView,
  ServerConfigHint,
  SetupAdminHint,
  SetupResultMessage,
  SetupSettingsLink,
  setupTextActionClassName,
  type SetupResult,
} from './CapabilityActions';

export const GITHUB_APP_ENV_HINT =
  'Set GITHUB_APP_ID, GITHUB_APP_SLUG, GITHUB_APP_PRIVATE_KEY (base64 PEM) and GITHUB_APP_WEBHOOK_SECRET on the server, then restart it.';

export type SetupGitHubStepProps = {
  capability: Capability;
  workspaceId: string;
  slug: string;
  canManage: boolean;
  isOwner: boolean;
  /** Page GitHub returns to after creating or installing the App. */
  returnTo?: GitHubReturnTo;
};

/**
 * Connects GitHub in the order the server needs it: an instance GitHub App
 * (created here from a manifest or configured on the server), an
 * installation for the organization, then repositories for this workspace.
 */
export function SetupGitHubStep({ capability, workspaceId, slug, canManage, isOwner, returnTo = 'settings' }: SetupGitHubStepProps) {
  const settled = capability.status === 'ready' || capability.status === 'unavailable';
  // Refetch on focus: the App is created and installed on GitHub, in this tab or another.
  const appStatus = useGitHubAppStatus(workspaceId, { enabled: canManage && !settled, refetchOnWindowFocus: 'always' });

  if (settled) return null;
  if (!canManage) return <SetupAdminHint />;
  if (appStatus.isLoading) return <Skeleton className="h-8 w-56" />;
  const status = appStatus.data;
  if (!status) return <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />;

  if (!status.configured) {
    if (status.manifest_blocked_reason) {
      return <p className="max-w-2xl text-[12.5px] leading-5 text-quiet-text-tertiary" data-testid="github-blocked">{status.manifest_blocked_reason}</p>;
    }
    if (status.manifest_available) {
      return isOwner ? (
        <div className="space-y-1.5">
          <GitHubAppCreateButton workspaceId={workspaceId} returnTo={returnTo} variant="outline" />
          <p className="text-[12px] text-quiet-text-tertiary">GitHub asks you to confirm the App and install it, then returns you here.</p>
        </div>
      ) : (
        <p className="text-[12.5px] text-quiet-text-tertiary">The workspace owner can create the GitHub App for this server from this page.</p>
      );
    }
    return <ServerConfigHint hint={GITHUB_APP_ENV_HINT} />;
  }

  const path = capability.action?.path;
  const needsInstall = path === 'settings/git-connections';
  const ownerNote = needsInstall ? gitHubAppOwnerNote(status) : null;
  return (
    <div className="space-y-1.5">
      {needsInstall ? (
        <>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <GitHubInstallAction workspaceId={workspaceId} returnTo={returnTo} />
            <SetupSettingsLink slug={slug} path="settings/git-connections">Git connections</SetupSettingsLink>
          </div>
          {ownerNote && <p className="text-[12px] text-quiet-text-tertiary">{ownerNote}</p>}
        </>
      ) : (
        <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />
      )}
      {!status.webhook_configured && (
        <p className="text-[12px] text-quiet-text-tertiary">GitHub webhooks aren’t configured on this server, so pushes and pull requests won’t update tasks.</p>
      )}
    </div>
  );
}

/**
 * Starts the installation with Helpin's signed install link, so GitHub
 * returns to this page and the installation is linked automatically.
 */
export function GitHubInstallAction({ workspaceId, returnTo }: { workspaceId: string; returnTo: GitHubReturnTo }) {
  const [pending, setPending] = useState(false);
  const [result, setResult] = useState<SetupResult | null>(null);

  const install = async () => {
    setPending(true);
    setResult(null);
    const { data, error } = await gitService.getGitHubInstallURL(workspaceId, { returnTo });
    if (error || !data?.install_url) {
      setPending(false);
      setResult({ tone: 'negative', message: error || 'The GitHub App install link is not available.' });
      return;
    }
    if (data.action === 'pick_repos') {
      setPending(false);
      window.open(data.install_url, '_blank', 'noopener,noreferrer');
      return;
    }
    assignBrowserLocation(data.install_url);
  };

  return (
    <>
      <button type="button" className={setupTextActionClassName} disabled={pending} onClick={() => void install()}>
        <GitProviderIcon provider="github" className="h-3.5 w-3.5" />
        Install the GitHub App
        {pending
          ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
          : <ArrowRight01Icon className="h-3.5 w-3.5" aria-hidden="true" />}
      </button>
      <SetupResultMessage result={result} className="basis-full" />
    </>
  );
}
