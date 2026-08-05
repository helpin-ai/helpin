// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { SidebarVisitorContext } from '../SidebarVisitorContext'

const mockSupportData = vi.hoisted(() => ({
  visitorContext: {
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
      phone: null,
      job_title: 'VP Product',
      lifecycle_stage: 'customer',
      lead_status: 'open',
      source: 'live_chat',
      custom_properties: { seats_requested: 25 },
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

function renderSidebarContext() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <TooltipProvider>
        <SidebarVisitorContext workspaceId="ws-1" conversationId="conv-1" />
      </TooltipProvider>,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('SidebarVisitorContext', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('merges CRM contact and current visit signals into Contact Details', () => {
    const { container, cleanup } = renderSidebarContext()

    expect(container.textContent).not.toContain('User details')
    expect(container.textContent).toContain('Contact Details')
    expect(container.textContent).toContain('VP Product')
    expect(container.textContent).toContain('Current visit')
    expect(container.textContent).not.toContain('Main information')
    expect(container.textContent).not.toContain('Visitor device')
    expect(container.textContent).toContain('Chat')
    expect(container.textContent).toContain('example.com/pricing')
    expect(container.textContent).toContain('New York, New York, United States')
    expect(container.textContent).toContain('English (United States)')
    expect(container.textContent).toContain('Desktop')
    expect(container.textContent).toContain('Chrome 125')
    expect(container.textContent).toContain('macOS 14')
    expect(container.textContent).not.toContain('alex@example.com')

    cleanup()
  })
})
