const BLOCK_NODE_VIEW_ATTRS = [
  'data-block-id',
  'data-docs-stale',
  'data-docs-stale-state',
  'data-docs-stale-reason',
  'data-docs-stale-source',
  'data-docs-stale-gap-id',
  'data-docs-stale-marked-at',
] as const

export function pickBlockNodeViewAttrs(HTMLAttributes: Record<string, unknown>): Record<string, string> {
  const attrs: Record<string, string> = {}
  for (const key of BLOCK_NODE_VIEW_ATTRS) {
    const value = HTMLAttributes[key]
    if (typeof value === 'string' && value !== '') {
      attrs[key] = value
    }
  }
  return attrs
}
