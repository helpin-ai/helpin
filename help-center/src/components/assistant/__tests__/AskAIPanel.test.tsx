import type { ReactNode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AskAIPanel } from '@/components/assistant/AskAIPanel'
import { DocsProvider } from '@/contexts/DocsContext'
import type { AssistantMessage, HelpCenterConfig } from '@/lib/types'

const api = vi.hoisted(() => ({
  ask: vi.fn(),
  session: vi.fn(),
  feedback: vi.fn(),
  portalSlug: vi.fn(),
  askAnswer: vi.fn(),
}))

vi.mock('@/lib/helpAssistant', () => ({
  assistantVisitorId: () => 'visitor-test-1',
  storedAssistantSession: () => null,
  storeAssistantSession: vi.fn(),
  helpAssistantService: { ask: api.ask, session: api.session, feedback: api.feedback, portalSlug: api.portalSlug },
}))
vi.mock('@/lib/services', () => ({ helpCenterService: { askAnswer: api.askAnswer } }))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to, className }: { children: ReactNode; to: string; className?: string }) => <a href={to} className={className}>{children}</a>,
}))

const config = {
  id: 'cfg-1', workspace_id: 'ws-1', subdomain: 'acme', custom_domain: null, brand_name: 'Acme',
  brand_logo_url: null, brand_logo_dark_url: null, brand_color: '#2b70fb', favicon_url: null, theme_mode: 'system',
  search_placeholder: null, default_locale: 'en', enabled_locales: ['en'], show_language_switcher: false,
  fallback_to_default_locale: true, is_published: true, seo_title: null, seo_description: null, support_email: 'help@acme.test',
  header_links: [], footer_config: { links: [], copyright_text: '' }, homepage_config: { featured_cards: [] },
  ai_assistant_enabled: true,
} as HelpCenterConfig

function renderPanel(ui: ReactNode) {
  return render(
    <DocsProvider basepath="" subdomain="acme" locale="en" defaultLocale="en" enabledLocales={['en']} multilingualEnabled={false} config={config} spaces={[]}>
      {ui}
    </DocsProvider>,
  )
}

const question: AssistantMessage = { id: 'q1', role: 'user', content: 'How do I export?', status: 'answered', citations: [], created_at: '2026-10-27T00:00:00Z' }
const pending: AssistantMessage = { id: 'a1', role: 'assistant', content: '', status: 'pending', progress_label: 'Reading the export guide…', citations: [], created_at: '2026-10-27T00:00:00Z' }

async function flush() {
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() })
}

async function ask(text: string) {
  fireEvent.change(screen.getByLabelText('Ask AI a question'), { target: { value: text } })
  fireEvent.keyDown(screen.getByLabelText('Ask AI a question'), { key: 'Enter' })
  await flush()
}

