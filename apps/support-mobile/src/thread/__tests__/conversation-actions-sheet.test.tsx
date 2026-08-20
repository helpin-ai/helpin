import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  useCreateTaskFromConversation,
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
import { openTaskInWeb } from '@mobile/lib/task-links'
import { workspacesService } from '@mobile/lib/services/workspaces-service'

vi.mock('@helpin-ai/support-core', () => ({
  useDeleteConversation: vi.fn(),
  useCreateTaskFromConversation: vi.fn(),
  useInboxScopes: vi.fn(),
  useMarkConversationUnread: vi.fn(),
  useMoveConversation: vi.fn(),
  useSendConversationTranscript: vi.fn(),
  useUpdateConversationStatus: vi.fn(),
  useUpdateConversationSubject: vi.fn(),
}))

vi.mock('@mobile/lib/task-links', () => ({ openTaskInWeb: vi.fn(async () => undefined) }))
vi.mock('@mobile/lib/services/workspaces-service', () => ({
  workspacesService: {
    getSettings: vi.fn(),
    updateSupportTaskPreferences: vi.fn(),
  },
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() } }))

const CONVERSATION: SupportConversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 42, subject: 'Billing help',
  status: 'open', priority: 'medium', customer_name: 'Ada', customer_email: 'ada@example.com',
  email_cc: ['finance@example.com'], source: 'email',
  created_at: '2026-08-20T00:00:00.000Z', updated_at: '2026-08-20T00:00:00.000Z',
}

const mockUseSendConversationTranscript = vi.mocked(useSendConversationTranscript)

function setup() {
  const transcript = vi.fn()
  const createTask = vi.fn(async () => ({
    task_id: 'task-1', task_key: 'ENG-1', task_name: 'Fix billing', summary: 'Billing failed',
  }))
  vi.mocked(useCreateTaskFromConversation).mockReturnValue({ mutateAsync: createTask, isPending: false } as unknown as ReturnType<typeof useCreateTaskFromConversation>)
  vi.mocked(useDeleteConversation).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useDeleteConversation>)
  vi.mocked(useInboxScopes).mockReturnValue({ data: { mailboxes: [] }, isPending: false } as unknown as ReturnType<typeof useInboxScopes>)
  vi.mocked(useMarkConversationUnread).mockReturnValue({ mutate: vi.fn() } as unknown as ReturnType<typeof useMarkConversationUnread>)
  vi.mocked(useMoveConversation).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useMoveConversation>)
  mockUseSendConversationTranscript.mockReturnValue({ mutate: transcript, isPending: false } as unknown as ReturnType<typeof useSendConversationTranscript>)
  vi.mocked(useUpdateConversationStatus).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useUpdateConversationStatus>)
  vi.mocked(useUpdateConversationSubject).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<typeof useUpdateConversationSubject>)
  vi.mocked(workspacesService.getSettings).mockResolvedValue({ data: { teams: [] }, error: null })
  vi.mocked(workspacesService.updateSupportTaskPreferences).mockResolvedValue({ data: { message: 'ok' }, error: null })
  return { transcript, createTask }
}

function renderSheet(props: Partial<React.ComponentProps<typeof ConversationActionsSheet>> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <ConversationActionsSheet
        open
        onOpenChange={vi.fn()}
        workspaceId="ws-1"
        workspaceSlug="acme"
        conversation={CONVERSATION}
        onLeave={vi.fn()}
        {...props}
      />
    </QueryClientProvider>,
  )
}

beforeEach(() => vi.clearAllMocks())

test('copies the current conversation URL', async () => {
  setup()
  const writeText = vi.fn(async () => undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  renderSheet()

  fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))

  await waitFor(() => expect(writeText).toHaveBeenCalledWith(window.location.href))
})

test('sends a transcript to a selected recent recipient', () => {
  const { transcript } = setup()
  renderSheet()

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
  renderSheet({ conversation: { ...CONVERSATION, customer_email: undefined, email_cc: [] } })

  fireEvent.click(screen.getByRole('button', { name: 'Email transcript' }))
  fireEvent.change(screen.getByPlaceholderText('recipient@example.com'), { target: { value: 'new@example.com' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send transcript' }))

  expect(transcript).toHaveBeenCalledWith(
    { conversationId: 'conv-1', email: 'new@example.com', updateCustomerEmail: true },
    expect.anything(),
  )
})

test('opens an existing linked task in the web app', async () => {
  setup()
  renderSheet({ conversation: { ...CONVERSATION, linked_task_id: 'task-42' }, canReadPM: true })

  fireEvent.click(screen.getByRole('button', { name: 'Open linked task' }))
  await waitFor(() => expect(openTaskInWeb).toHaveBeenCalledWith('acme', 'task-42'))
})

test('creates a linked task for the selected team and opens it', async () => {
  const { createTask } = setup()
  vi.mocked(workspacesService.getSettings).mockResolvedValue({
    data: { teams: [{ id: 'team-1', workspace_id: 'ws-1', name: 'Engineering' }] },
    error: null,
  })
  renderSheet({ canCreateTask: true, defaultTeamId: 'team-1' })

  fireEvent.click(screen.getByRole('button', { name: 'Create task' }))
  await screen.findByRole('button', { name: 'Assign task to Engineering' })
  fireEvent.click(screen.getByRole('button', { name: 'Create and open task' }))

  await waitFor(() => expect(createTask).toHaveBeenCalledWith({ conversationId: 'conv-1', teamId: 'team-1' }))
  await waitFor(() => expect(openTaskInWeb).toHaveBeenCalledWith('acme', 'task-1'))
})
