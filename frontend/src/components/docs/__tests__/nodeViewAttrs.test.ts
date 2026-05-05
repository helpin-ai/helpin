import { describe, expect, it } from 'vitest'

import { pickBlockNodeViewAttrs } from '../nodeViewAttrs'

describe('pickBlockNodeViewAttrs', () => {
  it('forwards addressable block attributes to React node view wrappers', () => {
    expect(
      pickBlockNodeViewAttrs({
        'data-block-id': 'block-1',
        'data-docs-stale': 'true',
        'data-docs-stale-state': 'stale',
        'data-docs-stale-reason': 'Needs review',
        'data-docs-stale-source': 'coverage',
        'data-docs-stale-gap-id': 'gap-1',
        'data-docs-stale-marked-at': '2026-05-05T00:00:00Z',
        href: 'https://example.com',
        class: 'internal-render-class',
      }),
    ).toEqual({
      'data-block-id': 'block-1',
      'data-docs-stale': 'true',
      'data-docs-stale-state': 'stale',
      'data-docs-stale-reason': 'Needs review',
      'data-docs-stale-source': 'coverage',
      'data-docs-stale-gap-id': 'gap-1',
      'data-docs-stale-marked-at': '2026-05-05T00:00:00Z',
    })
  })
})
