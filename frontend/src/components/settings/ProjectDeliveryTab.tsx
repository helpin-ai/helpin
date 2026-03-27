import { useCallback, useEffect, useMemo, useState } from 'react';
import { gitService } from '@/lib/services/gitService';
import type { GitIntegration, GitRepository } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { GitBranch, GitPullRequest, Loader2, Plus, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function ProjectDeliveryTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [installingGitHubApp, setInstallingGitHubApp] = useState(false);
  const [installActionError, setInstallActionError] = useState<string | null>(null);
  const [integrationDialogOpen, setIntegrationDialogOpen] = useState(false);
  const [creatingIntegration, setCreatingIntegration] = useState(false);
  const [integrationName, setIntegrationName] = useState('');
  const [accountLogin, setAccountLogin] = useState('');
  const [installationId, setInstallationId] = useState('');
  const [baseUrl, setBaseURL] = useState('');

  const hasIntegrations = integrations.length > 0;
  const hasRepositories = repositories.length > 0;
  const hasGitHubAppIntegration = integrations.some((integration) => integration.provider === 'github' && Boolean(integration.installation_id));

  const loadGitStatus = useCallback(async () => {
    const [integrationsRes, reposRes] = await Promise.all([
      gitService.listIntegrations(workspaceId),
      gitService.listRepositories(workspaceId, { all: true }),
    ]);
    setIntegrations(integrationsRes.data ?? []);
    setRepositories(reposRes.data ?? []);
  }, [workspaceId]);

  useEffect(() => {
    void loadGitStatus();
  }, [loadGitStatus]);

  useEffect(() => {
    const url = new URL(window.location.href);
    const status = url.searchParams.get('github_app');
    const message = url.searchParams.get('github_message');
    if (!status) return;

    if (status === 'connected') {
      setInstallActionError(null);
      toast.success(message || 'GitHub App connected');
    } else {
      toast.error(message || 'GitHub App connection failed');
    }

    url.searchParams.delete('github_app');
    url.searchParams.delete('github_message');
    url.searchParams.delete('integration_id');
    url.searchParams.delete('repo_count');
    const nextQuery = url.searchParams.toString();
    window.history.replaceState({}, '', `${url.pathname}${nextQuery ? `?${nextQuery}` : ''}${url.hash}`);
    void loadGitStatus();
  }, [loadGitStatus]);

  const installGuidance = useMemo(() => {
    if (!installActionError) return null;
    const normalized = installActionError.toLowerCase();
    if (normalized.includes('github app onboarding is not configured')) {
      return {
        title: 'GitHub App server setup required',
        description: 'The API server is missing GitHub App configuration, so it cannot generate the install URL yet.',
        details: ['GITHUB_APP_ID', 'GITHUB_APP_SLUG', 'GITHUB_APP_PRIVATE_KEY (base64 PEM)', 'APP_BASE_URL'],
      };
    }
    if (normalized.includes('workspace_id is required')) {
      return {
        title: 'Workspace context is missing',
        description: 'The request did not include a workspace ID. Refresh the page and try again from the workspace settings route.',
        details: [] as string[],
      };
    }
    return {
      title: 'GitHub App install failed',
      description: installActionError,
      details: [] as string[],
    };
  }, [installActionError]);

  return (
    <div className="space-y-6">
      {/* ── GitHub Connection ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <GitBranch className="h-4 w-4 text-muted-foreground" />
                <CardTitle className="text-base">GitHub Connection</CardTitle>
              </div>
              <CardDescription className="mt-1.5">Connect GitHub to sync repositories, automate branches, and track PRs.</CardDescription>
            </div>
            {editable ? (
              <div className="flex flex-shrink-0 flex-wrap gap-2">
                <Button
                  variant={hasIntegrations ? 'outline' : 'default'}
                  size="sm"
                  disabled={installingGitHubApp}
                  className="gap-1.5"
                  onClick={async () => {
                    setInstallActionError(null);
                    setInstallingGitHubApp(true);
                    const { data, error } = await gitService.getGitHubInstallURL(workspaceId);
                    setInstallingGitHubApp(false);
                    if (error || !data?.install_url) {
                      const message = error || 'GitHub App install URL is not available';
                      setInstallActionError(message);
                      toast.error(message);
                      return;
                    }
                    window.location.assign(data.install_url);
                  }}
                >
                  {installingGitHubApp ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                  {installingGitHubApp ? 'Opening GitHub...' : hasGitHubAppIntegration ? 'Manage access' : hasIntegrations ? 'Add integration' : 'Install GitHub App'}
                </Button>
                <Button variant="ghost" size="sm" className="gap-1.5" onClick={() => setIntegrationDialogOpen(true)}>
                  <Plus className="h-3.5 w-3.5" />
                  Manual
                </Button>
              </div>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">

          {installGuidance ? (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
              <div className="flex items-center gap-2">
                <Badge variant="destructive">Setup required</Badge>
                <p className="font-medium">{installGuidance.title}</p>
              </div>
              <p className="mt-2 text-sm text-muted-foreground">{installGuidance.description}</p>
              {installGuidance.details.length ? (
                <p className="mt-2 text-sm text-muted-foreground">
                  Required envs: {installGuidance.details.map((item, index) => (
                    <span key={item}>
                      <code className="rounded bg-background px-1 py-0.5 text-xs">{item}</code>
                      {index < installGuidance.details.length - 1 ? ', ' : ''}
                    </span>
                  ))}
                </p>
              ) : null}
              {installActionError && installActionError !== installGuidance.description ? (
                <p className="mt-2 text-xs text-muted-foreground">Backend response: {installActionError}</p>
              ) : null}
            </div>
          ) : null}

          {/* Connected integrations */}
          {integrations.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No integrations connected yet.
            </div>
          ) : (
            <div className="space-y-2">
              {integrations.map((integration) => (
                <div key={integration.id} className="flex flex-col gap-3 rounded-lg border border-border/60 p-4 md:flex-row md:items-center md:justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="font-medium">{integration.display_name}</p>
                      <Badge
                        variant="outline"
                        className={integration.active
                          ? 'border-green-500/30 bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : ''
                        }
                      >
                        {integration.active ? 'Active' : 'Inactive'}
                      </Badge>
                    </div>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {integration.provider}
                      {integration.account_login ? ` · ${integration.account_login}` : ''}
                      {integration.installation_id ? ` · installation ${integration.installation_id}` : ''}
                      {integration.last_synced_at ? ` · synced ${new Date(integration.last_synced_at).toLocaleString()}` : ''}
                    </p>
                    {integration.last_sync_error && (
                      <p className="mt-1 text-xs text-destructive">{integration.last_sync_error}</p>
                    )}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1.5"
                    disabled={syncingIntegrationId === integration.id}
                    onClick={async () => {
                      setSyncingIntegrationId(integration.id);
                      const { error } = await gitService.syncRepositories(workspaceId, integration.id);
                      setSyncingIntegrationId(null);
                      if (error) {
                        toast.error(error);
                        return;
                      }
                      toast.success('Repositories synced');
                      await loadGitStatus();
                    }}
                  >
                    {syncingIntegrationId === integration.id
                      ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      : <RefreshCw className="h-3.5 w-3.5" />
                    }
                    {syncingIntegrationId === integration.id ? 'Syncing...' : 'Sync'}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Repository Catalog ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitPullRequest className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Repository Catalog</CardTitle>
            {hasRepositories && (
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {repositories.length}
              </span>
            )}
          </div>
          <CardDescription>Choose which synced repositories are available to teams and story delivery targets.</CardDescription>
        </CardHeader>
        <CardContent>
          {hasRepositories ? (
            <div className="space-y-2">
              {repositories.map((repo) => (
                <div key={repo.id} className="rounded-lg border border-border/60 p-4">
                  <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="font-medium">{repo.full_name}</p>
                        <Badge variant="outline" className="text-[10px]">{repo.private ? 'Private' : 'Public'}</Badge>
                        {repo.archived ? <Badge variant="secondary" className="text-[10px]">Archived</Badge> : null}
                      </div>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        <span className="font-mono">{repo.default_branch}</span>
                      </p>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-xs text-muted-foreground">Available for delivery</span>
                      <Switch
                        checked={repo.selected}
                        disabled={!editable || repo.archived}
                        onCheckedChange={async (checked) => {
                          const { error } = await gitService.updateRepository(workspaceId, repo.id, { selected: checked });
                          if (error) {
                            toast.error(error);
                            return;
                          }
                          toast.success(`${repo.full_name} ${checked ? 'enabled' : 'hidden'} for delivery`);
                          await loadGitStatus();
                        }}
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              {hasIntegrations
                ? 'No repositories synced yet. Use the Sync button on your integration above to pull in repositories.'
                : 'Repositories will appear here after you connect a GitHub integration and sync.'}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={integrationDialogOpen} onOpenChange={setIntegrationDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Connect GitHub App</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              Fallback for environments where the install handshake is not available. Register a GitHub App installation manually so shared runners can mint short-lived installation tokens.
            </p>
            <div className="space-y-2">
              <Label>Display name</Label>
              <Input
                value={integrationName}
                onChange={(event) => setIntegrationName(event.target.value)}
                placeholder="GitHub Production"
              />
            </div>
            <div className="space-y-2">
              <Label>Account / org</Label>
              <Input
                value={accountLogin}
                onChange={(event) => setAccountLogin(event.target.value)}
                placeholder="acme-inc"
              />
            </div>
            <div className="space-y-2">
              <Label>Installation ID</Label>
              <Input
                value={installationId}
                onChange={(event) => setInstallationId(event.target.value)}
                placeholder="12345678"
              />
            </div>
            <div className="space-y-2">
              <Label>Base URL</Label>
              <Input
                value={baseUrl}
                onChange={(event) => setBaseURL(event.target.value)}
                placeholder="https://github.com"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIntegrationDialogOpen(false)}
              disabled={creatingIntegration}
            >
              Cancel
            </Button>
            <Button
              disabled={creatingIntegration || !integrationName.trim() || !installationId.trim()}
              onClick={async () => {
                setCreatingIntegration(true);
                const { error } = await gitService.createIntegration(workspaceId, {
                  provider: 'github',
                  display_name: integrationName.trim(),
                  credential_mode: 'github_app',
                  account_login: accountLogin.trim() || undefined,
                  installation_id: installationId.trim(),
                  base_url: baseUrl.trim() || undefined,
                });
                setCreatingIntegration(false);
                if (error) {
                  toast.error(error);
                  return;
                }
                toast.success('GitHub App integration connected');
                setIntegrationDialogOpen(false);
                setIntegrationName('');
                setAccountLogin('');
                setInstallationId('');
                setBaseURL('');
                await loadGitStatus();
              }}
            >
              {creatingIntegration ? 'Connecting...' : 'Connect'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
