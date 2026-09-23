import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { GitHubAppCreateButton } from '@/components/git/GitHubAppCreateButton';
import { GitProviderIcon } from '@/components/git/GitProviderIcon';
import { gitHubAppOwnerNote } from '@/components/git/githubApp';
import { GitHubInstallAction } from '@/components/setup/SetupGitHubStep';
import { SetupResultMessage } from '@/components/setup/CapabilityActions';
import { useGitHubAppStatus } from '@/hooks/queries/useGitHubApp';
import { useGitHubReturnResult } from '@/hooks/useGitHubReturnResult';
import type { Capability } from '@/lib/capabilityTypes';
import { CheckmarkCircle02Icon } from '@/lib/icons';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

type ConnectGitHubStepProps = {
  workspaceId: string;
  /** The workspace's `github` capability. */
  capability: Capability | undefined;
  onContinue: () => void;
};

/**
 * Optional onboarding step for Community owners who plan and ship projects:
 * create the instance GitHub App (GitHub then asks to install it), or install
 * an App that already exists. GitHub returns to this step with the result,
 * which is shown here. Repositories are chosen later in settings.
 */
export function ConnectGitHubStep({ workspaceId, capability, onContinue }: ConnectGitHubStepProps) {
  // Refetch on focus: the App is created and installed on GitHub, in this tab or another.
  const appStatus = useGitHubAppStatus(workspaceId, { refetchOnWindowFocus: 'always' });
  const returned = useGitHubReturnResult();
  const status = appStatus.data;
  const installed = capability?.status === 'ready' || capability?.action?.path === 'settings/repositories';
  const connected = installed || returned?.status === 'connected';

  let body;
  if (connected) {
    body = (
      <p className="flex items-center gap-2 text-[13.5px] font-semibold leading-5" data-testid="github-connected">
        <CheckmarkCircle02Icon className="h-4 w-4 text-quiet-positive" aria-hidden="true" />
        Connected
      </p>
    );
  } else if (appStatus.isLoading) {
    body = <Skeleton className="h-8 w-56" />;
  } else if (!status) {
    body = <p className="text-[12.5px] text-quiet-text-tertiary">GitHub’s status couldn’t be loaded. You can connect it later in Settings → Git connections.</p>;
  } else if (!status.configured) {
    body = status.manifest_blocked_reason ? (
      <p className="max-w-xl text-[12.5px] leading-5 text-quiet-text-tertiary" data-testid="github-blocked">{status.manifest_blocked_reason}</p>
    ) : status.manifest_available ? (
      <div className="space-y-1.5">
        <GitHubAppCreateButton workspaceId={workspaceId} returnTo="onboarding" variant="outline" />
        <p className="text-[12px] text-quiet-text-tertiary">GitHub asks you to confirm the App and install it, then returns you here.</p>
      </div>
    ) : (
      <p className="text-[12.5px] text-quiet-text-tertiary">
        This server has no GitHub App settings yet. Whoever runs the server can add them; see Settings → Git connections.
      </p>
    );
  } else {
    const ownerNote = gitHubAppOwnerNote(status);
    body = (
      <div className="space-y-1.5">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <GitHubInstallAction workspaceId={workspaceId} returnTo="onboarding" />
        </div>
        {ownerNote && <p className="text-[12px] text-quiet-text-tertiary">{ownerNote}</p>}
      </div>
    );
  }

  return (
    <div className="space-y-7">
      <div className="flex items-start gap-3 border-y border-border py-4">
        <GitProviderIcon provider="github" className="mt-0.5" />
        <div className="min-w-0 flex-1 space-y-3">
          <div>
            <p className="text-[13.5px] font-semibold leading-5">GitHub</p>
            <p className="text-[12.5px] leading-5 text-muted-foreground">
              Agents open branches and pull requests, and pushes and reviews update tasks.
            </p>
          </div>
          {body}
          {/* The result GitHub returned with, for example "GitHub App connected to acme." */}
          <SetupResultMessage
            result={returned ? { tone: returned.status === 'error' ? 'negative' : 'positive', message: returned.message } : null}
          />
        </div>
      </div>
      <p className="text-[12.5px] leading-5 text-muted-foreground">
        You choose which repositories each workspace uses later, in Settings → Repositories.
      </p>
      <OnboardingActions>
        {connected ? (
          <Button type="button" className="w-full sm:w-auto sm:min-w-32" onClick={onContinue}>Continue</Button>
        ) : (
          <OnboardingTextButton onClick={onContinue}>Skip for now</OnboardingTextButton>
        )}
      </OnboardingActions>
    </div>
  );
}
