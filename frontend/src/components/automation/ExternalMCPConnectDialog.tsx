import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Favicon } from '@/components/ui/favicon';
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
import {
  useCreateExternalMCPServer,
  useStartExternalMCPOAuth,
} from '@/hooks/queries/useExternalMCP';
import {
  buildExternalMCPCreateRequest,
  externalMCPConnectSources,
  isValidExternalMCPURL,
  type ExternalMCPConnectSource,
} from '@/lib/externalMCPConnect';
import type {
  ExternalMCPAuthType,
  ExternalMCPProvider,
  ExternalMCPServer,
} from '@/lib/externalMCPTypes';
import {
  Alert01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Cancel01Icon,
  Delete01Icon,
  Globe02Icon,
  Key01Icon,
  LinkSquare01Icon,
  Loading01Icon,
  PlusSignIcon,
  Tick01Icon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';

type ConnectStep = 'pick' | 'configure' | 'pending' | 'done';
type CredentialHeader = { id: string; name: string; value: string };

type ExternalMCPConnectDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  providers: ExternalMCPProvider[];
  resolveServer: (serverId: string) => Promise<ExternalMCPServer | undefined>;
};

const AUTH_OPTIONS: Array<{ value: ExternalMCPAuthType; label: string; description: string }> = [
  { value: 'oauth', label: 'OAuth', description: 'Sign in through the provider' },
  { value: 'bearer_token', label: 'Bearer token', description: 'Send an API token securely' },
  { value: 'headers', label: 'Custom headers', description: 'Add one or more secret headers' },
  { value: 'none', label: 'No authentication', description: 'For public or protected networks' },
];

const CUSTOMER_SCOPE_COPY: Record<string, string> = {
  read: 'Read workspace data',
  'read:sensitive': 'Read sensitive data',
  write: 'Create and update data',
  'write:live': 'Apply changes to live campaigns',
  configure: 'Change workspace configuration',
};

const BLOCKED_HEADERS = new Set([
  'connection',
  'content-length',
  'cookie',
  'host',
  'proxy-authorization',
  'proxy-connection',
  'te',
  'transfer-encoding',
  'upgrade',
]);

const STEP_PROGRESS: Record<ConnectStep, string> = {
  pick: 'w-1/3',
  configure: 'w-2/3',
  pending: 'w-[88%]',
  done: 'w-full',
};

