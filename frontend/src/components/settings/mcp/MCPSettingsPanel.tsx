import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { ArrowDown01Icon, ArrowRight01Icon, BotIcon, Copy01Icon, Delete01Icon, Key01Icon, Loading01Icon, LockIcon } from '@/lib/icons';
import { useConfirm } from '@/components/ui/confirm-dialog';
import {
  useCreateMCPServicePrincipal,
  useMCPActivity,
  useMCPDashboard,
  useRevokeMCPConnection,
  useRevokeMCPWorkspaceAccess,
  useRevokeMCPServicePrincipal,
  useRotateMCPServiceToken,
  useUpdateMCPPolicy,
} from '@/hooks/queries/useMCP';
import type {
  CreateMCPServicePrincipalRequest,
  MCPDashboard,
  MCPServicePrincipal,
  MCPServiceTokenSecret,
  UpdateMCPPolicyRequest,
} from '@/lib/mcpTypes';
import { ensureMCPWriteScopes, hasMCPWriteScope } from '@/lib/mcpPolicy';
import { cn } from '@/lib/utils';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import { ExternalMCPSettings } from './ExternalMCPSettings';

type MCPSettingsPanelProps = {
  workspaceId: string;
  workspaceName: string;
};

const TOOLSET_LABELS: Record<string, string> = {
  context: 'Workspace context',
  pm: 'Projects & tasks',
  docs: 'Docs',
  crm: 'CRM',
  support: 'Support',
  agents: 'Helpin agents',
};

const SCOPE_LABELS: Record<string, string> = {
  'helpin.context.read': 'Read workspace context',
  'helpin.pm.read': 'Read project work',
  'helpin.pm.write': 'Create and update project work',
  'helpin.docs.read': 'Read documents',
  'helpin.docs.write': 'Create and update documents',
  'helpin.crm.read': 'Read CRM records',
  'helpin.crm.write': 'Add notes and update CRM records',
  'helpin.support.read': 'Read support conversations',
  'helpin.agents.read': 'Read agents and runs',
  'helpin.agents.run': 'Start and cancel agent runs',
};

