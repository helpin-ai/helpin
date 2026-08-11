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

type ExternalMCPConnectionsProps = {
  workspaceId: string;
  workspaceName: string;
  canManageSettings: boolean;
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

const AUTH_OPTIONS: Array<{ value: ExternalMCPAuthType; label: string; description: string }> = [
  { value: 'oauth', label: 'OAuth', description: 'Sign in through the provider' },
  { value: 'bearer_token', label: 'Bearer token', description: 'Send an API token' },
  { value: 'headers', label: 'Custom headers', description: 'Send one or more secret headers' },
  { value: 'none', label: 'No authentication', description: 'For public or network-protected servers' },
];

export function ExternalMCPConnections({
  workspaceId,
  workspaceName,
  canManageSettings,
}: ExternalMCPConnectionsProps) {
  const providersQuery = useExternalMCPProviders(workspaceId);
  const enabled = providersQuery.data?.enabled === true;
  const serversQuery = useExternalMCPServers(workspaceId, enabled);
  const [createOpen, setCreateOpen] = useState(false);
  const servers = serversQuery.data ?? [];

  if (providersQuery.isLoading) {
    return <div className="grid gap-4 lg:grid-cols-2"><Skeleton className="h-48" /><Skeleton className="h-48" /></div>;
  }
  if (providersQuery.isError) {
    return <Alert variant="destructive"><AlertTitle>Could not load external MCP connections</AlertTitle><AlertDescription>{providersQuery.error.message}</AlertDescription></Alert>;
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
      {!canManageSettings ? (
        <Alert>
          <Shield01Icon className="h-4 w-4" />
          <AlertTitle>Connections are read-only</AlertTitle>
          <AlertDescription>A workspace admin can add servers, reconnect accounts, and change which tools agents may use.</AlertDescription>
        </Alert>
      ) : null}
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
                <CardTitle className="text-base">Connected servers</CardTitle>
                <CardDescription>Connect any approved remote MCP server, then choose which discovered tools agents may use.</CardDescription>
              </div>
              {canManageSettings ? (
                <Button size="sm" onClick={() => setCreateOpen(true)}>
                  <PlusSignIcon className="mr-1.5 h-4 w-4" /> Add server
                </Button>
              ) : null}
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {serversQuery.isLoading ? <Skeleton className="h-32" /> : null}
            {serversQuery.isError ? <Alert variant="destructive"><AlertTitle>Could not load servers</AlertTitle><AlertDescription>{serversQuery.error.message}</AlertDescription></Alert> : null}
            {!serversQuery.isLoading && servers.length === 0 ? (
              <div className="rounded-xl border border-dashed px-6 py-10 text-center">
                <Globe02Icon className="mx-auto h-6 w-6 text-muted-foreground" />
                <p className="mt-3 text-sm font-medium">No external servers yet</p>
                <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
                  {canManageSettings
                    ? 'Connect any approved Streamable HTTP MCP server to make its tools available to Helpin agents.'
                    : 'A workspace admin can connect an MCP server and make its tools available to Helpin agents.'}
                </p>
                {canManageSettings ? <Button className="mt-4" size="sm" variant="outline" onClick={() => setCreateOpen(true)}>Add your first server</Button> : null}
              </div>
            ) : null}
            {servers.map((server) => (
              <ExternalMCPServerCard
                key={server.id}
                workspaceId={workspaceId}
                server={server}
                canManageSettings={canManageSettings}
              />
            ))}
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

      {canManageSettings ? (
        <AddExternalMCPDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          workspaceId={workspaceId}
          providers={providersQuery.data?.providers ?? []}
        />
      ) : null}
    </div>
  );
}

function ExternalMCPServerCard({
  workspaceId,
  server,
  canManageSettings,
}: {
  workspaceId: string;
  server: ExternalMCPServer;
  canManageSettings: boolean;
}) {
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
            {canManageSettings && needsOAuth ? <Button size="sm" onClick={() => void connect()} disabled={oauth.isPending}>{oauth.isPending ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Key01Icon className="mr-1.5 h-3.5 w-3.5" />}{server.status === 'pending_oauth' ? 'Connect' : 'Reconnect'}</Button> : null}
            {canManageSettings && server.status === 'connected' ? <Button size="sm" variant="outline" onClick={() => void sync()} disabled={refresh.isPending}>{refresh.isPending ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ArrowReloadHorizontalIcon className="mr-1.5 h-3.5 w-3.5" />}Refresh tools</Button> : null}
            {tools.length > 0 ? <Button size="sm" variant="ghost" onClick={() => setExpanded((value) => !value)}>{expanded ? 'Hide tools' : canManageSettings ? 'Manage tools' : 'View tools'}<ArrowDown01Icon className={cn('ml-1.5 h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')} /></Button> : null}
          </div>
        </div>
        {canManageSettings ? (
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
        ) : null}
      </div>
      {expanded ? <ExternalMCPToolPolicies workspaceId={workspaceId} server={server} tools={tools} canManageSettings={canManageSettings} /> : null}
    </section>
  );
}

