import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  useDeleteSupportAttachment,
  useSendMessage,
  useUpdateConversationEmailRecipients,
  useUploadSupportAttachment,
} from '@helpin-ai/support-core'
import { haptic } from '@mobile/lib/haptics'
import { Composer } from '../composer'
import { useDraftStore } from '../draft-store'

vi.mock('@helpin-ai/support-core', () => ({
  useSendMessage: vi.fn(),
  useUploadSupportAttachment: vi.fn(),
  useDeleteSupportAttachment: vi.fn(),
  // Typing broadcast reads wsSend/wsConnected via a selector; return a
  // disconnected state so the composer's typing hook is a no-op here.
  useSupportPresenceStore: (selector: (s: { wsSend: null; wsConnected: boolean }) => unknown) =>
    selector({ wsSend: null, wsConnected: false }),
  useRewriteSupportDraft: () => ({ mutateAsync: vi.fn() }),
  useSupportCannedResponses: () => ({ data: [], isPending: false }),
  useUpdateConversationEmailRecipients: vi.fn(),
  useCreateSupportCannedResponse: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateSupportCannedResponse: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteSupportCannedResponse: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

const mockUseSendMessage = vi.mocked(useSendMessage)
const mockUseUploadSupportAttachment = vi.mocked(useUploadSupportAttachment)
const mockUseDeleteSupportAttachment = vi.mocked(useDeleteSupportAttachment)
const mockUseUpdateConversationEmailRecipients = vi.mocked(useUpdateConversationEmailRecipients)

function setupMutate(impl: (payload: { content: string; is_internal?: boolean; attachment_ids?: string[] }) => Promise<unknown>) {
  const mutateAsync = vi.fn(impl)
  mockUseSendMessage.mockReturnValue({ mutateAsync } as unknown as ReturnType<typeof useSendMessage>)
  return mutateAsync
}

beforeEach(() => {
  sessionStorage.clear()
  useDraftStore.setState({ drafts: {} })
  vi.mocked(haptic).mockClear()
  mockUseUploadSupportAttachment.mockReturnValue({
    mutateAsync: vi.fn(async () => ({ id: 'att-default', url: 'https://files.test/default' })),
  } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  mockUseDeleteSupportAttachment.mockReturnValue({ mutateAsync: vi.fn(async () => undefined) } as unknown as ReturnType<typeof useDeleteSupportAttachment>)
  mockUseUpdateConversationEmailRecipients.mockReturnValue({
    mutateAsync: vi.fn(async () => ({})),
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationEmailRecipients>)
})

test('reply mode shows the "Reply…" placeholder; switching to Note tints the composer and swaps the placeholder', () => {
  setupMutate(async () => ({}))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  expect(screen.getByPlaceholderText('Reply…')).toBeDefined()

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))

  expect(screen.getByPlaceholderText(/Internal note…/)).toBeDefined()
  expect(useDraftStore.getState().drafts['conv-1']?.mode).toBe('note')
})

test('tapping Reply expands and focuses the mobile writing area, while Note returns it to compact height', () => {
  setupMutate(async () => ({}))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const replyButton = screen.getByRole('button', { name: 'Reply' })
  const textarea = screen.getByPlaceholderText('Reply…') as HTMLTextAreaElement
  expect(textarea.style.minHeight).toBe('56px')

  fireEvent.click(replyButton)
  expect(replyButton.getAttribute('aria-expanded')).toBe('true')
  expect(textarea.style.minHeight).toBe('92px')
  expect(document.activeElement).toBe(textarea)

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))
  expect((screen.getByPlaceholderText(/Internal note…/) as HTMLTextAreaElement).style.minHeight).toBe('56px')
})

test('note mode exposes the workspace teammate picker and inserts the selected mention', () => {
  setupMutate(async () => ({}))
  render(
    <Composer
      workspaceId="ws-1"
      conversationId="conv-1"
      mentionMembers={[
        { id: 'member-1', user_id: 'user-1', email: 'marcus@example.com', display_name: 'Marcus Bell' },
        { id: 'member-2', email: 'pending@example.com', display_name: 'Pending Teammate' },
      ]}
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))
  fireEvent.click(screen.getByRole('button', { name: 'Mention teammate' }))

  expect(screen.getByText('Marcus Bell')).toBeDefined()
  expect(screen.getByText('Pending Teammate')).toBeDefined()
  fireEvent.click(screen.getByText('Marcus Bell'))
  expect((screen.getByPlaceholderText(/Internal note/) as HTMLTextAreaElement).value).toBe('@marcus.bell ')
})

