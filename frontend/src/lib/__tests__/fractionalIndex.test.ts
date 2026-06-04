import { describe, it, expect } from 'vitest'
import { between } from '../fractionalIndex'
import { readFileSync } from 'fs'
import { resolve } from 'path'

interface TestVector {
  lower: string
  upper: string
  expected?: string
  error?: boolean
}

// Load the shared Go↔TS test vectors — the single source of truth
// for algorithm parity. If either implementation drifts, CI blocks.
const vectorsPath = resolve(__dirname, '../../../../server/internal/ordering/testdata/vectors.json')
const vectors: TestVector[] = JSON.parse(readFileSync(vectorsPath, 'utf-8'))

describe('fractionalIndex — shared vectors', () => {
  vectors.forEach((v, i) => {
    const label = `#${i}: between(${JSON.stringify(v.lower)}, ${JSON.stringify(v.upper)})`
    it(label, () => {
      if (v.error) {
        expect(() => between(v.lower, v.upper)).toThrow()
      } else {
        expect(between(v.lower, v.upper)).toBe(v.expected)
      }
    })
  })
})

describe('fractionalIndex — sequential append', () => {
  it('100 sequential appends produce strictly ascending keys', () => {
    const keys: string[] = []
    let prev = ''
    for (let i = 0; i < 100; i++) {
      const key = between(prev, '')
      if (prev !== '') {
        expect(key > prev).toBe(true)
      }
      keys.push(key)
      prev = key
    }
    // Verify all keys are strictly ordered
    for (let i = 1; i < keys.length; i++) {
      expect(keys[i] > keys[i - 1]).toBe(true)
    }
  })
})

describe('fractionalIndex — insert between', () => {
  it('inserting between two keys always produces a valid midpoint', () => {
    let a = between('', '')
    let b = between(a, '')

    for (let i = 0; i < 50; i++) {
      const mid = between(a, b)
      expect(mid > a).toBe(true)
      expect(mid < b).toBe(true)
      // Narrow the window
      if (i % 2 === 0) {
        a = mid
      } else {
        b = mid
      }
    }
  })
})
