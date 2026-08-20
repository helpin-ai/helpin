import { describe, expect, test } from 'vitest'
import { parseEmailList } from '../new-conversation-helpers'

describe('parseEmailList', () => {
  test('normalizes separators, casing, whitespace, and duplicates', () => {
    expect(parseEmailList(' A@Example.com, b@example.com; a@example.com\nC@example.com ')).toEqual([
      'a@example.com',
      'b@example.com',
      'c@example.com',
    ])
  })

  test('drops empty entries', () => {
    expect(parseEmailList(' , ;  ')).toEqual([])
  })
})