test('typing populates the draft store keyed by conversationId', () => {
  setupMutate(async () => ({}))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'Hello' } })

  expect(useDraftStore.getState().drafts['conv-1']?.text).toBe('Hello')
})

test('send button is disabled while the draft is empty or whitespace-only', () => {
  setupMutate(async () => ({}))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const button = screen.getByRole('button', { name: 'Send message' })
  expect(button).toHaveProperty('disabled', true)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: '   ' } })
  expect(button).toHaveProperty('disabled', true)
})

test('blocks ambiguous email replies until the suggested primary recipient is confirmed', async () => {
  setupMutate(async () => ({}))
  const updateRecipients = vi.fn(async () => ({}))
  mockUseUpdateConversationEmailRecipients.mockReturnValue({
    mutateAsync: updateRecipients,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationEmailRecipients>)
  render(
    <Composer
      workspaceId="ws-1"
      conversationId="conv-1"
      conversation={{
        id: 'conv-1',
        customer_email: 'support@example.com',
        email_cc: ['customer@example.com'],
        primary_recipient_state: 'unconfirmed',
        suggested_primary_recipient_email: 'customer@example.com',
        suggested_primary_recipient_name: 'Ada',
      }}
    />,
  )

  expect(screen.getByText(/Support was copied on this email/)).toBeDefined()
  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'Hello' } })
  expect(screen.getByRole('button', { name: 'Send message' })).toHaveProperty('disabled', true)
  fireEvent.click(screen.getByRole('button', { name: 'Use customer@example.com' }))

  await waitFor(() => expect(updateRecipients).toHaveBeenCalledWith({
    conversationId: 'conv-1',
    payload: {
      primary_recipient_email: 'customer@example.com',
      primary_recipient_name: 'Ada',
      confirm_primary: true,
    },
  }))
})

test('sends web-compatible email channel and normalized Cc metadata', async () => {
  const mutateAsync = setupMutate(async () => ({ id: 'msg-email' }))
  render(
    <Composer
      workspaceId="ws-1"
      conversationId="conv-1"
      conversation={{
        id: 'conv-1',
        customer_email: 'ada@example.com',
        email_cc: ['ADA@example.com', 'finance@example.com', 'Finance@Example.com'],
        primary_recipient_state: 'confirmed',
      }}
    />,
  )

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'Invoice attached' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({
    content: 'Invoice attached',
    is_internal: false,
    channels: ['email'],
    cc_emails: ['finance@example.com'],
  }))
})

test('sending trims content and maps note mode to is_internal, clears the draft immediately, and fires a success haptic', async () => {
  const mutateAsync = setupMutate(async () => ({ id: 'msg-1' }))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))
  fireEvent.change(screen.getByPlaceholderText(/Internal note…/), { target: { value: '  internal thought  ' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

  // Draft text clears the instant Send is pressed, before the network resolves.
  expect(useDraftStore.getState().drafts['conv-1']?.text ?? '').toBe('')
  expect(mutateAsync).toHaveBeenCalledWith({ content: 'internal thought', is_internal: true })

  await waitFor(() => expect(haptic).toHaveBeenCalledWith('notificationSuccess'))
})

test('rapid double-tap fires exactly one send (synchronous sendingRef lock)', async () => {
  let resolveSend: (() => void) | undefined
  const mutateAsync = setupMutate(
    () =>
      new Promise((resolve) => {
        resolveSend = () => resolve({ id: 'msg-1' })
      }),
  )
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'once only' } })
  const button = screen.getByRole('button', { name: 'Send message' })
  fireEvent.click(button)
  fireEvent.click(button)
  fireEvent.click(button)

  expect(mutateAsync).toHaveBeenCalledTimes(1)
  resolveSend?.()
  await waitFor(() => expect(haptic).toHaveBeenCalledWith('notificationSuccess'))
})

