import { useDraftStore } from '../draft-store'
import type { FailedSend } from '../failed-sends-reducer'

beforeEach(() => {
  sessionStorage.clear()
  useDraftStore.setState({ drafts: {} })
})

test('setText stores text for a conversation, defaulting mode to reply', () => {
  useDraftStore.getState().setText('conv-1', 'Hello there')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Hello there', mode: 'reply', failedSends: [] })
})

test('setMode preserves existing text for that conversation', () => {
  useDraftStore.getState().setText('conv-1', 'Draft body')
  useDraftStore.getState().setMode('conv-1', 'note')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Draft body', mode: 'note', failedSends: [] })
})

test('setText preserves an existing mode for that conversation', () => {
  useDraftStore.getState().setMode('conv-1', 'note')
  useDraftStore.getState().setText('conv-1', 'Internal thought')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Internal thought', mode: 'note', failedSends: [] })
})

test('drafts are tracked independently per conversation', () => {
  useDraftStore.getState().setText('conv-1', 'For conv 1')
  useDraftStore.getState().setText('conv-2', 'For conv 2')
  useDraftStore.getState().setMode('conv-2', 'note')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'For conv 1', mode: 'reply', failedSends: [] })
  expect(useDraftStore.getState().drafts['conv-2']).toEqual({ text: 'For conv 2', mode: 'note', failedSends: [] })
})

test('clearDraft removes only the given conversation draft', () => {
  useDraftStore.getState().setText('conv-1', 'For conv 1')
  useDraftStore.getState().setText('conv-2', 'For conv 2')

  useDraftStore.getState().clearDraft('conv-1')

  expect(useDraftStore.getState().drafts['conv-1']).toBeUndefined()
  expect(useDraftStore.getState().drafts['conv-2']).toEqual({ text: 'For conv 2', mode: 'reply', failedSends: [] })
})

test('setText writes the full drafts map through to sessionStorage', () => {
  useDraftStore.getState().setText('conv-1', 'Persisted text')

  const raw = sessionStorage.getItem('support_composer_drafts')
  expect(raw).not.toBeNull()
  expect(JSON.parse(raw!)).toEqual({ 'conv-1': { text: 'Persisted text', mode: 'reply', failedSends: [] } })
})

test('clearDraft writes the updated (draft-removed) map through to sessionStorage', () => {
  useDraftStore.getState().setText('conv-1', 'Persisted text')
  useDraftStore.getState().clearDraft('conv-1')

  const raw = sessionStorage.getItem('support_composer_drafts')
  expect(JSON.parse(raw!)).toEqual({})
})

test('a freshly-imported store hydrates its initial state from sessionStorage', async () => {
  sessionStorage.setItem(
    'support_composer_drafts',
    JSON.stringify({ 'conv-9': { text: 'Restored after navigation', mode: 'note', failedSends: [] } }),
  )

  vi.resetModules()
  const fresh = await import('../draft-store')

  expect(fresh.useDraftStore.getState().drafts['conv-9']).toEqual({
    text: 'Restored after navigation',
    mode: 'note',
    failedSends: [],
  })
})

test('persisted entries predating failedSends hydrate with an empty failedSends array', async () => {
  sessionStorage.setItem(
    'support_composer_drafts',
    JSON.stringify({ 'conv-old': { text: 'Legacy draft', mode: 'note' } }),
  )

  vi.resetModules()
  const fresh = await import('../draft-store')

  expect(fresh.useDraftStore.getState().drafts['conv-old']).toEqual({
    text: 'Legacy draft',
    mode: 'note',
    failedSends: [],
  })
})

test('malformed sessionStorage content does not throw, warns once, and hydrates to an empty map', async () => {
  sessionStorage.setItem('support_composer_drafts', 'not-json{{{')

  vi.resetModules()
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  const fresh = await import('../draft-store')

  expect(fresh.useDraftStore.getState().drafts).toEqual({})
  expect(warn).toHaveBeenCalledTimes(1)
  warn.mockRestore()
})

describe('failed sends', () => {
  const failed: FailedSend = { id: 'f-1', content: 'lost message', mode: 'note' }

  test('addFailedSend appends a chip scoped to its conversation and persists it', () => {
    useDraftStore.getState().addFailedSend('conv-1', failed)

    expect(useDraftStore.getState().drafts['conv-1']?.failedSends).toEqual([failed])
    expect(useDraftStore.getState().drafts['conv-2']).toBeUndefined()

    const raw = sessionStorage.getItem('support_composer_drafts')
    expect(JSON.parse(raw!)['conv-1'].failedSends).toEqual([failed])
  })

  test('removeFailedSend removes only the matching chip in that conversation', () => {
    useDraftStore.getState().addFailedSend('conv-1', failed)
    useDraftStore.getState().addFailedSend('conv-1', { id: 'f-2', content: 'second', mode: 'reply' })
    useDraftStore.getState().addFailedSend('conv-2', { id: 'f-1', content: 'same id, other conv', mode: 'reply' })

    useDraftStore.getState().removeFailedSend('conv-1', 'f-1')

    expect(useDraftStore.getState().drafts['conv-1']?.failedSends.map((f) => f.id)).toEqual(['f-2'])
    // Chips are conversation-scoped: removing f-1 in conv-1 must not touch conv-2's f-1.
    expect(useDraftStore.getState().drafts['conv-2']?.failedSends.map((f) => f.id)).toEqual(['f-1'])
  })

  test('clearDraft resets text/mode but preserves failed-send chips', () => {
    useDraftStore.getState().setText('conv-1', 'about to send')
    useDraftStore.getState().setMode('conv-1', 'note')
    useDraftStore.getState().addFailedSend('conv-1', failed)

    useDraftStore.getState().clearDraft('conv-1')

    expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: '', mode: 'reply', failedSends: [failed] })
  })

  test('failed sends survive a fresh import (sessionStorage round-trip)', async () => {
    useDraftStore.getState().addFailedSend('conv-1', failed)

    vi.resetModules()
    const fresh = await import('../draft-store')

    expect(fresh.useDraftStore.getState().drafts['conv-1']?.failedSends).toEqual([failed])
  })
})
