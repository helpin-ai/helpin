// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { adjacentConversationId, dispatchSupportShortcut, onSupportShortcut, supportShortcutForEvent } from '../supportShortcuts'

function press(key: string, target: EventTarget | null = document.body, extra: Partial<KeyboardEvent> = {}) {
  return supportShortcutForEvent({ key, metaKey: false, ctrlKey: false, altKey: false, defaultPrevented: false, isComposing: false, repeat: false, target, ...extra })
}

describe('supportShortcutForEvent', () => {
  afterEach(() => { document.body.innerHTML = '' })

  it('maps single keys to inbox actions', () => {
    expect(press('j')).toBe('next')
    expect(press('k')).toBe('previous')
    expect(press('e')).toBe('resolve')
    expect(press('z')).toBe('snooze')
    expect(press('?')).toBe('cheatsheet')
    expect(press('x')).toBeNull()
    expect(press('J')).toBeNull()
  })

  it('never fires while typing or inside menus and dialogs', () => {
    document.body.innerHTML = '<input id="i"><textarea id="t"></textarea><div contenteditable="true"><p id="p">x</p></div><div role="dialog"><button id="b">x</button></div><div role="menu"><div id="m">x</div></div>'
    for (const id of ['i', 't', 'p', 'b', 'm']) expect(press('e', document.getElementById(id))).toBeNull()
  })

  it('ignores modified, composed and already handled keys', () => {
    expect(press('e', document.body, { metaKey: true })).toBeNull()
    expect(press('e', document.body, { ctrlKey: true })).toBeNull()
    expect(press('e', document.body, { altKey: true })).toBeNull()
    expect(press('e', document.body, { isComposing: true })).toBeNull()
    expect(press('e', document.body, { defaultPrevented: true })).toBeNull()
  })

  it('repeats only list movement', () => {
    expect(press('j', document.body, { repeat: true })).toBe('next')
    expect(press('e', document.body, { repeat: true })).toBeNull()
  })
})

describe('adjacentConversationId', () => {
  const ids = ['a', 'b', 'c']
  it('moves within the list and stops at the ends', () => {
    expect(adjacentConversationId(ids, 'b', 'next')).toBe('c')
    expect(adjacentConversationId(ids, 'b', 'previous')).toBe('a')
    expect(adjacentConversationId(ids, 'c', 'next')).toBeNull()
    expect(adjacentConversationId(ids, 'a', 'previous')).toBeNull()
  })
  it('continues from the place of a conversation that just left the list', () => {
    expect(adjacentConversationId(['a', 'c', 'd'], 'b', 'next', 1)).toBe('c')
    expect(adjacentConversationId(['a', 'c', 'd'], 'b', 'previous', 1)).toBe('a')
    expect(adjacentConversationId(['a', 'b'], 'c', 'next', 2)).toBe('b')
  })
  it('starts from an end when nothing in the list is open', () => {
    expect(adjacentConversationId(ids, null, 'next')).toBe('a')
    expect(adjacentConversationId(ids, 'gone', 'previous')).toBe('c')
    expect(adjacentConversationId([], null, 'next')).toBeNull()
  })
})

describe('dispatchSupportShortcut', () => {
  it('reports whether a listener handled the shortcut', () => {
    expect(dispatchSupportShortcut('resolve', 'c1')).toBe(false)
    const stop = onSupportShortcut((detail) => {
      if (detail.action === 'resolve' && detail.conversationId === 'c1') detail.handled = true
    })
    expect(dispatchSupportShortcut('resolve', 'c1')).toBe(true)
    expect(dispatchSupportShortcut('snooze', 'c1')).toBe(false)
    stop()
    expect(dispatchSupportShortcut('resolve', 'c1')).toBe(false)
  })
})