export function ExternalMCPConnectDialog({
  open,
  onOpenChange,
  workspaceId,
  providers,
  resolveServer,
}: ExternalMCPConnectDialogProps) {
  const create = useCreateExternalMCPServer(workspaceId);
  const oauth = useStartExternalMCPOAuth(workspaceId);
  const sources = useMemo(() => externalMCPConnectSources(providers), [providers]);
  const [step, setStep] = useState<ConnectStep>('pick');
  const [sourceKey, setSourceKey] = useState('');
  const [name, setName] = useState('');
  const [endpointURL, setEndpointURL] = useState('');
  const [authType, setAuthType] = useState<ExternalMCPAuthType>('headers');
  const [scopes, setScopes] = useState<string[]>([]);
  const [bearerToken, setBearerToken] = useState('');
  const [headers, setHeaders] = useState<CredentialHeader[]>([
    { id: 'header-1', name: 'X-API-Key', value: '' },
  ]);
  const [urlTouched, setURLTouched] = useState(false);
  const [authorizationURL, setAuthorizationURL] = useState('');
  const [createdServerID, setCreatedServerID] = useState('');
  const [pendingStage, setPendingStage] = useState<'approval' | 'tools'>('approval');
  const [toolCount, setToolCount] = useState(0);
  const popupRef = useRef<Window | null>(null);
  const selected = sources.find((source) => source.key === sourceKey);
  const isCustomerIO = selected?.provider === 'customer_io';
  const isSubmitting = create.isPending || oauth.isPending;

  useEffect(() => {
    if (step !== 'pending' || !createdServerID) return;

    const receiveOAuthResult = async (event: MessageEvent) => {
      if (event.origin !== window.location.origin) return;
      const data = event.data as { type?: string; status?: string; serverId?: string } | null;
      if (data?.type !== 'helpin.external-mcp.oauth' || data.serverId !== createdServerID) return;
      if (data.status !== 'connected') {
        toast.error('Authorization was not completed. Check the provider and try again.');
        onOpenChange(false);
        return;
      }

      setPendingStage('tools');
      try {
        const server = await resolveServer(createdServerID);
        setToolCount(server?.tools?.length ?? 0);
        setStep('done');
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Connected, but could not load the discovered tools');
        setStep('done');
      }
    };

    window.addEventListener('message', receiveOAuthResult);
    return () => window.removeEventListener('message', receiveOAuthResult);
  }, [createdServerID, onOpenChange, resolveServer, step]);

  const selectSource = (source: ExternalMCPConnectSource) => {
    setSourceKey(source.key);
    setName(source.preset ? source.name : '');
    setEndpointURL(source.endpointURL);
    setAuthType(source.authType);
    setScopes([...source.defaultScopes]);
    setBearerToken('');
    setHeaders([{ id: 'header-1', name: 'X-API-Key', value: '' }]);
    setURLTouched(false);
    setStep('configure');
  };

  const close = () => {
    if (isSubmitting) return;
    if (popupRef.current && !popupRef.current.closed) popupRef.current.close();
    popupRef.current = null;
    onOpenChange(false);
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) close();
  };

  const headersComplete = headers.length > 0 && headers.every((header) => header.name.trim() && header.value.trim());
  const headerNames = headers.map((header) => header.name.trim().toLowerCase()).filter(Boolean);
  const hasDuplicateHeader = new Set(headerNames).size !== headerNames.length;
  const blockedHeader = headerNames.find((header) => BLOCKED_HEADERS.has(header));
  const urlValid = selected ? isValidExternalMCPURL(endpointURL) : false;
  const canSubmit = Boolean(
    selected
    && name.trim()
    && urlValid
    && (isCustomerIO || (
      (authType !== 'bearer_token' || bearerToken.trim())
      && (authType !== 'headers' || (headersComplete && !hasDuplicateHeader && !blockedHeader))
    )),
  );

  const submit = async () => {
    if (!selected || !canSubmit) {
      setURLTouched(true);
      return;
    }

    const request = buildExternalMCPCreateRequest({
      source: selected,
      name,
      endpointURL,
      authType,
      scopes,
      bearerToken,
      headers,
    });
    const requiresOAuth = isCustomerIO || authType === 'oauth';
    const popup = requiresOAuth
      ? window.open('about:blank', 'helpin-external-mcp-oauth', 'popup,width=620,height=760')
      : null;
    popupRef.current = popup;

    try {
      const server = await create.mutateAsync(request);
      setCreatedServerID(server.id);
      if (!requiresOAuth) {
        if (server.status !== 'connected') {
          toast.error(server.last_error_message || 'Server saved, but Helpin could not discover its tools.');
          onOpenChange(false);
          return;
        }
        setToolCount(server.tools?.length ?? 0);
        setStep('done');
        return;
      }

      const separator = window.location.search ? '&' : '?';
      const returnPath = `${window.location.pathname}${window.location.search}${separator}external_mcp_popup=1`;
      const result = await oauth.mutateAsync({ serverId: server.id, returnPath });
      setAuthorizationURL(result.authorization_url);
      setPendingStage('approval');
      setStep('pending');
      if (popup) {
        popup.location.assign(result.authorization_url);
      } else {
        window.location.assign(result.authorization_url);
      }
    } catch (error) {
      if (popup && !popup.closed) popup.close();
      popupRef.current = null;
      toast.error(error instanceof Error ? error.message : 'Could not add the MCP server');
    }
  };

  const openAuthorization = () => {
    if (!authorizationURL) return;
    const popup = window.open(authorizationURL, 'helpin-external-mcp-oauth', 'popup,width=620,height=760');
    popupRef.current = popup;
    popup?.focus();
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="flex max-h-[min(90dvh,760px)] flex-col gap-0 overflow-hidden rounded-2xl p-0 shadow-2xl sm:max-w-[600px]"
        onEscapeKeyDown={(event) => { if (isSubmitting) event.preventDefault(); }}
        onPointerDownOutside={(event) => { if (isSubmitting) event.preventDefault(); }}
      >
        <DialogHeader className="relative shrink-0 gap-0 border-b px-6 pb-5 pt-6 text-left">
          {step === 'configure' ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute left-5 top-5 rounded-full"
              onClick={() => setStep('pick')}
            >
              <ArrowLeft01Icon className="h-4 w-4" />
              <span className="sr-only">Back to provider selection</span>
            </Button>
          ) : null}
          <div className={cn('pr-10', step === 'configure' && 'pl-10')}>
            <DialogTitle className="text-lg leading-6">
              {step === 'pick' && 'Connect an MCP server'}
              {step === 'configure' && 'Configure connection'}
              {step === 'pending' && 'Authorize connection'}
              {step === 'done' && 'Connection ready'}
            </DialogTitle>
            <DialogDescription className="mt-1 text-[13px] leading-5">
              {step === 'pick' && 'Choose a provider preset or bring your own remote MCP endpoint.'}
              {step === 'configure' && 'Confirm the endpoint and choose how Helpin should authenticate.'}
              {step === 'pending' && 'Complete authorization with the provider to finish setup.'}
              {step === 'done' && 'This MCP server is now available in your workspace.'}
            </DialogDescription>
          </div>
          <Button
            type="button"
            variant="secondary"
            size="icon-sm"
            className="absolute right-5 top-5 rounded-full text-muted-foreground"
            onClick={close}
            disabled={isSubmitting}
          >
            <Cancel01Icon className="h-4 w-4" />
            <span className="sr-only">Close</span>
          </Button>
          <div className="absolute inset-x-0 bottom-0 h-0.5 bg-muted">
            <div className={cn('h-full bg-primary transition-[width] duration-300', STEP_PROGRESS[step])} />
          </div>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {step === 'pick' ? (
            <PickSource sources={sources} onSelect={selectSource} />
          ) : null}
          {step === 'configure' && selected ? (
            <ConfigureConnection
              source={selected}
              name={name}
              onNameChange={setName}
              endpointURL={endpointURL}
              onEndpointChange={(value) => { setEndpointURL(value); setURLTouched(true); }}
              urlInvalid={urlTouched && !urlValid}
              authType={authType}
              onAuthTypeChange={setAuthType}
              scopes={scopes}
              onScopesChange={setScopes}
              bearerToken={bearerToken}
              onBearerTokenChange={setBearerToken}
              headers={headers}
              onHeadersChange={setHeaders}
              hasDuplicateHeader={hasDuplicateHeader}
              blockedHeader={blockedHeader}
              onChangeSource={() => setStep('pick')}
            />
          ) : null}
          {step === 'pending' && selected ? (
            <PendingConnection source={selected} stage={pendingStage} onOpenAgain={openAuthorization} />
          ) : null}
          {step === 'done' && selected ? (
            <ConnectedState source={selected} name={name} toolCount={toolCount} />
          ) : null}
        </div>

        <DialogFooter className="shrink-0 items-center border-t bg-muted/15 px-6 py-4 sm:justify-between">
          {step === 'pick' ? (
            <>
              <p className="hidden max-w-xs text-xs leading-5 text-muted-foreground sm:block">Credentials are encrypted and never returned by the API.</p>
              <div className="flex w-full flex-col-reverse gap-2 min-[440px]:w-auto min-[440px]:flex-row">
                <Button type="button" variant="outline" className="w-full min-[440px]:w-auto" onClick={close}>Cancel</Button>
                <Button type="button" className="w-full min-[440px]:w-auto" disabled={!selected} onClick={() => selected && setStep('configure')}>
                  Continue <ArrowRight01Icon className="ml-1.5 h-4 w-4" />
                </Button>
              </div>
            </>
          ) : null}
          {step === 'configure' ? (
            <>
              <p className="hidden max-w-[270px] text-xs leading-5 text-muted-foreground sm:block">Helpin only sends these credentials to this MCP server.</p>
              <div className="flex w-full flex-col-reverse gap-2 min-[440px]:w-auto min-[440px]:flex-row">
                <Button type="button" variant="outline" className="w-full min-[440px]:w-auto" onClick={close} disabled={isSubmitting}>Cancel</Button>
                <Button type="button" className="w-full min-[440px]:w-auto" onClick={() => void submit()} disabled={!canSubmit || isSubmitting}>
                  {isSubmitting ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
                  {isCustomerIO || authType === 'oauth' ? 'Continue to authorize' : 'Connect server'}
                  {!isSubmitting ? <ArrowRight01Icon className="ml-1.5 h-4 w-4" /> : null}
                </Button>
              </div>
            </>
          ) : null}
          {step === 'pending' ? (
            <div className="flex w-full justify-end">
              <Button type="button" variant="outline" onClick={close}>Cancel</Button>
            </div>
          ) : null}
          {step === 'done' ? (
            <div className="flex w-full justify-end">
              <Button type="button" onClick={close}>Close</Button>
            </div>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PickSource({ sources, onSelect }: { sources: ExternalMCPConnectSource[]; onSelect: (source: ExternalMCPConnectSource) => void }) {
  const presets = sources.filter((source) => source.preset);
  const custom = sources.find((source) => !source.preset);
  return (
    <div className="space-y-[22px] px-7 pb-6 pt-[22px]">
      <section className="space-y-2.5">
        <p className="font-mono text-[11px] font-medium uppercase leading-none tracking-[0.06em] text-muted-foreground">Provider presets</p>
        <div className="grid grid-cols-1 gap-2.5 min-[440px]:grid-cols-2">
          {presets.map((source) => (
            <SourceCard key={source.key} source={source} onSelect={onSelect} />
          ))}
        </div>
      </section>
      {custom ? (
        <section className="space-y-2.5">
          <p className="font-mono text-[11px] font-medium uppercase leading-none tracking-[0.06em] text-muted-foreground">Or bring your own</p>
          <Button
            type="button"
            variant="outline"
            className="h-auto w-full justify-start gap-3 rounded-xl px-3.5 py-3.5 text-left hover:border-primary/40 hover:bg-primary/[0.03]"
            onClick={() => onSelect(custom)}
          >
            <SourceMark source={custom} />
            <span className="min-w-0 flex-1">
              <span className="block text-[13.5px] font-semibold leading-[1.3] text-foreground">{custom.name}</span>
              <span className="mt-0.5 block whitespace-normal text-[12.5px] font-normal leading-[1.45] text-muted-foreground">{custom.description}</span>
            </span>
            <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          </Button>
        </section>
      ) : null}
    </div>
  );
}

function SourceCard({ source, onSelect }: { source: ExternalMCPConnectSource; onSelect: (source: ExternalMCPConnectSource) => void }) {
  return (
    <Button
      type="button"
      variant="outline"
      className="h-auto flex-col items-start gap-[5px] rounded-xl px-3.5 py-3 text-left hover:border-primary/40 hover:bg-primary/[0.03]"
      onClick={() => onSelect(source)}
    >
      <span className="flex items-center gap-2.5">
        <SourceMark source={source} />
        <span className="block text-[13.5px] font-semibold leading-[1.3] text-foreground">{sourceLabel(source)}</span>
      </span>
      <span className="min-w-0">
        <span className="block whitespace-normal text-xs font-normal leading-[1.45] text-muted-foreground">{source.description}</span>
      </span>
    </Button>
  );
}

function SourceMark({ source, size = 'md' }: { source: ExternalMCPConnectSource; size?: 'md' | 'lg' }) {
  const dimensions = size === 'lg' ? 'h-14 w-14' : source.preset ? 'h-[30px] w-[30px]' : 'h-[34px] w-[34px]';
  if (source.preset) {
    return (
      <Favicon
        url={source.websiteURL}
        name={source.monogram}
        size={size === 'lg' ? 128 : 64}
        className={cn('rounded-lg border-border/60 bg-muted/60', dimensions)}
        imageClassName="p-1"
        fallbackClassName={cn('font-mono font-medium normal-case', size === 'lg' ? 'text-sm' : 'text-[10px]')}
      />
    );
  }
  return (
    <span className={cn(
      'flex shrink-0 items-center justify-center rounded-lg border bg-background font-mono font-medium text-muted-foreground',
      dimensions,
      size === 'lg' ? 'text-lg' : 'text-sm',
    )}>
      <Globe02Icon className={size === 'lg' ? 'h-6 w-6' : 'h-5 w-5'} />
    </span>
  );
}

type ConfigureConnectionProps = {
  source: ExternalMCPConnectSource;
  name: string;
  onNameChange: (value: string) => void;
  endpointURL: string;
  onEndpointChange: (value: string) => void;
  urlInvalid: boolean;
  authType: ExternalMCPAuthType;
  onAuthTypeChange: (value: ExternalMCPAuthType) => void;
  scopes: string[];
  onScopesChange: (value: string[]) => void;
  bearerToken: string;
  onBearerTokenChange: (value: string) => void;
  headers: CredentialHeader[];
  onHeadersChange: (value: CredentialHeader[]) => void;
  hasDuplicateHeader: boolean;
  blockedHeader?: string;
  onChangeSource: () => void;
};

function ConfigureConnection({
  source,
  name,
  onNameChange,
  endpointURL,
  onEndpointChange,
  urlInvalid,
  authType,
  onAuthTypeChange,
  scopes,
  onScopesChange,
  bearerToken,
  onBearerTokenChange,
  headers,
  onHeadersChange,
  hasDuplicateHeader,
  blockedHeader,
  onChangeSource,
}: ConfigureConnectionProps) {
  const authLocked = source.provider === 'customer_io';
  return (
    <div className="space-y-6 px-7 py-[22px]">
      <div className="flex items-center gap-3 rounded-xl border bg-muted/20 p-3">
        <SourceMark source={source} />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">{sourceLabel(source)}</p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">{source.preset ? 'Provider preset' : 'Custom remote server'} · Streamable HTTP</p>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onChangeSource}>Change</Button>
      </div>

      <div className="space-y-2">
        <Label htmlFor="external-mcp-name">Display name</Label>
        <Input id="external-mcp-name" value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="My MCP server" maxLength={120} />
      </div>

      <div className="space-y-2">
        <Label htmlFor="external-mcp-url">MCP server URL</Label>
        <Input
          id="external-mcp-url"
          type="url"
          inputMode="url"
          autoCapitalize="off"
          spellCheck={false}
          readOnly={source.preset}
          value={endpointURL}
          onChange={(event) => onEndpointChange(event.target.value)}
          placeholder="https://mcp.example.com/mcp"
          aria-invalid={urlInvalid}
          className="font-mono read-only:text-muted-foreground"
        />
        <p className={cn('text-xs', urlInvalid ? 'text-destructive' : 'text-muted-foreground')}>
          {urlInvalid ? 'Enter a valid HTTPS MCP endpoint.' : source.preset ? 'Set by the provider preset.' : 'Use the HTTPS Streamable HTTP endpoint provided by the service.'}
        </p>
      </div>

      <fieldset className="space-y-3">
        <div className="flex items-baseline justify-between gap-3">
          <legend className="text-sm font-medium">Authentication</legend>
          {authLocked ? <span className="text-xs text-muted-foreground">Set by the preset</span> : null}
        </div>
        <div role="radiogroup" aria-label="Authentication method" className="grid grid-cols-1 gap-2 min-[440px]:grid-cols-2">
          {AUTH_OPTIONS.map((option) => {
            const selected = authType === option.value;
            return (
              <Button
                key={option.value}
                type="button"
                variant="outline"
                role="radio"
                aria-checked={selected}
                disabled={authLocked && !selected}
                className={cn(
                  'h-auto min-h-18 justify-start rounded-xl px-3 py-3 text-left disabled:opacity-45',
                  selected && 'border-primary bg-primary/[0.04] ring-1 ring-primary/20',
                )}
                onClick={() => onAuthTypeChange(option.value)}
              >
                <span className={cn('mr-2.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border', selected ? 'border-primary' : 'border-muted-foreground/40')}>
                  {selected ? <span className="h-2 w-2 rounded-full bg-primary" /> : null}
                </span>
                <span>
                  <span className="block text-sm font-medium text-foreground">{option.label}</span>
                  <span className="mt-0.5 block whitespace-normal text-xs font-normal leading-4 text-muted-foreground">{option.description}</span>
                </span>
              </Button>
            );
          })}
        </div>
      </fieldset>

      {authType === 'bearer_token' ? (
        <div className="space-y-2">
          <Label htmlFor="external-mcp-token">Bearer token</Label>
          <Input id="external-mcp-token" type="password" autoComplete="off" value={bearerToken} onChange={(event) => onBearerTokenChange(event.target.value)} placeholder="Paste token" className="font-mono" />
          <p className="text-xs text-muted-foreground">Sent as <span className="font-mono">Authorization: Bearer …</span></p>
        </div>
      ) : null}

      {authType === 'headers' ? (
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <Label>Credential headers</Label>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => onHeadersChange([...headers, { id: `header-${Date.now()}`, name: '', value: '' }])}
            >
              <PlusSignIcon className="mr-1.5 h-3.5 w-3.5" /> Add header
            </Button>
          </div>
          <div className="space-y-2">
            {headers.map((header, index) => (
              <div key={header.id} className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1fr)_auto] gap-2">
                <Input aria-label={`Header ${index + 1} name`} autoCapitalize="off" spellCheck={false} value={header.name} onChange={(event) => onHeadersChange(headers.map((item) => item.id === header.id ? { ...item, name: event.target.value } : item))} placeholder="X-API-Key" className="font-mono" />
                <Input aria-label={`Header ${index + 1} value`} type="password" autoComplete="off" value={header.value} onChange={(event) => onHeadersChange(headers.map((item) => item.id === header.id ? { ...item, value: event.target.value } : item))} placeholder="Secret value" />
                <Button type="button" size="icon" variant="ghost" disabled={headers.length === 1} onClick={() => onHeadersChange(headers.filter((item) => item.id !== header.id))}>
                  <Delete01Icon className="h-4 w-4" /><span className="sr-only">Remove header {index + 1}</span>
                </Button>
              </div>
            ))}
          </div>
          {hasDuplicateHeader ? <p className="text-xs text-destructive">Header names must be unique.</p> : null}
          {blockedHeader ? <p className="text-xs text-destructive"><span className="font-mono">{blockedHeader}</span> is a restricted transport header.</p> : null}
          {!hasDuplicateHeader && !blockedHeader ? <p className="text-xs text-muted-foreground">Transport headers such as Host, Cookie, Connection, and Proxy headers are blocked.</p> : null}
        </div>
      ) : null}

      {authType === 'oauth' && source.provider === 'custom' ? (
        <div className="space-y-3 rounded-xl border border-dashed bg-muted/15 p-4">
          <div className="flex gap-3">
            <Key01Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <div>
              <p className="text-sm font-medium">Authorization opens in a new window</p>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">Helpin discovers the server’s OAuth configuration and securely stores the resulting credential.</p>
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="external-mcp-scopes">OAuth scopes <span className="font-normal text-muted-foreground">(optional)</span></Label>
            <Input id="external-mcp-scopes" value={scopes.join(' ')} onChange={(event) => onScopesChange(event.target.value.split(/[\s,]+/).filter(Boolean))} placeholder="read write" className="font-mono" />
          </div>
        </div>
      ) : null}

      {authType === 'none' ? (
        <div className="flex gap-3 rounded-xl border border-amber-300/60 bg-amber-50/60 p-4 text-amber-900 dark:border-amber-900 dark:bg-amber-950/25 dark:text-amber-200">
          <Alert01Icon className="mt-0.5 h-4 w-4 shrink-0" />
          <p className="text-xs leading-5">Only use this for a public endpoint or a server protected by your private network.</p>
        </div>
      ) : null}

      {source.provider === 'customer_io' ? (
        <div className="space-y-3">
          <Label>OAuth access</Label>
          <div className="grid grid-cols-1 gap-2 min-[440px]:grid-cols-2">
            {Array.from(new Set([...source.defaultScopes, ...source.optionalScopes])).map((scope) => (
              <label key={scope} className="flex items-start gap-2.5 rounded-xl border p-3 text-sm">
                <Checkbox
                  checked={scopes.includes(scope)}
                  disabled={source.defaultScopes.includes(scope)}
                  onCheckedChange={(checked) => onScopesChange(checked === true ? Array.from(new Set([...scopes, scope])) : scopes.filter((item) => item !== scope))}
                />
                <span>
                  <span className="block font-medium">{CUSTOMER_SCOPE_COPY[scope] ?? scope}</span>
                  <span className="font-mono text-[10px] text-muted-foreground">{scope}</span>
                </span>
              </label>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function PendingConnection({ source, stage, onOpenAgain }: { source: ExternalMCPConnectSource; stage: 'approval' | 'tools'; onOpenAgain: () => void }) {
  return (
    <div className="flex min-h-[390px] flex-col items-center justify-center px-6 py-10 text-center">
      <div className="relative mb-6">
        <Loading01Icon className="h-20 w-20 animate-spin text-primary/30" />
        <div className="absolute inset-0 flex items-center justify-center"><SourceMark source={source} size="lg" /></div>
      </div>
      <h3 className="text-lg font-semibold">Waiting for provider approval</h3>
      <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">Complete the authorization in the window that opened. This page will update automatically.</p>
      <div className="mt-7 w-full max-w-sm rounded-xl border bg-muted/15 p-4 text-left">
        <PendingCheck label="Endpoint reachable" state="done" />
        <PendingCheck label="Provider approval" state={stage === 'approval' ? 'active' : 'done'} />
        <PendingCheck label="Discovering tools" state={stage === 'tools' ? 'active' : 'waiting'} last />
      </div>
      <Button type="button" variant="link" className="mt-4 text-xs" onClick={onOpenAgain}>
        <LinkSquare01Icon className="mr-1.5 h-3.5 w-3.5" /> Open authorization again
      </Button>
    </div>
  );
}

function PendingCheck({ label, state, last = false }: { label: string; state: 'done' | 'active' | 'waiting'; last?: boolean }) {
  return (
    <div className={cn('flex items-center gap-3 py-2', !last && 'border-b')}>
      <span className={cn('flex h-5 w-5 items-center justify-center rounded-full', state === 'done' && 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300', state === 'active' && 'bg-primary/10 text-primary', state === 'waiting' && 'border text-muted-foreground')}>
        {state === 'done' ? <Tick01Icon className="h-3.5 w-3.5" /> : null}
        {state === 'active' ? <span className="h-2 w-2 animate-pulse rounded-full bg-current" /> : null}
      </span>
      <span className={cn('text-sm', state === 'waiting' ? 'text-muted-foreground' : 'font-medium')}>{label}</span>
    </div>
  );
}

function ConnectedState({ source, name, toolCount }: { source: ExternalMCPConnectSource; name: string; toolCount: number }) {
  return (
    <div className="flex min-h-[390px] flex-col items-center justify-center px-6 py-12 text-center">
      <div className="mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
        <Tick01Icon className="h-9 w-9" />
      </div>
      <h3 className="text-xl font-semibold">{name || source.name} connected</h3>
      <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
        {toolCount === 1 ? '1 tool was discovered and is ready to configure.' : `${toolCount} tools were discovered and are ready to configure.`}
      </p>
      <div className="mt-7 flex items-center gap-2 rounded-full border bg-muted/20 px-4 py-2 text-xs text-muted-foreground">
        <Tick01Icon className="h-3.5 w-3.5 text-emerald-600" /> Credentials stored securely
      </div>
    </div>
  );
}

function sourceLabel(source: ExternalMCPConnectSource) {
  return source.provider === 'customer_io' ? 'Customer.io' : source.name;
}
