import { useMemo, useState } from 'react';
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
import { BotIcon, Copy01Icon, Delete01Icon, Key01Icon, Loading01Icon } from '@/lib/icons';
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
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';

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
        <AlertTitle>Could not load AI client settings</AlertTitle>
        <AlertDescription>{dashboardQuery.error?.message ?? 'Try refreshing the page.'}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <div className="flex items-center gap-2">
          <BotIcon className="h-5 w-5 text-muted-foreground" />
          <h2 className="text-xl font-semibold">AI Clients &amp; MCP</h2>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          Connect AI assistants to {workspaceName || 'this workspace'}, control their access, and review activity.
        </p>
      </div>

      {!dashboard.platform_enabled ? (
        <Alert>
          <AlertTitle>AI client access is not available in this environment</AlertTitle>
          <AlertDescription>The platform rollout switch is off. Existing connections can still be reviewed and revoked.</AlertDescription>
        </Alert>
      ) : !dashboard.policy.enabled ? (
        <Alert>
          <AlertTitle>External AI clients are off</AlertTitle>
          <AlertDescription>
            {dashboard.can_manage
              ? 'Enable MCP in Workspace policy before authorizing a client.'
              : 'A workspace manager must enable MCP before you can connect a client.'}
          </AlertDescription>
        </Alert>
      ) : null}

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList variant="line" className="max-w-full overflow-x-auto">
          <TabsTrigger value="setup">Setup</TabsTrigger>
          <TabsTrigger value="connections">Connections</TabsTrigger>
          {dashboard.can_manage ? <TabsTrigger value="service-accounts">Service accounts</TabsTrigger> : null}
          {dashboard.can_view_activity ? <TabsTrigger value="activity">Activity</TabsTrigger> : null}
          {dashboard.can_manage ? <TabsTrigger value="policy">Workspace policy</TabsTrigger> : null}
        </TabsList>

        <TabsContent value="setup" className="mt-4">
          <MCPSetup dashboard={dashboard} />
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

