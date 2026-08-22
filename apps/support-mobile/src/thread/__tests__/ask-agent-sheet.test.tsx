import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  useEnsureConversationDockChat,
  useGenerateSupportDockChatTitle,
  useResolveSupportDockRunInteraction,
  useSendSupportDockChatMessage,
  useSupportDockChat,
  useSupportDockChatMessages,
  useSupportDockRunInteractions,
  supportService,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { AskAgentSheet } from '../ask-agent-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useEnsureConversationDockChat: vi.fn(),
  useGenerateSupportDockChatTitle: vi.fn(),
  useResolveSupportDockRunInteraction: vi.fn(),
  useSendSupportDockChatMessage: vi.fn(),
  useSupportDockChat: vi.fn(),
  useSupportDockChatMessages: vi.fn(),
  useSupportDockRunInteractions: vi.fn(),
  supportService: { listSupportDockChatMessages: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const conversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: 'Refund request', status: 'open', priority: 'medium', source: 'widget',
  created_at: '2026-08-20T10:00:00Z', updated_at: '2026-08-20T10:00:00Z',
} satisfies SupportConversation
const chat = {
  id: 'chat-1', workspace_id: 'ws-1', user_id: 'user-1', title: '', visibility: 'module' as const, module_id: 'support' as const,
  support_conversation_id: 'conv-1', created_at: '2026-08-20T10:00:00Z', updated_at: '2026-08-20T10:00:00Z',
}
const ensureMutate = vi.fn()
const sendMutateAsync = vi.fn()
const generateMutate = vi.fn()
const resolveMutate = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
  ensureMutate.mockImplementation((_id, options) => options.onSuccess(chat))
  sendMutateAsync.mockResolvedValue({ chat, plan_ids: [] })
  vi.mocked(useEnsureConversationDockChat).mockReturnValue({ mutate: ensureMutate, isPending: false, isError: false } as never)
  vi.mocked(useSupportDockChat).mockReturnValue({ data: { chat, run: null, plan_ids: [] } } as never)
  vi.mocked(useSupportDockChatMessages).mockReturnValue({ data: { messages: [] }, isPending: false } as never)
  vi.mocked(useSendSupportDockChatMessage).mockReturnValue({ mutateAsync: sendMutateAsync, isPending: false } as never)
  vi.mocked(useGenerateSupportDockChatTitle).mockReturnValue({ mutate: generateMutate } as never)
  vi.mocked(useSupportDockRunInteractions).mockReturnValue({ data: { interactions: [] } } as never)
  vi.mocked(useResolveSupportDockRunInteraction).mockReturnValue({ mutate: resolveMutate, isPending: false } as never)
  vi.mocked(supportService.listSupportDockChatMessages).mockResolvedValue({ data: { messages: [], next_before: null }, error: null, status: 200 })
})

test('ensures the associated chat and sends a web-parity starter with conversation context', async () => {
  render(<AskAgentSheet workspaceId="ws-1" conversation={conversation} open onOpenChange={vi.fn()} canResolveInteractions />)
  await waitFor(() => expect(ensureMutate).toHaveBeenCalledWith('conv-1', expect.anything()))
  fireEvent.click(await screen.findByRole('button', { name: 'Draft a reply' }))
  await waitFor(() => expect(sendMutateAsync).toHaveBeenCalledWith(expect.objectContaining({
    content: expect.stringContaining('draft a helpful, accurate reply'),
    pageContext: { entity_type: 'support_conversation', entity_id: 'conv-1', display_title: 'Refund request' },
    clientMessageId: expect.stringMatching(/^[0-9a-f-]{36}$/i),
  })))
  expect(generateMutate).toHaveBeenCalled()
})

