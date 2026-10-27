// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PortalArticleSuggestions } from '../PortalArticleSuggestions'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

describe('PortalArticleSuggestions', () => {
  it('lists articles that open in a new tab and reports which one was opened', async () => {
    const onOpen = vi.fn()
    const articles = [
      { id: 'a', title: 'Export reports', space_name: 'Guides', snippet: 'Use Reports → Export.', url: 'https://help.example/export' },
      { id: 'b', title: 'Billing', url: 'https://help.example/billing' },
    ]
    await act(async () => root.render(<PortalArticleSuggestions articles={articles} onOpen={onOpen} />))
    const links = container.querySelectorAll<HTMLAnchorElement>('a')
    expect(links).toHaveLength(2)
    expect(links[0].target).toBe('_blank')
    expect(links[0].rel).toContain('noopener')
    expect(links[0].textContent).toContain('Guides')
    expect(container.querySelector('[aria-live="polite"]')?.textContent).toBe('2 related articles')
    links[0].addEventListener('click', (event) => event.preventDefault())
    await act(async () => links[0].click())
    expect(onOpen).toHaveBeenCalledWith(articles[0])
    const heading = container.querySelector('h2')
    expect(container.querySelector('aside')?.getAttribute('aria-labelledby')).toBe(heading?.id)
  })

  it('renders only an empty live region without articles', async () => {
    await act(async () => root.render(<PortalArticleSuggestions articles={[]} />))
    expect(container.querySelector('h2')).toBeNull()
    expect(container.querySelector('[aria-live="polite"]')?.textContent).toBe('')
  })
})
