// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { describe, expect, it } from 'vitest'

import { MentionText } from '../MentionText'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('MentionText', () => {
  it('renders team mentions with distinct styling while keeping person mentions and unresolved handles safe', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MentionText
          text="Ping @alice, @engineering, and @ghost."
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

    const personMention = container.querySelector('[data-mention-type="person"]') as HTMLElement | null
    const teamMention = container.querySelector('[data-mention-type="team"]') as HTMLElement | null
    const unresolvedMention = container.querySelector('[data-mention-type="unresolved"]') as HTMLElement | null

    expect(personMention).toBeTruthy()
    expect(teamMention).toBeTruthy()
    expect(unresolvedMention).toBeTruthy()

    expect(personMention?.textContent).toContain('@alice')
    expect(personMention?.closest('[data-mention-type="person"]')).toBeTruthy()
    expect(personMention?.querySelector('span')?.className).toContain('text-blue-600')

    expect(teamMention?.textContent).toContain('@engineering')
    expect(teamMention?.closest('[data-mention-type="team"]')).toBeTruthy()
    expect(teamMention?.querySelector('span')?.className).toContain('emerald')

    expect(unresolvedMention?.textContent).toContain('@ghost')
    expect(unresolvedMention?.closest('[data-mention-type="unresolved"]')).toBeTruthy()
    expect(unresolvedMention?.querySelector('span')?.className).toContain('text-blue-600')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
