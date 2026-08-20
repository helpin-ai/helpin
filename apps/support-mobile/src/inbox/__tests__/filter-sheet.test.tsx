import type { ReactNode } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { useInboxScopes, useSupportTags } from '@helpin-ai/support-core'
import { defaultConversationListFiltersForNav } from '@/lib/supportInboxFilters'
import { InboxFilterSheet } from '../filter-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useInboxScopes: vi.fn(),
  useSupportTags: vi.fn(),
}))
vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('@mobile/ui/sheet', () => ({
  Sheet: ({ open, children }: { open: boolean; children: ReactNode }) => open ? <div>{children}</div> : null,
}))

const mockScopes = vi.mocked(useInboxScopes)
const mockTags = vi.mocked(useSupportTags)
const baseline = defaultConversationListFiltersForNav('inbox')

beforeEach(() => {
  mockScopes.mockReturnValue({
    data: {
      shared_inbox: { id: 'shared', name: 'Main inbox' },
      mailboxes: [{ id: 'billing', name: 'Billing' }],
    },
    isPending: false,
  } as never)
  mockTags.mockReturnValue({
    data: [{ id: 'urgent', name: 'Urgent' }],
    isPending: false,
  } as never)
})

test('applies state, inbox, tag, and sort changes in the shared web filter shape', () => {
  const onApply = vi.fn()
  render(
    <InboxFilterSheet
      open
      onOpenChange={vi.fn()}
      workspaceId="ws-1"
      navFilter="inbox"
      selectedMailboxId="all"
      filters={baseline}
      baseline={baseline}
      onApply={onApply}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Resolved' }))
  fireEvent.click(screen.getByRole('button', { name: 'Billing' }))
  fireEvent.click(screen.getByRole('button', { name: 'Urgent' }))
  fireEvent.click(screen.getByRole('button', { name: 'Oldest first' }))
  fireEvent.click(screen.getByRole('button', { name: 'Show conversations' }))

  expect(onApply).toHaveBeenCalledWith({
    ...baseline,
    states: [...baseline.states, 'resolved'],
    mailboxIds: ['shared', 'billing'],
    tagIds: ['urgent'],
    sort: 'oldest',
  })
})

test('reset returns to the selected view baseline and clears overrides', () => {
  const onApply = vi.fn()
  render(
    <InboxFilterSheet
      open
      onOpenChange={vi.fn()}
      workspaceId="ws-1"
      filters={{ ...baseline, sort: 'oldest' }}
      navFilter="inbox"
      selectedMailboxId="all"
      baseline={baseline}
      onApply={onApply}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Reset filters' }))
  fireEvent.click(screen.getByRole('button', { name: 'Show conversations' }))

  expect(onApply).toHaveBeenCalledWith(null)
})
test('shows the implicit inbox scope and prevents an empty state selection', () => {
  const onApply = vi.fn()
  render(
    <InboxFilterSheet
      open
      onOpenChange={vi.fn()}
      workspaceId="ws-1"
      navFilter="inbox"
      selectedMailboxId="all"
      filters={baseline}
      baseline={baseline}
      onApply={onApply}
    />,
  )

  expect(screen.getByRole('button', { name: 'Main inbox' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'All inboxes' }))
  fireEvent.click(screen.getByRole('button', { name: 'Open' }))
  fireEvent.click(screen.getByRole('button', { name: 'Waiting' }))
  fireEvent.click(screen.getByRole('button', { name: 'Show conversations' }))

  expect(onApply).toHaveBeenCalledWith({
    ...baseline,
    states: ['waiting_on_customer'],
    mailboxIds: ['all'],
  })
})
