// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { describe, expect, it } from 'vitest'

import { MentionSuggestionsList } from '../MentionSuggestionsList'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('MentionSuggestionsList', () => {
  it('renders member avatars and team badges with a shared row layout', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MentionSuggestionsList
          items={[
            {
              id: 'user-1',
              name: 'Alice Smith',
              handle: 'alice.smith',
              type: 'member',
              avatarUrl: 'https://example.com/alice.png',
            },
            {
              id: 'team-1',
              name: 'Engineering',
              handle: 'engineering',
              type: 'team',
            },
          ]}
          selectedIndex={0}
          onSelect={() => {}}
        />,
      )
    })

    const memberRow = container.querySelector('[data-mention-suggestion-type="member"]') as HTMLElement | null
    const teamRow = container.querySelector('[data-mention-suggestion-type="team"]') as HTMLElement | null

    expect(memberRow).toBeTruthy()
    expect(teamRow).toBeTruthy()

    expect(memberRow?.textContent).toContain('Alice Smith')
    expect(memberRow?.textContent).toContain('@alice.smith')
    expect(memberRow?.querySelector('[data-mention-member-avatar]')).toBeTruthy()

    expect(teamRow?.textContent).toContain('Engineering')
    expect(teamRow?.textContent).toContain('@engineering')
    expect(teamRow?.querySelector('[data-mention-team-badge]')?.textContent).toBe('E')
  })
})
