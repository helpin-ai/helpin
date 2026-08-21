import {
  getLastWorkspaceSlug,
  setLastWorkspaceSlug,
  shouldShowPriming,
  type PushPrimingPref,
} from '@mobile/lib/prefs'

const RE_ASK_AFTER_MS = 7 * 24 * 60 * 60 * 1000

beforeEach(() => localStorage.clear())

test('browser preview remembers the last selected workspace', async () => {
  expect(await getLastWorkspaceSlug()).toBeNull()
  await setLastWorkspaceSlug('support-demo')
  expect(await getLastWorkspaceSlug()).toBe('support-demo')
})

describe('shouldShowPriming', () => {
  test('never asked (no pref persisted yet): shows', () => {
    expect(shouldShowPriming(null, new Date('2026-07-08T00:00:00Z'))).toBe(true)
  })

  test('declined ("later") less than 7 days ago: stays quiet', () => {
    const pref: PushPrimingPref = { decision: 'later', at: '2026-07-05T00:00:00Z' }
    const now = new Date('2026-07-06T00:00:00Z')
    expect(shouldShowPriming(pref, now)).toBe(false)
  })

  test('declined ("later") exactly 7 days ago: re-asks', () => {
    const pref: PushPrimingPref = { decision: 'later', at: '2026-07-01T00:00:00Z' }
    const now = new Date(new Date(pref.at).getTime() + RE_ASK_AFTER_MS)
    expect(shouldShowPriming(pref, now)).toBe(true)
  })

  test('declined ("later") more than 7 days ago: re-asks', () => {
    const pref: PushPrimingPref = { decision: 'later', at: '2026-06-01T00:00:00Z' }
    const now = new Date('2026-07-08T00:00:00Z')
    expect(shouldShowPriming(pref, now)).toBe(true)
  })

  test('already enabled: never shows again', () => {
    const pref: PushPrimingPref = { decision: 'enabled', at: '2026-01-01T00:00:00Z' }
    expect(shouldShowPriming(pref, new Date('2026-07-08T00:00:00Z'))).toBe(false)
  })
})
