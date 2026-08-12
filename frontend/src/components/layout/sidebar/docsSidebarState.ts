import type { DocsCollection, DocsDocument, DocsSpace } from '@/lib/docsTypes'

const ACTIVE_SPACE_KEY = 'docs_sidebar_active_space'
const EXPANDED_COLLECTIONS_KEY = 'docs_sidebar_expanded_collections'
const LAST_COLLECTION_KEY = 'docs_sidebar_last_collection'

export type DocsSidebarCollectionNode = {
  collection: DocsCollection
  documents: DocsDocument[]
  children: DocsSidebarCollectionNode[]
}

function scopedKey(prefix: string, workspaceId: string, spaceId?: string) {
  return [prefix, workspaceId, spaceId].filter(Boolean).join(':')
}

export function getStoredDocsSpaceId(workspaceId: string): string | null {
  try {
    return localStorage.getItem(scopedKey(ACTIVE_SPACE_KEY, workspaceId))
  } catch {
    return null
  }
}

export function saveStoredDocsSpaceId(workspaceId: string, spaceId: string) {
  try {
    localStorage.setItem(scopedKey(ACTIVE_SPACE_KEY, workspaceId), spaceId)
  } catch {
    // Keep navigation usable when storage is unavailable.
  }
}

export function getStoredDocsCollectionId(workspaceId: string, spaceId: string): string | null {
  try {
    return localStorage.getItem(scopedKey(LAST_COLLECTION_KEY, workspaceId, spaceId))
  } catch {
    return null
  }
}

export function saveStoredDocsCollectionId(workspaceId: string, spaceId: string, collectionId: string) {
  try {
    localStorage.setItem(scopedKey(LAST_COLLECTION_KEY, workspaceId, spaceId), collectionId)
  } catch {
    // Keep navigation usable when storage is unavailable.
  }
}

export function getStoredExpandedCollections(workspaceId: string, spaceId: string): Set<string> {
  try {
    const value = localStorage.getItem(scopedKey(EXPANDED_COLLECTIONS_KEY, workspaceId, spaceId))
    return value ? new Set(JSON.parse(value) as string[]) : new Set()
  } catch {
    return new Set()
  }
}

export function saveStoredExpandedCollections(
  workspaceId: string,
  spaceId: string,
  collectionIds: Set<string>,
) {
  try {
    localStorage.setItem(
      scopedKey(EXPANDED_COLLECTIONS_KEY, workspaceId, spaceId),
      JSON.stringify([...collectionIds]),
    )
  } catch {
    // Keep navigation usable when storage is unavailable.
  }
}

export function resolveDocsSidebarSpaceId({
  spaces,
  routeSpaceId,
  documentSpaceId,
  selectedSpaceId,
  storedSpaceId,
}: {
  spaces: DocsSpace[]
  routeSpaceId?: string | null
  documentSpaceId?: string | null
  selectedSpaceId?: string | null
  storedSpaceId?: string | null
}): string {
  const available = new Set(spaces.map((space) => space.id))
  return [routeSpaceId, documentSpaceId, selectedSpaceId, storedSpaceId]
    .find((candidate): candidate is string => !!candidate && available.has(candidate))
    ?? spaces[0]?.id
    ?? ''
}

export function buildDocsSidebarTree(
  collections: DocsCollection[],
  documents: DocsDocument[],
): DocsSidebarCollectionNode[] {
  const documentsByCollection = new Map<string, DocsDocument[]>()
  for (const document of documents) {
    if (!document.collection_id) continue
    const current = documentsByCollection.get(document.collection_id) ?? []
    current.push(document)
    documentsByCollection.set(document.collection_id, current)
  }

  const collectionsByParent = new Map<string | null, DocsCollection[]>()
  for (const collection of collections) {
    const parentId = collection.parent_collection_id ?? null
    const current = collectionsByParent.get(parentId) ?? []
    current.push(collection)
    collectionsByParent.set(parentId, current)
  }

  const sortDocuments = (items: DocsDocument[]) =>
    [...items].sort((a, b) => a.position - b.position || a.title.localeCompare(b.title))

  const build = (parentId: string | null): DocsSidebarCollectionNode[] =>
    [...(collectionsByParent.get(parentId) ?? [])]
      .sort((a, b) => a.position - b.position || a.name.localeCompare(b.name))
      .map((collection) => ({
        collection,
        documents: sortDocuments(documentsByCollection.get(collection.id) ?? []),
        children: build(collection.id),
      }))

  return build(null)
}

export function findCollectionAncestry(
  collections: DocsCollection[],
  collectionId?: string | null,
): string[] {
  if (!collectionId) return []
  const byId = new Map(collections.map((collection) => [collection.id, collection]))
  const ancestry: string[] = []
  let current = byId.get(collectionId)
  while (current) {
    ancestry.unshift(current.id)
    current = current.parent_collection_id ? byId.get(current.parent_collection_id) : undefined
  }
  return ancestry
}
