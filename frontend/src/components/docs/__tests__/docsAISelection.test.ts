// @vitest-environment jsdom
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { afterEach, describe, expect, it } from 'vitest'
import { captureAISelection, mapAISelection, replaceAISelection, sanitizeAIRewrite } from '../docsAISelection'
let editor: Editor
function setup(content = '<p>Hello <strong>world</strong> today.</p>') {
 editor = new Editor({ extensions: [StarterKit], content })
 editor.commands.setTextSelection({ from: 7, to: 12 })
 return captureAISelection(editor)
}
afterEach(() => editor?.destroy())
describe('Docs AI selection safety', () => {
 it('captures only the selection and preserves its marks', () => {
  const selection = setup()
  expect(selection.html).toContain('<strong>world</strong>')
  expect(selection.html).not.toContain('Hello')
 })
 it('replaces just the selected text and supports undo', () => {
  const selection = setup()
  replaceAISelection(editor, selection, '<p><strong>team</strong></p>')
  expect(editor.getText().trim()).toBe('Hello team today.')
  expect(editor.getHTML()).toContain('<strong>team</strong>')
  editor.commands.undo()
  expect(editor.getText().trim()).toBe('Hello world today.')
 })
 it('maps the selection when other text is inserted before it', () => {
  const selection = setup()
  const tr = editor.state.tr.insertText('Hey! ', 1)
  const mapped = mapAISelection(selection, tr)
  editor.view.dispatch(tr)
  replaceAISelection(editor, mapped, '<p>team</p>')
  expect(editor.getText().trim()).toBe('Hey! Hello team today.')
 })
 it('refuses to overwrite a selection edited while AI was working', () => {
  const selection = setup()
  const tr = editor.state.tr.insertText('changed', 7, 12)
  const mapped = mapAISelection(selection, tr)
  editor.view.dispatch(tr)
  expect(() => replaceAISelection(editor, mapped, '<p>team</p>')).toThrow(/changed/)
  expect(editor.getText()).toContain('changed')
 })
 it('removes scripts, embedded content, dangerous links, and event attributes', () => {
  const clean = sanitizeAIRewrite('<p onclick="alert(1)">Hi<script>alert(1)</script><img src=x onerror=alert(1)><a href="javascript:alert(1)">link</a></p>')
  expect(clean).toContain('Hi')
  expect(clean).not.toMatch(/script|onclick|onerror|<img|javascript/)
 })
})
