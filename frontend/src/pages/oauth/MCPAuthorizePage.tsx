import { useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { BotIcon, Loading01Icon, LockIcon } from '@/lib/icons';
import { useAuthorizeMCP, useMCPAuthorizationRequest } from '@/hooks/queries/useMCP';
import type { MCPAuthorizationQuery, MCPAuthorizationRequest } from '@/lib/mcpTypes';

const SCOPE_LABELS: Record<string, string> = {
  'helpin.context.read': 'Read workspace context',
  'helpin.pm.read': 'Read tasks and project work',
  'helpin.pm.write': 'Create and update tasks',
  'helpin.docs.read': 'Read documents',
  'helpin.docs.write': 'Create and update documents',
  'helpin.crm.read': 'Read CRM records',
  'helpin.crm.write': 'Add notes and update CRM records',
  'helpin.support.read': 'Read support conversations',
  'helpin.agents.read': 'Read agents and run status',
  'helpin.agents.run': 'Start and cancel agent runs',
};

const TOOLSET_LABELS: Record<string, string> = {
  context: 'Workspace context', pm: 'Projects & tasks', docs: 'Docs', crm: 'CRM', support: 'Support', agents: 'Helpin agents',
};

export function MCPAuthorizePage({ query }: { query: MCPAuthorizationQuery }) {
  useTitle('Authorize AI Client');
  const request = useMCPAuthorizationRequest(query);

  return (
    <main className="min-h-screen bg-muted/20 px-4 py-10 sm:py-16">
      <div className="mx-auto w-full max-w-xl">
        {request.isLoading ? <ConsentSkeleton /> : request.isError || !request.data ? (
          <Card className="rounded-xl shadow-none">
            <CardHeader><CardTitle>Authorization request is invalid</CardTitle><CardDescription>The client request may have expired, used an unregistered redirect, or omitted PKCE.</CardDescription></CardHeader>
            <CardContent><Alert variant="destructive"><AlertTitle>Could not verify this client</AlertTitle><AlertDescription>{request.error?.message ?? 'Return to the AI client and start again.'}</AlertDescription></Alert></CardContent>
          </Card>
        ) : (
          <ConsentForm key={`${request.data.client.client_id}:${request.data.query.state}`} request={request.data} />
        )}
      </div>
    </main>
  );
}

function ConsentForm({ request }: { request: MCPAuthorizationRequest }) {
  const [workspaceId, setWorkspaceId] = useState(request.workspaces[0]?.id ?? '');
  const [scopes, setScopes] = useState(request.requested_scopes);
  const [toolsets, setToolsets] = useState(request.proposed_toolsets);
  const [readOnly, setReadOnly] = useState(request.read_only_recommended);
  const authorize = useAuthorizeMCP();
  const writeScope = (scope: string) => scope.endsWith('.write') || scope === 'helpin.agents.run';
  const effectiveScopes = readOnly ? scopes.filter((scope) => !writeScope(scope)) : scopes;
  const selectedWorkspace = request.workspaces.find((workspace) => workspace.id === workspaceId);

  const approve = async () => {
    try {
      const result = await authorize.mutateAsync({
        query: request.query,
        workspace_id: workspaceId,
        scopes: effectiveScopes,
        toolsets,
        read_only: readOnly,
      });
      window.location.assign(result.redirect_url);
    } catch {
      // The mutation error is rendered inline so the client redirect is never guessed.
    }
  };

  const deny = () => {
    const redirect = new URL(request.query.redirect_uri);
    redirect.searchParams.set('error', 'access_denied');
    redirect.searchParams.set('state', request.query.state);
    window.location.assign(redirect.toString());
  };

  return (
    <Card className="rounded-xl shadow-none">
      <CardHeader className="border-b">
        <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-lg border bg-background">
          <BotIcon className="h-5 w-5" />
        </div>
        <CardTitle>{request.client.client_name} wants to access Helpin</CardTitle>
        <CardDescription>Choose one workspace and review the exact access this client will receive.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6 pt-6">
        {request.workspaces.length === 0 ? (
          <Alert><AlertTitle>No eligible workspaces</AlertTitle><AlertDescription>MCP is disabled, or the requested scopes are not allowed in your workspaces.</AlertDescription></Alert>
        ) : (
          <>
            <div className="space-y-2">
              <Label htmlFor="oauth-workspace">Workspace</Label>
              <Select value={workspaceId} onValueChange={setWorkspaceId}>
                <SelectTrigger id="oauth-workspace"><SelectValue placeholder="Choose a workspace" /></SelectTrigger>
                <SelectContent>{request.workspaces.map((workspace) => <SelectItem key={workspace.id} value={workspace.id}>{workspace.name} · {workspace.role}</SelectItem>)}</SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">This connection cannot switch to another workspace later.</p>
            </div>

            <div className="flex items-start justify-between gap-5 rounded-lg border p-4">
              <div><p className="text-sm font-medium">Read-only connection</p><p className="mt-1 text-sm text-muted-foreground">Recommended. The client can find context but cannot change records or start agents.</p></div>
              <Switch checked={readOnly} onCheckedChange={setReadOnly} />
            </div>

            <div className="space-y-3">
              <div><p className="text-sm font-medium">Permissions</p><p className="text-sm text-muted-foreground">Remove anything this client does not need.</p></div>
              <div className="space-y-3">
                {request.requested_scopes.map((scope) => {
                  const disabled = readOnly && writeScope(scope);
                  return (
                    <label key={scope} className="flex items-start gap-3 text-sm">
                      <Checkbox
                        checked={!disabled && scopes.includes(scope)}
                        disabled={disabled}
                        onCheckedChange={(checked) => setScopes((values) => checked === true ? [...values, scope] : values.filter((value) => value !== scope))}
                      />
                      <span><span className={disabled ? 'text-muted-foreground line-through' : ''}>{SCOPE_LABELS[scope] ?? scope}</span>{writeScope(scope) ? <Badge variant="outline" className="ml-2">Write</Badge> : null}</span>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="space-y-3">
              <div><p className="text-sm font-medium">Product areas</p><p className="text-sm text-muted-foreground">Only tools in selected areas will be visible to the client.</p></div>
              <div className="grid gap-3 sm:grid-cols-2">
                {request.proposed_toolsets.map((toolset) => (
                  <label key={toolset} className="flex items-center gap-2.5 text-sm"><Checkbox checked={toolsets.includes(toolset)} onCheckedChange={(checked) => setToolsets((values) => checked === true ? [...values, toolset] : values.filter((value) => value !== toolset))} />{TOOLSET_LABELS[toolset] ?? toolset}</label>
                ))}
              </div>
            </div>

            <Alert>
              <LockIcon className="h-4 w-4" />
              <AlertTitle>Helpin remains authoritative</AlertTitle>
              <AlertDescription>Your current role, team access, enabled modules, and workspace policy are checked again on every tool call. You can revoke this connection in AI Clients settings.</AlertDescription>
            </Alert>
          </>
        )}

        {authorize.isError ? <Alert variant="destructive"><AlertTitle>Could not authorize this client</AlertTitle><AlertDescription>{authorize.error.message}</AlertDescription></Alert> : null}
      </CardContent>
      <CardFooter className="flex items-center justify-between border-t pt-5">
        <Button variant="ghost" onClick={deny}>Cancel</Button>
        <Button onClick={() => void approve()} disabled={!selectedWorkspace || effectiveScopes.length === 0 || toolsets.length === 0 || authorize.isPending}>
          {authorize.isPending ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
          Authorize {selectedWorkspace?.name ?? 'workspace'}
        </Button>
      </CardFooter>
    </Card>
  );
}

function ConsentSkeleton() {
  return <Card className="rounded-xl shadow-none"><CardHeader><Skeleton className="h-10 w-10" /><Skeleton className="h-7 w-80 max-w-full" /><Skeleton className="h-4 w-full" /></CardHeader><CardContent className="space-y-5"><Skeleton className="h-10 w-full" /><Skeleton className="h-20 w-full" /><Skeleton className="h-40 w-full" /></CardContent></Card>;
}
