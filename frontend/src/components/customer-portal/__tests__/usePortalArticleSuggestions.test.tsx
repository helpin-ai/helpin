// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ suggestions: vi.fn(), suggestionEvent: vi.fn() }))
vi.mock('@/lib/services/customerPortalService', () => ({ customerPortalService: { suggestions: mocks.suggestions, suggestionEvent: mocks.suggestionEvent } }))

import { newIntakeId, PORTAL_SUGGESTION_DEBOUNCE_MS, portalSuggestionSearchable, portalSuggestionText, usePortalArticleSuggestions } from '../usePortalArticleSuggestions'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
const probe: { current?: ReturnType<typeof usePortalArticleSuggestions> } = {}

function Probe({ subject, message }: { subject: string; message: string }) {
  const result = usePortalArticleSuggestions('acme', subject, message)
  useEffect(() => { probe.current = result })
  return null
}

const article = (id: string) => ({ id, title: `Article ${id}`, url: `https://help.example/${id}` })

beforeEach(() => {
  vi.useFakeTimers()
  mocks.suggestions.mockReset()
  mocks.suggestionEvent.mockReset()
  mocks.suggestionEvent.mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.useRealTimers()
})

async function type(subject: string, message = '') {
  await act(async () => root.render(<Probe subject={subject} message={message} />))
}
async function pause() {
  await act(async () => { vi.advanceTimersByTime(PORTAL_SUGGESTION_DEBOUNCE_MS) })
  await act(async () => { await Promise.resolve() })
}

describe('portal suggestion text', () => {
  it('uses the subject and the start of the message, with whitespace collapsed', () => {
    expect(portalSuggestionText('  Export  fails ', 'on\nSafari')).toBe('Export fails on Safari')
    expect(portalSuggestionText('Subject', 'x'.repeat(800)).length).toBe('Subject '.length + 500)
  })
  it('mirrors the server minimum', () => {
    expect(portalSuggestionSearchable('Export')).toBe(false)
    expect(portalSuggestionSearchable('Exportfailures')).toBe(false)
    expect(portalSuggestionSearchable('Export fails!')).toBe(true)
  })
})

describe('usePortalArticleSuggestions', () => {
  it('searches once after a pause in typing', async () => {
    mocks.suggestions.mockResolvedValue({ articles: [article('a')] })
    await type('Export')
    await type('Export fails')
    await type('Export fails on Safari')
    expect(mocks.suggestions).not.toHaveBeenCalled()
    await pause()
    expect(mocks.suggestions).toHaveBeenCalledTimes(1)
    expect(mocks.suggestions.mock.calls[0].slice(0, 2)).toEqual(['acme', 'Export fails on Safari'])
    expect(probe.current?.articles.map((item) => item.id)).toEqual(['a'])
  })

  it('does not search short text', async () => {
    await type('Help')
    await pause()
    expect(mocks.suggestions).not.toHaveBeenCalled()
    expect(probe.current?.articles).toEqual([])
  })

  it('cancels a stale search and keeps results while the next one runs', async () => {
    mocks.suggestions.mockResolvedValue({ articles: [] })
    mocks.suggestions.mockResolvedValueOnce({ articles: [article('a')] })
    await type('Export fails on Safari')
    await pause()
    let resolveNext: (value: unknown) => void = () => {}
    mocks.suggestions.mockImplementationOnce(() => new Promise((resolve) => { resolveNext = resolve }))
    await type('Export fails on Firefox')
    await pause()
    const signal = mocks.suggestions.mock.calls[1][2] as AbortSignal
    expect(probe.current?.articles.map((item) => item.id)).toEqual(['a'])
    await type('Export fails on Chrome')
    expect(signal.aborted).toBe(false)
    await pause()
    expect(signal.aborted).toBe(true)
    resolveNext({ articles: [article('stale')] })
    await act(async () => { await Promise.resolve() })
    expect(probe.current?.articles.map((item) => item.id)).not.toContain('stale')
  })

  it('keeps the shown articles when a search fails', async () => {
    mocks.suggestions.mockResolvedValueOnce({ articles: [article('a')] })
    await type('Export fails on Safari')
    await pause()
    mocks.suggestions.mockRejectedValueOnce(new Error('offline'))
    await type('Export fails on Safari today')
    await pause()
    expect(probe.current?.articles.map((item) => item.id)).toEqual(['a'])
  })

  it('hides articles once the text is cleared', async () => {
    mocks.suggestions.mockResolvedValue({ articles: [article('a')] })
    await type('Export fails on Safari')
    await pause()
    await type('')
    expect(probe.current?.articles).toEqual([])
  })

  it('records each distinct set of shown articles once and every opened article', async () => {
    mocks.suggestions.mockResolvedValue({ articles: [article('a'), article('b')] })
    await type('Export fails on Safari')
    await pause()
    await type('Export fails on Safari today')
    await pause()
    const shown = mocks.suggestionEvent.mock.calls.filter(([, event]) => event.event === 'shown')
    expect(shown).toHaveLength(1)
    expect(shown[0][1].article_ids).toEqual(['a', 'b'])
    await act(async () => probe.current!.recordOpened(article('b')))
    const intake = probe.current!.intake()
    expect(mocks.suggestionEvent).toHaveBeenLastCalledWith('acme', { intake_id: intake.intake_id, event: 'opened', article_ids: ['b'] })
    expect(intake.suggested_article_ids).toEqual(['a', 'b'])
    expect(intake.opened_article_ids).toEqual(['b'])
    expect(shown[0][1].intake_id).toBe(intake.intake_id)
  })

  it('ignores a failed measurement event', async () => {
    mocks.suggestionEvent.mockRejectedValue(new Error('offline'))
    mocks.suggestions.mockResolvedValue({ articles: [article('a')] })
    await type('Export fails on Safari')
    await pause()
    expect(probe.current?.articles.map((item) => item.id)).toEqual(['a'])
  })
})

describe('newIntakeId', () => {
  it('makes a v4 UUID without randomUUID, as on plain-http pages', () => {
    const original = crypto.randomUUID
    Object.defineProperty(crypto, 'randomUUID', { value: undefined, configurable: true })
    try {
      expect(newIntakeId()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
    } finally {
      Object.defineProperty(crypto, 'randomUUID', { value: original, configurable: true })
    }
  })
})
