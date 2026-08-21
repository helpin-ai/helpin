import { fireEvent, render, screen } from '@testing-library/react'
import { useSupportTeammatePresence, useUpdateMySupportTeammatePresence } from '@helpin-ai/support-core'
import { SupportAvailabilityControl } from '../support-availability-control'

vi.mock('@helpin-ai/support-core', () => ({
  useSupportTeammatePresence: vi.fn(),
  useUpdateMySupportTeammatePresence: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const mutate = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useSupportTeammatePresence).mockReturnValue({ data: [
    { user_id: 'user-1', status: 'online', source: 'auto' },
  ], isPending: false } as never)
  vi.mocked(useUpdateMySupportTeammatePresence).mockReturnValue({ mutate, isPending: false } as never)
})

test('shows effective availability and sends a manual override', () => {
  render(<SupportAvailabilityControl workspaceId="ws-1" userId="user-1" enabled />)
  expect(screen.getAllByText('Online')).toHaveLength(2)
  fireEvent.click(screen.getByRole('button', { name: 'Away' }))
  expect(mutate).toHaveBeenCalledWith('away', expect.anything())
})

test('maps Automatic back to a null override', () => {
  vi.mocked(useSupportTeammatePresence).mockReturnValue({ data: [
    { user_id: 'user-1', status: 'offline', source: 'manual', manual_status: 'offline' },
  ], isPending: false } as never)
  render(<SupportAvailabilityControl workspaceId="ws-1" userId="user-1" enabled />)
  fireEvent.click(screen.getByRole('button', { name: 'Auto' }))
  expect(mutate).toHaveBeenCalledWith(null, expect.anything())
})

test('does not render without support access', () => {
  const { container } = render(<SupportAvailabilityControl workspaceId="ws-1" userId="user-1" enabled={false} />)
  expect(container.textContent).toBe('')
  expect(useSupportTeammatePresence).toHaveBeenCalledWith('ws-1', false)
})
