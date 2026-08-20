import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  useDeleteConversation,
  useInboxScopes,
  useMarkConversationUnread,
  useMoveConversation,
  useSendConversationTranscript,
  useUpdateConversationStatus,
  useUpdateConversationSubject,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { ConversationActionsSheet } from '../conversation-actions-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useDeleteConversation: vi.fn(),
  useInboxScopes: vi.fn(),
  useMarkConversationUnread: vi.fn(),
  useMoveConversation: vi.fn(),
  useSendConversationTranscript: vi.fn(),
  useUpdateConversationStatus: vi.fn(),
  useUpdateConversationSubject: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const CONVERSATION: SupportConversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 42, subject: 'Billing help',
  status: 'open', priority: 'medium', customer_name: 'Ada', customer_email: 'ada@example.com',
  email_cc: ['finance@example.com'], source: 'email',
  created_at: '2026-08-20T00:00:00.000Z', updated_at: '2026-08-20T00:00:00.000Z',
}

const mockUseSendConversationTranscript = vi.mocked(useSendConversationTranscript)

function setup() {
  const transcript = vi.fn()
  vi.mocked(useDeleteConversation).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useDeleteConversation>)
  vi.mocked(useInboxScopes).mockReturnValue({ data: { mailboxes: [] }, isPending: false } as unknown as ReturnType<typeof useInboxScopes>)
  vi.mocked(useMarkConversationUnread).mockReturnValue({ mutate: vi.fn() } as unknown as ReturnType<typeof useMarkConversationUnread>)
  vi.mocked(useMoveConversation).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useMoveConversation>)
  mockUseSendConversationTranscript.mockReturnValue({ mutate: transcript, isPending: false } as unknown as ReturnType<typeof useSendConversationTranscript>)
  vi.mocked(useUpdateConversationStatus).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useUpdateConversationStatus>)
  vi.mocked(useUpdateConversationSubject).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useUpdateConversationSubject>)
  return { transcript }
}

beforeEach(() => vi.clearAllMocks())

test('copies the current conversation URL', async () => {
  setup()
  const writeText = vi.fn(async () => undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  render(
    <ConversationActionsSheet open onOpenChange={vi.fn()} workspaceId="ws-1" conversation={CONVERSATION} onLeave={vi.fn()} />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))

  await waitFor(() => expect(writeText).toHaveBeenCalledWith(window.location.href))
})

test('sends a transcript to a selected recent recipient', () => {
  const { transcript } = setup()
  render(
    <ConversationActionsSheet open onOpenChange={vi.fn()} workspaceId="ws-1" conversation={CONVERSATION} onLeave={vi.fn()} />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Email transcript' }))
  fireEvent.click(screen.getByRole('button', { name: 'Send transcript to finance@example.com' }))
  fireEvent.click(screen.getByRole('button', { name: 'Send transcript' }))

  expect(transcript).toHaveBeenCalledWith(
    { conversationId: 'conv-1', email: 'finance@example.com', updateCustomerEmail: false },
    expect.anything(),
  )
})

test('can save a custom transcript email to a customer without an email', () => {
  const { transcript } = setup()
  render(
    <ConversationActionsSheet
      open
      onOpenChange={vi.fn()}
      workspaceId="ws-1"
      conversation={{ ...CONVERSATION, customer_email: undefined, email_cc: [] }}
      onLeave={vi.fn()}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Email transcript' }))
  fireEvent.change(screen.getByPlaceholderText('recipient@example.com'), { target: { value: 'new@example.com' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send transcript' }))

  expect(transcript).toHaveBeenCalledWith(
    { conversationId: 'conv-1', email: 'new@example.com', updateCustomerEmail: true },
    expect.anything(),
  )
})
