import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { gitRepoURL } from '@/lib/gitUrls';
import { gitService } from '@/lib/services/gitService';
import type { GitIntegration, GitIntegrationDetail, GitIntegrationWorkspaceUsage, GitRepository } from '@/lib/pmTypes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import {
  ArrowReloadHorizontalIcon,
  Delete01Icon,
  GitBranchIcon,
  GlobeIcon,
  LinkSquare01Icon,
  Loading01Icon,
  LockIcon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';

type OrgGitConnectionsTabProps = {
  organizationId?: string;
  workspaceId?: string;
  canManage: boolean;
};

export function OrgGitConnectionsTab({ organizationId, workspaceId, canManage }: OrgGitConnectionsTabProps) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [workspaceRepositories, setWorkspaceRepositories] = useState<GitRepository[]>([]);
  const [integrationUsage, setIntegrationUsage] = useState<Record<string, GitIntegrationWorkspaceUsage[]>>({});
  const [loading, setLoading] = useState(false);
  const [setupError, setSetupError] = useState<string | null>(null);
  const [installingGitHub, setInstallingGitHub] = useState(false);
  const [connectingGitLab, setConnectingGitLab] = useState(false);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [disconnectingIntegrationId, setDisconnectingIntegrationId] = useState<string | null>(null);
  const [disconnectConfirm, setDisconnectConfirm] = useState<GitIntegrationDetail | null>(null);

  const hasGitHubIntegration = useMemo(() => integrations.some((item) => item.provider === 'github'), [integrations]);
  const hasGitLabIntegration = useMemo(() => integrations.some((item) => item.provider === 'gitlab'), [integrations]);
  const workspaceReposByIntegration = useMemo(() => {
    const map = new Map<string, GitRepository[]>();
    for (const repo of workspaceRepositories) {
      if (!repo.active || repo.archived || !repo.selected) continue;
      const list = map.get(repo.integration_id) ?? [];
      list.push(repo);
      map.set(repo.integration_id, list);
    }
    return map;
  }, [workspaceRepositories]);

  const loadIntegrations = useCallback(async () => {
    if (!organizationId) return;
    setLoading(true);
    const [integrationsRes, reposRes] = await Promise.all([
      gitService.listOrgIntegrations(organizationId),
      workspaceId
        ? gitService.listRepositories(workspaceId)
        : Promise.resolve({ data: [] as GitRepository[], error: null, status: 200 }),
    ]);
    setLoading(false);
    if (integrationsRes.error) {
      toast.error(integrationsRes.error);
      return;
    }
    const nextIntegrations = integrationsRes.data ?? [];
    setIntegrations(nextIntegrations);
    setWorkspaceRepositories(reposRes.data ?? []);
    const usageEntries = await Promise.all(
      nextIntegrations.map(async (integration) => {
        const { data } = await gitService.getOrgIntegration(organizationId, integration.id);
        return [integration.id, data?.affected_workspaces ?? []] as const;
      }),
    );
    setIntegrationUsage(Object.fromEntries(usageEntries));
  }, [organizationId, workspaceId]);

  useEffect(() => {
    void loadIntegrations();
  }, [loadIntegrations]);

  useEffect(() => {
    const url = new URL(window.location.href);
    const githubStatus = url.searchParams.get('github_app');
    const githubMessage = url.searchParams.get('github_message');
    const gitlabStatus = url.searchParams.get('gitlab_oauth');
    const gitlabMessage = url.searchParams.get('gitlab_message');
    if (!githubStatus && !gitlabStatus) return;

    if (githubStatus === 'connected') {
      toast.success(githubMessage || 'GitHub connected');
    } else if (githubStatus) {
      toast.error(githubMessage || 'GitHub connection failed');
    }
    if (gitlabStatus === 'connected') {
      toast.success(gitlabMessage || 'GitLab connected');
    } else if (gitlabStatus) {
      toast.error(gitlabMessage || 'GitLab connection failed');
    }
    url.searchParams.delete('github_app');
    url.searchParams.delete('github_message');
    url.searchParams.delete('gitlab_oauth');
    url.searchParams.delete('gitlab_message');
    url.searchParams.delete('integration_id');
    window.history.replaceState({}, '', `${url.pathname}${url.search ? url.search : ''}${url.hash}`);
    void loadIntegrations();
  }, [loadIntegrations]);

  const startGitHubConnect = async (forceInstall = false) => {
    if (!organizationId) return;
    setSetupError(null);
    setInstallingGitHub(true);
    const { data, error } = await gitService.getOrgGitHubInstallURL(
      organizationId,
      workspaceId,
      forceInstall ? { forceInstall: true } : undefined,
    );
    setInstallingGitHub(false);
    if (error || !data) {
      const message = error || 'GitHub App install URL is not available';
      setSetupError(message);
      toast.error(message);
      return;
    }
    if (data.action === 'pick_repos' && data.install_url && !forceInstall) {
      window.open(data.install_url, '_blank', 'noopener,noreferrer');
      return;
    }
    if (!data.install_url) {
      const message = 'GitHub App install URL is not available';
      setSetupError(message);
      toast.error(message);
      return;
    }
    window.location.assign(data.install_url);
  };

  const startGitLabConnect = async () => {
    if (!organizationId) return;
    setConnectingGitLab(true);
    const { data, error } = await gitService.getOrgGitLabConnectURL(organizationId, workspaceId);
    setConnectingGitLab(false);
    if (error || !data?.connect_url) {
      toast.error(error || 'GitLab connect URL is not available');
      return;
    }
    window.location.assign(data.connect_url);
  };

  const gitHubAccessURL = (integration: GitIntegration) => {
    if (integration.provider !== 'github' || !integration.installation_id) {
      return null;
    }
    if (integration.account_login) {
      return `https://github.com/organizations/${encodeURIComponent(integration.account_login)}/settings/installations/${encodeURIComponent(integration.installation_id)}`;
    }
    return `https://github.com/settings/installations/${encodeURIComponent(integration.installation_id)}`;
  };

  const repositoryWebURL = (repo: GitRepository, integration: GitIntegration) => {
    return gitRepoURL(integration.provider || repo.provider, repo.full_name, repo.base_url ?? integration.base_url);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h2 className="flex items-center gap-2 text-xl font-semibold">
            <GitBranchIcon className="h-4 w-4" />
            Git Connections
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Organization provider access for repositories used across workspaces.
          </p>
        </div>
        {canManage ? (
          <div className="flex shrink-0 flex-wrap gap-2 md:justify-end">
            {hasGitHubIntegration ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={installingGitHub || !organizationId}
                onClick={() => void startGitHubConnect(true)}
              >
                Connect Another GitHub Org
              </Button>
            ) : (
              <Button
                type="button"
                variant="default"
                size="sm"
                disabled={installingGitHub || !organizationId}
                onClick={() => void startGitHubConnect(false)}
              >
                {installingGitHub ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
                Connect GitHub
              </Button>
            )}
            <Button
              type="button"
              variant={hasGitLabIntegration ? 'outline' : 'default'}
              size="sm"
              disabled={connectingGitLab || !organizationId}
              onClick={() => void startGitLabConnect()}
            >
              {connectingGitLab ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
              {hasGitLabIntegration ? 'Connect Another GitLab Account' : 'Authorize GitLab'}
            </Button>
          </div>
        ) : null}
      </div>

      {!organizationId ? (
        <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
          Select an organization to manage Git connections.
        </div>
      ) : (
        <>
          {setupError ? (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {setupError}
            </div>
          ) : null}
          {!canManage ? (
            <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              Organization admins manage provider access. GitLab.com authorization uses Helpin's OAuth app; you can use repositories already enabled for your workspace.
            </div>
          ) : null}
          {loading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => <Skeleton key={i} className="h-32 w-full" />)}
            </div>
          ) : integrations.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No Git providers are connected to this organization.
            </div>
          ) : (
            <div className="space-y-3">
              {integrations.map((integration) => {
                const providerLabel = integration.provider === 'gitlab' ? 'GitLab' : 'GitHub';
                const manageURL = gitHubAccessURL(integration);
                const workspaceRepos = workspaceReposByIntegration.get(integration.id) ?? [];
                const usage = integrationUsage[integration.id] ?? [];
                const workspaceCount = usage.length;
                const totalRepoCount = usage.reduce((sum, entry) => sum + entry.repo_count, 0);
                return (
                  <Card key={integration.id} className="gap-0 py-0">
                    <CardContent className="px-4 py-3">
                      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className={cn('h-2 w-2 rounded-full', integration.active ? 'bg-emerald-500' : 'bg-muted-foreground/50')} />
                            <p className="truncate text-sm font-medium">{integration.display_name}</p>
                            <Badge variant="outline" className="text-[10px] uppercase tracking-wide">{providerLabel}</Badge>
                          </div>
                          <p className="mt-1 truncate text-xs text-muted-foreground">
                            {integration.account_login || 'Connected account'}
                            {integration.last_synced_at ? ` · synced ${new Date(integration.last_synced_at).toLocaleString()}` : ''}
                          </p>
                          {integration.last_sync_error ? <p className="mt-1 text-xs text-destructive">{integration.last_sync_error}</p> : null}
                        </div>
                        {canManage ? (
                          <div className="flex shrink-0 items-center gap-1">
                            {manageURL ? (
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => window.open(manageURL, '_blank', 'noopener,noreferrer')}
                              >
                                Manage access
                              </Button>
                            ) : null}
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8"
                              disabled={syncingIntegrationId === integration.id}
                              onClick={async () => {
                                setSyncingIntegrationId(integration.id);
                                const { error } = await gitService.syncOrgRepositories(organizationId, integration.id);
                                setSyncingIntegrationId(null);
                                if (error) {
                                  toast.error(error);
                                  return;
                                }
                                toast.success('Git connection synced');
                                await loadIntegrations();
                              }}
                            >
                              {syncingIntegrationId === integration.id
                                ? <Loading01Icon className="h-4 w-4 animate-spin" />
                                : <ArrowReloadHorizontalIcon className="h-4 w-4" />}
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                              disabled={disconnectingIntegrationId === integration.id}
                              onClick={async () => {
                                const { data, error } = await gitService.getOrgIntegration(organizationId, integration.id);
                                if (error || !data) {
                                  toast.error(error || 'Failed to load Git connection');
                                  return;
                                }
                                setDisconnectConfirm(data);
                              }}
                            >
                              {disconnectingIntegrationId === integration.id
                                ? <Loading01Icon className="h-4 w-4 animate-spin" />
                                : <Delete01Icon className="h-4 w-4" />}
                            </Button>
                          </div>
                        ) : null}
                      </div>
                      <div className="mt-2 border-t border-border/60 pt-2">
                        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                          <span>
                            Used in {workspaceCount} {workspaceCount === 1 ? 'workspace' : 'workspaces'}
                            {totalRepoCount ? ` · ${totalRepoCount} enabled ${totalRepoCount === 1 ? 'repo' : 'repos'}` : ''}
                          </span>
                        </div>
                        {workspaceRepos.length > 0 ? (
                          <div className="mt-2 space-y-1">
                            {workspaceRepos.slice(0, 5).map((repo) => (
                              <a
                                key={repo.id}
                                href={repositoryWebURL(repo, integration)}
                                target="_blank"
                                rel="noreferrer"
                                className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto_auto_auto] items-center gap-2 rounded-sm px-1 py-1 text-xs text-foreground transition-colors hover:bg-muted/40"
                              >
                                <span className="shrink-0 font-mono text-muted-foreground">|-</span>
                                <span className="truncate font-medium">{repo.full_name}</span>
                                <span className="shrink-0 font-mono text-muted-foreground">{repo.default_branch}</span>
                                {repo.private
                                  ? <LockIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                  : <GlobeIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />}
                                <LinkSquare01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
                              </a>
                            ))}
                            {workspaceRepos.length > 5 ? (
                              <p className="px-1 pt-1 text-xs text-muted-foreground">
                                +{workspaceRepos.length - 5} more in this workspace
                              </p>
                            ) : null}
                          </div>
                        ) : (
                          <p className="mt-2 text-xs text-muted-foreground">
                            No enabled repositories from this connection in the current workspace.
                          </p>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          )}
          {workspaceSlug ? (
            <div className="flex justify-end">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void navigate({ to: '/w/$slug/settings/repositories', params: { slug: workspaceSlug } })}
              >
                Manage workspace repositories
              </Button>
            </div>
          ) : null}
        </>
      )}

      <ConfirmDialog
        open={disconnectConfirm !== null}
        onOpenChange={(open) => { if (!open) setDisconnectConfirm(null); }}
        title="Disconnect Git provider"
        description={
          disconnectConfirm?.affected_workspaces?.length
            ? `This will disconnect ${disconnectConfirm.integration.display_name} for the organization and remove repository access from ${disconnectConfirm.affected_workspaces.length} workspace${disconnectConfirm.affected_workspaces.length === 1 ? '' : 's'}.`
            : `This will disconnect ${disconnectConfirm?.integration.display_name ?? 'this Git provider'} for the organization.`
        }
        confirmLabel="Disconnect"
        variant="destructive"
        onConfirm={async () => {
          if (!disconnectConfirm || !organizationId) return;
          const integrationID = disconnectConfirm.integration.id;
          setDisconnectingIntegrationId(integrationID);
          const { error } = await gitService.deleteOrgIntegration(organizationId, integrationID, workspaceId);
          setDisconnectingIntegrationId(null);
          if (error) {
            toast.error(error);
            return;
          }
          toast.success('Git provider disconnected');
          setDisconnectConfirm(null);
          await loadIntegrations();
        }}
      />
    </div>
  );
}
