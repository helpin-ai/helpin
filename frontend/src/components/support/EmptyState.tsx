import type { ComponentType, ReactNode, SVGProps } from 'react'

interface EmptyStateProps {
  icon: ComponentType<SVGProps<SVGSVGElement>>
  title: string
  subtitle?: string
  actions?: ReactNode
  actionsClassName?: string
  /** Background — "none" (transparent) | "muted" (subtle tint). Default: none. */
  background?: 'none' | 'muted'
  className?: string
}

/**
 * Shared empty-state layout for the support inbox (conversation list,
 * message thread, thread contents, etc.). Keeps icon size, opacity,
 * spacing, and type scale consistent across surfaces while allowing
 * context-specific copy.
 */
export function EmptyState({
  icon: Icon,
  title,
  subtitle,
  actions,
  actionsClassName,
  background = 'none',
  className,
}: EmptyStateProps) {
  const bg = background === 'muted' ? 'bg-muted/30' : ''
  return (
    <div
      className={`flex min-h-[280px] flex-1 flex-col items-center justify-center px-6 py-12 ${bg} ${className ?? ''}`}
    >
      <div className="flex w-full max-w-[260px] flex-col items-center gap-2 text-center">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg border bg-background">
          <Icon className="h-4 w-4 text-muted-foreground" />
        </div>
        <p className="text-sm font-medium text-foreground">{title}</p>
        {subtitle && (
          <p className="text-xs leading-relaxed text-muted-foreground/70">{subtitle}</p>
        )}
        {actions && (
          <div className={`mt-2 flex flex-wrap justify-center gap-2 ${actionsClassName ?? ''}`}>
            {actions}
          </div>
        )}
      </div>
    </div>
  )
}
