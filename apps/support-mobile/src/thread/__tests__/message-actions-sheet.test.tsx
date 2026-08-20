import { fireEvent, render, screen } from '@testing-library/react'
import {
  useDeleteSupportMessage,
  useMessageInfo,
  type SupportMessage,
} from '@helpin-ai/support-core'
import { MessageActionsSheet, messageActionText } from '../message-actions-sheet'

vi.mock('@helpin-ai/support-core', () => ({
  useDeleteSupportMessage: vi.fn(),
  useMessageInfo: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('sonner', () => {
  const toast = Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  })
  return { toast }
})

const MESSAGE: SupportMessage = {
  id: 'msg-1',
  workspace_id: 'ws-1',
  conversation_id: 'conv-1',
  sender_type: 'user',
  sender_user_id: 'user-1',
  content: 'Hello\nthere',
  cancellable_until: '2099-08-20T00:00:00.000Z',
  is_internal: false,
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}

function setup(message = MESSAGE) {
  const mutate = vi.fn()
  vi.mocked(useDeleteSupportMessage).mockReturnValue({
    mutate,
    isPending: false,
  } as unknown as ReturnType<typeof useDeleteSupportMessage>)
  vi.mocked(useMessageInfo).mockReturnValue({
    data: {
      id: message.id,
      sent_at: '2026-08-20T00:00:00.000Z',
      sender: { name: 'Grace Hopper', type: 'user' },
      from: 'Grace Hopper',
      to_email: 'ada@example.com',
      origin: 'support_inbox',
      type: 'reply',
      email_delivery_status_label: 'Delivered',
      read: true,
      read_at: '2026-08-20T00:01:00.000Z',
      edited: false,
      translated: false,
      automated: false,
    },
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useMessageInfo>)
  const restoreDraft = vi.fn()
  const onOpenChange = vi.fn()

  render(
    <MessageActionsSheet
      open
      onOpenChange={onOpenChange}
      workspaceId="ws-1"
      message={message}
      currentUserId="user-1"
      canEditSupport
      onRestoreDraft={restoreDraft}
    />,
  )

  return { mutate, restoreDraft, onOpenChange }
}

beforeEach(() => vi.clearAllMocks())

test('quotes visible message text into the reply composer', () => {
  const { restoreDraft } = setup()

  fireEvent.click(screen.getByRole('button', { name: 'Reply with quote' }))

  expect(restoreDraft).toHaveBeenCalledWith('> Hello\n> there\n\n')
})

test('offers cancellable edit and own-message delete with exact mutation flags', () => {
  const { mutate } = setup()

  fireEvent.click(screen.getByRole('button', { name: 'Edit message' }))
  expect(mutate).toHaveBeenCalledWith(
    { messageId: 'msg-1', undo: true },
    expect.anything(),
  )

  fireEvent.click(screen.getByRole('button', { name: 'Delete message' }))
  fireEvent.click(screen.getByRole('button', { name: 'Delete message' }))
  expect(mutate).toHaveBeenLastCalledWith(
    { messageId: 'msg-1' },
    expect.anything(),
  )
})

test('shows delivery and recipient information', () => {
  setup()

  fireEvent.click(screen.getByRole('button', { name: 'Message info' }))

  expect(screen.getAllByText('Grace Hopper')).toHaveLength(2)
  expect(screen.getByText('ada@example.com')).toBeDefined()
  expect(screen.getByText('Delivered')).toBeDefined()
  expect(screen.getByText('Support Inbox')).toBeDefined()
})

test('uses projected visible email text and hides mutating actions for customer messages', () => {
  const customerMessage: SupportMessage = {
    ...MESSAGE,
    sender_type: 'customer',
    sender_user_id: undefined,
    email_visible_text: 'Visible reply',
    content: 'Visible reply\n\nOld quoted history',
  }
  setup(customerMessage)

  expect(messageActionText(customerMessage)).toBe('Visible reply')
  expect(screen.queryByRole('button', { name: 'Edit message' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Delete message' })).toBeNull()
})
