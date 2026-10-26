// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PortalRatingCard } from '../PortalRatingCard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function setValue(element: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set
  setter?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

describe('PortalRatingCard', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  it('submits a rating with a comment', async () => {
    const onSubmit = vi.fn().mockResolvedValue(true)
    act(() => root.render(<PortalRatingCard saving={false} onSubmit={onSubmit} />))
    expect(container.querySelector('form')).toBeNull()
    act(() => (container.querySelector('[aria-label="Satisfied"]') as HTMLButtonElement).click())
    act(() => setValue(container.querySelector('textarea')!, 'Quick fix'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    expect(onSubmit).toHaveBeenCalledWith(4, 'Quick fix')
  })

  it('shows the saved rating and lets the customer change it', () => {
    act(() => root.render(<PortalRatingCard saving={false} current={{ rating: 2, comment: 'Slow' }} onSubmit={vi.fn()} />))
    expect(container.textContent).toContain('You rated this request 2/5 (unsatisfied)')
    const change = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Change rating')
    act(() => change?.click())
    expect(container.querySelector('[aria-label="Unsatisfied"]')?.getAttribute('aria-checked')).toBe('true')
    expect(container.querySelector('textarea')?.value).toBe('Slow')
  })

  it('shows errors from the server', () => {
    act(() => root.render(<PortalRatingCard saving={false} error="This request can no longer be rated." onSubmit={vi.fn()} />))
    act(() => (container.querySelector('[aria-label="Neutral"]') as HTMLButtonElement).click())
    expect(container.querySelector('[role="alert"]')?.textContent).toBe('This request can no longer be rated.')
  })
})
