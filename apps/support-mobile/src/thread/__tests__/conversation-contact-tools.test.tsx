import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  useUpdateConversationCRMContact,
  useUpdateConversationEmailRecipients,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { mobileContactsService } from '@mobile/lib/services/mobile-contacts-service'
import {
  addCCRecipient,
  ConversationContactTools,
  conversationRecipients,
} from '../conversation-contact-tools'

vi.mock('@helpin-ai/support-core', () => ({
  useUpdateConversationCRMContact: vi.fn(),
  useUpdateConversationEmailRecipients: vi.fn(),
}))

vi.mock('@mobile/lib/services/mobile-contacts-service', () => ({
  mobileContactsService: { search: vi.fn() },
}))

vi.mock('@mobile/lib/haptics', () => ({ haptic: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

const CONVERSATION: SupportConversation = {
  id: 'conv-1',
  workspace_id: 'ws-1',
  display_id: 42,
  subject: 'Billing',
  status: 'open',
  priority: 'medium',
  customer_name: 'Ada Lovelace',
  customer_email: 'ada@example.com',
  email_cc: ['finance@example.com'],
  email_thread_participants: ['finance@example.com', 'ops@example.com'],
  crm_contact_id: 'contact-1',
  source: 'email',
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}

function setup(conversation: SupportConversation = CONVERSATION) {
  const updateRecipients = vi.fn()
  const updateContact = vi.fn()
  vi.mocked(useUpdateConversationEmailRecipients).mockReturnValue({
    mutate: updateRecipients,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationEmailRecipients>)
  vi.mocked(useUpdateConversationCRMContact).mockReturnValue({
    mutate: updateContact,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationCRMContact>)
  vi.mocked(mobileContactsService.search).mockResolvedValue({
    data: {
      data: [{
        id: 'contact-2',
        first_name: 'Grace',
        last_name: 'Hopper',
        email: 'grace@example.com',
        lifecycle_stage: 'customer',
      }],
      total: 1,
      page: 1,
    },
    error: null,
    status: 200,
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <ConversationContactTools workspaceId="ws-1" conversation={conversation} canEdit />
    </QueryClientProvider>,
  )
  return { updateRecipients, updateContact }
}

beforeEach(() => vi.clearAllMocks())

test('shows primary, Cc, and other thread recipients without duplicates', () => {
  setup()

  expect(screen.getByText('ada@example.com')).toBeDefined()
  expect(screen.getByText('finance@example.com')).toBeDefined()
  expect(screen.getByText('ops@example.com')).toBeDefined()
  expect(screen.getAllByText('finance@example.com')).toHaveLength(1)
})

test('adds and removes Cc recipients through the exact payload shape', () => {
  const { updateRecipients } = setup()

  fireEvent.click(screen.getByRole('button', { name: 'Add Cc recipient' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Cc email address' }), {
    target: { value: 'billing@example.com' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save Cc recipient' }))

  expect(updateRecipients).toHaveBeenCalledWith(
    {
      conversationId: 'conv-1',
      payload: { cc_emails: ['finance@example.com', 'billing@example.com'] },
    },
    expect.anything(),
  )

  fireEvent.click(screen.getByRole('button', {
    name: 'Remove Cc recipient finance@example.com',
  }))
  expect(updateRecipients).toHaveBeenLastCalledWith(
    { conversationId: 'conv-1', payload: { cc_emails: [] } },
    expect.anything(),
  )
})

test('validates duplicate primary and Cc recipients case-insensitively', () => {
  const recipients = conversationRecipients(CONVERSATION)

  expect(addCCRecipient(recipients, 'ADA@example.com').error).toBe('This is already the primary recipient')
  expect(addCCRecipient(recipients, 'Finance@Example.com').error).toBe('This recipient is already in Cc')
  expect(addCCRecipient(recipients, 'invalid').error).toBe('Enter a valid email address')
})

test('searches and links a different CRM contact', async () => {
  const { updateContact } = setup({ ...CONVERSATION, crm_contact_id: undefined })

  fireEvent.click(screen.getByRole('button', { name: 'Link CRM contact' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Search CRM contacts' }), {
    target: { value: 'grace' },
  })
  const result = await screen.findByRole('button', { name: 'Link CRM contact Grace Hopper' })
  fireEvent.click(result)

  expect(mobileContactsService.search).toHaveBeenCalledWith('ws-1', 'grace')
  expect(updateContact).toHaveBeenCalledWith(
    { conversationId: 'conv-1', contactId: 'contact-2' },
    expect.anything(),
  )
})

test('can unlink the current CRM contact', async () => {
  const { updateContact } = setup()

  fireEvent.click(screen.getByRole('button', { name: 'Unlink CRM contact' }))
  await waitFor(() => {
    expect(updateContact).toHaveBeenCalledWith(
      { conversationId: 'conv-1', contactId: null },
      expect.anything(),
    )
  })
})
