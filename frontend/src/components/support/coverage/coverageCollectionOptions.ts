import { buildCollectionTreeOptions } from '@/components/docs/CollectionTreePicker'
import type { DocsCollection } from '@/lib/docsTypes'

export interface CoverageCollectionOption {
  id: string
  label: string
}

export function buildCoverageCollectionOptions(
  spaceId: string,
  collections: DocsCollection[] = [],
): CoverageCollectionOption[] {
  return buildCollectionTreeOptions(spaceId, collections).map((option) => ({
    id: option.id,
    label: option.depth > 0 ? `${Array(option.depth).fill('↳').join(' ')} ${option.name}` : option.name,
  }))
}
