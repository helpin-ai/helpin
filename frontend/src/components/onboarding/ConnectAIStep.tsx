import { Button } from '@/components/ui/button';
import { SetupAIStep } from '@/components/setup/SetupAIStep';
import type { Capability } from '@/lib/capabilityTypes';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

type ConnectAIStepProps = {
  capability: Capability;
  workspaceId: string;
  slug: string;
  canManage: boolean;
  onContinue: () => void;
};

/**
 * Community only: connects the workspace to an AI provider with an API key,
 * or tests an existing connection. Reuses the Setup guide's AI step, which
 * reports the model and latency, or the provider's error, inline.
 */
export function ConnectAIStep({ capability, workspaceId, slug, canManage, onContinue }: ConnectAIStepProps) {
  const connected = capability.status === 'ready';
  return (
    <div className="space-y-7">
      {connected ? (
        <p className="text-sm leading-relaxed">
          AI is connected for this workspace.
        </p>
      ) : (
        <p className="text-sm leading-relaxed text-muted-foreground">
          {capability.status === 'unable_to_verify'
            ? 'An AI connection is set up but hasn’t been checked yet. Test it to make sure it answers.'
            : 'Add an API key from OpenRouter, OpenAI or Anthropic. Helpin tests it right away.'}
        </p>
      )}
      <SetupAIStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />
      {!connected && (
        <p className="text-[12.5px] leading-5 text-muted-foreground">
          If you skip this, AI features stay off. You can connect a provider later in Settings → AI.
        </p>
      )}
      <OnboardingActions>
        {connected ? (
          <Button type="button" className="w-full sm:w-auto sm:min-w-32" onClick={onContinue}>Continue</Button>
        ) : (
          <OnboardingTextButton onClick={onContinue}>Skip for now</OnboardingTextButton>
        )}
      </OnboardingActions>
    </div>
  );
}
