import { useId, useState } from 'react';
import { ProviderIcon } from '@/components/agents/ProviderIcon';
import { QuietUnderlineInput } from '@/components/design-system/quiet';
import { SetupAdminHint, SetupResultMessage, type SetupResult } from '@/components/setup/CapabilityActions';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useAIConnections,
  useCreateAIConnection,
  useReconnectAIConnection,
  useTestAIConnection,
} from '@/hooks/queries/useAIConnections';
import { providerMeta, type AIProviderKey } from '@/lib/aiProviders';
import { CheckmarkCircle02Icon, Loading01Icon } from '@/lib/icons';
import type { AIConnection } from '@/lib/services/aiConnectionService';
import { AI_SETTINGS_PATH_LABEL } from '@/lib/workspaceOnboardingFlow';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

const PROVIDERS: Array<{ key: AIProviderKey; note: string }> = [
  { key: 'openrouter', note: 'One key for models from many providers.' },
  { key: 'openai', note: 'GPT models, direct from OpenAI.' },
  { key: 'anthropic', note: 'Claude models, direct from Anthropic.' },
];

type ConnectAIStepProps = {
  workspaceId: string;
  canManage: boolean;
  onContinue: () => void;
};

/**
 * Community only: adds workspace API keys for OpenRouter, OpenAI and
 * Anthropic, each tested on the spot. Any one is enough; more let each
 * standard AI tier use its best provider directly.
 */
export function ConnectAIStep({ workspaceId, canManage, onContinue }: ConnectAIStepProps) {
  const connections = useAIConnections(workspaceId);
  const workspaceConnections = (connections.data?.connections ?? []).filter((connection) => connection.scope === 'workspace');
  const anyConnected = workspaceConnections.some((connection) => connection.status === 'connected');

  return (
    <div className="space-y-7">
      <p className="text-sm leading-relaxed text-muted-foreground">
        Add a key for one or more providers. Each key is tested right away. With more than one, Helpin uses each
        provider’s best model for the task.
      </p>
      {!canManage ? (
        <SetupAdminHint />
      ) : connections.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : (
        <ul className="divide-y divide-border border-y border-border">
          {PROVIDERS.map((provider) => (
            <ProviderRow
              key={provider.key}
              workspaceId={workspaceId}
              provider={provider.key}
              note={provider.note}
              connection={pickConnection(workspaceConnections, provider.key)}
            />
          ))}
        </ul>
      )}
      <p className="text-[12.5px] leading-5 text-muted-foreground">
        Keys are shared with the workspace and stored encrypted. You can add, replace or remove them later in{' '}
        {AI_SETTINGS_PATH_LABEL}.{!anyConnected && ' If you skip this, AI features stay off until a key is added.'}
      </p>
      <OnboardingActions>
        {anyConnected ? (
          <Button type="button" className="w-full sm:w-auto sm:min-w-32" onClick={onContinue}>Continue</Button>
        ) : (
          <OnboardingTextButton onClick={onContinue}>Skip for now</OnboardingTextButton>
        )}
      </OnboardingActions>
    </div>
  );
}

/** Prefers a connected workspace connection, else the provider's placeholder to fill. */
function pickConnection(connections: AIConnection[], provider: AIProviderKey) {
  const forProvider = connections.filter((connection) => connection.provider === provider);
  return forProvider.find((connection) => connection.status === 'connected') ?? forProvider[0];
}

function describeTest(ok: boolean, model: string, latencyMs: number, error?: string): SetupResult {
  if (ok) return { tone: 'positive', message: `Connected. ${model || 'The provider'} answered in ${Math.round(latencyMs)} ms.` };
  return { tone: 'negative', message: `The key was saved but the test failed: ${error || 'the provider did not answer.'}` };
}

