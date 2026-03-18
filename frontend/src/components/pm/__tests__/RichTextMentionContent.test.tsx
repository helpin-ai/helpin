// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { describe, expect, it } from 'vitest'

import { RichTextMentionContent } from '../RichTextMentionContent'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('RichTextMentionContent', () => {
  it('renders team and person mentions inside rich text html without losing markup structure', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<p>Hello <strong>@engineering</strong> and <a href="/story/1">@alice</a>.</p>'}
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

  it('shows an image loader until inline story images finish loading', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <RichTextMentionContent
          html={'<p><img src="https://cdn.example.com/story.png" alt="Story image" /></p>'}
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
})
