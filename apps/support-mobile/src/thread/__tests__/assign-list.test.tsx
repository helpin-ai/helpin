import { fireEvent, render, screen } from '@testing-library/react'
import { useConversationAssignees } from '@helpin-ai/support-core'
import { AssignList } from '../assign-list'

vi.mock('@helpin-ai/support-core', () => ({
  useConversationAssignees: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

const mockUseConversationAssignees = vi.mocked(useConversationAssignees)

function mockQuery(overrides: Partial<{ data: unknown; isLoading: boolean; isError: boolean; refetch: () => void }>) {
  mockUseConversationAssignees.mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useConversationAssignees>)
}

afterEach(() => {
  mockUseConversationAssignees.mockReset()
})

test('error state renders the "Couldn\'t load teammates" row', () => {
  mockQuery({ isError: true })
  render(<AssignList workspaceId="ws-1" conversationId="conv-1" onSelect={vi.fn()} />)
  expect(screen.getByText("Couldn't load teammates")).toBeDefined()
})

test('tapping Retry in the error state calls refetch', () => {
  const refetch = vi.fn().mockResolvedValue(undefined)
  mockQuery({ isError: true, refetch })
  render(<AssignList workspaceId="ws-1" conversationId="conv-1" onSelect={vi.fn()} />)

  fireEvent.click(screen.getByText('Retry'))
  expect(refetch).toHaveBeenCalledTimes(1)
})

test('success state renders assignable teammates, not the error row', () => {
  mockQuery({
    data: [
      { id: 'member-1', user_id: 'user-1', role: 'member', email: 'g@example.com', display_name: 'Grace Hopper' },
      // No user_id — not assignable, must be filtered out.
      { id: 'member-2', user_id: null, role: 'member', email: 'x@example.com', display_name: 'No Account' },
    ],
  })
  render(<AssignList workspaceId="ws-1" conversationId="conv-1" onSelect={vi.fn()} />)

  expect(screen.getByText('Grace Hopper')).toBeDefined()
  expect(screen.queryByText('No Account')).toBeNull()
  expect(screen.queryByText("Couldn't load teammates")).toBeNull()
})
