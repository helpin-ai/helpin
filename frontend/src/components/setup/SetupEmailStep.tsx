import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { useSendTestEmail } from '@/hooks/queries/useCapabilities';
import { Loading01Icon, Mail01Icon } from '@/lib/icons';
import type { Capability } from '@/lib/capabilityTypes';
import { CapabilityActionView, SetupAdminHint, SetupResultMessage, type SetupResult } from './CapabilityActions';

export type SetupEmailStepProps = {
  capability: Capability;
  workspaceId: string;
  slug: string;
  canManage: boolean;
};

/**
 * Verifies application email by sending a message to the signed-in user.
 * Server configuration gaps fall back to copyable guidance.
 */
export function SetupEmailStep({ capability, workspaceId, slug, canManage }: SetupEmailStepProps) {
  const sendTestEmail = useSendTestEmail(workspaceId);
  const [result, setResult] = useState<SetupResult | null>(null);
  const canSend = capability.action?.kind === 'send_test_email' && capability.status !== 'unavailable';

  const send = async () => {
    setResult(null);
    try {
      const outcome = await sendTestEmail.mutateAsync();
      if (outcome.ok) {
        setResult({
          tone: 'positive',
          message: outcome.recipient
            ? `Test email sent to ${outcome.recipient}. Check that it arrived.`
            : 'Test email sent. Check that it arrived.',
        });
      } else if (outcome.rate_limited) {
        setResult({ tone: 'negative', message: sentence(outcome.error) });
      } else {
        setResult({ tone: 'negative', message: `The test email failed: ${outcome.error ?? 'the mail server did not accept it.'}` });
      }
    } catch (error) {
      setResult({ tone: 'negative', message: `Couldn’t send the test email: ${error instanceof Error ? error.message : 'try again.'}` });
    }
  };

  return (
    <div className="space-y-2">
      {canSend ? (
        canManage ? (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <Button type="button" variant="outline" size="sm" disabled={sendTestEmail.isPending} onClick={() => void send()}>
              {sendTestEmail.isPending
                ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                : <Mail01Icon className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />}
              {sendTestEmail.isPending ? 'Sending…' : 'Send test email'}
            </Button>
            <span className="text-[12px] text-quiet-text-tertiary">Sends to the email on your account.</span>
          </div>
        ) : <SetupAdminHint />
      ) : (
        <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />
      )}
      <SetupResultMessage result={result} />
    </div>
  );
}

function sentence(text?: string) {
  const value = (text ?? 'Too many test emails. Wait a minute and try again.').trim();
  const capitalized = value.charAt(0).toUpperCase() + value.slice(1);
  return /[.!?]$/.test(capitalized) ? capitalized : `${capitalized}.`;
}
