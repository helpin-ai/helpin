import { useMemo, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useCreateDocsDocument, useDocsSpaces, useDocsCollections } from '@/hooks/queries'
import type { DocType } from '@/lib/docsTypes'
import { DOC_TYPE_LABELS } from '@/lib/docsTypes'
import { toast } from 'sonner'

interface CreateDocumentDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultSpaceId?: string
  defaultCollectionId?: string
  onCreated?: (docId: string) => void
}

const docTypes: DocType[] = ['wiki', 'sop', 'feature_doc', 'support_article', 'help_center_article']

export function CreateDocumentDialog({
  wsId,
  open,
  onOpenChange,
  defaultSpaceId,
  defaultCollectionId,
  onCreated,
}: CreateDocumentDialogProps) {
  const [title, setTitle] = useState('')
  const [docType, setDocType] = useState<DocType>('wiki')
  const [spaceId, setSpaceId] = useState(defaultSpaceId ?? '')
  const [collectionId, setCollectionId] = useState(defaultCollectionId ?? '')
  const createDocument = useCreateDocsDocument(wsId)
  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, spaceId || '')

  // Set defaults when spaces load
  useMemo(() => {
    if (!spaceId && spaces?.length) setSpaceId(spaces[0].id)
  }, [spaces, spaceId])

  const reset = () => {
    setTitle('')
    setDocType('wiki')
    setSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
    setCollectionId(defaultCollectionId ?? '')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !spaceId) return

    try {
      const doc = await createDocument.mutateAsync({
        title: title.trim(),
        doc_type: docType,
        space_id: spaceId,
        collection_id: collectionId || undefined,
      })
      toast.success('Document created')
      reset()
      onOpenChange(false)
      onCreated?.(doc.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create document')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create Document</DialogTitle>
            <DialogDescription>
              Add a new document to your documentation.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="doc-title">Title</Label>
              <Input
                id="doc-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Getting Started Guide"
                autoFocus
              />
            </div>

            <div className="grid gap-2">
              <Label>Document type</Label>
              <Select value={docType} onValueChange={(v) => setDocType(v as DocType)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {docTypes.map((dt) => (
                    <SelectItem key={dt} value={dt}>
                      {DOC_TYPE_LABELS[dt]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label>Space</Label>
              <Select value={spaceId} onValueChange={(v) => { setSpaceId(v); setCollectionId(''); }}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a space" />
                </SelectTrigger>
                <SelectContent>
                  {(spaces ?? []).map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.icon ? `${s.icon} ` : ''}{s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {(collections ?? []).length > 0 && (
              <div className="grid gap-2">
                <Label>Collection (optional)</Label>
                <Select value={collectionId || '__none__'} onValueChange={(v) => setCollectionId(v === '__none__' ? '' : v)}>
                  <SelectTrigger>
                    <SelectValue placeholder="No collection" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">None</SelectItem>
                    {(collections ?? []).map((c) => (
                      <SelectItem key={c.id} value={c.id}>
                        {c.icon ? `${c.icon} ` : ''}{c.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={!title.trim() || !spaceId || createDocument.isPending}
            >
              {createDocument.isPending ? 'Creating...' : 'Create Document'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
