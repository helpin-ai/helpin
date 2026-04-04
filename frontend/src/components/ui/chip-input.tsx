import { useRef, type ClipboardEvent, type KeyboardEvent } from 'react';
import { Cancel01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

const DELIMITER_REGEX = /[,\n]+/;

function parseChips(raw: string): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const part of raw.split(DELIMITER_REGEX)) {
    const trimmed = part.trim();
    if (!trimmed) continue;
    const lower = trimmed.toLowerCase();
    if (!seen.has(lower)) {
      seen.add(lower);
      result.push(trimmed);
    }
  }
  return result;
}

type ChipInputProps = {
  value: string[];
  onValueChange: (value: string[]) => void;
  inputValue: string;
  onInputValueChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  disabled?: boolean;
  /** Normalize chip text before adding (e.g. lowercase for domains) */
  normalize?: (value: string) => string;
};

export function ChipInput({
  value,
  onValueChange,
  inputValue,
  onInputValueChange,
  placeholder,
  className,
  disabled = false,
  normalize,
}: ChipInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  const addChips = (raw: string) => {
    const trimmed = raw.trim();
    if (!trimmed) return;

    const incoming = parseChips(raw);
    if (incoming.length === 0) return;

    const existing = new Set(value.map((v) => v.toLowerCase()));
    const next = [...value];
    for (const chip of incoming) {
      const normalized = normalize ? normalize(chip) : chip;
      if (!existing.has(normalized.toLowerCase())) {
        existing.add(normalized.toLowerCase());
        next.push(normalized);
      }
    }
    onValueChange(next);
    onInputValueChange('');
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;

    if ((event.key === ',' || event.key === 'Enter') && inputValue.trim()) {
      event.preventDefault();
      addChips(inputValue);
      return;
    }

    if (event.key === 'Backspace' && !inputValue && value.length > 0) {
      event.preventDefault();
      onValueChange(value.slice(0, -1));
    }
  };

  const handlePaste = (event: ClipboardEvent<HTMLInputElement>) => {
    const pasted = event.clipboardData.getData('text');
    if (!pasted || !/[,\n]/.test(pasted)) return;
    event.preventDefault();
    addChips([inputValue, pasted].filter(Boolean).join('\n'));
  };

  return (
    <div
      className={cn(
        'flex min-h-10 w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] focus-within:border-ring focus-within:ring-ring/50 focus-within:ring-[3px]',
        disabled && 'cursor-not-allowed opacity-50',
        className,
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((chip) => (
        <Badge key={chip} variant="secondary" className="h-6 gap-1 rounded-full pl-2.5 pr-1 text-xs font-medium">
          <span className="max-w-[16rem] truncate">{chip}</span>
          <button
            type="button"
            className="rounded-full p-0.5 text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground disabled:pointer-events-none"
            onClick={(event) => {
              event.stopPropagation();
              onValueChange(value.filter((v) => v !== chip));
            }}
            disabled={disabled}
            aria-label={`Remove ${chip}`}
          >
            <Cancel01Icon className="h-3 w-3" />
          </button>
        </Badge>
      ))}
      <input
        ref={inputRef}
        type="text"
        value={inputValue}
        onChange={(event) => onInputValueChange(event.target.value)}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onBlur={() => {
          if (inputValue.trim()) addChips(inputValue);
        }}
        placeholder={value.length === 0 ? placeholder : undefined}
        disabled={disabled}
        autoComplete="off"
        className="min-w-[10rem] flex-1 border-0 bg-transparent p-0 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed"
      />
    </div>
  );
}
