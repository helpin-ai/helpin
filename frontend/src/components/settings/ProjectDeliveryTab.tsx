import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { gitService } from '@/lib/services/gitService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type {
  GitAvailableRepo,
  GitIntegration,
  GitIntegrationDetail,
  GitRepository,
  WireGitRepositoriesConflictResponse,
} from '@/lib/pmTypes';
import type { TeamRepoDefault, WorkspaceTeam } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  GitBranchIcon,
  GitPullRequestIcon,
  Loading01Icon,
  PlusSignIcon,
  ArrowReloadHorizontalIcon,
  Delete01Icon,
  LinkSquare01Icon,
  CheckmarkCircle02Icon,
  LockIcon,
  GlobeIcon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function ProjectDeliveryTab({ workspaceId, editable, teams = [], teamRepoDefaults = [] }: {
  workspaceId: string;
  editable: boolean;
  teams?: WorkspaceTeam[];
  teamRepoDefaults?: TeamRepoDefault[];
}) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [disconnectingIntegrationId, setDisconnectingIntegrationId] = useState<string | null>(null);
  const [installingGitHubApp, setInstallingGitHubApp] = useState(false);
  const [installActionError, setInstallActionError] = useState<string | null>(null);
  const [installAction, setInstallAction] = useState<'install' | 'pick_repos'>('install');
  const [manageGitHubAccessURL, setManageGitHubAccessURL] = useState<string | null>(null);
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
  const githubIntegrations = useMemo(
    () => integrations.filter((integration) => integration.provider === 'github' && Boolean(integration.installation_id)),
    [integrations],
  );

  const reposByIntegration = useMemo(() => {
    const map = new Map<string, GitRepository[]>();
    for (const repo of repositories) {
      const list = map.get(repo.integration_id) ?? [];
      list.push(repo);
      map.set(repo.integration_id, list);
    }
    return map;
  }, [repositories]);

  const hasWorkspaceGitHubConnection = useMemo(() => (
    integrations.some((integration) => integration.provider === 'github' && (
      integration.workspace_id === workspaceId || (reposByIntegration.get(integration.id)?.length ?? 0) > 0
    ))
  ), [integrations, reposByIntegration, workspaceId]);

  const selectedRepoPickerIntegration = useMemo(
    () => githubIntegrations.find((integration) => integration.id === repoPickerIntegrationId) ?? null,
    [githubIntegrations, repoPickerIntegrationId],
  );

  const selectedManageGitHubAccessURL = useMemo(() => {
    if (selectedRepoPickerIntegration?.account_login && selectedRepoPickerIntegration.installation_id) {
      return `https://github.com/organizations/${encodeURIComponent(selectedRepoPickerIntegration.account_login)}/settings/installations/${encodeURIComponent(selectedRepoPickerIntegration.installation_id)}`;
    }
    if (selectedRepoPickerIntegration?.installation_id) {
      return `https://github.com/settings/installations/${encodeURIComponent(selectedRepoPickerIntegration.installation_id)}`;
    }
    return manageGitHubAccessURL;
  }, [manageGitHubAccessURL, selectedRepoPickerIntegration]);

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
      setManageGitHubAccessURL(null);
      setRepoPickerIntegrationId(null);
      if (error) {
        setInstallActionError(error);
      }
      return;
    }
    setInstallActionError(null);
    setInstallAction(data.action);
    setManageGitHubAccessURL(data.install_url || null);
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
    if (repoPickerIntegrationId || githubIntegrations.length === 0) {
      return;
    }
    void loadAvailableRepos(githubIntegrations[0].id);
  }, [githubIntegrations, loadAvailableRepos, repoPickerIntegrationId]);

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

  const installButtonLabel = installingGitHubApp
    ? 'Opening GitHub...'
    : hasWorkspaceGitHubConnection
      ? installAction === 'pick_repos'
        ? 'Pick repositories'
        : hasGitHubAppIntegration
          ? 'Manage access'
          : hasIntegrations
            ? 'Add integration'
            : 'Install GitHub App'
      : 'Connect GitHub';

  const startGitHubInstall = useCallback(async (forceInstall = false) => {
    setInstallActionError(null);
    setInstallingGitHubApp(true);
    const { data, error } = await gitService.getGitHubInstallURL(workspaceId, forceInstall ? { forceInstall: true } : undefined);
    setInstallingGitHubApp(false);
    if (error || !data) {
      const message = error || 'GitHub App install URL is not available';
      setInstallActionError(message);
      toast.error(message);
      return;
    }
    setInstallAction(data.action);
    setManageGitHubAccessURL(data.install_url || null);
    if (data.action === 'pick_repos' && data.integration_id && !forceInstall) {
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
  }, [loadAvailableRepos, workspaceId]);

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
              <CardDescription className="mt-1.5">Connect GitHub, return from the install flow, then choose which repositories this workspace should use.</CardDescription>
            </div>
            {editable ? (
              <div className="flex flex-shrink-0 flex-wrap gap-2">
                {installAction !== 'pick_repos' ? (
                  <Button
                    variant={hasIntegrations ? 'outline' : 'default'}
                    size="sm"
                    disabled={installingGitHubApp}
                    className="gap-1.5"
                    onClick={() => void startGitHubInstall(false)}
                  >
                    {installingGitHubApp ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
                    {installButtonLabel}
                  </Button>
                ) : null}
                {installAction === 'pick_repos' ? (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={installingGitHubApp}
                    onClick={() => void startGitHubInstall(true)}
                  >
                    Connect Another GitHub Org
                  </Button>
                ) : null}
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

          {!installGuidance && installAction === 'pick_repos' && !hasWorkspaceGitHubConnection ? (
            <div className="rounded-lg border border-border/60 bg-muted/30 p-4 text-sm text-muted-foreground">
              A GitHub installation already exists for this organization. Use <span className="font-medium text-foreground">Connect GitHub</span> to choose repositories from an existing install, or <span className="font-medium text-foreground">Connect Another GitHub Org</span> to add a different GitHub organization.
            </div>
          ) : null}

          {/* Connected integrations */}
          {integrations.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No GitHub organizations are connected to this workspace yet.
            </div>
          ) : (
            <div className="space-y-3">
              {integrations.map((integration) => {
                const integrationRepos = reposByIntegration.get(integration.id) ?? [];
                const activeRepos = integrationRepos.filter((r) => !r.archived);
                const connectedHere = integration.workspace_id === workspaceId || activeRepos.length > 0;
                return (
                  <div key={integration.id} className="rounded-lg border border-border/60">
                    {/* Integration header */}
                    <div className="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
                      <div>
                        <div className="flex items-center gap-2">
                          <TooltipProvider delayDuration={200}>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span
                                  className={cn(
                                    'inline-flex h-2 w-2 shrink-0 rounded-full',
                                    integration.active ? 'bg-emerald-500' : 'bg-muted-foreground/40',
                                  )}
                                  aria-label={integration.active ? 'Active' : 'Inactive'}
                                />
                              </TooltipTrigger>
                              <TooltipContent>{integration.active ? 'Active' : 'Inactive'}</TooltipContent>
                            </Tooltip>
                            <p className="font-medium">{integration.display_name}</p>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span
                                  className={cn(
                                    'inline-flex items-center',
                                    connectedHere ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground',
                                  )}
                                  aria-label={connectedHere ? 'Connected here' : 'Available in organization'}
                                >
                                  <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                                </span>
                              </TooltipTrigger>
                              <TooltipContent>
                                {connectedHere ? 'Connected to this workspace' : 'Available in organization — not yet connected here'}
                              </TooltipContent>
                            </Tooltip>
                            {activeRepos.length > 0 ? (
                              <span className="text-xs text-muted-foreground">
                                · {activeRepos.length} {activeRepos.length === 1 ? 'repo' : 'repos'}
                              </span>
                            ) : null}
                          </TooltipProvider>
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
                      <TooltipProvider delayDuration={200}>
                        <div className="flex items-center gap-1">
                          {integration.provider === 'github' && integration.installation_id ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8 text-muted-foreground hover:text-foreground"
                                  disabled={loadingAvailableRepos}
                                  aria-label="Pick repositories"
                                  onClick={async () => {
                                    await loadAvailableRepos(integration.id);
                                    toast.success(connectedHere ? 'Choose more repositories for this workspace' : 'Choose repositories from this GitHub organization');
                                  }}
                                >
                                  <GitBranchIcon className="h-4 w-4" />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>Pick repositories</TooltipContent>
                            </Tooltip>
                          ) : null}
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 text-muted-foreground hover:text-foreground"
                                disabled={syncingIntegrationId === integration.id || disconnectingIntegrationId === integration.id}
                                aria-label="Sync"
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
                                  ? <Loading01Icon className="h-4 w-4 animate-spin" />
                                  : <ArrowReloadHorizontalIcon className="h-4 w-4" />
                                }
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>{syncingIntegrationId === integration.id ? 'Syncing...' : 'Sync repositories'}</TooltipContent>
                          </Tooltip>
                          {editable && (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                                  disabled={disconnectingIntegrationId === integration.id || syncingIntegrationId === integration.id}
                                  aria-label="Disconnect"
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
                                    ? <Loading01Icon className="h-4 w-4 animate-spin" />
                                    : <Delete01Icon className="h-4 w-4" />
                                  }
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>{disconnectingIntegrationId === integration.id ? 'Disconnecting...' : 'Disconnect'}</TooltipContent>
                            </Tooltip>
                          )}
                        </div>
                      </TooltipProvider>
                    </div>

                    {/* Repository list for this integration */}
                    {activeRepos.length > 0 && (
                      <div className="border-t border-border/60 px-4 py-3">
                        <p className="mb-2 text-xs font-medium text-muted-foreground">Repositories with access</p>
                        <TooltipProvider delayDuration={200}>
                          <div className="flex flex-wrap gap-1.5">
                            {activeRepos.map((repo) => (
                              <a
                                key={repo.id}
                                href={`https://github.com/${repo.full_name}`}
                                target="_blank"
                                rel="noreferrer"
                                className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-muted/40 px-2 py-1 text-xs transition-colors hover:bg-muted"
                              >
                                <GitBranchIcon className="h-3 w-3 text-muted-foreground" />
                                <span className="font-medium">{repo.full_name}</span>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span
                                      className="inline-flex text-muted-foreground"
                                      aria-label={repo.private ? 'Private repository' : 'Public repository'}
                                    >
                                      {repo.private
                                        ? <LockIcon className="h-3 w-3" />
                                        : <GlobeIcon className="h-3 w-3" />
                                      }
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>{repo.private ? 'Private' : 'Public'}</TooltipContent>
                                </Tooltip>
                                <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0 text-muted-foreground" />
                              </a>
                            ))}
                          </div>
                        </TooltipProvider>
                      </div>
                    )}
                    {activeRepos.length === 0 && integration.last_synced_at && (
                      <div className="border-t border-border/60 px-4 py-3">
                        <p className="text-xs text-muted-foreground">
                          {connectedHere
                            ? 'No repositories synced for this workspace yet. Click Pick repositories to claim repos, or Sync to refresh existing claims.'
                            : 'This GitHub organization is available to your Helpin organization. Click Pick repositories to connect repos to this workspace.'}
                        </p>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {githubIntegrations.length > 0 ? (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardTitle className="text-base">Available GitHub Repositories</CardTitle>
                <CardDescription className="mt-1.5">Choose a GitHub organization, then select which repositories this workspace should claim from that installation.</CardDescription>
                <div className="mt-3 max-w-sm">
                  <Select
                    value={repoPickerIntegrationId ?? undefined}
                    onValueChange={(value) => void loadAvailableRepos(value)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a GitHub organization" />
                    </SelectTrigger>
                    <SelectContent>
                      {githubIntegrations.map((integration) => (
                        <SelectItem key={integration.id} value={integration.id}>
                          {integration.display_name}{integration.account_login ? ` (${integration.account_login})` : ''}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="flex gap-2">
                {selectedManageGitHubAccessURL ? (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => window.open(selectedManageGitHubAccessURL, '_blank', 'noopener,noreferrer')}
                  >
                    Not seeing your repository?
                  </Button>
                ) : null}
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5"
                  disabled={loadingAvailableRepos || !repoPickerIntegrationId}
                  onClick={() => repoPickerIntegrationId ? void loadAvailableRepos(repoPickerIntegrationId) : undefined}
                >
                  {loadingAvailableRepos ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />}
                  {loadingAvailableRepos ? 'Refreshing...' : 'Refresh'}
                </Button>
                <Button
                  size="sm"
                  disabled={!editable || wiringRepos || selectedRepoIDs.length === 0 || !repoPickerIntegrationId}
                  onClick={async () => {
                    if (!repoPickerIntegrationId) {
                      return;
                    }
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
            {repoPickerIntegrationId && hasAvailableRepos ? (
              <div className="space-y-2">
                {availableRepos.map((repo) => {
                  const claimedByOtherWorkspace = repo.claimed_by && repo.claimed_by.workspace_id !== workspaceId;
                  const claimedByThisWorkspace = repo.claimed_by?.workspace_id === workspaceId;
                  const checked = claimedByThisWorkspace || selectedRepoIDs.includes(repo.external_id);
                  return (
                    <div key={repo.external_id} className="rounded-lg border border-border/60 p-4">
                      <TooltipProvider delayDuration={200}>
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
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span
                                      className="inline-flex text-muted-foreground"
                                      aria-label={repo.private ? 'Private repository' : 'Public repository'}
                                    >
                                      {repo.private
                                        ? <LockIcon className="h-3.5 w-3.5" />
                                        : <GlobeIcon className="h-3.5 w-3.5" />
                                      }
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>{repo.private ? 'Private' : 'Public'}</TooltipContent>
                                </Tooltip>
                                {repo.archived ? (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="text-[11px] uppercase tracking-wide text-muted-foreground/80">Archived</span>
                                    </TooltipTrigger>
                                    <TooltipContent>Archived on GitHub</TooltipContent>
                                  </Tooltip>
                                ) : null}
                                {claimedByThisWorkspace ? (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span
                                        className="inline-flex items-center text-emerald-600 dark:text-emerald-400"
                                        aria-label="Connected to this workspace"
                                      >
                                        <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                                      </span>
                                    </TooltipTrigger>
                                    <TooltipContent>Connected to this workspace</TooltipContent>
                                  </Tooltip>
                                ) : null}
                                {claimedByOtherWorkspace ? (
                                  <span className="text-xs text-muted-foreground">
                                    Claimed by <span className="text-foreground/80">{repo.claimed_by?.workspace_name}</span>
                                  </span>
                                ) : null}
                              </div>
                              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                                <span className="font-mono">{repo.default_branch}</span>
                                {claimedByOtherWorkspace ? <span>This repo is already connected to another workspace in this organization.</span> : null}
                              </div>
                            </div>
                          </div>
                          {claimedByThisWorkspace ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                                  disabled={!editable}
                                  aria-label="Disconnect repository"
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
                                  <Delete01Icon className="h-4 w-4" />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>Disconnect from workspace</TooltipContent>
                            </Tooltip>
                          ) : null}
                        </div>
                      </TooltipProvider>
                    </div>
                  );
                })}
              </div>
            ) : !repoPickerIntegrationId ? (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                Select a GitHub organization to view its repositories.
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                {loadingAvailableRepos ? 'Loading repositories...' : 'No GitHub repositories are available for this installation.'}
                {!loadingAvailableRepos && selectedManageGitHubAccessURL ? (
                  <div className="mt-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => window.open(selectedManageGitHubAccessURL, '_blank', 'noopener,noreferrer')}
                    >
                      Not seeing your repository? Add it in GitHub
                    </Button>
                  </div>
                ) : null}
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
              <span className="text-xs text-muted-foreground">· {repositories.length}</span>
            )}
          </div>
          <CardDescription>Choose which synced repositories are available to teams and task delivery targets.</CardDescription>
        </CardHeader>
        <CardContent>
          {hasRepositories ? (
            <TooltipProvider delayDuration={200}>
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
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                className="inline-flex text-muted-foreground"
                                aria-label={repo.private ? 'Private repository' : 'Public repository'}
                              >
                                {repo.private
                                  ? <LockIcon className="h-3.5 w-3.5" />
                                  : <GlobeIcon className="h-3.5 w-3.5" />
                                }
                              </span>
                            </TooltipTrigger>
                            <TooltipContent>{repo.private ? 'Private' : 'Public'}</TooltipContent>
                          </Tooltip>
                          {repo.archived ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="text-[11px] uppercase tracking-wide text-muted-foreground/80">Archived</span>
                              </TooltipTrigger>
                              <TooltipContent>Archived on GitHub</TooltipContent>
                            </Tooltip>
                          ) : null}
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
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <div className="flex items-center gap-2">
                            <span className="text-xs text-muted-foreground">Delivery</span>
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
                        </TooltipTrigger>
                        <TooltipContent>
                          {repo.archived
                            ? 'Archived repositories cannot be used for delivery'
                            : repo.selected
                              ? 'Available as a task delivery target — toggle off to hide'
                              : 'Hidden from delivery targets — toggle on to enable'}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                ))}
              </div>
            </TooltipProvider>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              {hasIntegrations
                ? 'No repositories synced yet. Use the Sync button on your integration above to pull in repositories.'
                : 'Repositories will appear here after you connect a GitHub integration and sync.'}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Per-team Delivery Defaults ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitBranchIcon className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Team delivery defaults</CardTitle>
          </div>
          <CardDescription>
            Each team can pin a default repository, base branch, and branch template for new tasks. Task-level overrides still take precedence.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {teams.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No teams in this workspace yet. Create a team in Workspace Settings → Teams to assign delivery defaults.
            </div>
          ) : (
            <ul className="divide-y divide-border/60 rounded-lg border border-border/60">
              {teams.map((team) => {
                const teamDefault = teamRepoDefaults.find((entry) => entry.team_id === team.id);
                const repo = teamDefault
                  ? repositories.find((entry) => entry.id === teamDefault.repository_id)
                  : null;
                const meta = teamDefault
                  ? `${repo?.full_name ?? 'Repository selected'} · base ${teamDefault.base_branch}`
                  : 'Not configured';
                return (
                  <li key={team.id} className="flex items-center justify-between gap-3 px-4 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{team.name}</p>
                      <p className="truncate text-xs text-muted-foreground">{meta}</p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      {teamDefault ? (
                        <Badge variant="outline" className="border-emerald-500/40 bg-emerald-500/10 text-[10px] uppercase tracking-wide text-emerald-700 dark:text-emerald-400">
                          Configured
                        </Badge>
                      ) : null}
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={!editable || !workspaceSlug}
                        onClick={() => {
                          if (!workspaceSlug) return;
                          void navigate({
                            to: '/w/$slug/settings/teams',
                            params: { slug: workspaceSlug },
                            search: { team: team.id, section: 'delivery' },
                          });
                        }}
                      >
                        {teamDefault ? 'Edit' : 'Configure'}
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ul>
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
