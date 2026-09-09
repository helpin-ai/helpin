import { Switch } from '@/components/ui/switch';
import { QuietSection, QuietUnderlineInput } from '@/components/design-system/quiet';

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
    <QuietSection>
      <div className="flex items-center justify-between gap-4">
        <div>
          <label htmlFor="ai-follow-up-enabled" className="text-sm font-medium">Follow up on quiet conversations</label>
          <p className="text-xs text-muted-foreground">Send a helpful check-in, then a final follow-up before closing if the customer does not reply.</p>
        </div>
        <Switch id="ai-follow-up-enabled" checked={props.enabled} onCheckedChange={props.onEnabledChange} />
      </div>
      {props.enabled && (
        <div className="mt-4 space-y-4">
          {[
            { id: 'ai-follow-up-delay', label: 'First follow-up after inactivity (hours)', value: props.delayHours, max: 720, change: props.onDelayChange },
            { id: 'ai-follow-up-second-delay', label: 'Second follow-up after first (hours)', value: props.secondDelayHours, max: 720, change: props.onSecondDelayChange },
            { id: 'ai-follow-up-close', label: 'Close after final follow-up (hours)', value: props.closeHours, max: 720, change: props.onCloseChange },
          ].map(field => (
            <div key={field.id} className="flex flex-wrap items-center justify-between gap-3">
              <label htmlFor={field.id} className="text-sm">{field.label}</label>
              <QuietUnderlineInput id={field.id} type="number" min={1} max={field.max} step={1} value={field.value} className="w-24" onChange={event => {
                const value = Number(event.target.value);
                if (Number.isInteger(value) && value >= 1 && value <= field.max) field.change(value);
              }} />
            </div>
          ))}
          <p className="text-xs text-muted-foreground">For example: “Were you able to reconnect Gmail using those steps?” The final follow-up says “We’ll be closing shortly, but you can reply anytime if you still need help.”</p>
          <p className="text-xs text-muted-foreground">Existing follow-up sequences keep their saved timings. Changes apply to new sequences.</p>
          <p className="text-xs text-muted-foreground">Only public AI replies are eligible. Human-owned conversations and outstanding work are excluded. Enabling includes eligible conversations active in the last 30 days, with up to 25 assessments per workspace each day. No-response closures are recorded separately from confirmed resolutions.</p>
        </div>
      )}
    </QuietSection>
  );
}
