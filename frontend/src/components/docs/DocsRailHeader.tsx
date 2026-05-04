import { ArrowLeft01Icon, Cancel01Icon } from '@/lib/icons'
import { Button } from '@/components/ui/button'

interface DocsRailHeaderProps {
  title: string
  onBack?: () => void
  onClose: () => void
  count?: number
}

export function DocsRailHeader({ title, onBack, onClose, count }: DocsRailHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3 py-2">
      <div className="flex min-w-0 items-center gap-1.5">
        {onBack && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 w-7 shrink-0 p-0"
            onClick={onBack}
            aria-label="Back"
          >
            <ArrowLeft01Icon className="h-3.5 w-3.5" />
          </Button>
        )}
        <span className="truncate text-sm font-medium">{title}</span>
        {typeof count === 'number' && count > 0 && (
          <span className="shrink-0 rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
            {count}
          </span>
        )}
      </div>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-7 w-7 shrink-0 p-0"
        onClick={onClose}
        aria-label="Close panel"
      >
        <Cancel01Icon className="h-3.5 w-3.5" />
      </Button>
    </div>
  )
}
