import { motion } from 'motion/react'
import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'
import { haptic, type HapticKind } from '@mobile/lib/haptics'

export interface PressableProps {
  onPress?: () => void
  haptic?: HapticKind
  disabled?: boolean
  className?: string
  children: ReactNode
  'aria-label'?: string
}

export function Pressable({ onPress, haptic: hapticKind, disabled, className, children, ...rest }: PressableProps) {
  return (
    <motion.button
      type="button"
      disabled={disabled}
      whileTap={disabled ? undefined : { scale: 0.97, opacity: 0.85 }}
      transition={{ duration: 0.1 }}
      className={cn('min-h-[44px] min-w-[44px] touch-manipulation disabled:opacity-40', className)}
      onClick={() => {
        if (disabled) return
        if (hapticKind) haptic(hapticKind)
        onPress?.()
      }}
      {...rest}
    >
      {children}
    </motion.button>
  )
}
