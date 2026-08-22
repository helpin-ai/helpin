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
  initialContactValues,
} from '../conversation-contact-tools'

vi.mock('@helpin-ai/support-core', () => ({
  useUpdateConversationCRMContact: vi.fn(),
  useUpdateConversationEmailRecipients: vi.fn(),
}))

vi.mock('@mobile/lib/services/mobile-contacts-service', () => ({
  mobileContactsService: {
    search: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
  },
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

const CRM_CONTACT = {
  id: 'contact-1',
  workspace_id: 'ws-1',
  display_id: '1001',
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  phone: '+44 20 1234',
  job_title: 'Mathematician',
  lifecycle_stage: 'customer' as const,
  lead_status: 'open' as const,
  source: 'support',
  created_at: '2026-08-20T00:00:00.000Z',
  updated_at: '2026-08-20T00:00:00.000Z',
}

function setup(
  conversation: SupportConversation = CONVERSATION,
  permissions: { canEdit?: boolean; canReadCRM?: boolean; canEditCRM?: boolean } = {},
) {
  const updateRecipients = vi.fn()
  const updateContact = vi.fn()
  const updateContactAsync = vi.fn().mockResolvedValue(conversation)
  vi.mocked(useUpdateConversationEmailRecipients).mockReturnValue({
    mutate: updateRecipients,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationEmailRecipients>)
  vi.mocked(useUpdateConversationCRMContact).mockReturnValue({
    mutate: updateContact,
    mutateAsync: updateContactAsync,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationCRMContact>)
  vi.mocked(mobileContactsService.get).mockResolvedValue({
    data: CRM_CONTACT,
    error: null,
    status: 200,
  })
  vi.mocked(mobileContactsService.search).mockResolvedValue({
    data: {
      data: [{
        ...CRM_CONTACT,
        id: 'contact-2',
        first_name: 'Grace',
        last_name: 'Hopper',
        email: 'grace@example.com',
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
      <ConversationContactTools
        workspaceId="ws-1"
        conversation={conversation}
        canEdit={permissions.canEdit ?? true}
        canReadCRM={permissions.canReadCRM ?? true}
        canEditCRM={permissions.canEditCRM ?? true}
      />
    </QueryClientProvider>,
  )
  return { updateRecipients, updateContact, updateContactAsync }
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

  fireEvent.click(screen.getByRole('button', { name: 'Link existing contact' }))
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

test('prefills a support contact with the same defaults as the web profile', () => {
  expect(initialContactValues({ customer_name: 'Ada Byron Lovelace', customer_email: 'ada@example.com' })).toEqual({
    first_name: 'Ada',
    last_name: 'Byron Lovelace',
    email: 'ada@example.com',
    phone: '',
    job_title: '',
    lifecycle_stage: 'subscriber',
    lead_status: 'new',
    source: 'support',
  })
})

test('creates a prefilled CRM contact and links it after creation', async () => {
  const unlinked = { ...CONVERSATION, crm_contact_id: undefined }
  vi.mocked(mobileContactsService.create).mockResolvedValue({ data: CRM_CONTACT, error: null, status: 201 })
  const { updateContactAsync } = setup(unlinked)

  fireEvent.click(screen.getByRole('button', { name: 'Create and link contact' }))
  expect((screen.getByRole('textbox', { name: 'First name' }) as HTMLInputElement).value).toBe('Ada')
  expect((screen.getByRole('textbox', { name: 'Last name' }) as HTMLInputElement).value).toBe('Lovelace')
  expect((screen.getByRole('textbox', { name: 'Email' }) as HTMLInputElement).value).toBe('ada@example.com')
  fireEvent.change(screen.getByRole('textbox', { name: 'Phone' }), { target: { value: '+44 20 1234' } })
  fireEvent.click(screen.getByRole('button', { name: 'Create and link' }))

  await waitFor(() => {
    expect(mobileContactsService.create).toHaveBeenCalledWith({
      workspace_id: 'ws-1',
      first_name: 'Ada',
      last_name: 'Lovelace',
      email: 'ada@example.com',
      phone: '+44 20 1234',
      job_title: undefined,
      lifecycle_stage: 'subscriber',
      lead_status: 'new',
      source: 'support',
    })
    expect(updateContactAsync).toHaveBeenCalledWith({ conversationId: 'conv-1', contactId: 'contact-1' })
  })
})

test('loads and edits the canonical linked CRM profile', async () => {
  vi.mocked(mobileContactsService.update).mockResolvedValue({ data: CRM_CONTACT, error: null, status: 200 })
  setup()

  expect(await screen.findByText('Mathematician')).toBeDefined()
  fireEvent.click(screen.getByRole('button', { name: 'Edit CRM contact' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Job title' }), { target: { value: 'Analytical Engineer' } })
  fireEvent.change(screen.getByRole('textbox', { name: 'Email' }), { target: { value: '' } })
  fireEvent.change(screen.getByLabelText('Lifecycle stage'), { target: { value: 'evangelist' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save contact' }))

  await waitFor(() => {
    expect(mobileContactsService.update).toHaveBeenCalledWith(
      'ws-1',
      'contact-1',
      expect.objectContaining({
        first_name: 'Ada',
        email: undefined,
        job_title: 'Analytical Engineer',
        lifecycle_stage: 'evangelist',
        lead_status: 'open',
      }),
    )
  })
})

test('keeps CRM data and edit controls behind their matching permissions', async () => {
  setup(CONVERSATION, { canReadCRM: false, canEditCRM: false })
  expect(screen.queryByText('CRM contact')).toBeNull()
  expect(mobileContactsService.get).not.toHaveBeenCalled()
})

test('shows the upgrade sheet when the CRM contact limit blocks creation', async () => {
  vi.mocked(mobileContactsService.create).mockResolvedValue({
    data: null,
    error: 'Starter includes up to 5,000 contacts',
    status: 402,
  })
  setup({ ...CONVERSATION, crm_contact_id: undefined })

  fireEvent.click(screen.getByRole('button', { name: 'Create and link contact' }))
  fireEvent.click(screen.getByRole('button', { name: 'Create and link' }))

  expect(await screen.findByText('The Starter plan includes up to 5,000 CRM contacts.')).toBeDefined()
})
