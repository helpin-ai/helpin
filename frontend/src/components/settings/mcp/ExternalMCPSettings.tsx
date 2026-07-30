import { useMemo, useState } from 'react';
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
import { Textarea } from '@/components/ui/textarea';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import {
  useCreateExternalMCPServer,
  useDeleteExternalMCPServer,
  useExternalMCPProviders,
  useExternalMCPServers,
  useRefreshExternalMCPTools,
  useStartExternalMCPOAuth,
  useUpdateExternalMCPServer,
  useUpdateExternalMCPTools,
} from '@/hooks/queries/useExternalMCP';
import type {
  CreateExternalMCPServerRequest,
  ExternalMCPAuthType,
  ExternalMCPProvider,
  ExternalMCPServer,
  ExternalMCPTool,
} from '@/lib/externalMCPTypes';
import {
  ArrowDown01Icon,
  ArrowReloadHorizontalIcon,
  DatabaseIcon,
  Delete01Icon,
  Globe02Icon,
  Key01Icon,
  Loading01Icon,
  PlusSignIcon,
  Shield01Icon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';

type ExternalMCPSettingsProps = {
  workspaceId: string;
  workspaceName: string;
};

const STATUS_COPY: Record<ExternalMCPServer['status'], { label: string; className: string }> = {
  connected: { label: 'Connected', className: 'border-emerald-300/70 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300' },
  pending_oauth: { label: 'Authorization needed', className: 'border-amber-300/70 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300' },
  reauthorization_required: { label: 'Reconnect', className: 'border-amber-300/70 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300' },
  insufficient_scope: { label: 'More access needed', className: 'border-amber-300/70 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300' },
  remote_disabled: { label: 'Disabled at provider', className: 'border-destructive/30 bg-destructive/5 text-destructive' },
  error: { label: 'Connection error', className: 'border-destructive/30 bg-destructive/5 text-destructive' },
  disconnected: { label: 'Disconnected', className: 'text-muted-foreground' },
};

const CUSTOMER_SCOPE_COPY: Record<string, string> = {
  read: 'Read workspace data',
  'read:sensitive': 'Read sensitive data',
  write: 'Create and update data',
  'write:live': 'Apply changes to live campaigns',
  configure: 'Change workspace configuration',
};

export function ExternalMCPSettings({ workspaceId, workspaceName }: ExternalMCPSettingsProps) {
  const providersQuery = useExternalMCPProviders(workspaceId);
  const enabled = providersQuery.data?.enabled === true;
  const serversQuery = useExternalMCPServers(workspaceId, enabled);
  const [createOpen, setCreateOpen] = useState(false);
  const servers = serversQuery.data ?? [];

  if (providersQuery.isLoading) {
    return <div className="grid gap-4 lg:grid-cols-2"><Skeleton className="h-48" /><Skeleton className="h-48" /></div>;
  }
  if (providersQuery.isError) {
    return <Alert variant="destructive"><AlertTitle>Could not load external MCP settings</AlertTitle><AlertDescription>{providersQuery.error.message}</AlertDescription></Alert>;
  }
  if (!enabled) {
    return (
      <Alert>
        <Shield01Icon className="h-4 w-4" />
        <AlertTitle>External MCP is not enabled in this environment</AlertTitle>
        <AlertDescription>An operator must configure the external MCP encryption key, OAuth callback URL, runtime host allowlist, and then enable the rollout switch.</AlertDescription>
      </Alert>
    );
  }

  const reconnecting = servers.filter((server) => server.status === 'reauthorization_required');
  return (
    <div className="space-y-4">
      {reconnecting.length > 0 ? (
        <Alert>
          <Key01Icon className="h-4 w-4" />
          <AlertTitle>{reconnecting.length === 1 ? `${reconnecting[0].name} needs to be reconnected` : `${reconnecting.length} servers need to be reconnected`}</AlertTitle>
          <AlertDescription>Authorization expired or was revoked. Agents cannot use those tools until a workspace manager reconnects the server.</AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(260px,0.8fr)]">
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
              <div>
                <CardTitle className="text-base">Workspace MCP servers</CardTitle>
                <CardDescription>Add external systems once, then choose their individual tools on each Helpin agent.</CardDescription>
              </div>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <PlusSignIcon className="mr-1.5 h-4 w-4" /> Add server
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {serversQuery.isLoading ? <Skeleton className="h-32" /> : null}
            {serversQuery.isError ? <Alert variant="destructive"><AlertTitle>Could not load servers</AlertTitle><AlertDescription>{serversQuery.error.message}</AlertDescription></Alert> : null}
            {!serversQuery.isLoading && servers.length === 0 ? (
              <div className="rounded-xl border border-dashed px-6 py-10 text-center">
                <Globe02Icon className="mx-auto h-6 w-6 text-muted-foreground" />
                <p className="mt-3 text-sm font-medium">No external servers yet</p>
                <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">Connect Customer.io to test the full OAuth and agent tool flow, or add an approved custom server.</p>
                <Button className="mt-4" size="sm" variant="outline" onClick={() => setCreateOpen(true)}>Add your first server</Button>
              </div>
            ) : null}
            {servers.map((server) => <ExternalMCPServerCard key={server.id} workspaceId={workspaceId} server={server} />)}
          </CardContent>
        </Card>

        <Card className={`${LINEAR_CARD_CLASS} self-start`}>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base"><Shield01Icon className="h-4 w-4 text-muted-foreground" />Per-run security</CardTitle>
            <CardDescription>Helpin keeps durable credentials. Agent Runtime gets only the exact selected tools and the current credential for one run.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <SecurityRow label="Workspace isolation" value={workspaceName || 'Current workspace'} />
            <SecurityRow label="Tool policy" value="Explicit allowlist" />
            <SecurityRow label="Write actions" value="Approval-aware" />
            <SecurityRow label="Expired OAuth" value="Pauses and reconnects" />
            <p className="border-t pt-3 text-xs leading-relaxed text-muted-foreground">Normal access-token expiry refreshes silently. You are notified only when consent is revoked or the refresh credential no longer works.</p>
          </CardContent>
        </Card>
      </div>

      <AddExternalMCPDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        workspaceId={workspaceId}
        providers={providersQuery.data?.providers ?? []}
      />
    </div>
  );
}

function ExternalMCPServerCard({ workspaceId, server }: { workspaceId: string; server: ExternalMCPServer }) {
  const [expanded, setExpanded] = useState(false);
  const updateServer = useUpdateExternalMCPServer(workspaceId);
  const deleteServer = useDeleteExternalMCPServer(workspaceId);
  const refresh = useRefreshExternalMCPTools(workspaceId);
  const oauth = useStartExternalMCPOAuth(workspaceId);
  const confirm = useConfirm();
  const tools = server.tools ?? [];
  const status = STATUS_COPY[server.status];
  const needsOAuth = server.auth_type === 'oauth' && ['pending_oauth', 'reauthorization_required', 'insufficient_scope'].includes(server.status);

  const connect = async () => {
    try {
      const result = await oauth.mutateAsync(server.id);
      window.location.assign(result.authorization_url);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not start authorization');
    }
  };
  const sync = async () => {
    try {
      await refresh.mutateAsync(server.id);
      toast.success('Tools refreshed');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not refresh tools');
    }
  };
  const remove = async () => {
    const accepted = await confirm({
      title: `Remove ${server.name}?`,
      description: 'The stored credential and discovered tools will be deleted. Agents that still reference these tools must be updated.',
      confirmText: 'Remove server',
      variant: 'destructive',
    });
    if (!accepted) return;
    try {
      await deleteServer.mutateAsync(server.id);
      toast.success('Server removed');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not remove server');
    }
  };

  return (
    <section className={cn('overflow-hidden rounded-xl border bg-background transition-opacity', !server.enabled && 'opacity-70')}>
      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-start">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border bg-muted/30">
          <DatabaseIcon className="h-4 w-4 text-muted-foreground" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="font-medium">{server.name}</p>
            <Badge variant="outline" className={status.className}>{status.label}</Badge>
            {!server.enabled ? <Badge variant="secondary">Off</Badge> : null}
          </div>
          <p className="mt-1 truncate text-xs text-muted-foreground">{safeEndpointLabel(server.endpoint_url)} · {tools.filter((tool) => tool.enabled).length}/{tools.length} tools enabled</p>
          {server.last_error_message ? <p className="mt-2 text-xs text-destructive">{server.last_error_message}</p> : null}
          <div className="mt-3 flex flex-wrap gap-2">
            {needsOAuth ? <Button size="sm" onClick={() => void connect()} disabled={oauth.isPending}>{oauth.isPending ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Key01Icon className="mr-1.5 h-3.5 w-3.5" />}{server.status === 'pending_oauth' ? 'Connect' : 'Reconnect'}</Button> : null}
            {server.status === 'connected' ? <Button size="sm" variant="outline" onClick={() => void sync()} disabled={refresh.isPending}>{refresh.isPending ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ArrowReloadHorizontalIcon className="mr-1.5 h-3.5 w-3.5" />}Refresh tools</Button> : null}
            {tools.length > 0 ? <Button size="sm" variant="ghost" onClick={() => setExpanded((value) => !value)}>{expanded ? 'Hide tools' : 'Manage tools'}<ArrowDown01Icon className={cn('ml-1.5 h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')} /></Button> : null}
          </div>
        </div>
        <div className="flex items-center gap-1 self-start">
          <Switch
            checked={server.enabled}
            aria-label={`${server.enabled ? 'Disable' : 'Enable'} ${server.name}`}
            disabled={updateServer.isPending}
            onCheckedChange={(enabled) => updateServer.mutate({ serverId: server.id, request: { enabled } })}
          />
          <Button size="icon-sm" variant="ghost" onClick={() => void remove()} disabled={deleteServer.isPending}>
            <Delete01Icon className="h-4 w-4" /><span className="sr-only">Remove {server.name}</span>
          </Button>
        </div>
      </div>
      {expanded ? <ExternalMCPToolPolicies workspaceId={workspaceId} server={server} tools={tools} /> : null}
    </section>
  );
}

function ExternalMCPToolPolicies({ workspaceId, server, tools }: { workspaceId: string; server: ExternalMCPServer; tools: ExternalMCPTool[] }) {
  const updateTools = useUpdateExternalMCPTools(workspaceId);
  const update = async (toolID: string, patch: Partial<Pick<ExternalMCPTool, 'enabled' | 'access'>>) => {
    const next = tools.map((tool) => tool.id === toolID ? { ...tool, ...patch } : tool);
    try {
      await updateTools.mutateAsync({
        serverId: server.id,
        request: { tools: next.map((tool) => ({ id: tool.id, enabled: tool.enabled, access: tool.access })) },
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not update tool policy');
    }
  };
  return (
    <div className="border-t bg-muted/15 px-4 py-3">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div><p className="text-sm font-medium">Agent tools</p><p className="text-xs text-muted-foreground">Enabled tools appear under External MCP in the agent editor.</p></div>
        <Badge variant="outline">{tools.length} discovered</Badge>
      </div>
      <div className="space-y-2">
        {tools.map((tool) => (
          <div key={tool.id} className="flex flex-col gap-3 rounded-lg border bg-background p-3 sm:flex-row sm:items-center">
            <Checkbox checked={tool.enabled} disabled={updateTools.isPending} onCheckedChange={(checked) => void update(tool.id, { enabled: checked === true })} aria-label={`Enable ${tool.remote_name}`} />
            <div className="min-w-0 flex-1">
              <p className="font-mono text-xs font-medium">{tool.remote_name}</p>
              <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{tool.description}</p>
            </div>
            <div className="flex rounded-lg border p-0.5">
              {(['read', 'write'] as const).map((access) => (
                <button key={access} type="button" disabled={updateTools.isPending} onClick={() => void update(tool.id, { access })} className={cn('rounded-md px-2 py-1 text-xs capitalize transition-colors', tool.access === access ? 'bg-muted font-medium text-foreground' : 'text-muted-foreground hover:text-foreground')}>{access}</button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AddExternalMCPDialog({ open, onOpenChange, workspaceId, providers }: { open: boolean; onOpenChange: (open: boolean) => void; workspaceId: string; providers: ExternalMCPProvider[] }) {
  const create = useCreateExternalMCPServer(workspaceId);
  const oauth = useStartExternalMCPOAuth(workspaceId);
  const [providerKey, setProviderKey] = useState('customer_io:us');
  const [name, setName] = useState('');
  const [endpointURL, setEndpointURL] = useState('');
  const [authType, setAuthType] = useState<ExternalMCPAuthType>('oauth');
  const [scopes, setScopes] = useState<string[]>(['read']);
  const [bearerToken, setBearerToken] = useState('');
  const [headersJSON, setHeadersJSON] = useState('{\n  "X-API-Key": ""\n}');
  const selected = useMemo(() => providers.find((provider) => `${provider.provider}:${provider.region}` === providerKey) ?? providers[0], [providerKey, providers]);
  const isCustom = selected?.provider === 'custom';

  const close = () => {
    if (!create.isPending && !oauth.isPending) onOpenChange(false);
  };
  const submit = async () => {
    if (!selected) return;
    let headers: Record<string, string> | undefined;
    if (isCustom && authType === 'headers') {
      try {
        const parsed = JSON.parse(headersJSON) as unknown;
        if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('not an object');
        headers = parsed as Record<string, string>;
      } catch {
        toast.error('Credential headers must be a JSON object');
        return;
      }
    }
    const request: CreateExternalMCPServerRequest = isCustom ? {
      name: name.trim(), provider: 'custom', endpoint_url: endpointURL.trim(), auth_type: authType,
      oauth_scopes: authType === 'oauth' ? scopes : undefined,
      bearer_token: authType === 'bearer_token' ? bearerToken.trim() : undefined,
      headers,
    } : {
      name: name.trim(), provider: 'customer_io', region: selected.region, oauth_scopes: scopes,
    };
    try {
      const server = await create.mutateAsync(request);
      if (server.auth_type === 'oauth') {
        const result = await oauth.mutateAsync(server.id);
        window.location.assign(result.authorization_url);
        return;
      }
      toast.success('External MCP server added');
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not add server');
    }
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader><DialogTitle>Add external MCP server</DialogTitle><DialogDescription>Credentials are encrypted in Helpin and are never returned by the API.</DialogDescription></DialogHeader>
        <div className="space-y-5 py-1">
          <div className="grid gap-2 sm:grid-cols-2">
            {providers.map((provider) => {
              const key = `${provider.provider}:${provider.region}`;
              return <button key={key} type="button" onClick={() => { setProviderKey(key); setScopes(provider.default_scopes ?? []); setName(''); }} className={cn('rounded-xl border p-3 text-left transition-colors hover:bg-muted/40', providerKey === key && 'border-primary bg-primary/5 ring-1 ring-primary/20')}><p className="text-sm font-medium">{provider.name}</p><p className="mt-1 text-xs text-muted-foreground">{provider.provider === 'customer_io' ? `Browser OAuth · ${provider.region.toUpperCase()}` : 'Approved URL and credential'}</p></button>;
            })}
          </div>
          <div className="space-y-2"><Label htmlFor="external-mcp-name">Display name <span className="font-normal text-muted-foreground">(optional for presets)</span></Label><Input id="external-mcp-name" value={name} onChange={(event) => setName(event.target.value)} placeholder={selected?.name ?? 'Analytics MCP'} maxLength={120} /></div>
          {isCustom ? (
            <>
              <div className="space-y-2"><Label htmlFor="external-mcp-url">HTTPS endpoint</Label><Input id="external-mcp-url" value={endpointURL} onChange={(event) => setEndpointURL(event.target.value)} placeholder="https://mcp.example.com/mcp" /></div>
              <div className="space-y-2"><Label>Authentication</Label><div className="flex flex-wrap gap-2">{(['oauth', 'bearer_token', 'headers', 'none'] as const).map((value) => <Button key={value} type="button" size="sm" variant={authType === value ? 'default' : 'outline'} onClick={() => setAuthType(value)}>{value === 'bearer_token' ? 'Bearer token' : value === 'headers' ? 'Custom headers' : value === 'none' ? 'None' : 'Browser OAuth'}</Button>)}</div></div>
              {authType === 'bearer_token' ? <div className="space-y-2"><Label htmlFor="external-mcp-token">Bearer token</Label><Input id="external-mcp-token" type="password" autoComplete="off" value={bearerToken} onChange={(event) => setBearerToken(event.target.value)} /></div> : null}
              {authType === 'headers' ? <div className="space-y-2"><Label htmlFor="external-mcp-headers">Credential headers (JSON)</Label><Textarea id="external-mcp-headers" className="min-h-28 font-mono text-xs" value={headersJSON} onChange={(event) => setHeadersJSON(event.target.value)} /><p className="text-xs text-muted-foreground">Host, cookie, connection, proxy, and transfer headers are blocked.</p></div> : null}
              {authType === 'oauth' ? <div className="space-y-2"><Label htmlFor="external-mcp-scopes">OAuth scopes <span className="font-normal text-muted-foreground">(space separated)</span></Label><Input id="external-mcp-scopes" value={scopes.join(' ')} onChange={(event) => setScopes(event.target.value.split(/[\s,]+/).filter(Boolean))} placeholder="read write" /><p className="text-xs text-muted-foreground">Use the scopes documented by the MCP provider. Leave blank when the provider does not require one.</p></div> : null}
            </>
          ) : null}
          {!isCustom && selected ? (
            <div className="space-y-2"><Label>OAuth access</Label><div className="grid gap-2 sm:grid-cols-2">{Array.from(new Set([...selected.default_scopes, ...selected.optional_scopes])).map((scope) => <label key={scope} className="flex items-start gap-2.5 rounded-lg border p-2.5 text-sm"><Checkbox checked={scopes.includes(scope)} disabled={selected.default_scopes.includes(scope)} onCheckedChange={(checked) => setScopes((current) => checked === true ? Array.from(new Set([...current, scope])) : current.filter((item) => item !== scope))} /><span><span className="block font-medium">{CUSTOMER_SCOPE_COPY[scope] ?? scope}</span><span className="font-mono text-[10px] text-muted-foreground">{scope}</span></span></label>)}</div></div>
          ) : null}
        </div>
        <DialogFooter><Button variant="outline" onClick={close}>Cancel</Button><Button onClick={() => void submit()} disabled={create.isPending || oauth.isPending || !selected || (isCustom && (!name.trim() || !endpointURL.trim() || (authType === 'bearer_token' && !bearerToken.trim())))}>{create.isPending || oauth.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}{selected?.auth_type === 'oauth' || isCustom && authType === 'oauth' ? 'Continue to authorize' : 'Add server'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SecurityRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between gap-4"><span className="text-muted-foreground">{label}</span><span className="text-right font-medium">{value}</span></div>;
}

function safeEndpointLabel(value: string) {
  try {
    const url = new URL(value);
    return `${url.hostname}${url.pathname}`;
  } catch {
    return 'Configured endpoint';
  }
}
