import { render, screen, fireEvent } from '@testing-library/react'
import {
  useAssignConversationUser,
  useConversation,
  useConversationAssignees,
  useUpdateConversationCustomerName,
  useUpdateConversationStatus,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { ContextSheet } from '../context-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useConversation: vi.fn(),
  useConversationAssignees: vi.fn(),
  useUpdateConversationStatus: vi.fn(),
  useAssignConversationUser: vi.fn(),
  useUpdateConversationCustomerName: vi.fn(),
}))

const navigate = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ slug: 'test-workspace' }),
  useRouter: () => ({ navigate }),
}))

vi.mock('../customer-context', () => ({
  CustomerContext: () => <div>Customer context</div>,
}))

vi.mock('../conversation-contact-tools', () => ({
  ConversationContactTools: () => <div>Contact tools</div>,
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockUseConversation = vi.mocked(useConversation)
const mockUseConversationAssignees = vi.mocked(useConversationAssignees)
const mockUseUpdateConversationStatus = vi.mocked(useUpdateConversationStatus)
const mockUseAssignConversationUser = vi.mocked(useAssignConversationUser)
const mockUseUpdateConversationCustomerName = vi.mocked(useUpdateConversationCustomerName)

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
  customerNameMutate = vi.fn(),
}: {
  conversation?: SupportConversation
  statusMutate?: ReturnType<typeof vi.fn>
  assignMutate?: ReturnType<typeof vi.fn>
  customerNameMutate?: ReturnType<typeof vi.fn>
} = {}) {
  mockUseConversation.mockReturnValue({ data: conversation } as unknown as ReturnType<typeof useConversation>)
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
  mockUseUpdateConversationCustomerName.mockReturnValue({
    mutate: customerNameMutate,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationCustomerName>)

  return { statusMutate, assignMutate, customerNameMutate }
}

beforeEach(() => {
  vi.clearAllMocks()
})

test('renders the customer name and email from the conversation', () => {
  setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  expect(screen.getByText('Ada Lovelace')).toBeDefined()
  expect(screen.getByText('ada@example.com')).toBeDefined()
})

test('edits the customer name through the existing conversation mutation', () => {
  const { customerNameMutate } = setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  fireEvent.click(screen.getByRole('button', { name: 'Edit customer name' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Customer name' }), {
    target: { value: 'Ada Byron' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save customer name' }))

  expect(customerNameMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', customerName: 'Ada Byron' },
    expect.anything(),
  )
})

test('Resolve calls the status mutation with resolved for an open conversation', () => {
  const { statusMutate } = setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))

  expect(statusMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', status: 'resolved' },
    expect.anything(),
  )
})

test('Reopen calls the status mutation with open for a resolved conversation', () => {
  const { statusMutate } = setup({ conversation: { ...BASE_CONVERSATION, status: 'resolved' } })
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))

  expect(statusMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', status: 'open' },
    expect.anything(),
  )
})

test('tapping a teammate in the expanded assign list calls the assign mutation with that user id', () => {
  const { assignMutate } = setup()
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  fireEvent.click(screen.getByRole('button', { name: 'Assign' }))
  fireEvent.click(screen.getByRole('button', { name: 'Grace Hopper' }))

  expect(assignMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', userId: 'user-1' },
    expect.anything(),
  )
})

test('tapping Unassign in the expanded assign list calls the assign mutation with a null user id', () => {
  const { assignMutate } = setup({ conversation: { ...BASE_CONVERSATION, assigned_user_id: 'user-1' } })
  render(<ContextSheet workspaceId="ws-1" conversationId="conv-1" open onOpenChange={() => {}} canEdit />)

  fireEvent.click(screen.getByRole('button', { name: 'Assign' }))
  fireEvent.click(screen.getByRole('button', { name: 'Unassign' }))

  expect(assignMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', userId: null },
    expect.anything(),
  )
})

test('read-only access hides resolve and assignment controls', () => {
  const { statusMutate, assignMutate } = setup()
  render(
    <ContextSheet
      workspaceId="ws-1"
      conversationId="conv-1"
      open
      onOpenChange={() => {}}
      canEdit={false}
    />,
  )

  expect(screen.getByText('Read-only access')).toBeDefined()
  expect(screen.queryByRole('button', { name: 'Resolve' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Assign' })).toBeNull()
  expect(statusMutate).not.toHaveBeenCalled()
  expect(assignMutate).not.toHaveBeenCalled()
})
