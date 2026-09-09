import { useQuery } from '@tanstack/react-query';
import { supportService } from '@/lib/services/supportService';
import { unwrap } from '@/lib/queryUtils';
import { Switch } from '@/components/ui/switch';
import { QuietSection, QuietUnderlineInput, QuietTextAction } from '@/components/design-system/quiet';

interface Props {
  workspaceId: string;
  enabled: boolean;
  delayHours: number;
  closeHours: number;
  maxPerConversation: number;
  onEnabledChange: (value: boolean) => void;
  onDelayChange: (value: number) => void;
  onCloseChange: (value: number) => void;
  onMaxChange: (value: number) => void;
}

export function AIFollowUpSettings(props: Props) {
  const preview = useQuery({
    queryKey: ['support', props.workspaceId, 'follow-up-preview'],
    queryFn: () => supportService.previewConversationFollowUps(props.workspaceId).then(unwrap),
    enabled: false,
  });
  return (
    <QuietSection>
      <div className="flex items-center justify-between gap-4">
        <div>
          <label htmlFor="ai-follow-up-enabled" className="text-sm font-medium">Follow up on quiet conversations</label>
          <p className="text-xs text-muted-foreground">Ask one question about the customer’s issue, then close if they do not reply.</p>
        </div>
        <Switch id="ai-follow-up-enabled" checked={props.enabled} onCheckedChange={props.onEnabledChange} />
      </div>
      <QuietTextAction className="mt-3" onClick={() => { void preview.refetch(); }} disabled={preview.isFetching}>{preview.isFetching ? 'Checking conversations…' : 'Preview existing conversations'}</QuietTextAction>
      {preview.isError && <p role="alert" className="mt-2 text-xs text-quiet-warning">Unable to load preview. Try again.</p>}
      {preview.data && <div className="mt-2 text-xs text-muted-foreground">
        <p>{preview.data.candidates} conversations could be assessed using saved settings. Context review may exclude more. Up to {preview.data.daily_limit} assessments run per day.</p>
        {preview.data.sample.map(conversation => <p key={conversation.id} className="mt-1 truncate">#{conversation.display_id} {conversation.subject}</p>)}
      </div>}
      {props.enabled && (
        <div className="mt-4 space-y-4">
          {[
            { id: 'ai-follow-up-delay', label: 'Send after inactivity (hours)', value: props.delayHours, max: 720, change: props.onDelayChange },
            { id: 'ai-follow-up-close', label: 'Wait after follow-up (hours)', value: props.closeHours, max: 720, change: props.onCloseChange },
            { id: 'ai-follow-up-max', label: 'Maximum follow-ups per conversation', value: props.maxPerConversation, max: 5, change: props.onMaxChange },
          ].map(field => (
            <div key={field.id} className="flex flex-wrap items-center justify-between gap-3">
              <label htmlFor={field.id} className="text-sm">{field.label}</label>
              <QuietUnderlineInput id={field.id} type="number" min={1} max={field.max} step={1} value={field.value} className="w-24" onChange={event => {
                const value = Number(event.target.value);
                if (Number.isInteger(value) && value >= 1 && value <= field.max) field.change(value);
              }} />
            </div>
          ))}
          <p className="text-xs text-muted-foreground">For example: “Were you able to reconnect Gmail using those steps?” The message explains that this conversation closes after {props.closeHours} hours without a reply and can be reopened by replying.</p>
          <p className="text-xs text-muted-foreground">Only public AI replies are eligible. Human-owned conversations and outstanding work are excluded. Enabling includes eligible conversations active in the last 30 days, with up to 25 assessments per workspace each day. No-response closures are recorded separately from confirmed resolutions.</p>
        </div>
      )}
    </QuietSection>
  );
}
