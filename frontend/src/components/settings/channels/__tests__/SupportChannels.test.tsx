// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportChannelAccount } from '@/lib/pmTypes'

const connect = vi.fn()
const update = vi.fn()
const reconnect = vi.fn()
const disconnect = vi.fn()
let accounts: SupportChannelAccount[] = []

vi.mock('@/hooks/queries/useSupport', () => ({
  useInboxScopes: () => ({ data: { mailboxes: [{ id: 'mb-1', name: 'Billing' }] } }),
}))
vi.mock('@/hooks/queries/useSupportChannels', () => ({
  useSupportChannels: () => ({ data: accounts, isLoading: false, error: null }),
  useConnectChannel: () => ({ mutateAsync: connect, isPending: false }),
  useUpdateChannel: () => ({ mutate: update, isPending: false }),
  useReconnectChannel: () => ({ mutate: reconnect, mutateAsync: reconnect, isPending: false }),
  useDisconnectChannel: () => ({ mutate: disconnect, isPending: false }),
}))
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => async () => true }))

import { SupportChannels } from '../SupportChannels'

Element.prototype.scrollIntoView = () => {}
Object.assign(globalThis, {
  IS_REACT_ACT_ENVIRONMENT: true,
  ResizeObserver: class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
})

function account(overrides: Partial<SupportChannelAccount> = {}): SupportChannelAccount {
  return {
    id: 'acct-1', workspace_id: 'ws', provider: 'telegram', external_account_id: '424242', display_name: 'Acme Support', handle: 'acme_support_bot',
    mailbox_id: null, ai_replies_disabled: false, status: 'active', last_error_code: null, last_error_message: null, last_event_at: null,
    created_at: '2026-11-13T00:00:00Z', failed_deliveries: 0, unconfirmed_deliveries: 0, ...overrides,
  }
}

function setInput(element: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

function button(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((element) => element.textContent?.trim() === text || element.getAttribute('aria-label') === text) as HTMLButtonElement | undefined
}

describe('SupportChannels', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.clearAllMocks()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    document.body.innerHTML = ''
  })

  const render = (editable = true) => act(() => root.render(<TooltipProvider><SupportChannels workspaceId="ws" editable={editable} /></TooltipProvider>))

  it('explains how to start when nothing is connected', () => {
    accounts = []
    render()
    expect(container.textContent).toContain('No Telegram bots connected')
    expect(container.textContent).toContain('@BotFather')
    render(false)
    expect(button('Connect Telegram bot')).toBeUndefined()
  })

  it('shows each bot with its inbox, health and AI switch', () => {
    accounts = [
      account({ mailbox_id: 'mb-1', last_event_at: new Date(Date.now() - 5 * 60_000).toISOString() }),
      account({ id: 'acct-2', handle: 'second_bot', display_name: '', failed_deliveries: 2, ai_replies_disabled: true }),
      account({ id: 'acct-3', handle: 'revoked_bot', status: 'reauthorization_required' }),
    ]
    render()
    const rows = container.querySelectorAll('li')
    expect(rows).toHaveLength(3)
    expect(rows[0].textContent).toContain('@acme_support_bot')
    expect(rows[0].textContent).toContain('Billing')
    expect(rows[0].textContent).toContain('Last message 5 minutes ago')
    expect(rows[1].textContent).toContain('Main inbox')
    expect(rows[1].textContent).toContain('2 replies were not delivered')
    expect(rows[2].textContent).toContain('Reconnect with a new token')

    const aiSwitch = container.querySelector('[aria-label="AI replies for @second_bot"]') as HTMLButtonElement
    expect(aiSwitch.getAttribute('aria-checked')).toBe('false')
    act(() => aiSwitch.click())
    expect(update).toHaveBeenCalledWith({ id: 'acct-2', input: { ai_replies_disabled: false } })
  })

  it('validates the token before connecting', async () => {
    accounts = []
    connect.mockResolvedValue(account())
    render()
    act(() => button('Connect Telegram bot')?.click())
    const input = document.getElementById('channel-token') as HTMLInputElement
    act(() => setInput(input, '@acme_support_bot'))
    await act(async () => button('Connect bot')?.click())
    expect(document.getElementById('channel-token-error')?.textContent).toContain('full token')
    expect(connect).not.toHaveBeenCalled()

    act(() => setInput(input, ' 123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw '))
    await act(async () => button('Connect bot')?.click())
    expect(connect).toHaveBeenCalledWith({ token: '123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw', mailboxId: null })
  })

  it('locks controls for teammates who cannot manage channels', () => {
    accounts = [account()]
    render(false)
    expect((container.querySelector('[aria-label="AI replies for @acme_support_bot"]') as HTMLButtonElement).disabled).toBe(true)
    expect(button('Actions for @acme_support_bot')).toBeUndefined()
  })
})
