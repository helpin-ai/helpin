import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { gitBranchURL, gitRepoURL } from '@/lib/gitUrls';
import { gitService } from '@/lib/services/gitService';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type {
  GitAvailableRepo,
  GitIntegration,
  GitRepository,
  WireGitRepositoriesConflictResponse,
} from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  ArrowReloadHorizontalIcon,
  CheckmarkCircle02Icon,
  Delete01Icon,
  GitBranchIcon,
  GitPullRequestIcon,
  GlobeIcon,
  LinkSquare01Icon,
  Loading01Icon,
  LockIcon,
  Search01Icon,
} from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { LINEAR_CARD_CLASS } from './settingsConstants';

type WorkspaceRepositoriesTabProps = {
  workspaceId: string;
  editable: boolean;
};

export function WorkspaceRepositoriesTab({ workspaceId, editable }: WorkspaceRepositoriesTabProps) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const currentOrganization = useOrganizationStore((state) => state.currentOrganization);
  const canManageOrgGit = currentOrganization?.role === 'owner' || currentOrganization?.role === 'admin';
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [repoPickerIntegrationId, setRepoPickerIntegrationId] = useState<string | null>(null);
  const [availableRepos, setAvailableRepos] = useState<GitAvailableRepo[]>([]);
  const [selectedRepoIDs, setSelectedRepoIDs] = useState<string[]>([]);
  const [loadingAvailableRepos, setLoadingAvailableRepos] = useState(false);
  const [wiringRepos, setWiringRepos] = useState(false);
  const [repoSearchInput, setRepoSearchInput] = useState('');
  const [debouncedRepoSearch, setDebouncedRepoSearch] = useState('');
  const availableReposAbortRef = useRef<AbortController | null>(null);

  const hasIntegrations = integrations.length > 0;
  const hasRepositories = repositories.length > 0;
  const hasAvailableRepos = availableRepos.length > 0;
  const repoProviderIntegrations = useMemo(
    () => integrations.filter((integration) => (
      (integration.provider === 'github' && Boolean(integration.installation_id))
      || (integration.provider === 'gitlab' && Boolean(integration.credential_id))
    )),
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

  const selectedRepoPickerIntegration = useMemo(
    () => repoProviderIntegrations.find((integration) => integration.id === repoPickerIntegrationId) ?? null,
    [repoProviderIntegrations, repoPickerIntegrationId],
  );

  const providerLabel = useCallback((provider?: string) => (
    provider === 'gitlab' ? 'GitLab' : 'GitHub'
  ), []);

  const navigateToSettingsSection = useCallback((section: 'delivery' | 'git-connections') => {
    if (!workspaceSlug) return;
    void navigate({ to: '/w/$slug/settings/$section', params: { slug: workspaceSlug, section } });
  }, [navigate, workspaceSlug]);

  const loadAvailableRepos = useCallback(async (
    integrationId: string,
    options?: { search?: string; noCache?: boolean },
  ) => {
    availableReposAbortRef.current?.abort();
    const controller = new AbortController();
    availableReposAbortRef.current = controller;
    setLoadingAvailableRepos(true);
    const { data, error, status } = await gitService.listAvailableRepos(workspaceId, integrationId, {
      search: options?.search,
      noCache: options?.noCache,
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    setLoadingAvailableRepos(false);
    if (error) {
      if (controller.signal.aborted) return;
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

  const defaultPickerIntegrationId = repoProviderIntegrations[0]?.id ?? null;
  useEffect(() => {
    if (repoPickerIntegrationId || !defaultPickerIntegrationId) return;
    void loadAvailableRepos(defaultPickerIntegrationId);
    // intentionally depend only on the primitive id to avoid re-firing when
    // integrations re-fetch and produce a new array reference.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultPickerIntegrationId]);

  // Debounce the search input.
  useEffect(() => {
    const handle = setTimeout(() => setDebouncedRepoSearch(repoSearchInput.trim()), 300);
    return () => clearTimeout(handle);
  }, [repoSearchInput]);

  // Re-fetch when the debounced search changes (after a picker integration is selected).
  useEffect(() => {
    if (!repoPickerIntegrationId) return;
    void loadAvailableRepos(repoPickerIntegrationId, { search: debouncedRepoSearch || undefined });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedRepoSearch]);

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <GitBranchIcon className="h-4 w-4 text-muted-foreground" />
                <CardTitle className="text-base">Provider access</CardTitle>
              </div>
              <CardDescription className="mt-1.5">
                These organization connections feed the repository catalog for Projects, Automations, and Agents.
              </CardDescription>
            </div>
            {workspaceSlug ? (
              <Button type="button" variant="outline" size="sm" onClick={() => navigateToSettingsSection('git-connections')}>
                {canManageOrgGit ? 'Manage Git connections' : 'View Git connections'}
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {integrations.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              {canManageOrgGit
                ? "Connect GitHub or authorize GitLab first, then return here to enable repositories for this workspace."
                : "No Git providers are connected yet. An organization admin can connect GitHub or authorize Helpin's GitLab app."}
            </div>
          ) : (
            <div className="divide-y divide-border/60 rounded-lg border border-border/60">
              {integrations.map((integration) => {
                const integrationRepos = reposByIntegration.get(integration.id) ?? [];
                const activeRepos = integrationRepos.filter((repo) => !repo.archived);
                return (
                  <div key={integration.id} className="flex flex-col gap-3 px-4 py-3 md:flex-row md:items-center md:justify-between">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="inline-flex h-2 w-2 shrink-0 rounded-full bg-emerald-500" />
                        <p className="truncate text-sm font-medium">{integration.display_name}</p>
                        <Badge variant="outline" className="text-[10px] uppercase tracking-wide">{providerLabel(integration.provider)}</Badge>
                      </div>
                      <p className="mt-1 truncate text-xs text-muted-foreground">
                        {integration.account_login || 'Connected account'}
                        {integration.last_synced_at ? ` · synced ${new Date(integration.last_synced_at).toLocaleString()}` : ''}
                      </p>
                    </div>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {activeRepos.length} enabled {activeRepos.length === 1 ? 'repository' : 'repositories'}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {repoProviderIntegrations.length > 0 ? (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardTitle className="text-base">Add repositories to this workspace</CardTitle>
                <CardDescription className="mt-1.5">Choose a Git connection, then enable repositories for shared workspace use.</CardDescription>
                <div className="mt-3 max-w-sm">
                  <Select
                    value={repoPickerIntegrationId ?? undefined}
                    onValueChange={(value) => {
                      setRepoSearchInput('');
                      setDebouncedRepoSearch('');
                      void loadAvailableRepos(value);
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a Git connection" />
                    </SelectTrigger>
                    <SelectContent>
                      {repoProviderIntegrations.map((integration) => (
                        <SelectItem key={integration.id} value={integration.id}>
                          {integration.display_name}{integration.account_login ? ` (${integration.account_login})` : ''}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {selectedRepoPickerIntegration?.provider === 'gitlab' ? (
                  <div className="relative mt-3 max-w-sm">
                    <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      value={repoSearchInput}
                      onChange={(e) => setRepoSearchInput(e.target.value)}
                      placeholder="Search GitLab projects by name"
                      className="h-8 pl-8 text-xs"
                    />
                  </div>
                ) : null}
              </div>
              <div className="flex gap-2">
                {canManageOrgGit && workspaceSlug ? (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => navigateToSettingsSection('git-connections')}
                  >
                    Manage access
                  </Button>
                ) : null}
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5"
                  disabled={loadingAvailableRepos || !repoPickerIntegrationId}
                  onClick={() => repoPickerIntegrationId
                    ? void loadAvailableRepos(repoPickerIntegrationId, { search: debouncedRepoSearch || undefined, noCache: true })
                    : undefined}
                >
                  {loadingAvailableRepos ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />}
                  {loadingAvailableRepos ? 'Refreshing...' : 'Refresh'}
                </Button>
                <Button
                  size="sm"
                  disabled={!editable || wiringRepos || selectedRepoIDs.length === 0 || !repoPickerIntegrationId}
                  onClick={async () => {
                    if (!repoPickerIntegrationId) return;
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
                        toast.error(workspaceName ? `That repo was just enabled by ${workspaceName}.` : 'That repo was just enabled by another workspace.');
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
                  const claimedByThisWorkspace = repo.claimed_by?.workspace_id === workspaceId;
                  const checked = claimedByThisWorkspace || selectedRepoIDs.includes(repo.external_id);
                  return (
                    <div key={repo.external_id} className="rounded-lg border border-border/60 p-4">
                      <TooltipProvider>
                        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                          <div className="flex min-w-0 items-start gap-3">
                            <Checkbox
                              checked={checked}
                              disabled={!editable || claimedByThisWorkspace}
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
                                  href={gitRepoURL(
                                    selectedRepoPickerIntegration?.provider,
                                    repo.full_name,
                                    selectedRepoPickerIntegration?.base_url,
                                  )}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-primary"
                                >
                                  <span>{repo.full_name}</span>
                                  <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                                </a>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span className="inline-flex text-muted-foreground" aria-label={repo.private ? 'Private repository' : 'Public repository'}>
                                      {repo.private ? <LockIcon className="h-3.5 w-3.5" /> : <GlobeIcon className="h-3.5 w-3.5" />}
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>{repo.private ? 'Private' : 'Public'}</TooltipContent>
                                </Tooltip>
                                {repo.archived ? (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="text-[11px] uppercase tracking-wide text-muted-foreground/80">Archived</span>
                                    </TooltipTrigger>
                                    <TooltipContent>Archived on {providerLabel(selectedRepoPickerIntegration?.provider)}</TooltipContent>
                                  </Tooltip>
                                ) : null}
                                {claimedByThisWorkspace ? (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="inline-flex items-center text-emerald-600 dark:text-emerald-400" aria-label="Connected to this workspace">
                                        <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                                      </span>
                                    </TooltipTrigger>
                                    <TooltipContent>Connected to this workspace</TooltipContent>
                                  </Tooltip>
                                ) : null}
                              </div>
                              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                                <span className="font-mono">{repo.default_branch}</span>
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
                                    if (!repo.claimed_by?.repo_id) return;
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
                {selectedRepoPickerIntegration?.provider === 'gitlab'
                  && availableRepos.length >= (debouncedRepoSearch ? 300 : 100) ? (
                  <p className="px-1 pt-1 text-xs text-muted-foreground">
                    Showing the first {availableRepos.length} projects. {debouncedRepoSearch ? 'Refine your search to narrow further.' : 'Type above to find more.'}
                  </p>
                ) : null}
              </div>
            ) : !repoPickerIntegrationId ? (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                Select a Git connection to view its repositories.
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                {loadingAvailableRepos ? 'Loading repositories...' : 'No repositories are available for this connection.'}
                {!loadingAvailableRepos && canManageOrgGit && workspaceSlug ? (
                  <div className="mt-4">
                    <Button variant="outline" size="sm" onClick={() => navigateToSettingsSection('git-connections')}>
                      Manage access
                    </Button>
                  </div>
                ) : null}
              </div>
            )}
          </CardContent>
        </Card>
      ) : null}

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitPullRequestIcon className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Repository Catalog</CardTitle>
            {hasRepositories ? <span className="text-xs text-muted-foreground">· {repositories.length}</span> : null}
          </div>
          <CardDescription>Repositories enabled here are available to Projects, Automations, and Agents.</CardDescription>
        </CardHeader>
        <CardContent>
          {hasRepositories ? (
            <TooltipProvider>
              <div className="space-y-2">
                {repositories.map((repo) => {
                  const integration = integrations.find((item) => item.id === repo.integration_id);
                  const provider = integration?.provider ?? repo.provider;
                  const baseURL = repo.base_url ?? integration?.base_url;
                  return (
                    <div key={repo.id} className="rounded-lg border border-border/60 p-4">
                      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                        <div>
                          <div className="flex flex-wrap items-center gap-2">
                            <a
                              href={gitRepoURL(provider, repo.full_name, baseURL)}
                              target="_blank"
                              rel="noreferrer"
                              className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-primary"
                            >
                              <span>{repo.full_name}</span>
                              <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                            </a>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="inline-flex text-muted-foreground" aria-label={repo.private ? 'Private repository' : 'Public repository'}>
                                  {repo.private ? <LockIcon className="h-3.5 w-3.5" /> : <GlobeIcon className="h-3.5 w-3.5" />}
                                </span>
                              </TooltipTrigger>
                              <TooltipContent>{repo.private ? 'Private' : 'Public'}</TooltipContent>
                            </Tooltip>
                            {repo.archived ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <span className="text-[11px] uppercase tracking-wide text-muted-foreground/80">Archived</span>
                                </TooltipTrigger>
                                <TooltipContent>Archived on {providerLabel(integration?.provider)}</TooltipContent>
                              </Tooltip>
                            ) : null}
                          </div>
                          <a
                            href={gitBranchURL(provider, repo.full_name, repo.default_branch, baseURL)}
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
                              <span className="text-xs text-muted-foreground">Available</span>
                              <Switch
                                checked={repo.selected}
                                disabled={!editable || repo.archived}
                                onCheckedChange={async (checked) => {
                                  const { error } = await gitService.updateRepository(workspaceId, repo.id, { selected: checked });
                                  if (error) {
                                    toast.error(error);
                                    return;
                                  }
                                  toast.success(`${repo.full_name} ${checked ? 'enabled' : 'hidden'} for workspace use`);
                                  await loadGitStatus();
                                }}
                              />
                            </div>
                          </TooltipTrigger>
                          <TooltipContent>
                            {repo.archived
                              ? 'Archived repositories cannot be used'
                              : repo.selected
                                ? 'Available to Projects, Automations, and Agents'
                                : 'Hidden from Projects, Automations, and Agents'}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </div>
                  );
                })}
              </div>
            </TooltipProvider>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              {hasIntegrations
                ? 'No repositories are enabled for this workspace yet. Add repositories from the available list above.'
                : "Repositories will appear here after an organization admin connects GitHub or authorizes Helpin's GitLab app."}
            </div>
          )}
          {hasRepositories && workspaceSlug ? (
            <div className="mt-4 flex justify-end">
              <Button type="button" variant="outline" size="sm" onClick={() => navigateToSettingsSection('delivery')}>
                Configure project delivery defaults
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
