import { ChevronLeft } from 'lucide-react'
import { motion, useMotionValue, useTransform, type MotionValue } from 'motion/react'
import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'
import { Pressable } from './pressable'

export interface TopBarProps {
  title: string
  subtitle?: string
  onBack?: () => void
  trailing?: ReactNode
  large?: boolean
  /** Scroll offset motion value driving the large-title collapse; ignored when `large` is false. */
  scrollY?: MotionValue<number>
  className?: string
  /**
   * Overrides the compact title/subtitle text block with custom content
   * (e.g. a presence dot next to a customer name). Ignored when `large` is
   * true — large mode always shows the plain title/subtitle strings.
   */
  titleSlot?: ReactNode
  /** Makes the compact title area tappable (e.g. opening a context sheet). Ignored when `large` is true. */
  onTitlePress?: () => void
}

export function TopBar({
  title,
  subtitle,
  onBack,
  trailing,
  large,
  scrollY,
  className,
  titleSlot,
  onTitlePress,
}: TopBarProps) {
  // Always call the hooks (rules of hooks) even when `large`/`scrollY` aren't in play;
  // the fallback motion value just never changes, so the derived transforms stay static.
  const fallbackScrollY = useMotionValue(0)
  const y = scrollY ?? fallbackScrollY
  const largeTitleOpacity = useTransform(y, [0, 44], [1, 0])
  const largeTitleScale = useTransform(y, [0, 44], [1, 0.94])
  const compactTitleOpacity = useTransform(y, [0, 44], [0, 1])

  return (
    <div className={cn('sticky top-0 z-30 bg-background/95 backdrop-blur-sm', className)}>
      <div className="pt-[var(--safe-top)]">
        <div className="relative flex h-[52px] items-center px-2">
          <div className="z-10 flex min-w-[44px] items-center">
            {onBack && (
              <Pressable
                aria-label="Back"
                onPress={onBack}
                className="flex items-center justify-center rounded-full"
              >
                <ChevronLeft className="h-6 w-6" />
              </Pressable>
            )}
          </div>

          {large ? (
            <motion.div
              style={{ opacity: compactTitleOpacity }}
              className="pointer-events-none absolute inset-x-12 flex flex-col items-center text-center"
            >
              <span className="text-headline">{title}</span>
            </motion.div>
          ) : onTitlePress ? (
            <Pressable
              aria-label={title}
              onPress={onTitlePress}
              className="absolute inset-x-12 flex min-h-0 min-w-0 flex-col items-center justify-center text-center"
            >
              {titleSlot ?? (
                <>
                  <span className="text-headline">{title}</span>
                  {subtitle && <span className="text-footnote text-muted-foreground">{subtitle}</span>}
                </>
              )}
            </Pressable>
          ) : (
            <div className="pointer-events-none absolute inset-x-12 flex flex-col items-center text-center">
              {titleSlot ?? (
                <>
                  <span className="text-headline">{title}</span>
                  {subtitle && <span className="text-footnote text-muted-foreground">{subtitle}</span>}
                </>
              )}
            </div>
          )}

          <div className="z-10 ml-auto flex min-w-[44px] items-center justify-end gap-1">{trailing}</div>
        </div>

        {large && (
          <motion.div
            style={{ opacity: largeTitleOpacity, scale: largeTitleScale }}
            className="origin-left px-4 pb-2"
          >
            <span className="text-large-title">{title}</span>
            {subtitle && <p className="text-footnote text-muted-foreground">{subtitle}</p>}
          </motion.div>
        )}
      </div>
    </div>
  )
}
