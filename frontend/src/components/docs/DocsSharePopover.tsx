import { useId, useState } from 'react'
import { GlobeIcon, Link01Icon } from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard'

export function DocsSharePopover({ shared, shareToken, disabled, onToggle, helpCenter }: {
  helpCenter?: { published: boolean; liveUrl: string | null; canPreview: boolean; previewDisabledReason?: string; getPreviewUrl: () => Promise<string> }
  shared: boolean
  shareToken?: string | null
  disabled: boolean
  onToggle: (shared: boolean) => void
}) {
  const id = useId()
  const { copied, copy, error, reset } = useCopyToClipboard()
  const previewClipboard = useCopyToClipboard()
  const [creatingPreview, setCreatingPreview] = useState(false)
  const [previewError, setPreviewError] = useState(false)
  const copyPreview = async () => {
    if (!helpCenter?.canPreview || creatingPreview) return
    setCreatingPreview(true)
    setPreviewError(false)
    previewClipboard.reset()
    try {
      previewClipboard.copy(await helpCenter.getPreviewUrl())
    } catch {
      setPreviewError(true)
    } finally {
      setCreatingPreview(false)
    }
  }
  const url = helpCenter ? helpCenter.liveUrl : shared && shareToken ? `${window.location.origin}/share/${shareToken}` : null
  const publiclyAvailable = helpCenter ? helpCenter.published : shared
  return <Popover onOpenChange={() => { reset(); previewClipboard.reset(); setPreviewError(false) }}>
    <PopoverTrigger asChild>
      <Button type="button" variant="ghost" size="sm" className="gap-1.5 rounded-full text-quiet-text-secondary" aria-label="Share document" title={helpCenter ? 'Share article' : shared ? 'Public link is on' : 'Share document'}>
        {publiclyAvailable ? <GlobeIcon className="h-3.5 w-3.5" /> : <Link01Icon className="h-3.5 w-3.5" />}
        Share
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" sideOffset={6} collisionPadding={8} className="z-[60] w-80 max-w-[calc(100vw-1rem)] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto gap-4 p-4" aria-label="Share document">
      <div className="text-sm font-semibold">{helpCenter ? 'Share article' : 'Share document'}</div>
      {helpCenter ? (
        <div className="space-y-3">
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm">Live link</span>
              <QuickTooltip contentClassName="z-[70]" label={!helpCenter.published ? 'Publish first' : !url ? 'Link unavailable' : 'Copy published article URL'}>
                <span className="block" tabIndex={!helpCenter.published || !url ? 0 : undefined}>
                  <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs" aria-label="Copy live link" disabled={!helpCenter.published || !url} onClick={() => url && copy(url)}>{copied ? 'Copied!' : 'Copy'}</Button>
                </span>
              </QuickTooltip>
            </div>
            {url && <input aria-label="Live Help Center URL" value={url} readOnly onFocus={event => event.target.select()} className="w-full min-w-0 rounded-md border bg-muted/30 px-2 py-2 text-xs text-muted-foreground" />}
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm">Preview link</span>
              <QuickTooltip contentClassName="z-[70]" label={!helpCenter.canPreview ? helpCenter.previewDisabledReason || 'No unpublished changes' : creatingPreview ? 'Creating link…' : 'Expires in 15 minutes'}>
                <span className="block" tabIndex={!helpCenter.canPreview || creatingPreview ? 0 : undefined}>
                  <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs" aria-label="Copy preview link" disabled={!helpCenter.canPreview || creatingPreview} onClick={() => void copyPreview()}>{creatingPreview ? 'Copying…' : previewClipboard.copied ? 'Copied!' : 'Copy'}</Button>
                </span>
              </QuickTooltip>
            </div>
          </div>
          {helpCenter.canPreview && <p className="text-xs text-muted-foreground">Preview links expire in 15 minutes.</p>}
          {url && <Button asChild variant="link" size="sm" className="h-auto px-0 text-xs"><a href={url} target="_blank" rel="noopener noreferrer">Open article</a></Button>}
          {(error || previewClipboard.error || previewError) && <p role="alert" className="text-xs text-destructive">{previewError ? 'Could not create preview link. Try again.' : 'Could not copy link. Try again.'}</p>}
        </div>
      ) : <div>
        <div className="flex items-center justify-between gap-3">
          <label htmlFor={id} className="text-sm font-medium">Public link</label>
          <Switch id={id} checked={shared} onCheckedChange={onToggle} disabled={disabled} aria-describedby={`${id}-help`} />
        </div>
        <p id={`${id}-help`} className="mt-2 text-xs leading-relaxed text-muted-foreground">
          {shared ? 'Anyone with the link can view. No sign-in required.' : 'Turn on to let anyone with the link view this document.'}
        </p>
      </div>}
      {!helpCenter && url && <div className="border-t pt-3">
        <div className="flex min-w-0 items-center gap-2 rounded-md border bg-muted/30 p-1.5">
          <input aria-label="Public document link" value={url} readOnly onFocus={event => event.target.select()} className="min-w-0 flex-1 bg-transparent px-1 text-xs text-muted-foreground outline-none" />
          <Button type="button" size="sm" variant="outline" className="h-7 shrink-0 px-2 text-xs" onClick={() => copy(url)}>{copied ? 'Copied!' : 'Copy link'}</Button>
        </div>
        {error && <p role="alert" className="mt-2 text-xs text-destructive">Could not copy. Select the link and copy it manually.</p>}
      </div>}
      {helpCenter && shared && <div className="border-t pt-3">
        <div className="flex items-center justify-between gap-3">
          <label htmlFor={id} className="text-xs font-medium">Existing standalone link</label>
          <Switch id={id} checked={shared} onCheckedChange={onToggle} disabled={disabled} aria-describedby={`${id}-legacy-help`} />
        </div>
        <p id={`${id}-legacy-help`} className="mt-2 text-xs text-muted-foreground">The previously shared link is still active. Turn off to revoke access through that link.</p>
      </div>}
    </PopoverContent>
  </Popover>
}
