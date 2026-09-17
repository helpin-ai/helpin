import { QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Label } from '@/components/ui/label';
import type { SemanticConditionAvailability } from '@/lib/types';

export function FlowSemanticConditionField({ value, scheduled, disabled, availability, loading, onChange }: {
  availability?: SemanticConditionAvailability;
  loading?: boolean;
  value: string;
  scheduled: boolean;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const available = !loading && availability?.available === true;
  const unavailableReason = loading
    ? 'Checking semantic condition availability…'
    : availability?.reason === 'not_configured'
      ? 'Semantic conditions need Jev. Ask your administrator to configure Jev to enable them.'
      : availability?.reason === 'shadow'
        ? 'Jev is in evaluation mode. Ask your administrator to activate semantic conditions.'
        : availability?.reason === 'workspace_disabled'
          ? 'Semantic conditions are not enabled for this workspace. Ask your administrator to enable them.'
          : availability?.reason === 'disabled'
            ? 'Semantic conditions are disabled by your administrator.'
            : 'Semantic condition availability could not be confirmed. Reload to try again.';
  const error = scheduled && value.trim() ? 'Remove this condition before using a schedule.' : '';
  return (
    <div className="min-w-0 space-y-2 py-3">
      <Label htmlFor="flow-semantic-condition">Semantic condition (optional)</Label>
      <QuietUnderlineInput
        id="flow-semantic-condition"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled || !available || (scheduled && !value)}
        maxLength={500}
        placeholder="The task describes a customer-facing bug"
        aria-describedby="flow-semantic-condition-help flow-semantic-condition-error"
        aria-invalid={Boolean(error)}
      />
      <p id="flow-semantic-condition-help" className="text-xs text-quiet-text-secondary">
        {!available ? unavailableReason : scheduled ? 'Conditions require an event-triggered Flow.' : 'Checks event metadata and the task title and description, when available. The action runs only when the condition clearly matches. Uncertain or unavailable checks skip the action; results appear in Activity.'}
      </p>
      {!available && value.trim() && (
        <>
          <p className="text-xs text-quiet-text-secondary">
            Your saved condition is preserved. The action will be skipped while Jev is unavailable or in evaluation mode. Removing the condition lets the Flow run without this check.
          </p>
          <QuietTextAction type="button" disabled={disabled} onClick={() => onChange('')}>
            Remove condition
          </QuietTextAction>
        </>
      )}
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
