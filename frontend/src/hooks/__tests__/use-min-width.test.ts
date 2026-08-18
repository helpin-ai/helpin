import { describe, expect, it } from 'vitest'

import { isAtLeastWidth } from '../use-min-width'

describe('isAtLeastWidth', () => {
  it('matches the xl support sidebar boundary', () => {
    expect(isAtLeastWidth(1279, 1280)).toBe(false)
    expect(isAtLeastWidth(1280, 1280)).toBe(true)
  })
})
