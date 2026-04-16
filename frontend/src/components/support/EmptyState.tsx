import type { ComponentType, SVGProps } from 'react'

interface EmptyStateProps {
  icon: ComponentType<SVGProps<SVGSVGElement>>
  title: string
  subtitle?: string
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
  background = 'none',
  className,
}: EmptyStateProps) {
  const bg = background === 'muted' ? 'bg-muted/30' : ''
  return (
    <div
      className={`flex min-h-[280px] flex-1 flex-col items-center justify-center gap-2 px-6 py-12 text-center ${bg} ${className ?? ''}`}
    >
      <Icon className="h-10 w-10 text-muted-foreground/30" />
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      {subtitle && (
        <p className="max-w-xs text-xs leading-relaxed text-muted-foreground/70">{subtitle}</p>
      )}
    </div>
  )
}
