import { describe, expect, it } from 'vitest'

import { nextCursor, unwrapRequired } from '@/lib/queryUtils'

describe('unwrapRequired', () => {
  it('rejects a successful response with no response body', () => {
    expect(() =>
      unwrapRequired(
        { data: null, error: null, status: 204 },
        'Support message page',
      ),
    ).toThrow('Support message page returned no data (HTTP 204)')
  })

  it('returns a non-null response body', () => {
    const page = { data: [], next_cursor: null }

    expect(
      unwrapRequired({ data: page, error: null, status: 200 }, 'Support message page'),
    ).toBe(page)
  })
})

describe('nextCursor', () => {
  it('returns no cursor for a null page', () => {
    expect(nextCursor(null)).toBeUndefined()
  })

  it('returns a populated cursor', () => {
    expect(nextCursor({ next_cursor: 'older' })).toBe('older')
  })
})
