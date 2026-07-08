import { render, screen, fireEvent } from '@testing-library/react'
import {
  useAssignConversationUser,
  useConversation,
  useConversationAssignees,
  useUpdateConversationStatus,
  useVisitorContext,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { ContextSheet } from '../context-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useConversation: vi.fn(),
  useVisitorContext: vi.fn(),
  useConversationAssignees: vi.fn(),
  useUpdateConversationStatus: vi.fn(),
  useAssignConversationUser: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockUseConversation = vi.mocked(useConversation)
const mockUseVisitorContext = vi.mocked(useVisitorContext)
const mockUseConversationAssignees = vi.mocked(useConversationAssignees)
const mockUseUpdateConversationStatus = vi.mocked(useUpdateConversationStatus)
const mockUseAssignConversationUser = vi.mocked(useAssignConversationUser)

const BASE_CONVERSATION: SupportConversation = {
  id: 'conv-1',
  workspace_id: 'ws-1',
  display_id: 42,
  subject: 'Help with billing',
  status: 'open',
  priority: 'medium',
  customer_name: 'Ada Lovelace',
  customer_email: 'ada@example.com',
  assigned_user_id: undefined,
  source: 'widget',
  created_at: '2026-07-01T00:00:00.000Z',
  updated_at: '2026-07-01T00:00:00.000Z',
}

const TEAMMATES = [
  { id: 'member-1', user_id: 'user-1', role: 'member', email: 'grace@example.com', display_name: 'Grace Hopper' },
  { id: 'member-2', user_id: 'user-2', role: 'admin', email: 'alan@example.com', display_name: 'Alan Turing' },
]

function setup({
  conversation = BASE_CONVERSATION,
  statusMutate = vi.fn(),
  assignMutate = vi.fn(),
}: {
  conversation?: SupportConversation
  statusMutate?: ReturnType<typeof vi.fn>
  assignMutate?: ReturnType<typeof vi.fn>
} = {}) {
  mockUseConversation.mockReturnValue({ data: conversation } as unknown as ReturnType<typeof useConversation>)
  mockUseVisitorContext.mockReturnValue({ data: undefined } as unknown as ReturnType<typeof useVisitorContext>)
  mockUseConversationAssignees.mockReturnValue({
    data: TEAMMATES,
    isLoading: false,
  } as unknown as ReturnType<typeof useConversationAssignees>)
  mockUseUpdateConversationStatus.mockReturnValue({
    mutate: statusMutate,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationStatus>)
  mockUseAssignConversationUser.mockReturnValue({
    mutate: assignMutate,
    isPending: false,
  } as unknown as ReturnType<typeof useAssignConversationUser>)

  return { statusMutate, assignMutate }
}

beforeEach(() => {
  vi.clearAllMocks()
})

test('renders the customer name and email from the conversation', () => {
  setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} />)

  expect(screen.getByText('Ada Lovelace')).toBeDefined()
  expect(screen.getByText('ada@example.com')).toBeDefined()
})

test('Resolve calls the status mutation with resolved for an open conversation', () => {
  const { statusMutate } = setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} />)

  fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))

  expect(statusMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', status: 'resolved' },
    expect.anything(),
  )
})

test('Reopen calls the status mutation with open for a resolved conversation', () => {
  const { statusMutate } = setup({ conversation: { ...BASE_CONVERSATION, status: 'resolved' } })
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} />)

  fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))

  expect(statusMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', status: 'open' },
    expect.anything(),
  )
})

test('tapping a teammate in the expanded assign list calls the assign mutation with that user id', () => {
  const { assignMutate } = setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} />)

  fireEvent.click(screen.getByRole('button', { name: 'Assign' }))
  fireEvent.click(screen.getByRole('button', { name: 'Grace Hopper' }))

  expect(assignMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', userId: 'user-1' },
    expect.anything(),
  )
})

test('tapping Unassign in the expanded assign list calls the assign mutation with a null user id', () => {
  const { assignMutate } = setup({ conversation: { ...BASE_CONVERSATION, assigned_user_id: 'user-1' } })
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} />)

  fireEvent.click(screen.getByRole('button', { name: 'Assign' }))
  fireEvent.click(screen.getByRole('button', { name: 'Unassign' }))

  expect(assignMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', userId: null },
    expect.anything(),
  )
})
