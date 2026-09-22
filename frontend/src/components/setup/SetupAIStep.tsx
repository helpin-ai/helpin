import { useId, useState } from 'react';
import { ProviderIcon } from '@/components/agents/ProviderIcon';
import { QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { useAIConnections, useCreateAIConnection, useTestAIConnection } from '@/hooks/queries/useAIConnections';
import { providerMeta, type AIProviderKey } from '@/lib/aiProviders';
import { Loading01Icon, PlayCircleIcon } from '@/lib/icons';
import type { AIConnectionTestResult } from '@/lib/services/aiConnectionService';
import type { Capability } from '@/lib/capabilityTypes';
import { CapabilityActionView, SetupAdminHint, SetupResultMessage, type SetupResult } from './CapabilityActions';

/** API-key providers offered inline; the rest stay in AI settings. */
const SETUP_AI_PROVIDERS: AIProviderKey[] = ['openrouter', 'openai', 'anthropic'];

export type SetupAIStepProps = {
  capability: Capability;
  workspaceId: string;
  slug: string;
  canManage: boolean;
};

function describeAITest(result: AIConnectionTestResult): SetupResult {
  if (result.ok) {
    const model = result.model ? `${result.model} answered` : 'The provider answered';
    return { tone: 'positive', message: `Connected. ${model} in ${Math.round(result.latency_ms)} ms.` };
  }
  return { tone: 'negative', message: `The connection test failed: ${result.error || 'the provider did not answer.'}` };
}

function errorResult(prefix: string, error: unknown): SetupResult {
  return { tone: 'negative', message: `${prefix}: ${error instanceof Error ? error.message : 'try again.'}` };
}

/**
 * Gets AI working for the workspace. Without a workspace connection it adds
 * one with an API key and tests it immediately; with an untested or failing
 * connection it offers a live test.
 */
export function SetupAIStep({ capability, workspaceId, slug, canManage }: SetupAIStepProps) {
  const [result, setResult] = useState<SetupResult | null>(null);
  const testConnection = useTestAIConnection(workspaceId);
  const settled = capability.status === 'ready' || capability.status === 'unavailable';
  const connectionId = capability.action?.kind === 'test_ai_connection' ? capability.action.connection_id : undefined;

  const runTest = async (id: string) => {
    try {
      setResult(describeAITest(await testConnection.mutateAsync({ connectionId: id })));
    } catch (error) {
      setResult(errorResult('Couldn’t test the connection', error));
    }
  };

  let body = null;
  if (!settled) {
    if (!canManage) body = <SetupAdminHint />;
    else if (connectionId) {
      body = (
        <Button type="button" variant="outline" size="sm" disabled={testConnection.isPending} onClick={() => { setResult(null); void runTest(connectionId); }}>
          {testConnection.isPending
            ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />
            : <PlayCircleIcon className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />}
          {testConnection.isPending ? 'Testing…' : 'Test connection'}
        </Button>
      );
    } else {
      body = (
        <SetupAIConnect
          capability={capability}
          workspaceId={workspaceId}
          slug={slug}
          testing={testConnection.isPending}
          onResult={setResult}
          onCreated={runTest}
        />
      );
    }
  }

  return (
    <div className="space-y-2">
      {body}
      <SetupResultMessage result={result} />
    </div>
  );
}

function SetupAIConnect({
  capability,
  workspaceId,
  slug,
  testing,
  onResult,
  onCreated,
}: {
  capability: Capability;
  workspaceId: string;
  slug: string;
  testing: boolean;
  onResult: (result: SetupResult | null) => void;
  onCreated: (connectionId: string) => Promise<void>;
}) {
  const id = useId();
  const connections = useAIConnections(workspaceId);
  const create = useCreateAIConnection(workspaceId);
  const [provider, setProvider] = useState<AIProviderKey>(SETUP_AI_PROVIDERS[0]);
  const [apiKey, setApiKey] = useState('');

  if (connections.isLoading) return <Skeleton className="h-8 w-full max-w-md" />;
  const data = connections.data;
  const hasWorkspaceConnection = data?.connections.some((connection) => connection.scope === 'workspace') ?? false;
  // A workspace connection exists (for example it needs reconnecting, or no
  // default is set), or connections can't be listed: settings can fix it.
  if (!data || !data.enabled || hasWorkspaceConnection) {
    return <CapabilityActionView capability={capability} slug={slug} canManage />;
  }

  const supported = SETUP_AI_PROVIDERS.filter((key) => data.models.length === 0 || data.models.some((model) => model.provider === key));
  const providers = supported.length > 0 ? supported : SETUP_AI_PROVIDERS;
  const selected = providers.includes(provider) ? provider : providers[0];
  const busy = create.isPending || testing;

  const submit = async () => {
    const key = apiKey.trim();
    if (!key) {
      onResult({ tone: 'negative', message: 'Enter an API key to connect.' });
      return;
    }
    onResult(null);
    try {
      const login = await create.mutateAsync({
        name: providerMeta[selected].shortLabel,
        provider: selected,
        api_key: key,
        scope: 'workspace',
      });
      setApiKey('');
      await onCreated(login.connection.id);
    } catch (error) {
      onResult(errorResult('Couldn’t add the connection', error));
    }
  };

  return (
    <form
      className="grid max-w-xl gap-3 sm:grid-cols-[10rem_minmax(0,1fr)_auto] sm:items-end"
      onSubmit={(event) => { event.preventDefault(); void submit(); }}
    >
      <div className="space-y-1">
        <Label htmlFor={`${id}-provider`} className="text-[12px] text-quiet-text-secondary">Provider</Label>
        <Select value={selected} disabled={busy} onValueChange={(value) => setProvider(value as AIProviderKey)}>
          <SelectTrigger id={`${id}-provider`} aria-label="Provider" variant="underline" className="w-full px-0.5">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {providers.map((key) => (
              <SelectItem key={key} value={key} textValue={providerMeta[key].label}>
                <span className="flex items-center gap-2">
                  <ProviderIcon provider={key} className="h-3.5 w-3.5 shrink-0" />
                  <span>{providerMeta[key].label}</span>
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1">
        <Label htmlFor={`${id}-key`} className="text-[12px] text-quiet-text-secondary">API key</Label>
        <QuietUnderlineInput
          id={`${id}-key`}
          type="password"
          autoComplete="off"
          value={apiKey}
          disabled={busy}
          onChange={(event) => setApiKey(event.target.value)}
        />
      </div>
      <Button type="submit" size="sm" variant="outline" disabled={busy}>
        {busy && <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />}
        {create.isPending ? 'Connecting…' : testing ? 'Testing…' : 'Connect and test'}
      </Button>
      <p className="text-[12px] text-quiet-text-tertiary sm:col-span-3">
        Shared with the workspace and stored encrypted. Standard AI profiles use it automatically.
      </p>
    </form>
  );
}
