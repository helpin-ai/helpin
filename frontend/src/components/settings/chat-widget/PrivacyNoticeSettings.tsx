import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';

interface PrivacyNoticeSettingsProps {
  enabled: boolean;
  policyUrl: string;
  text: string;
  error?: string;
  onEnabledChange: (value: boolean) => void;
  onPolicyUrlChange: (value: string) => void;
  onTextChange: (value: string) => void;
}

export function PrivacyNoticeSettings({ enabled, policyUrl, text, error, onEnabledChange, onPolicyUrlChange, onTextChange }: PrivacyNoticeSettingsProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <Label htmlFor="privacy-notice-enabled">Show privacy notice</Label>
          <p className="mt-1 text-xs text-muted-foreground">Shown before the first message in each new conversation.</p>
        </div>
        <Switch id="privacy-notice-enabled" checked={enabled} onCheckedChange={onEnabledChange} />
      </div>
      {enabled && <>
        <div className="space-y-2">
          <Label htmlFor="privacy-policy-url">Privacy Policy URL</Label>
          <Input id="privacy-policy-url" type="url" placeholder="https://example.com/privacy" value={policyUrl} onChange={event => onPolicyUrlChange(event.target.value)} aria-describedby={error ? 'privacy-notice-error' : undefined} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="privacy-notice-text">Notice text</Label>
          <Textarea id="privacy-notice-text" rows={2} maxLength={500} value={text} onChange={event => onTextChange(event.target.value)} />
          <p className="text-xs text-muted-foreground">The Privacy Policy link is added at the end.</p>
        </div>
      </>}
      {error && <p id="privacy-notice-error" role="alert" className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
