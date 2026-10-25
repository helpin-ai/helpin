// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportConversation } from '@/lib/pmTypes'
import { ConversationRowOperations } from '../ConversationRowOperations'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function render(conversation: Partial<SupportConversation>) {
  const container = document.createElement('div')
  const root = createRoot(container)
  act(() => root.render(
    <TooltipProvider>
      <ConversationRowOperations conversation={{ priority: 'medium', status: 'open', ...conversation } as SupportConversation} />
    </TooltipProvider>,
  ))
  return { container, unmount: () => act(() => root.unmount()) }
}

describe('ConversationRowOperations', () => {
  it('renders nothing for an ordinary conversation', () => {
    const { container, unmount } = render({})
    expect(container.innerHTML).toBe('')
    unmount()
  })

  it('shows urgent priority, a breached SLA and the snooze time', () => {
    const { container, unmount } = render({
      priority: 'urgent',
      snoozed_until: '2030-01-07T09:00:00Z',
      sla: { policy_name: 'P', breached: true, next_due_at: '2020-01-01T00:00:00Z', metrics: [{ metric: 'first_response', state: 'breached', due_at: '2020-01-01T00:00:00Z' }] },
    })
    expect(container.querySelector('[aria-label="urgent priority"]')).not.toBeNull()
    expect(container.querySelector('[data-sla-tone="breached"]')?.textContent).toContain('FRT')
    expect(container.textContent).toMatch(/\w{3} \d{2}:\d{2}/)
    unmount()
  })

  it('hides SLA markers once a conversation is resolved', () => {
    const { container, unmount } = render({
      status: 'resolved',
      sla: { policy_name: 'P', breached: false, next_due_at: '2030-01-01T00:00:00Z', metrics: [{ metric: 'resolution', state: 'pending', due_at: '2030-01-01T00:00:00Z' }] },
    })
    expect(container.innerHTML).toBe('')
    unmount()
  })
})
