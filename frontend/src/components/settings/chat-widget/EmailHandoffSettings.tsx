import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { DEFAULT_EMAIL_HANDOFF_MESSAGE, DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS } from './emailHandoff';

interface Props {
  message: string;
  messageAfterHours: string;
  onMessageChange: (value: string) => void;
  onMessageAfterHoursChange: (value: string) => void;
}

export function EmailHandoffSettings({ message, messageAfterHours, onMessageChange, onMessageAfterHoursChange }: Props) {
  return (
    <div className="space-y-5 p-3">
      {[
        { id: 'email-handoff-message', label: 'During business hours', value: message, fallback: DEFAULT_EMAIL_HANDOFF_MESSAGE, onChange: onMessageChange },
        { id: 'email-handoff-message-after-hours', label: 'After hours', value: messageAfterHours, fallback: DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS, onChange: onMessageAfterHoursChange },
      ].map((field) => (
        <div key={field.id} className="space-y-2">
          <Label htmlFor={field.id}>{field.label}</Label>
          <Textarea id={field.id} value={field.value} onChange={(event) => field.onChange(event.target.value)} placeholder={field.fallback} maxLength={2000} rows={5} className="rounded-none border-0 border-b border-quiet-field bg-transparent px-0 shadow-none focus-visible:border-quiet-text-primary focus-visible:ring-0" />
        </div>
      ))}
      <p className="text-xs text-muted-foreground">
        Sent when the AI hands an email conversation to your team, with a greeting and a sign-off added. Use <code>{'{next_open}'}</code> for when you’re back. Customers who write in another language get it translated.
      </p>
    </div>
  );
}
