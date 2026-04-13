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
})
