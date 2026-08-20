import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { useSendMessage } from '@helpin-ai/support-core'
import { Composer } from '../composer'
import { useDraftStore } from '../draft-store'

// Controllable rewrite: echoes the operation so the test can assert the result.
const rewriteMutate = vi.fn(async ({ content, operation }: { content: string; operation: string }) => ({
  content: `[${operation}] ${content}`,
  operation,
  provider: 'test',
  model: 'test',
}))

vi.mock('@helpin-ai/support-core', () => ({
  useSendMessage: vi.fn(),
  useUploadSupportAttachment: () => ({ mutateAsync: vi.fn() }),
  useDeleteSupportAttachment: () => ({ mutateAsync: vi.fn() }),
  useSupportPresenceStore: (selector: (s: { wsSend: null; wsConnected: boolean }) => unknown) =>
    selector({ wsSend: null, wsConnected: false }),
  useRewriteSupportDraft: () => ({ mutateAsync: rewriteMutate }),
  useSupportCannedResponses: () => ({ data: [], isPending: false }),
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))

const mockUseSendMessage = vi.mocked(useSendMessage)

function setupSend(impl: (payload: { content: string; is_internal?: boolean }) => Promise<unknown>) {
  const mutateAsync = vi.fn(impl)
  mockUseSendMessage.mockReturnValue({ mutateAsync } as unknown as ReturnType<typeof useSendMessage>)
  return mutateAsync
}

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
  useDraftStore.setState({ drafts: {} })
  rewriteMutate.mockClear()
})

describe('AI rewrite + undo', () => {
  test('rewrites the draft in place, then Undo restores the original', async () => {
    setupSend(async () => ({}))
    render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

    fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'plz help' } })
    fireEvent.click(screen.getByRole('button', { name: 'AI writing tools' }))

    // Sheet opens with the operations; pick "Rephrase".
    fireEvent.click(await screen.findByLabelText('Rephrase'))

    await waitFor(() =>
      expect((screen.getByPlaceholderText('Reply…') as HTMLTextAreaElement).value).toBe('[rephrase] plz help'),
    )
    expect(rewriteMutate).toHaveBeenCalledWith({ content: 'plz help', operation: 'rephrase' })

    // The Undo affordance restores the pre-rewrite text. `hidden: true` because
    // the just-closed vaul sheet leaves the app content aria-hidden in jsdom
    // (its close animation doesn't complete under the test runner).
    fireEvent.click(screen.getByRole('button', { name: 'Undo', hidden: true }))
    expect((screen.getByPlaceholderText('Reply…') as HTMLTextAreaElement).value).toBe('plz help')
  })

  test('shows an upgrade sheet instead of the raw AI usage error', async () => {
    setupSend(async () => ({}))
    rewriteMutate.mockRejectedValueOnce(new Error('AI usage exhausted'))
    render(<Composer workspaceId="ws-1" conversationId="conv-1" />)

    fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'plz help' } })
    fireEvent.click(screen.getByRole('button', { name: 'AI writing tools' }))
    fireEvent.click(await screen.findByLabelText('Rephrase'))

    expect(await screen.findByText('AI usage for this workspace is exhausted.')).toBeDefined()
    expect(screen.getByText('Larger included AI usage allowance')).toBeDefined()
    expect(screen.queryByText('AI usage exhausted')).toBeNull()
    expect((screen.getByPlaceholderText('Reply…', { exact: true }) as HTMLTextAreaElement).value).toBe('plz help')
  })
})

describe('email-fallback send confirm', () => {
  test('a reply that will email the customer asks for confirmation before sending', async () => {
    const send = setupSend(async () => ({ id: 'm1' }))
    render(<Composer workspaceId="ws-1" conversationId="conv-1" willSendAsEmail variableContext={{ customer: { email: 'ada@ex.com' } }} />)

    fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'here you go' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))

    // Not sent yet — the confirm sheet is shown instead.
    expect(send).not.toHaveBeenCalled()
    expect(await screen.findByText(/sent as an email/i)).toBeDefined()

    fireEvent.click(screen.getByRole('button', { name: 'Send as email' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith({ content: 'here you go', is_internal: false }))
  })

  test('“Don’t ask again” suppresses the confirm on the next send', async () => {
    const send = setupSend(async () => ({ id: 'm1' }))
    render(<Composer workspaceId="ws-1" conversationId="conv-1" willSendAsEmail />)

    fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'first' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
    fireEvent.click(await screen.findByLabelText("Don't ask again"))
    fireEvent.click(screen.getByRole('button', { name: 'Send as email' }))
    await waitFor(() => expect(send).toHaveBeenCalledTimes(1))

    // Second send goes straight through — no confirm sheet. `hidden: true`: the
    // closed confirm sheet leaves the app aria-hidden in jsdom; also wait out
    // the transient "Sent" state (name returns to "Send message").
    fireEvent.change(screen.getByPlaceholderText('Reply…'), { target: { value: 'second' } })
    fireEvent.click(await screen.findByRole('button', { name: 'Send message', hidden: true }))
    await waitFor(() => expect(send).toHaveBeenCalledTimes(2))
    expect(send).toHaveBeenLastCalledWith({ content: 'second', is_internal: false })
  })

  test('note-mode replies never trigger the email confirm', async () => {
    const send = setupSend(async () => ({ id: 'm1' }))
    render(<Composer workspaceId="ws-1" conversationId="conv-1" willSendAsEmail />)

    fireEvent.click(screen.getByRole('button', { name: 'Note' }))
    fireEvent.change(screen.getByPlaceholderText(/Internal note…/), { target: { value: 'private' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith({ content: 'private', is_internal: true }))
  })
})
