import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  useAgentRunMessages,
  useApproveAgentRun,
  useConversationAgentRuns,
  type SupportAgentRun,
} from '@helpin-ai/support-core'
import { AssignedAgentRuns } from '../assigned-agent-runs'

vi.mock('@helpin-ai/support-core', () => ({
  useConversationAgentRuns: vi.fn(),
  useAgentRunMessages: vi.fn(),
  useApproveAgentRun: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const pendingRun = {
  id: 'run-1', workspace_id: 'ws-1', agent_id: 'agent-1', target_type: 'support_conversation', target_id: 'conv-1',
  runtime_kind: 'native_sdk', invocation_mode: 'interactive', approval_state: 'pending', pause_reason: 'human_approval',
  status: 'paused', input: {}, output_summary: { summary: 'Refund draft is ready.' }, tokens_used: 120,
  created_at: '2026-08-20T10:00:00Z', updated_at: '2026-08-20T10:05:00Z', started_at: '2026-08-20T10:01:00Z',
} satisfies SupportAgentRun

const mutateAsync = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useConversationAgentRuns).mockReturnValue({ data: [pendingRun] } as never)
  vi.mocked(useAgentRunMessages).mockReturnValue({ data: [{ id: 'msg-1', workspace_id: 'ws-1', run_id: 'run-1', role: 'assistant', content: 'I prepared the response.', message_type: 'message', sequence_no: 1, created_at: '2026-08-20T10:02:00Z' }] } as never)
  vi.mocked(useApproveAgentRun).mockReturnValue({ mutateAsync, isPending: false } as never)
  mutateAsync.mockResolvedValue(pendingRun)
})

test('shows run history and approves the pending draft', async () => {
  render(<AssignedAgentRuns workspaceId="ws-1" conversationId="conv-1" enabled canApprove />)
  expect(screen.getByText('Assigned agent activity')).toBeTruthy()
  expect(screen.getByText('Draft waiting for approval')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Approve draft' }))
  await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith('run-1'))
})

test('opens a mobile-readable detail sheet with status history and messages', () => {
  render(<AssignedAgentRuns workspaceId="ws-1" conversationId="conv-1" enabled canApprove />)
  fireEvent.click(screen.getByRole('button', { name: 'Open Paused agent run details' }))
  expect(screen.getByText('Status history')).toBeTruthy()
  expect(screen.getByText('Refund draft is ready.')).toBeTruthy()
  expect(screen.getByText('I prepared the response.')).toBeTruthy()
  expect(useAgentRunMessages).toHaveBeenLastCalledWith('ws-1', 'run-1', true)
})

test('hides approval when support editing is unavailable', () => {
  render(<AssignedAgentRuns workspaceId="ws-1" conversationId="conv-1" enabled canApprove={false} />)
  expect(screen.queryByRole('button', { name: 'Approve draft' })).toBeNull()
})