function MCPSetup({ dashboard }: { dashboard: MCPDashboard }) {
  const copy = async (value: string, label: string) => {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied`);
  };
  const json = JSON.stringify({ mcpServers: { helpin: { url: dashboard.mcp_url } } }, null, 2);

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(280px,1fr)]">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Connect an AI client</CardTitle>
          <CardDescription>Use the hosted endpoint below. OAuth will ask you to choose this workspace and narrow access.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="mcp-url">MCP server URL</Label>
            <div className="flex gap-2">
              <Input id="mcp-url" value={dashboard.mcp_url} readOnly className="font-mono text-xs" />
              <Button type="button" variant="outline" size="icon" onClick={() => void copy(dashboard.mcp_url, 'Server URL')}>
                <Copy01Icon className="h-4 w-4" />
                <span className="sr-only">Copy server URL</span>
              </Button>
            </div>
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label>JSON configuration</Label>
              <Button type="button" variant="ghost" size="sm" onClick={() => void copy(json, 'Configuration')}>
                <Copy01Icon className="mr-1.5 h-3.5 w-3.5" /> Copy
              </Button>
            </div>
            <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs leading-5">{json}</pre>
          </div>
          <ol className="space-y-2 text-sm text-muted-foreground">
            <li><span className="mr-2 font-medium text-foreground">1.</span>Add the URL in Codex, Claude, Cursor, VS Code, or another remote MCP client.</li>
            <li><span className="mr-2 font-medium text-foreground">2.</span>Sign in to Helpin and select one workspace.</li>
            <li><span className="mr-2 font-medium text-foreground">3.</span>Review scopes and keep read-only access unless writes are needed.</li>
          </ol>
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Effective workspace access</CardTitle>
          <CardDescription>Every call is still checked against your current Helpin role and module access.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">Status</span>
            <Badge variant={dashboard.can_use_mcp ? 'secondary' : 'outline'}>{dashboard.can_use_mcp ? 'Enabled' : 'Disabled'}</Badge>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">Default mode</span>
            <Badge variant="outline">{dashboard.policy.enforce_read_only ? 'Read only' : 'Client request'}</Badge>
          </div>
          <div>
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">Allowed toolsets</p>
            <div className="flex flex-wrap gap-1.5">
              {dashboard.policy.allowed_toolsets.map((toolset) => (
                <Badge key={toolset} variant="outline">{TOOLSET_LABELS[toolset] ?? toolset}</Badge>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function MCPConnections({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const revoke = useRevokeMCPConnection(workspaceId);
  const revokeWorkspace = useRevokeMCPWorkspaceAccess(workspaceId);
  const confirm = useConfirm();

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
      title: 'Revoke all AI client access?',
      description: 'Every user connection, refresh token, service account, and service token in this workspace will stop working immediately.',
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
          <EmptyState title="No connections yet" description="Add the MCP server URL to an AI client to start OAuth setup." />
        ) : (
          <Table>
            <TableHeader><TableRow><TableHead>Client</TableHead><TableHead>Access</TableHead><TableHead>Last used</TableHead><TableHead>Status</TableHead><TableHead className="w-16" /></TableRow></TableHeader>
            <TableBody>
              {dashboard.connections.map((connection) => (
                <TableRow key={connection.id}>
                  <TableCell><div className="font-medium">{connection.client_name}</div><div className="text-xs text-muted-foreground">Connected {relativeDate(connection.created_at)}</div></TableCell>
                  <TableCell><div className="flex flex-wrap gap-1"><Badge variant="outline">{connection.read_only ? 'Read only' : 'Bounded writes'}</Badge>{connection.toolsets.map((item) => <Badge key={item} variant="secondary">{TOOLSET_LABELS[item] ?? item}</Badge>)}</div></TableCell>
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
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function MCPServiceAccounts({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const [createOpen, setCreateOpen] = useState(false);
  const [secret, setSecret] = useState<MCPServiceTokenSecret | null>(null);
  const [selected, setSelected] = useState<MCPServicePrincipal | null>(null);
  const revoke = useRevokeMCPServicePrincipal(workspaceId);
  const confirm = useConfirm();

  const handleRevoke = async (principal: MCPServicePrincipal) => {
    if (!await confirm({ title: `Revoke ${principal.name}?`, description: 'All tokens for this service account will stop working.', confirmText: 'Revoke service account', variant: 'destructive' })) return;
    try {
      await revoke.mutateAsync(principal.id);
      toast.success('Service account revoked');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not revoke service account');
    }
  };

  return (
    <>
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-start justify-between gap-4">
            <div><CardTitle className="text-base">Service accounts</CardTitle><CardDescription>Restricted credentials for headless automations. Secrets are shown once.</CardDescription></div>
            <Button size="sm" onClick={() => setCreateOpen(true)} disabled={!dashboard.policy.service_accounts_enabled}>Create service account</Button>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!dashboard.policy.service_accounts_enabled ? (
            <EmptyState title="Service accounts are disabled" description="Enable them in Workspace policy before creating a credential." />
          ) : dashboard.service_principals.length === 0 ? (
            <EmptyState title="No service accounts" description="Create one for a narrowly scoped, non-interactive workflow." />
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Access</TableHead><TableHead>Last used</TableHead><TableHead>Status</TableHead><TableHead className="w-28" /></TableRow></TableHeader>
              <TableBody>
                {dashboard.service_principals.map((principal) => (
                  <TableRow key={principal.id}>
                    <TableCell><div className="font-medium">{principal.name}</div><div className="max-w-64 truncate text-xs text-muted-foreground">{principal.description || 'No description'}</div></TableCell>
                    <TableCell><Badge variant="outline">{principal.read_only ? 'Read only' : 'Bounded writes'}</Badge></TableCell>
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
      <CardHeader><CardTitle className="text-base">MCP activity</CardTitle><CardDescription>Sanitized outcomes and hashes are stored; raw tool inputs and outputs are not shown here.</CardDescription></CardHeader>
      <CardContent className="p-0">
        {loading ? <div className="space-y-2 p-5"><Skeleton className="h-9" /><Skeleton className="h-9" /><Skeleton className="h-9" /></div> : events.length === 0 ? (
          <EmptyState title="No MCP activity" description={dashboard.policy.enabled ? 'Calls will appear here after a connected client uses Helpin.' : 'Enable MCP before clients can create activity.'} />
        ) : (
          <Table>
            <TableHeader><TableRow><TableHead>Event</TableHead><TableHead>Client</TableHead><TableHead>Outcome</TableHead><TableHead>Duration</TableHead><TableHead>Time</TableHead></TableRow></TableHeader>
            <TableBody>{events.map((event) => <TableRow key={event.id}><TableCell><div className="font-medium">{event.tool_name || event.event_type}</div>{event.reason_code ? <div className="text-xs text-muted-foreground">{event.reason_code.replaceAll('_', ' ')}</div> : null}</TableCell><TableCell>{event.client_name || 'Unknown client'}</TableCell><TableCell><Badge variant={event.outcome === 'success' ? 'secondary' : event.outcome === 'error' ? 'destructive' : 'outline'}>{event.outcome}</Badge></TableCell><TableCell className="text-muted-foreground">{event.duration_ms == null ? '—' : `${event.duration_ms} ms`}</TableCell><TableCell className="text-muted-foreground">{relativeDate(event.created_at)}</TableCell></TableRow>)}</TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function MCPPolicyEditor({ workspaceId, dashboard }: { workspaceId: string; dashboard: MCPDashboard }) {
  const [policy, setPolicy] = useState<UpdateMCPPolicyRequest>({
    enabled: dashboard.policy.enabled,
    enforce_read_only: dashboard.policy.enforce_read_only,
    service_accounts_enabled: dashboard.policy.service_accounts_enabled,
    allowed_toolsets: dashboard.policy.allowed_toolsets,
    allowed_scopes: dashboard.policy.allowed_scopes,
  });
  const update = useUpdateMCPPolicy(workspaceId);

  const toggle = (field: 'allowed_toolsets' | 'allowed_scopes', value: string, checked: boolean) => {
    setPolicy((current) => ({ ...current, [field]: checked ? [...current[field], value] : current[field].filter((item) => item !== value) }));
  };
  const save = async () => {
    try {
      await update.mutateAsync(policy);
      toast.success('MCP policy saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not save MCP policy');
    }
  };

  return (
    <div className="space-y-4">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader><CardTitle className="text-base">Workspace controls</CardTitle><CardDescription>Policy changes are applied again on every tool discovery and call.</CardDescription></CardHeader>
        <CardContent className="divide-y">
          <PolicySwitch label="Enable MCP" description="Allow members to authorize external AI clients for this workspace." checked={policy.enabled} onCheckedChange={(checked) => setPolicy((value) => ({ ...value, enabled: checked }))} />
          <PolicySwitch label="Enforce read-only connections" description="Remove write scopes even when a client requests them." checked={policy.enforce_read_only} onCheckedChange={(checked) => setPolicy((value) => ({ ...value, enforce_read_only: checked }))} />
          <PolicySwitch label="Allow service accounts" description="Let managers create restricted tokens for headless workflows." checked={policy.service_accounts_enabled} onCheckedChange={(checked) => setPolicy((value) => ({ ...value, service_accounts_enabled: checked }))} />
        </CardContent>
      </Card>
      <div className="grid gap-4 lg:grid-cols-2">
        <PolicyChecklist title="Allowed toolsets" description="Clients can request a subset of these product areas." values={dashboard.available_toolsets} selected={policy.allowed_toolsets} labels={TOOLSET_LABELS} onToggle={(value, checked) => toggle('allowed_toolsets', value, checked)} />
        <PolicyChecklist title="Allowed OAuth scopes" description="Users may narrow these during consent, never expand them." values={dashboard.available_scopes} selected={policy.allowed_scopes} labels={SCOPE_LABELS} onToggle={(value, checked) => toggle('allowed_scopes', value, checked)} />
      </div>
      <div className="flex justify-end"><Button onClick={() => void save()} disabled={update.isPending || policy.allowed_toolsets.length === 0 || policy.allowed_scopes.length === 0}>{update.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}Save policy</Button></div>
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
      toast.error(error instanceof Error ? error.message : 'Could not create service account');
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader><DialogTitle>Create service account</DialogTitle><DialogDescription>Create a read-only credential first. You can create a different account later if a workflow needs writes.</DialogDescription></DialogHeader>
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

function PolicyChecklist({ title, description, values, selected, labels, onToggle }: { title: string; description: string; values: string[]; selected: string[]; labels: Record<string, string>; onToggle: (value: string, checked: boolean) => void }) {
  return <Card className={LINEAR_CARD_CLASS}><CardHeader><CardTitle className="text-base">{title}</CardTitle><CardDescription>{description}</CardDescription></CardHeader><CardContent className="space-y-3">{values.map((value) => <CheckRow key={value} label={labels[value] ?? value} checked={selected.includes(value)} onCheckedChange={(checked) => onToggle(value, checked)} />)}</CardContent></Card>;
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