test('renders persisted messages and resolves structured agent input', async () => {
  vi.mocked(useSupportDockChat).mockReturnValue({ data: { chat: { ...chat, title: 'Refund help' }, run: { id: 'run-1', status: 'paused' }, plan_ids: [] } } as never)
  vi.mocked(useSupportDockChatMessages).mockReturnValue({ data: { messages: [
    { id: 'msg-1', role: 'user', content: 'Please investigate', created_at: '2026-08-20T10:01:00Z' },
    { id: 'msg-2', role: 'assistant', content: 'I need your approval.', created_at: '2026-08-20T10:02:00Z' },
  ] }, isPending: false } as never)
  vi.mocked(useSupportDockRunInteractions).mockReturnValue({ data: { interactions: [
    { id: 'int-1', interaction_kind: 'approval_request', status: 'pending', request_schema_version: 'helpin.v1', request_payload: {}, title: 'Continue investigation' },
  ] } } as never)

  render(<AskAgentSheet workspaceId="ws-1" conversation={conversation} open onOpenChange={vi.fn()} canResolveInteractions />)
  expect(await screen.findByText('I need your approval.')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
  expect(resolveMutate).toHaveBeenCalledWith(
    { interactionId: 'int-1', payload: { response_payload: { decision: 'approve' } } },
    expect.anything(),
  )
})

test('renders Ask Agent transcript messages as GitHub-flavoured Markdown', async () => {
  vi.mocked(useSupportDockChatMessages).mockReturnValue({ data: { messages: [
    {
      id: 'msg-markdown',
      role: 'assistant',
      content: '## Suggested reply\n\n- **Confirm** the refund\n- Share `REF-123`\n\n[Open policy](https://example.com/policy)',
      created_at: '2026-08-20T10:02:00Z',
    },
  ] }, isPending: false } as never)

  render(<AskAgentSheet workspaceId="ws-1" conversation={conversation} open onOpenChange={vi.fn()} canResolveInteractions />)

  expect(await screen.findByRole('heading', { name: 'Suggested reply', level: 2 })).toBeTruthy()
  expect(screen.getAllByRole('listitem')).toHaveLength(2)
  expect(screen.getByText('Confirm').tagName).toBe('STRONG')
  expect(screen.getByText('REF-123').tagName).toBe('CODE')
  const link = screen.getByRole('link', { name: 'Open policy' })
  expect(link.getAttribute('href')).toBe('https://example.com/policy')
  expect(link.getAttribute('target')).toBe('_blank')
})

test('converts AI usage failures into the mobile upgrade sheet', async () => {
  sendMutateAsync.mockRejectedValue(new Error('AI usage exhausted'))
  render(<AskAgentSheet workspaceId="ws-1" conversation={conversation} open onOpenChange={vi.fn()} canResolveInteractions />)
  const input = await screen.findByLabelText('Message Ask Agent')
  fireEvent.change(input, { target: { value: 'Help me' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send to Ask Agent' }))
  expect((await screen.findAllByText('Upgrade to continue')).length).toBeGreaterThan(0)
})

test('loads earlier transcript pages through the stable sequence cursor', async () => {
  vi.mocked(useSupportDockChatMessages).mockReturnValue({ data: { messages: [
    { id: 'msg-51', role: 'assistant', content: 'Latest answer', dock_chat_sequence: 51, created_at: '2026-08-20T10:51:00Z' },
  ], next_before: 51 }, isPending: false } as never)
  vi.mocked(supportService.listSupportDockChatMessages).mockResolvedValue({ data: { messages: [
    { id: 'msg-1', workspace_id: 'ws-1', run_id: 'run-1', role: 'user', content: 'First question', message_type: 'message', sequence_no: 1, dock_chat_sequence: 1, created_at: '2026-08-20T10:01:00Z' },
  ], next_before: null }, error: null, status: 200 })

  render(<AskAgentSheet workspaceId="ws-1" conversation={conversation} open onOpenChange={vi.fn()} canResolveInteractions />)
  fireEvent.click(await screen.findByRole('button', { name: 'Load earlier messages' }))
  await waitFor(() => expect(supportService.listSupportDockChatMessages).toHaveBeenCalledWith('ws-1', 'chat-1', 51))
  expect(await screen.findByText('First question')).toBeTruthy()
})
