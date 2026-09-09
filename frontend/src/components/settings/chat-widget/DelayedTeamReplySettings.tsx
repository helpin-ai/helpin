import { useEffect, useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  DEFAULT_DELAYED_TEAM_REPLY_MESSAGE,
  DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL,
  isValidDelayedTeamReplyMinutes,
} from './delayedTeamReply';

interface Props {
  minutes: number;
  message: string;
  messageNoEmail: string;
  onMinutesChange: (value: number) => void;
  onMessageChange: (value: string) => void;
  onMessageNoEmailChange: (value: string) => void;
}

export function DelayedTeamReplySettings({ minutes, message, messageNoEmail, onMinutesChange, onMessageChange, onMessageNoEmailChange }: Props) {
  const [minutesInput, setMinutesInput] = useState(String(minutes));
  useEffect(() => setMinutesInput(String(minutes)), [minutes]);
  const invalidMinutes = !isValidDelayedTeamReplyMinutes(Number(minutesInput));

  return (
    <div className="space-y-5 p-3">
      <div className="space-y-2">
        <Label htmlFor="delayed-team-reply-minutes">Wait before sending</Label>
        <div className="flex items-center gap-2">
          <Input
            id="delayed-team-reply-minutes"
            type="number"
            min={1}
            max={1440}
            step={1}
            value={minutesInput}
            aria-invalid={invalidMinutes}
            aria-describedby="delayed-team-reply-timing-help"
            className="w-24"
            onChange={(event) => {
              setMinutesInput(event.target.value);
              const value = Number(event.target.value);
              if (isValidDelayedTeamReplyMinutes(value)) onMinutesChange(value);
            }}
            onBlur={() => { if (invalidMinutes) setMinutesInput(String(minutes)); }}
          />
          <span className="text-sm text-muted-foreground">minutes</span>
        </div>
        <p id="delayed-team-reply-timing-help" className="text-xs text-muted-foreground">
          Sent automatically once if no teammate has replied within this time after the handoff, including outside business hours. Assignments, internal notes, and customer messages don’t reset the timer.
        </p>
        {invalidMinutes && <p role="alert" className="text-xs text-destructive">Enter a whole number from 1 to 1,440 minutes.</p>}
      </div>
      {[
        { id: 'delayed-team-reply-message', label: 'Email known', value: message, fallback: DEFAULT_DELAYED_TEAM_REPLY_MESSAGE, onChange: onMessageChange, collectEmail: false },
        { id: 'delayed-team-reply-message-no-email', label: 'Email missing', value: messageNoEmail, fallback: DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL, onChange: onMessageNoEmailChange, collectEmail: true },
      ].map((field) => (
        <div key={field.id} className="space-y-2">
          <Label htmlFor={field.id}>{field.label}</Label>
          <Textarea id={field.id} value={field.value} onChange={(event) => field.onChange(event.target.value)} placeholder={field.fallback} maxLength={2000} rows={3} />
          <div className="space-y-2 rounded-md border bg-muted/20 p-3 text-xs text-muted-foreground">
            <p className="font-medium">Preview</p>
            <p className="whitespace-pre-wrap">{field.value.trim() ? field.value : field.fallback}</p>
            {field.collectEmail && (
              <div className="flex flex-wrap items-center gap-2" aria-label="Email capture preview">
                <Input type="email" placeholder="Your email address" aria-label="Preview email address" disabled className="h-8 min-w-0 flex-1 bg-background text-xs" />
                <span className="rounded-md bg-primary px-3 py-2 text-xs font-medium text-primary-foreground">Notify me by email</span>
              </div>
            )}
          </div>
        </div>
      ))}
      <p className="text-xs text-muted-foreground">Email collection uses the visitor’s existing contact details. The update stops if a teammate replies or the conversation is resolved.</p>
    </div>
  );
}
