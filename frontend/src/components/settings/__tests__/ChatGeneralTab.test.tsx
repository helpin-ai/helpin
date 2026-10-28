// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'

const save = vi.fn().mockResolvedValue({})
let chatSettings: { data: unknown; isLoading: boolean; refetch: () => void }

vi.mock('@/hooks/queries', () => ({
  useChatSettings: () => chatSettings,
  useUpdateChatSettings: () => ({ mutateAsync: save, isPending: false }),
  useRegenerateWidgetKey: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
  useDocsSpaces: () => ({ data: [], isLoading: false }),
}))
vi.mock('@/hooks/queries/useSupport', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useSupportAgents: () => ({ data: [] }),
  useSupportMailboxes: () => ({ data: [] }),
  // The allowed-websites editor saves on its own; it is not the form autosave.
  useUpdateChatSettings: () => ({ mutateAsync: vi.fn(), mutate: vi.fn(), isPending: false }),
}))
vi.mock('@/hooks/useWorkspaceTeams', () => ({ useWorkspaceTeams: () => ({ teams: [] }) }))
vi.mock('@edition', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useWorkspaceBilling: () => ({ data: undefined, isLoading: false }),
}))
vi.mock('../WidgetPreview', () => ({ WidgetPreview: () => null }))
vi.mock('../SupportAIPreview', () => ({ SupportAIPreview: () => null }))
vi.mock('../SettingsAutosaveGuard', () => ({ SettingsAutosaveGuard: () => null }))

import { ChatGeneralTab } from '../ChatGeneralTab'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function savedSettings() {
  return {
    email_capture_timing: 'after_ai_reply',
    require_email_before_chat: false,
    require_phone_after_email: false,
    welcome_message: 'Hi there, how can we help?',
    brand_color: '#123456',
    show_branding: true,
    launcher_position: 'bottom_right',
    launcher_icon: 'chat_bubble',
    color_scheme: 'light',
    ai_enabled: false,
    ai_agent_id: null,
    ai_confidence_threshold: 0.6,
    ai_response_mode: 'ai_first',
    ai_reply_channels: 'both',
    ai_email_code_check_enabled: true,
    ai_max_followups: 3,
    show_talk_to_human: true,
    escalation_message: 'Let me connect you with a team member.',
    email_handoff_message: 'Our team will reply here.',
    email_handoff_message_after_hours: 'Our team will reply when we reopen.',
    handoff_behavior: 'unassigned',
    handoff_team_id: null,
    ai_handoff_mailbox_id: null,
    business_hours_enabled: false,
    business_hours_timezone: 'UTC',
    business_hours_schedule: {},
    outside_hours_message: 'We are away.',
    csat_enabled: false,
    widget_help_space_ids: [],
  }
}

describe('ChatGeneralTab', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('ResizeObserver', class {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    HTMLElement.prototype.scrollIntoView = vi.fn()
    save.mockClear()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    chatSettings = { data: { settings: savedSettings(), allowed_origins: [], widget_key: 'key' }, isLoading: false, refetch: vi.fn() }
  })
  afterEach(() => {
    act(() => root.unmount())
    container.remove()
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  async function renderTab() {
    await act(async () => {
      root.render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><ChatGeneralTab workspaceId="ws-1" /></TooltipProvider></QueryClientProvider>)
    })
  }

  it('loads saved settings into the form without saving them back', async () => {
    await renderTab()
    expect(container.innerHTML).toContain('Hi there, how can we help?')
    // A widget with no allowed websites opens that section.
    const origins = [...container.querySelectorAll('button[aria-expanded]')].find((button) => button.textContent?.includes('Allowed websites'))
    expect(origins?.getAttribute('aria-expanded')).toBe('true')
    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(save).not.toHaveBeenCalled()
  })

  it('saves once after an edit', async () => {
    await renderTab()
    const csat = container.querySelector<HTMLButtonElement>('#chat-csat-enabled')
    expect(csat).not.toBeNull()
    await act(async () => { csat!.click() })
    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(save).toHaveBeenCalledTimes(1)
    expect(save.mock.calls[0][0]).toMatchObject({
      csat_enabled: true,
      welcome_message: 'Hi there, how can we help?',
      ai_email_code_check_enabled: true,
      email_capture_timing: 'after_ai_reply',
      email_handoff_message: 'Our team will reply here.',
      email_handoff_message_after_hours: 'Our team will reply when we reopen.',
    })
  })

  it('searches and autosaves a timezone outside the old support shortlist', async () => {
    chatSettings.data = {
      settings: { ...savedSettings(), business_hours_enabled: true },
      allowed_origins: [],
      widget_key: 'key',
    }
    await renderTab()
    const availability = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('Set when your team appears online'))!
    await act(async () => { availability.click() })
    const timezone = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.startsWith('UTC'))!
    expect(timezone).toBeDefined()
    await act(async () => {
      timezone.click()
      timezone.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    })
    const search = document.querySelector<HTMLInputElement>('input[placeholder="Search timezones..."]')
    expect(search).not.toBeNull()
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(search, 'Karachi')
      search!.dispatchEvent(new Event('input', { bubbles: true }))
    })
    const option = [...document.querySelectorAll<HTMLElement>('[role="option"]')].find((item) => item.textContent?.includes('Asia/Karachi'))
    expect(option).toBeDefined()
    await act(async () => { option!.click() })
    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(save).toHaveBeenCalledTimes(1)
    expect(save.mock.calls[0][0]).toMatchObject({
      business_hours_timezone: 'Asia/Karachi',
      business_hours_enabled: true,
      outside_hours_message: 'We are away.',
    })
  })
})
