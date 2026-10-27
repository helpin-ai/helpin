// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  ask: vi.fn(),
  session: vi.fn(),
  feedback: vi.fn(),
  navigate: vi.fn(),
}))

vi.mock('@/lib/services/customerPortalService', () => ({
  customerPortalService: { assistantAsk: mocks.ask, assistantSession: mocks.session, assistantFeedback: mocks.feedback },
}))
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mocks.navigate,
  Link: ({ children, to, params, className }: { children: ReactNode; to: string; params: Record<string, string>; className?: string }) => (
    <a href={Object.entries(params).reduce((path, [key, value]) => path.replace(`$${key}`, value), to)} className={className}>{children}</a>
  ),
}))

import { PortalAskAI } from '../PortalAssistant'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const question = { id: 'q1', role: 'user', content: 'How is renewal pricing set?', status: 'answered', citations: [], created_at: '2026-10-27T00:00:00Z' }

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  vi.useFakeTimers()
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: () => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() }) })
  window.sessionStorage.clear()
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  mocks.ask.mockResolvedValue({ data: { session_id: 's1', question_id: 'q1', answer_id: 'a1', turns_left: 19 }, error: null, status: 200 })
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.useRealTimers()
  vi.resetAllMocks()
})

async function flush() {
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() })
}

async function askQuestion(text: string) {
  await act(async () => root.render(<PortalAskAI slug="acme" />))
  const open = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Ask AI'))!
  await act(async () => open.click())
  const input = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Ask AI a question"]')!
  const setValue = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
  await act(async () => {
    setValue.call(input, text)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await act(async () => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
  await flush()
  await act(async () => { vi.advanceTimersByTime(1000) })
  await flush()
}

describe('PortalAskAI', () => {
  it('answers with a source for signed-in customers that opens in the portal', async () => {
    mocks.session.mockResolvedValue({ data: { id: 's1', turns_left: 19, messages: [question, { id: 'a1', role: 'assistant', content: 'Renewal pricing is **per contract**.', status: 'answered', created_at: '2026-10-27T00:00:00Z', citations: [{ article_id: 'd1', title: 'Renewal pricing', portal_article_id: 'hc_renewal' }] }] }, error: null, status: 200 })
    await askQuestion('How is renewal pricing set?')
    expect(mocks.ask).toHaveBeenCalledWith('acme', expect.objectContaining({ message: 'How is renewal pricing set?' }))
    expect(container.querySelector('strong')?.textContent).toBe('per contract')
    const source = Array.from(container.querySelectorAll('a')).find((link) => link.textContent?.includes('Renewal pricing'))
    expect(source?.getAttribute('href')).toBe('/portal/acme/help/hc_renewal')
  })

  it('starts a support request with the conversation when the articles do not answer', async () => {
    mocks.session.mockResolvedValue({ data: { id: 's1', turns_left: 19, messages: [question, { id: 'a1', role: 'assistant', content: 'The team can help with this.', status: 'handed_off', created_at: '2026-10-27T00:00:00Z', citations: [] }] }, error: null, status: 200 })
    await askQuestion('How is renewal pricing set?')
    const contact = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Contact support')!
    await act(async () => contact.click())
    expect(mocks.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/portal/$slug/new', params: { slug: 'acme' } }))
    const search = mocks.navigate.mock.calls[0][0].search as { subject: string; message: string }
    expect(search.subject).toBe('How is renewal pricing set?')
    expect(search.message).toContain('Help assistant: The team can help with this.')
  })

  it('offers support right away when the assistant is busy', async () => {
    mocks.ask.mockResolvedValue({ data: null, error: 'busy', status: 503 })
    await askQuestion('How is renewal pricing set?')
    expect(container.textContent).toContain('The assistant is busy')
    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent === 'Contact support')).toBe(true)
  })
})
