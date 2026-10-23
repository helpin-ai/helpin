// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { SidebarSessionDetails, SidebarVisitorContext } from '../SidebarVisitorContext'

const mockSupportData = vi.hoisted(() => ({
  visitorContext: {
    channels: ['widget', 'email'] as string[] | undefined,
    device: {
      browser: 'Chrome',
      browser_version: '125',
      os: 'macOS',
      os_version: '14',
      device_type: 'desktop',
    },
    location: {
      timezone: 'America/New_York',
      locale: 'en-US',
      last_page_url: 'https://example.com/pricing',
      country_code: 'US',
      country_name: 'United States',
      region_name: 'New York',
      city_name: 'New York',
    },
    contact: {
      id: 'contact-1',
      name: 'Alex Buyer',
      email: 'alex@example.com',
      phone: null as string | null,
      job_title: 'VP Product' as string | null,
      lifecycle_stage: 'customer',
      lead_status: 'open',
      source: 'live_chat',
      custom_properties: {
        seats_requested: 25,
        agent_enrichment: {
          linkedin_url: {
            value: 'https://linkedin.com/in/alex-buyer',
            source_url: 'https://example.com/internal-source',
            confidence: 0.95,
          },
        },
      } as Record<string, unknown>,
    },
    other_conversations: [],
    total_conversations: 1,
    session_created_at: '2026-06-01T12:00:00Z',
  },
  conversation: {
    id: 'conv-1',
    source: 'widget',
    customer_email: 'alex@example.com',
  },
}))

vi.mock('@/hooks/queries/useSupport', () => ({
  useVisitorContext: () => ({ data: mockSupportData.visitorContext, isLoading: false }),
  useConversation: () => ({ data: mockSupportData.conversation }),
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(<TooltipProvider>{node}</TooltipProvider>)
  })

  return {
    container,
    expand: () => act(() => (container.querySelector('[role="button"]') as HTMLElement).click()),
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('SidebarVisitorContext', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    window.localStorage.clear()
    mockSupportData.visitorContext.contact.job_title = 'VP Product'
    mockSupportData.visitorContext.contact.custom_properties = { seats_requested: 25 }
  })

  it('starts collapsed and shows only the contact’s own CRM fields', () => {
    const { container, expand, cleanup } = render(<SidebarVisitorContext workspaceId="ws-1" conversationId="conv-1" />)

    expect(container.textContent).toContain('Contact details')
    expect(container.textContent).not.toContain('VP Product')
    expand()

    expect(container.textContent).toContain('VP Product')
    expect(container.textContent).toContain('seats requested')
    expect(container.textContent).toContain('25')
    expect(container.textContent).not.toContain('agent enrichment')
    expect(container.textContent).not.toContain('internal-source')
    expect(container.textContent).not.toContain('Lifecycle')
    expect(container.textContent).not.toContain('Lead status')
    expect(container.querySelector('[aria-label="Channels"]')).toBeNull()
    expect(container.textContent).not.toContain('Current visit')
    expect(container.textContent).not.toContain('example.com/pricing')

    cleanup()
  })

  it('remembers an expanded Contact details section for the next conversation', () => {
    const first = render(<SidebarVisitorContext workspaceId="ws-1" conversationId="conv-1" />)
    first.expand()
    first.cleanup()

    const second = render(<SidebarVisitorContext workspaceId="ws-1" conversationId="conv-2" />)
    expect(second.container.textContent).toContain('VP Product')
    second.cleanup()
  })

  it('hides Contact details when the contact has no fields of their own', () => {
    mockSupportData.visitorContext.contact.job_title = null
    mockSupportData.visitorContext.contact.custom_properties = {}
    const { container, cleanup } = render(<SidebarVisitorContext workspaceId="ws-1" conversationId="conv-1" />)

    expect(container.textContent).toBe('')

    cleanup()
  })
})

describe('SidebarSessionDetails', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    window.localStorage.clear()
  })

  it('shows the latest browsing session in its own collapsed section', () => {
    const { container, expand, cleanup } = render(<SidebarSessionDetails workspaceId="ws-1" conversationId="conv-1" />)

    expect(container.textContent).toContain('Session details')
    expect(container.textContent).not.toContain('example.com/pricing')
    expand()

    expect(container.textContent).toContain('example.com/pricing')
    expect(container.textContent).toContain('New York, New York, United States')
    expect(container.textContent).toContain('English (United States)')
    expect(container.textContent).toContain('Desktop')
    expect(container.textContent).toContain('Chrome 125')
    expect(container.textContent).toContain('macOS 14')
    expect(container.textContent).not.toContain('VP Product')

    cleanup()
  })
})
