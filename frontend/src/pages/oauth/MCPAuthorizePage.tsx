import { useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Favicon } from '@/components/ui/favicon';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { BotIcon, Loading01Icon, LockIcon } from '@/lib/icons';
import { useAuthorizeMCP, useMCPAuthorizationRequest } from '@/hooks/queries/useMCP';
import type { MCPAuthorizationQuery, MCPAuthorizationRequest } from '@/lib/mcpTypes';
import { getMCPConsentAccess, hasMCPWriteScope, isMCPWriteScope } from '@/lib/mcpPolicy';

const SCOPE_LABELS: Record<string, string> = {
  'helpin.context.read': 'Read workspace context',
  'helpin.pm.read': 'Read tasks and project work',
  'helpin.pm.write': 'Create and update tasks',
  'helpin.docs.read': 'Read documents',
  'helpin.docs.write': 'Create and update documents',
  'helpin.docs.publish': 'Publish and unpublish Help Center articles',
  'helpin.crm.read': 'Read CRM records',
  'helpin.crm.write': 'Add notes and update CRM records',
  'helpin.support.read': 'Read support conversations',
  'helpin.agents.read': 'Read agents and run status',
  'helpin.agents.run': 'Start and cancel agent runs',
};

const TOOLSET_LABELS: Record<string, string> = {
  context: 'Workspace context', pm: 'Projects & tasks', docs: 'Docs', crm: 'CRM', support: 'Support', agents: 'Helpin agents',
};

