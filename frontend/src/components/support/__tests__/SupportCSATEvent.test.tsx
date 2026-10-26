// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { SupportCSATEvent } from '../SupportCSATEvent'
import { parseSupportCSATEvent } from '../supportCSAT'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('parseSupportCSATEvent', () => {
  it('reads the rating, comment and previous rating', () => {
    expect(parseSupportCSATEvent('{"rating":2,"comment":" Slow ","previous_rating":4}')).toEqual({ rating: 2, comment: 'Slow', previousRating: 4 })
  })

  it('rejects malformed metadata', () => {
    expect(parseSupportCSATEvent('')).toBeNull()
    expect(parseSupportCSATEvent('not json')).toBeNull()
    expect(parseSupportCSATEvent('{"rating":7}')).toBeNull()
    expect(parseSupportCSATEvent('{"rating":"4"}')).toBeNull()
  })
})

describe('SupportCSATEvent', () => {
  it('renders the rating, label and comment', () => {
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => root.render(<SupportCSATEvent details={{ rating: 4, comment: 'Great', previousRating: 2 }} customerName="Ada" timestamp="Oct 1" />))
    expect(container.textContent).toContain('Ada changed their rating')
    expect(container.textContent).toContain('Satisfied')
    expect(container.textContent).toContain('(was 2/5)')
    expect(container.textContent).toContain('“Great”')
    expect(container.querySelector('[aria-label="4 out of 5"]')).not.toBeNull()
    act(() => root.unmount())
  })
})
