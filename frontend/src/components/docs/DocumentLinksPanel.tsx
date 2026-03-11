import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { FileText, Link2, Plus, Search, Trash2, X } from 'lucide-react'
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
import { Input } from '@/components/ui/input'
import type { DocsLink } from '@/lib/docsTypes'
import type { SearchResult } from '@/lib/services/searchService'

interface DocumentLinksPanelProps {
  wsId: string
  docId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  canEdit: boolean
}

export function DocumentLinksPanel({
  wsId,
  docId,
  open,
  onOpenChange,
  canEdit,
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

  // Already linked story IDs to filter from search results
  const linkedStoryIds = new Set(
    (links ?? []).filter((l) => l.linked_object_type === 'story').map((l) => l.linked_object_id),
  )

  const filteredStories = (searchResults?.stories ?? []).filter(
    (s) => !linkedStoryIds.has(s.id),
  )

  const handleLinkStory = useCallback(
    async (story: SearchResult) => {
      try {
        await createLink.mutateAsync({
          docId,
          linked_object_type: 'story',
          linked_object_id: story.id,
          link_context: 'attached',
        })
        toast.success(`Linked story #${story.display_id}`)
        setQuery('')
        setShowSearch(false)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to link story')
      }
    },
    [createLink, docId],
  )

  const handleUnlink = useCallback(
    async (link: DocsLink) => {
      try {
        await deleteLink.mutateAsync({ linkId: link.id, docId })
        toast.success('Story unlinked')
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to unlink')
      }
    },
    [deleteLink, docId],
  )

  const navigateToStory = useCallback(
    (displayId?: number) => {
      if (!displayId) return
      navigate({
        to: '/w/$slug/pm/stories',
        params: { slug: wsSlug },
      })
    },
    [navigate, wsSlug],
  )

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-80 sm:w-96">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Link2 className="h-4 w-4" />
            Linked Stories
          </SheetTitle>
        </SheetHeader>

        <div className="mt-4 space-y-3">
          {/* Add button / Search input */}
          {canEdit && !showSearch && (
            <Button
              variant="outline"
              size="sm"
              className="w-full gap-1.5"
              onClick={() => setShowSearch(true)}
            >
              <Plus className="h-3.5 w-3.5" />
              Link a Story
            </Button>
          )}

          {showSearch && (
            <div className="space-y-2">
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  ref={inputRef}
                  className="h-8 pl-8 pr-8 text-xs"
                  placeholder="Search stories by name..."
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
                <button
                  type="button"
                  onClick={() => { setShowSearch(false); setQuery('') }}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>

              {/* Search results */}
              {query.length >= 2 && (
                <div className="max-h-48 overflow-y-auto rounded-md border border-border/60">
                  {filteredStories.length === 0 ? (
                    <p className="px-3 py-4 text-center text-xs text-muted-foreground">
                      No matching stories found
                    </p>
                  ) : (
                    filteredStories.map((story) => (
                      <button
                        key={story.id}
                        type="button"
                        onClick={() => handleLinkStory(story)}
                        disabled={createLink.isPending}
                        className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-muted/40 disabled:opacity-50"
                      >
                        <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                        <span className="shrink-0 font-medium text-muted-foreground">
                          #{story.display_id}
                        </span>
                        <span className="min-w-0 truncate">{story.name}</span>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
          )}

          {/* Linked stories list */}
          {isLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2].map((i) => (
                <div key={i} className="h-10 animate-pulse rounded-md bg-muted/60" />
              ))}
            </div>
          ) : !links || links.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No linked stories yet.
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
                    onClick={() => navigateToStory(link.linked_object_display_id)}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    disabled={!link.linked_object_display_id}
                  >
                    <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
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
                      <span className="text-xs text-muted-foreground">Story (deleted)</span>
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
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