export function MCPSettingsPanel({ workspaceId, workspaceName }: MCPSettingsPanelProps) {
  const initialTab = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('tab') === 'ai-clients'
    ? 'ai-clients'
    : 'external';
  const [section, setSection] = useState(initialTab);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const oauthResult = params.get('external_mcp_oauth');
    if (!oauthResult) return;
    if (oauthResult === 'connected') {
      toast.success(params.get('external_mcp_resume') === 'failed'
        ? 'Server connected. A paused run could not be resumed automatically.'
        : 'External MCP server connected');
    } else {
      toast.error('External MCP connection could not be completed. Check the server status for details.');
    }
    params.delete('external_mcp_oauth');
    params.delete('external_mcp_server_id');
    params.delete('external_mcp_resume');
    const query = params.toString();
    window.history.replaceState({}, '', `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`);
  }, []);

  return (
    <div className="space-y-4">
      <div>
        <div className="flex items-center gap-2">
          <BotIcon className="h-5 w-5 text-muted-foreground" />
          <h2 className="text-xl font-semibold">Model Context Protocol</h2>
        </div>
        <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
          Connect external systems for Helpin agents, or connect outside AI clients to {workspaceName || 'this workspace'}.
        </p>
      </div>
      <Tabs value={section} onValueChange={setSection}>
        <TabsList variant="line" className="max-w-full overflow-x-auto">
          <TabsTrigger value="external">External servers</TabsTrigger>
          <TabsTrigger value="ai-clients">AI clients</TabsTrigger>
        </TabsList>
        <TabsContent value="external" className="mt-4">
          <ExternalMCPSettings workspaceId={workspaceId} workspaceName={workspaceName} />
        </TabsContent>
        <TabsContent value="ai-clients" className="mt-4">
          <InboundMCPSettingsPanel workspaceId={workspaceId} workspaceName={workspaceName} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function InboundMCPSettingsPanel({ workspaceId, workspaceName }: MCPSettingsPanelProps) {
  const dashboardQuery = useMCPDashboard(workspaceId);
  const dashboard = dashboardQuery.data;
  const [tab, setTab] = useState('setup');
  const activityQuery = useMCPActivity(workspaceId, tab === 'activity' && Boolean(dashboard?.can_view_activity));

  if (dashboardQuery.isLoading) {
    return <MCPSettingsSkeleton />;
  }
  if (dashboardQuery.isError || !dashboard) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load MCP settings</AlertTitle>
        <AlertDescription>{dashboardQuery.error?.message ?? 'Try refreshing the page.'}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="space-y-4">
      {!dashboard.platform_enabled ? (
        <Alert>
          <AlertTitle>AI tool connections are not available in this environment</AlertTitle>
          <AlertDescription>The platform rollout switch is off. Existing connections can still be reviewed and revoked.</AlertDescription>
        </Alert>
      ) : !dashboard.policy.enabled ? (
        <Alert>
          <AlertTitle>Connections to AI tools are turned off</AlertTitle>
          <AlertDescription>
            {dashboard.can_manage
              ? 'Turn on AI tool access in the Permissions tab before connecting Claude, ChatGPT, Cursor, VS Code, or another MCP-compatible tool. Helpin roles and workspace permissions still apply.'
              : 'A workspace manager must turn on AI tool access before you can connect Claude, ChatGPT, Cursor, VS Code, or another MCP-compatible tool.'}
          </AlertDescription>
        </Alert>
      ) : null}

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList variant="line" className="max-w-full overflow-x-auto">
          <TabsTrigger value="setup">Setup</TabsTrigger>
          <TabsTrigger value="connections">Connections</TabsTrigger>
          {dashboard.can_manage ? <TabsTrigger value="service-accounts">Automations</TabsTrigger> : null}
          {dashboard.can_view_activity ? <TabsTrigger value="activity">Activity</TabsTrigger> : null}
          {dashboard.can_manage ? <TabsTrigger value="policy">Permissions</TabsTrigger> : null}
        </TabsList>

        <TabsContent value="setup" className="mt-4">
          <MCPSetup dashboard={dashboard} workspaceName={workspaceName} onManagePermissions={dashboard.can_manage ? () => setTab('policy') : undefined} />
        </TabsContent>
        <TabsContent value="connections" className="mt-4">
          <MCPConnections workspaceId={workspaceId} dashboard={dashboard} />
        </TabsContent>
        {dashboard.can_manage ? (
          <TabsContent value="service-accounts" className="mt-4">
            <MCPServiceAccounts workspaceId={workspaceId} dashboard={dashboard} />
          </TabsContent>
        ) : null}
        {dashboard.can_view_activity ? (
          <TabsContent value="activity" className="mt-4">
            <MCPActivity dashboard={dashboard} events={activityQuery.data ?? []} loading={activityQuery.isLoading} />
          </TabsContent>
        ) : null}
        {dashboard.can_manage ? (
          <TabsContent value="policy" className="mt-4">
            <MCPPolicyEditor key={dashboard.policy.updated_at ?? 'default'} workspaceId={workspaceId} dashboard={dashboard} />
          </TabsContent>
        ) : null}
      </Tabs>
    </div>
  );
}

const READ_ONLY_BADGE_CLASS = 'border-emerald-200/70 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300';

