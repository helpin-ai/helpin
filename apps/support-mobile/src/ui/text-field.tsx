import { useId, useState } from 'react'
import { cn } from '@mobile/lib/cn'

export interface TextFieldProps {
  label: string
  type?: string
  value: string
  onChange: (value: string) => void
  onBlur?: () => void
  error?: string
  autoComplete?: string
  name?: string
  disabled?: boolean
  className?: string
}

/**
 * 52px floating-label input. The label sits centered in the field until the
 * input is focused or has a value, then floats up into the top-left corner —
 * classic Material/iOS-settings-adjacent treatment. Height/radius are fixed
 * so a field never resizes when an error appears below it (the error line
 * lives outside the 52px box).
 */
export function TextField({
  label,
  type = 'text',
  value,
  onChange,
  onBlur,
  error,
  autoComplete,
  name,
  disabled,
  className,
}: TextFieldProps) {
  const [focused, setFocused] = useState(false)
  const id = useId()
  const errorId = `${id}-error`
  const floated = focused || value.length > 0

  return (
    <div className={cn('w-full', className)}>
      <div
        className={cn(
          'relative h-[52px] w-full rounded-xl border bg-background transition-colors',
          error ? 'border-destructive' : focused ? 'border-ring' : 'border-input',
        )}
      >
        <label
          htmlFor={id}
          className={cn(
            'pointer-events-none absolute left-3.5 origin-left text-muted-foreground transition-all duration-150 ease-out',
            floated ? 'top-[9px] scale-[0.78] text-caption tracking-normal' : 'top-1/2 -translate-y-1/2 text-body',
          )}
        >
          {label}
        </label>
        <input
          id={id}
          name={name}
          type={type}
          inputMode={type === 'email' ? 'email' : undefined}
          autoComplete={autoComplete}
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => {
            setFocused(false)
            onBlur?.()
          }}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? errorId : undefined}
          className={cn(
            'h-full w-full rounded-xl bg-transparent px-3.5 text-body text-foreground outline-none disabled:opacity-50',
            floated ? 'pt-[13px]' : 'pt-0',
          )}
        />
      </div>
      {error && (
        <p id={errorId} className="mt-1 px-1 text-footnote text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
