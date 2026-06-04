import { describe, expect, it } from 'vitest'
import type { DocsCollection } from '@/lib/docsTypes'
import { buildCoverageCollectionOptions } from '../coverageCollectionOptions'

function makeCollection(
  id: string,
  parent: string | null,
  depth: number,
  position: number,
  name = id,
): DocsCollection {
  return {
    id,
    space_id: 'space-1',
    workspace_id: 'ws-1',
    parent_collection_id: parent,
    depth,
    name,
    slug: id,
    position,
    sort_key: '~',
    created_by: 'user-1',
    created_at: `2026-04-28T10:00:${position.toString().padStart(2, '0')}Z`,
    updated_at: `2026-04-28T10:00:${position.toString().padStart(2, '0')}Z`,
  }
}

describe('buildCoverageCollectionOptions', () => {
  it('returns collections in depth-first hierarchy order with display labels', () => {
    const options = buildCoverageCollectionOptions('space-1', [
      makeCollection('root-b', null, 0, 1, 'Billing'),
      makeCollection('grandchild', 'child', 2, 0, 'Webhook retries'),
      makeCollection('root-a', null, 0, 0, 'Integrations'),
      makeCollection('child', 'root-a', 1, 0, 'Webhooks'),
    ])

    expect(options.map((option) => option.id)).toEqual(['root-a', 'child', 'grandchild', 'root-b'])
    expect(options.map((option) => option.label)).toEqual([
      'Integrations',
      '↳ Webhooks',
      '↳ ↳ Webhook retries',
      'Billing',
    ])
  })
})
