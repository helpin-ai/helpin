import { cannedToPlainText, detectShortcutToken, replaceRange } from '../canned-shortcuts'

describe('detectShortcutToken', () => {
  test('detects a token at the start of the text', () => {
    expect(detectShortcutToken('!tha', 4)).toEqual({ query: '!tha', start: 0, end: 4 })
  })

  test('detects a token after whitespace', () => {
    expect(detectShortcutToken('hello !ref', 10)).toEqual({ query: '!ref', start: 6, end: 10 })
  })

  test('detects a bare "!" (opens the full list)', () => {
    expect(detectShortcutToken('hi !', 4)).toEqual({ query: '!', start: 3, end: 4 })
  })

  test('returns null when the caret is not in a ! token', () => {
    expect(detectShortcutToken('hello world', 11)).toBeNull()
    expect(detectShortcutToken('email a@b.com', 13)).toBeNull() // ! must start the run
  })

  test('only considers the token ending at the cursor, not later ones', () => {
    // Cursor after "!one " (position 5) — the run ending at the cursor is empty/space.
    expect(detectShortcutToken('!one !two', 5)).toBeNull()
  })
})

describe('cannedToPlainText', () => {
  test('converts HTML paragraphs and <br> to newlines and strips tags', () => {
    expect(cannedToPlainText('<p>Hi there</p><p>Line two<br>Line three</p>')).toBe('Hi there\nLine two\nLine three')
  })

  test('passes plain text through with newlines intact', () => {
    expect(cannedToPlainText('Hello\nWorld')).toBe('Hello\nWorld')
  })

  test('decodes entities and trims', () => {
    expect(cannedToPlainText('<p>Thanks &amp; regards</p>')).toBe('Thanks & regards')
  })
})

describe('replaceRange', () => {
  test('replaces the token range and adds a trailing space, returning the new caret', () => {
    const { text, cursor } = replaceRange('hi !tha', 3, 7, 'Thanks for reaching out!')
    expect(text).toBe('hi Thanks for reaching out! ')
    expect(cursor).toBe(text.length)
  })

  test('does not double a trailing space', () => {
    const { text } = replaceRange('!x', 0, 2, 'done ')
    expect(text).toBe('done ')
  })

  test('inserts into the middle, preserving the tail', () => {
    const { text, cursor } = replaceRange('a ! b', 2, 3, 'X')
    expect(text).toBe('a X  b')
    expect(cursor).toBe(4)
  })
})
