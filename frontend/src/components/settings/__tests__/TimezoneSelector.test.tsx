// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TimezoneSelector } from '../TimezoneSelector'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('TimezoneSelector', () => {
  let container: HTMLDivElement
  let root: Root
  const onChange = vi.fn()
  const trigger = () => container.querySelector<HTMLButtonElement>('#timezone')!
  const option = (value: string) => document.querySelector<HTMLElement>(`[role="option"][data-value="${value}"]`)

  beforeEach(() => {
    onChange.mockClear()
    vi.stubGlobal('ResizeObserver', class {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    HTMLElement.prototype.scrollIntoView = vi.fn()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => root.unmount())
    container.remove()
    vi.unstubAllGlobals()
  })

  async function render(value = 'UTC', disabled = false) {
    await act(async () => {
      root.render(<TimezoneSelector id="timezone" value={value} onChange={onChange} disabled={disabled} />)
    })
  }

  async function search(query: string) {
    const input = document.querySelector<HTMLInputElement>('input[placeholder="Search timezones..."]')!
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, query)
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
  }

  it('searches city names with spaces and selects with the keyboard', async () => {
    await render()
    await act(async () => { trigger().click() })
    await search('NEW YORK')
    expect(option('America/New_York')).not.toBeNull()
    expect(option('Asia/Karachi')).toBeNull()
    await act(async () => {
      document.querySelector('[cmdk-root]')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    })
    expect(onChange).toHaveBeenCalledWith('America/New_York')
    expect(trigger().getAttribute('aria-expanded')).toBe('false')
  })

  it('includes UTC and supports offset search and empty results', async () => {
    await render()
    expect(trigger().textContent).toContain('UTC+00:00')
    await act(async () => { trigger().click() })
    expect(option('UTC')).not.toBeNull()
    await search('UTC+5:30')
    expect(option('Asia/Colombo')).not.toBeNull()
    expect(option('America/New_York')).toBeNull()
    await search('not a timezone')
    expect(document.body.textContent).toContain('No timezones found')
  })

  it('preserves saved timezone aliases omitted from the browser list', async () => {
    await render('US/Eastern')
    expect(trigger().textContent).toContain('US/Eastern')
    await act(async () => { trigger().click() })
    expect(option('US/Eastern')?.getAttribute('data-checked')).toBe('true')
    expect(onChange).not.toHaveBeenCalled()
  })

  it('respects read-only workspace permissions', async () => {
    await render('UTC', true)
    expect(trigger().disabled).toBe(true)
    await act(async () => { trigger().click() })
    expect(document.querySelector('[role="option"]')).toBeNull()
    expect(onChange).not.toHaveBeenCalled()
  })
})
