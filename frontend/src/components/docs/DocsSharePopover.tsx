import { useId } from 'react'
import { GlobeIcon, Link01Icon } from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard'

export function DocsSharePopover({ shared, shareToken, disabled, onToggle }: {
  shared: boolean
  shareToken?: string | null
  disabled: boolean
  onToggle: (shared: boolean) => void
}) {
  const id = useId()
  const { copied, copy, error, reset } = useCopyToClipboard()
  const url = shared && shareToken ? `${window.location.origin}/share/${shareToken}` : null
  return <Popover onOpenChange={() => reset()}>
    <PopoverTrigger asChild>
      <Button type="button" variant="ghost" size="sm" className="gap-1.5 rounded-full text-quiet-text-secondary" aria-label="Share document" title={shared ? 'Public link is on' : 'Share document'}>
        {shared ? <GlobeIcon className="h-3.5 w-3.5" /> : <Link01Icon className="h-3.5 w-3.5" />}
        Share
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" sideOffset={6} collisionPadding={8} className="z-[60] w-80 max-w-[calc(100vw-1rem)] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto gap-4 p-4" aria-label="Share document">
      <div className="text-sm font-semibold">Share document</div>
      <div>
        <div className="flex items-center justify-between gap-3">
          <label htmlFor={id} className="text-sm font-medium">Public link</label>
          <Switch id={id} checked={shared} onCheckedChange={onToggle} disabled={disabled} aria-describedby={`${id}-help`} />
        </div>
        <p id={`${id}-help`} className="mt-2 text-xs leading-relaxed text-muted-foreground">
          {shared ? 'Anyone with the link can view. No sign-in required.' : 'Turn on to let anyone with the link view this document.'}
        </p>
      </div>
      {url && <div className="border-t pt-3">
        <div className="flex min-w-0 items-center gap-2 rounded-md border bg-muted/30 p-1.5">
          <input aria-label="Public document link" value={url} readOnly onFocus={event => event.target.select()} className="min-w-0 flex-1 bg-transparent px-1 text-xs text-muted-foreground outline-none" />
          <Button type="button" size="sm" variant="outline" className="h-7 shrink-0 px-2 text-xs" onClick={() => copy(url)}>{copied ? 'Copied!' : 'Copy link'}</Button>
        </div>
        {error && <p role="alert" className="mt-2 text-xs text-destructive">Could not copy. Select the link and copy it manually.</p>}
      </div>}
    </PopoverContent>
  </Popover>
}
