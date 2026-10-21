// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { CommentBody } from '../CommentBody'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
afterEach(() => {
  act(() => root?.unmount())
  document.body.innerHTML = ''
})

function render(body: string) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root.render(<CommentBody body={body} variant="pm" teams={[
    { id: 'team-1', name: 'Engineering', handle: 'engineering' },
  ]} />))
  return container
}

describe('CommentBody', () => {
  it('formats Markdown comments from agents and integrations', () => {
    const container = render('## Update\n\n**Ready** and *reviewed*\n\n- First\n- Second\n\n> Review this\n\n[Details](https://example.com)')
    expect(container.querySelector('h2')?.textContent).toBe('Update')
    expect(container.querySelector('strong')?.textContent).toBe('Ready')
    expect(container.querySelector('em')?.textContent).toBe('reviewed')
    expect(container.querySelectorAll('li')).toHaveLength(2)
    expect(container.querySelector('blockquote')?.textContent).toContain('Review this')
    expect(container.querySelector('a')?.getAttribute('href')).toBe('https://example.com')
  })

  it('renders tables, checklists, and literal code', () => {
    const container = render('| Name | Status |\n| --- | --- |\n| Task | Done |\n\n- [x] Done\n- [ ] Pending\n\n`@engineering`\n\n```js\nconst team = "@engineering"\n```')
    expect(container.querySelector('td')?.textContent).toBe('Task')
    const checks = container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')
    expect(Array.from(checks).map(input => [input.checked, input.disabled])).toEqual([[true, true], [false, true]])
    expect(container.querySelector('pre code')?.textContent).toContain('const team = "@engineering"')
    expect(container.querySelector('code [data-mention-type]')).toBeNull()
  })

  it('preserves mentions in formatted text without nesting links', () => {
    const container = render('**@engineering** see https://example.com and [https://example.org](https://example.org)')
    expect(container.querySelector('strong [data-mention-type="team"]')?.textContent).toBe('@Engineering')
    expect(container.querySelectorAll('a')).toHaveLength(2)
    expect(container.querySelector('a a')).toBeNull()
  })

  it('keeps plain comment line breaks and email addresses', () => {
    const container = render('Hello team\nNext line\nContact person@example.com')
    expect(container.textContent).toContain('Hello team\nNext line')
    expect(container.querySelector('[data-mention-type]')).toBeNull()
  })

  it('preserves rich-editor HTML and inline images', () => {
    const container = render('<p><strong>Ready</strong> @engineering</p><p><img src="https://example.com/image.png" alt="Attachment"></p>')
    expect(container.querySelector('strong')?.textContent).toBe('Ready')
    expect(container.querySelector('[data-mention-type="team"]')).toBeTruthy()
    expect(container.querySelector('img')?.getAttribute('alt')).toBe('Attachment')
  })

  it('recognizes rich-editor HTML starting with a list or heading', () => {
    const container = render('  <h2>Update</h2><ul><li>Ready</li></ul>')
    expect(container.querySelector('h2')?.textContent).toBe('Update')
    expect(container.querySelector('li')?.textContent).toBe('Ready')
  })

  it('does not execute embedded HTML or unsafe Markdown links', () => {
    const container = render('Update\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert%281%29)')
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('a[href^="javascript:"]')).toBeNull()
  })

  it('sanitizes rich-editor HTML while preserving formatting', () => {
    const container = render('<h2 onclick="alert(1)">Update</h2><script>alert(1)</script><iframe src="https://example.com"></iframe><p><a href="javascript:alert(1)">Link</a></p>')
    expect(container.querySelector('h2')?.textContent).toBe('Update')
    expect(container.querySelector('script, iframe, [onclick], a[href^="javascript:"]')).toBeNull()
  })
})
