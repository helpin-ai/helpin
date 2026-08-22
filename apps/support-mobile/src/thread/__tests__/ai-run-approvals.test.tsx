import { fireEvent, render, screen } from '@testing-library/react'
import { useConversationAIRunInteractions, useResolveConversationAIRunInteraction } from '@helpin-ai/support-core'
import { AIRunApprovals } from '../ai-run-approvals'

vi.mock('@helpin-ai/support-core', () => ({
  useConversationAIRunInteractions: vi.fn(),
  useResolveConversationAIRunInteraction: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const mutate = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useResolveConversationAIRunInteraction).mockReturnValue({ mutate, isPending: false } as never)
})

test('resolves a pending approval with the exact decision payload', () => {
  vi.mocked(useConversationAIRunInteractions).mockReturnValue({ data: { run_id: 'run-1', interactions: [{ id: 'int-1', interaction_kind: 'approval_request', status: 'pending', request_schema_version: 'helpin.v1', request_payload: {}, title: 'Send refund' }] } } as never)
  render(<AIRunApprovals workspaceId="ws-1" conversationId="conv-1" enabled />)
  fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
  expect(mutate).toHaveBeenCalledWith({ interactionId: 'int-1', payload: { response_payload: { decision: 'approve' } } }, expect.anything())
})

test('answers a pending shared user-input question', () => {
  vi.mocked(useConversationAIRunInteractions).mockReturnValue({ data: { run_id: 'run-1', interactions: [{ id: 'int-2', interaction_kind: 'request_user_input', status: 'pending', request_schema_version: 'codex.v2', request_payload: { questions: [{ id: 'tone', question: 'Choose tone', options: [{ label: 'Friendly' }] }] } }] } } as never)
  render(<AIRunApprovals workspaceId="ws-1" conversationId="conv-1" enabled />)
  fireEvent.click(screen.getByRole('button', { name: 'Friendly' }))
  fireEvent.click(screen.getByRole('button', { name: 'Submit answers' }))
  expect(mutate).toHaveBeenCalledWith({ interactionId: 'int-2', payload: { response_payload: { answers: { tone: { answers: ['Friendly'] } } } } }, expect.anything())
})

test('approves only selected review findings', () => {
  vi.mocked(useConversationAIRunInteractions).mockReturnValue({ data: { run_id: 'run-1', interactions: [{ id: 'int-3', interaction_kind: 'review_checkpoint', status: 'pending', request_schema_version: 'helpin.v1', request_payload: { findings: [{ id: 'finding-1', title: 'Wrong total', body: 'Recalculate it' }] }, title: 'Review findings' }] } } as never)
  render(<AIRunApprovals workspaceId="ws-1" conversationId="conv-1" enabled />)
  expect((screen.getByRole('button', { name: 'Approve' }) as HTMLButtonElement).disabled).toBe(true)
  fireEvent.click(screen.getByRole('button', { name: /Wrong total/ }))
  fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
  expect(mutate).toHaveBeenCalledWith({ interactionId: 'int-3', payload: { response_payload: { decision: 'approve', selection_mode: 'selected', selected_finding_ids: ['finding-1'] } } }, expect.anything())
})
