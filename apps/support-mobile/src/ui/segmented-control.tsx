import { motion } from 'motion/react'
import { cn } from '@mobile/lib/cn'
import { Pressable } from './pressable'

export interface Segment<T extends string> {
  value: T
  label: string
  count?: number
}

export interface SegmentedControlProps<T extends string> {
  segments: Segment<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
}

export function SegmentedControl<T extends string>({
  segments,
  value,
  onChange,
  className,
}: SegmentedControlProps<T>) {
  return (
    <div className={cn('flex items-center gap-1 rounded-[10px] bg-muted p-1', className)}>
      {segments.map((segment) => {
        const selected = segment.value === value
        // Pressable's declared props don't include aria-pressed; SegmentedControl
        // still needs it on the rendered <button> for a11y + the RTL test, so it's
        // passed through the rest-spread with a targeted cast rather than widening
        // Pressable's public interface.
        const ariaProps = { 'aria-pressed': selected } as unknown as { 'aria-label'?: string }
        return (
          <Pressable
            key={segment.value}
            haptic="selection"
            onPress={() => onChange(segment.value)}
            className={cn(
              'relative isolate flex flex-1 items-center justify-center gap-1.5 rounded-[8px] px-3 py-1.5 text-footnote font-medium transition-colors',
              selected ? 'text-foreground' : 'text-muted-foreground',
            )}
            {...ariaProps}
          >
            {selected && (
              <motion.div
                layoutId="segment-thumb"
                className="absolute inset-0 -z-10 rounded-[8px] bg-background shadow-sm"
                transition={{ type: 'spring', bounce: 0.2, duration: 0.3 }}
              />
            )}
            <span>{segment.label}</span>
            {segment.count !== undefined && (
              <span className="tnum text-footnote text-muted-foreground">{segment.count}</span>
            )}
          </Pressable>
        )
      })}
    </div>
  )
}
