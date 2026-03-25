import type { DocsSpace, DocsCollection, DocsDocument, SpaceType } from '@/lib/docsTypes'

// ─── Tree types ─────────────────────────────────────────────────────────────

export interface DocsBucket {
  collectionId: string | null
  collectionName: string
  documents: DocsDocument[]
}

export interface SpaceNode {
  space: DocsSpace
  collections: DocsCollection[]
  buckets: DocsBucket[]
}

export interface OrderingSection {
  type: SpaceType
  label: string
  spaces: SpaceNode[]
}

// ─── Build tree ─────────────────────────────────────────────────────────────

export function buildOrderingSections(
  spaces: DocsSpace[],
  collections: DocsCollection[],
  documents: DocsDocument[],
): OrderingSection[] {
  const collsBySpace = new Map<string, DocsCollection[]>()
  for (const c of collections) {
    const list = collsBySpace.get(c.space_id) ?? []
    list.push(c)
    collsBySpace.set(c.space_id, list)
  }

  const docsByBucket = new Map<string, DocsDocument[]>()
  for (const d of documents) {
    const key = `${d.space_id}:${d.collection_id ?? '__uncategorized'}`
    const list = docsByBucket.get(key) ?? []
    list.push(d)
    docsByBucket.set(key, list)
  }

  function buildSpaceNode(space: DocsSpace): SpaceNode {
    const spaceColls = (collsBySpace.get(space.id) ?? [])
      .sort((a, b) => a.position - b.position)

    const buckets: DocsBucket[] = []

    // Uncategorized bucket first
    const uncatKey = `${space.id}:__uncategorized`
    const uncatDocs = (docsByBucket.get(uncatKey) ?? []).sort((a, b) => a.position - b.position)
    if (uncatDocs.length > 0) {
      buckets.push({ collectionId: null, collectionName: 'Uncategorized', documents: uncatDocs })
    }

    // Then each collection bucket
    for (const coll of spaceColls) {
      const collKey = `${space.id}:${coll.id}`
      const collDocs = (docsByBucket.get(collKey) ?? []).sort((a, b) => a.position - b.position)
      buckets.push({ collectionId: coll.id, collectionName: coll.name, documents: collDocs })
    }

    return { space, collections: spaceColls, buckets }
  }

  const internal = spaces
    .filter((s) => s.type === 'internal')
    .sort((a, b) => a.position - b.position)
    .map(buildSpaceNode)

  const external = spaces
    .filter((s) => s.type === 'external_capable')
    .sort((a, b) => a.position - b.position)
    .map(buildSpaceNode)

  return [
    { type: 'internal', label: 'Internal', spaces: internal },
    { type: 'external_capable', label: 'External (Help Center)', spaces: external },
  ]
}

// ─── Reorder reducers ───────────────────────────────────────────────────────

export function reorderSpacesInSection(
  sections: OrderingSection[],
  sectionType: SpaceType,
  orderedIds: string[],
): OrderingSection[] {
  return sections.map((section) => {
    if (section.type !== sectionType) return section
    const byId = new Map(section.spaces.map((s) => [s.space.id, s]))
    const reordered = orderedIds.map((id) => byId.get(id)).filter(Boolean) as SpaceNode[]
    return { ...section, spaces: reordered }
  })
}

export function reorderCollectionsInSpace(
  sections: OrderingSection[],
  spaceId: string,
  orderedIds: string[],
): OrderingSection[] {
  return sections.map((section) => ({
    ...section,
    spaces: section.spaces.map((node) => {
      if (node.space.id !== spaceId) return node
      const byId = new Map(node.collections.map((c) => [c.id, c]))
      const reordered = orderedIds.map((id) => byId.get(id)).filter(Boolean) as DocsCollection[]
      // Rebuild buckets in new collection order
      const buckets: DocsBucket[] = []
      const uncatBucket = node.buckets.find((b) => b.collectionId === null)
      if (uncatBucket) buckets.push(uncatBucket)
      for (const coll of reordered) {
        const bucket = node.buckets.find((b) => b.collectionId === coll.id)
        if (bucket) buckets.push(bucket)
      }
      return { ...node, collections: reordered, buckets }
    }),
  }))
}

export function reorderDocumentsInBucket(
  sections: OrderingSection[],
  spaceId: string,
  collectionId: string | null,
  orderedIds: string[],
): OrderingSection[] {
  return sections.map((section) => ({
    ...section,
    spaces: section.spaces.map((node) => {
      if (node.space.id !== spaceId) return node
      return {
        ...node,
        buckets: node.buckets.map((bucket) => {
          if (bucket.collectionId !== collectionId) return bucket
          const byId = new Map(bucket.documents.map((d) => [d.id, d]))
          const reordered = orderedIds.map((id) => byId.get(id)).filter(Boolean) as DocsDocument[]
          return { ...bucket, documents: reordered }
        }),
      }
    }),
  }))
}

// ─── Payload builders ───────────────────────────────────────────────────────

export function buildSpaceReorderPayload(section: OrderingSection) {
  return {
    section: section.type,
    space_ids: section.spaces.map((s) => s.space.id),
  }
}

export function buildCollectionReorderPayload(node: SpaceNode) {
  return {
    collection_ids: node.collections.map((c) => c.id),
  }
}

export function buildDocumentReorderPayload(bucket: DocsBucket) {
  return {
    collection_id: bucket.collectionId ?? undefined,
    document_ids: bucket.documents.map((d) => d.id),
  }
}
