import { fireEvent, render, screen } from '@testing-library/react'
import {
  useUpdateConversationCRMCompany,
  useVisitorContext,
  type VisitorContextResponse,
} from '@helpin-ai/support-core'
import {
  CustomerContext,
  formatContextValue,
  formatLocale,
  formatLocalTime,
} from '../customer-context'

vi.mock('@helpin-ai/support-core', () => ({
  useVisitorContext: vi.fn(),
  useUpdateConversationCRMCompany: vi.fn(),
}))

vi.mock('@mobile/lib/haptics', () => ({
  haptic: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn() },
}))

const mockUseVisitorContext = vi.mocked(useVisitorContext)
const mockUseUpdateConversationCRMCompany = vi.mocked(useUpdateConversationCRMCompany)

const VISITOR_CONTEXT: VisitorContextResponse = {
  contact: {
    id: 'contact-1',
    name: 'Ada Lovelace',
    email: 'ada@example.com',
    phone: '+44 20 0000 0000',
    job_title: 'Founder',
    lifecycle_stage: 'customer',
    lead_status: 'connected',
    source: 'support',
    custom_properties: {
      preferred_channel: 'Email',
      beta_customer: true,
      agent_enrichment: { internal: true },
    },
  },
  device: {
    browser: 'Safari',
    browser_version: '18',
    os: 'iOS',
    os_version: '19',
    device_type: 'mobile',
  },
  location: {
    timezone: 'Europe/London',
    locale: 'en-GB',
    last_page_url: 'https://example.com/pricing',
    country_code: 'GB',
    country_name: 'United Kingdom',
    region_name: 'England',
    city_name: 'London',
  },
  company: {
    id: 'company-1',
    display_id: 'ACME',
    name: 'Acme Ltd',
    domain: 'acme.example',
    industry: 'Software',
    employee_count: 42,
    custom_properties: {
      plan: 'pro',
      mrr: 1200,
      currency: 'USD',
      support_tier: 'priority',
      agent_enrichment: 'hidden',
    },
    updated_at: '2026-08-20T00:00:00.000Z',
  },
  company_options: [
    { id: 'company-1', display_id: 'ACME', name: 'Acme Ltd', domain: 'acme.example' },
    { id: 'company-2', display_id: 'BETA', name: 'Beta Inc', domain: 'beta.example' },
  ],
  company_context_status: 'ok',
  other_conversations: [
    {
      id: 'conv-2',
      display_id: 43,
      subject: 'Previous billing question',
      status: 'resolved',
      created_at: '2026-08-01T00:00:00.000Z',
    },
  ],
  total_conversations: 2,
  session_created_at: '2026-08-01T00:00:00.000Z',
}

function setup({ canEdit = true }: { canEdit?: boolean } = {}) {
  const companyMutate = vi.fn()
  const openConversation = vi.fn()
  mockUseVisitorContext.mockReturnValue({
    data: VISITOR_CONTEXT,
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useVisitorContext>)
  mockUseUpdateConversationCRMCompany.mockReturnValue({
    mutate: companyMutate,
    isPending: false,
  } as unknown as ReturnType<typeof useUpdateConversationCRMCompany>)

  render(
    <CustomerContext
      workspaceId="ws-1"
      conversationId="conv-1"
      canEdit={canEdit}
      channel="widget"
      onOpenConversation={openConversation}
    />,
  )

  return { companyMutate, openConversation }
}

beforeEach(() => {
  vi.clearAllMocks()
})

test('renders CRM, visit, company, and conversation context while hiding internal enrichment', () => {
  setup()

  expect(screen.getByText('Contact details')).toBeDefined()
  expect(screen.getByText('Founder')).toBeDefined()
  expect(screen.getByText('Preferred Channel')).toBeDefined()
  expect(screen.getByText('example.com/pricing')).toBeDefined()
  expect(screen.getByText('Safari 18 · iOS 19')).toBeDefined()
  expect(screen.getByText('Company details')).toBeDefined()
  expect(screen.getAllByText('Acme Ltd')).toHaveLength(2)
  expect(screen.getByText('Support Tier')).toBeDefined()
  expect(screen.getByText('Other conversations (1)')).toBeDefined()
  expect(screen.queryByText('Agent Enrichment')).toBeNull()
})

test('changes the linked company and opens another conversation', () => {
  const { companyMutate, openConversation } = setup()

  fireEvent.change(screen.getByRole('combobox', { name: 'Conversation company' }), {
    target: { value: 'company-2' },
  })
  fireEvent.click(screen.getByRole('button', {
    name: 'Open conversation 43: Previous billing question',
  }))

  expect(companyMutate).toHaveBeenCalledWith(
    { conversationId: 'conv-1', companyId: 'company-2' },
    expect.anything(),
  )
  expect(openConversation).toHaveBeenCalledWith('conv-2')
})

test('read-only context does not expose company editing controls', () => {
  setup({ canEdit: false })

  expect(screen.queryByRole('combobox', { name: 'Conversation company' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Clear company' })).toBeNull()
})

test('context formatters handle structured values, locales, and invalid timezones', () => {
  expect(formatContextValue(true)).toBe('Yes')
  expect(formatContextValue({ tier: 'pro' })).toBe('{"tier":"pro"}')
  expect(formatLocale('en-GB')).toMatch(/English/)
  expect(formatLocalTime('Not/A_Timezone')).toBeNull()
})