function MCPSetup({ dashboard, workspaceName, onManagePermissions }: { dashboard: MCPDashboard; workspaceName: string; onManagePermissions?: () => void }) {
  const [showJson, setShowJson] = useState(false);
  const copy = async (value: string, label: string) => {
    await navigator.clipboard.writeText(value);
    toast.success(label);
  };
  const json = JSON.stringify({ mcpServers: { helpin: { url: dashboard.mcp_url } } }, null, 2);
  const neverConnected = dashboard.connections.length === 0;

  return (
    <div className="space-y-3">
      {neverConnected ? (
        <p className="text-sm text-muted-foreground">No AI tools connected yet. It takes about a minute to set up your first one.</p>
      ) : null}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(280px,1fr)]">
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <CardTitle className="text-base">Connect a tool</CardTitle>
            <CardDescription>Paste this URL into your AI tool's MCP settings. The first time it connects, you'll sign in, pick a workspace, and approve exactly what it can access.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="mcp-url">Connection URL <span className="font-normal text-muted-foreground">(MCP)</span></Label>
              <div className="flex gap-2">
                <Input
                  id="mcp-url"
                  value={dashboard.mcp_url}
                  readOnly
                  className="cursor-pointer font-mono text-xs"
                  title="Click to copy"
                  onClick={() => void copy(dashboard.mcp_url, 'Copied')}
                />
                <Button type="button" variant="outline" size="icon" onClick={() => void copy(dashboard.mcp_url, 'Copied')}>
                  <Copy01Icon className="h-4 w-4" />
                  <span className="sr-only">Copy connection URL</span>
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <Label>Manual configuration</Label>
                  <p className="mt-0.5 text-xs text-muted-foreground">For tools that use a JSON config file.</p>
                </div>
                <div className="flex items-center gap-1">
                  {showJson ? (
                    <Button type="button" variant="ghost" size="sm" onClick={() => void copy(json, 'Configuration copied')}>
                      <Copy01Icon className="mr-1.5 h-3.5 w-3.5" /> Copy
                    </Button>
                  ) : null}
                  <Button type="button" variant="ghost" size="sm" onClick={() => setShowJson((value) => !value)}>
                    {showJson ? <ArrowDown01Icon className="mr-1.5 h-3.5 w-3.5" /> : <ArrowRight01Icon className="mr-1.5 h-3.5 w-3.5" />}
                    {showJson ? 'Hide JSON config' : 'Show JSON config'}
                  </Button>
                </div>
              </div>
              {showJson ? (
                <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs leading-5">{json}</pre>
              ) : null}
            </div>
            <ol className="space-y-2 text-sm text-muted-foreground">
              <li><span className="mr-2 font-medium text-foreground">1.</span>Add the URL to Claude, ChatGPT, Cursor, VS Code, or any MCP-compatible tool.</li>
              <li><span className="mr-2 font-medium text-foreground">2.</span>Sign in and choose a workspace when prompted.</li>
              <li><span className="mr-2 font-medium text-foreground">3.</span>Approve access — everything starts read-only, and you can expand or revoke it anytime.</li>
            </ol>
          </CardContent>
        </Card>

        <Card className={`${LINEAR_CARD_CLASS} lg:self-start`}>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-1.5 text-base">
              <LockIcon className="h-4 w-4 text-muted-foreground" />
              What connected tools can do
            </CardTitle>
            <CardDescription>Tools can never see more than you can. Every request runs with your role and permissions in {workspaceName || 'this workspace'}.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2.5">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">Status</span>
              <Badge variant={dashboard.can_use_mcp ? 'secondary' : 'outline'}>{dashboard.can_use_mcp ? 'Enabled' : 'Disabled'}</Badge>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">Default access</span>
              {dashboard.policy.enforce_read_only ? (
                <Badge variant="outline" className={READ_ONLY_BADGE_CLASS}>Read-only</Badge>
              ) : (
                <Badge variant="outline">Requested access</Badge>
              )}
            </div>
            <div className="pt-0.5">
              <p className="mb-1.5 text-sm text-muted-foreground">Can use</p>
              <div className="flex flex-wrap gap-1.5">
                {dashboard.policy.allowed_toolsets.map((toolset) => (
                  <Badge key={toolset} variant="outline">{TOOLSET_LABELS[toolset] ?? toolset}</Badge>
                ))}
              </div>
              {onManagePermissions ? (
                <button type="button" className="mt-2.5 text-sm text-primary hover:underline" onClick={onManagePermissions}>
                  Manage permissions →
                </button>
              ) : null}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MCPConnections({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const revoke = useRevokeMCPConnection(workspaceId);
  const revokeWorkspace = useRevokeMCPWorkspaceAccess(workspaceId);
  const confirm = useConfirm();
  const policyAllowsWrites = !dashboard.policy.enforce_read_only
    && hasMCPWriteScope(dashboard.policy.allowed_scopes);
  const activeConnectionNeedsReconnect = dashboard.connections.some((connection) => (
    connection.status === 'active'
    && (connection.read_only || !hasMCPWriteScope(connection.scopes))
  ));

  const handleRevoke = async (id: string, name: string) => {
    const accepted = await confirm({
      title: `Revoke ${name}?`,
      description: 'Access and refresh tokens for this connection will stop working immediately.',
      confirmText: 'Revoke connection',
      variant: 'destructive',
    });
    if (!accepted) return;
    try {
      await revoke.mutateAsync(id);
      toast.success('Connection revoked');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not revoke connection');
    }
  };

  const handleRevokeWorkspace = async () => {
    const accepted = await confirm({
      title: 'Revoke all AI tool access?',
      description: 'Every connected tool, sign-in session, automation account, and credential in this workspace will stop working immediately.',
      confirmText: 'Revoke all access',
      variant: 'destructive',
    });
    if (!accepted) return;
    try {
      const result = await revokeWorkspace.mutateAsync();
      toast.success(`Revoked ${result.revoked} active ${result.revoked === 1 ? 'identity' : 'identities'}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not revoke workspace access');
    }
  };

  return (
    <div className="space-y-4">
      {policyAllowsWrites && activeConnectionNeedsReconnect ? (
        <Alert>
          <AlertTitle>Reconnect to use write actions</AlertTitle>
          <AlertDescription>
            Workspace policy now permits bounded writes, but at least one active connection still has a read-only grant. Remove and add that connection again, then turn off Read-only connection and approve the write permissions during consent.
          </AlertDescription>
        </Alert>
      ) : null}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div><CardTitle className="text-base">{dashboard.can_manage ? 'Workspace connections' : 'My connections'}</CardTitle><CardDescription>Connections are bound to this workspace. Revocation is immediate.</CardDescription></div>
            {dashboard.can_manage && (dashboard.connections.some((item) => item.status === 'active') || dashboard.service_principals.some((item) => item.status === 'active')) ? (
              <Button type="button" variant="outline" size="sm" disabled={revokeWorkspace.isPending} onClick={() => void handleRevokeWorkspace()}>Revoke all</Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {dashboard.connections.length === 0 ? (
            <EmptyState title="No connections yet" description="Add the connection URL to an AI tool to start the secure sign-in flow." />
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Tool or automation</TableHead><TableHead>Access</TableHead><TableHead>Last used</TableHead><TableHead>Status</TableHead><TableHead className="w-16" /></TableRow></TableHeader>
              <TableBody>
                {dashboard.connections.map((connection) => {
                  const grantHasWriteScopes = hasMCPWriteScope(connection.scopes);
                  const connectionHasWrites = !connection.read_only
                    && grantHasWriteScopes
                    && connection.effective_read_only !== true;
                  const accessNote = connectionHasWrites
                    ? null
                    : !policyAllowsWrites
                      ? 'Workspace permissions currently do not allow writes.'
                      : connection.read_only || !grantHasWriteScopes
                        ? 'Reconnect and approve write permissions to expand this grant.'
                        : 'Writes are currently restricted by role, module access, or a platform setting.';
                  return (
                    <TableRow key={connection.id}>
                      <TableCell><div className="font-medium">{connection.client_name}</div><div className="text-xs text-muted-foreground">Connected {relativeDate(connection.created_at)}</div></TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1"><Badge variant="outline" className={connectionHasWrites ? undefined : READ_ONLY_BADGE_CLASS}>{connectionHasWrites ? 'Bounded writes' : 'Read-only'}</Badge>{connection.toolsets.map((item) => <Badge key={item} variant="secondary">{TOOLSET_LABELS[item] ?? item}</Badge>)}</div>
                        {accessNote ? <p className="mt-1 text-xs text-muted-foreground">{accessNote}</p> : null}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{connection.last_used_at ? relativeDate(connection.last_used_at) : 'Never'}</TableCell>
                      <TableCell><Badge variant={connection.status === 'active' ? 'secondary' : 'outline'}>{connection.status}</Badge></TableCell>
                      <TableCell>
                        {connection.status === 'active' ? (
                          <Button type="button" variant="ghost" size="icon-sm" disabled={revoke.isPending} onClick={() => void handleRevoke(connection.id, connection.client_name)}>
                            <Delete01Icon className="h-4 w-4" /><span className="sr-only">Revoke {connection.client_name}</span>
                          </Button>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function MCPServiceAccounts({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const [createOpen, setCreateOpen] = useState(false);
  const [secret, setSecret] = useState<MCPServiceTokenSecret | null>(null);
  const [selected, setSelected] = useState<MCPServicePrincipal | null>(null);
  const revoke = useRevokeMCPServicePrincipal(workspaceId);
  const confirm = useConfirm();

  const handleRevoke = async (principal: MCPServicePrincipal) => {
    if (!await confirm({ title: `Revoke ${principal.name}?`, description: 'All credentials for this automation account will stop working.', confirmText: 'Revoke automation', variant: 'destructive' })) return;
    try {
      await revoke.mutateAsync(principal.id);
      toast.success('Automation account revoked');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not revoke automation account');
    }
  };

  return (
    <>
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div><CardTitle className="text-base">Automation accounts</CardTitle><CardDescription>Restricted credentials for workflows that run without a person signing in. Secrets are shown once.</CardDescription></div>
            <Button size="sm" onClick={() => setCreateOpen(true)} disabled={!dashboard.policy.service_accounts_enabled}>Create automation account</Button>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!dashboard.policy.service_accounts_enabled ? (
            <EmptyState title="Automation accounts are disabled" description="Allow them in the Permissions tab before creating a credential." />
          ) : dashboard.service_principals.length === 0 ? (
            <EmptyState title="No automation accounts" description="Create one for a narrowly scoped workflow that runs without a person signing in." />
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Access</TableHead><TableHead>Last used</TableHead><TableHead>Status</TableHead><TableHead className="w-28" /></TableRow></TableHeader>
              <TableBody>
                {dashboard.service_principals.map((principal) => (
                  <TableRow key={principal.id}>
                    <TableCell><div className="font-medium">{principal.name}</div><div className="max-w-64 truncate text-xs text-muted-foreground">{principal.description || 'No description'}</div></TableCell>
                    <TableCell><Badge variant="outline" className={principal.read_only || principal.effective_read_only === true ? READ_ONLY_BADGE_CLASS : undefined}>{principal.read_only || principal.effective_read_only === true ? 'Read-only' : 'Bounded writes'}</Badge></TableCell>
                    <TableCell className="text-muted-foreground">{principal.last_used_at ? relativeDate(principal.last_used_at) : 'Never'}</TableCell>
                    <TableCell><Badge variant={principal.status === 'active' ? 'secondary' : 'outline'}>{principal.status}</Badge></TableCell>
                    <TableCell><div className="flex justify-end gap-1">{principal.status === 'active' ? <><Button variant="ghost" size="icon-sm" onClick={() => setSelected(principal)}><Key01Icon className="h-4 w-4" /><span className="sr-only">Create token</span></Button><Button variant="ghost" size="icon-sm" onClick={() => void handleRevoke(principal)}><Delete01Icon className="h-4 w-4" /><span className="sr-only">Revoke</span></Button></> : null}</div></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <CreateServiceAccountDialog open={createOpen} onOpenChange={setCreateOpen} workspaceId={workspaceId} dashboard={dashboard} onSecret={setSecret} />
      <RotateTokenDialog principal={selected} workspaceId={workspaceId} onOpenChange={(open) => { if (!open) setSelected(null); }} onSecret={setSecret} />
      <OneTimeSecretDialog secret={secret} onClose={() => setSecret(null)} />
    </>
  );
}

function MCPActivity({ dashboard, events, loading }: { dashboard: MCPDashboard; events: import('@/lib/mcpTypes').MCPAuditEvent[]; loading: boolean }) {
  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader><CardTitle className="text-base">AI tool activity</CardTitle><CardDescription>Review what connected tools have done in this workspace. Raw tool inputs and outputs are never stored.</CardDescription></CardHeader>
      <CardContent className="p-0">
        {loading ? <div className="space-y-2 p-5"><Skeleton className="h-9" /><Skeleton className="h-9" /><Skeleton className="h-9" /></div> : events.length === 0 ? (
          <EmptyState title="No AI tool activity" description={dashboard.policy.enabled ? 'Actions will appear here after a connected tool uses Helpin.' : 'Allow AI tools before they can create activity.'} />
        ) : (
          <Table>
            <TableHeader><TableRow><TableHead>Event</TableHead><TableHead>Tool or automation</TableHead><TableHead>Outcome</TableHead><TableHead>Duration</TableHead><TableHead>Time</TableHead></TableRow></TableHeader>
            <TableBody>{events.map((event) => <TableRow key={event.id}><TableCell><div className="font-medium">{event.tool_name || event.event_type}</div>{event.reason_code ? <div className="text-xs text-muted-foreground">{event.reason_code.replaceAll('_', ' ')}</div> : null}</TableCell><TableCell>{event.client_name || 'Unknown tool'}</TableCell><TableCell><Badge variant={event.outcome === 'success' ? 'secondary' : event.outcome === 'error' ? 'destructive' : 'outline'}>{event.outcome}</Badge></TableCell><TableCell className="text-muted-foreground">{event.duration_ms == null ? '—' : `${event.duration_ms} ms`}</TableCell><TableCell className="text-muted-foreground">{relativeDate(event.created_at)}</TableCell></TableRow>)}</TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function MCPPolicyEditor({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    () => new Set(['access-controls']),
  );
  const [policy, setPolicy] = useState<UpdateMCPPolicyRequest>({
    enabled: dashboard.policy.enabled,
    enforce_read_only: dashboard.policy.enforce_read_only,
    service_accounts_enabled: dashboard.policy.service_accounts_enabled,
    allowed_toolsets: dashboard.policy.allowed_toolsets,
    allowed_scopes: dashboard.policy.allowed_scopes,
  });
  const update = useUpdateMCPPolicy(workspaceId);
  const toggleSection = (section: string) => {
    setExpandedSections((current) => {
      const next = new Set(current);
      if (next.has(section)) next.delete(section);
      else next.add(section);
      return next;
    });
  };

  const setBoundedWrites = (enabled: boolean) => {
    setPolicy((current) => ({
      ...current,
      enforce_read_only: !enabled,
      allowed_scopes: enabled
        ? ensureMCPWriteScopes(
            current.allowed_scopes,
            current.allowed_toolsets,
            dashboard.available_scopes,
          )
        : current.allowed_scopes,
    }));
  };
  const toggleToolset = (value: string, checked: boolean) => {
    setPolicy((current) => {
      const allowedToolsets = checked
        ? [...current.allowed_toolsets, value]
        : current.allowed_toolsets.filter((item) => item !== value);
      return {
        ...current,
        allowed_toolsets: allowedToolsets,
        allowed_scopes: current.enforce_read_only
          ? current.allowed_scopes
          : ensureMCPWriteScopes(
              current.allowed_scopes,
              allowedToolsets,
              dashboard.available_scopes,
            ),
      };
    });
  };
  const toggleScope = (value: string, checked: boolean) => {
    setPolicy((current) => ({
      ...current,
      allowed_scopes: checked
        ? [...current.allowed_scopes, value]
        : current.allowed_scopes.filter((item) => item !== value),
    }));
  };
  const save = async () => {
    try {
      await update.mutateAsync(policy);
      toast.success('Permissions saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not save permissions');
    }
  };

  return (
    <div className="space-y-3">
      <MCPPolicySection
        title="AI tool access controls"
        description="Choose whether members can connect tools, approve writes, or create automation accounts."
        expanded={expandedSections.has('access-controls')}
        onToggle={() => toggleSection('access-controls')}
      >
        <div className="divide-y">
          <PolicySwitch label="Allow AI tools and automations" description="Let members connect MCP-compatible tools to this workspace. Their Helpin roles and module permissions still apply." checked={policy.enabled} onCheckedChange={(checked) => setPolicy((value) => ({ ...value, enabled: checked }))} />
          <PolicySwitch label="Allow bounded writes" description="Allow new connections to request create and update actions in the selected product areas. Existing connections must reconnect, turn off read-only during consent, and approve the write permissions." checked={!policy.enforce_read_only} onCheckedChange={setBoundedWrites} />
          <PolicySwitch label="Allow automation accounts" description="Let managers create restricted credentials for workflows that run without a person signing in." checked={policy.service_accounts_enabled} onCheckedChange={(checked) => setPolicy((value) => ({ ...value, service_accounts_enabled: checked }))} />
        </div>
      </MCPPolicySection>
      <MCPPolicySection
        title="Allowed product areas"
        description={`${policy.allowed_toolsets.length} of ${dashboard.available_toolsets.length} selected. Connected tools can request a subset of these areas.`}
        expanded={expandedSections.has('product-areas')}
        onToggle={() => toggleSection('product-areas')}
      >
        <PolicyChecklist values={dashboard.available_toolsets} selected={policy.allowed_toolsets} labels={TOOLSET_LABELS} onToggle={toggleToolset} />
      </MCPPolicySection>
      <MCPPolicySection
        title="Allowed OAuth scopes"
        description={`${policy.allowed_scopes.length} of ${dashboard.available_scopes.length} selected. Users may narrow these during consent, never expand them.`}
        expanded={expandedSections.has('oauth-scopes')}
        onToggle={() => toggleSection('oauth-scopes')}
      >
        <PolicyChecklist values={dashboard.available_scopes} selected={policy.allowed_scopes} labels={SCOPE_LABELS} onToggle={toggleScope} />
      </MCPPolicySection>
      {!policy.enforce_read_only && !hasMCPWriteScope(policy.allowed_scopes) ? (
        <Alert>
          <AlertTitle>Writes are still unavailable</AlertTitle>
          <AlertDescription>Select at least one create, update, or agent-run permission. Without a write scope, new connections remain effectively read-only.</AlertDescription>
        </Alert>
      ) : null}
      <div className="flex justify-end"><Button onClick={() => void save()} disabled={update.isPending || policy.allowed_toolsets.length === 0 || policy.allowed_scopes.length === 0}>{update.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}Save permissions</Button></div>
    </div>
  );
}

function CreateServiceAccountDialog({ open, onOpenChange, workspaceId, dashboard, onSecret }: { open: boolean; onOpenChange: (open: boolean) => void; workspaceId: string; dashboard: MCPDashboard; onSecret: (secret: MCPServiceTokenSecret) => void }) {
  const readScopes = useMemo(() => dashboard.policy.allowed_scopes.filter((scope) => !scope.endsWith('.write') && scope !== 'helpin.agents.run'), [dashboard.policy.allowed_scopes]);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [toolsets, setToolsets] = useState(dashboard.policy.allowed_toolsets);
  const [scopes] = useState(readScopes);
  const create = useCreateMCPServicePrincipal(workspaceId);
  const submit = async () => {
    const request: CreateMCPServicePrincipalRequest = { name: name.trim(), description: description.trim() || undefined, toolsets, scopes, read_only: true };
    try {
      const result = await create.mutateAsync(request);
      onOpenChange(false);
      onSecret(result.secret);
      setName(''); setDescription('');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not create automation account');
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader><DialogTitle>Create automation account</DialogTitle><DialogDescription>Create a read-only credential first. You can create a different account later if a workflow needs to update work.</DialogDescription></DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2"><Label htmlFor="service-name">Name</Label><Input id="service-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Release notes workflow" maxLength={100} /></div>
          <div className="space-y-2"><Label htmlFor="service-description">Description</Label><Textarea id="service-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Where this credential is used" /></div>
          <div className="space-y-2"><Label>Toolsets</Label><div className="grid gap-2 sm:grid-cols-2">{dashboard.policy.allowed_toolsets.map((value) => <CheckRow key={value} label={TOOLSET_LABELS[value] ?? value} checked={toolsets.includes(value)} onCheckedChange={(checked) => setToolsets((items) => checked ? [...items, value] : items.filter((item) => item !== value))} />)}</div></div>
          <p className="text-xs text-muted-foreground">The token inherits your current Helpin membership and can be revoked immediately.</p>
        </div>
        <DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button onClick={() => void submit()} disabled={!name.trim() || toolsets.length === 0 || scopes.length === 0 || create.isPending}>{create.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}Create</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RotateTokenDialog({ principal, workspaceId, onOpenChange, onSecret }: { principal: MCPServicePrincipal | null; workspaceId: string; onOpenChange: (open: boolean) => void; onSecret: (secret: MCPServiceTokenSecret) => void }) {
  const rotate = useRotateMCPServiceToken(workspaceId, principal?.id ?? '');
  const create = async () => {
    try { const secret = await rotate.mutateAsync(); onOpenChange(false); onSecret(secret); }
    catch (error) { toast.error(error instanceof Error ? error.message : 'Could not create token'); }
  };
  return <Dialog open={Boolean(principal)} onOpenChange={onOpenChange}><DialogContent><DialogHeader><DialogTitle>Create another token</DialogTitle><DialogDescription>This creates a new secret for {principal?.name}. Existing tokens remain active until revoked.</DialogDescription></DialogHeader><DialogFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button onClick={() => void create()} disabled={rotate.isPending}>{rotate.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}Create token</Button></DialogFooter></DialogContent></Dialog>;
}

function OneTimeSecretDialog({ secret, onClose }: { secret: MCPServiceTokenSecret | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => { if (!secret) return; await navigator.clipboard.writeText(secret.token); setCopied(true); toast.success('Token copied'); };
  return <Dialog open={Boolean(secret)} onOpenChange={(open) => { if (!open) { setCopied(false); onClose(); } }}><DialogContent><DialogHeader><DialogTitle>Copy this token now</DialogTitle><DialogDescription>Helpin stores only a hash. This secret cannot be shown again after you close this dialog.</DialogDescription></DialogHeader>{secret ? <div className="space-y-3"><div className="break-all rounded-lg border bg-muted/40 p-3 font-mono text-xs">{secret.token}</div><Button className="w-full" variant="outline" onClick={() => void copy()}><Copy01Icon className="mr-2 h-4 w-4" />{copied ? 'Copied' : 'Copy token'}</Button></div> : null}<DialogFooter><Button onClick={onClose}>I saved the token</Button></DialogFooter></DialogContent></Dialog>;
}

function PolicySwitch({ label, description, checked, onCheckedChange }: { label: string; description: string; checked: boolean; onCheckedChange: (checked: boolean) => void }) {
  return <div className="flex items-start justify-between gap-5 py-4 first:pt-0 last:pb-0"><div><p className="text-sm font-medium">{label}</p><p className="mt-1 text-sm text-muted-foreground">{description}</p></div><Switch checked={checked} onCheckedChange={onCheckedChange} /></div>;
}

function MCPPolicySection({ title, description, expanded, onToggle, children }: { title: string; description: string; expanded: boolean; onToggle: () => void; children: ReactNode }) {
  return (
    <section className="overflow-hidden rounded-lg border border-border/70 bg-card">
      <button type="button" className="flex w-full items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-muted/40" aria-expanded={expanded} onClick={onToggle}>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>
        </div>
        <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', expanded && 'rotate-180')} />
      </button>
      <div className="accordion-animate" data-open={expanded}>
        <div><div className="border-t px-5 py-4">{children}</div></div>
      </div>
    </section>
  );
}

function PolicyChecklist({ values, selected, labels, onToggle }: { values: string[]; selected: string[]; labels: Record<string, string>; onToggle: (value: string, checked: boolean) => void }) {
  return <div className="grid gap-3 sm:grid-cols-2">{values.map((value) => <CheckRow key={value} label={labels[value] ?? value} checked={selected.includes(value)} onCheckedChange={(checked) => onToggle(value, checked)} />)}</div>;
}

function CheckRow({ label, checked, onCheckedChange }: { label: string; checked: boolean; onCheckedChange: (checked: boolean) => void }) {
  return <label className="flex cursor-pointer items-center gap-2.5 text-sm"><Checkbox checked={checked} onCheckedChange={(value) => onCheckedChange(value === true)} /><span>{label}</span></label>;
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return <div className="px-5 py-10 text-center"><p className="text-sm font-medium">{title}</p><p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{description}</p></div>;
}

function MCPSettingsSkeleton() {
  return <div className="space-y-4"><Skeleton className="h-8 w-56" /><Skeleton className="h-9 w-96 max-w-full" /><div className="grid gap-4 lg:grid-cols-2"><Skeleton className="h-72" /><Skeleton className="h-72" /></div></div>;
}

function relativeDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? 'Unknown' : formatDistanceToNow(date, { addSuffix: true });
}
