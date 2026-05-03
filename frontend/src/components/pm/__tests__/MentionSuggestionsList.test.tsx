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
              label: 'Alice Smith',
              secondaryText: 'alice@example.com',
              handle: 'alice.smith',
              type: 'member',
              avatarUrl: 'https://example.com/alice.png',
            },
            {
              id: 'team-1',
              label: 'Engineering',
              secondaryText: '@engineering',
              handle: 'engineering',
              type: 'team',
            },
          ]}
          selectedIndex={0}
          onSelect={() => {}}
        />,
      )
    })

    const rows = Array.from(container.querySelectorAll('[role="option"]')) as HTMLElement[]
    expect(rows).toHaveLength(2)

    const [memberRow, teamRow] = rows

    expect(memberRow.textContent).toContain('Alice Smith')
    expect(memberRow.textContent).toContain('@alice.smith')

    expect(teamRow.textContent).toContain('Engineering')
    expect(teamRow.textContent).toContain('@engineering')
  })
})
