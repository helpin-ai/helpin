import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { QuietDropdown, QuietPageHeader, QuietPrimaryAction, QuietSection, QuietTextAction } from '@/components/design-system/quiet';
import { useTitle } from '@/hooks/useTitle';
import { unwrap } from '@/lib/queryUtils';
import { cliService } from '@/lib/services/cliService';
import type { CLIAuthorizationQuery, CLIConsentRequest } from '@/lib/cliTypes';

export function CLIAuthorizePage({ query }: { query: CLIAuthorizationQuery }) {
  useTitle('Connect Agent Runtime CLI');
  const request = useQuery({ queryKey: ['cli-consent', query], queryFn: async () => unwrap(await cliService.consentRequest(query)), retry: false });
  return (
    <main className="min-h-screen bg-background px-4 py-10 text-quiet-text-primary sm:py-16">
      <div className="mx-auto w-full max-w-xl">
        <QuietPageHeader title="Connect Agent Runtime CLI" description="Choose the Helpin workspace your terminal can access." />
        {request.isPending ? <p role="status" className="py-6 text-sm text-quiet-text-tertiary">Checking this authorization request…</p>
          : request.isError || !request.data ? <p role="alert" className="py-6 text-sm text-quiet-accent">This request could not be verified. Return to your terminal and start login again.</p>
          : <ConsentForm key={`${request.data.query.state}:${request.data.query.client_id}`} request={request.data} />}
      </div>
    </main>
  );
}
function ConsentForm({ request }: { request: CLIConsentRequest }) {
  const [workspaceId, setWorkspaceId] = useState('');
  const selected = request.workspaces.find((workspace) => workspace.id === workspaceId);
  const authorize = useMutation({ mutationFn: async () => unwrap(await cliService.authorize(request.query, workspaceId)),
    onSuccess: (result) => window.location.assign(result.redirect_url) });
  const deny = () => {
    const redirect = new URL(request.query.redirect_uri);
    redirect.searchParams.set('error', 'access_denied');
    redirect.searchParams.set('state', request.query.state);
    window.location.assign(redirect.toString());
  };
  return (
    <>
      <QuietSection title="Workspace" className="mt-6 px-0 sm:px-0 lg:px-0">
        {request.workspaces.length === 0
          ? <p className="text-sm text-quiet-text-tertiary">No eligible workspaces. You need permission to run agents and must meet the workspace’s sign-in requirements.</p>
          : <QuietDropdown label="Choose workspace" selected={workspaceId ? [workspaceId] : []}
              options={request.workspaces.map((workspace) => ({ value: workspace.id, label: workspace.name }))}
              onSelect={setWorkspaceId} disabled={authorize.isPending}
              trigger={<QuietTextAction aria-label="Choose workspace" className="w-full justify-between truncate">{selected?.name ?? 'Choose a workspace'}</QuietTextAction>} />}
      </QuietSection>
      <QuietSection title="Access you are granting" className="px-0 sm:px-0 lg:px-0">
        <ul className="space-y-2 text-sm text-quiet-text-secondary">
          <li>Read agent configuration and authorized task context.</li>
          <li>Prepare and manage local agent runs in the selected workspace.</li>
        </ul>
        <p className="mt-4 text-sm text-quiet-text-tertiary">Your workspace permissions still apply. Signing out in the CLI revokes this connection.</p>
      </QuietSection>
      {authorize.isError && <p role="alert" className="py-4 text-sm text-quiet-accent">Authorization could not be completed. Check your workspace access and try again.</p>}
      <div className="flex flex-wrap justify-end gap-3 py-6">
        <QuietTextAction onClick={deny} disabled={authorize.isPending}>Cancel</QuietTextAction>
        <QuietPrimaryAction disabled={!selected || authorize.isPending} onClick={() => authorize.mutate()}>
          {authorize.isPending ? 'Connecting…' : 'Authorize CLI'}
        </QuietPrimaryAction>
      </div>
    </>
  );
}