test('a failed send shows a retry chip with the original content instead of restoring the input, and fires an error haptic', async () => {
  setupMutate(async () => {
    throw new Error('network down')
  })
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'will fail' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

  await waitFor(() => expect(screen.getByText('will fail')).toBeDefined())
  expect(haptic).toHaveBeenCalledWith('notificationError')
  expect((screen.getByPlaceholderText('Reply…') as HTMLTextAreaElement).value).toBe('')
  // The chip lives in the draft store (conversation-scoped and persisted),
  // not in composer-local state that a navigation would discard.
  expect(useDraftStore.getState().drafts['conv-1']?.failedSends.map((f) => f.content)).toEqual(['will fail'])
})

test('failed chips are conversation-scoped: a chip from another conversation never renders here', () => {
  setupMutate(async () => ({}))
  useDraftStore.getState().addFailedSend('conv-OTHER', { id: 'f-x', content: 'other conv failure', mode: 'reply' })

  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  expect(screen.queryByText('other conv failure')).toBeNull()
})

test('tapping Retry on a failed chip re-sends the same content and removes the chip on success', async () => {
  let shouldFail = true
  const mutateAsync = setupMutate(async (payload) => {
    if (shouldFail) throw new Error('still down')
    return { id: 'msg-2', content: payload.content }
  })
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'retry me' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
  await waitFor(() => expect(screen.getByText('retry me')).toBeDefined())

  shouldFail = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

  await waitFor(() => expect(screen.queryByText('retry me')).toBeNull())
  expect(mutateAsync).toHaveBeenCalledTimes(2)
  expect(mutateAsync).toHaveBeenLastCalledWith({ content: 'retry me', is_internal: false })
  expect(useDraftStore.getState().drafts['conv-1']?.failedSends ?? []).toEqual([])
})

test('a failed NOTE keeps is_internal: true through the chip round-trip on retry', async () => {
  let shouldFail = true
  const mutateAsync = setupMutate(async (payload) => {
    if (shouldFail) throw new Error('still down')
    return { id: 'msg-3', content: payload.content }
  })
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))
  fireEvent.change(screen.getByPlaceholderText(/Internal note…/), { target: { value: 'secret note' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
  await waitFor(() => expect(screen.getByText('secret note')).toBeDefined())
  expect(mutateAsync).toHaveBeenNthCalledWith(1, { content: 'secret note', is_internal: true })

  shouldFail = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

  await waitFor(() => expect(screen.queryByText('secret note')).toBeNull())
  // The retry must re-send as an internal note, not silently downgrade to a customer-visible reply.
  expect(mutateAsync).toHaveBeenNthCalledWith(2, { content: 'secret note', is_internal: true })
})

test('dismissing a failed chip removes it without retrying', async () => {
  setupMutate(async () => {
    throw new Error('down')
  })
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'discard me' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
  await waitFor(() => expect(screen.getByText('discard me')).toBeDefined())

  fireEvent.click(screen.getByRole('button', { name: 'Dismiss failed message' }))

  expect(screen.queryByText('discard me')).toBeNull()
  expect(useDraftStore.getState().drafts['conv-1']?.failedSends ?? []).toEqual([])
})

test('uploads a file and allows an attachment-only reply', async () => {
  const mutateAsync = setupMutate(async () => ({ id: 'msg-with-file' }))
  const upload = vi.fn(async () => ({ id: 'att-1', url: 'https://files.test/guide.pdf' }))
  mockUseUploadSupportAttachment.mockReturnValue({ mutateAsync: upload } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['guide'], 'guide.pdf', { type: 'application/pdf' })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })

  await waitFor(() => expect(screen.getByText('Ready')).toBeDefined())
  expect(screen.getByRole('button', { name: 'Send message' })).toHaveProperty('disabled', false)
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

  await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({
    content: '',
    is_internal: false,
    attachment_ids: ['att-1'],
  }))
  await waitFor(() => expect(screen.queryByText('guide.pdf')).toBeNull())
})

