import { format, parseISO } from 'date-fns'
import { ExternalLink, Link2, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  useDocsLinks,
  useCreateDocsLink,
  useDeleteDocsLink,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useState } from 'react'
import type { LinkedObjectType, LinkContext } from '@/lib/docsTypes'

const OBJECT_TYPE_LABELS: Record<LinkedObjectType, string> = {
  epic: 'Epic',
  story: 'Story',
  project: 'Project',
  objective: 'Objective',
  sprint: 'Sprint',
  support_ticket: 'Support Ticket',
}

const LINK_CONTEXT_LABELS: Record<LinkContext, string> = {
  attached: 'Attached',
  mentioned: 'Mentioned',
  created_from: 'Created from',
  linked_in_content: 'Linked in content',
}

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
  const { data: links, isLoading } = useDocsLinks(wsId, docId)
  const createLink = useCreateDocsLink(wsId)
  const deleteLink = useDeleteDocsLink(wsId)

  const [showAdd, setShowAdd] = useState(false)
  const [objectType, setObjectType] = useState<LinkedObjectType>('story')
  const [objectId, setObjectId] = useState('')
  const [linkContext, setLinkContext] = useState<LinkContext>('attached')

  const handleAdd = async () => {
    if (!objectId.trim()) return
    try {
      await createLink.mutateAsync({
        docId,
        linked_object_type: objectType,
        linked_object_id: objectId.trim(),
        link_context: linkContext,
      })
      toast.success('Link added')
      setObjectId('')
      setShowAdd(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to add link')
    }
  }

  const handleDelete = async (linkId: string) => {
    try {
      await deleteLink.mutateAsync({ linkId, docId })
      toast.success('Link removed')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove link')
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-80 sm:w-96">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Link2 className="h-4 w-4" />
            Linked Items
          </SheetTitle>
        </SheetHeader>

        <div className="mt-4 space-y-3">
          {canEdit && !showAdd && (
            <Button
              variant="outline"
              size="sm"
              className="w-full gap-1.5"
              onClick={() => setShowAdd(true)}
            >
              <Plus className="h-3.5 w-3.5" />
              Add Link
            </Button>
          )}

          {showAdd && (
            <div className="space-y-3 rounded-md border border-border/60 p-3">
              <div className="grid gap-2">
                <Label className="text-xs">Object Type</Label>
                <Select value={objectType} onValueChange={(v) => setObjectType(v as LinkedObjectType)}>
                  <SelectTrigger className="h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.entries(OBJECT_TYPE_LABELS) as [LinkedObjectType, string][]).map(
                      ([val, label]) => (
                        <SelectItem key={val} value={val}>
                          {label}
                        </SelectItem>
                      ),
                    )}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label className="text-xs">Object ID</Label>
                <Input
                  className="h-8 text-xs"
                  placeholder="Paste ID here"
                  value={objectId}
                  onChange={(e) => setObjectId(e.target.value)}
                />
              </div>
              <div className="grid gap-2">
                <Label className="text-xs">Context</Label>
                <Select value={linkContext} onValueChange={(v) => setLinkContext(v as LinkContext)}>
                  <SelectTrigger className="h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.entries(LINK_CONTEXT_LABELS) as [LinkContext, string][]).map(
                      ([val, label]) => (
                        <SelectItem key={val} value={val}>
                          {label}
                        </SelectItem>
                      ),
                    )}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex gap-2">
                <Button size="sm" className="flex-1 h-7 text-xs" onClick={handleAdd} disabled={createLink.isPending}>
                  Add
                </Button>
                <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setShowAdd(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          )}

          {isLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2].map((i) => (
                <div key={i} className="h-12 animate-pulse rounded-md bg-muted/60" />
              ))}
            </div>
          ) : !links || links.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No linked items. Link documents to epics, stories, and more.
            </p>
          ) : (
            <div className="space-y-1">
              {links.map((link) => (
                <div
                  key={link.id}
                  className="group flex items-center gap-2.5 rounded-md border border-border/40 px-3 py-2 transition-colors hover:bg-muted/30"
                >
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium">
                      {OBJECT_TYPE_LABELS[link.linked_object_type] ?? link.linked_object_type}
                    </p>
                    <p className="text-[10px] text-muted-foreground">
                      {LINK_CONTEXT_LABELS[link.link_context] ?? link.link_context} ·{' '}
                      {format(parseISO(link.created_at), 'MMM d')}
                    </p>
                  </div>
                  {canEdit && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                      onClick={() => handleDelete(link.id)}
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
