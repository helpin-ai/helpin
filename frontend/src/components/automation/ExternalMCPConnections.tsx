import { useCallback, useState } from 'react';
import { toast } from 'sonner';
import { ExternalMCPConnectDialog } from '@/components/automation/ExternalMCPConnectDialog';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import {
  useDeleteExternalMCPServer,
  useExternalMCPProviders,
  useExternalMCPServers,
  useRefreshExternalMCPTools,
  useStartExternalMCPOAuth,
  useUpdateExternalMCPServer,
  useUpdateExternalMCPTools,
} from '@/hooks/queries/useExternalMCP';
import type { ExternalMCPServer, ExternalMCPTool } from '@/lib/externalMCPTypes';
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
  createOpen: boolean;
  onCreateOpenChange: (open: boolean) => void;
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

export function ExternalMCPConnections({
  workspaceId,
  workspaceName,
  canManageSettings,
  createOpen,
  onCreateOpenChange,
}: ExternalMCPConnectionsProps) {
  const providersQuery = useExternalMCPProviders(workspaceId);
  const enabled = providersQuery.data?.enabled === true;
  const serversQuery = useExternalMCPServers(workspaceId, enabled);
  const servers = serversQuery.data ?? [];
  const resolveServer = useCallback(async (serverId: string) => {
    const result = await serversQuery.refetch();
    return result.data?.find((server) => server.id === serverId);
  }, [serversQuery]);

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

      {serversQuery.isLoading ? <Skeleton className="h-48" /> : null}
      {serversQuery.isError ? <Alert variant="destructive"><AlertTitle>Could not load servers</AlertTitle><AlertDescription>{serversQuery.error.message}</AlertDescription></Alert> : null}
      {!serversQuery.isLoading && !serversQuery.isError && servers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/80 bg-card px-6 py-12">
          <div className="mx-auto max-w-xl text-center">
            <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <Globe02Icon className="h-5 w-5" />
            </div>
            <h3 className="mt-4 text-base font-semibold">No MCP servers connected</h3>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              {canManageSettings
                ? 'Connect a provider preset or any approved Streamable HTTP endpoint to make its tools available to Helpin agents.'
                : 'A workspace admin can connect an MCP server and make its tools available to Helpin agents.'}
            </p>
            {canManageSettings ? (
              <Button className="mt-5" size="sm" onClick={() => onCreateOpenChange(true)}>
                <PlusSignIcon className="h-4 w-4" /> Add your first server
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
      {!serversQuery.isLoading && !serversQuery.isError && servers.length > 0 ? (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(260px,0.8fr)]">
          <div className="min-w-0 space-y-3">
            {servers.map((server) => (
              <ExternalMCPServerCard
                key={server.id}
                workspaceId={workspaceId}
                server={server}
                canManageSettings={canManageSettings}
              />
            ))}
          </div>

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
      ) : null}

      {canManageSettings && createOpen ? (
        <ExternalMCPConnectDialog
          open={createOpen}
          onOpenChange={onCreateOpenChange}
          workspaceId={workspaceId}
          providers={providersQuery.data?.providers ?? []}
          resolveServer={resolveServer}
        />
      ) : null}
    </div>
  );
}

export function ExternalMCPAddButton({ workspaceId, onAdd }: { workspaceId: string; onAdd: () => void }) {
  const providersQuery = useExternalMCPProviders(workspaceId);
  return (
    <Button size="sm" onClick={onAdd} disabled={providersQuery.isLoading || providersQuery.data?.enabled !== true}>
      <PlusSignIcon className="mr-1.5 h-4 w-4" /> Add server
    </Button>
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
    <section className={cn('min-w-0 max-w-full overflow-hidden rounded-xl border bg-background transition-opacity', !server.enabled && 'opacity-70')}>
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
            {tools.length > 0 ? <Button size="sm" variant="ghost" onClick={() => setExpanded((value) => !value)}>{expanded ? 'Hide tools' : canManageSettings ? 'Manage tools' : 'View tools'}<ArrowDown01Icon className={cn('ml-1.5 h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')} /></Button> : null}
          </div>
        </div>
        {canManageSettings ? (
          <div className="flex shrink-0 items-center gap-3 self-start sm:flex-col sm:items-end sm:gap-1.5">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">{server.enabled ? 'Enabled' : 'Disabled'}</span>
              <Switch
                checked={server.enabled}
                aria-label={`${server.enabled ? 'Disable' : 'Enable'} ${server.name}`}
                disabled={updateServer.isPending}
                onCheckedChange={(enabled) => updateServer.mutate({ serverId: server.id, request: { enabled } })}
              />
            </div>
            <div className="flex items-center gap-0.5">
              {server.status === 'connected' ? (
                <Button size="icon-sm" variant="ghost" title={`Refresh tools for ${server.name}`} onClick={() => void sync()} disabled={refresh.isPending}>
                  <ArrowReloadHorizontalIcon className={cn('h-4 w-4', refresh.isPending && 'animate-spin')} />
                  <span className="sr-only">Refresh tools for {server.name}</span>
                </Button>
              ) : null}
              <Button size="icon-sm" variant="ghost" title={`Remove ${server.name}`} onClick={() => void remove()} disabled={deleteServer.isPending}>
                <Delete01Icon className="h-4 w-4" /><span className="sr-only">Remove {server.name}</span>
              </Button>
            </div>
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
    <div className="min-w-0 max-w-full overflow-hidden border-t bg-muted/15 px-4 py-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <div><p className="text-sm font-medium">Agent tools</p><p className="text-xs text-muted-foreground">Enabled tools appear under External MCP in the agent editor.</p></div>
        <Badge variant="outline">{tools.length} discovered</Badge>
      </div>
      <Table className="table-fixed">
        <colgroup>
          <col className="w-10" />
          <col />
          <col className="w-[116px]" />
        </colgroup>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="h-9 px-2"><span className="sr-only">Enabled</span></TableHead>
            <TableHead className="h-9 px-2 text-xs text-muted-foreground">Tool</TableHead>
            <TableHead className="h-9 px-2 text-xs text-muted-foreground">Access</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {tools.map((tool) => (
            <TableRow key={tool.id} className="hover:bg-transparent">
              <TableCell className="px-2 py-3">
                <Checkbox checked={tool.enabled} disabled={!canManageSettings || updateTools.isPending} onCheckedChange={(checked) => void update(tool.id, { enabled: checked === true })} aria-label={`Enable ${tool.remote_name}`} />
              </TableCell>
              <TableCell className="min-w-0 whitespace-normal px-2 py-3">
                <p className="break-all font-mono text-xs font-medium">{tool.remote_name}</p>
                <p className="mt-0.5 line-clamp-2 break-words text-xs leading-4 text-muted-foreground">{tool.description}</p>
              </TableCell>
              <TableCell className="px-2 py-3">
                <div className="flex w-fit rounded-lg bg-muted/60 p-0.5">
                  {(['read', 'write'] as const).map((access) => (
                    <Button
                      key={access}
                      type="button"
                      size="xs"
                      variant="ghost"
                      disabled={!canManageSettings || updateTools.isPending}
                      onClick={() => void update(tool.id, { access })}
                      className={cn('rounded-md px-2 capitalize', tool.access === access ? 'bg-background text-foreground shadow-sm hover:bg-background' : 'text-muted-foreground')}
                    >
                      {access}
                    </Button>
                  ))}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
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