function ExternalMCPToolPolicies({
  workspaceId,
  server,
  tools,
  canManageSettings,
}: {
  workspaceId: string;
  server: ExternalMCPServer;
  tools: ExternalMCPTool[];
  canManageSettings: boolean;
}) {
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
            <Checkbox checked={tool.enabled} disabled={!canManageSettings || updateTools.isPending} onCheckedChange={(checked) => void update(tool.id, { enabled: checked === true })} aria-label={`Enable ${tool.remote_name}`} />
            <div className="min-w-0 flex-1">
              <p className="font-mono text-xs font-medium">{tool.remote_name}</p>
              <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{tool.description}</p>
            </div>
            <div className="flex rounded-lg border p-0.5">
              {(['read', 'write'] as const).map((access) => (
                <button key={access} type="button" disabled={!canManageSettings || updateTools.isPending} onClick={() => void update(tool.id, { access })} className={cn('rounded-md px-2 py-1 text-xs capitalize transition-colors disabled:cursor-default', tool.access === access ? 'bg-muted font-medium text-foreground' : 'text-muted-foreground hover:text-foreground disabled:hover:text-muted-foreground')}>{access}</button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

type CredentialHeader = { id: string; name: string; value: string };

function AddExternalMCPDialog({ open, onOpenChange, workspaceId, providers }: { open: boolean; onOpenChange: (open: boolean) => void; workspaceId: string; providers: ExternalMCPProvider[] }) {
  const create = useCreateExternalMCPServer(workspaceId);
  const oauth = useStartExternalMCPOAuth(workspaceId);
  const availableProviders = useMemo(() => {
    const generic = providers.find((provider) => provider.provider === 'custom') ?? {
      provider: 'custom' as const,
      region: '',
      name: 'Any MCP server',
      endpoint_url: '',
      auth_type: 'oauth' as const,
      default_scopes: [],
      optional_scopes: [],
    };
    return [generic, ...providers.filter((provider) => provider.provider !== 'custom')];
  }, [providers]);
  const providerShortcuts = availableProviders.filter((provider) => provider.provider !== 'custom');
  const [providerKey, setProviderKey] = useState(() => {
    const preferred = availableProviders[0];
    return preferred ? `${preferred.provider}:${preferred.region}` : '';
  });
  const [showProviderShortcuts, setShowProviderShortcuts] = useState(false);
  const [name, setName] = useState('');
  const [endpointURL, setEndpointURL] = useState('');
  const [authType, setAuthType] = useState<ExternalMCPAuthType>('oauth');
  const [scopes, setScopes] = useState<string[]>([]);
  const [bearerToken, setBearerToken] = useState('');
  const [headers, setHeaders] = useState<CredentialHeader[]>([{ id: 'header-1', name: 'X-API-Key', value: '' }]);
  const selected = useMemo(
    () => availableProviders.find((provider) => `${provider.provider}:${provider.region}` === providerKey) ?? availableProviders[0],
    [availableProviders, providerKey],
  );
  const isCustom = selected?.provider === 'custom';
  const headersComplete = headers.length > 0 && headers.every((header) => header.name.trim() && header.value.trim());

  const close = () => {
    if (!create.isPending && !oauth.isPending) onOpenChange(false);
  };
  const submit = async () => {
    if (!selected) return;
    let credentialHeaders: Record<string, string> | undefined;
    if (isCustom && authType === 'headers') {
      const entries = new Map<string, string>();
      for (const header of headersComplete ? headers : []) {
        const headerName = header.name.trim();
        if (entries.has(headerName.toLowerCase())) {
          toast.error(`Credential header ${headerName} is duplicated`);
          return;
        }
        entries.set(headerName.toLowerCase(), header.value.trim());
      }
      if (entries.size === 0) {
        toast.error('Add a header name and value');
        return;
      }
      credentialHeaders = Object.fromEntries(headers.map((header) => [header.name.trim(), header.value.trim()]));
    }
    const request: CreateExternalMCPServerRequest = isCustom ? {
      name: name.trim(), provider: 'custom', endpoint_url: endpointURL.trim(), auth_type: authType,
      oauth_scopes: authType === 'oauth' ? scopes : undefined,
      bearer_token: authType === 'bearer_token' ? bearerToken.trim() : undefined,
      headers: credentialHeaders,
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
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Connect an MCP server</DialogTitle>
          <DialogDescription>Use a provider preset or connect any approved remote MCP endpoint. Credentials are encrypted and never returned by the API.</DialogDescription>
        </DialogHeader>
        <div className="space-y-6 py-1">
          <div className="space-y-2">
            <div className="flex items-start gap-3 rounded-xl border bg-muted/20 p-3.5">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border bg-background">
                <Globe02Icon className="h-4 w-4 text-muted-foreground" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{isCustom ? 'Remote MCP server' : selected?.name}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  {isCustom
                    ? 'Connect directly with a Streamable HTTP URL and the authentication method your server uses.'
                    : `Provider shortcut · ${selected?.region.toUpperCase()} · OAuth`}
                </p>
              </div>
              {!isCustom ? (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setProviderKey('custom:');
                    setScopes([]);
                    setName('');
                  }}
                >
                  Use a URL
                </Button>
              ) : null}
            </div>
            {isCustom && providerShortcuts.length > 0 ? (
              <>
                <Button type="button" size="sm" variant="ghost" onClick={() => setShowProviderShortcuts((current) => !current)}>
                  {showProviderShortcuts ? 'Hide provider shortcuts' : 'Use a provider shortcut'}
                  <ArrowDown01Icon className={cn('ml-1.5 h-3.5 w-3.5 transition-transform', showProviderShortcuts && 'rotate-180')} />
                </Button>
                {showProviderShortcuts ? (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {providerShortcuts.map((provider) => {
                      const key = `${provider.provider}:${provider.region}`;
                      return (
                        <button
                          key={key}
                          type="button"
                          onClick={() => {
                            setProviderKey(key);
                            setScopes(provider.default_scopes ?? []);
                            setName('');
                            setShowProviderShortcuts(false);
                          }}
                          className="rounded-lg border px-3 py-2.5 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          <span className="block text-sm font-medium">{provider.name}</span>
                          <span className="mt-0.5 block text-xs text-muted-foreground">Preconfigured OAuth · {provider.region.toUpperCase()}</span>
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="external-mcp-name">Display name {!isCustom ? <span className="font-normal text-muted-foreground">(optional)</span> : null}</Label>
            <Input id="external-mcp-name" value={name} onChange={(event) => setName(event.target.value)} placeholder={isCustom ? 'My MCP server' : selected?.name ?? 'MCP server'} maxLength={120} />
          </div>
          {isCustom ? (
            <>
              <div className="space-y-2">
                <Label htmlFor="external-mcp-url">MCP server URL</Label>
                <Input id="external-mcp-url" type="url" inputMode="url" autoCapitalize="off" spellCheck={false} value={endpointURL} onChange={(event) => setEndpointURL(event.target.value)} placeholder="https://mcp.example.com/mcp" />
                <p className="text-xs text-muted-foreground">Remote Streamable HTTP endpoint. The host must be allowed by your Helpin environment.</p>
              </div>
              <fieldset className="space-y-2">
                <legend className="text-sm font-medium">Authentication</legend>
                <div className="grid gap-2 sm:grid-cols-2">
                  {AUTH_OPTIONS.map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      aria-pressed={authType === option.value}
                      onClick={() => setAuthType(option.value)}
                      className={cn(
                        'rounded-lg border px-3 py-2.5 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                        authType === option.value && 'border-primary bg-primary/5 ring-1 ring-primary/20',
                      )}
                    >
                      <span className="block text-sm font-medium">{option.label}</span>
                      <span className="mt-0.5 block text-xs text-muted-foreground">{option.description}</span>
                    </button>
                  ))}
                </div>
              </fieldset>
              {authType === 'bearer_token' ? (
                <div className="space-y-2">
                  <Label htmlFor="external-mcp-token">Bearer token</Label>
                  <Input id="external-mcp-token" type="password" autoComplete="off" value={bearerToken} onChange={(event) => setBearerToken(event.target.value)} placeholder="Paste token" />
                </div>
              ) : null}
              {authType === 'headers' ? (
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <Label>Credential headers</Label>
                    <Button type="button" size="sm" variant="ghost" onClick={() => setHeaders((current) => [...current, { id: crypto.randomUUID(), name: '', value: '' }])}>
                      <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" /> Add header
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {headers.map((header, index) => (
                      <div key={header.id} className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1fr)_auto] gap-2">
                        <Input aria-label={`Header ${index + 1} name`} autoCapitalize="off" spellCheck={false} value={header.name} onChange={(event) => setHeaders((current) => current.map((item) => item.id === header.id ? { ...item, name: event.target.value } : item))} placeholder="X-API-Key" />
                        <Input aria-label={`Header ${index + 1} value`} type="password" autoComplete="off" value={header.value} onChange={(event) => setHeaders((current) => current.map((item) => item.id === header.id ? { ...item, value: event.target.value } : item))} placeholder="Secret value" />
                        <Button type="button" size="icon" variant="ghost" disabled={headers.length === 1} onClick={() => setHeaders((current) => current.filter((item) => item.id !== header.id))}>
                          <Delete01Icon className="h-4 w-4" /><span className="sr-only">Remove header {index + 1}</span>
                        </Button>
                      </div>
                    ))}
                  </div>
                  <p className="text-xs text-muted-foreground">Restricted transport headers such as Host, Cookie, Connection, and Proxy headers are blocked.</p>
                </div>
              ) : null}
              {authType === 'oauth' ? (
                <div className="space-y-2">
                  <Label htmlFor="external-mcp-scopes">OAuth scopes <span className="font-normal text-muted-foreground">(optional)</span></Label>
                  <Input id="external-mcp-scopes" value={scopes.join(' ')} onChange={(event) => setScopes(event.target.value.split(/[\s,]+/).filter(Boolean))} placeholder="read write" />
                  <p className="text-xs text-muted-foreground">Helpin discovers the server's OAuth endpoints. Enter only scopes documented by the provider.</p>
                </div>
              ) : null}
            </>
          ) : null}
          {!isCustom && selected ? (
            <div className="space-y-2"><Label>OAuth access</Label><div className="grid gap-2 sm:grid-cols-2">{Array.from(new Set([...selected.default_scopes, ...selected.optional_scopes])).map((scope) => <label key={scope} className="flex items-start gap-2.5 rounded-lg border p-2.5 text-sm"><Checkbox checked={scopes.includes(scope)} disabled={selected.default_scopes.includes(scope)} onCheckedChange={(checked) => setScopes((current) => checked === true ? Array.from(new Set([...current, scope])) : current.filter((item) => item !== scope))} /><span><span className="block font-medium">{CUSTOMER_SCOPE_COPY[scope] ?? scope}</span><span className="font-mono text-[10px] text-muted-foreground">{scope}</span></span></label>)}</div></div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={close}>Cancel</Button>
          <Button
            onClick={() => void submit()}
            disabled={create.isPending || oauth.isPending || !selected || (isCustom && (
              !name.trim()
              || !endpointURL.trim()
              || (authType === 'bearer_token' && !bearerToken.trim())
              || (authType === 'headers' && !headersComplete)
            ))}
          >
            {create.isPending || oauth.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
            {selected?.auth_type === 'oauth' || isCustom && authType === 'oauth' ? 'Continue to authorize' : 'Connect server'}
          </Button>
        </DialogFooter>
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
