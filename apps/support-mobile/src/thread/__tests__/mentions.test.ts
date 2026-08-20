import { detectMentionToken, memberMentionHandle, mentionSuggestions, normalizeMentionHandle } from '../mentions'
import type { MentionMember } from '../mentions'

const members: MentionMember[] = [
  { id: 'm1', user_id: 'u1', email: 'ada@ex.com', display_name: 'Ada Lovelace' },
  { id: 'm2', user_id: 'u2', email: 'bob@ex.com', display_name: 'Bob Stone' },
  { id: 'm3', user_id: 'u3', email: 'grace.hopper@ex.com', display_name: '' },
]

describe('normalizeMentionHandle', () => {
  test('lowercases, drops punctuation, joins words with dots', () => {
    expect(normalizeMentionHandle('Ada Lovelace!')).toBe('ada.lovelace')
  })
})

describe('memberMentionHandle', () => {
  test('derives from display name', () => {
    expect(memberMentionHandle(members[0])).toBe('ada.lovelace')
  })
  test('falls back to the email local-part when no display name', () => {
    expect(memberMentionHandle(members[2])).toBe('grace.hopper')
  })
})

describe('mentionSuggestions', () => {
  test('empty query returns everyone, alphabetized by label', () => {
    const out = mentionSuggestions('', members)
    expect(out.map((s) => s.handle)).toEqual(['ada.lovelace', 'bob.stone', 'grace.hopper'])
  })

  test('filters by handle/name and ranks prefix matches first', () => {
    const out = mentionSuggestions('ad', members)
    expect(out[0].handle).toBe('ada.lovelace')
    expect(out.some((s) => s.handle === 'bob.stone')).toBe(false)
  })

  test('carries availability into the suggestion presentation', () => {
    const out = mentionSuggestions('', [{ ...members[0], presence_status: 'online' }])
    expect(out[0].presenceStatus).toBe('online')
  })
})

describe('detectMentionToken', () => {
  test('detects @token after whitespace', () => {
    expect(detectMentionToken('hey @ad', 7)).toEqual({ query: 'ad', start: 4, end: 7 })
  })
  test('detects a bare @', () => {
    expect(detectMentionToken('note @', 6)).toEqual({ query: '', start: 5, end: 6 })
  })
  test('returns null for an email (@ not at a word boundary)', () => {
    expect(detectMentionToken('mail ada@ex', 11)).toBeNull()
  })
})
