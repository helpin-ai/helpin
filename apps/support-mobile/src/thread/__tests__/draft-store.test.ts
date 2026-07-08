import { useDraftStore } from '../draft-store'

beforeEach(() => {
  sessionStorage.clear()
  useDraftStore.setState({ drafts: {} })
})

test('setText stores text for a conversation, defaulting mode to reply', () => {
  useDraftStore.getState().setText('conv-1', 'Hello there')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Hello there', mode: 'reply' })
})

test('setMode preserves existing text for that conversation', () => {
  useDraftStore.getState().setText('conv-1', 'Draft body')
  useDraftStore.getState().setMode('conv-1', 'note')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Draft body', mode: 'note' })
})

test('setText preserves an existing mode for that conversation', () => {
  useDraftStore.getState().setMode('conv-1', 'note')
  useDraftStore.getState().setText('conv-1', 'Internal thought')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'Internal thought', mode: 'note' })
})

test('drafts are tracked independently per conversation', () => {
  useDraftStore.getState().setText('conv-1', 'For conv 1')
  useDraftStore.getState().setText('conv-2', 'For conv 2')
  useDraftStore.getState().setMode('conv-2', 'note')

  expect(useDraftStore.getState().drafts['conv-1']).toEqual({ text: 'For conv 1', mode: 'reply' })
  expect(useDraftStore.getState().drafts['conv-2']).toEqual({ text: 'For conv 2', mode: 'note' })
})

test('clearDraft removes only the given conversation draft', () => {
  useDraftStore.getState().setText('conv-1', 'For conv 1')
  useDraftStore.getState().setText('conv-2', 'For conv 2')

  useDraftStore.getState().clearDraft('conv-1')

  expect(useDraftStore.getState().drafts['conv-1']).toBeUndefined()
  expect(useDraftStore.getState().drafts['conv-2']).toEqual({ text: 'For conv 2', mode: 'reply' })
})

test('setText writes the full drafts map through to sessionStorage', () => {
  useDraftStore.getState().setText('conv-1', 'Persisted text')

  const raw = sessionStorage.getItem('support_composer_drafts')
  expect(raw).not.toBeNull()
  expect(JSON.parse(raw!)).toEqual({ 'conv-1': { text: 'Persisted text', mode: 'reply' } })
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
    JSON.stringify({ 'conv-9': { text: 'Restored after navigation', mode: 'note' } }),
  )

  vi.resetModules()
  const fresh = await import('../draft-store')

  expect(fresh.useDraftStore.getState().drafts['conv-9']).toEqual({
    text: 'Restored after navigation',
    mode: 'note',
  })
})

test('malformed sessionStorage content does not throw and hydrates to an empty map', async () => {
  sessionStorage.setItem('support_composer_drafts', 'not-json{{{')

  vi.resetModules()
  const fresh = await import('../draft-store')

  expect(fresh.useDraftStore.getState().drafts).toEqual({})
})
