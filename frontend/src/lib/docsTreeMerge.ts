/**
 * Merges collections and docs within a bucket by sort_key for rendering.
 *
 * In the fractional-sort-key model, collections and docs share the same
 * sort_key space per bucket. This helper merges two pre-sorted arrays
 * into a single ordered sequence using two-pointer merge on
 * (sort_key ASC, id ASC) — the canonical ORDER BY contract.
 */

export interface BucketItem {
  id: string
  sort_key: string
  type: 'doc' | 'collection'
}

/**
 * Two-pointer merge of collections and docs by (sort_key, id).
 * Both input arrays MUST already be sorted by the same key.
 */
export function mergeBucketItems(
  collections: BucketItem[],
  docs: BucketItem[],
): BucketItem[] {
  const result: BucketItem[] = []
  let ci = 0
  let di = 0

  while (ci < collections.length && di < docs.length) {
    const coll = collections[ci]
    const doc = docs[di]

    // Compare by sort_key first, then by id as tiebreaker
    const cmp = coll.sort_key < doc.sort_key
      ? -1
      : coll.sort_key > doc.sort_key
        ? 1
        : coll.id < doc.id
          ? -1
          : coll.id > doc.id
            ? 1
            : 0

    if (cmp <= 0) {
      result.push(coll)
      ci++
    } else {
      result.push(doc)
      di++
    }
  }

  // Append remaining items
  while (ci < collections.length) result.push(collections[ci++])
  while (di < docs.length) result.push(docs[di++])

  return result
}
