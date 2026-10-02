import { describe, expect, it, vi } from 'vitest'
import type { DocsDocument } from '@/lib/docsTypes'
import { eligibleDocuments, runDocumentBatch } from '../docsBulkActions'
const doc = (id: string, status: DocsDocument['status'], is_locked = false) => ({ id, status, is_locked }) as DocsDocument

describe('document bulk actions', () => {
  const docs = [doc('draft', 'draft'), doc('published', 'published'), doc('archived', 'archived'), doc('locked', 'draft', true)]
  it('only includes eligible unlocked documents', () => {
    expect(eligibleDocuments(docs, 'archive').map(d => d.id)).toEqual(['draft', 'published'])
    expect(eligibleDocuments(docs, 'restore').map(d => d.id)).toEqual(['archived'])
    expect(eligibleDocuments(docs, 'publish').map(d => d.id)).toEqual(['draft'])
    expect(eligibleDocuments(docs, 'delete').map(d => d.id)).toEqual(['draft', 'published', 'archived'])
    expect(eligibleDocuments(docs, 'move').map(d => d.id)).toEqual(['draft', 'published', 'archived'])
  })
  it('continues after failure and returns successful IDs and useful failures', async () => {
    const execute = vi.fn(async (d: DocsDocument) => { if (d.id === 'published') throw new Error('No access') })
    const result = await runDocumentBatch(docs.slice(0, 3), execute)
    expect(execute).toHaveBeenCalledTimes(3)
    expect(result.succeeded).toEqual(['draft', 'archived'])
    expect(result.failed).toEqual([{ id: 'published', message: 'No access' }])
  })
  it('bounds concurrent requests', async () => {
    let active = 0
    let peak = 0
    await runDocumentBatch(Array.from({ length: 15 }, (_, i) => doc(String(i), 'draft')), async () => {
      peak = Math.max(peak, ++active)
      await new Promise(resolve => setTimeout(resolve, 1))
      active--
    })
    expect(peak).toBeLessThanOrEqual(3)
    expect(active).toBe(0)
  })
})
