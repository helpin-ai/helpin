import { useState } from 'react';
import { isDefaultWelcomeMessage, resolveWelcomeMessage } from '@helpin-ai/widget-core';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { HelpCircleIcon } from '@/lib/icons';

export function WelcomeMessageSettings({ value, aiFirst, onChange, onFocus }: {
  value: string;
  aiFirst: boolean;
  onChange: (value: string) => void;
  onFocus?: () => void;
}) {
  // Keep an editable copy while focused, so clearing the field doesn't replace
  // the text mid-keystroke. Merely focusing a default must not save an override.
  const [editingValue, setEditingValue] = useState<string | null>(null);
  const welcomeMessage = resolveWelcomeMessage(value, aiFirst);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-1.5">
          <Label htmlFor="welcome-msg" className="text-sm">Welcome message</Label>
          <Tooltip>
            <TooltipTrigger asChild>
              <button type="button" aria-label="About the welcome message" className="text-muted-foreground hover:text-foreground">
                <HelpCircleIcon className="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent>The default greeting adapts to whether AI or your team handles chat. Leave blank to use it.</TooltipContent>
          </Tooltip>
        </div>
        {!isDefaultWelcomeMessage(value) && (
          <Button type="button" variant="ghost" size="sm" className="h-auto px-1 py-0.5 text-xs text-muted-foreground" onClick={() => {
            setEditingValue(null);
            onChange('');
          }}>
            Reset to default
          </Button>
        )}
      </div>
      <Textarea
        id="welcome-msg"
        value={editingValue ?? welcomeMessage}
        rows={3}
        onFocus={() => { setEditingValue(welcomeMessage); onFocus?.(); }}
        onChange={(event) => {
          setEditingValue(event.target.value);
          onChange(event.target.value);
        }}
        onBlur={() => setEditingValue(null)}
      />
    </div>
  );
}