describe('AskAIPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: () => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() }) })
    api.ask.mockResolvedValue({ data: { session_id: 's1', question_id: 'q1', answer_id: 'a1', turns_left: 19 }, error: null, status: 202 })
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.resetAllMocks()
  })

  it('asks, shows progress, then the answer with its sources and feedback', async () => {
    api.session
      .mockResolvedValueOnce({ data: { id: 's1', turns_left: 19, messages: [question, pending] }, error: null, status: 200 })
      .mockResolvedValueOnce({ data: { id: 's1', turns_left: 19, messages: [question, { ...pending, status: 'answered', progress_label: undefined, content: 'Open **Reports** and choose Export.', citations: [{ article_id: 'd1', title: 'Export reports', url: 'https://help.acme.test/articles/export' }] }] }, error: null, status: 200 })
    api.feedback.mockResolvedValue({ data: { saved: true }, error: null, status: 200 })
    renderPanel(<AskAIPanel request={{}} onClose={() => {}} />)
    expect(screen.getByText('How can I help?')).toBeTruthy()

    await ask('How do I export?')
    expect(api.ask).toHaveBeenCalledWith('acme', { visitor_id: 'visitor-test-1', session_id: undefined, message: 'How do I export?', page_article_id: undefined, locale: 'en' })
    expect(screen.getByText('How do I export?')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('Searching the help articles')

    await act(async () => { vi.advanceTimersByTime(1000) })
    await flush()
    expect(screen.getByRole('status').textContent).toContain('Reading the export guide')

    await act(async () => { vi.advanceTimersByTime(1000) })
    await flush()
    expect(screen.getByText('Reports').tagName).toBe('STRONG')
    const source = screen.getByText('Export reports').closest('a')
    expect(source?.getAttribute('href')).toBe('https://help.acme.test/articles/export')

    fireEvent.click(screen.getByLabelText('Helpful'))
    expect(api.feedback).toHaveBeenCalledWith('acme', 'a1', { visitor_id: 'visitor-test-1', session_id: 's1', helpful: true })
    expect(screen.getByText('Thanks for the feedback.')).toBeTruthy()
  })

  it('asks the question it opened with, about the current page', async () => {
    api.session.mockResolvedValue({ data: { id: 's1', turns_left: 19, messages: [question, pending] }, error: null, status: 200 })
    renderPanel(<AskAIPanel request={{ question: 'Summarize this', pageArticleId: 'doc-1', pageTitle: 'Export reports' }} onClose={() => {}} />)
    await flush()
    expect(api.ask).toHaveBeenCalledWith('acme', expect.objectContaining({ message: 'Summarize this', page_article_id: 'doc-1' }))
    expect(screen.getByText('Asking about “Export reports”')).toBeTruthy()
  })

  it('falls back to a quick answer when the assistant is busy', async () => {
    api.ask.mockResolvedValue({ data: null, error: 'busy', status: 503 })
    api.askAnswer.mockResolvedValue({ data: { answer_id: 'x', status: 'answered', answer: 'Use the Export button.', citations: [{ document_id: 'd1', title: 'Export reports', slug: 'export', public_id: 'p1' }], confidence: 0.9, cached: false }, error: null, status: 200 })
    renderPanel(<AskAIPanel request={{}} onClose={() => {}} />)
    await ask('How do I export?')
    await flush()
    expect(screen.getByText('The assistant is busy, so this is a quick answer.')).toBeTruthy()
    expect(screen.getByText('Use the Export button.')).toBeTruthy()
    expect(screen.getByText('Export reports')).toBeTruthy()
  })

  it('offers support when the assistant hands off, and starts a new chat', async () => {
    api.session.mockResolvedValue({ data: { id: 's1', turns_left: 19, messages: [question, { ...pending, status: 'handed_off', progress_label: undefined, content: 'I could not find this. The team can help.' }] }, error: null, status: 200 })
    api.portalSlug.mockResolvedValue({ data: null, error: 'not found', status: 404 })
    renderPanel(<AskAIPanel request={{}} onClose={() => {}} />)
    await ask('Can I get a refund?')
    await act(async () => { vi.advanceTimersByTime(1000) })
    await flush()
    fireEvent.click(screen.getByText('Contact support'))
    await flush()
    expect(screen.getByText('Email help@acme.test and the team will help you.')).toBeTruthy()

    fireEvent.click(screen.getByLabelText('New chat'))
    expect(screen.getByText('How can I help?')).toBeTruthy()
  })

  it('offers a retry when answering fails', async () => {
    api.session.mockResolvedValue({ data: { id: 's1', turns_left: 19, messages: [question, { ...pending, status: 'failed', progress_label: undefined }] }, error: null, status: 200 })
    renderPanel(<AskAIPanel request={{}} onClose={() => {}} />)
    await ask('How do I export?')
    await act(async () => { vi.advanceTimersByTime(1000) })
    await flush()
    expect(screen.getByRole('alert').textContent).toContain('Something went wrong')
    fireEvent.click(screen.getByText('Try again'))
    await flush()
    expect(api.ask).toHaveBeenCalledTimes(2)
  })
})
