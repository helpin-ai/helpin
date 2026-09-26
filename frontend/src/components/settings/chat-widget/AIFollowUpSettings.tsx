import { Switch } from '@/components/ui/switch';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { QuietUnderlineInput } from '@/components/design-system/quiet';

interface Props {
  enabled: boolean;
  delayHours: number;
  closeHours: number;
  secondDelayHours: number;
  onEnabledChange: (value: boolean) => void;
  onDelayChange: (value: number) => void;
  onCloseChange: (value: number) => void;
  onSecondDelayChange: (value: number) => void;
}

export function AIFollowUpSettings(props: Props) {
  return (
    <Card className="gap-5 rounded-lg border-border/70">
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="space-y-1">
          <CardTitle
            role="heading"
            aria-level={2}
            className="text-sm font-semibold text-quiet-text-primary"
          >
            <label htmlFor="ai-follow-up-enabled">Follow-ups</label>
          </CardTitle>
          <p
            id="ai-follow-up-help"
            className="text-sm text-quiet-text-secondary"
          >
            Check in when a customer goes quiet, then close the conversation if
            they don’t reply.
          </p>
        </div>
        <Switch
          id="ai-follow-up-enabled"
          aria-label="Follow up on quiet conversations"
          aria-describedby="ai-follow-up-help"
          checked={props.enabled}
          onCheckedChange={props.onEnabledChange}
        />
      </CardHeader>
      {props.enabled && (
        <CardContent className="space-y-4">
          <p className="text-xs text-quiet-text-secondary">
            Only conversations with public AI replies are eligible.
            Conversations handled by a teammate or with outstanding work are
            excluded.
          </p>
          <ol className="divide-y divide-quiet-divider">
            {[
              {
                id: 'ai-follow-up-delay',
                label: 'First check-in',
                description: 'After the customer stops replying',
                value: props.delayHours,
                change: props.onDelayChange,
              },
              {
                id: 'ai-follow-up-second-delay',
                label: 'Final follow-up',
                description:
                  'After the first check-in, if there is still no reply',
                value: props.secondDelayHours,
                change: props.onSecondDelayChange,
              },
              {
                id: 'ai-follow-up-close',
                label: 'Close conversation',
                description:
                  'After the final follow-up, if there is still no reply',
                value: props.closeHours,
                change: props.onCloseChange,
              },
            ].map((field, index) => (
              <li
                key={field.id}
                className="flex items-start gap-3 py-4 first:pt-0"
              >
                <span
                  aria-hidden="true"
                  className="pt-1 text-sm tabular-nums text-quiet-text-tertiary"
                >
                  {index + 1}.
                </span>
                <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-6 gap-y-3">
                  <div className="min-w-0 space-y-1">
                    <label htmlFor={field.id} className="text-sm font-medium">
                      {field.label}
                    </label>
                    <p
                      id={`${field.id}-help`}
                      className="text-xs text-quiet-text-secondary"
                    >
                      {field.description}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <QuietUnderlineInput
                      id={field.id}
                      type="number"
                      min={1}
                      max={720}
                      step={1}
                      value={field.value}
                      aria-describedby={`${field.id}-help ${field.id}-unit`}
                      className="w-16 text-right tabular-nums"
                      onChange={(event) => {
                        const value = Number(event.target.value);
                        if (
                          Number.isInteger(value) &&
                          value >= 1 &&
                          value <= 720
                        )
                          field.change(value);
                      }}
                    />
                    <span
                      id={`${field.id}-unit`}
                      className="text-xs text-quiet-text-secondary"
                    >
                      hours
                    </span>
                  </div>
                </div>
              </li>
            ))}
          </ol>
          <p className="text-xs text-quiet-text-secondary">
            Changes apply to new sequences. Existing follow-up sequences keep
            their saved timings.
          </p>
          <p className="text-xs text-quiet-text-secondary">
            Enabling also includes eligible conversations active in the last 30
            days.
          </p>
          <details className="border-t border-quiet-divider pt-4 text-xs text-quiet-text-secondary">
            <summary className="w-fit cursor-pointer font-medium text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
              Messages and limits
            </summary>
            <div className="mt-3 max-w-2xl space-y-2 leading-relaxed">
              <p>
                Check-ins refer to the customer’s question, for example: “Were
                you able to reconnect Gmail using those steps?”
              </p>
              <p>
                The final follow-up explains: “We’ll be closing shortly, but you
                can reply anytime if you still need help.”
              </p>
              <p>
                Up to 25 conversations are assessed per workspace each day.
                No-response closures are recorded separately from confirmed
                resolutions.
              </p>
            </div>
          </details>
        </CardContent>
      )}
    </Card>
  );
}
