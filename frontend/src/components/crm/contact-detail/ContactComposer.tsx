import { useRef, useState, type KeyboardEvent } from 'react';
import {
  Calendar01Icon,
  Mail01Icon,
  StickyNote01Icon,
  TelephoneIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { CRMActivityType } from '@/lib/crmTypes';

type ComposerMode = 'note' | 'call' | 'meeting';

const MODES: { id: ComposerMode; label: string; activityType: CRMActivityType; icon: typeof StickyNote01Icon }[] = [
  { id: 'note', label: 'Note', activityType: 'note', icon: StickyNote01Icon },
  { id: 'call', label: 'Log call', activityType: 'call', icon: TelephoneIcon },
  { id: 'meeting', label: 'Log meeting', activityType: 'meeting', icon: Calendar01Icon },
];

const PLACEHOLDERS: Record<ComposerMode, string> = {
  note: 'Leave a note…',
  call: 'Summarize your call…',
  meeting: 'What happened in this meeting?',
};

export const contactComposerModeBarClassName = 'flex items-center gap-1 rounded-t-xl border-b border-border/60 px-2 py-1.5';
export const contactComposerTextareaClassName = 'block w-full resize-none border-0 bg-transparent px-4 py-3 text-sm leading-relaxed text-foreground placeholder:text-muted-foreground focus:outline-none';
export const contactComposerSaveBarClassName = 'flex items-center gap-1 border-t border-border/60 px-2 py-1.5';

interface ContactComposerProps {
  initialMode?: ComposerMode;
  onSubmit: (args: { activityType: CRMActivityType; subject: string; body?: string }) => Promise<void>;
  isPending?: boolean;
  onEmailClick?: () => void;
}

export function ContactComposer({
  initialMode = 'note',
  onSubmit,
  isPending,
  onEmailClick,
}: ContactComposerProps) {
  const [mode, setMode] = useState<ComposerMode>(initialMode);
  const [value, setValue] = useState('');
  const [focused, setFocused] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const activeMode = MODES.find((m) => m.id === mode) ?? MODES[0];
  const canSubmit = value.trim().length > 0 && !isPending;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    const text = value.trim();
    const firstLine = text.split('\n', 1)[0]?.slice(0, 120) || activeMode.label;
    const rest = text.includes('\n') ? text.slice(firstLine.length).trim() : '';
    try {
      await onSubmit({
        activityType: activeMode.activityType,
        subject: firstLine,
        body: rest || undefined,
      });
      setValue('');
      textareaRef.current?.blur();
    } catch {
      // parent handles error toast
    }
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div
      className={cn(
        'rounded-xl border border-border/70 bg-background transition-colors',
        focused && 'border-border',
      )}
    >
      <div className={contactComposerModeBarClassName}>
        {MODES.map((m) => {
          const Icon = m.icon;
          const active = mode === m.id;
          return (
            <button
              key={m.id}
              type="button"
              onClick={() => setMode(m.id)}
              className={cn(
                'inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium transition-colors',
                active
                  ? 'bg-muted text-foreground'
                  : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
              )}
            >
              <Icon className="h-3.5 w-3.5" />
              {m.label}
            </button>
          );
        })}
        {onEmailClick && (
          <button
            type="button"
            onClick={onEmailClick}
            className="inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
          >
            <Mail01Icon className="h-3.5 w-3.5" />
            Email
          </button>
        )}
      </div>
      <textarea
        ref={textareaRef}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={handleKeyDown}
        placeholder={PLACEHOLDERS[mode]}
        rows={focused || value ? 3 : 1}
        className={contactComposerTextareaClassName}
      />
      <div className={contactComposerSaveBarClassName}>
        <div className="flex-1" />
        <span className="mr-2 hidden font-mono text-[10.5px] text-muted-foreground/70 sm:inline">
          ⌘↵
        </span>
        <Button
          size="sm"
          className="h-7 px-3"
          onClick={handleSubmit}
          disabled={!canSubmit}
        >
          {isPending ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </div>
  );
}
