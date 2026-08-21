import { useRef } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import { useResetScrollOnChange } from '../use-reset-scroll-on-change'

function ScrollHarness({ resetKey }: { resetKey: string }) {
  const scrollRef = useRef<HTMLDivElement>(null)
  useResetScrollOnChange(scrollRef, resetKey)
  return <div ref={scrollRef} data-testid="conversation-list" />
}

describe('useResetScrollOnChange', () => {
  test('returns a reused conversation list to the top when its query changes', () => {
    const { rerender } = render(<ScrollHarness resetKey="workspace-1:inbox" />)
    const list = screen.getByTestId('conversation-list')
    list.scrollTop = 640

    rerender(<ScrollHarness resetKey="workspace-1:mine" />)

    expect(list.scrollTop).toBe(0)
  })

  test('does not reset for ordinary rerenders of the same view', () => {
    const { rerender } = render(<ScrollHarness resetKey="workspace-1:mine" />)
    const list = screen.getByTestId('conversation-list')
    list.scrollTop = 160

    rerender(<ScrollHarness resetKey="workspace-1:mine" />)

    expect(list.scrollTop).toBe(160)
  })
})
