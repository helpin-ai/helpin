import { QuietUnderlineInput } from '@/components/design-system/quiet';
import { Label } from '@/components/ui/label';

export function FlowSemanticConditionField({ value, scheduled, disabled, onChange }: {
  value: string;
  scheduled: boolean;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const error = scheduled && value.trim() ? 'Remove this condition before using a schedule.' : '';
  return (
    <div className="min-w-0 space-y-2 py-3">
      <Label htmlFor="flow-semantic-condition">Semantic condition (optional)</Label>
      <QuietUnderlineInput
        id="flow-semantic-condition"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled || (scheduled && !value)}
        maxLength={500}
        placeholder="The task describes a customer-facing bug"
        aria-describedby="flow-semantic-condition-help flow-semantic-condition-error"
        aria-invalid={Boolean(error)}
      />
      <p id="flow-semantic-condition-help" className="text-xs text-quiet-text-secondary">
        {scheduled ? 'Conditions require an event-triggered Flow.' : 'Checks event metadata and the task title and description, when available. The action runs only when the condition clearly matches. Uncertain or unavailable checks skip the action; results appear in Activity.'}
      </p>
      <p id="flow-semantic-condition-error" role={error ? 'alert' : undefined} className="text-xs text-destructive">{error}</p>
    </div>
  );
}

export function flowConditionOutcomeLabel(outcome: string) {
  const labels: Record<string, string> = {
    matched: 'Condition matched — action may proceed',
    no_match: 'Condition did not match — action skipped',
    uncertain: 'Condition was uncertain — action skipped',
    unavailable: 'Condition check unavailable — action skipped',
    shadow: 'Condition is in evaluation mode — action skipped',
    input_limit: 'Event exceeds the condition input limit — action skipped',
    stale: 'Flow or task changed during the check — action skipped',
  };
  return labels[outcome] ?? 'Condition did not allow the action';
}
