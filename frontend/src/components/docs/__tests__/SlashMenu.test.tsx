// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SlashMenu } from '../SlashMenu'
import { SlashMenuExtension, slashMenuPluginKey } from '../SlashMenuExtension'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('slash menu dismissal', () => {
  let editor: Editor
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    editor = new Editor({ extensions: [StarterKit, SlashMenuExtension], content: '<p>/</p>' })
    vi.spyOn(editor.view, 'coordsAtPos').mockReturnValue({ top: 0, bottom: 20, left: 0, right: 0 })
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    act(() => root.render(<SlashMenu editor={editor} />))
    act(() => {
      editor.commands.setTextSelection(2)
      editor.view.dispatch(editor.state.tr.setMeta(slashMenuPluginKey, { open: true, from: 2, query: '', selectedIndex: 0 }))
    })
  })

  afterEach(() => {
    act(() => root.unmount())
    editor.destroy()
    container.remove()
  })

  function key(key: string) {
    let handled = false
    act(() => {
      const plugin = slashMenuPluginKey.get(editor.state)!
      handled = !!plugin.props.handleKeyDown!.call(plugin, editor.view, new KeyboardEvent('keydown', { key }))
    })
    return handled
  }

  it('dismisses on outside click without deleting typed text', () => {
    act(() => document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true })))
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
    expect(editor.getText()).toBe('/')
  })

  it('offers a visible dismiss button', () => {
    const close = container.querySelector<HTMLButtonElement>('[aria-label="Close block menu"]')
    expect(close).not.toBeNull()
    act(() => close!.click())
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
    expect(editor.getText()).toBe('/')
  })

  it('Escape dismisses the menu and typing does not reopen it', () => {
    expect(key('Escape')).toBe(true)
    act(() => editor.commands.insertContent('text'))
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
    expect(editor.getText()).toBe('/text')
  })

  it('Escape dismisses a submenu completely', () => {
    const parent = Array.from(container.querySelectorAll<HTMLButtonElement>('[data-slash-item]')).find(button => button.textContent?.includes('›'))!
    act(() => parent.click())
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
  })

  it.each(['Enter', 'ArrowDown', 'ArrowUp'])('does not trap %s when no commands match', (pressed) => {
    act(() => editor.commands.insertContent('zzzzzz'))
    expect(key(pressed)).toBe(false)
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
  })

  it('still inserts a selected block and removes the slash query', () => {
    act(() => editor.commands.insertContent('heading'))
    expect(key('Enter')).toBe(true)
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
    expect(editor.isActive('heading', { level: 2 })).toBe(true)
    expect(editor.state.doc.textContent).toBe('')
  })

  it('allows outside dismissal when no commands match', () => {
    act(() => editor.commands.insertContent('zzzzzz'))
    act(() => document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true })))
    expect(slashMenuPluginKey.getState(editor.state).open).toBe(false)
    expect(editor.getText()).toBe('/zzzzzz')
  })
})
