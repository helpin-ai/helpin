import { useEffect, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { QuietDropdown, QuietPageHeader, QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet';
import { setupTextActionClassName } from '@/components/setup/CapabilityActions';
import { useWorkspaces } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { assignBrowserLocation, gitHubReturnPath, readGitHubReturnResult, type GitHubInstalledQuery } from '@/lib/githubReturn';
import { ArrowRight01Icon } from '@/lib/icons';
import type { GitHubInstallationClaimResponse } from '@/lib/pmTypes';
import { unwrap } from '@/lib/queryUtils';
import { gitService } from '@/lib/services/gitService';
import type { Workspace } from '@/lib/types';
import { useWorkspaceStore } from '@/stores/workspaceStore';

/**
 * Finishes GitHub App flows that could not return to a workspace page:
 * installations made or changed on GitHub (no Helpin link) are claimed for the
 * chosen workspace's organization, and other results are shown.
 */
export function GitHubInstalledPage({ query }: { query: GitHubInstalledQuery }) {
  useTitle('Connect GitHub');
  const installationId = /^\d+$/.test(query.installation_id) ? query.installation_id : '';
  const update = query.setup_action === 'update';
  return (
    <main className="min-h-screen bg-background px-4 py-10 text-quiet-text-primary sm:py-16">
      <div className="mx-auto w-full max-w-xl">
        <QuietPageHeader
          title={update ? 'Update GitHub access' : 'Connect GitHub'}
          description={installationId
            ? update
              ? 'GitHub changed which repositories Helpin can use. Helpin refreshes the connection.'
              : 'GitHub installed the Helpin App. Helpin links the installation to your organization.'
            : undefined}
        />
        {installationId ? <ClaimInstallation installationId={installationId} /> : <FlowResult query={query} />}
      </div>
    </main>
  );
}

function useCandidateWorkspaces() {
  const current = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaces = useWorkspaces();
  if (current) return { loading: false, workspaces: [current] };
  return { loading: workspaces.isPending, workspaces: workspaces.data ?? [] };
}

function FlowResult({ query }: { query: GitHubInstalledQuery }) {
  const { workspaces } = useCandidateWorkspaces();
  const result = readGitHubReturnResult(new URLSearchParams({ github: query.github || 'error', github_message: query.github_message }).toString());
  const workspace = workspaces[0];
  return (
    <div className="space-y-4 py-6">
      <p role={result?.status === 'error' ? 'alert' : 'status'} className={result?.status === 'error' ? 'text-sm text-quiet-accent' : 'text-sm text-quiet-positive'}>
        {result?.message}
      </p>
      <GitConnectionsLink workspace={workspace} />
    </div>
  );
}

function ClaimInstallation({ installationId }: { installationId: string }) {
  const { loading, workspaces } = useCandidateWorkspaces();
  const [chosenId, setChosenId] = useState('');
  const claim = useMutation({
    mutationFn: async (workspace: Workspace): Promise<{ workspace: Workspace; claim: GitHubInstallationClaimResponse }> => {
      if (!workspace.organization_id) throw new Error('This workspace does not belong to an organization.');
      const result = unwrap(await gitService.claimOrgGitHubInstallation(workspace.organization_id, installationId, workspace.id));
      return { workspace, claim: result };
    },
    onSuccess: ({ workspace, claim: result }) => {
      const params = new URLSearchParams({ github: 'connected', github_message: connectedMessage(result) });
      assignBrowserLocation(`${gitHubReturnPath(workspace.slug)}?${params.toString()}`);
    },
  });

  const onlyWorkspace = workspaces.length === 1 ? workspaces[0] : undefined;
  const autoClaimed = useRef(false);
  const { mutate } = claim;
  useEffect(() => {
    if (!onlyWorkspace || autoClaimed.current) return;
    autoClaimed.current = true;
    mutate(onlyWorkspace);
  }, [onlyWorkspace, mutate]);

  if (loading) return <p role="status" className="py-6 text-sm text-quiet-text-tertiary">Loading your workspaces…</p>;
  if (workspaces.length === 0) {
    return <p role="alert" className="py-6 text-sm text-quiet-accent">You don’t have a Helpin workspace to connect this installation to.</p>;
  }

  const target = claim.variables ?? onlyWorkspace ?? workspaces.find((workspace) => workspace.id === chosenId);
  if (claim.isSuccess) {
    return (
      <div className="space-y-4 py-6">
        <p role="status" className="text-sm text-quiet-positive">{connectedMessage(claim.data.claim)} Opening Git connections…</p>
        <GitConnectionsLink workspace={claim.data.workspace} result={claim.data.claim} />
      </div>
    );
  }
  if (claim.isError) {
    return (
      <div className="space-y-4 py-6">
        <p role="alert" className="text-sm text-quiet-accent">{claim.error instanceof Error ? claim.error.message : 'GitHub could not be connected.'}</p>
        <div className="flex flex-wrap items-center gap-4">
          {target && <QuietTextAction onClick={() => claim.mutate(target)}>Try again</QuietTextAction>}
          <GitConnectionsLink workspace={target} />
        </div>
      </div>
    );
  }
  if (claim.isPending || onlyWorkspace) {
    return <p role="status" className="py-6 text-sm text-quiet-text-tertiary">Connecting the installation to {target?.name ?? 'your workspace'}…</p>;
  }

  const chosen = workspaces.find((workspace) => workspace.id === chosenId);
  return (
    <div className="space-y-5 py-6">
      <div className="space-y-2">
        <p className="text-sm text-quiet-text-secondary">Choose the workspace whose organization should use this installation.</p>
        <QuietDropdown
          label="Choose workspace"
          selected={chosenId ? [chosenId] : []}
          options={workspaces.map((workspace) => ({ value: workspace.id, label: workspace.name }))}
          onSelect={setChosenId}
          trigger={<QuietTextAction aria-label="Choose workspace" className="w-full justify-between truncate">{chosen?.name ?? 'Choose a workspace'}</QuietTextAction>}
        />
      </div>
      <div className="flex justify-end">
        <QuietPrimaryAction disabled={!chosen} onClick={() => chosen && claim.mutate(chosen)}>Connect GitHub</QuietPrimaryAction>
      </div>
    </div>
  );
}

function connectedMessage(result: GitHubInstallationClaimResponse) {
  const account = result.account_login || 'GitHub';
  return result.created ? `GitHub App connected to ${account}.` : `GitHub access for ${account} is up to date.`;
}

function GitConnectionsLink({ workspace, result }: { workspace?: Workspace; result?: GitHubInstallationClaimResponse }) {
  if (!workspace) {
    return <a href="/workspaces" className={setupTextActionClassName}>Go to workspaces<ArrowRight01Icon className="h-3.5 w-3.5" aria-hidden="true" /></a>;
  }
  const params = result ? `?${new URLSearchParams({ github: 'connected', github_message: connectedMessage(result) }).toString()}` : '';
  return (
    <a href={`${gitHubReturnPath(workspace.slug)}${params}`} className={setupTextActionClassName}>
      Go to Git connections
      <ArrowRight01Icon className="h-3.5 w-3.5" aria-hidden="true" />
    </a>
  );
}
