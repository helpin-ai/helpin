/* eslint-disable react-refresh/only-export-components -- Existing shared recipient parser API is used by mail composers. */
import { useEffect, useRef, useState, type ClipboardEvent, type KeyboardEvent } from 'react';
import { Cancel01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

const EMAIL_DELIMITER_REGEX = /[;,\n\s]+/;
const SIMPLE_EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/i;

const normalizeEmail = (value: string) => value.trim().toLowerCase();

export const isValidEmailChip = (value: string) => SIMPLE_EMAIL_REGEX.test(normalizeEmail(value));

export const classifyEmailChipInput = (raw: string): { valid: string[]; invalid: string[] } => {
  const valid = new Set<string>();
  const invalid = new Set<string>();

  raw.split(EMAIL_DELIMITER_REGEX).forEach((part) => {
    const token = part.trim();
    if (!token) return;

    const email = normalizeEmail(token);
    if (isValidEmailChip(token)) {
      valid.add(email);
    } else {
      invalid.add(token);
    }
  });

  return {
    valid: [...valid],
    invalid: [...invalid],
  };
};

export const parseEmailChips = (raw: string): string[] => classifyEmailChipInput(raw).valid;

export const mergeEmailChips = (current: string[], incoming: string | string[]): string[] => {
  const merged = new Set(current.map(normalizeEmail).filter(Boolean));
  const nextValues = Array.isArray(incoming) ? incoming : parseEmailChips(incoming);

  nextValues.forEach((value) => {
    const email = normalizeEmail(value);
    if (isValidEmailChip(email)) {
      merged.add(email);
    }
  });

  return [...merged];
};

type EmailChipInputProps = {
  value: string[];
  onValueChange: (value: string[]) => void;
  inputValue: string;
  onInputValueChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  disabled?: boolean;
  autoComplete?: string;
  autoFocus?: boolean;
  ariaLabel?: string;
};

export function EmailChipInput({
  value,
  onValueChange,
  inputValue,
  onInputValueChange,
  placeholder,
  className,
  disabled = false,
  autoComplete = 'off',
  autoFocus = false,
  ariaLabel,
}: EmailChipInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [invalidEntries, setInvalidEntries] = useState<string[]>([]);

  useEffect(() => {
    if (autoFocus && !disabled) {
      inputRef.current?.focus();
    }
  }, [autoFocus, disabled]);

  const commitInput = (raw = inputValue) => {
    const trimmed = raw.trim();
    if (!trimmed) {
      setInvalidEntries([]);
      return false;
    }

    const { valid, invalid } = classifyEmailChipInput(raw);
    if (valid.length > 0) {
      onValueChange(mergeEmailChips(value, valid));
    }
    setInvalidEntries(invalid);
    onInputValueChange(invalid.join(', '));
    return valid.length > 0;
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;

    if ((event.key === ';' || event.key === ',' || event.key === 'Enter' || event.key === 'Tab') && inputValue.trim()) {
      event.preventDefault();
      commitInput();
      return;
    }

    if (event.key === 'Backspace' && !inputValue && value.length > 0) {
      event.preventDefault();
      onValueChange(value.slice(0, -1));
    }
  };

  const handlePaste = (event: ClipboardEvent<HTMLInputElement>) => {
    const pastedText = event.clipboardData.getData('text');
    if (!pastedText || !/[;,\n\s]/.test(pastedText)) {
      return;
    }

    event.preventDefault();
    commitInput([inputValue, pastedText].filter(Boolean).join('\n'));
  };

  return (
    <div
      className={cn(
        'flex min-h-10 w-full flex-wrap items-center gap-2 rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] focus-within:border-ring focus-within:ring-ring/50 focus-within:ring-[3px]',
        invalidEntries.length > 0 && 'border-destructive focus-within:border-destructive focus-within:ring-destructive/20',
        disabled && 'cursor-not-allowed opacity-50',
        className,
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((email) => (
        <Badge key={email} variant="secondary" className="h-7 max-w-full gap-1 rounded-full pl-2.5 pr-1 text-xs font-medium">
          <span className="min-w-0 max-w-[20rem] truncate">{email}</span>
          <button
            type="button"
            className="rounded-full p-0.5 text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground disabled:pointer-events-none"
            onClick={(event) => {
              event.stopPropagation();
              onValueChange(value.filter((item) => item !== email));
            }}
            disabled={disabled}
            aria-label={`Remove ${email}`}
          >
            <Cancel01Icon className="h-3 w-3" />
          </button>
        </Badge>
      ))}
      <input
        ref={inputRef}
        aria-label={ariaLabel}
        type="text"
        inputMode="email"
        value={inputValue}
        onChange={(event) => {
          setInvalidEntries([]);
          onInputValueChange(event.target.value);
        }}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onBlur={() => {
          if (inputValue.trim()) {
            commitInput();
          }
        }}
        placeholder={value.length === 0 ? placeholder : undefined}
        disabled={disabled}
        autoComplete={autoComplete}
        className="min-w-[12rem] flex-1 border-0 bg-transparent p-0 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed"
      />
      {invalidEntries.length > 0 ? (
        <p className="w-full text-xs text-destructive">
          {invalidEntries.length === 1 ? `Invalid email: ${invalidEntries[0]}` : `Invalid emails: ${invalidEntries.join(', ')}`}
        </p>
      ) : null}
    </div>
  );
}
