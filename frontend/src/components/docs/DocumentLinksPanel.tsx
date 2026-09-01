import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { File01Icon, Link01Icon, PlusSignIcon, Delete01Icon, Cancel01Icon } from '@/lib/icons'
import { toast } from 'sonner'
import {
  useDocsLinks,
  useCreateDocsLink,
  useDeleteDocsLink,
  useSearch,
} from '@/hooks/queries'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { QuietSearchInput } from '@/components/design-system/quiet'
import type { DocsLink } from '@/lib/docsTypes'
import type { SearchResult } from '@/lib/services/searchService'

interface DocumentLinksPanelProps {
  wsId: string
  docId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  canEdit: boolean
  /** When true, render only the body inline (no Sheet wrapper) — used inside the docs swap rail */
  embedded?: boolean
}

export function DocumentLinksPanel({
  wsId,
  docId,
  open,
  onOpenChange,
  canEdit,
  embedded,
}: DocumentLinksPanelProps) {
  const navigate = useNavigate()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsSlug = workspace?.slug ?? ''

  const { data: links, isLoading } = useDocsLinks(wsId, docId)
  const createLink = useCreateDocsLink(wsId)
  const deleteLink = useDeleteDocsLink(wsId)

  const [showSearch, setShowSearch] = useState(false)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const { data: searchResults } = useSearch(wsId, query)

  // Focus input when search opens
  useEffect(() => {
    if (showSearch) {
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [showSearch])

  // Already linked task IDs to filter from search results
  const linkedTaskIds = new Set(
    (links ?? []).filter((l) => l.linked_object_type === 'task').map((l) => l.linked_object_id),
  )

  const filteredTasks = (searchResults?.tasks ?? searchResults?.tasks ?? []).filter((task) => !linkedTaskIds.has(task.id))

  const handleLinkTask = useCallback(
    async (task: SearchResult) => {
      try {
        await createLink.mutateAsync({
          docId,
          linked_object_type: 'task',
          linked_object_id: task.id,
          link_context: 'attached',
        })
        toast.success(`Linked task #${task.display_id}`)
        setQuery('')
        setShowSearch(false)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to link task')
      }
    },
    [createLink, docId],
  )

  const handleUnlink = useCallback(
    async (link: DocsLink) => {
      try {
        await deleteLink.mutateAsync({ linkId: link.id, docId })
        toast.success('Task unlinked')
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to unlink')
      }
    },
    [deleteLink, docId],
  )

  const navigateToTask = useCallback(
    (displayId?: number) => {
      if (!displayId) return
      navigate({
        to: '/w/$slug/pm/tasks',
        params: { slug: wsSlug },
        search: { task: String(displayId) },
      })
    },
    [navigate, wsSlug],
  )

  if (!open) return null
  if (embedded) {
    return (
      <div className="p-4 space-y-3">
          {/* Add button / Search input */}
          {canEdit && !showSearch && (
            <Button
              variant="outline"
              size="sm"
              className="w-full gap-1.5"
              onClick={() => setShowSearch(true)}
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              Link a Task
            </Button>
          )}

          {showSearch && (
            <div className="space-y-2">
              <QuietSearchInput
                ref={inputRef}
                placeholder="Search tasks by name..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                trailing={(
                  <button
                    type="button"
                    aria-label="Close search"
                    onClick={() => { setShowSearch(false); setQuery('') }}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <Cancel01Icon className="h-3.5 w-3.5" />
                  </button>
                )}
              />

              {/* Search results */}
              {query.length >= 2 && (
                <div className="max-h-48 overflow-y-auto rounded-md border border-border/60">
                  {filteredTasks.length === 0 ? (
                    <p className="px-3 py-4 text-center text-xs text-muted-foreground">
                      No matching tasks found
                    </p>
                  ) : (
                    filteredTasks.map((task) => (
                      <button
                        key={task.id}
                        type="button"
                        onClick={() => handleLinkTask(task)}
                        disabled={createLink.isPending}
                        className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-muted/40 disabled:opacity-50"
                      >
                        <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                        <span className="shrink-0 font-medium text-muted-foreground">
                          #{task.display_id}
                        </span>
                        <span className="min-w-0 truncate">{task.name}</span>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
          )}

          {/* Linked tasks list */}
          {isLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2].map((i) => (
                <div key={i} className="h-10 animate-pulse rounded-md bg-muted/60" />
              ))}
            </div>
          ) : !links || links.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No linked tasks yet.
            </p>
          ) : (
            <div className="space-y-1">
              {links.map((link) => (
                <div
                  key={link.id}
                  className="group flex items-center gap-2 rounded-md border border-border/40 px-3 py-2 transition-colors hover:bg-muted/30"
                >
                  <button
                    type="button"
                    onClick={() => navigateToTask(link.linked_object_display_id)}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    disabled={!link.linked_object_display_id}
                  >
                    <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    {link.linked_object_display_id ? (
                      <>
                        <span className="shrink-0 text-xs font-medium text-muted-foreground">
                          #{link.linked_object_display_id}
                        </span>
                        <span className="min-w-0 truncate text-xs">
                          {link.linked_object_name || 'Untitled'}
                        </span>
                      </>
                    ) : (
                      <span className="text-xs text-muted-foreground">Task (deleted)</span>
                    )}
                  </button>
                  {canEdit && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                      onClick={() => handleUnlink(link)}
                      disabled={deleteLink.isPending}
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
      </div>
    )
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-80 sm:w-96">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Link01Icon className="h-4 w-4" />
            Linked Tasks
          </SheetTitle>
        </SheetHeader>
        <div className="mt-4 space-y-3" />
      </SheetContent>
    </Sheet>
  )
}