function ProviderRow({
  workspaceId,
  provider,
  note,
  connection,
}: {
  workspaceId: string;
  provider: AIProviderKey;
  note: string;
  connection: AIConnection | undefined;
}) {
  const id = useId();
  const create = useCreateAIConnection(workspaceId);
  const reconnect = useReconnectAIConnection(workspaceId);
  const test = useTestAIConnection(workspaceId);
  const [apiKey, setApiKey] = useState('');
  // The key field stays folded away until the person chooses to add or replace a key.
  const [editing, setEditing] = useState(false);
  const [result, setResult] = useState<SetupResult | null>(null);
  const busy = create.isPending || reconnect.isPending || test.isPending;
  const connected = connection?.status === 'connected';
  const replacing = connected && editing;
  const label = providerMeta[provider].label;
  const shortLabel = providerMeta[provider].shortLabel;

  const submit = async () => {
    const key = apiKey.trim();
    if (!key) {
      setResult({ tone: 'negative', message: `Enter your ${label} to connect.` });
      return;
    }
    setResult(null);
    try {
      // Fill the workspace's existing connection for this provider (Community
      // creates keyless placeholders) rather than adding a duplicate.
      const saved = connection
        ? await reconnect.mutateAsync({ id: connection.id, apiKey: key })
        : await create.mutateAsync({ name: providerMeta[provider].shortLabel, provider, api_key: key, scope: 'workspace' });
      setApiKey('');
      setEditing(false);
      const outcome = await test.mutateAsync({ connectionId: saved.connection.id });
      setResult(describeTest(outcome.ok, outcome.model, outcome.latency_ms, outcome.error));
    } catch (error) {
      setResult({ tone: 'negative', message: `Couldn’t save the key: ${error instanceof Error ? error.message : 'try again.'}` });
    }
  };

  const cancel = () => { setEditing(false); setApiKey(''); setResult(null); };
  return (
    <li className="space-y-3 py-4" data-provider={provider}>
      <div className="flex items-start gap-3">
        <ProviderIcon provider={provider} className="mt-0.5 h-4 w-4 shrink-0" />
        <div className="min-w-0 flex-1">
          <p id={`${id}-name`} className="text-[13.5px] font-semibold leading-5">{shortLabel}</p>
          <p className="text-[12.5px] leading-5 text-muted-foreground">{note}</p>
        </div>
        {connected && !replacing && (
          <span className="flex shrink-0 items-center gap-1.5 text-[12.5px] font-medium">
            <CheckmarkCircle02Icon className="h-4 w-4" aria-hidden="true" />
            Connected
          </span>
        )}
        {!connected && !editing && (
          <OnboardingTextButton
            className="min-h-0 shrink-0 px-0 text-[12.5px]"
            aria-describedby={`${id}-name`}
            onClick={() => { setEditing(true); setResult(null); }}
          >
            Add key
          </OnboardingTextButton>
        )}
      </div>
      {editing ? (
        <form
          className="flex flex-col gap-3 pl-7 sm:flex-row sm:items-end"
          onSubmit={(event) => { event.preventDefault(); void submit(); }}
        >
          <div className="min-w-0 flex-1 space-y-1">
            <label htmlFor={`${id}-key`} className="text-[12px] text-muted-foreground">{label}</label>
            <QuietUnderlineInput
              id={`${id}-key`}
              type="password"
              autoComplete="off"
              // Opened by an explicit "Add key" or "Replace key" click, so focus goes to the field.
              autoFocus
              value={apiKey}
              disabled={busy}
              onChange={(event) => setApiKey(event.target.value)}
            />
          </div>
          <div className="flex items-center gap-2">
            <OnboardingTextButton onClick={cancel} disabled={busy}>
              Cancel
            </OnboardingTextButton>
            <Button type="submit" size="sm" variant="outline" disabled={busy}>
              {busy && <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />}
              {create.isPending || reconnect.isPending ? 'Saving…' : test.isPending ? 'Testing…' : replacing ? 'Replace and test' : 'Connect and test'}
            </Button>
          </div>
        </form>
      ) : connected ? (
        <div className="pl-7">
          <OnboardingTextButton className="min-h-0 px-0 text-[12.5px]" onClick={() => { setEditing(true); setResult(null); }}>
            Replace key
          </OnboardingTextButton>
        </div>
      ) : null}
      <div className="pl-7">
        <SetupResultMessage result={result} />
      </div>
    </li>
  );
}
