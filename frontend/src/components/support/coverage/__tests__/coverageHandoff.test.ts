import { describe, expect, it } from 'vitest'
import {
  coverageHandoffKey,
  loadCoverageHandoffContent,
  storeCoverageHandoffContent,
} from '../coverageHandoff'

describe('coverage editor handoff', () => {
  it('stores and loads suggestion content by gap and suggestion id', () => {
    const storage = new Map<string, string>()
    const adapter = {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => storage.set(key, value),
      removeItem: (key: string) => storage.delete(key),
    }
    const content = { type: 'doc', content: [{ type: 'paragraph' }] }

    storeCoverageHandoffContent('gap-1', 'suggestion-1', content, adapter)

    expect(storage.has(coverageHandoffKey('gap-1', 'suggestion-1'))).toBe(true)
    expect(loadCoverageHandoffContent('gap-1', 'suggestion-1', adapter)).toEqual(content)
    expect(storage.has(coverageHandoffKey('gap-1', 'suggestion-1'))).toBe(false)
  })
})
