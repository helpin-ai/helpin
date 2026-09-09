import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { QuietTextAction } from '@/components/design-system/quiet';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { recheckSuggestionRouting, type RoutingRecheckResult } from '@/lib/services/pmAISuggestionsService';

const outcomes: Record<RoutingRecheckResult['items'][number]['outcome'], string> = {
  internal: 'Moved to the recipient’s AI suggestions',
  customer: 'Customer-related · stays in CRM',
  uncertain: 'Needs clearer context · stays in CRM',
  missing_transcript: 'Transcript unavailable · stays in CRM',
  not_eligible: 'Has linked CRM work or no eligible recipient · stays in CRM',
  changed: 'Changed during this check · refresh to review',
  retry_needed: 'Could not classify · try again later',
};

export function RoutingRecheck({ ws, onComplete }: { ws: string; onComplete: () => void }) {
  const client = useQueryClient();
  const [result, setResult] = useState<RoutingRecheckResult>();
  const [error, setError] = useState('');
  const [upgrade, setUpgrade] = useState<UpgradeRequiredReason | null>(null);
  const check = useMutation({
    mutationFn: async () => {
      setError('');
      const response = await recheckSuggestionRouting(ws, result?.next_cursor);
      if (response.error || !response.data) throw new Error(response.error || 'Routing check unavailable');
      return response.data;
    },
    onSuccess: (data) => {
      setResult(data);
      setError(data.failure || '');
      if (data.billing_error) {
        const reason = getUpgradeRequiredReason(data.billing_error);
        if (reason) setUpgrade(reason);
        else setError('The routing check is paused. Check your workspace billing and AI usage settings, then try again.');
      }
      onComplete();
    },
    onError: (cause) => {
      const reason = getUpgradeRequiredReason(cause);
      if (reason) setUpgrade(reason);
      else setError('The routing check could not finish. Please try again.');
    },
    // A request can complete some items before a later error or disconnection.
    onSettled: () => {
      void client.invalidateQueries({ queryKey: ['pm', ws, 'ai-suggestions'] });
      void client.invalidateQueries({ queryKey: ['crm', ws] });
    },
  });

  return <div className="mb-5">
    <div className="flex justify-end">
      <QuietTextAction disabled={check.isPending} onClick={() => check.mutate()}>
        {check.isPending ? 'Checking routing…' : result?.next_cursor ? 'Check more' : 'Recheck routing'}
      </QuietTextAction>
    </div>
    {check.isPending && <p role="status" className="mt-2 text-sm text-quiet-text-tertiary">Checking up to three recorded meeting follow-ups. This may take a minute.</p>}
    {error && <p role="alert" className="mt-2 text-sm text-destructive">{error}</p>}
    {!check.isPending && result && (result.items.length > 0 || (!result.billing_error && !result.failure)) && <section aria-label="Routing check results" aria-live="polite" className="mt-3 border-y border-quiet-divider-strong py-3">
      {result.items.length ? <ul className="space-y-3">{result.items.map((item) => <li key={item.id} className="text-sm">
        <p className="break-words text-quiet-text-primary">{item.title}</p>
        <p className="mt-0.5 text-xs text-quiet-text-secondary">{outcomes[item.outcome] || 'Refresh to check this follow-up'}</p>
      </li>)}</ul> : <p className="text-sm text-quiet-text-secondary">No more unchecked meeting follow-ups found.</p>}
    </section>}
    <UpgradeRequiredDialog open={upgrade !== null} onOpenChange={(open) => { if (!open) setUpgrade(null); }} reason={upgrade} />
  </div>;
}
