import { useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { resolveEmbedUrl, type ResolvedEmbed } from './embedResolver'
import { docsService } from '@/lib/services/docsService'
import { Loading01Icon } from '@/lib/icons'

export function InsertEmbedDialog({
  open,
  onOpenChange,
  onInsert,
  workspaceId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onInsert: (embed: ResolvedEmbed) => void
  workspaceId?: string
}) {
  const [url, setUrl] = useState('')
  const [serverResolved, setServerResolved] = useState<ResolvedEmbed | null>(null)
  const [resolving, setResolving] = useState(false)
  const fallbackResolved = useMemo(() => resolveEmbedUrl(url), [url])
  const resolved = serverResolved ?? fallbackResolved

  useEffect(() => {
    if (!open || !workspaceId || !fallbackResolved) {
      setServerResolved(null)
      setResolving(false)
      return
    }
    let cancelled = false
    setResolving(true)
    const timer = window.setTimeout(() => {
      docsService.resolveEmbed(workspaceId, fallbackResolved.url).then(({ data }) => {
        if (cancelled) return
        setServerResolved(data ? {
          provider: data.provider,
          title: data.title,
          description: data.description ?? fallbackResolved.description,
          url: data.url,
          image_url: data.image_url ?? null,
        } : null)
      }).finally(() => {
        if (!cancelled) setResolving(false)
      })
    }, 250)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [fallbackResolved, open, workspaceId])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Embed</DialogTitle>
        </DialogHeader>
        <Input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://..." autoFocus />
        {resolved && (
          <div className="flex gap-3 rounded-md border border-border bg-muted/20 px-3 py-2 text-sm">
            {resolved.image_url && (
              <div
                className="h-14 w-20 shrink-0 rounded border border-border/70 bg-muted bg-cover bg-center"
                style={{ backgroundImage: `url("${resolved.image_url.replace(/"/g, '\\"')}")` }}
                aria-hidden
              />
            )}
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 text-xs font-medium uppercase text-muted-foreground">
                <span>{resolved.provider}</span>
                {resolving && <Loading01Icon className="h-3 w-3 animate-spin" />}
              </div>
              <div className="truncate font-medium">{resolved.title}</div>
              <div className="truncate text-xs text-muted-foreground">{resolved.description}</div>
            </div>
          </div>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            type="button"
            disabled={!resolved}
            onClick={() => {
              if (!resolved) return
              onInsert(resolved)
              setUrl('')
              onOpenChange(false)
            }}
          >
            Insert
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
