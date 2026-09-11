// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act, createElement } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { RichTextMentionContent } from '../RichTextMentionContent'

vi.mock('@/components/editor/MermaidBlock', () => ({
  MermaidBlock: ({ source }: { source: string }) => createElement('div', {
    'data-testid': 'saved-mermaid-diagram',
    'data-source': source,
  }),
}));

vi.mock('@/components/editor/NwdiagBlock', () => ({
  NwdiagBlock: ({ source }: { source: string }) => createElement('div', {
    'data-testid': 'saved-nwdiag-diagram',
    'data-source': source,
  }),
}));

vi.mock('@/components/editor/SvgBlock', () => ({
  SvgBlock: ({ source }: { source: string }) => createElement('div', {
    'data-testid': 'saved-svg-diagram',
    'data-source': source,
  }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('RichTextMentionContent', () => {
  it('applies the shared compact PM rich-text variant', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html="<p>Shared typography</p>"
          variant="pm"
          className="custom-content-class"
        />,
      )
    })

    const content = container.querySelector('.tiptap')
    expect(content?.classList.contains('pm-rich-text')).toBe(true)
    expect(content?.classList.contains('prose')).toBe(true)
    expect(content?.classList.contains('custom-content-class')).toBe(true)

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it.each([['mermaid', 'graph TD\nA-->B'], ['nwdiag', 'nwdiag { network dmz { web01; } }'], ['svg', '<svg><text>Qatar</text></svg>']])('renders a saved %s code block as a diagram', (language, source) => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<pre><code class="language-' + language + '">' + source.replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</code></pre>'}
        />,
      )
    })

    const diagram = container.querySelector('[data-testid="saved-' + language + '-diagram"]')
    expect(diagram?.getAttribute('data-source')).toBe(source)
    expect(container.querySelector('pre')).toBeNull()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders team and person mentions inside rich text html without losing markup structure', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<p>Hello <strong>@engineering</strong> and <a href="/task/1">@alice</a>.</p>'}
          members={[
            {
              id: 'member-1',
              status: 'active',
              role: 'member',
              email: 'alice@example.com',
              display_name: 'Alice',
            },
          ]}
          teams={[
            {
              id: 'team-1',
              name: 'Engineering',
              handle: 'engineering',
            },
          ]}
        />,
      )
    })

    expect(container.querySelector('strong [data-mention-type="team"]')).toBeTruthy()
    expect(container.querySelector('a [data-mention-type="person"]')).toBeTruthy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('does not convert email domains into mention chips inside rich text content', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html="<p>Email: paul.sonneveld@merchantspring.com.au and ping @alice</p>"
          members={[
            {
              id: 'member-1',
              status: 'active',
              role: 'member',
              email: 'alice@example.com',
              display_name: 'Alice',
            },
          ]}
        />,
      )
    })

    const mentions = container.querySelectorAll('[data-mention-type]')

    expect(mentions).toHaveLength(1)
    expect(mentions[0]?.textContent).toContain('@Alice')
    expect(container.textContent).toContain('paul.sonneveld@merchantspring.com.au')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('shows an image loader until inline task images finish loading', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<p><img src="https://cdn.example.com/task.png" alt="Task image" /></p>'}
        />,
      )
    })

    expect(container.querySelector('[data-loading-image-spinner="true"]')).toBeTruthy()

    const image = container.querySelector('img')
    expect(image).toBeTruthy()

    act(() => {
      image?.dispatchEvent(new Event('load'))
    })

    expect(container.querySelector('[data-loading-image-spinner="true"]')).toBeNull()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders saved description images left aligned by default', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<p><img src="https://cdn.example.com/task.png" alt="Task image" /></p>'}
        />,
      )
    })

    const imageWrapper = container.querySelector('[data-inline-image-align="left"]')
    expect(imageWrapper).toBeTruthy()
    expect(imageWrapper?.querySelector('img')?.getAttribute('src')).toBe('https://cdn.example.com/task.png')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
