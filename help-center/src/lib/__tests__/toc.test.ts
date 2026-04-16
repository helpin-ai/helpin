import { describe, expect, it } from 'vitest'
import { extractTocFromHtml, slugifyHeading, uniqueSlug } from '@/lib/toc'

describe('slugifyHeading', () => {
  it('lowercases and kebab-cases plain text', () => {
    expect(slugifyHeading('Getting Started')).toBe('getting-started')
  })

  it('strips punctuation', () => {
    expect(slugifyHeading("Why this method?")).toBe('why-this-method')
  })
})

describe('uniqueSlug', () => {
  it('returns the base slug for the first occurrence', () => {
    const seen = new Map<string, number>()
    expect(uniqueSlug('why-this-method', seen)).toBe('why-this-method')
  })

  it('suffixes duplicates with -2, -3, …', () => {
    const seen = new Map<string, number>()
    expect(uniqueSlug('why-this-method', seen)).toBe('why-this-method')
    expect(uniqueSlug('why-this-method', seen)).toBe('why-this-method-2')
    expect(uniqueSlug('why-this-method', seen)).toBe('why-this-method-3')
    expect(uniqueSlug('why-this-method', seen)).toBe('why-this-method-4')
  })

  it('keeps unrelated slugs independent', () => {
    const seen = new Map<string, number>()
    expect(uniqueSlug('intro', seen)).toBe('intro')
    expect(uniqueSlug('setup', seen)).toBe('setup')
    expect(uniqueSlug('intro', seen)).toBe('intro-2')
  })
})

describe('extractTocFromHtml', () => {
  it('produces unique ids for repeated heading text', () => {
    // Real-world case: an article with 4 tabbed options each with
    // a "Why this method" sub-heading. Without dedup every TOC
    // entry pointed to #why-this-method and all 4 highlighted
    // simultaneously in the scroll-spy.
    const html = `
      <h2>Option 1</h2>
      <h3>Why this method</h3>
      <h2>Option 2</h2>
      <h3>Why this method</h3>
      <h2>Option 3</h2>
      <h3>Why this method</h3>
      <h2>Option 4</h2>
      <h3>Why this method</h3>
    `
    const ids = extractTocFromHtml(html).map((t) => t.id)
    expect(ids).toEqual([
      'option-1',
      'why-this-method',
      'option-2',
      'why-this-method-2',
      'option-3',
      'why-this-method-3',
      'option-4',
      'why-this-method-4',
    ])
  })

  it('respects explicit ids when present', () => {
    const html = `
      <h2 id="custom-anchor">First</h2>
      <h3>Second</h3>
    `
    const items = extractTocFromHtml(html)
    expect(items.map((t) => t.id)).toEqual(['custom-anchor', 'second'])
  })

  it('dedupes when explicit ids collide too', () => {
    const html = `
      <h2 id="dup">First</h2>
      <h3 id="dup">Second</h3>
    `
    const items = extractTocFromHtml(html)
    expect(items.map((t) => t.id)).toEqual(['dup', 'dup-2'])
  })
})
