import type { ReactNode } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { useInboxScopes, useSupportTags } from '@helpin-ai/support-core'
import { SearchFilterSheet } from '../search-filter-sheet'
import { EMPTY_CONVERSATION_SEARCH_FILTERS } from '../search-filters'

vi.mock('@helpin-ai/support-core', () => ({ useInboxScopes: vi.fn(), useSupportTags: vi.fn() }))
vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('@mobile/ui/sheet', () => ({
  Sheet: ({ open, children }: { open: boolean; children: ReactNode }) => open ? <div>{children}</div> : null,
}))

beforeEach(() => {
  vi.mocked(useInboxScopes).mockReturnValue({
    data: { shared_inbox: { id: 'shared', name: 'Main inbox' }, mailboxes: [{ id: 'billing', name: 'Billing' }] },
    isPending: false,
  } as never)
  vi.mocked(useSupportTags).mockReturnValue({ data: [{ id: 'vip', name: 'VIP' }], isPending: false } as never)
})

test('applies advanced support search filters', () => {
  const onApply = vi.fn()
  render(
    <SearchFilterSheet
      open
      onOpenChange={vi.fn()}
      workspaceId="ws-1"
      filters={EMPTY_CONVERSATION_SEARCH_FILTERS}
      onApply={onApply}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Resolved' }))
  fireEvent.click(screen.getByRole('button', { name: 'Urgent' }))
  fireEvent.click(screen.getByRole('button', { name: 'Assigned to me' }))
  fireEvent.click(screen.getByRole('button', { name: 'Billing' }))
  fireEvent.click(screen.getByRole('button', { name: 'VIP' }))
  fireEvent.click(screen.getByRole('button', { name: /Oldest/ }))
  fireEvent.change(screen.getByLabelText('Customer email'), { target: { value: 'ada@example.com' } })
  fireEvent.click(screen.getByRole('button', { name: 'Apply filters' }))

  expect(onApply).toHaveBeenCalledWith(expect.objectContaining({
    sort: 'oldest',
    statuses: ['resolved'],
    priorities: ['urgent'],
    assignedTo: ['me'],
    mailboxIds: ['billing'],
    tagIds: ['vip'],
    customerEmail: 'ada@example.com',
  }))
})
