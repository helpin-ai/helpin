import { useEffect, useMemo, useState } from 'react'
import {
  Building03Icon,
  CheckListIcon,
  DollarCircleIcon,
  FolderKanbanIcon,
  Loading01Icon,
  Message01Icon,
  UserIcon,
} from '@/lib/icons'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { QuietSearchInput } from '@/components/design-system/quiet'
import type { EntityEmbedAttrs, DocsEntityEmbedType } from './EntityEmbedExtension'
import {
  entityTypeLabel,
  entityTypePluralLabel,
  searchDocsEntityItems,
  type DocsEntitySearchType,
  type DocsEntitySearchItem,
} from './entitySearch'
import { cn } from '@/lib/utils'

interface EntityEmbedDialogProps {
  open: boolean
  workspaceId: string
  fixedEntityType?: DocsEntityEmbedType
  onOpenChange: (open: boolean) => void
  onSelect: (attrs: EntityEmbedAttrs) => void
}

type EntityEmbedSearchItem = DocsEntitySearchItem & { entityType: DocsEntityEmbedType }

function isEmbedEntityType(type: DocsEntitySearchType): type is DocsEntityEmbedType {
  return type !== 'document'
}

function itemIcon(type: DocsEntitySearchType) {
  if (type === 'epic') return FolderKanbanIcon
  if (type === 'support_conversation') return Message01Icon
  if (type === 'deal') return DollarCircleIcon
  if (type === 'contact') return UserIcon
  if (type === 'company') return Building03Icon
  return CheckListIcon
}

function entityDialogLabel(type: DocsEntityEmbedType) {
  return entityTypeLabel(type)
}

function entityDialogPluralLabel(type: DocsEntityEmbedType) {
  return entityTypePluralLabel(type)
}

function entityDialogPlaceholder(type?: DocsEntityEmbedType) {
  if (!type) return 'Search tasks, epics, deals, contacts, companies, conversations...'
  if (type === 'support_conversation') return 'Search conversations...'
  return `Search ${entityDialogPluralLabel(type)}...`
}

function entityDialogEmptyMessage(query: string, loading: boolean, type?: DocsEntityEmbedType) {
  if (query.trim().length < 2) {
    if (!type) return 'Search tasks, epics, CRM records, or support conversations.'
    return `Search ${entityDialogPluralLabel(type)} to embed.`
  }
  if (loading) return 'Searching...'
  if (!type) return 'No matching entities found.'
  return `No matching ${entityDialogPluralLabel(type)} found.`
}

function itemTypeLabel(type: DocsEntityEmbedType) {
  return entityTypeLabel(type)
}

export async function searchEntityEmbedItems(
  workspaceId: string,
  query: string,
  fixedEntityType?: DocsEntityEmbedType,
): Promise<{ items: EntityEmbedSearchItem[]; error: string | null }> {
  const result = await searchDocsEntityItems({ workspaceId, query, fixedEntityType })
  return {
    items: result.items.filter((item): item is EntityEmbedSearchItem => isEmbedEntityType(item.entityType)),
    error: result.error,
  }
}

export function EntityEmbedDialog({ open, workspaceId, fixedEntityType, onOpenChange, onSelect }: EntityEmbedDialogProps) {
  const [query, setQuery] = useState('')
  const [items, setItems] = useState<EntityEmbedSearchItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      setQuery('')
      setItems([])
      setError(null)
    }
  }, [open])

  useEffect(() => {
    if (!open || !workspaceId) return
    const trimmed = query.trim()
    if (trimmed.length < 2) {
      setItems([])
      setError(null)
      return
    }

    let cancelled = false
    const timer = window.setTimeout(async () => {
      setLoading(true)
      setError(null)
      try {
        const result = await searchEntityEmbedItems(workspaceId, trimmed, fixedEntityType)
        if (cancelled) return
        setItems(result.items)
        if (result.error) setError(result.error)
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Search failed')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }, 180)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [fixedEntityType, open, query, workspaceId])

  const emptyMessage = useMemo(() => {
    return entityDialogEmptyMessage(query, loading, fixedEntityType)
  }, [fixedEntityType, loading, query])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{fixedEntityType ? `Embed ${entityDialogLabel(fixedEntityType)}` : 'Embed Entity'}</DialogTitle>
        </DialogHeader>
        <QuietSearchInput
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={entityDialogPlaceholder(fixedEntityType)}
          autoFocus
        />
        <div className="max-h-80 overflow-y-auto rounded-lg border border-border/70 p-1">
          {items.length === 0 ? (
            <div className="flex min-h-24 items-center justify-center px-4 text-sm text-muted-foreground">
              {loading ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
              {emptyMessage}
            </div>
          ) : (
            <div className="space-y-1">
              {items.map((item) => {
                const Icon = itemIcon(item.entityType)
                return (
                  <button
                    key={`${item.entityType}:${item.entityId}`}
                    type="button"
                    className={cn(
                      'flex w-full items-center gap-3 rounded-md px-2.5 py-2 text-left transition-colors',
                      'hover:bg-accent focus:bg-accent focus:outline-none',
                    )}
                    onClick={() => {
                      onSelect({
                        entityType: item.entityType,
                        entityId: item.entityId,
                        title: item.title,
                        displayId: item.displayId,
                        status: item.status,
                      })
                      onOpenChange(false)
                    }}
                  >
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                      <Icon className="h-4 w-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{item.title}</span>
                      <span className="block truncate text-xs text-muted-foreground">{item.meta}</span>
                    </span>
                    <span className="shrink-0 text-[11px] uppercase text-muted-foreground">
                      {itemTypeLabel(item.entityType)}
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
        {error && <p className="text-xs text-destructive">{error}</p>}
      </DialogContent>
    </Dialog>
  )
}