test('keeps a failed upload visible and retries it in place', async () => {
  setupMutate(async () => ({}))
  let shouldFail = true
  const upload = vi.fn(async () => {
    if (shouldFail) throw new Error('storage unavailable')
    return { id: 'att-retried', url: 'https://files.test/retried.pdf' }
  })
  mockUseUploadSupportAttachment.mockReturnValue({ mutateAsync: upload } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['retry'], 'retry.pdf', { type: 'application/pdf' })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })
  await waitFor(() => expect(screen.getByText('Upload failed')).toBeDefined())

  shouldFail = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry upload retry.pdf' }))

  await waitFor(() => expect(screen.getByText('Ready')).toBeDefined())
  expect(upload).toHaveBeenCalledTimes(2)
})

test('removing a confirmed staged attachment deletes its server record', async () => {
  setupMutate(async () => ({}))
  const remove = vi.fn(async () => undefined)
  mockUseUploadSupportAttachment.mockReturnValue({
    mutateAsync: vi.fn(async () => ({ id: 'att-remove', url: 'https://files.test/remove.pdf' })),
  } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  mockUseDeleteSupportAttachment.mockReturnValue({ mutateAsync: remove } as unknown as ReturnType<typeof useDeleteSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['remove'], 'remove.pdf', { type: 'application/pdf' })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })
  await waitFor(() => expect(screen.getByText('Ready')).toBeDefined())
  fireEvent.click(screen.getByRole('button', { name: 'Remove remove.pdf' }))

  await waitFor(() => expect(remove).toHaveBeenCalledWith('att-remove'))
  expect(screen.queryByText('remove.pdf')).toBeNull()
})

test('rejects files larger than 10 MB before starting an upload', () => {
  setupMutate(async () => ({}))
  const upload = vi.fn()
  mockUseUploadSupportAttachment.mockReturnValue({ mutateAsync: upload } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['large'], 'too-large.pdf', { type: 'application/pdf' })
  Object.defineProperty(file, 'size', { value: 10 * 1024 * 1024 + 1 })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })

  expect(upload).not.toHaveBeenCalled()
  expect(screen.queryByText('too-large.pdf')).toBeNull()
})

test('deletes an upload that finishes after its preview was removed', async () => {
  setupMutate(async () => ({}))
  let finishUpload: ((value: { id: string; url: string }) => void) | undefined
  const upload = vi.fn(() => new Promise<{ id: string; url: string }>((resolve) => { finishUpload = resolve }))
  const remove = vi.fn(async () => undefined)
  mockUseUploadSupportAttachment.mockReturnValue({ mutateAsync: upload } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  mockUseDeleteSupportAttachment.mockReturnValue({ mutateAsync: remove } as unknown as ReturnType<typeof useDeleteSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['pending'], 'pending.pdf', { type: 'application/pdf' })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })
  expect(await screen.findByText('Uploading')).toBeDefined()
  fireEvent.click(screen.getByRole('button', { name: 'Remove pending.pdf' }))

  finishUpload?.({ id: 'att-orphan', url: 'https://files.test/orphan.pdf' })
  await waitFor(() => expect(remove).toHaveBeenCalledWith('att-orphan'))
  expect(screen.queryByText('pending.pdf')).toBeNull()
})

test('keeps attachment IDs on a failed message and reuses them on retry', async () => {
  let shouldFail = true
  const send = setupMutate(async () => {
    if (shouldFail) throw new Error('message failed')
    return { id: 'msg-retried' }
  })
  mockUseUploadSupportAttachment.mockReturnValue({
    mutateAsync: vi.fn(async () => ({ id: 'att-retry-send', url: 'https://files.test/retry-send.pdf' })),
  } as unknown as ReturnType<typeof useUploadSupportAttachment>)
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  const file = new File(['retry send'], 'retry-send.pdf', { type: 'application/pdf' })
  fireEvent.change(screen.getByLabelText('Choose attachments'), { target: { files: [file] } })
  await waitFor(() => expect(screen.getByText('Ready')).toBeDefined())
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

  await waitFor(() => expect(screen.getByText('1 attachment')).toBeDefined())
  expect(useDraftStore.getState().drafts['conv-1']?.failedSends[0]?.attachmentIds).toEqual(['att-retry-send'])
  shouldFail = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

  await waitFor(() => expect(screen.queryByText('1 attachment')).toBeNull())
  expect(send).toHaveBeenLastCalledWith({
    content: '',
    is_internal: false,
    attachment_ids: ['att-retry-send'],
  })
  expect(screen.queryByText('retry-send.pdf')).toBeNull()
})
