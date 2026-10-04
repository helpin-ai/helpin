import { useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { ArrowDown01Icon } from '@/lib/icons'
import { docsService } from '@/lib/services/docsService'
import { unwrap } from '@/lib/queryUtils'
import type { DocsDocument, MoveDocsDocumentRequest } from '@/lib/docsTypes'
import { MoveDocumentDialog } from './MoveDocumentDialog'
import { eligibleDocuments, runDocumentBatch, type DocsBulkAction } from './docsBulkActions'
import type { DocsSelection } from './useDocsSelection'

const labels: Record<DocsBulkAction, string> = { move: 'Move', archive: 'Archive', restore: 'Restore', publish: 'Publish', delete: 'Delete' }
const completedLabels: Record<DocsBulkAction, string> = { move: 'moved', archive: 'archived', restore: 'restored as drafts', publish: 'published', delete: 'deleted' }

export function DocsSelectionCheckbox({ doc, selection }: { doc: DocsDocument; selection: DocsSelection }) {
  return <Checkbox
    aria-label={`Select ${doc.title || 'Untitled'}`}
    title={doc.is_locked ? 'Unlock this document to select it' : undefined}
    disabled={selection.busy || doc.is_locked}
    checked={selection.ids.includes(doc.id)}
    onCheckedChange={() => selection.toggle(doc.id)}
  />
}

export function DocsBulkToolbar({ wsId, selection }: { wsId: string; selection: DocsSelection }) {
  const queryClient = useQueryClient()
  const running = useRef(false)
  const [dialog, setDialog] = useState<{ action: DocsBulkAction; docs: DocsDocument[] } | null>(null)

  const execute = async (action: DocsBulkAction, docs: DocsDocument[], destination?: MoveDocsDocumentRequest) => {
    if (running.current) return
    running.current = true
    selection.setBusy(true)
    try {
      const result = await runDocumentBatch(docs, async doc => {
        switch (action) {
          case 'move':
            if (!destination) throw new Error('Choose a destination')
            return unwrap(await docsService.moveDocument(wsId, doc.id, destination))
          case 'archive': return unwrap(await docsService.archiveDocument(wsId, doc.id))
          case 'restore': return unwrap(await docsService.unarchiveDocument(wsId, doc.id))
          case 'publish': return unwrap(await docsService.publishDocument(wsId, doc.id))
          case 'delete': return unwrap(await docsService.deleteDocument(wsId, doc.id))
        }
      })
      selection.remove(result.succeeded)
      if (result.failed.length) {
        const detail = result.failed.slice(0, 3).map(failure => {
          const doc = docs.find(doc => doc.id === failure.id)
          return `${doc?.title || 'Untitled'}: ${failure.message}`
        }).join('\n')
        toast.error(`${result.succeeded.length} ${completedLabels[action]}, ${result.failed.length} failed`, { description: detail })
      } else {
        toast.success(`${result.succeeded.length} document${result.succeeded.length === 1 ? '' : 's'} ${completedLabels[action]}`)
      }
      // Refresh lists, individual docs, collection counts, and help-center state once.
      await queryClient.invalidateQueries({ queryKey: ['docs', wsId] })
    } finally {
      running.current = false
      selection.setBusy(false)
    }
  }

  const choose = (action: DocsBulkAction) => {
    const docs = eligibleDocuments(selection.selected, action)
    if (!docs.length || running.current) return
    if (action === 'move' || action === 'publish' || action === 'delete') setDialog({ action, docs })
    else void execute(action, docs)
  }

  return <>
    <div className="flex min-h-9 flex-wrap items-center gap-3 px-3" aria-label="Document selection">
      <Checkbox
        aria-label="Select all visible documents"
        checked={selection.allChecked ? true : selection.ids.length ? 'indeterminate' : false}
        disabled={selection.busy || !selection.hasSelectable}
        onCheckedChange={selection.toggleAll}
      />
      {selection.ids.length ? <>
        <span className="text-sm text-muted-foreground" role="status">{selection.busy ? 'Updating…' : `${selection.ids.length} selected`}</span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" disabled={selection.busy}>Actions<ArrowDown01Icon className="h-4 w-4" /></Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {(['move', 'archive', 'restore', 'publish', 'delete'] as const).map(action => {
              const count = eligibleDocuments(selection.selected, action).length
              return <div key={action}>
                {action === 'delete' && <DropdownMenuSeparator />}
                <DropdownMenuItem disabled={!count} onSelect={() => choose(action)} className={action === 'delete' ? 'text-destructive focus:text-destructive' : undefined}>
                  {labels[action]}{count !== selection.ids.length ? ` (${count})` : ''}
                </DropdownMenuItem>
              </div>
            })}
          </DropdownMenuContent>
        </DropdownMenu>
        <Button variant="ghost" size="sm" disabled={selection.busy} onClick={selection.clear}>Clear</Button>
      </> : <span className="text-xs text-muted-foreground">Select all visible</span>}
    </div>
    {dialog?.action === 'move' && <MoveDocumentDialog
      wsId={wsId}
      open
      onOpenChange={open => { if (!open) setDialog(null) }}
      docTitle="Selected documents"
      documentCount={dialog.docs.length}
      currentSpaceId={dialog.docs[0].space_id}
      currentCollectionId={dialog.docs[0].collection_id}
      pending={selection.busy}
      onMove={destination => execute('move', dialog.docs, destination)}
    />}
    {dialog && (dialog.action === 'delete' || dialog.action === 'publish') && <ConfirmDialog
      open
      onOpenChange={open => { if (!open) setDialog(null) }}
      title={`${labels[dialog.action]} ${dialog.docs.length} document${dialog.docs.length === 1 ? '' : 's'}?`}
      description={dialog.action === 'publish'
        ? 'Documents in help-center spaces will also be published publicly. Other documents will be published within their workspace.'
        : 'The selected documents will be removed from Docs and any published help-center pages.'}
      confirmLabel={labels[dialog.action]}
      variant={dialog.action === 'delete' ? 'destructive' : 'default'}
      onConfirm={() => { void execute(dialog.action, dialog.docs); setDialog(null) }}
    />}
  </>
}
