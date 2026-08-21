import { useMemo, useState } from 'react'
import { Check, Search, Tag } from 'lucide-react'
import {
  useAddConversationTag,
  useRemoveConversationTag,
  useSupportTags,
  type SupportTag,
} from '@helpin-ai/support-core'
import { getSupportTagPillStyle } from '@/components/support/conversationRowVisual'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { haptic } from '@mobile/lib/haptics'
import { toast } from 'sonner'

interface ConversationTagEditorProps {
  workspaceId: string
  conversationId: string
  selectedTags: SupportTag[]
}

export function ConversationTagEditor({ workspaceId, conversationId, selectedTags }: ConversationTagEditorProps) {
  const [search, setSearch] = useState('')
  const tagsQuery = useSupportTags(workspaceId)
  const addTag = useAddConversationTag(workspaceId)
  const removeTag = useRemoveConversationTag(workspaceId)
  const availableTags = tagsQuery.data ?? []
  const selectedIds = useMemo(() => new Set(selectedTags.map((tag) => tag.id)), [selectedTags])
  const filteredTags = useMemo(() => {
    const query = search.trim().toLowerCase()
    return availableTags.filter((tag) => !query || tag.name.toLowerCase().includes(query))
  }, [availableTags, search])
  const busy = addTag.isPending || removeTag.isPending

  function toggleTag(tag: SupportTag) {
    if (busy) return
    const selected = selectedIds.has(tag.id)
    const mutation = selected ? removeTag : addTag
    haptic('selection')
    mutation.mutate(
      { conversationId, tagId: tag.id },
      {
        onSuccess: () => haptic('notificationSuccess'),
        onError: () => {
          haptic('notificationError')
          toast.error(selected ? 'Could not remove tag' : 'Could not add tag')
        },
      },
    )
  }

  return (
    <div className="flex flex-col border-t border-border/60 py-2">
      {selectedTags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 px-4 pb-2">
          {selectedTags.map((tag) => (
          <span
            key={tag.id}
            style={getSupportTagPillStyle(tag.color)}
            className="rounded-full border px-2 py-0.5 text-caption font-medium"
          >
            {tag.name}
          </span>
          ))}
        </div>
      )}

      {tagsQuery.isPending ? (
        <div className="flex justify-center py-4"><Spinner size={16} /></div>
      ) : tagsQuery.isError ? (
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-footnote text-muted-foreground">Couldn’t load tags</span>
          <Pressable onPress={() => void tagsQuery.refetch()} className="h-auto min-h-0 w-auto min-w-0 text-footnote font-medium text-primary">
            Retry
          </Pressable>
        </div>
      ) : availableTags.length === 0 ? (
        <div className="flex flex-col items-center px-4 py-5 text-center">
          <span className="flex h-9 w-9 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <Tag className="h-4 w-4" />
          </span>
          <span className="mt-2 text-footnote font-medium text-foreground">No tags available</span>
          <span className="mt-0.5 text-caption text-muted-foreground">Create a workspace tag first, then add it here.</span>
        </div>
      ) : (
        <>
          <label className="mx-4 mb-1.5 flex items-center gap-2 rounded-xl border border-input bg-background px-3">
            <Search className="h-4 w-4 text-muted-foreground" />
            <span className="sr-only">Search tags</span>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search tags"
              className="h-10 min-w-0 flex-1 bg-transparent text-body outline-none placeholder:text-muted-foreground"
            />
          </label>

          {filteredTags.length === 0 ? (
            <div className="px-4 py-3 text-center text-footnote text-muted-foreground">
              No tags match “{search.trim()}”
            </div>
          ) : (
            <div className="max-h-52 overflow-y-auto">
              {filteredTags.map((tag) => {
                const selected = selectedIds.has(tag.id)
                return (
                  <Pressable
                    key={tag.id}
                    aria-label={`${selected ? 'Remove' : 'Add'} tag ${tag.name}`}
                    aria-pressed={selected}
                    disabled={busy}
                    onPress={() => toggleTag(tag)}
                    className="flex h-auto min-h-0 w-full items-center gap-3 px-4 py-2.5 text-left disabled:opacity-50"
                  >
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted">
                      <Tag className="h-4 w-4" style={{ color: tag.color ?? undefined }} />
                    </span>
                    <span className="min-w-0 flex-1 truncate text-body">{tag.name}</span>
                    {selected && <Check className="h-4 w-4 text-primary" />}
                  </Pressable>
                )
              })}
            </div>
          )}
        </>
      )}
    </div>
  )
}