function formatWorkspaceRole(role: string) {
  const normalized = role.trim().replaceAll('_', ' ');
  return normalized ? normalized.charAt(0).toUpperCase() + normalized.slice(1) : 'Member';
}

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
  const clientRequestedWrites = hasMCPWriteScope(request.requested_scopes);
  const [workspaceId, setWorkspaceId] = useState(request.workspaces[0]?.id ?? '');
  const [scopes, setScopes] = useState(request.requested_scopes);
  const [toolsets, setToolsets] = useState(request.proposed_toolsets);
  const [readOnly, setReadOnly] = useState(request.read_only_recommended || !clientRequestedWrites);
  const authorize = useAuthorizeMCP();
  const selectedWorkspace = request.workspaces.find((workspace) => workspace.id === workspaceId);
  const effectiveAccess = getMCPConsentAccess(scopes, toolsets, selectedWorkspace, readOnly);
  const workspaceRestricted = selectedWorkspace ? (
    (clientRequestedWrites && selectedWorkspace.read_only_required)
    || selectedWorkspace.allowed_scopes.length < request.requested_scopes.length
    || selectedWorkspace.allowed_toolsets.length < request.proposed_toolsets.length
  ) : false;

  const approve = async () => {
    try {
      const result = await authorize.mutateAsync({
        query: request.query,
        workspace_id: workspaceId,
        scopes: effectiveAccess.scopes,
        toolsets: effectiveAccess.toolsets,
        read_only: effectiveAccess.readOnly,
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
                <SelectTrigger
                  id="oauth-workspace"
                  className="min-h-14 w-full rounded-lg border-border bg-background px-3 py-2 data-[size=default]:h-auto hover:bg-muted/30"
                >
                  {selectedWorkspace ? (
                    <div className="flex min-w-0 flex-1 items-center gap-2.5 text-left">
                      <Favicon
                        src={selectedWorkspace.logo_url}
                        url={selectedWorkspace.website_url}
                        name={selectedWorkspace.name}
                        size={128}
                        className="h-8 w-8 rounded-md"
                        fallbackClassName="text-xs"
                      />
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-foreground">{selectedWorkspace.name}</p>
                        <p className="truncate text-xs text-muted-foreground">
                          {selectedWorkspace.slug} · {formatWorkspaceRole(selectedWorkspace.role)}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <SelectValue placeholder="Choose a workspace" />
                  )}
                </SelectTrigger>
                <SelectContent
                  position="popper"
                  align="start"
                  className="w-[var(--radix-select-trigger-width)]"
                >
                  {request.workspaces.map((workspace) => (
                    <SelectItem
                      key={workspace.id}
                      value={workspace.id}
                      textValue={`${workspace.name} ${workspace.slug} ${workspace.role}`}
                      className="py-2"
                    >
                      <div className="flex min-w-0 flex-1 items-center gap-2.5">
                        <Favicon
                          src={workspace.logo_url}
                          url={workspace.website_url}
                          name={workspace.name}
                          className="h-8 w-8 rounded-md"
                          size={128}
                          fallbackClassName="text-xs"
                        />
                        <div className="min-w-0 flex-1">
                          <p className="truncate font-medium">{workspace.name}</p>
                          <p className="truncate text-xs text-muted-foreground">{workspace.slug}</p>
                        </div>
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {formatWorkspaceRole(workspace.role)}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">This connection cannot switch to another workspace later.</p>
            </div>

            <div className="flex items-start justify-between gap-5 rounded-lg border p-4">
              <div>
                <p className="text-sm font-medium">Read-only connection</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {!clientRequestedWrites
                    ? 'Required because this client did not request any write permissions.'
                    : selectedWorkspace?.read_only_required
                      ? `Required by ${selectedWorkspace.name}'s workspace policy.`
                      : 'Recommended. Turn this off to approve the write permissions listed below.'}
                </p>
              </div>
              <Switch
                checked={effectiveAccess.readOnly}
                disabled={!effectiveAccess.canRequestWrites}
                onCheckedChange={setReadOnly}
              />
            </div>

            <div className="space-y-3">
              <div><p className="text-sm font-medium">Permissions</p><p className="text-sm text-muted-foreground">Remove anything this client does not need.</p></div>
              <div className="space-y-3">
                {request.requested_scopes.map((scope) => {
                  const blockedByWorkspace = !selectedWorkspace?.allowed_scopes.includes(scope);
                  const disabled = blockedByWorkspace
                    || (effectiveAccess.readOnly && isMCPWriteScope(scope));
                  return (
                    <label key={scope} className="flex items-start gap-3 text-sm">
                      <Checkbox
                        checked={!disabled && scopes.includes(scope)}
                        disabled={disabled}
                        onCheckedChange={(checked) => setScopes((values) => checked === true ? [...values, scope] : values.filter((value) => value !== scope))}
                      />
                      <span>
                        <span className={disabled ? 'text-muted-foreground line-through' : ''}>{SCOPE_LABELS[scope] ?? scope}</span>
                        {isMCPWriteScope(scope) ? <Badge variant="outline" className="ml-2">Write</Badge> : null}
                        {blockedByWorkspace ? <span className="ml-2 text-xs text-muted-foreground">Not allowed by workspace</span> : null}
                      </span>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="space-y-3">
              <div><p className="text-sm font-medium">Product areas</p><p className="text-sm text-muted-foreground">Only tools in selected areas will be visible to the client.</p></div>
              <div className="grid gap-3 sm:grid-cols-2">
                {request.proposed_toolsets.map((toolset) => {
                  const blockedByWorkspace = !selectedWorkspace?.allowed_toolsets.includes(toolset);
                  return (
                    <label key={toolset} className="flex items-center gap-2.5 text-sm">
                      <Checkbox
                        checked={!blockedByWorkspace && toolsets.includes(toolset)}
                        disabled={blockedByWorkspace}
                        onCheckedChange={(checked) => setToolsets((values) => checked === true ? [...values, toolset] : values.filter((value) => value !== toolset))}
                      />
                      <span className={blockedByWorkspace ? 'text-muted-foreground line-through' : ''}>{TOOLSET_LABELS[toolset] ?? toolset}</span>
                    </label>
                  );
                })}
              </div>
            </div>

            {workspaceRestricted ? (
              <Alert>
                <LockIcon className="h-4 w-4" />
                <AlertTitle>Workspace restrictions applied</AlertTitle>
                <AlertDescription>
                  The connection will receive only the permissions this workspace allows. Change workspace MCP permissions before authorizing if this client needs broader access.
                </AlertDescription>
              </Alert>
            ) : null}

            <Alert>
              <LockIcon className="h-4 w-4" />
              <AlertTitle>Helpin remains authoritative</AlertTitle>
              <AlertDescription>Your current role, team access, enabled modules, and workspace policy are checked again on every tool call. You can revoke this connection in MCP settings.</AlertDescription>
            </Alert>
          </>
        )}

        {authorize.isError ? <Alert variant="destructive"><AlertTitle>Could not authorize this client</AlertTitle><AlertDescription>{authorize.error.message}</AlertDescription></Alert> : null}
      </CardContent>
      <CardFooter className="flex items-center justify-between border-t pt-5">
        <Button variant="ghost" onClick={deny}>Cancel</Button>
        <Button onClick={() => void approve()} disabled={!selectedWorkspace || effectiveAccess.scopes.length === 0 || effectiveAccess.toolsets.length === 0 || authorize.isPending}>
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
