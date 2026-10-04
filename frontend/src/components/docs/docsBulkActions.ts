import type { DocsDocument } from '@/lib/docsTypes'

export type DocsBulkAction = 'move' | 'archive' | 'restore' | 'publish' | 'delete'

export function eligibleDocuments(docs: DocsDocument[], action: DocsBulkAction) {
  return docs.filter(doc => !doc.is_locked && (
    action === 'archive' ? doc.status !== 'archived' :
    action === 'restore' ? doc.status === 'archived' :
    action === 'publish' ? doc.status === 'draft' : true
  ))
}

// Use the existing permission-checked endpoints, with bounded request concurrency.
export async function runDocumentBatch(docs: DocsDocument[], execute: (doc: DocsDocument) => Promise<unknown>) {
  const succeeded: string[] = []
  const failed: { id: string; message: string }[] = []
  let next = 0
  await Promise.all(Array.from({ length: Math.min(3, docs.length) }, async () => {
    while (next < docs.length) {
      const doc = docs[next++]
      try {
        await execute(doc)
        succeeded.push(doc.id)
      } catch (error) {
        failed.push({ id: doc.id, message: error instanceof Error ? error.message : 'Request failed' })
      }
    }
  }))
  return { succeeded, failed }
}
