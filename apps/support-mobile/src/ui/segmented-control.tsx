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
  /** `sm` is a compact variant for toolbars (e.g. the composer reply/note toggle). */
  size?: 'default' | 'sm'
}

export function SegmentedControl<T extends string>({
  segments,
  value,
  onChange,
  className,
  size = 'default',
}: SegmentedControlProps<T>) {
  const sm = size === 'sm'
  const thumbRadius = sm ? 'rounded-[6px]' : 'rounded-[8px]'
  return (
    <div
      className={cn(
        'flex items-center bg-muted',
        sm ? 'gap-0.5 rounded-[8px] p-0.5' : 'gap-1 rounded-[10px] p-1',
        className,
      )}
    >
      {segments.map((segment) => {
        const selected = segment.value === value
        return (
          <Pressable
            key={segment.value}
            haptic="selection"
            aria-pressed={selected}
            onPress={() => onChange(segment.value)}
            className={cn(
              'relative isolate flex flex-1 items-center justify-center gap-1.5 font-medium transition-colors',
              thumbRadius,
              sm ? 'px-2.5 py-0.5 text-[11px]' : 'px-3 py-1.5 text-footnote',
              selected ? 'text-foreground' : 'text-muted-foreground',
            )}
          >
            {selected && (
              <motion.div
                layoutId="segment-thumb"
                className={cn('absolute inset-0 -z-10 bg-background', thumbRadius)}
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
