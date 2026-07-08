import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { useSendMessage } from '@helpin-ai/support-core'
import { haptic } from '@mobile/lib/haptics'
import { Composer } from '../composer'
import { useDraftStore } from '../draft-store'

vi.mock('@helpin-ai/support-core', () => ({
  useSendMessage: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

const mockUseSendMessage = vi.mocked(useSendMessage)

function setupMutate(impl: (payload: { content: string; is_internal?: boolean }) => Promise<unknown>) {
  const mutateAsync = vi.fn(impl)
  mockUseSendMessage.mockReturnValue({ mutateAsync } as unknown as ReturnType<typeof useSendMessage>)
  return mutateAsync
}

beforeEach(() => {
  sessionStorage.clear()
  useDraftStore.setState({ drafts: {} })
  vi.mocked(haptic).mockClear()
})

test('reply mode shows the "Reply…" placeholder; switching to Note tints the composer and swaps the placeholder', () => {
  setupMutate(async () => ({}))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  expect(screen.getByPlaceholderText('Reply…')).toBeDefined()

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))

  expect(screen.getByPlaceholderText('Internal note…')).toBeDefined()
  expect(useDraftStore.getState().drafts['conv-1']?.mode).toBe('note')
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

test('sending trims content and maps note mode to is_internal, clears the draft immediately, and fires a success haptic', async () => {
  const mutateAsync = setupMutate(async () => ({ id: 'msg-1' }))
  render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

  fireEvent.click(screen.getByRole('button', { name: 'Note' }))
  fireEvent.change(screen.getByPlaceholderText('Internal note…'), { target: { value: '  internal thought  ' } })
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
  fireEvent.change(screen.getByPlaceholderText('Internal note…'), { target: { value: 'secret note' } })
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
