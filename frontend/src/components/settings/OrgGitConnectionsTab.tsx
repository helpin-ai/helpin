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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import type { GitLabTokenAuthType } from '@/lib/pmTypes';
import {
  ArrowReloadHorizontalIcon,
  Delete01Icon,
  GitBranchIcon,
  GlobeIcon,
  LinkSquare01Icon,
  Loading01Icon,
  LockIcon,
  Mail01Icon,
  PencilEdit01Icon,
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
  const [gitlabDialogOpen, setGitlabDialogOpen] = useState(false);
  const [gitlabBaseURL, setGitlabBaseURL] = useState('https://gitlab.com');
  const [gitlabToken, setGitlabToken] = useState('');
  const [gitlabAuthType, setGitlabAuthType] = useState<GitLabTokenAuthType>('personal_token');
  const [gitlabLabel, setGitlabLabel] = useState('');
  const [gitlabCommitAuthorName, setGitlabCommitAuthorName] = useState('');
  const [gitlabCommitAuthorEmail, setGitlabCommitAuthorEmail] = useState('');
  const [connectingGitLab, setConnectingGitLab] = useState(false);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [disconnectingIntegrationId, setDisconnectingIntegrationId] = useState<string | null>(null);
  const [disconnectConfirm, setDisconnectConfirm] = useState<GitIntegrationDetail | null>(null);
  const [commitIdentityIntegration, setCommitIdentityIntegration] = useState<GitIntegration | null>(null);
  const [commitAuthorName, setCommitAuthorName] = useState('');
  const [commitAuthorEmail, setCommitAuthorEmail] = useState('');
  const [savingCommitIdentity, setSavingCommitIdentity] = useState(false);

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
    if (!githubStatus) return;

    if (githubStatus === 'connected') {
      toast.success(githubMessage || 'GitHub connected');
    } else {
      toast.error(githubMessage || 'GitHub connection failed');
    }
    url.searchParams.delete('github_app');
    url.searchParams.delete('github_message');
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

  const openGitLabDialog = () => {
    setGitlabBaseURL('https://gitlab.com');
    setGitlabToken('');
    setGitlabAuthType('personal_token');
    setGitlabLabel('');
    setGitlabCommitAuthorName('');
    setGitlabCommitAuthorEmail('');
    setGitlabDialogOpen(true);
  };

  const submitGitLabConnect = async () => {
    if (!organizationId) return;
    if (!gitlabToken.trim()) {
      toast.error('Paste a GitLab access token to continue');
      return;
    }
    setConnectingGitLab(true);
    const { data, error } = await gitService.connectOrgGitLab(organizationId, workspaceId, {
      base_url: gitlabBaseURL.trim(),
      token: gitlabToken.trim(),
      auth_type: gitlabAuthType,
      label: gitlabLabel.trim() || undefined,
      default_commit_author_name: gitlabCommitAuthorName.trim() || undefined,
      default_commit_author_email: gitlabCommitAuthorEmail.trim() || undefined,
    });
    setConnectingGitLab(false);
    if (error || !data) {
      toast.error(error || 'GitLab connection failed');
      return;
    }
    toast.success(data.account_login
      ? `GitLab connected as ${data.account_login}`
      : 'GitLab connected');
    setGitlabDialogOpen(false);
    await loadIntegrations();
  };

  const openCommitIdentityDialog = (integration: GitIntegration) => {
    setCommitIdentityIntegration(integration);
    setCommitAuthorName(integration.default_commit_author_name ?? '');
    setCommitAuthorEmail(integration.default_commit_author_email ?? '');
  };

  const submitCommitIdentityUpdate = async () => {
    if (!organizationId || !commitIdentityIntegration) return;
    setSavingCommitIdentity(true);
    const { data, error } = await gitService.updateOrgIntegration(organizationId, commitIdentityIntegration.id, {
      default_commit_author_name: commitAuthorName.trim(),
      default_commit_author_email: commitAuthorEmail.trim(),
    });
    setSavingCommitIdentity(false);
    if (error || !data) {
      toast.error(error || 'Failed to update commit identity');
      return;
    }
    setIntegrations((items) => items.map((item) => (item.id === data.id ? data : item)));
    setCommitIdentityIntegration(null);
    toast.success('Commit identity updated');
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
              disabled={!organizationId}
              onClick={openGitLabDialog}
            >
              {hasGitLabIntegration ? 'Connect Another GitLab Account' : 'Connect GitLab'}
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
              Organization admins manage provider access. You can still use repositories already enabled for your workspace.
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
                          <div className="mt-2 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                            <Mail01Icon className="h-3.5 w-3.5 shrink-0" />
                            <span className="truncate">
                              {integration.default_commit_author_email
                                ? `${integration.default_commit_author_name || 'Git author'} <${integration.default_commit_author_email}>`
                                : integration.provider === 'gitlab'
                                  ? 'Commit author email required before Forge can push'
                                  : 'Default commit author: Helpin Agent'}
                            </span>
                          </div>
                        </div>
                        {canManage ? (
                          <div className="flex shrink-0 items-center gap-1">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8"
                              aria-label="Edit commit identity"
                              onClick={() => openCommitIdentityDialog(integration)}
                            >
                              <PencilEdit01Icon className="h-4 w-4" />
                            </Button>
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

      <Dialog open={gitlabDialogOpen} onOpenChange={(open) => { if (!connectingGitLab) setGitlabDialogOpen(open); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Connect GitLab</DialogTitle>
            <DialogDescription>
              Paste a GitLab access token. The token must have <code>api</code>, <code>read_repository</code>, and <code>write_repository</code> scopes, plus Maintainer access on the projects you want to connect. Helpin acts as the token's owner for branches, merge requests, and webhooks.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="gitlab-base-url">GitLab URL</Label>
              <Input
                id="gitlab-base-url"
                value={gitlabBaseURL}
                onChange={(e) => setGitlabBaseURL(e.target.value)}
                placeholder="https://gitlab.com"
                disabled={connectingGitLab}
              />
              <p className="text-xs text-muted-foreground">Use https://gitlab.com or your self-hosted GitLab URL.</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="gitlab-auth-type">Token type</Label>
              <Select
                value={gitlabAuthType}
                onValueChange={(value) => setGitlabAuthType(value as GitLabTokenAuthType)}
                disabled={connectingGitLab}
              >
                <SelectTrigger id="gitlab-auth-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="personal_token">Personal Access Token</SelectItem>
                  <SelectItem value="group_token">Group Access Token</SelectItem>
                  <SelectItem value="project_token">Project Access Token</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="gitlab-token">Token</Label>
              <Input
                id="gitlab-token"
                type="password"
                value={gitlabToken}
                onChange={(e) => setGitlabToken(e.target.value)}
                placeholder="glpat-..."
                disabled={connectingGitLab}
                autoComplete="off"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="gitlab-label">Label (optional)</Label>
              <Input
                id="gitlab-label"
                value={gitlabLabel}
                onChange={(e) => setGitlabLabel(e.target.value)}
                placeholder="GitLab acme"
                disabled={connectingGitLab}
              />
            </div>
            <div className="grid gap-3 rounded-md border border-border/70 bg-muted/20 p-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="gitlab-commit-author-name">Commit author name</Label>
                <Input
                  id="gitlab-commit-author-name"
                  value={gitlabCommitAuthorName}
                  onChange={(e) => setGitlabCommitAuthorName(e.target.value)}
                  placeholder="Helpin Agent"
                  disabled={connectingGitLab}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="gitlab-commit-author-email">Commit author email</Label>
                <Input
                  id="gitlab-commit-author-email"
                  type="email"
                  value={gitlabCommitAuthorEmail}
                  onChange={(e) => setGitlabCommitAuthorEmail(e.target.value)}
                  placeholder="verified-user@company.com"
                  disabled={connectingGitLab}
                />
              </div>
              <p className="sm:col-span-2 text-xs text-muted-foreground">
                GitLab push rules may require this email to be verified for the token owner.
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setGitlabDialogOpen(false)} disabled={connectingGitLab}>
              Cancel
            </Button>
            <Button type="button" onClick={() => void submitGitLabConnect()} disabled={connectingGitLab || !gitlabToken.trim()}>
              {connectingGitLab ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
              Connect
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={commitIdentityIntegration !== null}
        onOpenChange={(open) => {
          if (!savingCommitIdentity && !open) setCommitIdentityIntegration(null);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Commit identity</DialogTitle>
            <DialogDescription>
              Default author used when Helpin creates agent commits for this Git connection.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="commit-author-name">Author name</Label>
              <Input
                id="commit-author-name"
                value={commitAuthorName}
                onChange={(e) => setCommitAuthorName(e.target.value)}
                placeholder="Helpin Agent"
                disabled={savingCommitIdentity}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="commit-author-email">Author email</Label>
              <Input
                id="commit-author-email"
                type="email"
                value={commitAuthorEmail}
                onChange={(e) => setCommitAuthorEmail(e.target.value)}
                placeholder="verified-user@company.com"
                disabled={savingCommitIdentity}
              />
              {commitIdentityIntegration?.provider === 'gitlab' ? (
                <p className="text-xs text-muted-foreground">
                  Use an email verified for the GitLab token owner.
                </p>
              ) : null}
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCommitIdentityIntegration(null)} disabled={savingCommitIdentity}>
              Cancel
            </Button>
            <Button type="button" onClick={() => void submitCommitIdentityUpdate()} disabled={savingCommitIdentity}>
              {savingCommitIdentity ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
