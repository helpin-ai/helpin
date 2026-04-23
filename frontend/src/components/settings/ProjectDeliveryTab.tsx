import { useCallback, useEffect, useMemo, useState } from 'react';
import { gitService } from '@/lib/services/gitService';
import type {
  GitAvailableRepo,
  GitIntegration,
  GitIntegrationDetail,
  GitRepository,
  WireGitRepositoriesConflictResponse,
} from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { GitBranchIcon, GitPullRequestIcon, Loading01Icon, PlusSignIcon, ArrowReloadHorizontalIcon, Delete01Icon, LinkSquare01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function ProjectDeliveryTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [disconnectingIntegrationId, setDisconnectingIntegrationId] = useState<string | null>(null);
  const [installingGitHubApp, setInstallingGitHubApp] = useState(false);
  const [installActionError, setInstallActionError] = useState<string | null>(null);
  const [installAction, setInstallAction] = useState<'install' | 'pick_repos'>('install');
  const [repoPickerIntegrationId, setRepoPickerIntegrationId] = useState<string | null>(null);
  const [availableRepos, setAvailableRepos] = useState<GitAvailableRepo[]>([]);
  const [selectedRepoIDs, setSelectedRepoIDs] = useState<string[]>([]);
  const [loadingAvailableRepos, setLoadingAvailableRepos] = useState(false);
  const [wiringRepos, setWiringRepos] = useState(false);
  const [disconnectDialogOpen, setDisconnectDialogOpen] = useState(false);
  const [integrationDetail, setIntegrationDetail] = useState<GitIntegrationDetail | null>(null);
  const [integrationDialogOpen, setIntegrationDialogOpen] = useState(false);
  const [creatingIntegration, setCreatingIntegration] = useState(false);
  const [integrationName, setIntegrationName] = useState('');
  const [accountLogin, setAccountLogin] = useState('');
  const [installationId, setInstallationId] = useState('');
  const [baseUrl, setBaseURL] = useState('');

  const hasIntegrations = integrations.length > 0;
  const hasRepositories = repositories.length > 0;
  const hasGitHubAppIntegration = integrations.some((integration) => integration.provider === 'github' && Boolean(integration.installation_id));
  const hasAvailableRepos = availableRepos.length > 0;

  const reposByIntegration = useMemo(() => {
    const map = new Map<string, GitRepository[]>();
    for (const repo of repositories) {
      const list = map.get(repo.integration_id) ?? [];
      list.push(repo);
      map.set(repo.integration_id, list);
    }
    return map;
  }, [repositories]);

  const loadAvailableRepos = useCallback(async (integrationId: string) => {
    setLoadingAvailableRepos(true);
    const { data, error, status } = await gitService.listAvailableRepos(workspaceId, integrationId);
    setLoadingAvailableRepos(false);
    if (error) {
      if (status === 403) {
        toast.error("You don't have access to this organization's integrations.");
      } else {
        toast.error(error);
      }
      return;
    }
    setAvailableRepos(data ?? []);
    setRepoPickerIntegrationId(integrationId);
    setSelectedRepoIDs([]);
  }, [workspaceId]);

  const refreshInstallAction = useCallback(async (preferReloadRepos = false) => {
    const { data, error } = await gitService.getGitHubInstallURL(workspaceId);
    if (error || !data) {
      setInstallAction('install');
      setRepoPickerIntegrationId(null);
      if (error) {
        setInstallActionError(error);
      }
      return;
    }
    setInstallActionError(null);
    setInstallAction(data.action);
    const nextIntegrationId = data.integration_id ?? integrations.find((integration) => integration.provider === 'github' && integration.installation_id)?.id ?? null;
    setRepoPickerIntegrationId(nextIntegrationId);
    if ((preferReloadRepos || data.action === 'pick_repos') && nextIntegrationId) {
      void loadAvailableRepos(nextIntegrationId);
    }
  }, [integrations, loadAvailableRepos, workspaceId]);

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
    void refreshInstallAction();
  }, [refreshInstallAction]);

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
    void refreshInstallAction(true);
  }, [loadGitStatus, refreshInstallAction]);

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
                <GitBranchIcon className="h-4 w-4 text-muted-foreground" />
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
                    if (error || !data) {
                      const message = error || 'GitHub App install URL is not available';
                      setInstallActionError(message);
                      toast.error(message);
                      return;
                    }
                    setInstallAction(data.action);
                    if (data.action === 'pick_repos' && data.integration_id) {
                      await loadAvailableRepos(data.integration_id);
                      toast.success('Choose the repositories this workspace should use');
                      return;
                    }
                    if (!data.install_url) {
                      const message = 'GitHub App install URL is not available';
                      setInstallActionError(message);
                      toast.error(message);
                      return;
                    }
                    window.location.assign(data.install_url);
                  }}
                >
                  {installingGitHubApp ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
                  {installingGitHubApp
                    ? 'Opening GitHub...'
                    : installAction === 'pick_repos'
                      ? 'Pick repositories'
                      : hasGitHubAppIntegration
                        ? 'Manage access'
                        : hasIntegrations
                          ? 'Add integration'
                          : 'Install GitHub App'}
                </Button>
                <Button variant="ghost" size="sm" className="gap-1.5" onClick={() => setIntegrationDialogOpen(true)}>
                  <PlusSignIcon className="h-3.5 w-3.5" />
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
            <div className="space-y-3">
              {integrations.map((integration) => {
                const integrationRepos = reposByIntegration.get(integration.id) ?? [];
                const activeRepos = integrationRepos.filter((r) => !r.archived);
                return (
                  <div key={integration.id} className="rounded-lg border border-border/60">
                    {/* Integration header */}
                    <div className="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
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
                          {activeRepos.length > 0 && (
                            <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                              {activeRepos.length} {activeRepos.length === 1 ? 'repo' : 'repos'}
                            </span>
                          )}
                        </div>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          {integration.provider}
                          {integration.account_login ? ` · ${integration.account_login}` : ''}
                          {integration.last_synced_at ? ` · synced ${new Date(integration.last_synced_at).toLocaleString()}` : ''}
                        </p>
                        {integration.last_sync_error && (
                          <p className="mt-1 text-xs text-destructive">{integration.last_sync_error}</p>
                        )}
                      </div>
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className="gap-1.5"
                          disabled={syncingIntegrationId === integration.id || disconnectingIntegrationId === integration.id}
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
                            ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                            : <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
                          }
                          {syncingIntegrationId === integration.id ? 'Syncing...' : 'Sync'}
                        </Button>
                        {editable && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                            disabled={disconnectingIntegrationId === integration.id || syncingIntegrationId === integration.id}
                            onClick={async () => {
                              const { data, error } = await gitService.getIntegration(workspaceId, integration.id);
                              if (error || !data) {
                                toast.error(error || 'Failed to load integration details');
                                return;
                              }
                              setIntegrationDetail(data);
                              setDisconnectDialogOpen(true);
                            }}
                          >
                            {disconnectingIntegrationId === integration.id
                              ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                              : <Delete01Icon className="h-3.5 w-3.5" />
                            }
                            {disconnectingIntegrationId === integration.id ? 'Disconnecting...' : 'Disconnect'}
                          </Button>
                        )}
                      </div>
                    </div>

                    {/* Repository list for this integration */}
                    {activeRepos.length > 0 && (
                      <div className="border-t border-border/60 px-4 py-3">
                        <p className="mb-2 text-xs font-medium text-muted-foreground">Repositories with access</p>
                        <div className="flex flex-wrap gap-1.5">
                          {activeRepos.map((repo) => (
                            <a
                              key={repo.id}
                              href={`https://github.com/${repo.full_name}`}
                              target="_blank"
                              rel="noreferrer"
                              className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-muted/50 px-2 py-1 text-xs"
                            >
                              <GitBranchIcon className="h-3 w-3 text-muted-foreground" />
                              <span className="font-medium">{repo.full_name}</span>
                              <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0 text-muted-foreground" />
                              {repo.private && <Badge variant="outline" className="h-4 px-1 text-[9px]">Private</Badge>}
                            </a>
                          ))}
                        </div>
                      </div>
                    )}
                    {activeRepos.length === 0 && integration.last_synced_at && (
                      <div className="border-t border-border/60 px-4 py-3">
                        <p className="text-xs text-muted-foreground">No repositories synced. Click Sync to pull repositories from GitHub.</p>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {repoPickerIntegrationId ? (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardTitle className="text-base">Available GitHub Repositories</CardTitle>
                <CardDescription className="mt-1.5">Choose which repositories this workspace should claim from the shared organization installation.</CardDescription>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5"
                  disabled={loadingAvailableRepos}
                  onClick={() => void loadAvailableRepos(repoPickerIntegrationId)}
                >
                  {loadingAvailableRepos ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />}
                  {loadingAvailableRepos ? 'Refreshing...' : 'Refresh'}
                </Button>
                <Button
                  size="sm"
                  disabled={!editable || wiringRepos || selectedRepoIDs.length === 0}
                  onClick={async () => {
                    setWiringRepos(true);
                    const result = await gitService.wireRepositories(workspaceId, repoPickerIntegrationId, {
                      workspace_id: workspaceId,
                      repo_ids: selectedRepoIDs,
                    });
                    setWiringRepos(false);
                    if (result.error) {
                      if (result.status === 409) {
                        const conflictPayload = result.data as WireGitRepositoriesConflictResponse | null;
                        const workspaceName = conflictPayload?.conflicts?.[0]?.claimed_by_workspace_name;
                        toast.error(workspaceName ? `That repo was just claimed by ${workspaceName}.` : 'That repo was just claimed by another workspace.');
                        await loadAvailableRepos(repoPickerIntegrationId);
                        await loadGitStatus();
                        return;
                      }
                      toast.error(result.error);
                      return;
                    }
                    toast.success(`${selectedRepoIDs.length} ${selectedRepoIDs.length === 1 ? 'repository' : 'repositories'} connected`);
                    setSelectedRepoIDs([]);
                    await loadAvailableRepos(repoPickerIntegrationId);
                    await loadGitStatus();
                  }}
                >
                  {wiringRepos ? 'Connecting...' : `Add Selected${selectedRepoIDs.length ? ` (${selectedRepoIDs.length})` : ''}`}
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {hasAvailableRepos ? (
              <div className="space-y-2">
                {availableRepos.map((repo) => {
                  const claimedByOtherWorkspace = repo.claimed_by && repo.claimed_by.workspace_id !== workspaceId;
                  const claimedByThisWorkspace = repo.claimed_by?.workspace_id === workspaceId;
                  const checked = claimedByThisWorkspace || selectedRepoIDs.includes(repo.external_id);
                  return (
                    <div key={repo.external_id} className="rounded-lg border border-border/60 p-4">
                      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                        <div className="flex min-w-0 items-start gap-3">
                          <Checkbox
                            checked={checked}
                            disabled={!editable || claimedByOtherWorkspace || claimedByThisWorkspace}
                            onCheckedChange={(nextChecked) => {
                              setSelectedRepoIDs((current) => (
                                nextChecked
                                  ? [...current, repo.external_id]
                                  : current.filter((id) => id !== repo.external_id)
                              ));
                            }}
                          />
                          <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-2">
                              <a
                                href={`https://github.com/${repo.full_name}`}
                                target="_blank"
                                rel="noreferrer"
                                className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-primary"
                              >
                                <span>{repo.full_name}</span>
                                <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                              </a>
                              <Badge variant="outline" className="text-[10px]">{repo.private ? 'Private' : 'Public'}</Badge>
                              {repo.archived ? <Badge variant="secondary" className="text-[10px]">Archived</Badge> : null}
                              {claimedByThisWorkspace ? <Badge className="text-[10px]">Connected Here</Badge> : null}
                              {claimedByOtherWorkspace ? <Badge variant="secondary" className="text-[10px]">Claimed by {repo.claimed_by?.workspace_name}</Badge> : null}
                            </div>
                            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                              <span className="font-mono">{repo.default_branch}</span>
                              {claimedByOtherWorkspace ? <span>This repo is already connected to another workspace in this organization.</span> : null}
                            </div>
                          </div>
                        </div>
                        {claimedByThisWorkspace ? (
                          <Button
                            variant="outline"
                            size="sm"
                            className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                            disabled={!editable}
                            onClick={async () => {
                              if (!repo.claimed_by?.repo_id) {
                                return;
                              }
                              const { error } = await gitService.unwireRepository(workspaceId, repoPickerIntegrationId, repo.claimed_by.repo_id);
                              if (error) {
                                toast.error(error);
                                return;
                              }
                              toast.success(`${repo.full_name} disconnected from this workspace`);
                              await loadAvailableRepos(repoPickerIntegrationId);
                              await loadGitStatus();
                            }}
                          >
                            <Delete01Icon className="h-3.5 w-3.5" />
                            Disconnect
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                {loadingAvailableRepos ? 'Loading repositories...' : 'No GitHub repositories are available for this installation.'}
              </div>
            )}
          </CardContent>
        </Card>
      ) : null}

      {/* ── Repository Catalog ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitPullRequestIcon className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Repository Catalog</CardTitle>
            {hasRepositories && (
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {repositories.length}
              </span>
            )}
          </div>
          <CardDescription>Choose which synced repositories are available to teams and task delivery targets.</CardDescription>
        </CardHeader>
        <CardContent>
          {hasRepositories ? (
            <div className="space-y-2">
              {repositories.map((repo) => (
                <div key={repo.id} className="rounded-lg border border-border/60 p-4">
                  <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <a
                          href={`https://github.com/${repo.full_name}`}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-primary"
                        >
                          <span>{repo.full_name}</span>
                          <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                        </a>
                        <Badge variant="outline" className="text-[10px]">{repo.private ? 'Private' : 'Public'}</Badge>
                        {repo.archived ? <Badge variant="secondary" className="text-[10px]">Archived</Badge> : null}
                      </div>
                      <a
                        href={`https://github.com/${repo.full_name}/tree/${repo.default_branch}`}
                        target="_blank"
                        rel="noreferrer"
                        className="mt-0.5 inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-primary"
                      >
                        <span className="font-mono">{repo.default_branch}</span>
                        <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                      </a>
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

      <Dialog open={disconnectDialogOpen} onOpenChange={setDisconnectDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Uninstall GitHub App</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              This uninstalls the shared GitHub integration for the whole organization. Every workspace below will lose its repo claims until the app is reinstalled.
            </p>
            {integrationDetail?.affected_workspaces?.length ? (
              <div className="space-y-2 rounded-lg border border-border/60 p-3">
                {integrationDetail.affected_workspaces.map((workspace) => (
                  <div key={workspace.workspace_id} className="flex items-center justify-between text-sm">
                    <span>{workspace.workspace_name}</span>
                    <span className="text-muted-foreground">{workspace.repo_count} {workspace.repo_count === 1 ? 'repo' : 'repos'}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No workspace repo claims are currently attached to this integration.</p>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setDisconnectDialogOpen(false);
                setIntegrationDetail(null);
              }}
              disabled={Boolean(disconnectingIntegrationId)}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={!integrationDetail || Boolean(disconnectingIntegrationId)}
              onClick={async () => {
                if (!integrationDetail) {
                  return;
                }
                setDisconnectingIntegrationId(integrationDetail.integration.id);
                const { error } = await gitService.deleteIntegration(workspaceId, integrationDetail.integration.id);
                setDisconnectingIntegrationId(null);
                if (error) {
                  toast.error(error);
                  return;
                }
                toast.success('GitHub integration uninstalled');
                setDisconnectDialogOpen(false);
                setIntegrationDetail(null);
                await loadGitStatus();
                await refreshInstallAction(true);
              }}
            >
              {disconnectingIntegrationId ? 'Uninstalling...' : 'Uninstall for Organization'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
